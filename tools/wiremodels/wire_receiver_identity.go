package main

import (
	"go/ast"
	"maps"
)

// Resolve the lexical type declaration rather than trusting a method spelling.
func wireReceiverDeclaration(expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) *ast.TypeSpec {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.StarExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.UnaryExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.IndexExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.IndexListExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.CompositeLit:
		return wireReceiverDeclaration(expression.Type, assignments, visiting)
	case *ast.MapType:
		return wireReceiverDeclaration(expression.Value, assignments, visiting)
	case *ast.TypeAssertExpr:
		return wireReceiverDeclaration(expression.Type, assignments, visiting)
	case *ast.Ident:
		return wireReceiverIdentifier(expression, assignments, visiting)
	}

	return nil
}

//nolint:cyclop // Receiver identity follows local type aliases and helper provenance with cycle guards.
func wireReceiverIdentifier(identifier *ast.Ident, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) *ast.TypeSpec {
	if identifier.Obj == nil {
		return nil
	}

	if declaration, named := identifier.Obj.Decl.(*ast.TypeSpec); named {
		return declaration
	}

	declaration, valid := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !valid || visiting[key] {
		return nil
	}

	visiting[key] = true

	switch declaration := declaration.(type) {
	case *ast.Field:
		return wireReceiverDeclaration(declaration.Type, assignments, visiting)
	case *ast.ValueSpec:
		if declaration.Type != nil {
			return wireReceiverDeclaration(declaration.Type, assignments, visiting)
		}
	}

	var owner *ast.TypeSpec

	for _, value := range wireAliasExpressions(identifier, assignments) {
		candidate := wireReceiverDeclaration(value, assignments, maps.Clone(visiting))
		if candidate == nil || (owner != nil && candidate != owner) {
			return nil
		}

		owner = candidate
	}

	return owner
}

func wireCallableFieldDeclared(selector *ast.SelectorExpr, assignments wireSourceAssignments) bool {
	owner := wireReceiverDeclaration(selector.X, assignments, make(map[wireSourceVariable]bool))
	if owner == nil {
		return false
	}

	object, structured := owner.Type.(*ast.StructType)
	if !structured {
		return false
	}

	for _, field := range object.Fields.List {
		if _, callable := field.Type.(*ast.FuncType); !callable {
			continue
		}

		for _, name := range field.Names {
			if name.Name == selector.Sel.Name {
				return true
			}
		}
	}

	return false
}
