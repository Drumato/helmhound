package yamldiff

import (
	"reflect"
	"strconv"
)

// CompareYAML compares two YAML maps and returns a slice of paths where differences are found
func CompareYAML(left, right map[string]interface{}) []string {
	var diffs []string
	compareValues("", left, right, &diffs)
	return diffs
}

// compareValues recursively compares two values and records differences
func compareValues(path string, left, right interface{}, diffs *[]string) {
	// Handle nil cases
	if left == nil && right == nil {
		return
	}
	if left == nil || right == nil {
		*diffs = append(*diffs, path)
		return
	}

	leftType := reflect.TypeOf(left)
	rightType := reflect.TypeOf(right)

	// If types are different, record as difference
	if leftType != rightType {
		*diffs = append(*diffs, path)
		return
	}

	switch leftVal := left.(type) {
	case map[string]interface{}:
		rightVal := right.(map[string]interface{})
		compareMap(path, leftVal, rightVal, diffs)
	case []interface{}:
		rightVal := right.([]interface{})
		compareSlice(path, leftVal, rightVal, diffs)
	default:
		// Compare primitive values
		if !reflect.DeepEqual(left, right) {
			*diffs = append(*diffs, path)
		}
	}
}

// compareMap compares two maps and records differences
func compareMap(basePath string, left, right map[string]interface{}, diffs *[]string) {
	// Check all keys in left map
	for key, leftValue := range left {
		newPath := buildPath(basePath, key)
		if rightValue, exists := right[key]; exists {
			compareValues(newPath, leftValue, rightValue, diffs)
		} else {
			// Key exists in left but not in right
			*diffs = append(*diffs, newPath)
		}
	}

	// Check for keys that exist only in right map
	for key := range right {
		if _, exists := left[key]; !exists {
			newPath := buildPath(basePath, key)
			*diffs = append(*diffs, newPath)
		}
	}
}

// compareSlice compares two slices and records differences
func compareSlice(basePath string, left, right []interface{}, diffs *[]string) {
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	for i := 0; i < maxLen; i++ {
		newPath := buildArrayPath(basePath, i)

		if i >= len(left) {
			// Element exists in right but not in left
			*diffs = append(*diffs, newPath)
		} else if i >= len(right) {
			// Element exists in left but not in right
			*diffs = append(*diffs, newPath)
		} else {
			// Compare elements at the same index
			compareValues(newPath, left[i], right[i], diffs)
		}
	}
}

// buildPath constructs a path string for nested map keys
func buildPath(basePath, key string) string {
	if basePath == "" {
		return key
	}
	return basePath + "." + key
}

// buildArrayPath constructs a path string for array indices
func buildArrayPath(basePath string, index int) string {
	indexStr := "[" + strconv.Itoa(index) + "]"
	if basePath == "" {
		return indexStr
	}
	return basePath + indexStr
}

// FindDifferencesWithValues compares two YAML maps and returns differences with their values
func FindDifferencesWithValues(left, right map[string]interface{}) map[string]DiffValue {
	diffs := make(map[string]DiffValue)
	findDifferencesWithValues("", left, right, diffs)
	return diffs
}

// DiffValue represents a difference between two values
type DiffValue struct {
	Left  interface{} `json:"left"`
	Right interface{} `json:"right"`
	Type  DiffType    `json:"type"`
}

// DiffType represents the type of difference
type DiffType string

const (
	DiffTypeModified DiffType = "modified"
	DiffTypeAdded    DiffType = "added"
	DiffTypeRemoved  DiffType = "removed"
)

// findDifferencesWithValues recursively finds differences and stores them with values
func findDifferencesWithValues(path string, left, right interface{}, diffs map[string]DiffValue) {
	// Handle nil cases
	if left == nil && right == nil {
		return
	}
	if left == nil {
		diffs[path] = DiffValue{Left: nil, Right: right, Type: DiffTypeAdded}
		return
	}
	if right == nil {
		diffs[path] = DiffValue{Left: left, Right: nil, Type: DiffTypeRemoved}
		return
	}

	leftType := reflect.TypeOf(left)
	rightType := reflect.TypeOf(right)

	// If types are different, record as modified
	if leftType != rightType {
		diffs[path] = DiffValue{Left: left, Right: right, Type: DiffTypeModified}
		return
	}

	switch leftVal := left.(type) {
	case map[string]interface{}:
		rightVal := right.(map[string]interface{})
		findMapDifferencesWithValues(path, leftVal, rightVal, diffs)
	case []interface{}:
		rightVal := right.([]interface{})
		findSliceDifferencesWithValues(path, leftVal, rightVal, diffs)
	default:
		// Compare primitive values
		if !reflect.DeepEqual(left, right) {
			diffs[path] = DiffValue{Left: left, Right: right, Type: DiffTypeModified}
		}
	}
}

// findMapDifferencesWithValues finds differences in maps with values
func findMapDifferencesWithValues(basePath string, left, right map[string]interface{}, diffs map[string]DiffValue) {
	// Check all keys in left map
	for key, leftValue := range left {
		newPath := buildPath(basePath, key)
		if rightValue, exists := right[key]; exists {
			findDifferencesWithValues(newPath, leftValue, rightValue, diffs)
		} else {
			// Key exists in left but not in right
			diffs[newPath] = DiffValue{Left: leftValue, Right: nil, Type: DiffTypeRemoved}
		}
	}

	// Check for keys that exist only in right map
	for key, rightValue := range right {
		if _, exists := left[key]; !exists {
			newPath := buildPath(basePath, key)
			diffs[newPath] = DiffValue{Left: nil, Right: rightValue, Type: DiffTypeAdded}
		}
	}
}

// findSliceDifferencesWithValues finds differences in slices with values
func findSliceDifferencesWithValues(basePath string, left, right []interface{}, diffs map[string]DiffValue) {
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	for i := 0; i < maxLen; i++ {
		newPath := buildArrayPath(basePath, i)

		if i >= len(left) {
			// Element exists in right but not in left
			diffs[newPath] = DiffValue{Left: nil, Right: right[i], Type: DiffTypeAdded}
		} else if i >= len(right) {
			// Element exists in left but not in right
			diffs[newPath] = DiffValue{Left: left[i], Right: nil, Type: DiffTypeRemoved}
		} else {
			// Compare elements at the same index
			findDifferencesWithValues(newPath, left[i], right[i], diffs)
		}
	}
}

// GroupedDifferences represents differences grouped by manifest
type GroupedDifferences map[string][]string

// GroupedDifferenceItem represents a single difference item with user-friendly display
type GroupedDifferenceItem struct {
	Path        string // Original path
	DisplayText string // User-friendly display text
}

// GroupedDifferencesDetailed represents differences grouped by manifest with detailed information
type GroupedDifferencesDetailed map[string][]GroupedDifferenceItem

// CompareYAMLGrouped compares two YAML maps and returns differences grouped by manifest
func CompareYAMLGrouped(left, right map[string]interface{}) GroupedDifferences {
	var diffs []string
	compareValues("", left, right, &diffs)

	grouped := make(GroupedDifferences)
	for _, path := range diffs {
		manifestKey := extractManifestKey(path)
		grouped[manifestKey] = append(grouped[manifestKey], path)
	}

	return grouped
}

// CompareYAMLGroupedDetailed compares two YAML maps and returns differences grouped by manifest with detailed information
func CompareYAMLGroupedDetailed(left, right map[string]interface{}) GroupedDifferencesDetailed {
	var diffs []string
	compareValues("", left, right, &diffs)

	grouped := make(GroupedDifferencesDetailed)
	for _, path := range diffs {
		manifestKey := extractManifestKey(path)
		displayText := createUserFriendlyDisplayText(path, manifestKey, left, right)
		item := GroupedDifferenceItem{
			Path:        path,
			DisplayText: displayText,
		}
		grouped[manifestKey] = append(grouped[manifestKey], item)
	}

	return grouped
}

// createUserFriendlyDisplayText creates a user-friendly display text for a difference
func createUserFriendlyDisplayText(path, manifestKey string, left, right map[string]interface{}) string {
	// If the path is exactly the manifest key, it means the entire manifest was affected
	if path == manifestKey {
		return "(affects entire manifest)"
	}

	// For field-level changes, return the original path
	return path
}

// extractManifestKey extracts the manifest key from a path
// For example: "Secret/alertmanager-helmhound-render-kube-prom-alertmanager" from
// "Secret/alertmanager-helmhound-render-kube-prom-alertmanager.metadata.name"
func extractManifestKey(path string) string {
	// Find the first dot to separate manifest key from field path
	for i, char := range path {
		if char == '.' {
			return path[:i]
		}
	}
	// If no dot found, return the entire path as manifest key
	return path
}
