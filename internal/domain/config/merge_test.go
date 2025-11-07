package config_test

import (
	"reflect"
	"testing"

	"github.com/truewebber/golangci-config/internal/domain/config"
)

func TestNormalizeYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    interface{}
		wantErr bool
	}{
		{
			name: "map_with_various_keys",
			input: `
foo: bar
1: numeric-key
list:
  - item1
  - item2
`,
			want: map[string]interface{}{
				"foo": "bar",
				"1":   "numeric-key",
				"list": []interface{}{
					"item1",
					"item2",
				},
			},
		},
		{
			name:    "empty_document",
			input:   "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "invalid_yaml",
			input:   "foo: [unclosed",
			wantErr: true,
		},
		{
			name: "nested_map_interface_keys",
			input: `
map:
  1: numeric-key
  true: boolean-key
  null: null-key
  "string": string-key
`,
			want: map[string]interface{}{
				"map": map[string]interface{}{
					"1":      "numeric-key",
					"true":   "boolean-key",
					"<nil>":  "null-key",
					"string": "string-key",
				},
			},
		},
		{
			name: "mixed_types_in_array",
			input: `
list:
  - string
  - 42
  - 3.14
  - true
  - false
  - null
  - [nested, array]
  - {key: value}
`,
			want: map[string]interface{}{
				"list": []interface{}{
					"string",
					42,
					3.14,
					true,
					false,
					nil,
					[]interface{}{"nested", "array"},
					map[string]interface{}{"key": "value"},
				},
			},
		},
		{
			name: "numbers_int_float",
			input: `
int: 42
float: 3.14
negative_int: -10
negative_float: -2.5
`,
			want: map[string]interface{}{
				"int":          42,
				"float":       3.14,
				"negative_int": -10,
				"negative_float": -2.5,
			},
		},
		{
			name: "boolean_values",
			input: `
enabled: true
disabled: false
`,
			want: map[string]interface{}{
				"enabled":  true,
				"disabled": false,
			},
		},
		{
			name: "null_values",
			input: `
null_value: null
empty: ~
`,
			want: map[string]interface{}{
				"null_value": nil,
				"empty":      nil,
			},
		},
		{
			name: "deep_nesting",
			input: `
level1:
  level2:
    level3:
      level4:
        value: deep
`,
			want: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"level4": map[string]interface{}{
								"value": "deep",
							},
						},
					},
				},
			},
		},
		{
			name:    "only_whitespace",
			input:   "   \n\t  \n  ",
			wantErr: true,
		},
		{
			name: "yaml_with_comments",
			input: `
# This is a comment
key: value
# Another comment
`,
			want: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "incomplete_yaml_document",
			input: `
linters:
  enable:
    - govet
  disable:
`,
			wantErr: false,
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable":  []interface{}{"govet"},
					"disable": nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.NormalizeYAML([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeYAML() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     interface{}
		override interface{}
		want     interface{}
	}{
		{
			name: "merge_nested_maps",
			base: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
					"config": map[string]interface{}{
						"maxIssuesPerFile": 10,
					},
				},
				"run": map[string]interface{}{
					"tests": false,
				},
			},
			override: map[string]interface{}{
				"linters": map[string]interface{}{
					"disable": []interface{}{"gofmt"},
					"config": map[string]interface{}{
						"maxIssuesPerFile": 5,
					},
				},
				"run": map[string]interface{}{
					"tests": true,
				},
			},
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable":  []interface{}{"govet"},
					"disable": []interface{}{"gofmt"},
					"config": map[string]interface{}{
						"maxIssuesPerFile": 5,
					},
				},
				"run": map[string]interface{}{
					"tests": true,
				},
			},
		},
		{
			name: "override_list_replaces_base",
			base: map[string]interface{}{
				"issues": map[string]interface{}{
					"exclude": []interface{}{"deprecated"},
				},
			},
			override: map[string]interface{}{
				"issues": map[string]interface{}{
					"exclude": []interface{}{"legacy", "generated"},
				},
			},
			want: map[string]interface{}{
				"issues": map[string]interface{}{
					"exclude": []interface{}{"legacy", "generated"},
				},
			},
		},
		{
			name: "override_scalar",
			base: map[string]interface{}{
				"run": map[string]interface{}{
					"timeout": "5m",
				},
			},
			override: map[string]interface{}{
				"run": map[string]interface{}{
					"timeout": "2m",
				},
			},
			want: map[string]interface{}{
				"run": map[string]interface{}{
					"timeout": "2m",
				},
			},
		},
		{
			name: "override_replaces_different_type",
			base: map[string]interface{}{
				"linters": "legacy-string",
			},
			override: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
		},
		{
			name: "base_nil_override_map",
			base: nil,
			override: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
		},
		{
			name: "base_map_override_nil",
			base: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
			override: nil,
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
		},
		{
			name: "base_nil_override_nil",
			base: nil,
			override: nil,
			want: map[string]interface{}{},
		},
		{
			name: "deep_nesting_3_levels",
			base: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"value": "base",
						},
					},
				},
			},
			override: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"newValue": "override",
						},
					},
				},
			},
			want: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"value":    "base",
							"newValue": "override",
						},
					},
				},
			},
		},
		{
			name: "array_with_different_types",
			base: map[string]interface{}{
				"items": []interface{}{
					"string",
					42,
					true,
				},
			},
			override: map[string]interface{}{
				"items": []interface{}{
					"new",
					100,
					false,
				},
			},
			want: map[string]interface{}{
				"items": []interface{}{
					"new",
					100,
					false,
				},
			},
		},
		{
			name: "map_contains_array_array_contains_map",
			base: map[string]interface{}{
				"config": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
			override: map[string]interface{}{
				"config": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"newKey": "newValue",
						},
					},
				},
			},
			want: map[string]interface{}{
				"config": map[string]interface{}{
					"items": []interface{}{
						map[string]interface{}{
							"newKey": "newValue",
						},
					},
				},
			},
		},
		{
			name: "base_empty_map_override_map",
			base: map[string]interface{}{},
			override: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
		},
		{
			name: "base_map_override_empty_map",
			base: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
			override: map[string]interface{}{},
			want: map[string]interface{}{
				"linters": map[string]interface{}{
					"enable": []interface{}{"govet"},
				},
			},
		},
		{
			name: "base_array_override_map",
			base: map[string]interface{}{
				"items": []interface{}{"item1", "item2"},
			},
			override: map[string]interface{}{
				"items": map[string]interface{}{
					"key": "value",
				},
			},
			want: map[string]interface{}{
				"items": map[string]interface{}{
					"key": "value",
				},
			},
		},
		{
			name: "base_map_override_array",
			base: map[string]interface{}{
				"items": map[string]interface{}{
					"key": "value",
				},
			},
			override: map[string]interface{}{
				"items": []interface{}{"item1", "item2"},
			},
			want: map[string]interface{}{
				"items": []interface{}{"item1", "item2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseSnapshot := config.DeepCopy(tt.base)

			got := config.Merge(tt.base, tt.override)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Merge() = %#v, want %#v", got, tt.want)
			}

			if !reflect.DeepEqual(tt.base, baseSnapshot) {
				t.Fatalf("Merge() modified base. got=%#v, want=%#v", tt.base, baseSnapshot)
			}
		})
	}
}

func TestDeepCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		input interface{}
	}{
		{
			name: "copy_map",
			input: map[string]interface{}{
				"key": "value",
			},
		},
		{
			name: "copy_array",
			input: []interface{}{"item1", "item2", "item3"},
		},
		{
			name: "copy_scalar_string",
			input: "scalar",
		},
		{
			name: "copy_scalar_int",
			input: 42,
		},
		{
			name: "copy_scalar_bool",
			input: true,
		},
		{
			name: "copy_nested_structure",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": []interface{}{
						"item1",
						map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		},
		{
			name: "copy_nil",
			input: nil,
		},
		{
			name: "copy_empty_map",
			input: map[string]interface{}{},
		},
		{
			name: "copy_empty_array",
			input: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := config.DeepCopy(tt.input)

			if !reflect.DeepEqual(got, tt.input) {
				t.Fatalf("DeepCopy() = %#v, want %#v", got, tt.input)
			}

			// Verify that modifying the copy doesn't affect the original
			if tt.input == nil {
				return
			}

			const modifiedValue = "modified"

			switch v := got.(type) {
			case map[string]interface{}:
				original, ok := tt.input.(map[string]interface{})
				if !ok {
					return
				}

				v[modifiedValue] = true

				if _, exists := original[modifiedValue]; exists {
					t.Fatalf("DeepCopy() modification of copy affected original")
				}

			case []interface{}:
				original, ok := tt.input.([]interface{})
				if !ok || len(v) == 0 {
					return
				}

				originalFirst := original[0]
				v[0] = modifiedValue

				if original[0] == modifiedValue || original[0] != originalFirst {
					t.Fatalf("DeepCopy() modification of copy affected original")
				}
			}
		})
	}
}
