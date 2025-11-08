package config_test

import (
	"errors"
	"testing"

	"github.com/truewebber/golangcix/internal/domain/config"
)

func TestParseConfigFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		want      config.ConfigFlagResult
		wantErr  bool
		errCheck  func(error) bool
	}{
		{
			name: "flag_c_with_value",
			args: []string{"-c", "config.yml"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "flag_config_with_value",
			args: []string{"--config", "config.yml"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "flag_config_equals",
			args: []string{"--config=path/to/file.yml"},
			want: config.ConfigFlagResult{
				Path:     "path/to/file.yml",
				Provided: true,
			},
		},
		{
			name: "flag_in_middle_of_args",
			args: []string{"run", "-c", "config.yml", "./..."},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "flag_at_end",
			args: []string{"run", "./...", "-c", "config.yml"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "flag_c_no_value_at_end",
			args: []string{"run", "-c"},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: true,
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, config.ErrMissingConfigValue)
			},
		},
		{
			name: "flag_config_no_value_at_end",
			args: []string{"run", "--config"},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: true,
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, config.ErrMissingConfigValue)
			},
		},
		{
			name: "empty_args",
			args: []string{},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: false,
			},
		},
		{
			name: "flag_c_with_empty_string",
			args: []string{"-c", ""},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: true,
			},
		},
		{
			name: "flag_config_equals_empty",
			args: []string{"--config="},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: true,
			},
		},
		{
			name: "both_flags_first_found",
			args: []string{"--config=first.yml", "-c", "second.yml"},
			want: config.ConfigFlagResult{
				Path:     "first.yml",
				Provided: true,
			},
		},
		{
			name: "flag_with_spaces_not_matched",
			args: []string{"--config = path.yml"},
			want: config.ConfigFlagResult{
				Path:     "",
				Provided: false,
			},
		},
		{
			name: "flag_lowercase_directive",
			args: []string{"--config=config.yml"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "other_flags_before_config",
			args: []string{"--verbose", "-v", "-c", "config.yml"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
		{
			name: "config_flag_after_other_content",
			args: []string{"run", "./pkg", "--config", "config.yml", "--verbose"},
			want: config.ConfigFlagResult{
				Path:     "config.yml",
				Provided: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ParseConfigFlag(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseConfigFlag() expected error, got nil")
				}

				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Fatalf("ParseConfigFlag() error = %v, want specific error", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseConfigFlag() unexpected error: %v", err)
			}

			if got.Path != tt.want.Path {
				t.Fatalf("ParseConfigFlag() Path = %q, want %q", got.Path, tt.want.Path)
			}

			if got.Provided != tt.want.Provided {
				t.Fatalf("ParseConfigFlag() Provided = %v, want %v", got.Provided, tt.want.Provided)
			}
		})
	}
}

