package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Run("Valid input", func(t *testing.T) {
		content := "mode: set\nfile.go:1.1,2.2 1 1\n"
		tmpfile, _ := os.CreateTemp("", "coverage.out")
		defer os.Remove(tmpfile.Name())
		tmpfile.WriteString(content)
		tmpfile.Close()

		var buf bytes.Buffer
		err := run([]string{"cmd", tmpfile.Name(), "50"}, &buf)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}
		if !strings.Contains(buf.String(), "Total coverage: 100.00%") {
			t.Errorf("unexpected output: %s", buf.String())
		}
	})

	t.Run("Below threshold", func(t *testing.T) {
		content := "mode: set\nfile.go:1.1,2.2 1 0\n"
		tmpfile, _ := os.CreateTemp("", "coverage.out")
		defer os.Remove(tmpfile.Name())
		tmpfile.WriteString(content)
		tmpfile.Close()

		var buf bytes.Buffer
		err := run([]string{"cmd", tmpfile.Name(), "90"}, &buf)
		if err == nil {
			t.Fatal("expected error for low coverage")
		}
		if !strings.Contains(buf.String(), "Total coverage: 0.00%") {
			t.Errorf("unexpected output: %s", buf.String())
		}
	})

	t.Run("Invalid threshold", func(t *testing.T) {
		err := run([]string{"cmd", "file.out", "abc"}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "invalid threshold") {
			t.Error("expected error for invalid threshold")
		}
	})

	t.Run("File not found", func(t *testing.T) {
		err := run([]string{"cmd", "non-existent.out", "90"}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "error opening coverage file") {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("Usage error", func(t *testing.T) {
		err := run([]string{"cmd"}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "usage") {
			t.Error("expected usage error")
		}
	})
}

func TestCalculateCoverage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name:     "Full coverage",
			input:    "mode: set\nfile.go:1.1,2.2 1 1\n",
			expected: 100.0,
		},
		{
			name:     "No coverage",
			input:    "mode: set\nfile.go:1.1,2.2 1 0\n",
			expected: 0.0,
		},
		{
			name:     "Partial coverage",
			input:    "mode: set\nfile.go:1.1,2.2 1 1\nfile.go:3.1,4.2 1 0\n",
			expected: 50.0,
		},
		{
			name:     "Skip generated files",
			input:    "mode: set\nfile.go:1.1,2.2 1 1\ngenerated.go:1.1,2.2 10 0\n",
			expected: 100.0,
		},
		{
			name:     "Skip test files",
			input:    "mode: set\nfile.go:1.1,2.2 1 1\nfile_test.go:1.1,2.2 10 0\n",
			expected: 100.0,
		},
		{
			name:     "Empty input",
			input:    "mode: set\n",
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, _, _, _, err := calculateCoverage(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("calculateCoverage() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("calculateCoverage() got = %v, want %v", got, tt.expected)
			}
		})
	}
}
