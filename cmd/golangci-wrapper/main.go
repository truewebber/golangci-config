package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/truewebber/golangci-config/internal/application"
	configinfra "github.com/truewebber/golangci-config/internal/infrastructure/config"
	"github.com/truewebber/golangci-config/internal/infrastructure/lint"
	"github.com/truewebber/golangci-config/internal/infrastructure/remote"
	"github.com/truewebber/golangci-config/internal/log"
)

const (
	defaultCacheDir             = ".cache/golangci-wrapper"
	remoteFetcherTimeoutSeconds = 15
)

func main() {
	logger := log.NewStdLogger()

	args := os.Args[1:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(logger)

		return
	}

	cacheDir, err := resolveCacheDir()
	if err != nil {
		logger.Error("Failed to resolve cache directory", "error", err)
		os.Exit(1)
	}

	timeout := time.Duration(remoteFetcherTimeoutSeconds) * time.Second
	fetcher := remote.NewHTTPFetcher(cacheDir, timeout)
	configService := configinfra.NewService(logger, fetcher)
	locator := configinfra.NewLocator()
	linter := lint.NewToolRunner()
	runner := application.NewRunner(logger, locator, configService, linter)

	if runErr := runner.Run(context.TODO(), args); runErr != nil {
		logger.Error("golangci-wrapper failed", "error", runErr)
		os.Exit(1)
	}
}

func printUsage(logger log.Logger) {
	logger.Info("Usage: golangci-wrapper run [golangci-lint flags]\n")
	logger.Info("The wrapper looks for a local configuration file (.golangci.local.yml/.yaml or .golangci.yml/.yaml).")
	logger.Info("If the file contains a directive in comments of the form:")
	logger.Info("  # GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml")
	logger.Info("the remote configuration is downloaded, merged with the local one, and passed to golangci-lint.")
	logger.Info("Without the directive the wrapper uses only the local configuration.\n")
	logger.Info("Examples:")
	logger.Info("  golangci-wrapper run")
	logger.Info("  golangci-wrapper run ./...")
	logger.Info("  golangci-wrapper run -c custom.yml ./...\n")
	logger.Info("Make sure golangci-lint is installed (via go tool or go install).")
}

func resolveCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	return filepath.Join(home, defaultCacheDir), nil
}
