package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func Merge(base, override interface{}) interface{} {
	if override == nil {
		override = map[string]interface{}{}
	}

	switch baseTyped := base.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(baseTyped))
		for key, val := range baseTyped {
			result[key] = DeepCopy(val)
		}

		overrideMap, ok := override.(map[string]interface{})
		if !ok {
			return DeepCopy(override)
		}

		for key, value := range overrideMap {
			if existing, exists := result[key]; exists {
				result[key] = Merge(existing, value)
			} else {
				result[key] = DeepCopy(value)
			}
		}

		return result
	case []interface{}:
		overrideSlice, ok := override.([]interface{})
		if !ok {
			return DeepCopy(override)
		}

		return DeepCopy(overrideSlice)
	default:
		return DeepCopy(override)
	}
}

func DeepCopy(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			result[key] = DeepCopy(value)
		}

		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = DeepCopy(item)
		}

		return result
	default:
		return v
	}
}

func NormalizeYAML(data []byte) (interface{}, error) {
	var content interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	return normalize(content), nil
}

func normalize(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			result[key] = normalize(value)
		}

		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			result[fmt.Sprint(key)] = normalize(value)
		}

		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = normalize(item)
		}

		return result
	default:
		return v
	}
}
