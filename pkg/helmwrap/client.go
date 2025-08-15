package helmwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

type Client interface {
	DownloadChart(chartUrl, chartVersion string) (string, string, error)
	ReadValuesFromChart(chartDir, chartName string) (string, error)
	RenderTemplate(chartDir, chartName, valuesFile string) (map[string]interface{}, error)
	RenderTemplateWithModifiedValue(chartDir, chartName, valuePath, valuesFile string) (map[string]interface{}, error)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %v", err)
	}
	helmhoundDir := filepath.Join(homeDir, ".helmhound")

	if err := os.MkdirAll(helmhoundDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create helmhound directory: %v", err)
	}

	// Check cache first
	if entry, exists := checkCacheEntry(helmhoundDir, chartUrl, chartVersion); exists {
		return entry.DownloadDir, entry.ChartName, nil
	}

	// Extract chart name from URL for directory naming
	baseName := extractChartNameFromURL(chartUrl)

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
				// Add to cache and return
				addCacheEntry(helmhoundDir, chartUrl, existingVersion, finalChartName, helmhoundDir)
				return helmhoundDir, finalChartName, nil
			}

			// Rename existing chart to versioned directory
			if err := os.Rename(actualChartDir, finalChartDir); err == nil {
				// Add to cache and return
				addCacheEntry(helmhoundDir, chartUrl, existingVersion, finalChartName, helmhoundDir)
				return helmhoundDir, finalChartName, nil
			}
		}

		// If there's an issue with existing chart, remove it and redownload
		if err := os.RemoveAll(actualChartDir); err != nil {
			return "", "", fmt.Errorf("failed to remove existing chart directory: %v", err)
		}
	}

	// Download the chart
	_, err = pull.Run(chartUrl)
	if err != nil {
		return "", "", fmt.Errorf("failed to pull chart: %v", err)
	}

	// Read the actual version from Chart.yaml
	actualVersion, err := readChartVersion(actualChartDir)
	if err != nil {
		// Clean up on error
		if removeErr := os.RemoveAll(actualChartDir); removeErr != nil {
			// Log the cleanup error but return the original error
			fmt.Fprintf(os.Stderr, "failed to clean up chart directory: %v\n", removeErr)
		}
		return "", "", fmt.Errorf("failed to read chart version: %v", err)
	}

	// Create final chart name with actual version
	finalChartName := fmt.Sprintf("%s-%s", baseName, actualVersion)
	finalChartDir := filepath.Join(helmhoundDir, finalChartName)

	// Check if the versioned chart already exists
	if _, err := os.Stat(finalChartDir); err == nil {
		// Versioned chart already exists, remove the downloaded one and return existing
		if err := os.RemoveAll(actualChartDir); err != nil {
			return "", "", fmt.Errorf("failed to remove duplicate chart directory: %v", err)
		}
		// Add to cache and return
		addCacheEntry(helmhoundDir, chartUrl, actualVersion, finalChartName, helmhoundDir)
		return helmhoundDir, finalChartName, nil
	}

	// Rename downloaded directory to final versioned directory
	if err := os.Rename(actualChartDir, finalChartDir); err != nil {
		if removeErr := os.RemoveAll(actualChartDir); removeErr != nil {
			// Log the cleanup error but return the original error
			fmt.Fprintf(os.Stderr, "failed to clean up chart directory: %v\n", removeErr)
		}
		return "", "", fmt.Errorf("failed to rename chart directory: %v", err)
	}

	// Add to cache
	addCacheEntry(helmhoundDir, chartUrl, actualVersion, finalChartName, helmhoundDir)

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

// CacheEntry represents a single cache entry
type CacheEntry struct {
	ChartURL    string `yaml:"chart_url"`
	Version     string `yaml:"version"`
	ChartName   string `yaml:"chart_name"`
	DownloadDir string `yaml:"download_dir"`
}

// CacheFile represents the structure of cache.yaml
type CacheFile struct {
	Entries []CacheEntry `yaml:"entries"`
}

// readChartVersion reads the version from Chart.yaml in the downloaded chart directory
func readChartVersion(chartDir string) (string, error) {
	chartYamlPath := filepath.Join(chartDir, "Chart.yaml")

	// Check if Chart.yaml exists
	if _, err := os.Stat(chartYamlPath); os.IsNotExist(err) {
		return "", fmt.Errorf("chart.yaml not found in directory: %s", chartDir)
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

// mergeValues merges user-provided values file with chart default values
func (c *helmClient) mergeValues(chartDir, chartName, valuesFile string) (map[string]interface{}, error) {
	// Start with chart default values
	defaultValuesContent, err := c.ReadValuesFromChart(chartDir, chartName)
	if err != nil {
		return nil, fmt.Errorf("failed to read default values: %v", err)
	}

	var defaultValues map[string]interface{}
	if err := yaml.Unmarshal([]byte(defaultValuesContent), &defaultValues); err != nil {
		return nil, fmt.Errorf("failed to parse default values: %v", err)
	}

	// If no custom values file provided, return default values
	if valuesFile == "" {
		return defaultValues, nil
	}

	// Read custom values file
	customContent, err := os.ReadFile(valuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read custom values file %s: %v", valuesFile, err)
	}

	var customValues map[string]interface{}
	if err := yaml.Unmarshal(customContent, &customValues); err != nil {
		return nil, fmt.Errorf("failed to parse custom values file: %v", err)
	}

	// Merge custom values into default values (custom values override defaults)
	mergedValues := make(map[string]interface{})
	copyMap(defaultValues, mergedValues)
	deepMergeMap(mergedValues, customValues)

	return mergedValues, nil
}

// deepMergeMap merges source map into destination map recursively
func deepMergeMap(dst, src map[string]interface{}) {
	for key, srcValue := range src {
		if dstValue, exists := dst[key]; exists {
			// If both values are maps, merge them recursively
			if dstMap, dstIsMap := dstValue.(map[string]interface{}); dstIsMap {
				if srcMap, srcIsMap := srcValue.(map[string]interface{}); srcIsMap {
					deepMergeMap(dstMap, srcMap)
					continue
				}
			}
		}
		// For all other cases, override with source value
		dst[key] = srcValue
	}
}

// RenderTemplate renders the Helm chart with merged values and returns the result as map[string]interface{}
func (c *helmClient) RenderTemplate(chartDir, chartName, valuesFile string) (map[string]interface{}, error) {
	// Get merged values
	mergedValues, err := c.mergeValues(chartDir, chartName, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to merge values: %v", err)
	}

	// Load chart from the downloaded directory
	chartPath := filepath.Join(chartDir, chartName)
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %v", err)
	}

	// Create install action to render templates
	install := action.NewInstall(c.actionConfig)
	install.DryRun = true // This makes it only render templates without installing
	install.ReleaseName = "helmhound-render"
	install.Namespace = c.settings.Namespace()
	install.IsUpgrade = false
	install.ClientOnly = true
	install.IncludeCRDs = true
	install.SkipSchemaValidation = true

	// Remove kubeVersion constraint from chart metadata to avoid compatibility issues
	originalKubeVersion := chart.Metadata.KubeVersion
	chart.Metadata.KubeVersion = ""

	// Run the install action in dry-run mode to get rendered templates
	release, err := install.Run(chart, mergedValues) // Use merged values

	// Restore original kubeVersion constraint
	chart.Metadata.KubeVersion = originalKubeVersion

	if err != nil {
		return nil, fmt.Errorf("failed to render templates: %v", err)
	}

	// Parse the manifest as multiple YAML documents separated by ---
	documents := strings.Split(release.Manifest, "---\n")
	manifestMap := make(map[string]interface{})

	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var docData map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &docData); err != nil {
			// If parsing as map fails, store as raw string
			manifestMap[fmt.Sprintf("document_%d", i)] = doc
			continue
		}

		// Use kind and name as key if available
		if kind, ok := docData["kind"].(string); ok {
			key := kind
			if metadata, ok := docData["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok {
					key = fmt.Sprintf("%s_%s", kind, name)
				}
			}
			manifestMap[key] = docData
		} else {
			manifestMap[fmt.Sprintf("document_%d", i)] = docData
		}
	}

	return manifestMap, nil
}

// RenderTemplateWithModifiedValue renders the Helm chart with a modified value at the specified path
func (c *helmClient) RenderTemplateWithModifiedValue(chartDir, chartName, valuePath, valuesFile string) (map[string]interface{}, error) {
	// Get merged values
	mergedValues, err := c.mergeValues(chartDir, chartName, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to merge values: %v", err)
	}

	// Convert merged values to YAML for GetValueType function
	mergedValuesYAML, err := yaml.Marshal(mergedValues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged values: %v", err)
	}

	// Get the current value type from merged values
	valueType, err := GetValueType(string(mergedValuesYAML), valuePath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine value type at path %s: %v", valuePath, err)
	}

	// Modify the value based on its type
	modifiedValues, err := modifyValueAtPath(mergedValues, valuePath, valueType)
	if err != nil {
		return nil, fmt.Errorf("failed to modify value at path %s: %v", valuePath, err)
	}

	// Load chart from the downloaded directory
	chartPath := filepath.Join(chartDir, chartName)
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %v", err)
	}

	// Create install action to render templates
	install := action.NewInstall(c.actionConfig)
	install.DryRun = true // This makes it only render templates without installing
	install.ReleaseName = "helmhound-render"
	install.Namespace = c.settings.Namespace()
	install.IsUpgrade = false
	install.ClientOnly = true
	install.IncludeCRDs = true
	install.SkipSchemaValidation = true

	// Remove kubeVersion constraint from chart metadata to avoid compatibility issues
	originalKubeVersion := chart.Metadata.KubeVersion
	chart.Metadata.KubeVersion = ""

	// Run the install action with modified values
	release, err := install.Run(chart, modifiedValues)

	// Restore original kubeVersion constraint
	chart.Metadata.KubeVersion = originalKubeVersion

	if err != nil {
		return nil, fmt.Errorf("failed to render templates: %v", err)
	}

	// Parse the manifest as multiple YAML documents separated by ---
	documents := strings.Split(release.Manifest, "---\n")
	manifestMap := make(map[string]interface{})

	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var docData map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &docData); err != nil {
			// If parsing as map fails, store as raw string
			manifestMap[fmt.Sprintf("document_%d", i)] = doc
			continue
		}

		// Use kind and name as key if available
		if kind, ok := docData["kind"].(string); ok {
			key := kind
			if metadata, ok := docData["metadata"].(map[string]interface{}); ok {
				if name, ok := metadata["name"].(string); ok {
					key = fmt.Sprintf("%s_%s", kind, name)
				}
			}
			manifestMap[key] = docData
		} else {
			manifestMap[fmt.Sprintf("document_%d", i)] = docData
		}
	}

	return manifestMap, nil
}

// modifyValueAtPath modifies a value at the specified path based on its type
func modifyValueAtPath(values map[string]interface{}, path string, valueType ValueType) (map[string]interface{}, error) {
	// Create a deep copy of the values map
	modifiedValues := make(map[string]interface{})
	copyMap(values, modifiedValues)

	// Get the current value
	currentValue, err := getValueAtPath(modifiedValues, path)
	if err != nil {
		return nil, err
	}

	// Modify based on type
	var newValue interface{}
	switch valueType {
	case ValueTypeString:
		if strVal, ok := currentValue.(string); ok {
			// Use a more distinctive test value that will definitely cause differences
			newValue = "helmhound-test-" + strVal
		} else {
			return nil, fmt.Errorf("expected string value at path %s", path)
		}
	case ValueTypeInt:
		switch v := currentValue.(type) {
		case int:
			newValue = v + 1
		case int8:
			newValue = v + 1
		case int16:
			newValue = v + 1
		case int32:
			newValue = v + 1
		case int64:
			newValue = v + 1
		case uint:
			newValue = v + 1
		case uint8:
			newValue = v + 1
		case uint16:
			newValue = v + 1
		case uint32:
			newValue = v + 1
		case uint64:
			newValue = v + 1
		case float32:
			newValue = int(v) + 1
		case float64:
			newValue = int(v) + 1
		default:
			return nil, fmt.Errorf("expected numeric value at path %s", path)
		}
	case ValueTypeBool:
		if boolVal, ok := currentValue.(bool); ok {
			newValue = !boolVal
		} else {
			return nil, fmt.Errorf("expected boolean value at path %s", path)
		}
	case ValueTypeSlice:
		if sliceVal, ok := currentValue.([]interface{}); ok {
			// Add a test element to the slice
			modifiedSlice := make([]interface{}, len(sliceVal))
			copy(modifiedSlice, sliceVal)
			modifiedSlice = append(modifiedSlice, "helmhound-test-element")
			newValue = modifiedSlice
		} else {
			return nil, fmt.Errorf("expected slice value at path %s", path)
		}
	case ValueTypeMap:
		if mapVal, ok := currentValue.(map[string]interface{}); ok {
			// Add a test key-value pair to the map
			modifiedMap := make(map[string]interface{})
			copyMap(mapVal, modifiedMap)
			modifiedMap["helmhound-test-key"] = "helmhound-test-value"
			newValue = modifiedMap
		} else {
			return nil, fmt.Errorf("expected map value at path %s", path)
		}
	default:
		return nil, fmt.Errorf("unsupported value type for modification: %v", valueType)
	}

	// Set the new value at the specified path
	err = setValueAtPath(modifiedValues, path, newValue)
	if err != nil {
		return nil, err
	}

	return modifiedValues, nil
}

// copyMap creates a deep copy of a map[string]interface{}
func copyMap(src, dst map[string]interface{}) {
	for key, value := range src {
		switch v := value.(type) {
		case map[string]interface{}:
			nested := make(map[string]interface{})
			copyMap(v, nested)
			dst[key] = nested
		case []interface{}:
			dst[key] = copySlice(v)
		default:
			dst[key] = v
		}
	}
}

// copySlice creates a deep copy of a []interface{}
func copySlice(src []interface{}) []interface{} {
	dst := make([]interface{}, len(src))
	for i, item := range src {
		switch v := item.(type) {
		case map[string]interface{}:
			nested := make(map[string]interface{})
			copyMap(v, nested)
			dst[i] = nested
		case []interface{}:
			dst[i] = copySlice(v)
		default:
			dst[i] = v
		}
	}
	return dst
}

// setValueAtPath sets a value at the specified path in the map
func setValueAtPath(data map[string]interface{}, path string, value interface{}) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}

	keys := splitPath(path)
	current := data

	// Navigate to the parent of the target key
	for i := 0; i < len(keys)-1; i++ {
		key := keys[i]
		if next, ok := current[key]; ok {
			if nextMap, ok := next.(map[string]interface{}); ok {
				current = nextMap
			} else {
				return fmt.Errorf("cannot navigate through non-map value at key '%s'", key)
			}
		} else {
			// Create missing intermediate maps
			newMap := make(map[string]interface{})
			current[key] = newMap
			current = newMap
		}
	}

	// Set the final value
	finalKey := keys[len(keys)-1]
	current[finalKey] = value

	return nil
}

// checkCacheEntry checks if a chart with given URL and version exists in cache
func checkCacheEntry(helmhoundDir, chartUrl, chartVersion string) (CacheEntry, bool) {
	cacheFile := loadCacheFile(helmhoundDir)

	for _, entry := range cacheFile.Entries {
		if entry.ChartURL == chartUrl && entry.Version == chartVersion {
			// Verify that the cached directory still exists
			chartPath := filepath.Join(entry.DownloadDir, entry.ChartName)
			if _, err := os.Stat(chartPath); err == nil {
				return entry, true
			}
		}
	}

	return CacheEntry{}, false
}

// addCacheEntry adds a new entry to the cache file
func addCacheEntry(helmhoundDir, chartUrl, version, chartName, downloadDir string) {
	cacheFile := loadCacheFile(helmhoundDir)

	// Check if entry already exists (avoid duplicates)
	for _, entry := range cacheFile.Entries {
		if entry.ChartURL == chartUrl && entry.Version == version {
			return // Already exists
		}
	}

	// Add new entry
	newEntry := CacheEntry{
		ChartURL:    chartUrl,
		Version:     version,
		ChartName:   chartName,
		DownloadDir: downloadDir,
	}

	cacheFile.Entries = append(cacheFile.Entries, newEntry)
	saveCacheFile(helmhoundDir, cacheFile)
}

// loadCacheFile loads the cache file or returns empty cache if file doesn't exist
func loadCacheFile(helmhoundDir string) CacheFile {
	cacheFilePath := filepath.Join(helmhoundDir, "cache.yaml")

	// If cache file doesn't exist, return empty cache
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		return CacheFile{Entries: []CacheEntry{}}
	}

	// Read and parse cache file
	content, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return CacheFile{Entries: []CacheEntry{}}
	}

	var cacheFile CacheFile
	if err := yaml.Unmarshal(content, &cacheFile); err != nil {
		return CacheFile{Entries: []CacheEntry{}}
	}

	return cacheFile
}

// saveCacheFile saves the cache file to disk
func saveCacheFile(helmhoundDir string, cacheFile CacheFile) {
	cacheFilePath := filepath.Join(helmhoundDir, "cache.yaml")

	content, err := yaml.Marshal(cacheFile)
	if err != nil {
		return // Silent failure for now
	}

	if err := os.WriteFile(cacheFilePath, content, 0644); err != nil {
		// Log the error but don't fail the operation
		fmt.Fprintf(os.Stderr, "failed to write cache file: %v\n", err)
	}
}
