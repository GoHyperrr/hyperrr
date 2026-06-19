package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModuleScaffoldConfig holds inputs for creating a new custom module.
type ModuleScaffoldConfig struct {
	ModuleName  string
	ProjectRoot string
	IsWorkspace bool // true if we have a go.work at root
}

func CreateModule(cfg *ModuleScaffoldConfig) error {
	pkgName := strings.ToLower(filepath.Base(cfg.ModuleName))
	
	var moduleDir string
	var importPath string
	
	if cfg.IsWorkspace {
		moduleDir = filepath.Join(cfg.ProjectRoot, pkgName)
		importPath = "github.com/GoHyperrr/" + pkgName
	} else {
		// Local project module under modules/ directory
		modulesDir := filepath.Join(cfg.ProjectRoot, "modules")
		if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
			modulesDir = cfg.ProjectRoot // fallback to root if no modules dir
		}
		moduleDir = filepath.Join(modulesDir, pkgName)
		
		// Find main module path from go.mod
		mainMod, err := GetModuleName(cfg.ProjectRoot)
		if err == nil {
			if modulesDir == cfg.ProjectRoot {
				importPath = mainMod + "/" + pkgName
			} else {
				importPath = mainMod + "/modules/" + pkgName
			}
		} else {
			importPath = pkgName
		}
	}
	
	fmt.Printf("Scaffolding new module '%s' in '%s'...\n", cfg.ModuleName, moduleDir)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 1. If workspace, write go.mod
	if cfg.IsWorkspace {
		goModContent := fmt.Sprintf(`module github.com/GoHyperrr/%s

go 1.26

require (
	github.com/GoHyperrr/mdk v0.3.0
)
`, pkgName)
		if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goModContent), 0644); err != nil {
			return err
		}
	}

	// 2. Write module.go
	moduleContent := fmt.Sprintf(`package %s

import (
	"context"

	"github.com/GoHyperrr/mdk"
)

// Module implements the mdk.Module interface.
type Module struct{}

func NewModule() *Module {
	return &Module{}
}

func init() {
	mdk.Register(func() mdk.Module { return NewModule() })
}

func (m *Module) ID() string {
	return "plugin.%s"
}

func (m *Module) Init(ctx context.Context, rt mdk.Runtime) error {
	// Initialize resources and hooks here
	return nil
}

func (m *Module) Shutdown(ctx context.Context) error {
	return nil
}

func (m *Module) Models() []any {
	return []any{}
}

func (m *Module) Routes() []mdk.Route {
	return nil
}
`, pkgName, pkgName)
	if err := os.WriteFile(filepath.Join(moduleDir, "module.go"), []byte(moduleContent), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(moduleDir, "models.go"), []byte(modelsContent), 0644); err != nil {
		return err
	}

	// 4. Write graphql.go
	graphqlContent := fmt.Sprintf(`package %s

func (m *Module) Queries() map[string]any {
	return map[string]any{
		// "myQueryField": m.MyQueryResolver,
	}
}

func (m *Module) Mutations() map[string]any {
	return map[string]any{
		// "myMutationField": m.MyMutationResolver,
	}
}

func (m *Module) FieldResolvers() map[string]any {
	return nil
}
`, pkgName)
	if err := os.WriteFile(filepath.Join(moduleDir, "graphql.go"), []byte(graphqlContent), 0644); err != nil {
		return err
	}

	// 5. Write <pkgName>.graphqls
	titlePkg := strings.ToUpper(pkgName[:1]) + pkgName[1:]
	schemaContent := fmt.Sprintf(`# GraphQL schema for the %s module

# extend type Query {
#   hello%s: String!
# }
`, pkgName, titlePkg)
	if err := os.WriteFile(filepath.Join(moduleDir, pkgName+".graphqls"), []byte(schemaContent), 0644); err != nil {
		return err
	}

	// 6. Register in go.work if workspace
	if cfg.IsWorkspace {
		workPath := filepath.Join(cfg.ProjectRoot, "go.work")
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
	}

	fmt.Printf("Successfully scaffolded module '%s'!\n", cfg.ModuleName)
	fmt.Printf("Import path: %s\n", importPath)
	return nil
}

// GetModuleName extracts the module name defined in a go.mod file.
func GetModuleName(dir string) (string, error) {
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
