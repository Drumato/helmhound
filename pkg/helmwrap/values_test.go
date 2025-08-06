package helmwrap

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestExtractValuePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		valuesYaml string
		want       []string
		wantErr    bool
	}{
		{
			name: "should extract paths from simple YAML",
			valuesYaml: `
key1: value1
key2: value2
`,
			want: []string{"key1", "key2"},
		},
		{
			name: "should extract nested paths",
			valuesYaml: `
database:
  host: localhost
  port: 5432
  credentials:
    username: user
    password: pass
`,
			want: []string{
				"database",
				"database.host",
				"database.port",
				"database.credentials",
				"database.credentials.username",
				"database.credentials.password",
			},
		},
		{
			name: "should extract array paths",
			valuesYaml: `
items:
  - name: item1
    value: val1
  - name: item2
    value: val2
`,
			want: []string{
				"items",
				"items[0]",
				"items[0].name",
				"items[0].value",
				"items[1]",
				"items[1].name",
				"items[1].value",
			},
		},
		{
			name:       "should return error for invalid YAML",
			valuesYaml: "invalid: yaml: content: [",
			wantErr:    true,
		},
		{
			name:       "should handle empty YAML",
			valuesYaml: "",
			want:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractValuePaths(tt.valuesYaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractValuePaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Sort both slices for comparison since map iteration order is not guaranteed
				sort.Strings(got)
				sort.Strings(tt.want)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("ExtractValuePaths() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestFilterValuesByPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
		prefix string
		want   []string
	}{
		{
			name:   "should filter values with prefix",
			values: []string{"database.host", "database.port", "api.host", "api.port"},
			prefix: "database",
			want:   []string{"database.host", "database.port"},
		},
		{
			name:   "should return all values when prefix is empty",
			values: []string{"key1", "key2", "key3"},
			prefix: "",
			want:   []string{"key1", "key2", "key3"},
		},
		{
			name:   "should return empty slice when no matches",
			values: []string{"api.host", "api.port"},
			prefix: "database",
			want:   nil,
		},
		{
			name:   "should handle empty values slice",
			values: []string{},
			prefix: "any",
			want:   nil,
		},
		{
			name:   "should match exact prefix",
			values: []string{"database", "database.host", "databaseconfig"},
			prefix: "database",
			want:   []string{"database", "database.host", "databaseconfig"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FilterValuesByPrefix(tt.values, tt.prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterValuesByPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkExtractValuePaths(b *testing.B) {
	testCases := []struct {
		name       string
		valuesYaml string
	}{
		{
			name: "simple_yaml",
			valuesYaml: `
key1: value1
key2: value2
key3: value3
`,
		},
		{
			name: "nested_yaml",
			valuesYaml: `
database:
  host: localhost
  port: 5432
  credentials:
    username: user
    password: pass
api:
  host: api.example.com
  port: 8080
  timeout: 30s
`,
		},
		{
			name: "complex_yaml",
			valuesYaml: `
services:
  frontend:
    replicas: 3
    image: nginx:latest
    ports:
      - 80
      - 443
    env:
      - name: NODE_ENV
        value: production
  backend:
    replicas: 2
    image: app:v1.0.0
    ports:
      - 8080
    env:
      - name: DATABASE_URL
        value: postgresql://localhost:5432/app
      - name: REDIS_URL
        value: redis://localhost:6379
database:
  host: postgres
  port: 5432
  name: myapp
  credentials:
    username: dbuser
    password: dbpass
`,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := ExtractValuePaths(tc.valuesYaml)
				if err != nil {
					b.Fatalf("ExtractValuePaths failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkFilterValuesByPrefix(b *testing.B) {
	// Generate test data with different sizes
	testSizes := []struct {
		name   string
		values []string
		prefix string
	}{
		{
			name:   "small_10_items",
			values: generateTestValues(10),
			prefix: "database",
		},
		{
			name:   "medium_100_items",
			values: generateTestValues(100),
			prefix: "database",
		},
		{
			name:   "large_1000_items",
			values: generateTestValues(1000),
			prefix: "database",
		},
		{
			name:   "xlarge_10000_items",
			values: generateTestValues(10000),
			prefix: "database",
		},
	}

	for _, tc := range testSizes {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = FilterValuesByPrefix(tc.values, tc.prefix)
			}
		})
	}
}

func generateTestValues(size int) []string {
	values := make([]string, size)
	prefixes := []string{"database", "api", "cache", "storage", "monitoring"}

	for i := 0; i < size; i++ {
		prefix := prefixes[i%len(prefixes)]
		values[i] = fmt.Sprintf("%s.config%d", prefix, i)
	}
	return values
}

func TestSearchValueInTemplates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		templates map[string]string
		valuePath string
		want      []ValueReference
	}{
		{
			name: "should find simple value reference",
			templates: map[string]string{
				"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.nameOverride }}
spec:
  replicas: {{ .Values.replicaCount }}`,
			},
			valuePath: "nameOverride",
			want: []ValueReference{
				{File: "deployment.yaml", Line: 4, Content: ".Values.nameOverride", FullLine: "name: {{ .Values.nameOverride }}", ManifestPath: ".metadata.name"},
			},
		},
		{
			name: "should find nested value reference",
			templates: map[string]string{
				"service.yaml": `apiVersion: v1
kind: Service
metadata:
  name: service
spec:
  ports:
  - port: {{ .Values.service.port }}
  type: {{ .Values.service.type }}`,
			},
			valuePath: "service.port",
			want: []ValueReference{
				{File: "service.yaml", Line: 7, Content: ".Values.service.port", FullLine: "- port: {{ .Values.service.port }}", ManifestPath: ".spec.- port"},
			},
		},
		{
			name: "should find multiple references in same file",
			templates: map[string]string{
				"configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.name }}
data:
  config: |
    name: {{ .Values.name }}
    debug: {{ .Values.debug }}`,
			},
			valuePath: "name",
			want: []ValueReference{
				{File: "configmap.yaml", Line: 4, Content: ".Values.name", FullLine: "name: {{ .Values.name }}", ManifestPath: ".metadata.name"},
				{File: "configmap.yaml", Line: 7, Content: ".Values.name", FullLine: "name: {{ .Values.name }}", ManifestPath: ".data.config.name"},
			},
		},
		{
			name: "should find references across multiple files",
			templates: map[string]string{
				"deployment.yaml": `spec:
  replicas: {{ .Values.replicaCount }}`,
				"hpa.yaml": `spec:
  targetCPUUtilizationPercentage: {{ .Values.replicaCount }}`,
			},
			valuePath: "replicaCount",
			want: []ValueReference{
				{File: "deployment.yaml", Line: 2, Content: ".Values.replicaCount", FullLine: "replicas: {{ .Values.replicaCount }}"},
				{File: "hpa.yaml", Line: 2, Content: ".Values.replicaCount", FullLine: "targetCPUUtilizationPercentage: {{ .Values.replicaCount }}"},
			},
		},
		{
			name: "should handle different template formats",
			templates: map[string]string{
				"pod.yaml": `metadata:
  name: {{- .Values.podName }}
  labels:
    app: {{ .Values.appName -}}
  annotations:
    version: {{- .Values.version -}}`,
			},
			valuePath: "podName",
			want: []ValueReference{
				{File: "pod.yaml", Line: 2, Content: ".Values.podName", FullLine: "name: {{- .Values.podName }}", ManifestPath: ".metadata.name"},
			},
		},
		{
			name: "should return empty for no matches",
			templates: map[string]string{
				"service.yaml": `apiVersion: v1
kind: Service
metadata:
  name: my-service`,
			},
			valuePath: "nonexistent",
			want:      nil,
		},
		{
			name:      "should handle empty templates",
			templates: map[string]string{},
			valuePath: "anything",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SearchValueInTemplates(tt.templates, tt.valuePath)

			// Handle non-deterministic map iteration order for multi-file tests
			if tt.name == "should find references across multiple files" {
				if len(got) != len(tt.want) {
					t.Errorf("SearchValueInTemplates() length = %v, want %v", len(got), len(tt.want))
					return
				}

				// Check if we have the expected references regardless of order
				foundDeployment := false
				foundHpa := false
				for _, ref := range got {
					if ref.File == "deployment.yaml" && ref.Line == 2 && ref.Content == ".Values.replicaCount" {
						foundDeployment = true
					}
					if ref.File == "hpa.yaml" && ref.Line == 2 && ref.Content == ".Values.replicaCount" {
						foundHpa = true
					}
				}

				if !foundDeployment || !foundHpa {
					t.Errorf("SearchValueInTemplates() = %v, want both deployment and hpa references", got)
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchValueInTemplates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatValueReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		references []ValueReference
		want       string
	}{
		{
			name: "should format single reference",
			references: []ValueReference{
				{File: "deployment.yaml", Line: 10, Content: ".Values.name"},
			},
			want: "=== deployment.yaml ===\n10L: .Values.name\n",
		},
		{
			name: "should format multiple references in same file",
			references: []ValueReference{
				{File: "service.yaml", Line: 5, Content: ".Values.port"},
				{File: "service.yaml", Line: 8, Content: ".Values.port"},
			},
			want: "=== service.yaml ===\n5L: .Values.port\n8L: .Values.port\n",
		},
		{
			name: "should format references across multiple files",
			references: []ValueReference{
				{File: "deployment.yaml", Line: 10, Content: ".Values.image"},
				{File: "pod.yaml", Line: 5, Content: ".Values.image"},
			},
			want: "=== deployment.yaml ===\n10L: .Values.image\n=== pod.yaml ===\n5L: .Values.image\n",
		},
		{
			name:       "should handle empty references",
			references: []ValueReference{},
			want:       "No references found for the specified value.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FormatValueReferences(tt.references)
			// Since map iteration order is not guaranteed, we need to check both possible orders for multi-file tests
			if tt.name == "should format references across multiple files" {
				alternativeWant := "=== pod.yaml ===\n5L: {{ .Values.image }}\n=== deployment.yaml ===\n10L: {{ .Values.image }}\n"
				if got != tt.want && got != alternativeWant {
					t.Errorf("FormatValueReferences() = %v, want %v or %v", got, tt.want, alternativeWant)
				}
			} else if got != tt.want {
				t.Errorf("FormatValueReferences() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateSearchPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		valuePath string
		testLine  string
		wantMatch bool
	}{
		{
			name:      "should match simple template reference",
			valuePath: "name",
			testLine:  "name: {{ .Values.name }}",
			wantMatch: true,
		},
		{
			name:      "should match nested path",
			valuePath: "database.host",
			testLine:  "host: {{ .Values.database.host }}",
			wantMatch: true,
		},
		{
			name:      "should match with left trim",
			valuePath: "debug",
			testLine:  "debug: {{- .Values.debug }}",
			wantMatch: true,
		},
		{
			name:      "should match with right trim",
			valuePath: "version",
			testLine:  "version: {{ .Values.version -}}",
			wantMatch: true,
		},
		{
			name:      "should match with both trims",
			valuePath: "enabled",
			testLine:  "enabled: {{- .Values.enabled -}}",
			wantMatch: true,
		},
		{
			name:      "should not match partial names",
			valuePath: "name",
			testLine:  "name: {{ .Values.nameOverride }}",
			wantMatch: false,
		},
		{
			name:      "should not match without Values prefix",
			valuePath: "config",
			testLine:  "config: {{ .config }}",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			patterns := createSearchPatterns(tt.valuePath)
			matched := false
			for _, pattern := range patterns {
				if pattern.MatchString(tt.testLine) {
					matched = true
					break
				}
			}

			if matched != tt.wantMatch {
				t.Errorf("createSearchPatterns(%q) match on %q = %v, want %v", tt.valuePath, tt.testLine, matched, tt.wantMatch)
			}
		})
	}
}

func BenchmarkSearchValueInTemplates(b *testing.B) {
	// Create test templates with various sizes
	templates := map[string]string{
		"deployment.yaml": generateLargeTemplate("deployment", 100),
		"service.yaml":    generateLargeTemplate("service", 50),
		"configmap.yaml":  generateLargeTemplate("configmap", 75),
	}
	valuePath := "database.host"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SearchValueInTemplates(templates, valuePath)
	}
}

func generateLargeTemplate(prefix string, lines int) string {
	var content strings.Builder
	content.WriteString(fmt.Sprintf("apiVersion: v1\nkind: %s\n", strings.Title(prefix)))

	for i := 0; i < lines; i++ {
		if i%10 == 0 {
			content.WriteString(fmt.Sprintf("  config%d: {{ .Values.database.host }}\n", i))
		} else {
			content.WriteString(fmt.Sprintf("  field%d: value%d\n", i, i))
		}
	}

	return content.String()
}
