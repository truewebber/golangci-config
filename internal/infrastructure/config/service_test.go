package configinfra

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	domainconfig "github.com/truewebber/golangci-config/internal/domain/config"
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

type stubFetcher struct {
	data      []byte
	fromCache bool
	err       error

	calls []string
}

func (s *stubFetcher) Fetch(url string) ([]byte, bool, error) {
	s.calls = append(s.calls, url)
	return s.data, s.fromCache, s.err
}

func TestServicePrepare(t *testing.T) {
	remoteDirective := "# " + domainconfig.RemoteDirective + ": https://example.com/base.yml"

	tests := []struct {
		name               string
		localContent       string
		remoteData         []byte
		remoteFromCache    bool
		remoteErr          error
		expectRemoteCalled bool
		expectMerged       string
		expectWarnings     []string
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
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working directory: %v", err)
			}
			if err := os.Chdir(tempDir); err != nil {
				t.Fatalf("chdir to temp dir: %v", err)
			}
			defer func() {
				_ = os.Chdir(cwd)
			}()

			localPath := "config.yml"
			if err := os.WriteFile(localPath, []byte(tt.localContent), 0o644); err != nil {
				t.Fatalf("write local config: %v", err)
			}

			// Create stray generated config to ensure cleanup removes it.
			strayDir := filepath.Join("stray")
			if err := os.Mkdir(strayDir, 0o755); err != nil {
				t.Fatalf("create stray dir: %v", err)
			}
			strayPath := filepath.Join(strayDir, domainconfig.GeneratedFileName)
			if err := os.WriteFile(strayPath, []byte("stale"), 0o644); err != nil {
				t.Fatalf("write stray file: %v", err)
			}

			logger := &stubLogger{}
			fetcher := &stubFetcher{
				data:      tt.remoteData,
				fromCache: tt.remoteFromCache,
				err:       tt.remoteErr,
			}

			svc := NewService(logger, fetcher)

			generatedPath, err := svc.Prepare(localPath)
			if err != nil {
				t.Fatalf("Prepare returned error: %v", err)
			}

			expectedGenerated := domainconfig.GeneratedPath(localPath)
			if generatedPath != expectedGenerated {
				t.Fatalf("generated path = %s, want %s", generatedPath, expectedGenerated)
			}

			// Ensure remote fetcher usage matches expectation.
			if tt.expectRemoteCalled != (len(fetcher.calls) > 0) {
				t.Fatalf("remote called = %v, want %v", len(fetcher.calls) > 0, tt.expectRemoteCalled)
			}

			if tt.expectRemoteCalled && fetcher.calls[0] != "https://example.com/base.yml" {
				t.Fatalf("remote called with %s, want https://example.com/base.yml", fetcher.calls[0])
			}

			// Read generated file and verify merged content ignores header.
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

			if !reflect.DeepEqual(normalized, wantNormalized) {
				t.Fatalf("generated config mismatch\n\tgot:  %s\n\twant: %s", body, tt.expectMerged)
			}

			// Stray file should be removed.
			if _, err := os.Stat(strayPath); !os.IsNotExist(err) {
				t.Fatalf("stray generated file was not removed")
			}

			// Check warning messages, if expected.
			var warnings []string
			for _, entry := range logger.entries {
				if entry.level == "warn" {
					warnings = append(warnings, entry.msg)
				}
			}

			if !equalStringSlices(tt.expectWarnings, warnings) {
				t.Fatalf("warnings = %v, want %v", warnings, tt.expectWarnings)
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
