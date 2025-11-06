package config

import (
	"bufio"
	"regexp"
	"strings"
)

const directiveMatchLength = 2

var remoteDirectivePattern = regexp.MustCompile(`(?i)` + RemoteDirective + `:\s*(\S+)`)

// ExtractRemoteURL parses YAML/JSON-like content and returns the first remote configuration URL found.
func ExtractRemoteURL(data []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			matches := remoteDirectivePattern.FindStringSubmatch(line)
			if len(matches) == directiveMatchLength {
				return matches[1]
			}
		}
	}

	return ""
}
