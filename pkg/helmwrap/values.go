package helmwrap

import (
	"fmt"
	"gopkg.in/yaml.v3"
)

// ValueType represents the type of a value in Helm chart values
type ValueType uint

const (
	ValueTypeString ValueType = iota
	ValueTypeInt
	ValueTypeBool
	ValueTypeSlice
	ValueTypeMap
	ValueTypeUnknown
)

// ExtractValuePaths recursively extracts all possible paths from a YAML string.
// Time complexity: O(n) where n is the total number of nodes in the YAML structure
// Space complexity: O(n) for storing all paths + O(d) for recursion stack depth d
func ExtractValuePaths(valuesYaml string) ([]string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYaml), &data); err != nil {
		return nil, err
	}

	var paths []string
	extractPaths("", data, &paths)
	return paths, nil
}

func extractPaths(prefix string, data interface{}, paths *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			fullPath := key
			if prefix != "" {
				fullPath = prefix + "." + key
			}
			*paths = append(*paths, fullPath)
			extractPaths(fullPath, value, paths)
		}
	case []interface{}:
		for i, item := range v {
			indexPath := prefix + "[" + string(rune(i+'0')) + "]"
			*paths = append(*paths, indexPath)
			extractPaths(indexPath, item, paths)
		}
	}
}

// GetValueType determines the type of a value at the specified path in the YAML structure
func GetValueType(valuesYaml, path string) (ValueType, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(valuesYaml), &data); err != nil {
		return ValueTypeUnknown, err
	}

	value, err := getValueAtPath(data, path)
	if err != nil {
		return ValueTypeUnknown, err
	}

	return determineValueType(value), nil
}

func getValueAtPath(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	keys := splitPath(path)
	current := data

	for _, key := range keys {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[key]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("key '%s' not found", key)
			}
		default:
			return nil, fmt.Errorf("cannot access key '%s' on non-map type", key)
		}
	}

	return current, nil
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	var parts []string
	var current string

	for _, char := range path {
		if char == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func determineValueType(value interface{}) ValueType {
	switch value.(type) {
	case string:
		return ValueTypeString
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return ValueTypeInt
	case float32, float64:
		return ValueTypeInt
	case bool:
		return ValueTypeBool
	case []interface{}:
		return ValueTypeSlice
	case map[string]interface{}:
		return ValueTypeMap
	default:
		return ValueTypeUnknown
	}
}
