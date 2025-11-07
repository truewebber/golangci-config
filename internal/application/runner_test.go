package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/truewebber/golangci-config/internal/application"
	"go.uber.org/mock/gomock"
)

var (
	errLocateFailed = errors.New("locate failed")
	errPrepareFailed = errors.New("prepare failed")
	errEnsureFailed = errors.New("ensure failed")
	errRunFailed = errors.New("run failed")
)

func TestBuildFinalArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		original        []string
		generatedConfig string
		originalConfig  string
		want            []string
	}{
		{
			name:            "no_config_flags",
			original:        []string{"run", "./..."},
			generatedConfig: "generated.yml",
			originalConfig:  "",
			want:            []string{"run", "./...", "--config", "generated.yml"},
		},
		{
			name:            "with_c_flag",
			original:        []string{"-c", "custom.yml", "run"},
			generatedConfig: "generated.yml",
			originalConfig:  "custom.yml",
			want:            []string{"run", "--config", "generated.yml"},
		},
		{
			name:            "with_config_flag",
			original:        []string{"--config", "custom.yml", "run"},
			generatedConfig: "generated.yml",
			originalConfig:  "custom.yml",
			want:            []string{"run", "--config", "generated.yml"},
		},
		{
			name:            "with_config_equals",
			original:        []string{"--config=custom.yml", "run"},
			generatedConfig: "generated.yml",
			originalConfig:  "custom.yml",
			want:            []string{"run", "--config", "generated.yml"},
		},
		{
			name:            "with_generated_config",
			original:        []string{"run", "./..."},
			generatedConfig: "generated.yml",
			originalConfig:  "original.yml",
			want:            []string{"run", "./...", "--config", "generated.yml"},
		},
		{
			name:            "without_generated_with_original",
			original:        []string{"run", "./..."},
			generatedConfig: "",
			originalConfig:  "original.yml",
			want:            []string{"run", "./...", "--config", "original.yml"},
		},
		{
			name:            "empty_args_adds_run",
			original:        []string{},
			generatedConfig: "",
			originalConfig:  "",
			want:            []string{"run"},
		},
		{
			name:            "config_flag_at_end",
			original:        []string{"run", "-c", "custom.yml"},
			generatedConfig: "generated.yml",
			originalConfig:  "custom.yml",
			want:            []string{"run", "--config", "generated.yml"},
		},
		{
			name:            "multiple_config_flags_all_removed",
			original:        []string{"-c", "first.yml", "--config", "second.yml", "--config=third.yml"},
			generatedConfig: "generated.yml",
			originalConfig:  "third.yml",
			want:            []string{"--config", "generated.yml"},
		},
		{
			name:            "other_flags_between_config",
			original:        []string{"-c", "custom.yml", "--verbose", "--config", "other.yml"},
			generatedConfig: "generated.yml",
			originalConfig:  "other.yml",
			want:            []string{"--verbose", "--config", "generated.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := application.BuildFinalArgs(tt.original, tt.generatedConfig, tt.originalConfig)

			if len(got) != len(tt.want) {
				t.Fatalf("BuildFinalArgs() length = %d, want %d\ngot: %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("BuildFinalArgs()[%d] = %q, want %q\ngot: %v\nwant: %v", i, got[i], tt.want[i], got, tt.want)
				}
			}
		})
	}
}

func TestRunnerRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		locateResult    string
		locateErr       error
		prepareResult   string
		prepareErr      error
		ensureErr       error
		runErr          error
		wantErr         bool
		errContains     string
		expectFinalArgs []string
	}{
		{
			name:          "successful_run",
			args:          []string{"run", "./..."},
			locateResult:  "config.yml",
			prepareResult: "generated.yml",
			expectFinalArgs: []string{"run", "./...", "--config", "generated.yml"},
		},
		{
			name:      "locate_error",
			args:      []string{"run"},
			locateErr: errLocateFailed,
			wantErr:   true,
			errContains: "locate config",
		},
		{
			name:        "prepare_error",
			args:        []string{"run"},
			locateResult: "config.yml",
			prepareErr:  errPrepareFailed,
			wantErr:     true,
			errContains:  "prepare config",
		},
		{
			name:      "ensure_error",
			args:      []string{"run"},
			locateResult: "config.yml",
			prepareResult: "generated.yml",
			ensureErr: errEnsureFailed,
			wantErr:   true,
			errContains: "ensure linter available",
		},
		{
			name:      "run_error",
			args:      []string{"run"},
			locateResult: "config.yml",
			prepareResult: "generated.yml",
			runErr:    errRunFailed,
			wantErr:   true,
			errContains: "run linter",
		},
		{
			name:          "no_config_file",
			args:          []string{"run"},
			locateResult:  "",
			expectFinalArgs: []string{"run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			configLocator := NewMockConfigLocator(ctrl)
			configService := NewMockConfigService(ctrl)
			linter := NewMockLinter(ctrl)

			configLocator.EXPECT().
				Locate(tt.args).
				Return(tt.locateResult, tt.locateErr)

			if tt.locateErr != nil {
				runner := application.NewRunner(logger, configLocator, configService, linter)

				err := runner.Run(context.Background(), tt.args)

				if !tt.wantErr {
					t.Fatalf("Run() unexpected error: %v", err)

					return
				}

				if err == nil {
					t.Fatalf("Run() expected error, got nil")
				}

				if !contains(err.Error(), tt.errContains) {
					t.Fatalf("Run() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			if tt.locateResult != "" {
				if tt.prepareErr != nil {
					configService.EXPECT().
						Prepare(gomock.Any(), tt.locateResult).
						Return("", tt.prepareErr)
				} else {
					configService.EXPECT().
						Prepare(gomock.Any(), tt.locateResult).
						Return(tt.prepareResult, nil)
				}
			}

			if tt.prepareErr != nil {
				runner := application.NewRunner(logger, configLocator, configService, linter)

				err := runner.Run(context.Background(), tt.args)

				if !tt.wantErr {
					t.Fatalf("Run() unexpected error: %v", err)

					return
				}

				if err == nil {
					t.Fatalf("Run() expected error, got nil")
				}

				if !contains(err.Error(), tt.errContains) {
					t.Fatalf("Run() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			linter.EXPECT().
				EnsureAvailable(gomock.Any()).
				Return(tt.ensureErr)

			if tt.ensureErr != nil {
				runner := application.NewRunner(logger, configLocator, configService, linter)

				err := runner.Run(context.Background(), tt.args)

				if !tt.wantErr {
					t.Fatalf("Run() unexpected error: %v", err)

					return
				}

				if err == nil {
					t.Fatalf("Run() expected error, got nil")
				}

				if !contains(err.Error(), tt.errContains) {
					t.Fatalf("Run() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			linter.EXPECT().
				Run(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, args []string) error {
					if tt.expectFinalArgs != nil {
						if len(args) != len(tt.expectFinalArgs) {
							t.Errorf("Run() args length = %d, want %d\ngot: %v\nwant: %v", len(args), len(tt.expectFinalArgs), args, tt.expectFinalArgs)
						} else {
							for i := range args {
								if args[i] != tt.expectFinalArgs[i] {
									t.Errorf("Run() args[%d] = %q, want %q", i, args[i], tt.expectFinalArgs[i])
								}
							}
						}
					}

					return tt.runErr
				})

			runner := application.NewRunner(logger, configLocator, configService, linter)

			err := runner.Run(context.Background(), tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Run() expected error, got nil")
				}

				if !contains(err.Error(), tt.errContains) {
					t.Fatalf("Run() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}
		})
	}
}

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

func TestRunnerPrepareConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		localConfig     string
		prepareResult   string
		prepareErr      error
		wantGenerated   string
		wantErr         bool
		errContains     string
		expectWarnings  []string
		expectInfoLogs  []string
	}{
		{
			name:          "successful_preparation",
			localConfig:   "config.yml",
			prepareResult: "generated.yml",
			wantGenerated: "generated.yml",
			expectInfoLogs: []string{},
		},
		{
			name:          "empty_local_config",
			localConfig:   "",
			wantGenerated: "",
			expectWarnings: []string{"Local configuration file not found"},
		},
		{
			name:        "prepare_error",
			localConfig: "config.yml",
			prepareErr:  errPrepareFailed,
			wantErr:     true,
			errContains:  "prepare config",
		},
		{
			name:          "empty_prepare_result",
			localConfig:   "config.yml",
			prepareResult: "",
			wantGenerated: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := &stubLogger{}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			configLocator := NewMockConfigLocator(ctrl)
			configService := NewMockConfigService(ctrl)
			linter := NewMockLinter(ctrl)

			// Setup mocks for Run flow
			configLocator.EXPECT().
				Locate([]string{"run"}).
				Return(tt.localConfig, nil)

			if tt.localConfig != "" {
				if tt.prepareErr != nil {
					configService.EXPECT().
						Prepare(gomock.Any(), tt.localConfig).
						Return("", tt.prepareErr)
				} else {
					configService.EXPECT().
						Prepare(gomock.Any(), tt.localConfig).
						Return(tt.prepareResult, nil)
				}
			}

			if tt.prepareErr == nil {
				linter.EXPECT().
					EnsureAvailable(gomock.Any()).
					Return(nil)

				linter.EXPECT().
					Run(gomock.Any(), gomock.Any()).
					Return(nil)
			}

			runner := application.NewRunner(logger, configLocator, configService, linter)

			err := runner.Run(context.Background(), []string{"run"})

			if tt.wantErr {
				if err == nil {
					t.Fatalf("prepareConfig() expected error, got nil")
				}

				if !contains(err.Error(), tt.errContains) {
					t.Fatalf("prepareConfig() error = %v, want to contain %q", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Fatalf("prepareConfig() unexpected error: %v", err)
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
						t.Fatalf("prepareConfig() missing expected warning %q, got %v", expected, warnings)
					}
				}
			} else if len(warnings) > 0 {
				t.Fatalf("prepareConfig() unexpected warnings: %v", warnings)
			}
		})
	}
}

func contains(s, substr string) bool {
	if substr == "" {
		return true
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

