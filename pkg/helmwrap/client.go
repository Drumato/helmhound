package helmwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

type Client interface {
	DownloadChart(chartUrl, chartVersion string) (string, string, error)
	ReadValuesFromChart(chartDir, chartName string) (string, error)
	ReadAllTemplateFiles(chartDir, chartName string) (map[string]string, error)
}

type helmClient struct {
	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

func NewClient() (Client, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), func(format string, v ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", v...)
	}); err != nil {
		return nil, fmt.Errorf("failed to initialize action config: %v", err)
	}

	registryClient, err := registry.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %v", err)
	}
	actionConfig.RegistryClient = registryClient

	return &helmClient{
		settings:     settings,
		actionConfig: actionConfig,
	}, nil
}

func (c *helmClient) DownloadChart(chartUrl, chartVersion string) (string, string, error) {
	tempDir := os.TempDir()
	helmhoundDir := filepath.Join(tempDir, "helmhound")

	// Extract chart name from URL for directory naming
	baseName := extractChartNameFromURL(chartUrl)

	if err := os.MkdirAll(helmhoundDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create helmhound directory: %v", err)
	}

	// The chart will be downloaded using the actual chart name from the repository
	actualChartDir := filepath.Join(helmhoundDir, baseName)

	// Create Pull action with proper configuration using NewPullWithOpts and WithConfig
	pull := action.NewPullWithOpts(action.WithConfig(c.actionConfig))
	pull.Version = chartVersion
	pull.Untar = true
	pull.Settings = c.settings
	pull.DestDir = helmhoundDir

	// Check if we already have the chart, if so read its version
	if _, err := os.Stat(actualChartDir); err == nil {
		// Chart directory already exists, check if it's the right version
		existingVersion, err := readChartVersion(actualChartDir)
		if err == nil {
			finalChartName := fmt.Sprintf("%s-%s", baseName, existingVersion)
			finalChartDir := filepath.Join(helmhoundDir, finalChartName)

			// Check if versioned directory exists
			if _, err := os.Stat(finalChartDir); err == nil {
				return helmhoundDir, finalChartName, nil
			}

			// Rename existing chart to versioned directory
			if err := os.Rename(actualChartDir, finalChartDir); err == nil {
				return helmhoundDir, finalChartName, nil
			}
		}

		// If there's an issue with existing chart, remove it and redownload
		os.RemoveAll(actualChartDir)
	}

	// Download the chart
	_, err := pull.Run(chartUrl)
	if err != nil {
		return "", "", fmt.Errorf("failed to pull chart: %v", err)
	}

	// Read the actual version from Chart.yaml
	actualVersion, err := readChartVersion(actualChartDir)
	if err != nil {
		// Clean up on error
		os.RemoveAll(actualChartDir)
		return "", "", fmt.Errorf("failed to read chart version: %v", err)
	}

	// Create final chart name with actual version
	finalChartName := fmt.Sprintf("%s-%s", baseName, actualVersion)
	finalChartDir := filepath.Join(helmhoundDir, finalChartName)

	// Check if the versioned chart already exists
	if _, err := os.Stat(finalChartDir); err == nil {
		// Versioned chart already exists, remove the downloaded one and return existing
		os.RemoveAll(actualChartDir)
		return helmhoundDir, finalChartName, nil
	}

	// Rename downloaded directory to final versioned directory
	if err := os.Rename(actualChartDir, finalChartDir); err != nil {
		os.RemoveAll(actualChartDir)
		return "", "", fmt.Errorf("failed to rename chart directory: %v", err)
	}

	return helmhoundDir, finalChartName, nil
}

// extractChartNameFromURL extracts chart name from various chart URL formats
func extractChartNameFromURL(chartUrl string) string {
	// Handle OCI URLs like "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack"
	if strings.HasPrefix(chartUrl, "oci://") {
		parts := strings.Split(chartUrl, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1] // Return the last part as chart name
		}
	}

	// Handle other URL formats or fallback
	parts := strings.Split(strings.TrimSuffix(chartUrl, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return "chart" // fallback name
}

// ChartMetadata represents the structure of Chart.yaml
type ChartMetadata struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// readChartVersion reads the version from Chart.yaml in the downloaded chart directory
func readChartVersion(chartDir string) (string, error) {
	chartYamlPath := filepath.Join(chartDir, "Chart.yaml")

	// Check if Chart.yaml exists
	if _, err := os.Stat(chartYamlPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Chart.yaml not found in directory: %s", chartDir)
	}

	// Read Chart.yaml content
	content, err := os.ReadFile(chartYamlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Chart.yaml: %v", err)
	}

	// Parse YAML to extract version
	var metadata ChartMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		return "", fmt.Errorf("failed to parse Chart.yaml: %v", err)
	}

	if metadata.Version == "" {
		return "", fmt.Errorf("version not found in Chart.yaml")
	}

	return metadata.Version, nil
}

// ReadValuesFromChart reads values.yaml from downloaded chart directory
func (c *helmClient) ReadValuesFromChart(chartDir, chartName string) (string, error) {
	// Look for values.yaml in the chart directory
	valuesPath := filepath.Join(chartDir, chartName, "values.yaml")

	// Check if the file exists
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		return "", fmt.Errorf("values.yaml not found in chart directory: %s", valuesPath)
	}

	// Read the values.yaml file
	content, err := os.ReadFile(valuesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read values.yaml: %v", err)
	}

	return string(content), nil
}

// ReadAllTemplateFiles recursively reads all template files in the chart's templates directory.
// It returns a map where keys are relative file paths and values are file contents.
// Only .yaml and .yml files are included in the result.
// Time complexity: O(n) where n is the number of template files
// Space complexity: O(m) where m is the total size of all template files
func (c *helmClient) ReadAllTemplateFiles(chartDir, chartName string) (map[string]string, error) {
	templatesPath := filepath.Join(chartDir, chartName, "templates")

	// Check if templates directory exists
	if _, err := os.Stat(templatesPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("templates directory not found: %s", templatesPath)
	}

	templateFiles := make(map[string]string)

	err := filepath.Walk(templatesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-template files
		if info.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %v", path, err)
		}

		// Use relative path from templates directory as key
		relativePath, err := filepath.Rel(templatesPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %v", path, err)
		}

		templateFiles[relativePath] = string(content)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory: %v", err)
	}

	return templateFiles, nil
}
