package config

import (
	"errors"
	"strings"
)

var ErrMissingConfigValue = errors.New("missing value for -c/--config flag")

type ConfigFlagResult struct {
	Path     string
	Provided bool
}

func ParseConfigFlag(args []string) (ConfigFlagResult, error) {
	for index, arg := range args {
		switch {
		case arg == "-c", arg == "--config":
			nextIndex := index + 1
			if nextIndex >= len(args) {
				return ConfigFlagResult{Path: "", Provided: true}, ErrMissingConfigValue
			}

			return ConfigFlagResult{Path: args[nextIndex], Provided: true}, nil
		case strings.HasPrefix(arg, "--config="):
			return ConfigFlagResult{Path: strings.TrimPrefix(arg, "--config="), Provided: true}, nil
		}
	}

	return ConfigFlagResult{Path: "", Provided: false}, nil
}

func DefaultCandidates() []string {
	return []string{
		".golangci.local.yml",
		".golangci.local.yaml",
		".golangci.yml",
		".golangci.yaml",
	}
}
