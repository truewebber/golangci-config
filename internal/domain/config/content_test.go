package config_test

import (
	"testing"

	"github.com/truewebber/golangci-config/internal/domain/config"
)

func TestHasContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name: "yaml_with_content",
			input: `
linters:
  enable:
    - govet
`,
			want: true,
		},
		{
			name:  "empty_yaml_document",
			input: "---\n",
			want:  false,
		},
		{
			name:  "yaml_only_whitespace",
			input: "   \n\t  \n  ",
			want:  false,
		},
		{
			name:  "invalid_yaml_returns_true",
			input: "foo: [unclosed",
			want:  true,
		},
		{
			name: "yaml_with_null",
			input: `
key: null
`,
			want: true,
		},
		{
			name: "yaml_with_empty_map",
			input: `
linters: {}
`,
			want: true,
		},
		{
			name: "yaml_with_empty_array",
			input: `
linters:
  enable: []
`,
			want: true,
		},
		{
			name: "yaml_string_with_whitespace",
			input: `
key: "   "
`,
			want: true,
		},
		{
			name:  "empty_file",
			input: "",
			want:  false,
		},
		{
			name:  "only_newlines",
			input: "\n\n\n",
			want:  false,
		},
		{
			name: "yaml_with_comments_only",
			input: `
# Comment 1
# Comment 2
`,
			want: false,
		},
		{
			name: "yaml_with_content_and_comments",
			input: `
# Comment
linters:
  enable: [govet]
`,
			want: true,
		},
		{
			name: "yaml_with_nested_content",
			input: `
linters:
  enable:
    - govet
  config:
    maxIssuesPerFile: 10
`,
			want: true,
		},
		{
			name: "yaml_with_multiple_keys",
			input: `
linters:
  enable: [govet]
run:
  timeout: 5m
`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := config.HasContent([]byte(tt.input))
			if got != tt.want {
				t.Fatalf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

