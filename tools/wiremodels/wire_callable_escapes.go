package main

import (
	"go/ast"
)

const jsonEncodeMethod = "Encode"
const jsonUnmarshalMethod = "Unmarshal"

// Unknown callbacks cannot be inspected for fixed wire keys or values. Require
// a local body, generated constructor, or an import-resolved read-only codec.
func unverifiedWireCallable(
	file *ast.File, call *ast.CallExpr, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) bool {
	if verifiedLocalWireCallable(call.Fun, assignments) ||
		isWireCopyFunction(file, call.Fun, assignments, make(map[wireSourceVariable]bool)) ||
		isGeneratedWireConstruction(call.Fun, aliases, path, models) ||
		verifiedWireCallable(file, call.Fun, assignments) ||
		verifiedWireURLValuesMutation(file, call, assignments) {
		return false
	}

	for _, argument := range call.Args {
		if generatedWireReceiver(argument, aliases, path, models, make(map[wireSourceVariable]bool), assignments) ||
			wireURLValuesReceiver(argument, file, assignments, make(map[wireSourceVariable]bool)) {
			return true
		}
	}

	return false
}

//nolint:cyclop // The exact codec and standard-library allowlist is fail-closed by selector and import owner.
func verifiedWireCallable(file *ast.File, expression ast.Expr, assignments wireSourceAssignments) bool {
	if array, conversion := expression.(*ast.ArrayType); conversion {
		element, primitive := array.Elt.(*ast.Ident)

		return primitive && element.Name == "byte"
	}

	if identifier, plain := expression.(*ast.Ident); plain {
		if identifier.Obj != nil {
			_, conversion := identifier.Obj.Decl.(*ast.TypeSpec)

			return conversion
		}

		switch identifier.Name {
		case "len", "cap", "append", builtinDeleteName, "clear", schemaStringType, "bool", "int", "int32", "int64", "float32", "float64":
			return true
		}
	}

	selector, selected := expression.(*ast.SelectorExpr)
	if !selected {
		return false
	}

	if selector.Sel.Name == "After" || selector.Sel.Name == "Before" || selector.Sel.Name == "Equal" || selector.Sel.Name == "UnixNano" {
		return verifiedTimeValue(file, selector.X)
	}

	if selector.Sel.Name == "Decode" || selector.Sel.Name == jsonEncodeMethod {
		return verifiedJSONCodec(file, selector.X, assignments, make(map[wireSourceVariable]bool))
	}

	if selector.Sel.Name == "DecodeString" || selector.Sel.Name == "EncodeToString" {
		return verifiedBase64Codec(file, selector.X)
	}

	if verifiedMQTTConfigMethod(file, selector, assignments) {
		return true
	}

	return verifiedImportedWireCallable(file, selector)
}

func verifiedMQTTConfigMethod(file *ast.File, selector *ast.SelectorExpr, assignments wireSourceAssignments) bool {
	switch selector.Sel.Name {
	case "AddBroker", "SetClientID", "SetUsername", "SetPassword":
	default:
		return false
	}

	return verifiedMQTTOptionsValue(file, selector.X, assignments, make(map[wireSourceVariable]bool))
}

func verifiedMQTTOptionsValue(
	file *ast.File, expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	identifier, isIdentifier := expression.(*ast.Ident)
	if !isIdentifier || identifier.Obj == nil {
		return false
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return false
	}

	visiting[key] = true

	for _, value := range wireAliasExpressions(identifier, assignments) {
		if verifiedMQTTOptionsConstructor(file, value) {
			return true
		}

		if verifiedMQTTOptionsValue(file, value, assignments, visiting) {
			return true
		}
	}

	return false
}

func verifiedMQTTOptionsConstructor(file *ast.File, expression ast.Expr) bool {
	call, isCall := expression.(*ast.CallExpr)
	if !isCall || len(call.Args) != 0 {
		return false
	}

	function, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || function.Sel.Name != "NewClientOptions" {
		return false
	}

	packageName, isPackage := function.X.(*ast.Ident)
	if !isPackage || packageName.Obj != nil {
		return false
	}

	owner := primitivePackage{ImportPath: "github.com/portpowered/go-tuya/pkg/dependencies/mqtttransport", Name: "mqtt", Directory: ""}

	return primitiveImportAliases(file, owner)[packageName.Name]
}

func verifiedBase64Codec(file *ast.File, expression ast.Expr) bool {
	encoding, isEncoding := expression.(*ast.SelectorExpr)
	if !isEncoding || (encoding.Sel.Name != "StdEncoding" && encoding.Sel.Name != "RawStdEncoding") {
		return false
	}

	identifier, isIdentifier := encoding.X.(*ast.Ident)
	if !isIdentifier || identifier.Obj != nil {
		return false
	}

	owner := primitivePackage{ImportPath: "encoding/base64", Name: "base64", Directory: ""}

	return primitiveImportAliases(file, owner)[identifier.Name]
}

//nolint:cyclop // Each import-specific branch permits only known read-only wire or transport helpers.
func verifiedImportedWireCallable(file *ast.File, selector *ast.SelectorExpr) bool {
	identifier, imported := selector.X.(*ast.Ident)
	if !imported || identifier.Obj != nil {
		return false
	}

	stringsOwner := primitivePackage{ImportPath: stringsImportPath, Name: stringsImportPath, Directory: ""}
	if primitiveImportAliases(file, stringsOwner)[identifier.Name] {
		switch selector.Sel.Name {
		case "TrimPrefix", "TrimSpace", "TrimRight", "ToLower", "NewReader", "Join", "HasPrefix", "Contains", "ReplaceAll":
			return true
		}
	}

	timeOwner := primitivePackage{ImportPath: timeImportPath, Name: timeImportPath, Directory: ""}
	if primitiveImportAliases(file, timeOwner)[identifier.Name] {
		return selector.Sel.Name == "Parse" || selector.Sel.Name == "ParseInLocation" || selector.Sel.Name == "Now"
	}

	formatOwner := primitivePackage{ImportPath: "fmt", Name: "fmt", Directory: ""}
	if primitiveImportAliases(file, formatOwner)[identifier.Name] {
		switch selector.Sel.Name {
		case "Sprint", "Sprintf", "Sprintln", "Errorf":
			return true
		}
	}

	strconvOwner := primitivePackage{ImportPath: "strconv", Name: "strconv", Directory: ""}
	if primitiveImportAliases(file, strconvOwner)[identifier.Name] {
		return selector.Sel.Name == "FormatInt" || selector.Sel.Name == "FormatUint"
	}

	owner := primitivePackage{ImportPath: "encoding/json", Name: "json", Directory: ""}
	if primitiveImportAliases(file, owner)[identifier.Name] {
		return selector.Sel.Name == "Marshal" || selector.Sel.Name == "MarshalIndent" || selector.Sel.Name == jsonUnmarshalMethod
	}

	transportOwner := primitivePackage{ImportPath: "github.com/portpowered/go-tuya/pkg/dependencies/httptransport", Name: "httptransport", Directory: ""}
	if primitiveImportAliases(file, transportOwner)[identifier.Name] {
		return selector.Sel.Name == "Do"
	}

	wireOwner := primitivePackage{ImportPath: wireImportPath, Name: generatedModelPackageName, Directory: generatedModelDirectory}
	if primitiveImportAliases(file, wireOwner)[identifier.Name] {
		return selector.Sel.Name == "IsKnownHeader" || selector.Sel.Name == "IsKnownQueryParam"
	}

	return false
}

//nolint:cyclop // The alias and constructor cases establish the exact encoding/json codec origin.
func verifiedJSONCodec(file *ast.File, expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) bool {
	if identifier, named := expression.(*ast.Ident); named && identifier.Obj != nil {
		declaration, valid := identifier.Obj.Decl.(ast.Node)

		key := wireSourceVariable{declaration: declaration, name: identifier.Name}

		if !valid || visiting[key] {
			return false
		}

		visiting[key] = true
		for _, value := range wireAliasExpressions(identifier, assignments) {
			if verifiedJSONCodec(file, value, assignments, visiting) {
				return true
			}
		}
	}

	call, called := expression.(*ast.CallExpr)
	if !called {
		return false
	}

	selector, selected := call.Fun.(*ast.SelectorExpr)
	if !selected || selector.Sel.Name != "NewDecoder" && selector.Sel.Name != "NewEncoder" {
		return false
	}

	identifier, imported := selector.X.(*ast.Ident)
	if !imported || identifier.Obj != nil {
		return false
	}

	owner := primitivePackage{ImportPath: "encoding/json", Name: "json", Directory: ""}

	return primitiveImportAliases(file, owner)[identifier.Name]
}

func verifiedTimeValue(file *ast.File, expression ast.Expr) bool {
	call, called := expression.(*ast.CallExpr)
	if !called {
		return false
	}

	selector, selected := call.Fun.(*ast.SelectorExpr)
	if !selected {
		return false
	}

	if selector.Sel.Name == wireAddMethod || selector.Sel.Name == "AddDate" {
		return verifiedTimeValue(file, selector.X)
	}

	identifier, imported := selector.X.(*ast.Ident)
	if !imported || identifier.Obj != nil || selector.Sel.Name != "Now" {
		return false
	}

	owner := primitivePackage{ImportPath: timeImportPath, Name: timeImportPath, Directory: ""}

	return primitiveImportAliases(file, owner)[identifier.Name]
}

func verifiedLocalWireCallable(expression ast.Expr, assignments wireSourceAssignments) bool {
	functions := wireLocalHelpers(expression, assignments, make(map[wireSourceVariable]bool))
	if len(functions) == 0 {
		return false
	}

	for _, function := range functions {
		if function == nil {
			return false
		}
	}

	return true
}
