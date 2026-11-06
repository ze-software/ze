// Design: docs/architecture/plugin/feature-gates.md -- type shapes preserve mutable backing references.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalTypeShapeReferenceMetadata keeps the metadata consumer
// on its supported direct slice value. Only the backing allocation's declared
// type changes: aliases and instantiated types must preserve its location.
// The shared helper first exposes a disjoint allocation with equal contents,
// then exposes the exact metadata backing slice through a local alias.
func TestCommittedExternalTypeShapeReferenceMetadata(t *testing.T) {
	for _, shape := range []struct{ name, declarations string }{
		{"alias", "type labelsAlias = []string\nvar Labels labelsAlias = labelsAlias{\"optional\"}\n"},
		{"instantiated", "type labels[T any] []T\nvar Labels labels[string] = labels[string]{\"optional\"}\n"},
	} {
		t.Run(shape.name, func(t *testing.T) {
			externalReferenceMetadataCase(t, shape.declarations)
		})
	}
}

// TestCommittedExternalOpaqueTypeArgumentAuthority supplies an unexported type
// to an opaque generic operation without supplying a value of that type.
// The effect-free method first proves the exact call and origin certificate.
// Changing only its body must retain possible registration authority.
func TestCommittedExternalOpaqueTypeArgumentAuthority(t *testing.T) {
	for _, shape := range []struct{ name, declarations, operation string }{
		{name: "named", operation: "_ = mirror.TypeFor[hidden]()"},
		{name: "alias", declarations: "type hiddenAlias = hidden\n", operation: "_ = mirror.TypeFor[hiddenAlias]()"},
		{name: "pointer", operation: "_ = mirror.TypeFor[*hidden]()"},
		{name: "slice", operation: "_ = mirror.TypeFor[[]hidden]()"},
		{name: "struct-field", operation: "_ = mirror.TypeFor[struct { Value hidden }]()"},
		{name: "function-parameter", operation: "_ = mirror.TypeFor[func(hidden)]()"},
		{name: "interface-signature", operation: "_ = mirror.TypeFor[interface { Inspect(hidden) }]()"},
		{name: "instantiated", declarations: "type carrier[T any] struct { Value T }\n", operation: "_ = mirror.TypeFor[carrier[hidden]]()"},
		{name: "generic-alias", declarations: "type hiddenAlias[T any] = struct { Value T }\n", operation: "_ = mirror.TypeFor[hiddenAlias[hidden]]()"},
		{name: "retained-wrapper", declarations: "func relay() { _ = mirror.TypeFor[hidden]() }\n", operation: "relay()"},
		{name: "escaped-function-value", operation: "_ = mirror.ValueOf(mirror.TypeFor[hidden])"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalOpaqueTypeArgumentFixture(t, kind)
				declarations := "type hidden struct{}\nfunc (hidden) Trigger() { BODY }\n" + shape.declarations
				fixture.goFile(t, callableOwnerCaller, externalOpaqueTypeArgumentSource(
					strings.Replace(declarations, "BODY", "", 1), shape.operation))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				fixture.goFile(t, callableOwnerCaller, externalOpaqueTypeArgumentSource(
					strings.Replace(declarations, "BODY", "install(0)", 1), shape.operation))
				commitInventoryFixture(t, fixture.root)
				externalOpaqueTypeArgumentRefusal(t, fixture, kind)
			})
		}
	}
}

// TestCommittedExternalOpaqueTypeArgumentDoesNotEscape keeps registration
// methods private when an instantiated function is not called or transferred.
// It also distinguishes a methodless type from a same-shaped registration type,
// and preserves ordinary retained calls that do not expose their input.
func TestCommittedExternalOpaqueTypeArgumentDoesNotEscape(t *testing.T) {
	for _, control := range []struct{ name, declarations, operation string }{
		{name: "plain", declarations: "type plain struct{}\n", operation: "_ = mirror.TypeFor[plain]()"},
		{name: "defined-type", declarations: "type distinct hidden\n", operation: "_ = mirror.TypeFor[distinct]()"},
		{name: "opaque-instantiation-local", operation: "instantiated := mirror.TypeFor[hidden]; _ = instantiated"},
		{name: "retained-instantiation-local", declarations: "func keep[T any]() {}\n", operation: "instantiated := keep[hidden]; _ = instantiated"},
		{name: "retained-call-ignores-value", declarations: "func keep(_ hidden) {}\n", operation: "keep(hidden{})"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(control.name+"/"+kind, func(t *testing.T) {
				fixture := externalOpaqueTypeArgumentFixture(t, kind)
				declarations := "type hidden struct{}\nfunc (hidden) Trigger() { BODY }\n" + control.declarations
				fixture.goFile(t, callableOwnerCaller, externalOpaqueTypeArgumentSource(
					strings.Replace(declarations, "BODY", "", 1), control.operation))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				fixture.goFile(t, callableOwnerCaller, externalOpaqueTypeArgumentSource(
					strings.Replace(declarations, "BODY", "install(0)", 1), control.operation))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// externalOpaqueTypeArgumentFixture adds exact reflect import-cycle witnesses
// for both sinks. The ordinary caller reaches that origin and is not a static
// entry for it; hidden.Trigger can escape only through the authored operation.
func externalOpaqueTypeArgumentFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/generic_type_argument_witness.go",
			"package "+sink.name+"\nimport _ \"reflect\"\n")
	}
	return fixture
}

func externalOpaqueTypeArgumentSource(declarations, operation string) string {
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "mirror \"reflect\"",
		prefix:       "_ = mirror.TypeFor[int]\n",
	}, operation)
}

// externalOpaqueTypeArgumentRefusal retains the shared no-partial-output checks
// and requires the semantic sink cause at an authored call or sink source.
// A generic capability unknown is not proof of escaped registration authority.
func externalOpaqueTypeArgumentRefusal(t *testing.T, fixture *asmInventoryFixture, kind string) {
	t.Helper()
	const cause = "registration may execute through an escaped capability"
	externalEffectsRefusal(t, fixture, cause)
	_, reportErr := Collect(fixture.root)
	_, pluginErr := Plugins(fixture.root)
	for _, err := range []error{reportErr, pluginErr} {
		if err == nil {
			t.Fatal("the repeated committed consumer lost its escaped-registration refusal")
		}
		text := err.Error()
		if !strings.Contains(text, "external "+cause) {
			t.Fatalf("type-argument refusal did not reach the semantic registration sink: %v", err)
		}
		if strings.Contains(text, callableOwnerCaller+":") {
			continue
		}
		if strings.Contains(text, callableOwnerFile+":") {
			continue
		}
		switch kind {
		case "plugin":
			if !strings.Contains(text, registrationPackage+"/registry.go:") {
				t.Fatalf("escaped plugin registration has no authored call or sink attribution: %v", err)
			}
		case "yang":
			if !strings.Contains(text, "internal/component/config/yang/register.go:") {
				t.Fatalf("escaped YANG registration has no authored call or sink attribution: %v", err)
			}
		default:
			t.Fatalf("unknown type-argument fixture semantic kind %q", kind)
		}
	}
}
