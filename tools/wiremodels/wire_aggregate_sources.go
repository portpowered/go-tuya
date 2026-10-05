package main

import "go/ast"

func wireAggregateFieldExpressions(selector *ast.SelectorExpr, assignments wireSourceAssignments) []ast.Expr {
	owner := wireReceiverDeclaration(selector.X, assignments, make(map[wireSourceVariable]bool))
	if owner == nil {
		return nil
	}

	return assignments.aggregateFields[wireSourceField{declaration: owner, name: selector.Sel.Name}]
}

func generatedWireAggregate(
	object *ast.CompositeLit, aliases map[string]bool, path string, models map[string]generatedModel,
	visiting map[wireSourceVariable]bool, assignments wireSourceAssignments,
) bool {
	if generatedWireDeclaredType(object.Type, aliases, path, models) {
		return true
	}

	for _, value := range object.Elts {
		if pair, keyed := value.(*ast.KeyValueExpr); keyed {
			value = pair.Value
		}

		if generatedWireReceiver(value, aliases, path, models, visiting, assignments) {
			return true
		}
	}

	return false
}

func generatedWireReturnedValue(
	function *ast.FuncType, body *ast.BlockStmt, aliases map[string]bool, path string, models map[string]generatedModel,
	visiting map[wireSourceVariable]bool, assignments wireSourceAssignments,
) bool {
	if body == nil {
		return false
	}

	key := wireSourceVariable{declaration: body, name: "generated return"}
	if visiting[key] {
		return false
	}

	visiting[key] = true
	found := false

	ast.Inspect(body, func(node ast.Node) bool {
		if _, closure := node.(*ast.FuncLit); closure {
			return false
		}

		if returned, valid := node.(*ast.ReturnStmt); valid {
			for _, result := range wireReturnExpressions(function, returned) {
				if generatedWireReceiver(result, aliases, path, models, visiting, assignments) {
					found = true
				}
			}
		}

		return !found
	})

	return found
}

func generatedWireCallResult(
	call *ast.CallExpr, aliases map[string]bool, path string, models map[string]generatedModel,
	visiting map[wireSourceVariable]bool, assignments wireSourceAssignments,
) bool {
	if generatedWireMapResult(call, aliases, path, models, assignments, visiting) {
		return true
	}

	for _, function := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
		if function != nil && !wireReturnsPublicInterface(function, assignments) &&
			generatedWireReturnedValue(function, assignments.functionBodies[function], aliases, path, models, visiting,
				wireReturnParameterValues(function, call, assignments)) {
			return true
		}
	}

	return false
}

func wireReturnsPublicInterface(function *ast.FuncType, assignments wireSourceAssignments) bool {
	if function.Results == nil || len(function.Results.List) == 0 {
		return false
	}

	first := function.Results.List[0].Type
	identifier, isIdentifier := first.(*ast.Ident)

	return isIdentifier && assignments.publicInterfaces[identifier.Name]
}
