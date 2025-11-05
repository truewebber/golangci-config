package config

import (
	"errors"
	"strings"
)

var ErrMissingConfigValue = errors.New("missing value for -c/--config flag")

func ParseConfigFlag(args []string) (string, bool, error) {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-c", arg == "--config":
			if i+1 >= len(args) {
				return "", true, ErrMissingConfigValue
			}
			return args[i+1], true, nil
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
