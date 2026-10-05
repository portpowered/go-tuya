package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var errWireSourceProvenance = errors.New("generated wire source provenance violation")

const (
	wireImportPath             = generatedModelImport
	schemaStringType           = stringSchemaType
	generatedModelDirectory    = "pkg/dependencymodels"
	generatedModelPackageName  = "tuyamodels"
	stringsImportPath          = "strings"
	timeImportPath             = "time"
	generatedCommentCheckBytes = 2048
	generatedAuxiliaryFiles    = 3
)

type generatedModel struct {
	Name        string
	File        string
	Alias       string
	Fields      map[string]string
	EnumMembers map[string]string
}

type primitivePackage struct {
	ImportPath string
	Name       string
	Directory  string
}

func primitiveImportAliases(file *ast.File, owner primitivePackage) map[string]bool {
	aliases := make(map[string]bool)

	for _, dependency := range file.Imports {
		path, err := strconv.Unquote(dependency.Path.Value)
		if err != nil || path != owner.ImportPath {
			continue
		}

		name := owner.Name
		if dependency.Name != nil {
			name = dependency.Name.Name
		}

		aliases[name] = true
	}

	return aliases
}

func generatedWireModelIndex(generatedFiles map[string]string) (map[string]generatedModel, error) {
	models := make(map[string]generatedModel, len(generatedFiles))

	for name, path := range generatedFiles {
		models[name] = generatedModel{Name: name, File: filepath.ToSlash(path), Alias: "", Fields: nil, EnumMembers: nil}
	}

	parsed := make(map[string]bool)

	for _, path := range generatedFiles {
		path = filepath.ToSlash(path)
		if parsed[path] {
			continue
		}

		parsed[path] = true

		file, err := parser.ParseFile(token.NewFileSet(), filepath.FromSlash(path), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse generated model metadata %s: %w", path, err)
		}

		indexGeneratedWireTypeMetadata(file, models)
	}

	return models, nil
}

//nolint:cyclop,gocognit // Generated enum values and model fields are indexed in one pass per source file.
func indexGeneratedWireTypeMetadata(file *ast.File, models map[string]generatedModel) {
	for _, declaration := range file.Decls {
		group, isGroup := declaration.(*ast.GenDecl)
		if !isGroup {
			continue
		}

		for _, spec := range group.Specs {
			switch item := spec.(type) {
			case *ast.TypeSpec:
				model, exists := models[item.Name.Name]
				if !exists {
					continue
				}

				if structure, isStruct := item.Type.(*ast.StructType); isStruct && structure.Fields != nil {
					model.Fields = make(map[string]string)

					for _, field := range structure.Fields.List {
						for _, name := range field.Names {
							if typeName := wireEnumTypeExpressionName(field.Type); typeName != "" {
								model.Fields[name.Name] = typeName
							}
						}
					}
				} else if item.Assign.IsValid() {
					model.Alias = wireTypeExpressionName(item.Type)
				}

				models[item.Name.Name] = model
			case *ast.ValueSpec:
				identifier, isEnum := item.Type.(*ast.Ident)
				if !isEnum {
					continue
				}

				model, exists := models[identifier.Name]
				if !exists {
					continue
				}

				if model.EnumMembers == nil {
					model.EnumMembers = make(map[string]string)
				}

				for index, name := range item.Names {
					if index < len(item.Values) {
						model.EnumMembers[name.Name] = literalValue(item.Values[index])
					}
				}

				models[identifier.Name] = model
			}
		}
	}
}

func wireTypeExpressionName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.StarExpr:
		return wireTypeExpressionName(expression.X)
	case *ast.ArrayType:
		return wireTypeExpressionName(expression.Elt)
	case *ast.MapType:
		return wireTypeExpressionName(expression.Value)
	case *ast.ParenExpr:
		return wireTypeExpressionName(expression.X)
	}

	return ""
}

func wireEnumTypeExpressionName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.StarExpr:
		return wireEnumTypeExpressionName(expression.X)
	case *ast.ArrayType:
		return wireEnumTypeExpressionName(expression.Elt)
	case *ast.MapType:
		return wireEnumTypeExpressionName(expression.Value)
	case *ast.ParenExpr:
		return wireEnumTypeExpressionName(expression.X)
	}

	return ""
}

func checkGeneratedWireConstructions(generatedFiles map[string]string) error {
	generated := make(map[string]bool, len(generatedFiles)+generatedAuxiliaryFiles)
	for _, path := range generatedFiles {
		generated[filepath.ToSlash(path)] = true
	}

	for _, path := range []string{
		propertiesGeneratedPath,
		"pkg/dependencymodels/routes.gen.go",
		"pkg/dependencymodels/mqtt.gen.go",
	} {
		generated[path] = true
	}

	models, err := generatedWireModelIndex(generatedFiles)
	if err != nil {
		return err
	}

	for _, root := range []string{"pkg", "cmd", "examples"} {
		err := checkGeneratedWireConstructionRoot(root, generated, models)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkUnregisteredGeneratedModelFiles(generatedFiles map[string]string) error {
	registered := map[string]bool{
		propertiesGeneratedPath:              true,
		"pkg/dependencymodels/routes.gen.go": true,
		"pkg/dependencymodels/mqtt.gen.go":   true,
	}
	for _, path := range generatedFiles {
		registered[filepath.ToSlash(path)] = true
	}

	entries, err := os.ReadDir(generatedModelDirectory)
	if err != nil {
		return fmt.Errorf("read generated model directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		path := generatedModelDirectory + "/" + entry.Name()
		if registered[path] {
			continue
		}

		data, readErr := os.ReadFile(filepath.FromSlash(path))
		if readErr != nil {
			return fmt.Errorf("read model source %s: %w", path, readErr)
		}

		if strings.Contains(string(data[:min(len(data), generatedCommentCheckBytes)]), "Code generated") || strings.HasSuffix(path, ".gen.go") {
			return fmt.Errorf("unregistered generated model source %q: %w", path, errMissingGeneratedType)
		}
	}

	return nil
}

func readWireSource(path string) ([]byte, error) {
	data, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil, fmt.Errorf("read source %s: %w", path, err)
	}

	return data, nil
}
