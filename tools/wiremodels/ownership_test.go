package main

import (
	"errors"
	"slices"
	"testing"
)

const (
	listOperation         = "listDevices"
	ownershipReferenceKey = "$ref"
	ownershipKnownSchema  = "Known"
	ownershipKnownValue   = "known"
)

func TestOwnershipRejectsUnassignedAndDuplicateOperations(t *testing.T) {
	t.Parallel()

	manifest := modelManifest{
		Input:  "api/http.openapi.yaml",
		Groups: []modelGroup{{Name: "devices", Source: "api/models/devices.yaml", Operations: []string{listOperation}}},
	}

	for _, identifier := range []string{listOperation, "newWireOperation"} {
		t.Run(identifier, func(t *testing.T) {
			t.Parallel()

			bundle := ownershipBundle(identifier)

			err := validateModelOwnership(manifest, bundle)
			if identifier == listOperation {
				if err != nil {
					t.Fatal(err)
				}

				return
			}

			if !errors.Is(err, errModelOwnership) {
				t.Fatalf("unassigned operation accepted: %v", err)
			}
		})
	}

	duplicate := modelManifest{
		Input: manifest.Input,
		Groups: append(manifest.Groups, modelGroup{
			Name: "homes", Source: "api/models/homes.yaml", Operations: []string{listOperation},
		}),
	}

	err := validateModelOwnership(duplicate, ownershipBundle(listOperation))
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("duplicate operation ownership accepted: %v", err)
	}
}

func TestOwnershipRejectsInvalidGroupAndReferences(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../escape", "Devices", ""} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			manifest := modelManifest{
				Input: "api/http.openapi.yaml",
				Groups: []modelGroup{{
					Name: name, Source: "api/models/" + name + ".yaml", Operations: []string{listOperation},
				}},
			}

			err := validateModelOwnership(manifest, ownershipBundle(listOperation))
			if !errors.Is(err, errModelOwnership) {
				t.Fatalf("invalid group accepted: %v", err)
			}
		})
	}

	bundle := ownershipBundle(listOperation)
	bundle[propertyFixtureComponents] = map[string]any{
		propertyFixtureSchemas: map[string]any{
			"Device": map[string]any{ownershipReferenceKey: "#/components/schemas/Missing"},
		},
	}

	err := validateComponentReferences(bundle, bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("unresolved reference accepted: %v", err)
	}
}

func TestKnownValueBindingsKeepWireFieldsOpenAndResolveEnums(t *testing.T) {
	t.Parallel()

	bundle := map[string]any{
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				"RawStatus": map[string]any{
					propertyFixtureType: schemaObjectType,
					propertyFixtureProperties: map[string]any{
						"code": map[string]any{
							"type":               schemaStringType,
							knownValuesExtension: "KnownCode",
						},
					},
				},
				"KnownCode": map[string]any{
					propertyFixtureType:  stringSchemaType,
					schemaFixtureEnumKey: []any{"switch_led", "bright_value"},
				},
			},
		},
	}

	err := validateComponentReferences(bundle, bundle)
	if err != nil {
		t.Fatalf("valid open-string binding rejected: %v", err)
	}

	components, validComponents := bundle[propertyFixtureComponents].(map[string]any)
	if !validComponents {
		t.Fatal("components are missing")
	}

	schemas, validSchemas := components[propertyFixtureSchemas].(map[string]any)
	if !validSchemas {
		t.Fatal("schemas are missing")
	}

	delete(schemas, "KnownCode")

	err = validateComponentReferences(bundle, bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("unresolved known-value enum accepted: %v", err)
	}
}

func TestKnownValueBindingsRejectClosedOrDuplicateEnums(t *testing.T) {
	t.Parallel()

	for name, known := range map[string]any{
		"closed source": map[string]any{
			propertyFixtureType:  stringSchemaType,
			schemaFixtureEnumKey: []any{ownershipKnownValue},
		},
		"duplicate enum": map[string]any{
			propertyFixtureType:  stringSchemaType,
			schemaFixtureEnumKey: []any{ownershipKnownValue, ownershipKnownValue},
		},
		"non-string enum": map[string]any{
			propertyFixtureType:  stringSchemaType,
			schemaFixtureEnumKey: []any{1},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source := map[string]any{propertyFixtureType: stringSchemaType, knownValuesExtension: ownershipKnownSchema}

			if name == "closed source" {
				closedSource, validSource := known.(map[string]any)
				if !validSource {
					t.Fatal("closed source fixture has an invalid shape")
				}

				source = closedSource
				source[knownValuesExtension] = ownershipKnownSchema
			}

			bundle := map[string]any{
				propertyFixtureComponents: map[string]any{
					propertyFixtureSchemas: map[string]any{
						ownershipKnownSchema: known,
						"Source":             source,
					},
				},
			}

			err := validateComponentReferences(bundle, bundle)
			if !errors.Is(err, errModelOwnership) {
				t.Fatalf("invalid known-value binding accepted: %v", err)
			}
		})
	}
}

func TestKnownValueBindingsFollowNestedReferencesAndArrayItems(t *testing.T) {
	t.Parallel()

	bundle := nestedKnownValueBundle(false)

	err := validateComponentReferences(bundle, bundle)
	if err != nil {
		t.Fatalf("valid nested open-string bindings rejected: %v", err)
	}

	components, validComponents := bundle[propertyFixtureComponents].(map[string]any)
	if !validComponents {
		t.Fatal("components are missing")
	}

	schemas, validSchemas := components[propertyFixtureSchemas].(map[string]any)
	if !validSchemas {
		t.Fatal("schemas are missing")
	}

	locations := knownValueBindingsByTarget(schemas)[ownershipKnownSchema]
	for _, want := range []string{"Nested.status (open string)", "Source.events.items (open string)"} {
		if !slices.Contains(locations, want) {
			t.Errorf("nested binding locations %v do not include %q", locations, want)
		}
	}
}

func TestKnownValueBindingsRejectAnonymousClosedEnumInArrayItem(t *testing.T) {
	t.Parallel()

	bundle := nestedKnownValueBundle(true)

	err := validateComponentReferences(bundle, bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("anonymous closed enum in a referenced array item was accepted: %v", err)
	}
}

func nestedKnownValueBundle(closedItem bool) map[string]any {
	item := map[string]any{
		propertyFixtureType: stringSchemaType, knownValuesExtension: ownershipKnownSchema,
	}
	if closedItem {
		item[schemaFixtureEnumKey] = []any{ownershipKnownValue}
	}

	return map[string]any{
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				ownershipKnownSchema: map[string]any{
					propertyFixtureType:  stringSchemaType,
					schemaFixtureEnumKey: []any{ownershipKnownValue},
				},
				"Nested": map[string]any{
					propertyFixtureType: schemaObjectType,
					propertyFixtureProperties: map[string]any{"status": map[string]any{
						propertyFixtureType: stringSchemaType, knownValuesExtension: ownershipKnownSchema,
					}},
				},
				"Source": map[string]any{
					propertyFixtureType: schemaObjectType,
					propertyFixtureProperties: map[string]any{
						"events": map[string]any{
							propertyFixtureType: "array", "items": item,
						},
						"nested": map[string]any{ownershipReferenceKey: "#/components/schemas/Nested"},
					},
				},
			},
		},
	}
}

func TestHTTPContentTypeUsesGeneratedMediaTypeEnum(t *testing.T) {
	t.Parallel()

	bundle := contentTypeBundle()

	err := validateHTTPContentTypeBindings(bundle)
	if err != nil {
		t.Fatalf("schema-bound Content-Type rejected: %v", err)
	}

	parameter := contentTypeParameter(t, bundle)
	parameter["schema"] = map[string]any{propertyFixtureType: stringSchemaType}

	err = validateHTTPContentTypeBindings(bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("unbound Content-Type schema accepted: %v", err)
	}

	bundle = contentTypeBundle()

	setJSONMediaTypeSchema(t, bundle, map[string]any{
		propertyFixtureType: stringSchemaType, schemaFixtureEnumKey: []any{"application/xml"},
	})

	err = validateHTTPContentTypeBindings(bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("changed media type enum accepted: %v", err)
	}
}

func contentTypeBundle() map[string]any {
	return map[string]any{
		"paths": map[string]any{
			"/auth": map[string]any{
				"get": map[string]any{
					"parameters": []any{map[string]any{
						"name": "Content-Type", "in": "header",
						"schema": map[string]any{ownershipReferenceKey: "#/components/schemas/JSONMediaType"},
					}},
				},
			},
		},
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				"JSONMediaType": map[string]any{
					propertyFixtureType: stringSchemaType, schemaFixtureEnumKey: []any{"application/json"},
				},
			},
		},
	}
}

func contentTypeParameter(t *testing.T, bundle map[string]any) map[string]any {
	t.Helper()

	paths, validPaths := bundle["paths"].(map[string]any)
	if !validPaths {
		t.Fatal("paths are missing")
	}

	operations, validOperations := paths["/auth"].(map[string]any)
	if !validOperations {
		t.Fatal("auth path is missing")
	}

	operation, validOperation := operations["get"].(map[string]any)
	if !validOperation {
		t.Fatal("GET operation is missing")
	}

	parameters, validParameters := operation["parameters"].([]any)
	if !validParameters || len(parameters) == 0 {
		t.Fatal("auth parameters are missing")
	}

	parameter, validParameter := parameters[0].(map[string]any)
	if !validParameter {
		t.Fatal("Content-Type parameter has an invalid shape")
	}

	return parameter
}

func setJSONMediaTypeSchema(t *testing.T, bundle, schema map[string]any) {
	t.Helper()

	components, validComponents := bundle[propertyFixtureComponents].(map[string]any)
	if !validComponents {
		t.Fatal("components are missing")
	}

	schemas, validSchemas := components[propertyFixtureSchemas].(map[string]any)
	if !validSchemas {
		t.Fatal("schemas are missing")
	}

	schemas["JSONMediaType"] = schema
}

func ownershipBundle(identifier string) map[string]any {
	return map[string]any{
		"paths": map[string]any{"/devices": map[string]any{"get": map[string]any{
			"operationId": identifier,
			"parameters": []any{map[string]any{
				"name": "Content-Type", "in": "header",
				"schema": map[string]any{ownershipReferenceKey: "#/components/schemas/JSONMediaType"},
			}},
		}}},
		propertyFixtureComponents: map[string]any{propertyFixtureSchemas: map[string]any{
			"JSONMediaType": map[string]any{propertyFixtureType: stringSchemaType, schemaFixtureEnumKey: []any{"application/json"}},
		}},
	}
}
