package generator

import "strings"

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getInt(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}

func getFloat(m map[string]interface{}, key string, def float64) float64 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return def
}

func getNumber(m map[string]interface{}, key string, def float64) interface{} {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			if n == float64(int(n)) {
				return int(n)
			}
			return n
		}
	}
	if def == float64(int(def)) {
		return int(def)
	}
	return def
}

func getBool(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}

func getStringSlice(m map[string]interface{}, key string, def []string) []interface{} {
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case []interface{}:
			return s
		case []string:
			result := make([]interface{}, len(s))
			for i, str := range s {
				result[i] = str
			}
			return result
		}
	}
	if def == nil {
		return nil
	}
	result := make([]interface{}, len(def))
	for i, s := range def {
		result[i] = s
	}
	return result
}

// getStringSliceAsStrings returns a string slice (for comparison panel datasources).
func getStringSliceAsStrings(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		switch s := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(s))
			for _, item := range s {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		case []string:
			return s
		}
	}
	return nil
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
