package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/truewebber/golangci-config/internal/application"
	configinfra "github.com/truewebber/golangci-config/internal/infrastructure/config"
	"github.com/truewebber/golangci-config/internal/infrastructure/lint"
	"github.com/truewebber/golangci-config/internal/infrastructure/remote"
	"github.com/truewebber/golangci-config/internal/logger"
)

const defaultCacheDir = ".cache/golangci-wrapper"

func main() {
	log := logger.NewStdLogger()

	args := os.Args[1:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return
	}

	cacheDir, err := resolveCacheDir()
	if err != nil {
		log.Error("Failed to resolve cache directory", "error", err)
		os.Exit(1)
	}

	fetcher := remote.NewHTTPFetcher(cacheDir, 15*time.Second)
	configService := configinfra.NewService(log, fetcher)
	locator := configinfra.NewLocator()
	linter := lint.NewToolRunner()
	runner := application.NewRunner(log, locator, configService, linter)

	if err := runner.Run(args); err != nil {
		log.Error("golangci-wrapper failed", "error", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: golangci-wrapper run [golangci-lint flags]")
	fmt.Println()
	fmt.Println("The wrapper looks for a local configuration file (.golangci.local.yml/.yaml or .golangci.yml/.yaml).")
	fmt.Println("If the file contains a directive in comments of the form:")
	fmt.Println("  // GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml")
	fmt.Println("the remote configuration is downloaded, merged with the local one, and passed to golangci-lint.")
	fmt.Println("Without the directive the wrapper uses only the local configuration.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  golangci-wrapper run")
	fmt.Println("  golangci-wrapper run ./...")
	fmt.Println("  golangci-wrapper run -c custom.yml ./...")
	fmt.Println()
	fmt.Println("Make sure golangci-lint is installed (via go tool or go install).")
}

func resolveCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, defaultCacheDir), nil
}
