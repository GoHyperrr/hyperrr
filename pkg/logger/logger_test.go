package logger

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	t.Run("Basic logging", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{
			Level:  "debug",
			Format: "text",
			Output: buf,
		})

		l.Debug("debug message")
		l.Info("info message")
		l.Warn("warn message")
		l.Error("error message")

		output := buf.String()
		if !strings.Contains(output, "level=DEBUG msg=\"debug message\"") {
			t.Errorf("expected debug message, got %s", output)
		}
		if !strings.Contains(output, "level=INFO msg=\"info message\"") {
			t.Errorf("expected info message, got %s", output)
		}
	})

	t.Run("JSON logging", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{
			Level:  "info",
			Format: "json",
			Output: buf,
		})

		l.Info("json message", "key", "value")

		output := buf.String()
		if !strings.Contains(output, "\"level\":\"INFO\"") {
			t.Errorf("expected JSON level INFO, got %s", output)
		}
		if !strings.Contains(output, "\"msg\":\"json message\"") {
			t.Errorf("expected JSON msg, got %s", output)
		}
		if !strings.Contains(output, "\"key\":\"value\"") {
			t.Errorf("expected JSON key:value, got %s", output)
		}
	})

	t.Run("Global logger", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{
			Level:  "info",
			Format: "text",
			Output: buf,
		})
		SetGlobal(l)

		if Get() != l {
			t.Fatal("Get() did not return the expected global logger")
		}

		Info("global info")
		Debug("global debug") // Should not appear as level is info
		Warn("global warn")
		Error("global error")

		output := buf.String()
		if !strings.Contains(output, "msg=\"global info\"") {
			t.Errorf("expected global info, got %s", output)
		}
		if strings.Contains(output, "msg=\"global debug\"") {
			t.Error("did not expect global debug message")
		}
	})

	t.Run("With and Context", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{Output: buf})
		SetGlobal(l)

		wl := With("request_id", "123")
		wl.Info("with context")
		
		ctxL := Context(context.Background())
		ctxL.Info("from context")

		output := buf.String()
		if !strings.Contains(output, "request_id=123") {
			t.Errorf("expected request_id attribute, got %s", output)
		}
	})
	
	t.Run("Default level", func(t *testing.T) {
		l := New(&Config{Level: "invalid"})
		// Should default to Info
		if !l.Enabled(context.Background(), LevelInfo) {
			t.Error("expected Info to be enabled by default")
		}
		if l.Enabled(context.Background(), LevelDebug) {
			t.Error("did not expect Debug to be enabled by default")
		}
	})

	t.Run("All levels", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{Level: "debug", Output: buf})
		l.Debug("d")
		l.Info("i")
		l.Warn("w")
		l.Error("e")

		output := buf.String()
		for _, m := range []string{"DEBUG", "INFO", "WARN", "ERROR"} {
			if !strings.Contains(output, "level="+m) {
				t.Errorf("expected level %s in output", m)
			}
		}
	})

	t.Run("Level specific", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{Level: "warn", Output: buf})
		l.Info("should not appear")
		l.Warn("should appear")

		output := buf.String()
		if strings.Contains(output, "INFO") {
			t.Error("Info should be filtered out")
		}
		if !strings.Contains(output, "WARN") {
			t.Error("Warn should appear")
		}
	})

	t.Run("Explicit error level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		l := New(&Config{Level: "error", Output: buf})
		l.Warn("w")
		l.Error("e")
		if strings.Contains(buf.String(), "WARN") {
			t.Error("Warn should be filtered out")
		}
	})
}
