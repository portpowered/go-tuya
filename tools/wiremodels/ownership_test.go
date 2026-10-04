package main

import (
	"errors"
	"testing"
)

const listOperation = "listDevices"

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
	bundle["components"] = map[string]any{"schemas": map[string]any{"Device": map[string]any{"$ref": "#/components/schemas/Missing"}}}

	err := validateComponentReferences(bundle, bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("unresolved reference accepted: %v", err)
	}
}

func ownershipBundle(identifier string) map[string]any {
	return map[string]any{
		"paths":      map[string]any{"/devices": map[string]any{"get": map[string]any{"operationId": identifier}}},
		"components": map[string]any{"schemas": map[string]any{}},
	}
}
