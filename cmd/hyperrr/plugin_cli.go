package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/GoHyperrr/hyperrr/internal/builder"
	"github.com/GoHyperrr/hyperrr/internal/scaffold"
	"github.com/GoHyperrr/hyperrr/pkg/registry"
	"gopkg.in/yaml.v3"
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

		fmt.Printf("  • %-20s [Models: %d]\n",
			m.ID(), modelsCount)
	}
	fmt.Println()
	return nil
}

func runCreate(args []string) error {
	if len(args) < 2 || args[0] != "module" {
		fmt.Println("Usage: hyperrr module create <name>")
		return fmt.Errorf("invalid arguments")
	}

	moduleName := args[1]
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// Check if go.work exists at project root to determine if it is core workspace
	isWorkspace := false
	if _, err := os.Stat(filepath.Join(root, "go.work")); err == nil {
		isWorkspace = true
	}

	scConfig := &scaffold.ModuleScaffoldConfig{
		ModuleName:  moduleName,
		ProjectRoot: root,
		IsWorkspace: isWorkspace,
	}

	return scaffold.CreateModule(scConfig)
}

func runInstall(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: hyperrr module add <module-name>")
		return fmt.Errorf("missing module name")
	}

	userInput := args[0]

	// 1. Resolve module shorthand name
	resolved, ok := scaffold.ResolveModule(userInput)
	if !ok {
		return fmt.Errorf("could not resolve module name: %s", userInput)
	}

	fmt.Printf("Resolving '%s' to module ID '%s' (%s)...\n", userInput, resolved.ID, resolved.Resolve)

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// 2. Fetch package via go get (only if it's a remote URL and not a local module path)
	if strings.Contains(resolved.Resolve, "/") && !strings.HasPrefix(resolved.Resolve, "./") && !strings.HasPrefix(resolved.Resolve, "../") {
		fmt.Printf("Fetching package '%s' via 'go get'...\n", resolved.Resolve)
		cmd := exec.Command("go", "get", resolved.Resolve+"@latest")
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fetch package: %w", err)
		}
	}

	// 3. Add to configuration file
	cfgPath := findConfigFileInProject(root)
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Updating configuration file: %s...\n", cfgPath)
		if err := updateConfigModulesList(cfgPath, resolved, true); err != nil {
			return fmt.Errorf("failed to update config file: %w", err)
		}
	} else {
		fmt.Printf("Warning: Config file not found at %s. Skipping configuration registration.\n", cfgPath)
	}

	// 4. Update imports_generated.go
	importsFile := findImportsFile(root)
	fmt.Printf("Updating imports in: %s...\n", importsFile)
	if err := addImport(importsFile, resolved.Resolve); err != nil {
		return fmt.Errorf("failed to add import: %w", err)
	}

	// 5. Rebuild
	return rebuildBinary(root)
}

func runUninstall(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: hyperrr module remove <module-name>")
		return fmt.Errorf("missing module name")
	}

	userInput := args[0]
	resolved, ok := scaffold.ResolveModule(userInput)
	if !ok {
		// Fallback to treat input directly as package path/ID if resolution fails
		resolved = scaffold.ModuleInfo{ID: userInput, Resolve: userInput}
	}

	fmt.Printf("Removing module '%s'...\n", resolved.ID)

	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// 1. Remove from configuration file
	cfgPath := findConfigFileInProject(root)
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Updating configuration file: %s...\n", cfgPath)
		if err := updateConfigModulesList(cfgPath, resolved, false); err != nil {
			return fmt.Errorf("failed to update config file: %w", err)
		}
	}

	// 2. Remove from imports file
	importsFile := findImportsFile(root)
	fmt.Printf("Removing imports from: %s...\n", importsFile)
	if err := removeImport(importsFile, resolved.Resolve); err != nil {
		return fmt.Errorf("failed to remove import: %w", err)
	}

	// 3. Run 'go mod tidy'
	fmt.Println("Cleaning go.mod via 'go mod tidy'...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tidy module: %w", err)
	}

	// 4. Rebuild
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

func findConfigFileInProject(root string) string {
	paths := []string{
		filepath.Join(root, "configs", "hyperrr.yml"),
		filepath.Join(root, "configs", "hyperrr.yaml"),
		filepath.Join(root, "hyperrr.yml"),
		filepath.Join(root, "hyperrr.yaml"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(root, "configs", "hyperrr.yml")
}

func findImportsFile(root string) string {
	paths := []string{
		filepath.Join(root, "cmd", "server", "imports_generated.go"),
		filepath.Join(root, "cmd", "hyperrr", "imports_generated.go"),
		filepath.Join(root, "cmd", "hyperrr", "imports.go"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(root, "cmd", "server", "imports_generated.go")
}

func updateConfigModulesList(filename string, mod scaffold.ModuleInfo, add bool) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	if len(root.Content) == 0 {
		return fmt.Errorf("empty yaml root")
	}

	mapNode := root.Content[0]
	if mapNode.Kind != yaml.MappingNode {
		return fmt.Errorf("root of YAML is not a map")
	}

	// Find "modules" key
	var modulesNode *yaml.Node
	for i := 0; i < len(mapNode.Content); i += 2 {
		kNode := mapNode.Content[i]
		if strings.EqualFold(kNode.Value, "modules") {
			modulesNode = mapNode.Content[i+1]
			break
		}
	}

	if modulesNode == nil {
		if !add {
			return nil // Nothing to remove
		}
		// Create modules node
		kNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "modules",
		}
		modulesNode = &yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  "!!seq",
		}
		mapNode.Content = append(mapNode.Content, kNode, modulesNode)
	}

	if modulesNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("'modules' key is not a sequence")
	}

	if add {
		// Check if already exists
		exists := false
		for _, entry := range modulesNode.Content {
			if entry.Kind == yaml.MappingNode {
				for j := 0; j < len(entry.Content); j += 2 {
					if entry.Content[j].Value == "resolve" && entry.Content[j+1].Value == mod.Resolve {
						exists = true
						break
					}
				}
			}
		}

		if !exists {
			// Create mapping node for new module
			entryNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			}
			rKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "resolve"}
			rVal := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mod.Resolve}
			iKey := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"}
			iVal := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: mod.ID}

			entryNode.Content = append(entryNode.Content, rKey, rVal, iKey, iVal)
			modulesNode.Content = append(modulesNode.Content, entryNode)
		}
	} else {
		// Remove
		var newContent []*yaml.Node
		for _, entry := range modulesNode.Content {
			match := false
			if entry.Kind == yaml.MappingNode {
				for j := 0; j < len(entry.Content); j += 2 {
					k := entry.Content[j].Value
					v := entry.Content[j+1].Value
					if (k == "resolve" && v == mod.Resolve) || (k == "id" && v == mod.ID) {
						match = true
						break
					}
				}
			}
			if !match {
				newContent = append(newContent, entry)
			}
		}
		modulesNode.Content = newContent
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&root); err != nil {
		return err
	}
	_ = encoder.Close()

	return os.WriteFile(filename, buf.Bytes(), 0644)
}

func addImport(filename, pkgPath string) error {
	pkgPath = strings.Trim(pkgPath, "\"`'` \t\n\r")
	pkgPath = strings.ReplaceAll(pkgPath, "\n", "")
	pkgPath = strings.ReplaceAll(pkgPath, "\r", "")

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
		lines = []string{
			"package main",
			"",
			"// Generated by hyperrr. DO NOT EDIT.",
			"import (",
			")",
		}
	}

	importLine := fmt.Sprintf("\t_ \"%s\"", pkgPath)
	for _, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(importLine) {
			fmt.Println("Module is already installed in imports list.")
			return nil
		}
	}

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
	return builder.RunBuild()
}
