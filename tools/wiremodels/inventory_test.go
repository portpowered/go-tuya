package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const capabilityPercentMaximumName = "CapabilityScalePercentMaximum"
const capabilityTemperatureTenthsName = "CapabilityScaleTemperatureTenths"

const schemaFixtureEnumKey = "enum"

func TestInventoryIncludesUnreferencedExportedJSONModel(t *testing.T) {
	t.Parallel()

	tag := "`json:\"value\"`"
	source := "package tuya\ntype LegacyResponse struct { Value string " + tag + " }\n"
	files := parseInventoryTestSource(t, source)

	inventorySources := inspectInventorySources(files)
	models := inventorySources.models

	err := rejectAnonymousJSONObjects(inventorySources.anonymous)
	if err != nil {
		t.Fatal(err)
	}

	if len(models) != 1 || models[0].name != "LegacyResponse" || len(models[0].uses) != 0 {
		t.Fatalf("unreferenced exported JSON model was not inventoried: %#v", models)
	}

	var inventory strings.Builder

	writeHandwrittenPopulation(&inventory, models)

	if !strings.Contains(inventory.String(), "`LegacyResponse`") || !strings.Contains(inventory.String(), "value") {
		t.Fatalf("inventory omitted the unreferenced model: %s", inventory.String())
	}
}

func TestInventoryRejectsAnonymousNestedJSONModel(t *testing.T) {
	t.Parallel()

	tagValue := "`json:\"value\"`"
	tagPayload := "`json:\"payload\"`"
	source := "package tuya\ntype WireEnvelope struct { Payload struct { Value string " + tagValue + " } " + tagPayload + " }\n"
	files := parseInventoryTestSource(t, source)

	inventorySources := inspectInventorySources(files)

	err := rejectAnonymousJSONObjects(inventorySources.anonymous)
	if err == nil {
		t.Fatal("anonymous nested JSON object was accepted")
	}
}

func TestInventoryRejectsGeneratedEnumValueDrift(t *testing.T) {
	t.Parallel()

	schemas := map[string]any{
		"Status": map[string]any{"type": "string", schemaFixtureEnumKey: []any{"ready", "sleeping"}, "x-enum-varnames": []any{"StatusReady", "StatusSleeping"}},
	}

	generated := map[string]map[string]string{"Status": {"StatusReady": "ready", "StatusSleeping": "sleeping"}}

	err := validateGeneratedEnumValues(schemas, generated)
	if err != nil {
		t.Fatalf("matching schema and generated enum rejected: %v", err)
	}

	generated["Status"]["StatusSleeping"] = "asleep"

	err = validateGeneratedEnumValues(schemas, generated)
	if err == nil {
		t.Fatal("generated enum value drift was accepted")
	}
}

func TestPublicNumericConstantsRequireExactGeneratedValuesAndUses(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"x-go-tuya-numeric-constants": map[string]any{
			capabilityPercentMaximumName:    100,
			capabilityTemperatureTenthsName: 10,
		},
	}
	constants := []generatedConstant{
		{name: capabilityPercentMaximumName, typeName: "", value: "100", file: publicValuesGeneratedPath, numeric: true},
		{name: capabilityTemperatureTenthsName, typeName: "", value: "10", file: publicValuesGeneratedPath, numeric: true},
	}
	uses := map[string][]modelUse{
		capabilityPercentMaximumName:    {{path: "pkg/tuya/capabilities.go", line: 10, name: capabilityPercentMaximumName}},
		capabilityTemperatureTenthsName: {{path: "pkg/tuya/capabilities.go", line: 20, name: capabilityTemperatureTenthsName}},
	}

	err := validatePublicNumericConstants("CapabilityNormalization", schema, constants, uses)
	if err != nil {
		t.Fatalf("matching generated constants and production uses were rejected: %v", err)
	}

	constants[0].value = "99"

	err = validatePublicNumericConstants("CapabilityNormalization", schema, constants, uses)
	if err == nil {
		t.Fatal("numeric constant drift was accepted")
	}

	constants[0].value = "100"

	delete(uses, capabilityTemperatureTenthsName)

	err = validatePublicNumericConstants("CapabilityNormalization", schema, constants, uses)
	if err == nil {
		t.Fatal("unreferenced numeric constant was accepted")
	}
}

func parseInventoryTestSource(t *testing.T, source string) []sourceFile {
	t.Helper()

	set := token.NewFileSet()

	file, err := parser.ParseFile(set, "pkg/tuya/inventory_test_input.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	return []sourceFile{{path: "pkg/tuya/inventory_test_input.go", file: file, set: set}}
}
