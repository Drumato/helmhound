package yamldiff

import (
	"reflect"
	"sort"
	"testing"
)

func TestCompareYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected []string
	}{
		{
			name: "identical maps",
			left: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			right: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			expected: []string{},
		},
		{
			name: "different primitive values",
			left: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			right: map[string]interface{}{
				"key1": "different",
				"key2": 43,
			},
			expected: []string{"key1", "key2"},
		},
		{
			name: "missing keys",
			left: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			right: map[string]interface{}{
				"key1": "value1",
			},
			expected: []string{"key2"},
		},
		{
			name: "additional keys",
			left: map[string]interface{}{
				"key1": "value1",
			},
			right: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			expected: []string{"key2"},
		},
		{
			name: "nested maps",
			left: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"name": "test",
						},
					},
				},
			},
			right: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 5,
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"name": "test-modified",
						},
					},
				},
			},
			expected: []string{"spec.replicas", "spec.template.metadata.name"},
		},
		{
			name: "array differences",
			left: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1"},
						map[string]interface{}{"fieldA": "valueA2"},
					},
				},
			},
			right: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1-modified"},
						map[string]interface{}{"fieldA": "valueA2"},
					},
				},
			},
			expected: []string{"spec.list[0].fieldA"},
		},
		{
			name: "array length differences",
			left: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1"},
						map[string]interface{}{"fieldA": "valueA2"},
					},
				},
			},
			right: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1"},
					},
				},
			},
			expected: []string{"spec.list[1]"},
		},
		{
			name: "complex nested structure",
			left: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-app",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx:1.20",
							"ports": []interface{}{
								map[string]interface{}{"containerPort": 80},
							},
						},
					},
				},
			},
			right: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-app",
				},
				"spec": map[string]interface{}{
					"replicas": 5,
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx:1.21",
							"ports": []interface{}{
								map[string]interface{}{"containerPort": 80},
								map[string]interface{}{"containerPort": 443},
							},
						},
					},
				},
			},
			expected: []string{"spec.replicas", "spec.containers[0].image", "spec.containers[0].ports[1]"},
		},
		{
			name: "nil values",
			left: map[string]interface{}{
				"key1": nil,
				"key2": "value",
			},
			right: map[string]interface{}{
				"key1": "not-nil",
				"key2": nil,
			},
			expected: []string{"key1", "key2"},
		},
		{
			name: "type changes",
			left: map[string]interface{}{
				"key1": "string",
				"key2": 42,
				"key3": true,
			},
			right: map[string]interface{}{
				"key1": 123,
				"key2": "string",
				"key3": map[string]interface{}{"nested": "value"},
			},
			expected: []string{"key1", "key2", "key3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := CompareYAML(tt.left, tt.right)

			// Sort both slices for comparison since order doesn't matter
			sort.Strings(result)
			sort.Strings(tt.expected)

			// Handle the case where both are empty slices
			if len(result) == 0 && len(tt.expected) == 0 {
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFindDifferencesWithValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected map[string]DiffValue
	}{
		{
			name: "modified values",
			left: map[string]interface{}{
				"key1": "old",
				"key2": 42,
			},
			right: map[string]interface{}{
				"key1": "new",
				"key2": 43,
			},
			expected: map[string]DiffValue{
				"key1": {Left: "old", Right: "new", Type: DiffTypeModified},
				"key2": {Left: 42, Right: 43, Type: DiffTypeModified},
			},
		},
		{
			name: "added and removed values",
			left: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			right: map[string]interface{}{
				"key1": "value1",
				"key3": "value3",
			},
			expected: map[string]DiffValue{
				"key2": {Left: "value2", Right: nil, Type: DiffTypeRemoved},
				"key3": {Left: nil, Right: "value3", Type: DiffTypeAdded},
			},
		},
		{
			name: "nested structure changes",
			left: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1"},
					},
				},
			},
			right: map[string]interface{}{
				"spec": map[string]interface{}{
					"list": []interface{}{
						map[string]interface{}{"fieldA": "valueA1-modified"},
					},
				},
			},
			expected: map[string]DiffValue{
				"spec.list[0].fieldA": {Left: "valueA1", Right: "valueA1-modified", Type: DiffTypeModified},
			},
		},
		{
			name: "array length changes",
			left: map[string]interface{}{
				"items": []interface{}{"a", "b"},
			},
			right: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
			expected: map[string]DiffValue{
				"items[2]": {Left: nil, Right: "c", Type: DiffTypeAdded},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := FindDifferencesWithValues(tt.left, tt.right)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestBuildPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		basePath string
		key      string
		expected string
	}{
		{
			name:     "empty base path",
			basePath: "",
			key:      "key1",
			expected: "key1",
		},
		{
			name:     "non-empty base path",
			basePath: "spec.template",
			key:      "metadata",
			expected: "spec.template.metadata",
		},
		{
			name:     "single level",
			basePath: "root",
			key:      "child",
			expected: "root.child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildPath(tt.basePath, tt.key)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuildArrayPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		basePath string
		index    int
		expected string
	}{
		{
			name:     "empty base path",
			basePath: "",
			index:    0,
			expected: "[0]",
		},
		{
			name:     "non-empty base path",
			basePath: "spec.list",
			index:    1,
			expected: "spec.list[1]",
		},
		{
			name:     "nested array",
			basePath: "data.items[0].subItems",
			index:    2,
			expected: "data.items[0].subItems[2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildArrayPath(tt.basePath, tt.index)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCompareYAMLGrouped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected GroupedDifferences
	}{
		{
			name: "no differences",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			expected: GroupedDifferences{},
		},
		{
			name: "grouped differences",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
					"data": map[string]interface{}{
						"key1": "value1",
					},
				},
				"Deployment/app": map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 3,
					},
				},
			},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager-modified",
					},
					"data": map[string]interface{}{
						"key1": "value1-modified",
					},
				},
				"Deployment/app": map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 5,
					},
				},
			},
			expected: GroupedDifferences{
				"Secret/alertmanager": {
					"Secret/alertmanager.metadata.name",
					"Secret/alertmanager.data.key1",
				},
				"Deployment/app": {
					"Deployment/app.spec.replicas",
				},
			},
		},
		{
			name: "single manifest multiple changes",
			left: map[string]interface{}{
				"ConfigMap/config": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config",
						"namespace": "default",
					},
					"data": map[string]interface{}{
						"config.yaml": "old-config",
					},
				},
			},
			right: map[string]interface{}{
				"ConfigMap/config": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-new",
						"namespace": "kube-system",
					},
					"data": map[string]interface{}{
						"config.yaml": "new-config",
					},
				},
			},
			expected: GroupedDifferences{
				"ConfigMap/config": {
					"ConfigMap/config.metadata.name",
					"ConfigMap/config.metadata.namespace",
					"ConfigMap/config.data.config.yaml",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := CompareYAMLGrouped(tt.left, tt.right)

			// Sort the slices in each group for comparison
			for key, paths := range result {
				sort.Strings(paths)
				result[key] = paths
			}
			for key, paths := range tt.expected {
				sort.Strings(paths)
				tt.expected[key] = paths
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestExtractManifestKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "secret with nested path",
			path:     "Secret/alertmanager-helmhound-render-kube-prom-alertmanager.metadata.name",
			expected: "Secret/alertmanager-helmhound-render-kube-prom-alertmanager",
		},
		{
			name:     "deployment with deep nesting",
			path:     "Deployment/app.spec.template.metadata.labels.version",
			expected: "Deployment/app",
		},
		{
			name:     "path without dots",
			path:     "Secret/simple-secret",
			expected: "Secret/simple-secret",
		},
		{
			name:     "configmap with array index",
			path:     "ConfigMap/config.data.items[0].key",
			expected: "ConfigMap/config",
		},
		{
			name:     "single key",
			path:     "key",
			expected: "key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractManifestKey(tt.path)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCompareYAMLGroupedDetailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected GroupedDifferencesDetailed
	}{
		{
			name: "entire manifest removed",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			right: map[string]interface{}{},
			expected: GroupedDifferencesDetailed{
				"Secret/alertmanager": {
					{
						Path:        "Secret/alertmanager",
						DisplayText: "(affects entire manifest)",
					},
				},
			},
		},
		{
			name: "entire manifest added",
			left: map[string]interface{}{},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			expected: GroupedDifferencesDetailed{
				"Secret/alertmanager": {
					{
						Path:        "Secret/alertmanager",
						DisplayText: "(affects entire manifest)",
					},
				},
			},
		},
		{
			name: "field-level changes",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
					"data": map[string]interface{}{
						"key1": "value1",
					},
				},
			},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager-modified",
					},
					"data": map[string]interface{}{
						"key1": "value1-modified",
					},
				},
			},
			expected: GroupedDifferencesDetailed{
				"Secret/alertmanager": {
					{
						Path:        "Secret/alertmanager.metadata.name",
						DisplayText: "Secret/alertmanager.metadata.name",
					},
					{
						Path:        "Secret/alertmanager.data.key1",
						DisplayText: "Secret/alertmanager.data.key1",
					},
				},
			},
		},
		{
			name: "mixed changes",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
				"ConfigMap/config": map[string]interface{}{
					"data": map[string]interface{}{
						"config.yaml": "old-config",
					},
				},
			},
			right: map[string]interface{}{
				"ConfigMap/config": map[string]interface{}{
					"data": map[string]interface{}{
						"config.yaml": "new-config",
					},
				},
				"Deployment/app": map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": 3,
					},
				},
			},
			expected: GroupedDifferencesDetailed{
				"Secret/alertmanager": {
					{
						Path:        "Secret/alertmanager",
						DisplayText: "(affects entire manifest)",
					},
				},
				"ConfigMap/config": {
					{
						Path:        "ConfigMap/config.data.config.yaml",
						DisplayText: "ConfigMap/config.data.config.yaml",
					},
				},
				"Deployment/app": {
					{
						Path:        "Deployment/app",
						DisplayText: "(affects entire manifest)",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := CompareYAMLGroupedDetailed(tt.left, tt.right)

			// Sort the slices in each group for comparison
			for key, items := range result {
				sort.Slice(items, func(i, j int) bool {
					return items[i].Path < items[j].Path
				})
				result[key] = items
			}
			for key, items := range tt.expected {
				sort.Slice(items, func(i, j int) bool {
					return items[i].Path < items[j].Path
				})
				tt.expected[key] = items
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestCreateUserFriendlyDisplayText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		manifestKey string
		left        map[string]interface{}
		right       map[string]interface{}
		expected    string
	}{
		{
			name:        "entire manifest removed",
			path:        "Secret/alertmanager",
			manifestKey: "Secret/alertmanager",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			right:    map[string]interface{}{},
			expected: "(affects entire manifest)",
		},
		{
			name:        "entire manifest added",
			path:        "Secret/alertmanager",
			manifestKey: "Secret/alertmanager",
			left:        map[string]interface{}{},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			expected: "(affects entire manifest)",
		},
		{
			name:        "field-level change",
			path:        "Secret/alertmanager.metadata.name",
			manifestKey: "Secret/alertmanager",
			left: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager",
					},
				},
			},
			right: map[string]interface{}{
				"Secret/alertmanager": map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "alertmanager-modified",
					},
				},
			},
			expected: "Secret/alertmanager.metadata.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := createUserFriendlyDisplayText(tt.path, tt.manifestKey, tt.left, tt.right)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
