package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func HasContent(data []byte) bool {
	if len(strings.TrimSpace(string(data))) == 0 {
		return false
	}

	var content interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		// Unparseable YAML – assume user intended to provide content.
		return true
	}

	switch v := content.(type) {
	case nil:
		return false
	case map[string]interface{}:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return true
	}
}
