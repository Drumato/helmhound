package helmwrap

import (
	"reflect"
	"testing"
)

func TestModifyValueAtPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		values      map[string]interface{}
		path        string
		valueType   ValueType
		expected    interface{}
		expectError bool
	}{
		{
			name: "modify string value",
			values: map[string]interface{}{
				"test": "hello",
			},
			path:      "test",
			valueType: ValueTypeString,
			expected:  "helmhound-test-hello",
		},
		{
			name: "modify int value",
			values: map[string]interface{}{
				"count": 42,
			},
			path:      "count",
			valueType: ValueTypeInt,
			expected:  43,
		},
		{
			name: "modify bool value",
			values: map[string]interface{}{
				"enabled": true,
			},
			path:      "enabled",
			valueType: ValueTypeBool,
			expected:  false,
		},
		{
			name: "modify nested string value",
			values: map[string]interface{}{
				"config": map[string]interface{}{
					"database": map[string]interface{}{
						"host": "localhost",
					},
				},
			},
			path:      "config.database.host",
			valueType: ValueTypeString,
			expected:  "helmhound-test-localhost",
		},
		{
			name: "modify nested int value",
			values: map[string]interface{}{
				"server": map[string]interface{}{
					"port": 8080,
				},
			},
			path:      "server.port",
			valueType: ValueTypeInt,
			expected:  8081,
		},
		{
			name: "modify nested bool value",
			values: map[string]interface{}{
				"features": map[string]interface{}{
					"auth": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			path:      "features.auth.enabled",
			valueType: ValueTypeBool,
			expected:  true,
		},
		{
			name: "modify slice value",
			values: map[string]interface{}{
				"tags": []interface{}{"tag1", "tag2"},
			},
			path:      "tags",
			valueType: ValueTypeSlice,
			expected:  []interface{}{"tag1", "tag2", "helmhound-test-element"},
		},
		{
			name: "modify nested slice value",
			values: map[string]interface{}{
				"config": map[string]interface{}{
					"environments": []interface{}{"dev", "prod"},
				},
			},
			path:      "config.environments",
			valueType: ValueTypeSlice,
			expected:  []interface{}{"dev", "prod", "helmhound-test-element"},
		},
		{
			name: "modify map value",
			values: map[string]interface{}{
				"settings": map[string]interface{}{
					"debug":   true,
					"timeout": 30,
				},
			},
			path:      "settings",
			valueType: ValueTypeMap,
			expected: map[string]interface{}{
				"debug":              true,
				"timeout":            30,
				"helmhound-test-key": "helmhound-test-value",
			},
		},
		{
			name: "modify nested map value",
			values: map[string]interface{}{
				"database": map[string]interface{}{
					"postgres": map[string]interface{}{
						"host": "localhost",
						"port": 5432,
					},
				},
			},
			path:      "database.postgres",
			valueType: ValueTypeMap,
			expected: map[string]interface{}{
				"host":               "localhost",
				"port":               5432,
				"helmhound-test-key": "helmhound-test-value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := modifyValueAtPath(tt.values, tt.path, tt.valueType)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Get the modified value
			actual, err := getValueAtPath(result, tt.path)
			if err != nil {
				t.Errorf("failed to get value at path %s: %v", tt.path, err)
				return
			}

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestCopyMap(t *testing.T) {
	t.Parallel()

	original := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"key": "nested_value",
		},
		"slice": []interface{}{1, 2, 3},
	}

	copied := make(map[string]interface{})
	copyMap(original, copied)

	// Modify the original
	original["simple"] = "modified"
	if nestedMap, ok := original["nested"].(map[string]interface{}); ok {
		nestedMap["key"] = "modified_nested"
	}
	if slice, ok := original["slice"].([]interface{}); ok {
		slice[0] = 999
	}

	// Check that copied values are not affected
	if copied["simple"] != "value" {
		t.Errorf("expected 'value', got %v", copied["simple"])
	}

	if nestedMap, ok := copied["nested"].(map[string]interface{}); ok {
		if nestedMap["key"] != "nested_value" {
			t.Errorf("expected 'nested_value', got %v", nestedMap["key"])
		}
	} else {
		t.Error("nested map not found in copied data")
	}

	if slice, ok := copied["slice"].([]interface{}); ok {
		if slice[0] != 1 {
			t.Errorf("expected 1, got %v", slice[0])
		}
	} else {
		t.Error("slice not found in copied data")
	}
}

func TestSetValueAtPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		data        map[string]interface{}
		path        string
		value       interface{}
		expectError bool
	}{
		{
			name: "set simple value",
			data: map[string]interface{}{
				"test": "old",
			},
			path:  "test",
			value: "new",
		},
		{
			name: "set nested value",
			data: map[string]interface{}{
				"config": map[string]interface{}{
					"database": map[string]interface{}{
						"host": "old_host",
					},
				},
			},
			path:  "config.database.host",
			value: "new_host",
		},
		{
			name:  "create missing path",
			data:  map[string]interface{}{},
			path:  "new.nested.key",
			value: "value",
		},
		{
			name:        "empty path",
			data:        map[string]interface{}{},
			path:        "",
			value:       "value",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := setValueAtPath(tt.data, tt.path, tt.value)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the value was set correctly
			actual, err := getValueAtPath(tt.data, tt.path)
			if err != nil {
				t.Errorf("failed to get value at path %s: %v", tt.path, err)
				return
			}

			if actual != tt.value {
				t.Errorf("expected %v, got %v", tt.value, actual)
			}
		})
	}
}
