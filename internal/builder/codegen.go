package builder

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type ModuleInfo struct {
	Name       string
	ImportPath string
	StructName string
	Queries    map[string]string
	Mutations  map[string]string
	Fields     map[string]map[string]string
}

type ResolverMethod struct {
	Name      string
	Params    string
	Results   string
	CleanName string
}

type ResolverInterface struct {
	Name    string
	Methods []ResolverMethod
}

// RunCodegen scans modules implementing GraphQLProvider, aggregates queries/mutations/fields, and regenerates resolver implementations.
func RunCodegen() error {
	fmt.Println("Scanning for GraphQLProvider modules...")
	modules, err := discoverModules()
	if err != nil {
		return fmt.Errorf("failed to discover modules: %w", err)
	}

	fmt.Printf("Found %d GraphQLProvider modules:\n", len(modules))
	for _, mod := range modules {
		fmt.Printf("  - %s (%s)\n", mod.Name, mod.ImportPath)
	}

	generatedFile := filepath.Join("api", "graph", "generated.go")
	if _, err := os.Stat(generatedFile); os.IsNotExist(err) {
		return fmt.Errorf("gqlgen generated.go not found at %s. Please run gqlgen generate first", generatedFile)
	}

	fmt.Println("Parsing resolver interfaces from generated.go...")
	interfaces, err := parseResolverInterfaces(generatedFile)
	if err != nil {
		return fmt.Errorf("failed to parse resolver interfaces: %w", err)
	}

	// Generate api/graph/resolver.go
	if err := generateResolverStruct(modules, interfaces); err != nil {
		return fmt.Errorf("failed to generate resolver.go: %w", err)
	}

	// Generate api/graph/resolvers_impl.go
	if err := generateResolversImpl(modules, interfaces); err != nil {
		return fmt.Errorf("failed to generate resolvers_impl.go: %w", err)
	}

	return nil
}

func discoverModules() ([]ModuleInfo, error) {
	var modules []ModuleInfo

	type scanItem struct {
		root   string
		prefix string
	}
	var scanPaths []scanItem

	// Discover Go modules via go list
	if goMods, err := getGoModules(); err == nil && len(goMods) > 0 {
		for _, mod := range goMods {
			scanPaths = append(scanPaths, scanItem{
				root:   mod.Dir,
				prefix: mod.Path + "/",
			})
		}
	} else {
		// Fallback to go.work workspace parsing
		if workDir, err := findWorkspaceRoot(); err == nil {
			if wsModules, err := getWorkspaceModules(workDir); err == nil {
				for _, mPath := range wsModules {
					if filepath.Base(mPath) == "hyperrr" {
						continue
					}
					modName, err := getModuleName(mPath)
					if err == nil {
						scanPaths = append(scanPaths, scanItem{
							root:   mPath,
							prefix: modName + "/",
						})
					}
				}
			}
		}
	}

	// Always scan local folders in hyperrr core
	scanPaths = append(scanPaths,
		scanItem{root: "pkg", prefix: "github.com/GoHyperrr/hyperrr/pkg/"},
		scanItem{root: "modules", prefix: "github.com/GoHyperrr/hyperrr/modules/"},
	)

	for _, scan := range scanPaths {
		if _, err := os.Stat(scan.root); os.IsNotExist(err) {
			continue
		}

		// Check if the root itself is a package (e.g. flat modules or pkg/ctxengine)
		if scan.root == "pkg" {
			// Special-case ctxengine
			dirPath := filepath.Join(scan.root, "ctxengine")
			if _, err := os.Stat(dirPath); err == nil {
				queries, mutations, fields, err := parseModuleQueriesMutations(dirPath)
				if err == nil && (len(queries) > 0 || len(mutations) > 0 || len(fields) > 0) {
					modules = append(modules, ModuleInfo{
						Name:       "ctxengine",
						ImportPath: "github.com/GoHyperrr/hyperrr/pkg/ctxengine",
						StructName: "Module",
						Queries:    queries,
						Mutations:  mutations,
						Fields:     fields,
					})
				}
			}
			continue
		}

		// Check if the root itself contains queries/mutations (for flat modules)
		queries, mutations, fields, err := parseModuleQueriesMutations(scan.root)
		if err == nil && (len(queries) > 0 || len(mutations) > 0 || len(fields) > 0) {
			pkgName := filepath.Base(scan.root)
			importPath := strings.TrimSuffix(scan.prefix, "/")
			modules = append(modules, ModuleInfo{
				Name:       pkgName,
				ImportPath: importPath,
				StructName: "Module",
				Queries:    queries,
				Mutations:  mutations,
				Fields:     fields,
			})
		}

		infos, err := os.ReadDir(scan.root)
		if err != nil {
			return nil, err
		}

		for _, info := range infos {
			if !info.IsDir() {
				continue
			}
			if strings.HasPrefix(info.Name(), ".") {
				continue
			}

			dirPath := filepath.Join(scan.root, info.Name())
			queries, mutations, fields, err := parseModuleQueriesMutations(dirPath)
			if err != nil {
				continue
			}

			if len(queries) == 0 && len(mutations) == 0 && len(fields) == 0 {
				continue
			}

			pkgName := info.Name()
			importPath := scan.prefix + pkgName

			modules = append(modules, ModuleInfo{
				Name:       pkgName,
				ImportPath: importPath,
				StructName: "Module",
				Queries:    queries,
				Mutations:  mutations,
				Fields:     fields,
			})
		}
	}

	return modules, nil
}

func parseModuleQueriesMutations(dir string) (map[string]string, map[string]string, map[string]map[string]string, error) {
	queries := make(map[string]string)
	mutations := make(map[string]string)
	fields := make(map[string]map[string]string)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, 0)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
					continue
				}
				recvType := funcDecl.Recv.List[0].Type
				if starExpr, ok := recvType.(*ast.StarExpr); ok {
					recvType = starExpr.X
				}
				ident, ok := recvType.(*ast.Ident)
				if !ok || ident.Name != "Module" {
					continue
				}

				methodName := funcDecl.Name.Name
				if methodName == "Queries" || methodName == "Mutations" || methodName == "FieldResolvers" {
					if funcDecl.Body == nil {
						continue
					}
					for _, stmt := range funcDecl.Body.List {
						retStmt, ok := stmt.(*ast.ReturnStmt)
						if !ok || len(retStmt.Results) == 0 {
							continue
						}
						compLit, ok := retStmt.Results[0].(*ast.CompositeLit)
						if !ok {
							if id, isIdent := retStmt.Results[0].(*ast.Ident); isIdent && id.Name == "nil" {
								continue
							}
							fmt.Printf("WARNING: Method %s in %s returns non-composite literal (%T). Resolver mapping will be skipped in codegen. Please return inline map[string]any{...} directly.\n", methodName, dir, retStmt.Results[0])
							continue
						}
						for _, elt := range compLit.Elts {
							kvExpr, ok := elt.(*ast.KeyValueExpr)
							if !ok {
								continue
							}
							keyStr := ""
							if basicLit, ok := kvExpr.Key.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
								keyStr = strings.Trim(basicLit.Value, `"` + "`")
							}
							if keyStr == "" {
								continue
							}

							valStr := ""
							switch valExpr := kvExpr.Value.(type) {
							case *ast.SelectorExpr:
								valStr = valExpr.Sel.Name
							case *ast.Ident:
								valStr = valExpr.Name
							}

							if valStr == "" {
								continue
							}

							if methodName == "Queries" {
								queries[keyStr] = valStr
							} else if methodName == "Mutations" {
								mutations[keyStr] = valStr
							} else if methodName == "FieldResolvers" {
								parts := strings.Split(keyStr, ".")
								if len(parts) == 2 {
									typeName := parts[0]
									fieldName := parts[1]
									if fields[typeName] == nil {
										fields[typeName] = make(map[string]string)
									}
									fields[typeName][fieldName] = valStr
								}
							}
						}
					}
				}
			}
		}
	}

	return queries, mutations, fields, nil
}

func parseResolverInterfaces(filename string) ([]ResolverInterface, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}

	var interfaces []ResolverInterface

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if !strings.HasSuffix(typeSpec.Name.Name, "Resolver") {
				continue
			}
			interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			var methods []ResolverMethod
			for _, field := range interfaceType.Methods.List {
				if len(field.Names) == 0 {
					continue
				}
				methodName := field.Names[0].Name
				funcType, ok := field.Type.(*ast.FuncType)
				if !ok {
					continue
				}

				paramsStr, resultsStr := formatFuncSig(fset, funcType.Params, funcType.Results)

				methods = append(methods, ResolverMethod{
					Name:    methodName,
					Params:  paramsStr,
					Results: resultsStr,
				})
			}

			interfaces = append(interfaces, ResolverInterface{
				Name:    typeSpec.Name.Name,
				Methods: methods,
			})
		}
	}

	return interfaces, nil
}

func generateResolverStruct(modules []ModuleInfo, interfaces []ResolverInterface) error {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by hyperrr build. DO NOT EDIT.\n")
	buf.WriteString("package graph\n\n")

	// Write imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"github.com/GoHyperrr/hyperrr/pkg/registry\"\n")
	buf.WriteString("\t\"github.com/GoHyperrr/hyperrr/pkg/workflow\"\n")
	buf.WriteString("\t\"github.com/GoHyperrr/hyperrr/pkg/ctxengine\"\n")
	buf.WriteString("\t\"github.com/GoHyperrr/mdk\"\n")
	for _, mod := range modules {
		if mod.Name != "ctxengine" {
			buf.WriteString(fmt.Sprintf("\t%s \"%s\"\n", mod.Name, mod.ImportPath))
		}
	}
	buf.WriteString(")\n\n")

	// Write Resolver struct
	buf.WriteString("type Resolver struct {\n")
	buf.WriteString("\tProjector          mdk.Projector\n")
	for _, mod := range modules {
		if mod.Name == "ctxengine" {
			buf.WriteString("\tCtxEngineModule    *ctxengine.Module\n")
		} else {
			fieldName := strings.ToUpper(mod.Name[:1]) + mod.Name[1:] + "Module"
			buf.WriteString(fmt.Sprintf("\t%s *%s.Module\n", fieldName, mod.Name))
		}
	}
	buf.WriteString("\tRunner             *workflow.Runner\n")
	buf.WriteString("\tRegistry           *workflow.Registry\n")
	buf.WriteString("}\n\n")

	// Write interface resolver methods
	for _, iface := range interfaces {
		rootMethodName := strings.TrimSuffix(iface.Name, "Resolver")
		resolverStructName := strings.ToLower(rootMethodName[:1]) + rootMethodName[1:] + "Resolver"
		buf.WriteString(fmt.Sprintf("func (r *Resolver) %s() %s { return &%s{r} }\n", rootMethodName, iface.Name, resolverStructName))
	}
	buf.WriteString("\n")

	// Write thin resolver struct types
	for _, iface := range interfaces {
		rootMethodName := strings.TrimSuffix(iface.Name, "Resolver")
		resolverStructName := strings.ToLower(rootMethodName[:1]) + rootMethodName[1:] + "Resolver"
		buf.WriteString(fmt.Sprintf("type %s struct{ *Resolver }\n", resolverStructName))
	}
	buf.WriteString("\n")

	// Write NewResolver constructor
	buf.WriteString("func NewResolver(modules []registry.Module, runner *workflow.Runner, registryStore *workflow.Registry, projector mdk.Projector) *Resolver {\n")
	buf.WriteString("\tres := &Resolver{\n")
	buf.WriteString("\t\tRunner:    runner,\n")
	buf.WriteString("\t\tRegistry:  registryStore,\n")
	buf.WriteString("\t\tProjector: projector,\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tfor _, m := range modules {\n")
	buf.WriteString("\t\tswitch m.ID() {\n")
	for _, mod := range modules {
		if mod.Name == "ctxengine" {
			buf.WriteString("\t\tcase \"core.context\":\n")
			buf.WriteString("\t\t\tres.CtxEngineModule = m.(*ctxengine.Module)\n")
		} else {
			prefix := "commerce."
			if strings.Contains(mod.ImportPath, "/auth/") {
				prefix = "auth."
			} else if !strings.Contains(mod.ImportPath, "/commerce/") {
				prefix = ""
			}
			moduleID := prefix + mod.Name
			fieldName := strings.ToUpper(mod.Name[:1]) + mod.Name[1:] + "Module"
			buf.WriteString(fmt.Sprintf("\t\tcase \"%s\":\n", moduleID))
			buf.WriteString(fmt.Sprintf("\t\t\tres.%s = m.(*%s.Module)\n", fieldName, mod.Name))
		}
	}
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn res\n")
	buf.WriteString("}\n")

	return os.WriteFile(filepath.Join("api", "graph", "resolver.go"), buf.Bytes(), 0644)
}

func generateResolversImpl(modules []ModuleInfo, interfaces []ResolverInterface) error {
	var methodsBuf bytes.Buffer

	for _, iface := range interfaces {
		rootMethodName := strings.TrimSuffix(iface.Name, "Resolver")
		resolverStructName := strings.ToLower(rootMethodName[:1]) + rootMethodName[1:] + "Resolver"

		for _, method := range iface.Methods {
			found := false
			var targetMod ModuleInfo
			var targetMethod string

			if iface.Name == "QueryResolver" {
				for _, mod := range modules {
					for qKey, qVal := range mod.Queries {
						if strings.EqualFold(qKey, method.Name) || (strings.HasPrefix(qKey, "_") && strings.EqualFold(qKey[1:], method.Name)) {
							found = true
							targetMod = mod
							targetMethod = qVal
							break
						}
					}
					if found {
						break
					}
				}
			} else if iface.Name == "MutationResolver" {
				for _, mod := range modules {
					for mKey, mVal := range mod.Mutations {
						if strings.EqualFold(mKey, method.Name) || (strings.HasPrefix(mKey, "_") && strings.EqualFold(mKey[1:], method.Name)) {
							found = true
							targetMod = mod
							targetMethod = mVal
							break
						}
					}
					if found {
						break
					}
				}
			} else {
				typeName := strings.TrimSuffix(iface.Name, "Resolver")
				for _, mod := range modules {
					if typeFields, ok := mod.Fields[typeName]; ok {
						for fKey, fVal := range typeFields {
							if strings.EqualFold(fKey, method.Name) || (strings.HasPrefix(fKey, "_") && strings.EqualFold(fKey[1:], method.Name)) {
								found = true
								targetMod = mod
								targetMethod = fVal
								break
							}
						}
					}
					if found {
						break
					}
				}
			}

			methodsBuf.WriteString(fmt.Sprintf("func (r *%s) %s(%s) %s {\n", resolverStructName, method.Name, method.Params, method.Results))

			if iface.Name == "ActorResolver" {
				if method.Name == "ID" {
					methodsBuf.WriteString("\treturn obj.GetID(), nil\n")
				} else if method.Name == "Type" {
					methodsBuf.WriteString("\treturn string(obj.GetType()), nil\n")
				} else if method.Name == "Name" {
					methodsBuf.WriteString("\treturn obj.GetName(), nil\n")
				} else {
					methodsBuf.WriteString(fmt.Sprintf("\tpanic(fmt.Errorf(\"not implemented: %s\"))\n", method.Name))
				}
			} else if found {
				var moduleRef string
				if targetMod.Name == "ctxengine" {
					moduleRef = "r.CtxEngineModule"
				} else {
					moduleRef = "r." + strings.ToUpper(targetMod.Name[:1]) + targetMod.Name[1:] + "Module"
				}

				paramNames := extractParamNames(method.Params)

				methodsBuf.WriteString(fmt.Sprintf("\tif %s == nil {\n", moduleRef))
				if strings.Contains(method.Results, "error") {
					zeroValsExceptError := getZeroReturnStringExceptLast(method.Results)
					if zeroValsExceptError != "" {
						methodsBuf.WriteString(fmt.Sprintf("\t\treturn %s, fmt.Errorf(\"module %s not loaded\")\n", zeroValsExceptError, targetMod.Name))
					} else {
						methodsBuf.WriteString(fmt.Sprintf("\t\treturn fmt.Errorf(\"module %s not loaded\")\n", targetMod.Name))
					}
				} else {
					methodsBuf.WriteString(fmt.Sprintf("\t\treturn %s\n", getZeroReturnString(method.Results)))
				}
				methodsBuf.WriteString("\t}\n")

				methodsBuf.WriteString(fmt.Sprintf("\treturn %s.%s(%s)\n", moduleRef, targetMethod, paramNames))
			} else {
				methodsBuf.WriteString(fmt.Sprintf("\tpanic(fmt.Errorf(\"not implemented: %s\"))\n", method.Name))
			}

			methodsBuf.WriteString("}\n\n")
		}
	}

	methodsStr := methodsBuf.String()

	var buf bytes.Buffer
	buf.WriteString("// Code generated by hyperrr build. DO NOT EDIT.\n")
	buf.WriteString("package graph\n\n")

	// Write imports
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"fmt\"\n")
	buf.WriteString("\t\"time\"\n")
	buf.WriteString("\t\"github.com/GoHyperrr/hyperrr/api/graph/model\"\n")
	buf.WriteString("\t\"github.com/GoHyperrr/mdk\"\n")
	for _, mod := range modules {
		if mod.Name != "ctxengine" {
			if strings.Contains(methodsStr, mod.Name+".") {
				buf.WriteString(fmt.Sprintf("\t%s \"%s\"\n", mod.Name, mod.ImportPath))
			}
		}
	}
	buf.WriteString(")\n\n")

	buf.WriteString(methodsStr)

	return os.WriteFile(filepath.Join("api", "graph", "resolvers_impl.go"), buf.Bytes(), 0644)
}

func extractParamNames(paramsStr string) string {
	if paramsStr == "" {
		return ""
	}
	parts := strings.Split(paramsStr, ",")
	var names []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		words := strings.Fields(part)
		if len(words) > 0 {
			names = append(names, words[0])
		}
	}
	return strings.Join(names, ", ")
}

func getZeroReturnString(resultsStr string) string {
	return getZeroReturnStringInternal(resultsStr, false)
}

func getZeroReturnStringExceptLast(resultsStr string) string {
	return getZeroReturnStringInternal(resultsStr, true)
}

func getZeroReturnStringInternal(resultsStr string, exceptLast bool) string {
	resultsStr = strings.TrimSpace(resultsStr)
	resultsStr = strings.TrimPrefix(resultsStr, "(")
	resultsStr = strings.TrimSuffix(resultsStr, ")")

	parts := strings.Split(resultsStr, ",")
	if len(parts) == 0 {
		return ""
	}

	limit := len(parts)
	if exceptLast {
		limit = len(parts) - 1
	}

	var zeroVals []string
	for i := 0; i < limit; i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if part == "error" {
			zeroVals = append(zeroVals, "nil")
			continue
		}

		if strings.HasPrefix(part, "*") || strings.HasPrefix(part, "[]") || strings.HasPrefix(part, "map[") || part == "mdk.Actor" {
			zeroVals = append(zeroVals, "nil")
		} else if part == "string" {
			zeroVals = append(zeroVals, `""`)
		} else if part == "int" || part == "int64" || part == "float64" {
			zeroVals = append(zeroVals, "0")
		} else if part == "bool" {
			zeroVals = append(zeroVals, "false")
		} else {
			zeroVals = append(zeroVals, part+"{}")
		}
	}
	return strings.Join(zeroVals, ", ")
}

func formatNode(fset *token.FileSet, node any) string {
	var buf strings.Builder
	err := printer.Fprint(&buf, fset, node)
	if err != nil {
		fmt.Printf("printer.Fprint error for node %T: %v\n", node, err)
	}
	return buf.String()
}

func formatFuncSig(fset *token.FileSet, params *ast.FieldList, results *ast.FieldList) (string, string) {
	dummyFunc := &ast.FuncType{
		Params:  params,
		Results: results,
	}
	sigStr := formatNode(fset, dummyFunc)
	sigStr = strings.TrimPrefix(sigStr, "func")

	depth := 0
	splitIdx := -1
	for i, char := range sigStr {
		if char == '(' {
			depth++
		} else if char == ')' {
			depth--
			if depth == 0 {
				splitIdx = i
				break
			}
		}
	}

	if splitIdx == -1 {
		return "", ""
	}

	paramsStr := sigStr[1:splitIdx]
	resultsStr := strings.TrimSpace(sigStr[splitIdx+1:])
	return paramsStr, resultsStr
}
