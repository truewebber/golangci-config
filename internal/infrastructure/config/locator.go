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
	if path, provided, err := domainconfig.ParseConfigFlag(args); err != nil {
		return "", fmt.Errorf("parse config flag: %w", err)
	} else if provided {
		return path, nil
	}

	for _, candidate := range domainconfig.DefaultCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", nil
}
