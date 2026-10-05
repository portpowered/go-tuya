package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGeneratedWireConstructionKeepsCallerJoinValuesOpen(t *testing.T) {
	t.Parallel()

	assertWireConstructionProbe(t, `package tuya
import (
 "strings"
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
func Build(values []string) {
 _ = wire.RTCOfferBody{Sdp: strings.Join(values, ",")}
}
`, false)
}

func TestGeneratedWireConstructionRejectsFixedJoinedValues(t *testing.T) {
	t.Parallel()

	assertWireConstructionProbe(t, `package tuya
import (
 "strings"
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
func Build() {
 _ = wire.RTCOfferBody{Sdp: strings.Join([]string{"unregistered"}, ",")}
}
`, true)
}

func TestGeneratedWireConstructionRejectsShadowedJoinPackage(t *testing.T) {
	t.Parallel()

	assertWireConstructionProbe(t, `package tuya
import (
 "strings"
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
type joiner struct{}
func (joiner) Join(_ []string, sep string) string { return sep }
func Build(strings joiner, values []string) {
 _ = wire.RTCOfferBody{Sdp: strings.Join(values, ",")}
}
`, true)
}

func assertWireConstructionProbe(t *testing.T, source string, wantError bool) {
	t.Helper()

	set := token.NewFileSet()

	file, err := parser.ParseFile(set, "synthetic.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	assignments := indexWireSourceAssignments(file, "synthetic.go")
	models := map[string]generatedModel{
		wireProvenanceEnumBody: {
			Name: wireProvenanceEnumBody, File: wireProvenanceGeneratedGroup, Alias: "",
			Fields: map[string]string{"Sdp": "", "Type": wireProvenanceEnumType}, EnumMembers: nil,
		},
	}

	err = rejectRawGeneratedWireConstructionsWithAssignments(file, set, "pkg/tuya/synthetic.go", models, assignments)
	if wantError && err == nil {
		t.Fatal("fixed wire value escaped provenance gate")
	}

	if !wantError && err != nil {
		t.Fatalf("caller-open string field was rejected: %v", err)
	}

	if wantError && !strings.Contains(err.Error(), "fixed wire") {
		t.Fatalf("unexpected gate error: %v", err)
	}
}
