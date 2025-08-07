package helmwrap

import (
	"reflect"
	"sort"
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

func TestGetValueType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		valuesYaml string
		path       string
		want       ValueType
		wantErr    bool
	}{
		{
			name: "should detect string type",
			valuesYaml: `
app:
  name: "myapp"
`,
			path: "app.name",
			want: ValueTypeString,
		},
		{
			name: "should detect int type",
			valuesYaml: `
app:
  replicas: 3
`,
			path: "app.replicas",
			want: ValueTypeInt,
		},
		{
			name: "should detect bool type",
			valuesYaml: `
app:
  enabled: true
`,
			path: "app.enabled",
			want: ValueTypeBool,
		},
		{
			name: "should detect slice type",
			valuesYaml: `
app:
  tags:
    - "tag1"
    - "tag2"
`,
			path: "app.tags",
			want: ValueTypeSlice,
		},
		{
			name: "should detect map type",
			valuesYaml: `
app:
  config:
    debug: true
    timeout: 30
`,
			path: "app.config",
			want: ValueTypeMap,
		},
		{
			name: "should detect float as int type",
			valuesYaml: `
app:
  ratio: 1.5
`,
			path: "app.ratio",
			want: ValueTypeInt,
		},
		{
			name: "should return error for non-existent path",
			valuesYaml: `
app:
  name: "myapp"
`,
			path:    "app.nonexistent",
			wantErr: true,
		},
		{
			name:       "should return error for invalid YAML",
			valuesYaml: "invalid: yaml: content: [",
			path:       "app.name",
			wantErr:    true,
		},
		{
			name: "should handle root level values",
			valuesYaml: `
rootString: "value"
rootInt: 42
rootBool: false
`,
			path: "rootString",
			want: ValueTypeString,
		},
		{
			name: "should handle empty path",
			valuesYaml: `
app:
  name: "myapp"
`,
			path: "",
			want: ValueTypeMap,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetValueType(tt.valuesYaml, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValueType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetValueType() = %v, want %v", got, tt.want)
			}
		})
	}
}
