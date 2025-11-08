package config_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/truewebber/golangcix/internal/domain/config"
)

func TestExtractRemoteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantURL   string
		wantErr   bool
		errCheck  func(error) bool
	}{
		{
			name:    "directive_at_start",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml\nlinters:\n  enable:\n    - govet",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "directive_in_middle",
			input:   "linters:\n  enable:\n    - govet\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml\nrun:\n  timeout: 5m",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "directive_at_end",
			input:   "linters:\n  enable:\n    - govet\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "comment_hash",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "comment_double_slash",
			input:   "// GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "url_http",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: http://example.com/config.yml",
			wantURL: "http://example.com/config.yml",
		},
		{
			name:    "url_https",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "url_with_params",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml?version=1&token=abc",
			wantURL: "https://example.com/config.yml?token=abc&version=1",
		},
		{
			name:    "directive_case_insensitive",
			input:   "# golangci_lint_remote_config: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "spaces_around_colon_not_supported",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG : https://example.com/config.yml",
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, config.ErrNoURLFound)
			},
		},
		{
			name:    "multiple_directives_first_found",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://first.com/config.yml\n# GOLANGCI_LINT_REMOTE_CONFIG: https://second.com/config.yml",
			wantURL: "https://first.com/config.yml",
		},
		{
			name:    "directive_after_code",
			input:   "linters:\n  enable: [govet]\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "empty_file",
			input:   "",
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, config.ErrNoURLFound)
			},
		},
		{
			name:    "no_directive",
			input:   "linters:\n  enable:\n    - govet",
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, config.ErrNoURLFound)
			},
		},
		{
			name:    "directive_no_url",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG:",
			wantErr: true,
		},
		{
			name:    "invalid_url",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: not-a-valid-url",
			wantErr: true,
		},
		{
			name:    "url_with_spaces_truncated",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config with spaces.yml",
			wantURL: "https://example.com/config",
		},
		{
			name:    "very_long_url",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/" + string(make([]byte, 1000)) + ".yml",
			wantErr: true,
		},
		{
			name:    "url_with_special_chars",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config%20file.yml",
			wantURL: "https://example.com/config%20file.yml",
		},
		{
			name:    "directive_with_tabs",
			input:   "#\tGOLANGCI_LINT_REMOTE_CONFIG:\thttps://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "multiple_spaces",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG:    https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "directive_uppercase",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: HTTPS://EXAMPLE.COM/CONFIG.YML",
			wantURL: "https://example.com/CONFIG.YML",
		},
		{
			name:    "url_with_port",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com:8080/config.yml",
			wantURL: "https://example.com:8080/config.yml",
		},
		{
			name:    "url_with_path",
			input:   "# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/path/to/config.yml",
			wantURL: "https://example.com/path/to/config.yml",
		},
		{
			name:    "non_comment_before_directive",
			input:   "linters:\n  enable: [govet]\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "empty_line_before_directive",
			input:   "\n\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
		{
			name:    "whitespace_only_lines",
			input:   "   \n\t\n# GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml",
			wantURL: "https://example.com/config.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ExtractRemoteURL([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExtractRemoteURL() expected error, got nil")
				}

				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Fatalf("ExtractRemoteURL() error = %v, want specific error", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("ExtractRemoteURL() unexpected error: %v", err)
			}

			if got == nil {
				t.Fatalf("ExtractRemoteURL() returned nil URL")
			}

			gotStr := got.String()

			// Verify it's a valid URL
			gotURL, parseErr := url.Parse(gotStr)
			if parseErr != nil {
				t.Fatalf("ExtractRemoteURL() returned invalid URL: %v", parseErr)
			}

			// For URLs with params, order might differ after normalization
			if tt.name == "url_with_params" {
				wantURL, parseErr := url.Parse(tt.wantURL)
				if parseErr != nil {
					t.Fatalf("test setup: invalid wantURL: %v", parseErr)
				}

				if gotURL.Scheme != wantURL.Scheme || gotURL.Host != wantURL.Host || gotURL.Path != wantURL.Path {
					t.Fatalf("ExtractRemoteURL() URL = %q, want similar to %q", gotStr, tt.wantURL)
				}

				// Check that params are present (order may differ)
				if len(gotURL.Query()) != len(wantURL.Query()) {
					t.Fatalf("ExtractRemoteURL() URL params count = %d, want %d", len(gotURL.Query()), len(wantURL.Query()))
				}

				return
			}

			if gotStr != tt.wantURL {
				t.Fatalf("ExtractRemoteURL() URL = %q, want %q", gotStr, tt.wantURL)
			}
		})
	}
}

