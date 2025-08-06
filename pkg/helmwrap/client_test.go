package helmwrap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestHelmClient_ReadAllTemplateFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupFunc     func(t *testing.T) (string, string, func())
		wantFiles     map[string]string
		wantErr       bool
		wantErrString string
	}{
		{
			name: "should read all template files successfully",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				chartName := "test-chart"
				chartDir := filepath.Join(tempDir, chartName)
				templatesDir := filepath.Join(chartDir, "templates")

				if err := os.MkdirAll(templatesDir, 0755); err != nil {
					t.Fatalf("failed to create templates directory: %v", err)
				}

				// Create test template files
				deploymentContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  replicas: {{ .Values.replicaCount }}`

				serviceContent := `apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}-service
spec:
  type: {{ .Values.service.type }}`

				if err := os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte(deploymentContent), 0644); err != nil {
					t.Fatalf("failed to write deployment.yaml: %v", err)
				}

				if err := os.WriteFile(filepath.Join(templatesDir, "service.yml"), []byte(serviceContent), 0644); err != nil {
					t.Fatalf("failed to write service.yml: %v", err)
				}

				// Create a non-template file (should be ignored)
				if err := os.WriteFile(filepath.Join(templatesDir, "README.md"), []byte("# Chart Templates"), 0644); err != nil {
					t.Fatalf("failed to write README.md: %v", err)
				}

				return tempDir, chartName, func() {}
			},
			wantFiles: map[string]string{
				"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
spec:
  replicas: {{ .Values.replicaCount }}`,
				"service.yml": `apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.name }}-service
spec:
  type: {{ .Values.service.type }}`,
			},
			wantErr: false,
		},
		{
			name: "should read nested template files",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				chartName := "nested-chart"
				chartDir := filepath.Join(tempDir, chartName)
				templatesDir := filepath.Join(chartDir, "templates")
				nestedDir := filepath.Join(templatesDir, "nested")

				if err := os.MkdirAll(nestedDir, 0755); err != nil {
					t.Fatalf("failed to create nested directory: %v", err)
				}

				configContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  key: {{ .Values.config.key }}`

				if err := os.WriteFile(filepath.Join(nestedDir, "configmap.yaml"), []byte(configContent), 0644); err != nil {
					t.Fatalf("failed to write nested configmap.yaml: %v", err)
				}

				return tempDir, chartName, func() {}
			},
			wantFiles: map[string]string{
				"nested/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
data:
  key: {{ .Values.config.key }}`,
			},
			wantErr: false,
		},
		{
			name: "should return error when templates directory does not exist",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				chartName := "no-templates-chart"
				// Don't create templates directory

				return tempDir, chartName, func() {}
			},
			wantFiles:     nil,
			wantErr:       true,
			wantErrString: "templates directory not found",
		},
		{
			name: "should handle empty templates directory",
			setupFunc: func(t *testing.T) (string, string, func()) {
				tempDir := t.TempDir()
				chartName := "empty-chart"
				chartDir := filepath.Join(tempDir, chartName)
				templatesDir := filepath.Join(chartDir, "templates")

				if err := os.MkdirAll(templatesDir, 0755); err != nil {
					t.Fatalf("failed to create empty templates directory: %v", err)
				}

				return tempDir, chartName, func() {}
			},
			wantFiles: map[string]string{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chartDir, chartName, cleanup := tt.setupFunc(t)
			defer cleanup()

			client := &helmClient{}
			got, err := client.ReadAllTemplateFiles(chartDir, chartName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadAllTemplateFiles() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.wantErrString != "" && !containsString(err.Error(), tt.wantErrString) {
					t.Errorf("ReadAllTemplateFiles() error = %v, wantErrString %v", err, tt.wantErrString)
				}
				return
			}

			if err != nil {
				t.Errorf("ReadAllTemplateFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.wantFiles) {
				t.Errorf("ReadAllTemplateFiles() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}

func TestHelmClient_ReadAllTemplateFiles_Integration(t *testing.T) {
	t.Parallel()

	// Create a realistic test chart structure
	tempDir := t.TempDir()
	chartName := "integration-chart"
	chartDir := filepath.Join(tempDir, chartName)
	templatesDir := filepath.Join(chartDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates directory: %v", err)
	}

	// Create multiple template files with realistic content
	templates := map[string]string{
		"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "chart.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "chart.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        ports:
        - name: http
          containerPort: {{ .Values.service.port }}`,
		"service.yaml": `apiVersion: v1
kind: Service
metadata:
  name: {{ include "chart.fullname" . }}
  labels:
    {{- include "chart.labels" . | nindent 4 }}
spec:
  type: {{ .Values.service.type }}
  ports:
  - port: {{ .Values.service.port }}
    targetPort: http
    protocol: TCP
    name: http
  selector:
    {{- include "chart.selectorLabels" . | nindent 4 }}`,
		"helpers/_helpers.tpl": `{{- define "chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}`,
	}

	for fileName, content := range templates {
		filePath := filepath.Join(templatesDir, fileName)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", fileName, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", fileName, err)
		}
	}

	client := &helmClient{}
	got, err := client.ReadAllTemplateFiles(tempDir, chartName)
	if err != nil {
		t.Fatalf("ReadAllTemplateFiles() error = %v", err)
	}

	// Verify we got all expected files (except .tpl files which should be ignored)
	expectedFiles := []string{"deployment.yaml", "service.yaml"}
	var gotFiles []string
	for fileName := range got {
		gotFiles = append(gotFiles, fileName)
	}
	sort.Strings(gotFiles)
	sort.Strings(expectedFiles)

	if !reflect.DeepEqual(gotFiles, expectedFiles) {
		t.Errorf("ReadAllTemplateFiles() files = %v, want %v", gotFiles, expectedFiles)
	}

	// Verify content of one file
	if deploymentContent, exists := got["deployment.yaml"]; !exists {
		t.Error("deployment.yaml not found in results")
	} else if !containsString(deploymentContent, ".Values.replicaCount") {
		t.Error("deployment.yaml content doesn't contain expected value reference")
	}
}

func BenchmarkHelmClient_ReadAllTemplateFiles(b *testing.B) {
	// Setup test data
	tempDir := b.TempDir()
	chartName := "benchmark-chart"
	chartDir := filepath.Join(tempDir, chartName)
	templatesDir := filepath.Join(chartDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		b.Fatalf("failed to create templates directory: %v", err)
	}

	// Create many template files
	for i := 0; i < 50; i++ {
		content := generateLargeTemplate("resource", 100)
		fileName := filepath.Join(templatesDir, fmt.Sprintf("template%d.yaml", i))
		if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
			b.Fatalf("failed to write template file: %v", err)
		}
	}

	client := &helmClient{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.ReadAllTemplateFiles(tempDir, chartName)
		if err != nil {
			b.Fatalf("ReadAllTemplateFiles() error = %v", err)
		}
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr)+1 && containsSubstring(s[1:len(s)-1], substr)))
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
