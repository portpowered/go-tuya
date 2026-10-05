package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const (
	wireProvenanceSampleFile     = "sample.go"
	wireProvenanceEnumBody       = "RTCOfferBody"
	wireProvenanceEnumType       = "RTCOfferBodyType"
	wireProvenanceEnumMember     = "Offer"
	wireProvenanceGeneratedGroup = "pkg/dependencymodels/rtc.gen.go"
)

func TestGeneratedEnumMutationRejectsFixedDirectPointerAndGlobalValues(t *testing.T) {
	t.Parallel()

	for name, source := range map[string]string{
		"direct": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func mutate() { body := wire.RTCOfferBody{}; body.Type = wire.RTCOfferBodyType("answer") }
`,
		"pointer": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func mutate() { body := wire.RTCOfferBody{}; pointer := &body; pointer.Type = "answer" }
`,
		"global": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
var body wire.RTCOfferBody
func mutate() { body.Type = "answer" }
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: source}, provenanceModels())
			if err == nil || !strings.Contains(err.Error(), "generated enum field") {
				t.Fatalf("generated enum mutation was not rejected: %v", err)
			}
		})
	}
}

func TestGeneratedEnumMutationPreservesSchemaAndCallerValues(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func registered() wire.RTCOfferBodyType { return wire.Offer }
func mutate(value wire.RTCOfferBodyType) {
 body := wire.RTCOfferBody{}
 body.Type = wire.Offer
 body.Type = registered()
 body.Type = value
}
`}, provenanceModels())
	if err != nil {
		t.Fatalf("registered generated enum and caller enum were rejected: %v", err)
	}
}

func TestGeneratedMapStoredInAggregateFieldRetainsRawKeyProvenance(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
type envelope struct { Values map[string]any }
func mutate() {
	 generated := wire.RTCOfferBody{AdditionalProperties: map[string]any{"known": "caller"}}
 stored := envelope{Values: generated.AdditionalProperties}
 stored.Values["audit-unregistered-holder-field-key"] = "value"
}
`}, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-unregistered-holder-field-key") {
		t.Fatalf("generated map stored in an aggregate field escaped raw-key validation: %v", err)
	}
}

func TestURLValuesStoredInAggregateFieldRetainsRawKeyProvenance(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import "net/url"
type envelope struct { Values url.Values }
func mutate(key string) {
 stored := envelope{Values: url.Values{}}
 stored.Values.Set("audit-unregistered-aggregate-query", "value")
 stored.Values.Add(key, "caller")
}
`}, map[string]generatedModel{})
	if err == nil || !strings.Contains(err.Error(), "audit-unregistered-aggregate-query") {
		t.Fatalf("url.Values stored in an aggregate field escaped raw-key validation: %v", err)
	}
}

func TestWireRequestMapReturnedMapRetainsGeneratedProvenanceAcrossSiblingHelper(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{
		"first.go": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func wireRequestMap(value any) (map[string]any, error) { return nil, nil }
func mutate(values map[string]any) { values["audit-unregistered-query"] = "value" }
func build() { result, _ := wireRequestMap(wire.RTCOfferBody{}); mutate(result) }
`,
		"second.go": `package sample
func callAgain() { build() }
`,
	}, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-unregistered-query") {
		t.Fatalf("map returned from generated wire conversion escaped sibling mutation check: %v", err)
	}
}

func TestWireGeneratedFieldRejectsRecursiveAndImportedUnknownValues(t *testing.T) {
	t.Parallel()

	for name, source := range map[string]string{
		"recursive": `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func recursiveKey() string { return recursiveKey() }
func build() { _ = wire.RTCOfferBody{Sdp: recursiveKey()} }
`,
		"imported": `package sample
import (
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
 "strconv"
)
func build() { _ = wire.RTCOfferBody{Sdp: strconv.Itoa(123)} }
`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: source}, provenanceModels())
			if err == nil {
				t.Fatal("unresolved generated field value escaped provenance validation")
			}
		})
	}
}

func TestGeneratedFieldPreservesCallerJoinValues(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import (
 "strings"
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
func Build(values []string, old, replacement string) { _ = wire.RTCOfferBody{Sdp: strings.ReplaceAll(strings.Join(values, ","), old, replacement)} }
`}, provenanceModels())
	if err != nil {
		t.Fatalf("caller-supplied joined wire value was rejected: %v", err)
	}
}

func TestGeneratedFieldHelperSinkRejectsHardcodedCallerAndPreservesPublicInput(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func internalBuild(value string) wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: value} }
func hardcoded() { _ = internalBuild("audit-helper-sink-fixed-value") }
func Build(value string) wire.RTCOfferBody { return internalBuild(value) }
`}, provenanceModels())
	if err == nil || !strings.Contains(err.Error(), "audit-helper-sink-fixed-value") {
		t.Fatalf("hardcoded value passed through helper sink was not rejected: %v", err)
	}
}

func TestGeneratedFieldHelperSinkPreservesBoundPublicInput(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func internalBuild(value string) wire.RTCOfferBody { return wire.RTCOfferBody{Sdp: value} }
func Build(value string) wire.RTCOfferBody { return internalBuild(value) }
`}, provenanceModels())
	if err != nil {
		t.Fatalf("caller-controlled value passed through a helper sink was rejected: %v", err)
	}
}

func TestGeneratedValueCanFlowToNestedReceiverLocalMethod(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
type service struct { client *session }
type devicesService service
type session struct { EncryptedClient *encryptedClient }
type encryptedClient struct{}
func (*encryptedClient) requestOperation(map[string]any) {}
func build(c *devicesService) {
	generated := wire.RTCOfferBody{}
	c.client.EncryptedClient.requestOperation(generated.AdditionalProperties)
}
`}, provenanceModels())
	if err != nil {
		t.Fatalf("generated value passed to a local method through nested receiver fields was rejected: %v", err)
	}
}

func TestGeneratedAggregateFieldDoesNotTaintSiblingRuntimeField(t *testing.T) {
	t.Parallel()

	err := checkWireProvenanceProbe(t, map[string]string{wireProvenanceSampleFile: `package sample
import (
	"regexp"
	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
type envelope struct {
	Payload wire.RTCOfferBody
	Secret string
}
func newEnvelope() envelope {
	return envelope{Payload: wire.RTCOfferBody{}, Secret: "runtime"}
}
func build() {
	value := newEnvelope()
	_, _ = regexp.Compile(value.Secret)
}
`}, provenanceModels())
	if err != nil {
		t.Fatalf("generated data in one aggregate field tainted an unrelated sibling string: %v", err)
	}
}

func TestWireURLValuesSetRequiresKnownQueryKeyGuard(t *testing.T) {
	t.Parallel()

	for name, guarded := range map[string]bool{"guarded": true, "unguarded": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source := `package sample
import (
	"net/url"
	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
func Build(fields map[string]string) {
	query := make(url.Values)
	for name, value := range fields {
` + guardSource(guarded) + `
		query.Set(name, value)
	}
}
`
			set := token.NewFileSet()

			file, err := parser.ParseFile(set, "sample.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}

			assignments := indexWireSourceAssignments(file, "sample.go")

			var mutation *ast.CallExpr

			ast.Inspect(file, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}

				selector, isSelector := call.Fun.(*ast.SelectorExpr)
				if isSelector && selector.Sel.Name == "Set" {
					mutation = call
				}

				return true
			})

			if mutation == nil {
				t.Fatal("query.Set call was not found")
			}

			actual := verifiedWireURLValuesMutation(file, mutation, assignments)
			if actual != guarded {
				t.Fatalf("verifiedWireURLValuesMutation() = %v, want %v", actual, guarded)
			}
		})
	}
}

func guardSource(guarded bool) string {
	if guarded {
		return `		if !wire.IsKnownQueryParam(name) {
			continue
		}`
	}

	return ""
}

func checkWireProvenanceProbe(t *testing.T, sources map[string]string, models map[string]generatedModel) error {
	t.Helper()

	set := token.NewFileSet()
	files := make([]wireSourceFile, 0, len(sources))

	for path, source := range sources {
		file, err := parser.ParseFile(set, path, source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		files = append(files, wireSourceFile{path: path, file: file})
	}

	return checkWireSourcePackage(files, set, models)
}

func provenanceModels() map[string]generatedModel {
	return map[string]generatedModel{
		wireProvenanceEnumBody: {
			Name: wireProvenanceEnumBody, File: wireProvenanceGeneratedGroup, Alias: "",
			Fields: map[string]string{"Sdp": "", "Type": wireProvenanceEnumType, "AdditionalProperties": "map"}, EnumMembers: nil,
		},
		wireProvenanceEnumType: {
			Name: wireProvenanceEnumType, File: wireProvenanceGeneratedGroup, Alias: "", Fields: nil,
			EnumMembers: map[string]string{wireProvenanceEnumMember: "offer"},
		},
	}
}
