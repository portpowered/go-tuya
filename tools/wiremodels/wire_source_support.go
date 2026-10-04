package main

import (
	"errors"
	"fmt"
	"go/ast"
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
	Name string
	File string
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

func generatedWireModelIndex(generatedFiles map[string]string) map[string]generatedModel {
	models := make(map[string]generatedModel, len(generatedFiles))
	for name, path := range generatedFiles {
		models[name] = generatedModel{Name: name, File: filepath.ToSlash(path)}
	}

	return models
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

	models := generatedWireModelIndex(generatedFiles)
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
