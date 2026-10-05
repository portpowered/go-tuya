package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

const (
	wireProbeFirstFile  = "first.go"
	wireProbeSecondFile = "second.go"
)

func TestGeneratedFieldRejectsFixedBareNamedResultAndPreservesCallerInput(t *testing.T) {
	t.Parallel()

	for name, sources := range map[string]map[string]string{
		"fixed named result": {wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func fixed() (result string) { result = "audit-named-result-fixed-wire"; return }
func Build() wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: fixed()} }
`},
		"caller named result": {wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func fromCaller(value string) (result string) { result = value; return }
func Build(value string) wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: fromCaller(value)} }
`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertWireProbeCompiles(t, sources)

			err := checkWireProvenanceProbe(t, sources, provenanceModels())
			if name == "fixed named result" {
				if err == nil || !strings.Contains(err.Error(), "audit-named-result-fixed-wire") {
					t.Fatalf("fixed named result escaped generated field provenance: %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("caller-owned named result was rejected: %v", err)
			}
		})
	}
}

func TestGeneratedFieldRejectsSiblingFixedBareNamedResultAndPreservesCallerInput(t *testing.T) {
	t.Parallel()

	for name, sources := range map[string]map[string]string{
		"fixed sibling result": {
			"fixed.go": `package sample
func fixedSibling() (result string) { result = "audit-sibling-fixed-wire"; return }
`,
			"build.go": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func Build() wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: fixedSibling()} }
`,
		},
		"caller sibling result": {
			"fixed.go": `package sample
func callerSibling(value string) (result string) { result = value; return }
`,
			"build.go": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func Build(value string) wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: callerSibling(value)} }
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertWireProbeCompiles(t, sources)

			err := checkWireProvenanceProbe(t, sources, provenanceModels())
			if name == "fixed sibling result" {
				if err == nil || !strings.Contains(err.Error(), "audit-sibling-fixed-wire") {
					t.Fatalf("fixed sibling result escaped generated field provenance: %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("caller sibling result was rejected: %v", err)
			}
		})
	}
}

func TestGeneratedMapNamedResultRetainsProvenanceAcrossSiblingFiles(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		wireProbeFirstFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func wireRequestMap(value wire.RTCOfferBody) (result map[string]any) {
	result = value.AdditionalProperties
	return
}
func mutate(values map[string]any) { values["audit-named-map-result-key"] = "value" }
`,
		wireProbeSecondFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func build(value wire.RTCOfferBody) { values := wireRequestMap(value); mutate(values) }
func callerOwned(values map[string]any) { mutate(values) }
`,
	}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-named-map-result-key") {
		t.Fatalf("named generated map result escaped a sibling mutator: %v", err)
	}
}

func TestCallerOwnedMapRemainsOpenAcrossSiblingMutator(t *testing.T) {
	t.Parallel()

	sources := map[string]string{
		wireProbeFirstFile: `package sample
func mutate(values map[string]any, key string, value any) { values[key] = value }
`,
		wireProbeSecondFile: `package sample
func Build(values map[string]any, key string, value any) { mutate(values, key, value) }
`,
	}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err != nil {
		t.Fatalf("caller-owned open map was rejected across sibling helper: %v", err)
	}
}

func TestGeneratedFieldHelperProvenanceCrossesDeepSiblingChain(t *testing.T) {
	t.Parallel()

	const helperCount = 32

	first := strings.Builder{}
	first.WriteString("package sample\n")

	for index := range helperCount - 1 {
		first.WriteString("func step")
		first.WriteString(twoDigits(index))
		first.WriteString("(value string) string { return step")
		first.WriteString(twoDigits(index + 1))
		first.WriteString("(value) }\n")
	}

	first.WriteString("func step31(value string) string { return \"audit-deep-fixed-wire-value\" }\n")

	second := `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func Build() wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: step00("ignored")} }
`
	sources := map[string]string{wireProbeFirstFile: first.String(), wireProbeSecondFile: second}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-deep-fixed-wire-value") {
		t.Fatalf("fixed value beyond helper chain escaped provenance: %v", err)
	}
}

func TestGeneratedFieldDeepSiblingChainPreservesCallerInput(t *testing.T) {
	t.Parallel()

	const helperCount = 24

	first := strings.Builder{}
	first.WriteString("package sample\n")

	for index := range helperCount - 1 {
		first.WriteString("func pass")
		first.WriteString(twoDigits(index))
		first.WriteString("(value string) string { return pass")
		first.WriteString(twoDigits(index + 1))
		first.WriteString("(value) }\n")
	}

	first.WriteString("func pass23(value string) string { return value }\n")

	second := `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func Build(value string) wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: pass00(value)} }
`
	sources := map[string]string{wireProbeFirstFile: first.String(), wireProbeSecondFile: second}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err != nil {
		t.Fatalf("caller value through deep sibling chain was rejected: %v", err)
	}
}

func TestGeneratedFieldRejectsRecursiveFixedFallback(t *testing.T) {
	t.Parallel()

	sources := map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func recursive(depth int) (result string) {
	if depth == 0 { result = "audit-recursive-fixed-fallback" } else { result = recursive(depth - 1) }
	return
}
func build() wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: recursive(2)} }
`}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-recursive-fixed-fallback") ||
		!strings.Contains(err.Error(), "recursive generated wire field helper") {
		t.Fatalf("recursive helper with a fixed fallback escaped complete provenance validation: %v", err)
	}
}

func TestGeneratedEnumMutationFollowsNestedReferenceAndArrayReceivers(t *testing.T) {
	t.Parallel()

	for name, body := range generatedEnumMutationProbes() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertGeneratedEnumMutationRejected(t, name, body)
		})
	}
}

func generatedEnumMutationProbes() map[string]string {
	return map[string]string{
		"nested reference": `
type bodyAlias = *wire.RTCOfferBody
func mutate(value string) {
	var body bodyAlias = &wire.RTCOfferBody{}
	body.Type = wire.RTCOfferBodyType(value)
}`,
		"array element": `
func mutate(value string) {
	bodies := [1]*wire.RTCOfferBody{{}}
	bodies[0].Type = wire.RTCOfferBodyType(value)
}`,
		"slice element": `
func mutate(value string) {
	bodies := []*wire.RTCOfferBody{{}}
	bodies[0].Type = wire.RTCOfferBodyType(value)
}`,
		"slice expression then index": `
func mutate(value string) {
	bodies := []*wire.RTCOfferBody{{}}
	bodies[:][0].Type = wire.RTCOfferBodyType(value)
}`,
		"map then slice then index": `
func mutate(value string) {
	bodies := map[int][]*wire.RTCOfferBody{0: {&wire.RTCOfferBody{}}}
	bodies[0][0].Type = wire.RTCOfferBodyType(value)
}`,
		"type assertion": `
func mutate(value string) {
	var raw any = &wire.RTCOfferBody{}
	body := raw.(*wire.RTCOfferBody)
	body.Type = wire.RTCOfferBodyType(value)
}`,
		"type assertion receiver": `
func mutate(value string) {
	var raw any = &wire.RTCOfferBody{}
	raw.(*wire.RTCOfferBody).Type = wire.RTCOfferBodyType(value)
}`,
		"indexed closed enum field": `
func mutate(value string) {
	body := wire.RTCOfferListBody{}
	body.Types[0] = wire.RTCOfferBodyType(value)
}`,
		"generated reference array": `
func mutate(value string) {
	var envelope wire.Envelope
	envelope.Bodies[0].Type = wire.RTCOfferBodyType(value)
}`,
		"anonymous reference": `
func mutate(value string) {
	var envelope struct { Body wire.RTCOfferBody }
	envelope.Body.Type = wire.RTCOfferBodyType(value)
}`,
	}
}

func assertGeneratedEnumMutationRejected(t *testing.T, name, body string) {
	t.Helper()

	sources := map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
` + body + `
`}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "unregistered or unresolved value") {
		t.Fatalf("arbitrary caller string was accepted through %s: %v", name, err)
	}
}

func TestGeneratedEnumSliceAssignmentPreservesRegisteredAndTypedValues(t *testing.T) {
	t.Parallel()

	sources := map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func Build(value wire.RTCOfferBodyType) {
	body := wire.RTCOfferListBody{}
	body.Types[0] = value
	body.Types[1] = wire.Offer
}
`}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, provenanceModels())
	if err != nil {
		t.Fatalf("registered or caller-typed enum slice assignment was rejected: %v", err)
	}
}

func TestGeneratedEnumClosedAnonymousAndNestedSchemaControls(t *testing.T) {
	t.Parallel()

	models := provenanceModels()

	sources := map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
type localEnvelope struct { Body *wire.RTCOfferBody }
func Build(value string) {
	var reference *wire.RTCOfferBody
	alias := reference
	alias = &wire.RTCOfferBody{}
	alias.Type = wire.RTCOfferBodyType(value)
	var local localEnvelope
	local.Body.Type = wire.RTCOfferBodyType(value)
}
`}
	assertWireProbeCompiles(t, sources)

	err := checkWireProvenanceProbe(t, sources, models)
	if err == nil || !strings.Contains(err.Error(), "unregistered or unresolved value") {
		t.Fatalf("closed generated enum escaped nested reference controls: %v", err)
	}
}

func assertWireProbeCompiles(t *testing.T, sources map[string]string) {
	t.Helper()

	set := token.NewFileSet()

	files := make([]*ast.File, 0, len(sources))

	for path, source := range sources {
		file, err := parser.ParseFile(set, path, source, 0)
		if err != nil {
			t.Fatalf("parse compile-valid probe %s: %v", path, err)
		}

		files = append(files, file)
	}

	var config types.Config

	config.Importer = wireProbeImporter{base: importer.Default()}

	_, err := config.Check("sample", set, files, nil)
	if err != nil {
		t.Fatalf("provenance probe is not compile-valid: %v", err)
	}
}

type wireProbeImporter struct{ base types.Importer }

func (importer wireProbeImporter) Import(path string) (*types.Package, error) {
	if path == wireImportPath {
		return wireProbePackage(), nil
	}

	pkg, err := importer.base.Import(path)
	if err != nil {
		return nil, fmt.Errorf("import compile probe dependency %q: %w", path, err)
	}

	return pkg, nil
}

func wireProbePackage() *types.Package {
	pkg := types.NewPackage(wireImportPath, generatedModelPackageName)
	stringTypeName := types.NewTypeName(token.NoPos, pkg, wireProvenanceEnumType, nil)
	enumType := types.NewNamed(stringTypeName, types.Typ[types.String], nil)
	pkg.Scope().Insert(stringTypeName)
	pkg.Scope().Insert(types.NewConst(token.NoPos, pkg, wireProvenanceEnumMember, enumType, constant.MakeString("offer")))
	fields := []*types.Var{
		types.NewField(token.NoPos, pkg, "Sdp", types.Typ[types.String], false),
		types.NewField(token.NoPos, pkg, "Type", enumType, false),
		types.NewField(token.NoPos, pkg, "AdditionalProperties", types.NewMap(types.Typ[types.String], types.NewInterfaceType(nil, nil).Complete()), false),
	}
	bodyName := types.NewTypeName(token.NoPos, pkg, wireProvenanceEnumBody, nil)
	bodyType := types.NewNamed(bodyName, types.NewStruct(fields, nil), nil)
	pkg.Scope().Insert(bodyName)
	envelopeFields := []*types.Var{
		types.NewField(token.NoPos, pkg, "Bodies", types.NewSlice(types.NewPointer(bodyType)), false),
	}
	envelopeName := types.NewTypeName(token.NoPos, pkg, "Envelope", nil)
	_ = types.NewNamed(envelopeName, types.NewStruct(envelopeFields, nil), nil)
	pkg.Scope().Insert(envelopeName)
	listFields := []*types.Var{
		types.NewField(token.NoPos, pkg, "Types", types.NewSlice(enumType), false),
	}
	listName := types.NewTypeName(token.NoPos, pkg, "RTCOfferListBody", nil)
	_ = types.NewNamed(listName, types.NewStruct(listFields, nil), nil)
	pkg.Scope().Insert(listName)
	pkg.MarkComplete()

	return pkg
}

func twoDigits(number int) string {
	return fmt.Sprintf("%02d", number)
}
