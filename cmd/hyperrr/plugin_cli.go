package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

func runList() error {
	modules := registry.List()
	fmt.Printf("\n🦄 Loaded Plug-in Modules (%d):\n", len(modules))
	fmt.Println(strings.Repeat("─", 60))

	if len(modules) == 0 {
		fmt.Println("  No active plugins loaded.")
		return nil
	}

	for _, m := range modules {
		modelsCount := len(m.Models())
		tasksCount := len(m.Handlers())

		tuiSupport := "NO"
		if _, ok := m.(registry.TUIProvider); ok {
			tuiSupport = "YES"
		}

		fmt.Printf("  • %-20s [Models: %d | Workflow Tasks: %d | TUI View: %s]\n",
			m.ID(), modelsCount, tasksCount, tuiSupport)
	}
	fmt.Println()
	return nil
}

func runCreate(args []string) error {
	if len(args) < 2 || args[0] != "module" {
		fmt.Println("Usage: hyperrr create module <name>")
		return fmt.Errorf("invalid arguments")
	}

	moduleName := args[1]
	// Standardize names: e.g. "hotel"
	pkgName := strings.ToLower(filepath.Base(moduleName))
	
	fmt.Printf("Scaffolding new module '%s' in './%s'...\n", moduleName, pkgName)

	if err := os.MkdirAll(pkgName, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 1. Write go.mod
	goModContent := fmt.Sprintf(`module github.com/GoHyperrr/%s

go 1.25.5

replace github.com/GoHyperrr/hyperrr => ../

require (
	github.com/GoHyperrr/hyperrr v0.0.0
)
`, pkgName)
	if err := os.WriteFile(filepath.Join(pkgName, "go.mod"), []byte(goModContent), 0644); err != nil {
		return err
	}

	// 2. Write module.go
	moduleContent := fmt.Sprintf(`package %s

import (
	"context"

	"github.com/GoHyperrr/hyperrr/pkg/workflow"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
)

// Module implements the registry.Module interface.
type Module struct{}

func NewModule() *Module {
	return &Module{}
}

func init() {
	registry.Register(NewModule())
}

func (m *Module) ID() string {
	return "plugin.%s"
}

func (m *Module) Init(ctx context.Context, deps *registry.Dependencies) error {
	// Initialize resources and hooks here
	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{}
}

func (m *Module) Handlers() map[string]workflow.TaskHandler {
	return map[string]workflow.TaskHandler{}
}
`, pkgName, pkgName)
	if err := os.WriteFile(filepath.Join(pkgName, "module.go"), []byte(moduleContent), 0644); err != nil {
		return err
	}

	// 3. Write models.go
	modelsContent := fmt.Sprintf(`package %s

// Define your GORM Database tables here.
// E.g.:
// type CustomEntity struct {
//     ID   string ` + "`" + `gorm:"primaryKey"` + "`" + `
//     Name string
// }
`, pkgName)
	if err := os.WriteFile(filepath.Join(pkgName, "models.go"), []byte(modelsContent), 0644); err != nil {
		return err
	}

	// 4. Register in go.work if present
	workPath := "go.work"
	if _, err := os.Stat(workPath); err == nil {
		fmt.Println("Adding new module to go.work workspace...")
		content, err := os.ReadFile(workPath)
		if err == nil {
			lines := strings.Split(string(content), "\n")
			var newLines []string
			inUse := false
			added := false
			for _, line := range lines {
				if strings.Contains(line, "use (") {
					inUse = true
				}
				if inUse && strings.Contains(line, ")") && !added {
					newLines = append(newLines, "\t./"+pkgName)
					added = true
				}
				newLines = append(newLines, line)
			}
			_ = os.WriteFile(workPath, []byte(strings.Join(newLines, "\n")), 0644)
		}
	}

	fmt.Printf("Successfully scaffolded module '%s'!\n", moduleName)
	return nil
}

func runInstall(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: hyperrr install <git-package-url>")
		return fmt.Errorf("missing package path")
	}

	pkgPath := args[0]
	fmt.Printf("Installing module '%s'...\n", pkgPath)

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// 1. Run 'go get <package>'
	fmt.Println("Fetching dependencies via 'go get'...")
	cmd := exec.Command("go", "get", pkgPath)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch package: %w", err)
	}

	// 2. Append to cmd/hyperrr/imports.go
	importsFile := filepath.Join(root, "cmd", "hyperrr", "imports.go")
	if err := addImport(importsFile, pkgPath); err != nil {
		return fmt.Errorf("failed to write imports: %w", err)
	}

	// 3. Rebuild
	return rebuildBinary(root)
}

func runUninstall(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: hyperrr uninstall <package-url>")
		return fmt.Errorf("missing package path")
	}

	pkgPath := args[0]
	fmt.Printf("Uninstalling module '%s'...\n", pkgPath)

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// 1. Remove from cmd/hyperrr/imports.go
	importsFile := filepath.Join(root, "cmd", "hyperrr", "imports.go")
	if err := removeImport(importsFile, pkgPath); err != nil {
		return fmt.Errorf("failed to remove import: %w", err)
	}

	// 2. Run 'go mod tidy'
	fmt.Println("Cleaning go.mod via 'go mod tidy'...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tidy module: %w", err)
	}

	// 3. Rebuild
	return rebuildBinary(root)
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find project root (missing go.mod)")
}

func addImport(filename, pkgPath string) error {
	var lines []string
	if _, err := os.Stat(filename); err == nil {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	} else {
		// Create basic skeleton
		lines = []string{
			"package main",
			"",
			"// Generated by hyperrr. DO NOT EDIT.",
			"import (",
			")",
		}
	}

	// Check if already imported
	importLine := fmt.Sprintf("\t_ \"%s\"", pkgPath)
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(importLine) {
			fmt.Println("Module is already installed in imports list.")
			return nil
		}
	}

	// Insert import
	var newLines []string
	inserted := false
	for _, l := range lines {
		if strings.Contains(l, ")") && !inserted {
			newLines = append(newLines, importLine)
			inserted = true
		}
		newLines = append(newLines, l)
	}

	if !inserted {
		newLines = append(newLines, "import (", importLine, ")")
	}

	return os.WriteFile(filename, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func removeImport(filename, pkgPath string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	var newLines []string
	scanner := bufio.NewScanner(file)
	escapedPkg := regexp.QuoteMeta(pkgPath)
	r := regexp.MustCompile(`_.*"` + escapedPkg + `"`)

	for scanner.Scan() {
		line := scanner.Text()
		if r.MatchString(line) {
			continue
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(filename, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
}

func rebuildBinary(root string) error {
	fmt.Println("Recompiling Hyperrr binary with updated imports...")
	binName := "hyperrr"
	if filepath.Separator == '\\' {
		binName = "hyperrr.exe"
	}
	
	binDir := filepath.Join(root, "bin")
	_ = os.MkdirAll(binDir, 0755)

	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, binName), "./cmd/hyperrr")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compilation failed: %w", err)
	}

	fmt.Printf("\n🚀 Rebuild completed! Recompiled binary saved to: bin/%s\n\n", binName)
	return nil
}
