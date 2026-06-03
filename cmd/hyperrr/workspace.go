package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// findWorkspaceRoot searches upwards from the current working directory for a go.work file.
func findWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find workspace root (missing go.work)")
}

// getWorkspaceModules parses the go.work file and returns absolute paths to each module.
func getWorkspaceModules(workDir string) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(workDir, "go.work"))
	if err != nil {
		return nil, err
	}

	var paths []string
	lines := strings.Split(string(content), "\n")
	inUseBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}
		if inUseBlock && line == ")" {
			inUseBlock = false
			continue
		}
		if inUseBlock {
			path := strings.Trim(line, `"'`)
			paths = append(paths, filepath.Clean(filepath.Join(workDir, path)))
		} else if strings.HasPrefix(line, "use ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "use"))
			path = strings.Trim(path, `"'`)
			paths = append(paths, filepath.Clean(filepath.Join(workDir, path)))
		}
	}
	return paths, nil
}

// getModuleName extracts the module name defined in a go.mod file.
func getModuleName(dir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("module name not found in go.mod in %s", dir)
}
