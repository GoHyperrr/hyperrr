package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	covFile := "test_run.out"
	content := "mode: set\nfile.go:1.1,2.2 10 1\n"
	os.WriteFile(covFile, []byte(content), 0644)
	defer os.Remove(covFile)

	t.Run("Successful run", func(t *testing.T) {
		out := &bytes.Buffer{}
		err := run([]string{"cmd", covFile, "90"}, out)
		if err != nil {
			t.Errorf("run() failed: %v", err)
		}
		if !strings.Contains(out.String(), "Total coverage: 100.00%") {
			t.Errorf("unexpected output: %s", out.String())
		}
	})

	t.Run("Below threshold", func(t *testing.T) {
		out := &bytes.Buffer{}
		err := run([]string{"cmd", covFile, "110"}, out)
		if err == nil {
			t.Error("expected error for below threshold, got nil")
		}
	})

	t.Run("Missing args", func(t *testing.T) {
		err := run([]string{"cmd"}, &bytes.Buffer{})
		if err == nil {
			t.Error("expected error for missing args, got nil")
		}
	})

	t.Run("Invalid threshold", func(t *testing.T) {
		err := run([]string{"cmd", covFile, "abc"}, &bytes.Buffer{})
		if err == nil {
			t.Error("expected error for invalid threshold, got nil")
		}
	})

	t.Run("Non-existent file", func(t *testing.T) {
		err := run([]string{"cmd", "missing.out", "90"}, &bytes.Buffer{})
		if err == nil {
			t.Error("expected error for non-existent file, got nil")
		}
	})
}

func TestMainFunc(t *testing.T) {
	oldExit := osExit
	defer func() { osExit = oldExit }()
	osExit = func(code int) {}

	// Mock valid args for main
	covFile := "test_main_func.out"
	os.WriteFile(covFile, []byte("mode: set\nfile.go:1.1,2.2 10 1\n"), 0644)
	defer os.Remove(covFile)

	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	t.Run("main success", func(t *testing.T) {
		os.Args = []string{"cmd", covFile, "0"}
		main()
	})

	t.Run("main failure", func(t *testing.T) {
		os.Args = []string{"cmd"}
		main()
	})
}

func TestCalculateCoverage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{
			name: "Basic coverage",
			input: `mode: set
github.com/user/repo/file.go:1.2,3.4 10 1
github.com/user/repo/file.go:5.6,7.8 10 0`,
			expected: 50.0,
		},
		{
			name: "No statements",
			input: `mode: set`,
			expected: 100.0,
		},
		{
			name: "100 percent coverage",
			input: `mode: set
file1.go:1.1,2.2 5 1
file2.go:1.1,2.2 5 1`,
			expected: 100.0,
		},
		{
			name: "0 percent coverage",
			input: `mode: set
file1.go:1.1,2.2 5 0`,
			expected: 0.0,
		},
		{
			name: "Standard format with more parts",
			input: `mode: set
github.com/user/repo/file.go:1.2,3.4 1 2 3 4 5 10 1`,
			expected: 100.0,
		},
		{
			name: "Invalid lines",
			input: `mode: set
invalid line
another:invalid 1
yet:another:invalid 1 2`,
			expected: 100.0,
		},
		{
			name: "Partial coverage complex",
			input: `mode: set
file1.go:1.1,2.2 10 5
file1.go:3.3,4.4 10 0`,
			expected: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateCoverage(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("calculateCoverage() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("calculateCoverage() got = %v, want %v", got, tt.expected)
			}
		})
	}
}
