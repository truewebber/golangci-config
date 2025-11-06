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
