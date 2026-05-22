package utils

// GetString safely extracts a string from a map[string]any.
func GetString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetFloat64 safely extracts a float64 from a map[string]any.
func GetFloat64(m map[string]any, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
		if i, ok := v.(int); ok {
			return float64(i)
		}
	}
	return 0
}

// GetBool safely extracts a bool from a map[string]any.
func GetBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetMap safely extracts a map[string]any from a map[string]any.
func GetMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if res, ok := v.(map[string]any); ok {
			return res
		}
	}
	return nil
}

// GetSlice safely extracts an []any from a map[string]any.
func GetSlice(m map[string]any, key string) []any {
	if v, ok := m[key]; ok {
		if res, ok := v.([]any); ok {
			return res
		}
	}
	return nil
}
