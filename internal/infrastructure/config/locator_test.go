package configinfra_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	configinfra "github.com/truewebber/golangci-config/internal/infrastructure/config"
)

//nolint:paralleltest // Cannot use t.Parallel() with t.TempDir() and file operations
func TestLocatorLocate(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		setup     func(string) error
		want      string
		wantErr   bool
		errCheck  func(error) bool
	}{
		{
			name: "flag_c_provided_file_exists",
			args: []string{"-c", "custom.yml"},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "custom.yml"), []byte("test"), 0o600)
			},
			want: "custom.yml",
		},
		{
			name: "flag_config_provided_file_exists",
			args: []string{"--config", "custom.yml"},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "custom.yml"), []byte("test"), 0o600)
			},
			want: "custom.yml",
		},
		{
			name: "flag_config_equals_provided",
			args: []string{"--config=custom.yml"},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "custom.yml"), []byte("test"), 0o600)
			},
			want: "custom.yml",
		},
		{
			name: "flag_not_provided_first_candidate_exists",
			args: []string{"run", "./..."},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".golangci.local.yml"), []byte("test"), 0o600)
			},
			want: ".golangci.local.yml",
		},
		{
			name: "flag_not_provided_second_candidate_exists",
			args: []string{"run", "./..."},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".golangci.local.yaml"), []byte("test"), 0o600)
			},
			want: ".golangci.local.yaml",
		},
		{
			name: "flag_not_provided_all_candidates_exist_first_found",
			args: []string{"run", "./..."},
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, ".golangci.local.yml"), []byte("test"), 0o600); err != nil {
					return fmt.Errorf("write first candidate: %w", err)
				}

				if err := os.WriteFile(filepath.Join(dir, ".golangci.local.yaml"), []byte("test"), 0o600); err != nil {
					return fmt.Errorf("write second candidate: %w", err)
				}

				return nil
			},
			want: ".golangci.local.yml",
		},
		{
			name: "flag_provided_file_not_exists",
			args: []string{"-c", "nonexistent.yml"},
			setup: func(_ string) error {
				return nil
			},
			want: "nonexistent.yml",
		},
		{
			name: "flag_provided_parse_error",
			args: []string{"-c"},
			setup: func(_ string) error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "flag_not_provided_no_candidates_exist",
			args: []string{"run", "./..."},
			setup: func(_ string) error {
				return nil
			},
			want: "",
		},
		{
			name: "flag_with_empty_path",
			args: []string{"-c", ""},
			setup: func(_ string) error {
				return nil
			},
			want: "",
		},
		{
			name: "flag_not_provided_third_candidate_exists",
			args: []string{"run", "./..."},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".golangci.yml"), []byte("test"), 0o600)
			},
			want: ".golangci.yml",
		},
		{
			name: "flag_not_provided_fourth_candidate_exists",
			args: []string{"run", "./..."},
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, ".golangci.yaml"), []byte("test"), 0o600)
			},
			want: ".golangci.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if tt.setup != nil {
				if err := tt.setup(tempDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			t.Chdir(tempDir)

			locator := configinfra.NewLocator()
			got, err := locator.Locate(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Locate() expected error, got nil")
				}

				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Fatalf("Locate() error = %v, want specific error", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("Locate() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("Locate() = %q, want %q", got, tt.want)
			}
		})
	}
}

