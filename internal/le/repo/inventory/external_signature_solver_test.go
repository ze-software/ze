// Design: docs/architecture/plugin/feature-gates.md -- exposed callable signatures retain ordinary reflection authority.
package repoinventory

import (
	"strings"
	"testing"
)

// TestExternalSignatureTypeCapabilitiesSolver uses production source selection,
// cycle witnesses and the external-effect solver. It seeds no private metadata,
// ambient capabilities, bindings or registration classifications.
func TestExternalSignatureTypeCapabilitiesSolver(t *testing.T) {
	for _, scenario := range []struct {
		name, declaration string
	}{
		{name: "unused-named-parameter", declaration: "func accept(unused root) {}"},
		{name: "unnamed-parameter", declaration: "func accept(root) {}"},
		{name: "nested-function-parameter", declaration: "func accept(unused func(root)) {}"},
		{name: "nested-interface-parameter", declaration: "func accept(unused interface { Anchor(root) }) {}"},
		{name: "nil-pointer-result", declaration: "func accept() *root { return nil }"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, exposure := range []struct {
				name, argument string
				mutates        bool
			}{
				{name: "scalar-control", argument: `"scalar copy"`},
				{name: "callable-signature", argument: "accept", mutates: true},
			} {
				t.Run(exposure.name, func(t *testing.T) {
					const directory = "internal/signatureproof"
					const filename = directory + "/proof.go"
					selected := typeIdentitySelectedSource(t, directory, map[string]string{
						"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
						"feature-gates.txt":                          "",
						registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
						"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
						filename: `package signatureproof
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
type root struct{}
func (root) Mutate() { label = "mutated registration metadata" }
` + scenario.declaration + `
func init() {
	registry.Register(label)
	fmt.Print(` + exposure.argument + `)
}
`,
					})
					source, err := newExternalSource(selected.source.snapshot)
					if err != nil {
						t.Fatal(err)
					}
					effects := newExternalEffects(source)
					err = effects.solve()
					file := typeIdentitySelectedFile(t, source.snapshot.packages[directory], filename)
					metadata := file.Scope.Lookup("label")
					if !effects.metadata[metadata] {
						t.Fatal("source-selected fixture did not retain registration metadata")
					}
					if exposure.mutates {
						if err == nil {
							t.Fatal("exposed callable signature lost its metadata-mutating type method")
						}
						if !strings.Contains(err.Error(), "metadata") {
							t.Fatalf("signature-type refusal lost its metadata cause: %v", err)
						}
						return
					}
					if err != nil {
						t.Fatalf("inert scalar exposed an unreachable callable signature: %v", err)
					}
					if len(effects.admitted) != 1 {
						t.Fatalf("scalar control did not admit its external call: %+v", effects.admitted)
					}
				})
			}
		})
	}
}

// The permitted opaque consumer can recover root's reflect.Type without calling
// accept: TypeOf(fn).In(0) for direct parameters; In(0).In(0) for the nested
// function; In(0).Method(0).Type.In(0) for the interface signature; and Out(0).Elem()
// for the nil-pointer result. In each case Zero(rootType).MethodByName("Mutate").Call(nil)
// invokes the exported value method on a valid zero value of the private type.
// Neither sink is statically importable by fmt because both retained sinks import
// fmt. The root package imports fmt too, excluding an independent static root.
// The scalar controls retain exactly the same dormant declarations and metadata.
