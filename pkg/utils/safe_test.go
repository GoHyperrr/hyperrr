package utils

import "testing"

func TestSafeUtils(t *testing.T) {
	t.Run("GetString", func(t *testing.T) {
		m := map[string]any{"key": "value", "wrong": 123}
		if GetString(m, "key") != "value" {
			t.Error("GetString failed to retrieve string")
		}
		if GetString(m, "wrong") != "" {
			t.Error("GetString should return empty for non-string")
		}
		if GetString(m, "missing") != "" {
			t.Error("GetString should return empty for missing key")
		}
	})

	t.Run("GetFloat64", func(t *testing.T) {
		m := map[string]any{
			"float": 10.5,
			"int":   42,
			"int64": int64(64),
			"int32": int32(32),
			"str":   "fail",
		}
		if GetFloat64(m, "float") != 10.5 {
			t.Error("GetFloat64 failed to retrieve float64")
		}
		if GetFloat64(m, "int") != 42.0 {
			t.Error("GetFloat64 failed to convert int to float64")
		}
		if GetFloat64(m, "int64") != 64.0 {
			t.Error("GetFloat64 failed to convert int64 to float64")
		}
		if GetFloat64(m, "int32") != 32.0 {
			t.Error("GetFloat64 failed to convert int32 to float64")
		}
		if GetFloat64(m, "str") != 0 {
			t.Error("GetFloat64 should return 0 for non-numeric")
		}
		if GetFloat64(m, "missing") != 0 {
			t.Error("GetFloat64 should return 0 for missing key")
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		m := map[string]any{"true": true, "false": false, "str": "fail"}
		if GetBool(m, "true") != true {
			t.Error("GetBool failed")
		}
		if GetBool(m, "str") != false {
			t.Error("GetBool should return false for non-bool")
		}
		if GetBool(m, "missing") != false {
			t.Error("GetBool should return false for missing key")
		}
	})

	t.Run("GetMap", func(t *testing.T) {
		inner := map[string]any{"a": 1}
		m := map[string]any{"map": inner, "str": "fail"}
		if res := GetMap(m, "map"); res == nil || res["a"] != 1 {
			t.Error("GetMap failed")
		}
		if GetMap(m, "str") != nil {
			t.Error("GetMap should return nil for non-map")
		}
		if GetMap(m, "missing") != nil {
			t.Error("GetMap should return nil for missing key")
		}
	})

	t.Run("GetSlice", func(t *testing.T) {
		inner := []any{1, 2}
		m := map[string]any{"slice": inner, "str": "fail"}
		if res := GetSlice(m, "slice"); res == nil || len(res) != 2 {
			t.Error("GetSlice failed")
		}
		if GetSlice(m, "str") != nil {
			t.Error("GetSlice should return nil for non-slice")
		}
		if GetSlice(m, "missing") != nil {
			t.Error("GetSlice should return nil for missing key")
		}
	})
}
