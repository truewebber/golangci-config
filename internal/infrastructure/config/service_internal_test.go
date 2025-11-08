package configinfra_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domainconfig "github.com/truewebber/golangcix/internal/domain/config"
	configinfra "github.com/truewebber/golangcix/internal/infrastructure/config"
	"github.com/truewebber/golangcix/internal/infrastructure/remote"
	"go.uber.org/mock/gomock"
)

//nolint:paralleltest // Cannot use t.Parallel() with t.Chdir()
func TestServiceHandleRemoteConfigEdgeCases(t *testing.T) {
	const remoteURL = "https://example.com/config.yml"

	remoteDirective := "# " + domainconfig.RemoteDirective + ": " + remoteURL

	tests := []struct {
		name               string
		localContent       string
		remoteData         []byte
		remoteErr          error
		expectRemoteCalled bool
		expectWarnings     []string
		expectMerged       string
	}{
		{
			name: "extract_url_error_not_no_url_found",
			localContent: "# " + domainconfig.RemoteDirective + ": invalid url\nlinters:\n  enable: [govet]",
			expectRemoteCalled: false,
			expectWarnings:     []string{"failed to extract remote URL"},
			expectMerged:       "linters:\n  enable:\n    - govet\n",
		},
		{
			name: "empty_remote_document",
			localContent: remoteDirective + "\nlinters:\n  enable: [govet]",
			remoteData:         []byte(""),
			expectRemoteCalled: true,
			expectMerged:        "linters:\n  enable:\n    - govet\n",
		},
		{
			name: "very_large_remote_document",
			localContent: remoteDirective + "\nlinters:\n  enable: [govet]",
			remoteData:         []byte("linters:\n  enable:\n    - " + strings.Repeat("verylonglintername", 100)),
			expectRemoteCalled: true,
			expectMerged:        "linters:\n  enable:\n    - " + strings.Repeat("verylonglintername", 100) + "\n",
		},
		{
			name: "context_canceled_during_fetch",
			localContent: remoteDirective + "\nlinters:\n  enable: [govet]",
			remoteErr:          context.Canceled,
			expectRemoteCalled: true,
			expectWarnings:     []string{"Unable to fetch remote configuration"},
			expectMerged:        "linters:\n  enable:\n    - govet\n",
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
			if err := os.WriteFile(localPath, []byte(tt.localContent), 0o600); err != nil {
				t.Fatalf("write local config: %v", err)
			}

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fetcher := remote.NewMockRemoteFetcher(ctrl)

			if tt.expectRemoteCalled {
				fetcher.EXPECT().
					Fetch(gomock.Any(), gomock.AssignableToTypeOf(&url.URL{})).
					DoAndReturn(func(_ context.Context, _ *url.URL) (domainconfig.FetchResult, error) {
						if tt.remoteErr != nil {
							return domainconfig.FetchResult{}, tt.remoteErr
						}

						return domainconfig.FetchResult{
							Data:      tt.remoteData,
							FromCache: false,
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

			// Verify generated file exists
			if _, err := os.Stat(generatedPath); os.IsNotExist(err) {
				t.Fatalf("generated file does not exist: %s", generatedPath)
			}

			// Check warning messages
			var warnings []string

			for _, entry := range logger.entries {
				if entry.level == "warn" {
					warnings = append(warnings, entry.msg)
				}
			}

			if len(tt.expectWarnings) > 0 {
				for _, expected := range tt.expectWarnings {
					found := false

					for _, got := range warnings {
						if contains(got, expected) {
							found = true

							break
						}
					}

					if !found {
						t.Fatalf("missing expected warning %q, got %v", expected, warnings)
					}
				}
			}

			// Verify merged content (simplified check)
			//nolint:gosec // G304: generatedPath is controlled by the test
			content, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("read generated: %v", err)
			}

			body := extractBody(string(content))
			if !strings.Contains(body, "govet") {
				t.Fatalf("generated config should contain 'govet', got: %s", body)
			}
		})
	}
}

//nolint:paralleltest // Cannot use t.Parallel() with t.Chdir()
func TestServiceCleanupGeneratedFilesEdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(string) error
		currentPath     string
		expectRemoved   []string
		expectRemaining []string
		expectInfoLogs  int
	}{
		{
			name: "remove_one_old_file",
			setup: func(dir string) error {
				// Create old file in subdirectory
				subDir := filepath.Join(dir, "olddir")

				if err := os.MkdirAll(subDir, 0o750); err != nil {
					return fmt.Errorf("mkdir all: %w", err)
				}

				oldPath := filepath.Join(subDir, domainconfig.GeneratedFileName)

				return os.WriteFile(oldPath, []byte("old"), 0o600)
			},
			currentPath:     "current.yml",
			expectRemoved:   []string{filepath.Join("olddir", domainconfig.GeneratedFileName)},
			expectRemaining: []string{},
			expectInfoLogs:  1,
		},
		{
			name: "remove_multiple_old_files_in_different_dirs",
			setup: func(dir string) error {
				dir1 := filepath.Join(dir, "dir1")

				if err := os.MkdirAll(dir1, 0o750); err != nil {
					return fmt.Errorf("mkdir all dir1: %w", err)
				}

				dir2 := filepath.Join(dir, "dir2")

				if err := os.MkdirAll(dir2, 0o750); err != nil {
					return fmt.Errorf("mkdir all dir2: %w", err)
				}

				old1 := filepath.Join(dir1, domainconfig.GeneratedFileName)

				if err := os.WriteFile(old1, []byte("old1"), 0o600); err != nil {
					return fmt.Errorf("write file old1: %w", err)
				}

				old2 := filepath.Join(dir2, domainconfig.GeneratedFileName)

				return os.WriteFile(old2, []byte("old2"), 0o600)
			},
			currentPath:     filepath.Join("dir3", domainconfig.GeneratedFileName),
			expectRemoved:   []string{filepath.Join("dir1", domainconfig.GeneratedFileName), filepath.Join("dir2", domainconfig.GeneratedFileName)},
			expectRemaining: []string{},
			expectInfoLogs:  2,
		},
		{
			name: "current_file_not_removed",
			setup: func(dir string) error {
				// Create an old file in a different subdirectory
				oldDir := filepath.Join(dir, "olddir")

				if err := os.MkdirAll(oldDir, 0o750); err != nil {
					return fmt.Errorf("mkdir all olddir: %w", err)
				}

				oldPath := filepath.Join(oldDir, domainconfig.GeneratedFileName)

				return os.WriteFile(oldPath, []byte("old"), 0o600)
			},
			currentPath:     filepath.Join("subdir", domainconfig.GeneratedFileName),
			expectRemoved:   []string{filepath.Join("olddir", domainconfig.GeneratedFileName)},
			expectRemaining: []string{},
			expectInfoLogs:  1,
		},
		{
			name: "files_with_different_names_not_removed",
			setup: func(dir string) error {
				otherFile := filepath.Join(dir, "other.yml")

				return os.WriteFile(otherFile, []byte("other"), 0o600)
			},
			currentPath:     "current.yml",
			expectRemoved:   []string{},
			expectRemaining: []string{"other.yml"},
			expectInfoLogs:  0,
		},
		{
			name: "no_old_files",
			setup: func(_ string) error {
				return nil
			},
			currentPath:     "current.yml",
			expectRemoved:   []string{},
			expectRemaining: []string{},
			expectInfoLogs:  0,
		},
		{
			name: "cleanup_in_hidden_directory",
			setup: func(dir string) error {
				hiddenDir := filepath.Join(dir, ".hidden")

				if err := os.MkdirAll(hiddenDir, 0o750); err != nil {
					return fmt.Errorf("mkdir all hidden: %w", err)
				}

				oldPath := filepath.Join(hiddenDir, domainconfig.GeneratedFileName)

				return os.WriteFile(oldPath, []byte("old"), 0o600)
			},
			currentPath:     "current.yml",
			expectRemoved:   []string{filepath.Join(".hidden", domainconfig.GeneratedFileName)},
			expectRemaining: []string{},
			expectInfoLogs:  1,
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

			if tt.setup != nil {
				if err := tt.setup(tempDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fetcher := remote.NewMockRemoteFetcher(ctrl)

			svc := configinfra.NewService(logger, fetcher)

			// Test cleanupGeneratedFiles through Prepare
			// Create a minimal config file
			configPath := "config.yml"
			if err := os.WriteFile(configPath, []byte("linters:\n  enable: [govet]"), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			generatedPath, err := svc.Prepare(context.Background(), configPath)
			if err != nil {
				t.Fatalf("Prepare() unexpected error: %v", err)
			}

			// Verify removed files (paths are relative to current dir after Chdir)
			for _, path := range tt.expectRemoved {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("cleanupGeneratedFiles() file %s was not removed", path)
				}
			}

			// Verify remaining files (paths are relative to current dir after Chdir)
			for _, path := range tt.expectRemaining {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Fatalf("cleanupGeneratedFiles() file %s was removed but should remain", path)
				}
			}

			// Verify generated file is not removed
			if _, err := os.Stat(generatedPath); os.IsNotExist(err) {
				t.Fatalf("cleanupGeneratedFiles() generated file was removed")
			}

			// Check info logs
			var infoLogs []string

			for _, entry := range logger.entries {
				if entry.level == "info" && contains(entry.msg, "Removed old generated config") {
					infoLogs = append(infoLogs, entry.msg)
				}
			}

			if len(infoLogs) != tt.expectInfoLogs {
				t.Fatalf("cleanupGeneratedFiles() info logs count = %d, want %d\ngot: %v", len(infoLogs), tt.expectInfoLogs, infoLogs)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

