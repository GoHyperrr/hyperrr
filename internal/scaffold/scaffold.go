package scaffold

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

type ScaffoldConfig struct {
	ProjectName      string
	ProjectPath      string
	ModulePath       string
	PresetName       string
	DBDriver         string
	DBDSN            string
	EventBusProvider string
	Modules          []ModuleInfo
	SkipGit          bool
}

func Run(cfg *ScaffoldConfig) error {
	// 1. Resolve preset if specified
	if cfg.PresetName != "" {
		preset, ok := Presets[cfg.PresetName]
		if !ok {
			return fmt.Errorf("unknown preset: %s", cfg.PresetName)
		}
		// Merge preset modules if none provided explicitly
		if len(cfg.Modules) == 0 {
			cfg.Modules = preset.Modules
		}
	}

	// Ensure project path exists
	absPath, err := filepath.Abs(cfg.ProjectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute project path: %w", err)
	}

	fmt.Printf("Generating project structure in: %s\n", absPath)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", absPath, err)
	}

	// Define files to generate
	files := []struct {
		name     string
		dir      string
		template string
	}{
		{name: "main.go", dir: filepath.Join("cmd", "server"), template: MainGoTemplate},
		{name: "imports_generated.go", dir: filepath.Join("cmd", "server"), template: ImportsGoTemplate},
		{name: "hyperrr.yml", dir: "configs", template: HyperrrYmlTemplate},
		{name: ".env", dir: "", template: DotEnvTemplate},
		{name: ".env.example", dir: "", template: DotEnvTemplate},
		{name: ".gitignore", dir: "", template: GitIgnoreTemplate},
		{name: "schema.graphqls", dir: filepath.Join("api", "graph"), template: SchemaGraphqlsTemplate},
		{name: "README.md", dir: "", template: ReadmeTemplate},
		{name: "Makefile", dir: "", template: MakefileTemplate},
		{name: "go.mod", dir: "", template: GoModTemplate},
	}

	// Create directories and write files
	for _, f := range files {
		targetDir := filepath.Join(absPath, f.dir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create folder %s: %w", targetDir, err)
		}

		targetFile := filepath.Join(targetDir, f.name)
		fmt.Printf("  • Creating %s...\n", filepath.Join(f.dir, f.name))

		tmpl, err := template.New(f.name).Parse(f.template)
		if err != nil {
			return fmt.Errorf("failed to parse template for %s: %w", f.name, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, cfg); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", f.name, err)
		}

		if err := os.WriteFile(targetFile, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetFile, err)
		}
	}

	// Create placeholder folders
	placeholderDirs := []string{
		filepath.Join(absPath, "modules"),
		filepath.Join(absPath, "bin"),
	}
	for _, dir := range placeholderDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		// Write .gitkeep inside modules to persist in git
		gitkeep := filepath.Join(dir, ".gitkeep")
		if filepath.Base(dir) == "modules" {
			_ = os.WriteFile(gitkeep, []byte(""), 0644)
		}
	}

	// Run git init if not skipped
	if !cfg.SkipGit {
		fmt.Println("Initializing git repository...")
		cmdGit := exec.Command("git", "init")
		cmdGit.Dir = absPath
		_ = cmdGit.Run()
	}

	// Run go mod tidy
	fmt.Println("Running go mod tidy to resolve dependencies...")
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = absPath
	cmdTidy.Stdout = os.Stdout
	cmdTidy.Stderr = os.Stderr
	if err := cmdTidy.Run(); err != nil {
		fmt.Printf("Warning: 'go mod tidy' failed: %v. You may need to run it manually.\n", err)
	}

	fmt.Println("\n🚀 Project successfully scaffolded!")
	return nil
}
