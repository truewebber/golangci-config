package config

import (
	"errors"
	"strings"
)

var ErrMissingConfigValue = errors.New("missing value for -c/--config flag")

func ParseConfigFlag(args []string) (path string, provided bool, err error) {
	for index, arg := range args {
		switch {
		case arg == "-c", arg == "--config":
			nextIndex := index + 1
			if nextIndex >= len(args) {
				return "", true, ErrMissingConfigValue
			}

			return args[nextIndex], true, nil
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config="), true, nil
		}
	}

	return "", false, nil
}

func DefaultCandidates() []string {
	return []string{
		".golangci.local.yml",
		".golangci.local.yaml",
		".golangci.yml",
		".golangci.yaml",
	}
}
