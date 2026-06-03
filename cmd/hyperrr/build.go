package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func runBuild() error {
	fmt.Println("=== Hyperrr Core Builder ===")

	// 1. Clean and recreate the schema_gen directory
	schemaGenDir := filepath.Join("api", "graph", "schema_gen")
	fmt.Printf("Cleaning schema generation cache: %s\n", schemaGenDir)
	_ = os.RemoveAll(schemaGenDir)
	if err := os.MkdirAll(schemaGenDir, 0755); err != nil {
		return fmt.Errorf("failed to create schema_gen directory: %w", err)
	}

	// 2. Scan and aggregate schemas
	// We scan ../commerce and modules for any .graphqls files
	type scanItem struct {
		src  string
		name string
	}
	var scanPaths []scanItem

	// Dynamically discover external workspace modules using go.work
	if workDir, err := findWorkspaceRoot(); err == nil {
		if modules, err := getWorkspaceModules(workDir); err == nil {
			for _, mPath := range modules {
				if filepath.Base(mPath) == "hyperrr" {
					continue
				}
				scanPaths = append(scanPaths, scanItem{
					src:  mPath,
					name: filepath.Base(mPath),
				})
			}
		}
	}

	// Always scan local folders in hyperrr core
	scanPaths = append(scanPaths,
		scanItem{src: "modules", name: "modules"},
		scanItem{src: "pkg", name: "pkg"},
		scanItem{src: "internal", name: "internal"},
	)

	for _, scan := range scanPaths {
		if _, err := os.Stat(scan.src); os.IsNotExist(err) {
			fmt.Printf("Skipping path %s (does not exist)\n", scan.src)
			continue
		}

		fmt.Printf("Scanning %s for GraphQL schemas...\n", scan.src)
		err := filepath.Walk(scan.src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && filepath.Ext(path) == ".graphqls" {
				// Copy to schemaGenDir
				rel, err := filepath.Rel(scan.src, path)
				if err != nil {
					return err
				}
				destPath := filepath.Join(schemaGenDir, scan.name, rel)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}
				if err := copyFile(path, destPath); err != nil {
					return fmt.Errorf("failed to copy %s to %s: %w", path, destPath, err)
				}
				fmt.Printf("  Added: %s -> %s\n", filepath.Base(path), destPath)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error scanning %s: %w", scan.src, err)
		}
	}

	// 3. Run gqlgen code generation
	fmt.Println("Running gqlgen schema and resolver generation...")
	cmdGen := exec.Command("go", "run", "github.com/99designs/gqlgen", "generate", "--config", filepath.Join("api", "gqlgen.yml"))
	cmdGen.Stdout = os.Stdout
	cmdGen.Stderr = os.Stderr
	if err := cmdGen.Run(); err != nil {
		return fmt.Errorf("gqlgen generation failed: %w", err)
	}
	fmt.Println("GraphQL code generated successfully!")

	// 3.5. Run custom resolver codegen
	fmt.Println("Running custom resolver codegen...")
	if err := runCodegen(); err != nil {
		return fmt.Errorf("custom resolver codegen failed: %w", err)
	}
	fmt.Println("Custom resolver code generated successfully!")

	// 4. Run go build
	fmt.Println("Compiling binary...")
	binaryPath := filepath.Join("bin", "hyperrr")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}
	cmdBuild := exec.Command("go", "build", "-o", binaryPath, "./cmd/hyperrr")
	cmdBuild.Stdout = os.Stdout
	cmdBuild.Stderr = os.Stderr
	if err := cmdBuild.Run(); err != nil {
		return fmt.Errorf("compilation failed: %w", err)
	}
	fmt.Printf("Build successful! Binary written to: %s\n", binaryPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
