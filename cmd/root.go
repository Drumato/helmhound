package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/Drumato/helmhound/pkg/helmwrap"
	"github.com/Drumato/helmhound/pkg/yamldiff"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	c := &cobra.Command{
		Use:   "helmhound",
		Short: "A Helm chart value selector using fzf.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logger based on log-level flag
			logLevel, err := cmd.Flags().GetString("log-level")
			if err != nil {
				return fmt.Errorf("failed to get log-level flag: %v", err)
			}

			var level slog.Level
			switch strings.ToLower(logLevel) {
			case "debug":
				level = slog.LevelDebug
			case "info":
				level = slog.LevelInfo
			case "warn", "warning":
				level = slog.LevelWarn
			case "error":
				level = slog.LevelError
			default:
				level = slog.LevelInfo
			}

			// Set up slog to output to stderr with the specified level
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))
			slog.SetDefault(logger)

			chartUrl, err := cmd.Flags().GetString("chart-url")
			if err != nil {
				return fmt.Errorf("failed to get chart-url flag: %v", err)
			}
			chartVersion, err := cmd.Flags().GetString("chart-version")
			if err != nil {
				return fmt.Errorf("failed to get chart-version flag: %v", err)
			}

			if chartUrl == "" {
				return fmt.Errorf("chart-url is required")
			}
			if chartVersion == "" {
				return fmt.Errorf("chart-version is required")
			}

			client, err := helmwrap.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create helm client: %v", err)
			}

			slog.Info("Downloading chart...")
			chartPath, chartName, err := client.DownloadChart(chartUrl, chartVersion)
			if err != nil {
				return fmt.Errorf("failed to download chart: %v", err)
			}

			slog.Debug("Chart downloaded", "path", chartPath, "name", chartName)

			slog.Info("Reading chart values...")
			values, err := client.ReadValuesFromChart(chartPath, chartName)
			if err != nil {
				return fmt.Errorf("failed to read chart values: %v", err)
			}

			slog.Debug("Values extracted", "length", len(values))

			valuePaths, err := helmwrap.ExtractValuePaths(values)
			if err != nil {
				return fmt.Errorf("failed to extract value paths: %v", err)
			}

			slog.Debug("Value paths extracted", "count", len(valuePaths))

			valuePath, err := cmd.Flags().GetString("value-path")
			if err != nil {
				return fmt.Errorf("failed to get value-path flag: %v", err)
			}

			valuesFile, err := cmd.Flags().GetString("values-file")
			if err != nil {
				return fmt.Errorf("failed to get values-file flag: %v", err)
			}

			var selectedPath string
			if valuePath != "" {
				selectedPath = valuePath
			} else {
				selectedPath, err = selectValueWithFzf(valuePaths)
				if err != nil {
					return fmt.Errorf("failed to select value: %v", err)
				}
			}

			// Get the type of the selected value and log it
			valueType, err := helmwrap.GetValueType(values, selectedPath)
			if err != nil {
				slog.Debug("Failed to get value type", "path", selectedPath, "error", err)
			} else {
				slog.Debug("Value type detected", "path", selectedPath, "type", valueTypeToString(valueType))
			}

			fmt.Printf("Selected value path: %s\n", selectedPath)

			// Render original template
			slog.Info("Rendering original template...")
			originalManifest, err := client.RenderTemplate(chartPath, chartName, valuesFile)
			if err != nil {
				return fmt.Errorf("failed to render original template: %v", err)
			}

			slog.Debug("Original template rendered", "manifest_keys", len(originalManifest))

			// Render template with modified value
			slog.Info("Rendering template with modified value...")
			modifiedManifest, err := client.RenderTemplateWithModifiedValue(chartPath, chartName, selectedPath, valuesFile)
			if err != nil {
				return fmt.Errorf("failed to render template with modified value: %v", err)
			}

			slog.Debug("Modified template rendered", "manifest_keys", len(modifiedManifest))

			// Compare manifests and find differences
			slog.Info("Comparing manifests...")
			groupedDiffs := yamldiff.CompareYAMLGroupedDetailed(originalManifest, modifiedManifest)

			if len(groupedDiffs) == 0 {
				fmt.Printf("No differences found in the rendered manifests for path '%s'.\n", selectedPath)
				fmt.Println("This suggests that the selected value path may not affect the template rendering.")
				fmt.Println("The value might be:")
				fmt.Println("  - Used only in specific conditions that are not met")
				fmt.Println("  - A configuration option that doesn't impact manifest generation")
				fmt.Println("  - An unused or deprecated field in the chart")
				return nil
			}

			totalPaths := 0
			for _, items := range groupedDiffs {
				totalPaths += len(items)
			}

			fmt.Printf("\nDifferences found (%d paths):\n", totalPaths)
			for manifestKey, items := range groupedDiffs {
				fmt.Printf("%s:\n", manifestKey)
				for _, item := range items {
					fmt.Printf("  - %s\n", item.DisplayText)
				}
				fmt.Println()
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	c.Flags().String("chart-url", "", "URL of the Helm chart")
	c.Flags().String("chart-version", "", "Version of the Helm chart")
	c.Flags().String("value-path", "", "Specific value path to search for (skips interactive selection)")
	c.Flags().String("values-file", "", "Path to custom values.yaml file to merge with chart defaults")
	c.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")

	// Add cache subcommand
	c.AddCommand(NewCacheCommand())

	return c
}

func selectValueWithFzf(values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("no values to select from")
	}

	cmd := exec.Command("fzf", "--prompt=Select value: ")
	cmd.Stdin = strings.NewReader(strings.Join(values, "\n"))
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 130 { // User canceled (Ctrl+C)
				return "", fmt.Errorf("selection canceled")
			}
		}
		return "", fmt.Errorf("fzf execution failed: %v", err)
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", fmt.Errorf("no value selected")
	}

	return selected, nil
}

func valueTypeToString(vt helmwrap.ValueType) string {
	switch vt {
	case helmwrap.ValueTypeString:
		return "string"
	case helmwrap.ValueTypeInt:
		return "int"
	case helmwrap.ValueTypeBool:
		return "bool"
	case helmwrap.ValueTypeSlice:
		return "slice"
	case helmwrap.ValueTypeMap:
		return "map"
	default:
		return "unknown"
	}
}
