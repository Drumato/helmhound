package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/Drumato/helmhound/pkg/helmwrap"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	c := &cobra.Command{
		Use:   "helmhound",
		Short: "A Helm analyzer that shows which Helm values affect which resources.",
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

			client, err := helmwrap.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create helm client: %v", err)
			}

			spinner, err := pterm.DefaultSpinner.Start("Downloading chart...")
			if err != nil {
				return fmt.Errorf("failed to start spinner: %v", err)
			}
			chartPath, chartName, err := client.DownloadChart(chartUrl, chartVersion)
			if err != nil {
				return fmt.Errorf("failed to download chart: %v", err)
			}
			spinner.Stop()

			slog.Debug("Chart downloaded", "path", chartPath, "name", chartName)

			spinner, err = pterm.DefaultSpinner.Start("Reading chart values...")
			if err != nil {
				return fmt.Errorf("failed to start spinner: %v", err)
			}
			values, err := client.ReadValuesFromChart(chartPath, chartName)
			if err != nil {
				return fmt.Errorf("failed to read chart values: %v", err)
			}
			spinner.Stop()

			slog.Debug("Values extracted", "length", len(values))

			valuePaths, err := helmwrap.ExtractValuePaths(values)
			if err != nil {
				return fmt.Errorf("failed to extract value paths: %v", err)
			}

			slog.Debug("Value paths extracted", "count", len(valuePaths))
			if len(valuePaths) > 0 && slog.Default().Enabled(context.TODO(), slog.LevelDebug) {
				limit := min(len(valuePaths), 10)
				firstPaths := valuePaths[:limit]
				slog.Debug("First value paths", "paths", firstPaths)
			}

			valuePath, err := cmd.Flags().GetString("value-path")
			if err != nil {
				return fmt.Errorf("failed to get value-path flag: %v", err)
			}

			slog.Debug("Searching for value path", "path", valuePath)

			// Log debug information about path search
			if slog.Default().Enabled(cmd.Context(), slog.LevelDebug) {
				// Check if the requested path exists
				found := false
				for _, path := range valuePaths {
					if path == valuePath {
						found = true
						break
					}
				}
				slog.Debug("Value path existence check", "path", valuePath, "found", found)

				// If looking for prometheus-related paths, show additional info
				if strings.Contains(valuePath, "prometheus") {
					prometheusCount := 0
					var prometheusPaths []string
					for _, path := range valuePaths {
						if strings.Contains(path, "prometheus") {
							prometheusCount++
							if prometheusCount <= 20 { // Limit to first 20
								prometheusPaths = append(prometheusPaths, path)
							}
						}
					}
					slog.Debug("Prometheus-related paths found", "total_count", prometheusCount, "sample_paths", prometheusPaths)
				}
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

			spinner, err = pterm.DefaultSpinner.Start("Reading template files...")
			if err != nil {
				return fmt.Errorf("failed to start spinner: %v", err)
			}
			templates, err := client.ReadAllTemplateFiles(chartPath, chartName)
			if err != nil {
				return fmt.Errorf("failed to read template files: %v", err)
			}
			spinner.Stop()

			spinner, err = pterm.DefaultSpinner.Start("Searching for value references...")
			if err != nil {
				return fmt.Errorf("failed to start spinner: %v", err)
			}

			slog.Debug("Template files loaded", "count", len(templates))
			slog.Debug("Searching for value references", "value", selectedPath)

			references := helmwrap.SearchValueInTemplates(templates, selectedPath)
			spinner.Stop()

			slog.Debug("Value references found", "count", len(references))

			output := helmwrap.FormatValueReferences(references)
			fmt.Printf("\nSearching for value: %s\n\n%s", selectedPath, output)
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	c.Flags().String("chart-url", "", "URL of the Helm chart")
	c.Flags().String("chart-version", "", "Version of the Helm chart")
	c.Flags().String("value-path", "", "Specific value path to search for (skips interactive selection)")
	c.Flags().String("log-level", "info", "Log level (debug, info, warn, error)")

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
