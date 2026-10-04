package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
)

type wireSourceFile struct {
	path string
	file *ast.File
}

//nolint:cyclop // Walking each production root keeps generated sources and tests excluded consistently.
func checkGeneratedWireConstructionRoot(root string, generated map[string]bool, models map[string]generatedModel) error {
	set := token.NewFileSet()
	packages := make(map[string][]wireSourceFile)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk wire construction sources: %w", walkErr)
		}

		path = filepath.ToSlash(path)
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || generated[path] {
			return nil
		}

		data, readErr := readWireSource(path)
		if readErr != nil {
			return readErr
		}

		file, parseErr := parser.ParseFile(set, path, data, 0)
		if parseErr != nil {
			return fmt.Errorf("parse wire construction source %s: %w", path, parseErr)
		}

		key := filepath.Dir(path) + "/" + file.Name.Name
		packages[key] = append(packages[key], wireSourceFile{path: path, file: file})

		return nil
	})
	if err != nil {
		return fmt.Errorf("collect wire construction sources: %w", err)
	}

	for _, files := range packages {
		err := checkWireSourcePackage(files, set, models)
		if err != nil {
			return err
		}
	}

	return nil
}

func resolveWirePackageNames(files []wireSourceFile) {
	owners := make(map[string]*ast.File)

	for _, source := range files {
		for name := range source.file.Scope.Objects {
			owners[name] = source.file
		}
	}

	for _, source := range files {
		for _, identifier := range source.file.Unresolved {
			owner := owners[identifier.Name]
			if identifier.Obj == nil && owner != nil {
				identifier.Obj = owner.Scope.Objects[identifier.Name]
			}
		}
	}
}

func checkWireSourcePackage(files []wireSourceFile, set *token.FileSet, models map[string]generatedModel) error {
	resolveWirePackageNames(files)

	assignments := wireSourceAssignments{
		values:              make(map[wireSourceVariable][]ast.Expr),
		generatedParameters: make(map[wireSourceVariable]bool),
		globalVariables:     make(map[wireSourceVariable]bool),
		publicInterfaces:    make(map[string]bool),
		urlValuesVariables:  make(map[wireSourceVariable]bool),
		urlValuesFunctions:  make(map[*ast.FuncType]bool),
		callableFields:      make(map[string][]ast.Expr),
		functionBodies:      make(map[*ast.FuncType]*ast.BlockStmt),
		methods:             make(map[string][]*ast.FuncDecl),
	}

	for _, source := range files {
		local := indexWireSourceAssignments(source.file)
		maps.Copy(assignments.globalVariables, local.globalVariables)

		maps.Copy(assignments.publicInterfaces, local.publicInterfaces)

		maps.Copy(assignments.urlValuesVariables, local.urlValuesVariables)

		maps.Copy(assignments.urlValuesFunctions, local.urlValuesFunctions)

		maps.Copy(assignments.functionBodies, local.functionBodies)

		for name, values := range local.callableFields {
			assignments.callableFields[name] = append(assignments.callableFields[name], values...)
		}

		for name, methods := range local.methods {
			assignments.methods[name] = append(assignments.methods[name], methods...)
		}

		for variable, values := range local.values {
			assignments.values[variable] = append(assignments.values[variable], values...)
		}
	}

	indexWirePackageHelperParameters(files, models, assignments)

	for _, source := range files {
		err := rejectRawGeneratedWireConstructionsWithAssignments(source.file, set, source.path, models, assignments)
		if err != nil {
			return fmt.Errorf("check generated wire construction: %w", err)
		}
	}

	return nil
}

func indexWirePackageHelperParameters(files []wireSourceFile, models map[string]generatedModel, assignments wireSourceAssignments) {
	owner := primitivePackage{ImportPath: wireImportPath, Name: generatedModelPackageName, Directory: generatedModelDirectory}

	for {
		before := len(assignments.generatedParameters) + len(assignments.values)

		for _, source := range files {
			aliases := primitiveImportAliases(source.file, owner)
			indexWireHelperParameters(source.file, aliases, source.path, models, assignments)
		}

		if before == len(assignments.generatedParameters)+len(assignments.values) {
			return
		}
	}
}
