package configinfra_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	domainconfig "github.com/truewebber/golangcix/internal/domain/config"
	configinfra "github.com/truewebber/golangcix/internal/infrastructure/config"
	"github.com/truewebber/golangcix/internal/infrastructure/remote"
	"go.uber.org/mock/gomock"
)

type stubLogger struct {
	entries []logEntry
}

type logEntry struct {
	level string
	msg   string
	kv    []interface{}
}

func (s *stubLogger) Info(msg string, kv ...interface{}) {
	s.entries = append(s.entries, logEntry{level: "info", msg: msg, kv: append([]interface{}(nil), kv...)})
}

func (s *stubLogger) Warn(msg string, kv ...interface{}) {
	s.entries = append(s.entries, logEntry{level: "warn", msg: msg, kv: append([]interface{}(nil), kv...)})
}

func (s *stubLogger) Error(msg string, kv ...interface{}) {
	s.entries = append(s.entries, logEntry{level: "error", msg: msg, kv: append([]interface{}(nil), kv...)})
}

//nolint:paralleltest // Cannot use t.Parallel() with t.Chdir()
func TestServicePrepare(t *testing.T) {
	const remoteURL = "https://example.com/base.yml"

	remoteDirective := "# " + domainconfig.RemoteDirective + ": " + remoteURL

	tests := []struct {
		name               string
		localContent       string
		remoteData         []byte
		remoteFromCache    bool
		remoteErr          error
		expectRemoteCalled bool
		expectMerged       string
		expectWarnings     []string
		expectInfoLogs     []string
	}{
		{
			name: "no_remote_directive",
			localContent: `linters:
  disable:
    - gosimple
`,
			expectMerged: `linters:
  disable:
    - gosimple
`,
			expectWarnings: []string{"Remote configuration directive not found. Using local configuration only."},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name: "remote_merge_success",
			localContent: remoteDirective + `
linters:
  disable:
    - gofmt
run:
  timeout: 2m
`,
			remoteData: []byte(`linters:
  enable:
    - govet
run:
  timeout: 5m
`),
			expectRemoteCalled: true,
			expectMerged: `linters:
  enable:
    - govet
  disable:
    - gofmt
run:
  timeout: 2m
`,
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name: "remote_fetch_error",
			localContent: remoteDirective + `
linters:
  enable:
    - staticcheck
`,
			remoteErr:          assertiveError("network failure"),
			expectRemoteCalled: true,
			expectMerged: `linters:
  enable:
    - staticcheck
`,
			expectWarnings: []string{"Unable to fetch remote configuration; using local config only"},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name: "remote_from_cache",
			localContent: remoteDirective + `
linters:
  disable:
    - wsl
`,
			remoteData: []byte(`linters:
  enable:
    - govet
`),
			remoteFromCache:    true,
			expectRemoteCalled: true,
			expectMerged: `linters:
  enable:
    - govet
  disable:
    - wsl
`,
			expectWarnings: []string{"Using cached remote configuration"},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name: "remote_invalid_yaml",
			localContent: remoteDirective + `
linters:
  enable:
    - gofmt
`,
			remoteData:         []byte("invalid: ["),
			expectRemoteCalled: true,
			expectMerged: `linters:
  enable:
    - gofmt
`,
			expectWarnings: []string{"Failed to parse remote configuration; using local config only"},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name:         "empty_local_config_file",
			localContent: "",
			expectMerged:  "{}\n",
			expectWarnings: []string{"Remote configuration directive not found. Using local configuration only."},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
		{
			name: "generation_in_subdirectory",
			localContent: `linters:
  enable:
    - govet
`,
			expectMerged: `linters:
  enable:
    - govet
`,
			expectWarnings: []string{"Remote configuration directive not found. Using local configuration only."},
			expectInfoLogs: []string{"Removed old generated config", "Generated configuration file"},
		},
	}

	negativeTests := []struct {
		name          string
		setup         func(string) error
		localPath     string
		expectErr     bool
		errContains   string
		remoteErr     error
		remoteCalled  bool
	}{
		{
			name:      "file_not_exists",
			localPath: "nonexistent.yml",
			expectErr: true,
			errContains: "read local configuration",
		},
		{
			name: "invalid_yaml_in_local_config",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "invalid.yml"), []byte("invalid: ["), 0o600)
			},
			localPath:   "invalid.yml",
			expectErr:   true,
			errContains: "parse local configuration",
		},
		{
			name: "context_canceled",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "config.yml"), []byte(remoteDirective+"\nlinters:\n  enable: [govet]"), 0o600)
			},
			localPath:    "config.yml",
			expectErr:    false, // Context cancellation handled gracefully
			remoteCalled: true,
			remoteErr:    context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}

			t.Chdir(tempDir)

			defer t.Chdir(cwd)

			localPath := tt.name + ".yml"
			if tt.name == "generation_in_subdirectory" {
				subDir := filepath.Join(tempDir, "subdir")
				if err := os.MkdirAll(subDir, 0o750); err != nil {
					t.Fatalf("create subdir: %v", err)
				}

				localPath = filepath.Join("subdir", "config.yml")
			}

			if err := os.WriteFile(localPath, []byte(tt.localContent), 0o600); err != nil {
				t.Fatalf("write local config: %v", err)
			}

			// Create stray generated config to ensure cleanup removes it.
			const strayDir = "stray"
			if err := os.Mkdir(strayDir, 0o750); err != nil {
				t.Fatalf("create stray dir: %v", err)
			}

			strayPath := filepath.Join(strayDir, domainconfig.GeneratedFileName)
			if err := os.WriteFile(strayPath, []byte("stale"), 0o600); err != nil {
				t.Fatalf("write stray file: %v", err)
			}

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fetcher := remote.NewMockRemoteFetcher(ctrl)

			if tt.expectRemoteCalled {
				fetcher.EXPECT().
					Fetch(gomock.Any(), gomock.AssignableToTypeOf(&url.URL{})).
					DoAndReturn(func(_ context.Context, u *url.URL) (domainconfig.FetchResult, error) {
						if got := u.String(); got != remoteURL {
							t.Fatalf("remote called with %s, want %s", got, remoteURL)
						}

						if tt.remoteErr != nil {
							return domainconfig.FetchResult{}, tt.remoteErr
						}

						return domainconfig.FetchResult{
							Data:      tt.remoteData,
							FromCache: tt.remoteFromCache,
						}, nil
					})
			} else {
				fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Times(0)
			}

			svc := configinfra.NewService(logger, fetcher)

			generatedPath, err := svc.Prepare(context.Background(), localPath)
			if err != nil {
				t.Fatalf("Prepare returned error: %v", err)
			}

			expectedGenerated := domainconfig.GeneratedPath(localPath)
			if generatedPath != expectedGenerated {
				t.Fatalf("generated path = %s, want %s", generatedPath, expectedGenerated)
			}

			// Read generated file and verify merged content ignores header.
			//nolint:gosec // G304: generatedPath is controlled by the test
			content, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("read generated: %v", err)
			}

			body := extractBody(string(content))

			normalized, err := domainconfig.NormalizeYAML([]byte(body))
			if err != nil {
				t.Fatalf("normalize generated yaml: %v", err)
			}

			wantNormalized, err := domainconfig.NormalizeYAML([]byte(tt.expectMerged))
			if err != nil {
				t.Fatalf("normalize expected yaml: %v", err)
			}

			// For empty config, normalize to empty map
			if tt.name == "empty_local_config_file" {
				if normalized == nil {
					normalized = map[string]interface{}{}
				}

				if wantNormalized == nil {
					wantNormalized = map[string]interface{}{}
				}
			}

			if !reflect.DeepEqual(normalized, wantNormalized) {
				t.Fatalf("generated config mismatch\n\tgot:  %s\n\twant: %s", body, tt.expectMerged)
			}

			// Stray file should be removed.
			if _, err := os.Stat(strayPath); !os.IsNotExist(err) {
				t.Fatalf("stray generated file was not removed")
			}

			// Check warning messages, if expected.
			var (
				warnings []string
				infoLogs []string
			)

			const (
				logLevelWarn = "warn"
				logLevelInfo = "info"
			)

			for _, entry := range logger.entries {
				switch entry.level {
				case logLevelWarn:
					warnings = append(warnings, entry.msg)
				case logLevelInfo:
					infoLogs = append(infoLogs, entry.msg)
				}
			}

			if !equalStringSlices(tt.expectWarnings, warnings) {
				t.Fatalf("warnings = %v, want %v", warnings, tt.expectWarnings)
			}

			if !equalStringSlices(tt.expectInfoLogs, infoLogs) {
				t.Fatalf("info logs = %v, want %v", infoLogs, tt.expectInfoLogs)
			}
		})
	}

	for _, tt := range negativeTests {
		t.Run("negative_"+tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}

			t.Chdir(tempDir)

			defer t.Chdir(cwd)

			if tt.setup != nil {
				if err := tt.setup(tempDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fetcher := remote.NewMockRemoteFetcher(ctrl)

			if tt.remoteCalled {
				fetcher.EXPECT().
					Fetch(gomock.Any(), gomock.AssignableToTypeOf(&url.URL{})).
					Return(domainconfig.FetchResult{}, tt.remoteErr)
			} else {
				fetcher.EXPECT().Fetch(gomock.Any(), gomock.Any()).Times(0)
			}

			svc := configinfra.NewService(logger, fetcher)

			_, err = svc.Prepare(context.Background(), tt.localPath)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("Prepare() expected error, got nil")
				}

				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("Prepare() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			if err != nil && tt.remoteErr == nil {
				t.Fatalf("Prepare() unexpected error: %v", err)
			}
		})
	}
}

func extractBody(content string) string {
	parts := strings.SplitN(content, "\n\n", 2)
	if len(parts) == 2 {
		return parts[1]
	}

	return content
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

type assertiveError string

func (e assertiveError) Error() string { return string(e) }
