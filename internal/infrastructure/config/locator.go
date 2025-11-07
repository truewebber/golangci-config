package configinfra

import (
	"fmt"
	"os"

	domainconfig "github.com/truewebber/golangci-config/internal/domain/config"
)

type Locator struct {
}

func NewLocator() *Locator {
	return &Locator{}
}

func (l *Locator) Locate(args []string) (string, error) {
	result, err := domainconfig.ParseConfigFlag(args)
	if err != nil {
		return "", fmt.Errorf("parse config flag: %w", err)
	}

	if result.Provided {
		return result.Path, nil
	}

	for _, candidate := range domainconfig.DefaultCandidates() {
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	return "", nil
}
