package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldRun(t *testing.T) {
	// Create a temporary project directory inside the workspace
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	tempProjectName := "test_temp_project"
	tempProjectPath := filepath.Join(cwd, tempProjectName)

	// Cleanup any leftover directory
	_ = os.RemoveAll(tempProjectPath)
	defer os.RemoveAll(tempProjectPath)

	cfg := &ScaffoldConfig{
		ProjectName:      tempProjectName,
		ProjectPath:      tempProjectPath,
		ModulePath:       "github.com/test-user/test-project",
		PresetName:       "commerce-minimal",
		DBDriver:         "sqlite",
		DBDSN:            "test_temp.db",
		EventBusProvider: "inmem",
		SkipGit:          true, // skip git init in unit tests
	}

	// Run scaffold
	if err := Run(cfg); err != nil {
		t.Fatalf("Run() scaffold failed: %v", err)
	}

	// Verify expected files exist
	expectedFiles := []string{
		filepath.Join(tempProjectPath, "cmd", "server", "main.go"),
		filepath.Join(tempProjectPath, "cmd", "server", "imports_generated.go"),
		filepath.Join(tempProjectPath, "configs", "hyperrr.yml"),
		filepath.Join(tempProjectPath, ".env"),
		filepath.Join(tempProjectPath, ".env.example"),
		filepath.Join(tempProjectPath, ".gitignore"),
		filepath.Join(tempProjectPath, "api", "graph", "schema.graphqls"),
		filepath.Join(tempProjectPath, "README.md"),
		filepath.Join(tempProjectPath, "Makefile"),
		filepath.Join(tempProjectPath, "go.mod"),
		filepath.Join(tempProjectPath, "modules", ".gitkeep"),
	}

	for _, path := range expectedFiles {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s does not exist", path)
		}
	}

	// Check yml content for modules list
	ymlBytes, err := os.ReadFile(filepath.Join(tempProjectPath, "configs", "hyperrr.yml"))
	if err != nil {
		t.Fatalf("failed to read hyperrr.yml: %v", err)
	}

	ymlContent := string(ymlBytes)
	preset, _ := Presets["commerce-minimal"]
	for _, mod := range preset.Modules {
		if !strings.Contains(ymlContent, mod.Resolve) {
			t.Errorf("expected module %s resolve path in hyperrr.yml, but it was missing", mod.Resolve)
		}
	}
}

func TestResolveModule(t *testing.T) {
	tests := []struct {
		input    string
		expected ModuleInfo
		found    bool
	}{
		{
			input:    "product",
			expected: ModuleInfo{ID: "commerce.product", Resolve: "github.com/GoHyperrr/commerce/product"},
			found:    true,
		},
		{
			input:    "apikey",
			expected: ModuleInfo{ID: "auth.apikey", Resolve: "github.com/GoHyperrr/auth/apikey"},
			found:    true,
		},
		{
			input:    "github.com/test-org/custom-plugin",
			expected: ModuleInfo{ID: "custom-plugin", Resolve: "github.com/test-org/custom-plugin"},
			found:    true,
		},
		{
			input:    "invalid-module-no-slashes",
			expected: ModuleInfo{},
			found:    false,
		},
	}

	for _, test := range tests {
		res, ok := ResolveModule(test.input)
		if ok != test.found {
			t.Errorf("ResolveModule(%s) returned ok=%t, expected %t", test.input, ok, test.found)
		}
		if ok {
			if res.ID != test.expected.ID || res.Resolve != test.expected.Resolve {
				t.Errorf("ResolveModule(%s) returned ID=%s, Resolve=%s; expected ID=%s, Resolve=%s",
					test.input, res.ID, res.Resolve, test.expected.ID, test.expected.Resolve)
			}
		}
	}
}
