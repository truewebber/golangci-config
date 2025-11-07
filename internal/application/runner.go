package application

import (
	"context"
	"fmt"
	"strings"

	loggerpkg "github.com/truewebber/golangci-config/internal/log"
)

//go:generate go run go.uber.org/mock/mockgen -source=runner.go -destination=mocks_test.go -package application_test
type ConfigLocator interface {
	Locate(args []string) (string, error)
}

type ConfigService interface {
	Prepare(ctx context.Context, localConfigPath string) (string, error)
}

type Linter interface {
	EnsureAvailable(ctx context.Context) error
	Run(ctx context.Context, args []string) error
}

type Runner struct {
	logger        loggerpkg.Logger
	configLocator ConfigLocator
	configService ConfigService
	linter        Linter
}

func NewRunner(
	logger loggerpkg.Logger,
	configLocator ConfigLocator,
	configService ConfigService,
	linter Linter,
) *Runner {
	return &Runner{
		logger:        logger,
		configLocator: configLocator,
		configService: configService,
		linter:        linter,
	}
}

func (r *Runner) Run(ctx context.Context, args []string) error {
	localConfig, err := r.configLocator.Locate(args)
	if err != nil {
		return fmt.Errorf("locate config: %w", err)
	}

	generatedConfig, prepareErr := r.prepareConfig(ctx, localConfig)
	if prepareErr != nil {
		return fmt.Errorf("prepare config: %w", prepareErr)
	}

	if ensureErr := r.linter.EnsureAvailable(ctx); ensureErr != nil {
		return fmt.Errorf("ensure linter available: %w", ensureErr)
	}

	finalArgs := BuildFinalArgs(args, generatedConfig, localConfig)

	if linterErr := r.linter.Run(ctx, finalArgs); linterErr != nil {
		return fmt.Errorf("run linter: %w", linterErr)
	}

	return nil
}

// BuildFinalArgs builds final arguments for linter by removing config flags
// and adding the generated or original config.
// Exported for testing.
func BuildFinalArgs(original []string, generatedConfig, originalConfig string) []string {
	const configArgumentsCount = 2

	finalArgs := make([]string, 0, len(original)+configArgumentsCount)
	skipNext := false

	for _, arg := range original {
		if skipNext {
			skipNext = false

			continue
		}

		switch {
		case arg == "-c", arg == "--config":
			skipNext = true
		case strings.HasPrefix(arg, "--config="):
			// drop entirely
		default:
			finalArgs = append(finalArgs, arg)
		}
	}

	switch {
	case generatedConfig != "":
		finalArgs = append(finalArgs, "--config", generatedConfig)
	case originalConfig != "":
		finalArgs = append(finalArgs, "--config", originalConfig)
	}

	if len(finalArgs) == 0 {
		finalArgs = append(finalArgs, "run")
	}

	return finalArgs
}

func (r *Runner) prepareConfig(ctx context.Context, localConfig string) (string, error) {
	if localConfig == "" {
		r.logger.Warn("Local configuration file not found; running without generated config")

		return "", nil
	}

	generatedConfig, err := r.configService.Prepare(ctx, localConfig)
	if err != nil {
		return "", fmt.Errorf("prepare config: %w", err)
	}

	return generatedConfig, nil
}
