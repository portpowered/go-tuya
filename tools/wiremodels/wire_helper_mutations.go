package main

import (
	"go/ast"
	"slices"
)

// Local helper parameters retain generated receiver provenance, so moving a
// mutation into a helper cannot turn a library-defined wire map into open input.
func indexWireHelperParameters(
	file *ast.File, aliases map[string]bool, path string, models map[string]generatedModel, assignments wireSourceAssignments,
) {
	indexDeclaredWireParameters(file, aliases, path, models, assignments)

	for {
		before := wireProvenanceCount(assignments)

		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			for _, function := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
				if function != nil {
					markWireHelperParameters(function, call, aliases, path, models, assignments)
				}
			}

			return true
		})

		if before == wireProvenanceCount(assignments) {
			return
		}
	}
}

//nolint:cyclop // Helper discovery follows Go expression forms and resolves declared callbacks fail-closed.
func wireLocalHelpers(expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) []*ast.FuncType {
	switch expression := expression.(type) {
	case *ast.CallExpr:
		return wireReturnedHelpers(expression, assignments, visiting)
	case *ast.SelectorExpr:
		key := wireSourceVariable{declaration: expression, name: expression.Sel.Name}
		if visiting[key] {
			return nil
		}

		visiting[key] = true

		functions := wireMethodHelpers(expression, assignments)

		if wireCallableFieldDeclared(expression, assignments) {
			for _, value := range assignments.callableFields[expression.Sel.Name] {
				functions = append(functions, wireLocalHelpers(value, assignments, visiting)...)
			}
		}

		return wireHelperCandidates(functions)
	case *ast.FuncLit:
		return []*ast.FuncType{expression.Type}
	case *ast.IndexExpr:
		return wireLocalHelpers(expression.X, assignments, visiting)
	case *ast.IndexListExpr:
		return wireLocalHelpers(expression.X, assignments, visiting)
	case *ast.ParenExpr:
		return wireLocalHelpers(expression.X, assignments, visiting)
	}

	identifier, isIdentifier := expression.(*ast.Ident)
	if !isIdentifier || identifier.Obj == nil {
		return []*ast.FuncType{nil}
	}

	if function, isFunction := identifier.Obj.Decl.(*ast.FuncDecl); isFunction {
		return []*ast.FuncType{function.Type}
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return nil
	}

	visiting[key] = true

	var functions []*ast.FuncType

	for _, value := range wireAliasExpressions(identifier, assignments) {
		functions = append(functions, wireLocalHelpers(value, assignments, visiting)...)
	}

	return wireHelperCandidates(functions)
}

func inspectWireCopyCall(
	file *ast.File, call *ast.CallExpr, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, report func(*ast.BasicLit),
) {
	inspectWireBuiltinMutation(file, call, aliases, path, models, assignments, report)

	if len(call.Args) != 2 || !isWireCopyFunction(file, call.Fun, assignments, make(map[wireSourceVariable]bool)) ||
		!generatedWireReceiver(call.Args[0], aliases, path, models, make(map[wireSourceVariable]bool), assignments) {
		return
	}

	inspectWireReceiverKeys(call.Args[0], make(map[wireSourceVariable]bool), report, assignments)
	inspectWireValueLiterals(call.Args[1], make(map[wireSourceVariable]bool), report, assignments)
}

//nolint:cyclop // Copy recognition resolves aliases and import owners without trusting method names alone.
func isWireCopyFunction(file *ast.File, expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) bool {
	switch expression := expression.(type) {
	case *ast.IndexExpr:
		return isWireCopyFunction(file, expression.X, assignments, visiting)
	case *ast.IndexListExpr:
		return isWireCopyFunction(file, expression.X, assignments, visiting)
	case *ast.ParenExpr:
		return isWireCopyFunction(file, expression.X, assignments, visiting)
	}

	if identifier, isIdentifier := expression.(*ast.Ident); isIdentifier {
		if identifier.Obj == nil {
			return identifier.Name == "copy"
		}

		declaration, isNode := identifier.Obj.Decl.(ast.Node)

		key := wireSourceVariable{declaration: declaration, name: identifier.Name}

		if !isNode || visiting[key] {
			return false
		}

		visiting[key] = true
		for _, value := range wireAliasExpressions(identifier, assignments) {
			if isWireCopyFunction(file, value, assignments, visiting) {
				return true
			}
		}

		return false
	}

	selector, isSelector := expression.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "Copy" {
		return false
	}

	identifier, isIdentifier := selector.X.(*ast.Ident)
	if !isIdentifier || identifier.Obj != nil {
		return false
	}

	owner := primitivePackage{ImportPath: "maps", Name: "maps", Directory: ""}

	return primitiveImportAliases(file, owner)[identifier.Name]
}

func markWireHelperParameters(
	function *ast.FuncType, call *ast.CallExpr, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) {
	if function.Params == nil {
		return
	}

	index := 0

	for _, field := range function.Params.List {
		if len(field.Names) == 0 {
			index++

			continue
		}

		for _, name := range field.Names {
			if _, callback := field.Type.(*ast.FuncType); callback && index < len(call.Args) {
				recordWireCallbackArgument(field, name.Name, call.Args[index], assignments)
			}

			if wireHelperArgumentGenerated(field, index, call, aliases, path, models, assignments) {
				assignments.generatedParameters[wireSourceVariable{declaration: field, name: name.Name}] = true
			}

			index++
		}
	}
}

func wireMethodHelpers(selector *ast.SelectorExpr, assignments wireSourceAssignments) []*ast.FuncType {
	functions := make([]*ast.FuncType, 0, len(assignments.methods[selector.Sel.Name]))
	owner := wireReceiverDeclaration(selector.X, assignments, make(map[wireSourceVariable]bool))

	for _, declaration := range assignments.methods[selector.Sel.Name] {
		if owner == nil || wireReceiverDeclaration(declaration.Recv.List[0].Type, assignments, make(map[wireSourceVariable]bool)) != owner {
			continue
		}

		function := declaration.Type

		if wireMethodExpression(selector.X) {
			fields := append([]*ast.Field{}, declaration.Recv.List...)
			fields = append(fields, function.Params.List...)
			function = &ast.FuncType{
				Func: function.Func, TypeParams: function.TypeParams,
				Params:  &ast.FieldList{Opening: function.Params.Opening, List: fields, Closing: function.Params.Closing},
				Results: function.Results,
			}
		}

		functions = append(functions, function)
	}

	return functions
}
func wireMethodExpression(expression ast.Expr) bool {
	switch expression := expression.(type) {
	case *ast.IndexExpr:
		return wireMethodExpression(expression.X)
	case *ast.IndexListExpr:
		return wireMethodExpression(expression.X)
	case *ast.ParenExpr:
		return wireMethodExpression(expression.X)
	case *ast.StarExpr:
		return wireMethodExpression(expression.X)
	case *ast.Ident:
		if expression.Obj == nil {
			return false
		}

		_, typeName := expression.Obj.Decl.(*ast.TypeSpec)

		return typeName
	}

	return false
}

func wireHelperArgumentGenerated(
	field *ast.Field, index int, call *ast.CallExpr, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) bool {
	if index >= len(call.Args) {
		return false
	}

	arguments := call.Args[index : index+1]
	if _, variadic := field.Type.(*ast.Ellipsis); variadic {
		arguments = call.Args[index:]
	}

	for _, argument := range arguments {
		if generatedWireReceiver(argument, aliases, path, models, make(map[wireSourceVariable]bool), assignments) {
			return true
		}
	}

	return false
}

func recordWireCallbackArgument(field *ast.Field, name string, value ast.Expr, assignments wireSourceAssignments) {
	key := wireSourceVariable{declaration: field, name: name}
	if slices.Contains(assignments.values[key], value) {
		return
	}

	assignments.values[key] = append(assignments.values[key], value)
}
func wireReturnedHelpers(call *ast.CallExpr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) []*ast.FuncType {
	key := wireSourceVariable{declaration: call, name: "returned helper"}
	if visiting[key] {
		return nil
	}

	visiting[key] = true

	var functions []*ast.FuncType

	for _, factory := range wireLocalHelpers(call.Fun, assignments, visiting) {
		body := assignments.functionBodies[factory]
		if body == nil {
			functions = append(functions, nil)

			continue
		}

		ast.Inspect(body, func(node ast.Node) bool {
			if _, nested := node.(*ast.FuncLit); nested {
				return false
			}

			if returned, valid := node.(*ast.ReturnStmt); valid {
				for _, value := range wireReturnExpressions(factory, returned) {
					functions = append(functions, wireLocalHelpers(value, assignments, visiting)...)
				}
			}

			return true
		})
	}

	return wireHelperCandidates(functions)
}

// A nil candidate retains an unresolved callable origin for fail-closed checks.
func wireHelperCandidates(functions []*ast.FuncType) []*ast.FuncType {
	if len(functions) == 0 {
		return []*ast.FuncType{nil}
	}

	return functions
}

func wireProvenanceCount(assignments wireSourceAssignments) int {
	count := len(assignments.generatedParameters)
	for _, values := range assignments.values {
		count += len(values)
	}

	return count
}
