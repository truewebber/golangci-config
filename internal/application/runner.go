package application

import (
	"fmt"
	"strings"

	loggerpkg "github.com/truewebber/golangci-config/internal/logger"
)

type ConfigLocator interface {
	Locate(args []string) (string, error)
}

type ConfigService interface {
	Prepare(localConfigPath string) (string, error)
}

type Linter interface {
	EnsureAvailable() error
	Run(args []string) error
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

func (r *Runner) Run(args []string) error {
	localConfig, err := r.configLocator.Locate(args)
	if err != nil {
		return fmt.Errorf("locate config: %w", err)
	}

	var generatedConfig string
	if localConfig != "" {
		generatedConfig, err = r.configService.Prepare(localConfig)
		if err != nil {
			return fmt.Errorf("prepare config: %w", err)
		}
	} else {
		r.logger.Warn("Local configuration file not found; running without generated config")
	}

	if err := r.linter.EnsureAvailable(); err != nil {
		return fmt.Errorf("ensure linter available: %w", err)
	}

	finalArgs := buildFinalArgs(args, generatedConfig, localConfig)
	return r.linter.Run(finalArgs)
}

func buildFinalArgs(original []string, generatedConfig, originalConfig string) []string {
	finalArgs := make([]string, 0, len(original)+2)
	skipNext := false

	for i := 0; i < len(original); i++ {
		if skipNext {
			skipNext = false
			continue
		}

		arg := original[i]
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
