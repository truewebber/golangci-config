package config

import (
	"bufio"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	urlpkg "github.com/truewebber/gopkg/url"
)

const directiveMatchLength = 2

var remoteDirectivePattern = regexp.MustCompile(`(?i)` + RemoteDirective + `:\s*(\S+)`)

var ErrNoURLFound = fmt.Errorf("no URL found")

// ExtractRemoteURL parses YAML/JSON-like content and returns the first remote configuration URL found.
func ExtractRemoteURL(data []byte) (*url.URL, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			continue
		}

		matches := remoteDirectivePattern.FindStringSubmatch(line)
		if len(matches) != directiveMatchLength {
			continue
		}

		remoteURL, err := urlpkg.NormalizeWithOptions(matches[1])
		if err != nil {
			return nil, fmt.Errorf("normalize url: %w", err)
		}

		return remoteURL, nil
	}

	return nil, ErrNoURLFound
}
