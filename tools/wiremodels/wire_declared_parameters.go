package main

import "go/ast"

// Exported helpers can receive generated payloads from callers outside this
// package. Resolve their declared types using the imports of their own file.
func indexDeclaredWireParameters(
	file *ast.File, aliases map[string]bool, path string, models map[string]generatedModel, assignments wireSourceAssignments,
) {
	ast.Inspect(file, func(node ast.Node) bool {
		function, callable := node.(*ast.FuncType)
		if !callable || function.Params == nil {
			return true
		}

		for _, field := range function.Params.List {
			if generatedWireDeclaredType(field.Type, aliases, path, models) {
				for _, name := range field.Names {
					key := wireSourceVariable{declaration: field, name: name.Name}
					assignments.generatedParameters[key] = true
				}
			}
		}

		return true
	})
}
