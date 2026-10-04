package main

import "go/ast"

// Every indexed segment contributes a wire key, including segments reached
// through a local alias before a later map or field mutation.
func inspectWireReceiverKeys(
	expression ast.Expr, visiting map[wireSourceVariable]bool, report func(*ast.BasicLit), assignments wireSourceAssignments,
) {
	switch expression := expression.(type) {
	case *ast.IndexExpr:
		inspectWireValueLiterals(expression.Index, make(map[wireSourceVariable]bool), report, assignments)
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.SelectorExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.ParenExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.UnaryExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.StarExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.SliceExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.TypeAssertExpr:
		inspectWireReceiverKeys(expression.X, visiting, report, assignments)
	case *ast.Ident:
		inspectWireReceiverAliasKeys(expression, visiting, report, assignments)
	}
}

func inspectWireReceiverAliasKeys(
	identifier *ast.Ident, visiting map[wireSourceVariable]bool, report func(*ast.BasicLit), assignments wireSourceAssignments,
) {
	if identifier.Obj == nil {
		return
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return
	}

	visiting[key] = true

	if value := wireAliasValue(identifier.Name, declaration); value != nil {
		inspectWireReceiverKeys(value, visiting, report, assignments)
	}

	for _, value := range assignments.values[key] {
		if value.Pos() < identifier.Pos() {
			inspectWireReceiverKeys(value, visiting, report, assignments)
		}
	}
}
