// Design: docs/architecture/plugin/feature-gates.md -- flattening retains nested capabilities and precise root fields.
package repoinventory

import (
	"go/ast"
	"strings"
	"testing"
)

// TestExternalFlatNestedCallbackSolver drives the production source selection,
// external-entry witness and effect solver without requiring fixture Git commits.
// Signed public-API fixtures separately prove capture and publication behavior.
func TestExternalFlatNestedCallbackSolver(t *testing.T) {
	for _, scenario := range []struct {
		name, argument string
		mutates        bool
	}{
		{name: "scalar-sibling", argument: "holder.Label"},
		{name: "nested-callable", argument: "holder", mutates: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			const directory = "internal/flatproof"
			const filename = directory + "/proof.go"
			selected := typeIdentitySelectedSource(t, directory, map[string]string{
				"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
				"feature-gates.txt":                          "",
				registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
				"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
				filename: `package flatproof
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
type inner struct { Label func() }
type box struct { Inner inner; Label string }
func init() {
	registry.Register(label)
	holder := box{Inner: inner{Label: func() { label = "mutated registration metadata" }}, Label: "scalar copy"}
	fmt.Print([]any{` + scenario.argument + `})
}
`,
			})
			source, err := newExternalSource(selected.source.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			effects := newExternalEffects(source)
			err = effects.solve()
			if scenario.mutates {
				if err == nil {
					t.Fatal("whole holder lost its nested metadata-mutating callback")
				}
				if !strings.Contains(err.Error(), "metadata") {
					t.Fatalf("nested callback refusal lost its metadata cause: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("scalar sibling borrowed the nested callback: %v", err)
			}
			file := typeIdentitySelectedFile(t, source.snapshot.packages[directory], filename)
			metadata := file.Scope.Lookup("label")
			if !effects.metadata[metadata] {
				t.Fatal("successful scalar control did not retain registration metadata")
			}
			if len(effects.admitted) != 1 {
				t.Fatalf("scalar control did not admit its external call: %+v", effects.admitted)
			}
		})
	}
}

// TestExternalFlatCutoffAndOwnership checks the abstract-domain boundary that
// cannot be reached by a shallow source fixture without changing other limits.
func TestExternalFlatCutoffAndOwnership(t *testing.T) {
	rootCallback := ast.NewIdent("rootCallback")
	childCallback := ast.NewIdent("childCallback")
	excludedCallback := ast.NewIdent("excludedCallback")
	location := &ast.Object{Name: "retained location"}
	read := &ast.Object{Name: "retained read"}
	excluded := externalValue{
		callables: map[ast.Node]sourceExpr{excludedCallback: {value: excludedCallback}},
		unknown:   "excluded node unknown", references: true,
	}
	child := externalValue{
		callables: map[ast.Node]sourceExpr{childCallback: {value: childCallback}},
		locations: map[*ast.Object]bool{location: false}, reads: map[*ast.Object]bool{read: false},
		origins: map[string]bool{"retained origin": false}, unresolvedFields: "child selector obligation",
	}
	root := externalValue{
		callables: map[ast.Node]sourceExpr{rootCallback: {value: rootCallback}},
		fields: map[string]externalValue{
			"branch":  {unknown: "nearer unknown", fields: map[string]externalValue{"excluded": excluded}},
			"sibling": child,
		},
		unresolvedFields: "root selector obligation",
	}
	flat := externalFlat(root, 126)
	if flat.unknown != "nearer unknown" {
		t.Fatalf("nearer unknown lost precedence over the cutoff: %q", flat.unknown)
	}
	if len(flat.callables) != 2 {
		t.Fatalf("reachable callable union changed at the boundary: %+v", flat.callables)
	}
	if flat.callables[rootCallback].value != rootCallback {
		t.Fatal("flattening lost the root callable source")
	}
	if flat.callables[childCallback].value != childCallback {
		t.Fatal("a cutoff or unknown prevented visiting the surviving sibling")
	}
	if !flat.locations[location] {
		t.Fatal("flattening lost a reachable location map key")
	}
	if !flat.reads[read] {
		t.Fatal("flattening lost a reachable metadata read map key")
	}
	if !flat.origins["retained origin"] {
		t.Fatal("flattening lost a reachable external origin map key")
	}
	if !flat.references {
		t.Fatal("a retained location no longer implies reference capability")
	}
	if reference := externalFlat(externalValue{references: true}, 0); !reference.references {
		t.Fatal("flattening lost an explicit reference without a location")
	}
	if len(flat.fields) != 0 {
		t.Fatal("flattening retained a child root field name")
	}
	if flat.unresolvedFields != "" {
		t.Fatal("flattening retained a root selector obligation")
	}
	root.unknown = "root unknown"
	if first := externalFlat(root, 126); first.unknown != root.unknown {
		t.Fatalf("root unknown lost first precedence: %q", first.unknown)
	}
	root.references = true
	cutoff := externalFlat(root, 128)
	if cutoff.unknown != "function capture structure exceeds 128 steps" {
		t.Fatalf("cutoff consumed the excluded root's own unknown: %q", cutoff.unknown)
	}
	if cutoff.callables != nil || cutoff.locations != nil || cutoff.reads != nil || cutoff.origins != nil || cutoff.fields != nil {
		t.Fatal("a root at the cutoff allocated or retained capability maps")
	}
	if cutoff.references || cutoff.unresolvedFields != "" {
		t.Fatal("a root at the cutoff retained an excluded capability or selector obligation")
	}
	// Changing the returned union must not edit any input or another output.
	independent := externalFlat(root, 126)
	delete(flat.callables, childCallback)
	delete(flat.locations, location)
	delete(flat.reads, read)
	delete(flat.origins, "retained origin")
	// join allocates the field table only when a value carries fields, so the
	// mutation makes its own before writing a field into the flattened output.
	if flat.fields == nil {
		flat.fields = make(map[string]externalValue)
	}
	flat.fields["new root"] = externalValue{}
	if child.callables[childCallback].value != childCallback || independent.callables[childCallback].value != childCallback {
		t.Fatal("flattened callable maps share mutable input or output storage")
	}
	if len(child.locations) != 1 || !independent.locations[location] {
		t.Fatal("flattened location maps share mutable input or output storage")
	}
	if len(child.reads) != 1 || !independent.reads[read] {
		t.Fatal("flattened read maps share mutable input or output storage")
	}
	if len(child.origins) != 1 || !independent.origins["retained origin"] {
		t.Fatal("flattened origin maps share mutable input or output storage")
	}
	if len(root.fields) != 2 || len(independent.fields) != 0 {
		t.Fatal("flattened root fields share mutable input or output storage")
	}
	if root.unresolvedFields != "root selector obligation" || child.unresolvedFields != "child selector obligation" {
		t.Fatal("flattening cleared an input's selector obligation")
	}
}
