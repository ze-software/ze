// Design: docs/architecture/plugin/feature-gates.md -- reflection authority is separate from executable ownership.
package repoinventory

import (
	"strings"
	"testing"
)

// TestExternalSignatureAuthoritySourceSolver retains the production source
// selection, two sink witnesses, metadata producer and opaque-call admission.
func TestExternalSignatureAuthoritySourceSolver(t *testing.T) {
	const mutator = `type root struct{}
func (root) Mutate() { label = "mutated registration metadata" }
`
	for _, scenario := range []struct {
		name, declarations, before, argument, refusal string
	}{
		{
			name: "typed-nil-function-parameter", declarations: mutator + "var callback func(root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "typed-nil-function-conversion", declarations: mutator + "type signature func(root)",
			argument: "signature(nil)", refusal: "metadata",
		},
		{
			name: "typed-nil-function-alias", declarations: mutator + "type signature = func(root)\nvar callback signature",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "typed-nil-function-result", declarations: mutator + "var callback func() *root",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "typed-nil-nested-function-result", declarations: mutator + "var callback func(func() *root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-channel-element", declarations: mutator + "var callback func(chan root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-array-element", declarations: mutator + "var callback func([1]root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-slice-element", declarations: mutator + "var callback func([]root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-map-key", declarations: mutator + "var callback func(map[root]int)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-map-element", declarations: mutator + "var callback func(map[int]root)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "signature-private-struct-field-type", declarations: mutator + "var callback func(struct { hidden root })",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "inert-recursive-descriptor", declarations: "type chain struct { Next *chain }\nvar callback func(chain)",
			argument: "callback",
		},
		{
			name: "generic-typed-nil-field", declarations: mutator + "type holder[T any] struct { Callback func(T); Label string }\nvar container holder[root]",
			argument: "container.Callback", refusal: "metadata",
		},
		{
			name: "generic-scalar-field", declarations: mutator + "type holder[T any] struct { Callback func(T); Label string }\nvar container holder[root]",
			argument: "container.Label",
		},
		{
			name: "method-signature-result-chain", declarations: `type root struct{}
type other struct{}
func (root) Reveal() *other { return nil }
func (other) Mutate() { label = "mutated registration metadata" }
func accept(root) {}`,
			argument: "accept", refusal: "metadata",
		},
		{
			name: "inert-primitive-signature", declarations: "var callback func(int) string",
			argument: "callback",
		},
		{
			name: "inert-channel-descriptor", declarations: "var callback func(chan int)",
			argument: "callback",
		},
		{
			name: "inert-direct-channel-signature", declarations: "func accept(unused chan int) {}",
			argument: "accept",
		},
		{
			name: "ordinary-typed-function-call", declarations: mutator,
			before:   "var invoke func(root) = func(unused root) {}\ninvoke(root{})",
			argument: `"scalar copy"`,
		},
		{
			name: "ordinary-function-conversion-call", declarations: mutator + "type signature func(root)",
			before:   "invoke := signature(func(unused root) {})\ninvoke(root{})",
			argument: `"scalar copy"`,
		},
		{
			name: "ordinary-nil-result-call", declarations: mutator + "func produce() *root { return nil }",
			before: "produce()", argument: `"scalar copy"`,
		},
		{
			name: "ordinary-typed-nil-call-refuses", declarations: mutator + "var invoke func() *root",
			before: "invoke()", argument: `"scalar copy"`, refusal: "callable ownership is unresolved",
		},
		{
			name: "private-method-is-not-reflect-callable", declarations: `type root struct{}
func (root) mutate() { label = "mutated registration metadata" }
func accept(root) {}`,
			argument: "accept",
		},
		{
			name: "bound-receiver-type-hidden", declarations: mutator + "func (root) Accept() {}\nvar receiver root",
			argument: "receiver.Accept",
		},
		{
			name: "nil-interface-has-no-runtime-type", declarations: mutator + "var opaque interface { Anchor(root) }",
			argument: "opaque",
		},
		{
			name: "nil-interface-field-projection", declarations: mutator + "type holder struct { Opaque interface { Anchor(root) }; Label string }\nvar container holder",
			argument: "container.Opaque",
		},
		{
			name: "whole-holder-reveals-interface-field-type", declarations: mutator + "type holder struct { Opaque interface { Anchor(root) }; Label string }\nvar container holder",
			argument: "container", refusal: "metadata",
		},
		{
			name: "interface-holder-scalar-field", declarations: mutator + "type holder struct { Opaque interface { Anchor(root) }; Label string }\nvar container holder",
			argument: "container.Label",
		},
		{
			name: "defined-type-does-not-inherit-source-methods", declarations: mutator + "type distinct root\nvar callback func(distinct)",
			argument: "callback",
		},
		{
			name: "recursive-defined-representation-reenables-child-methods", declarations: `type root struct { Next *root }
func (root) Mutate() { label = "mutated registration metadata" }
type distinct root
var callback func(distinct)`,
			argument: "callback", refusal: "metadata",
		},
		{
			name: "alias-type-keeps-source-methods", declarations: mutator + "type alias = root\nvar callback func(alias)",
			argument: "callback", refusal: "metadata",
		},
		{
			name: "defined-pointer-still-reveals-element", declarations: mutator + "type pointer *root\nvar callback func(pointer)",
			argument: "callback", refusal: "metadata",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			const directory = "internal/signatureauthority"
			const filename = directory + "/proof.go"
			selected := typeIdentitySelectedSource(t, directory, map[string]string{
				"go.mod":                                     "module github.com/ze-software/ze\n\ngo 1.26\n",
				"feature-gates.txt":                          "",
				registrationPackage + "/register.go":         "package registry\nimport _ \"fmt\"\nfunc Register(label string) {}\n",
				"internal/component/config/yang/register.go": "package yang\nimport _ \"fmt\"\n",
				filename: `package signatureauthority
import (
	"fmt"
	registry "github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/component/config/yang"
)
var label = "retained registration metadata"
` + scenario.declarations + `
func init() {
	registry.Register(label)
	` + scenario.before + `
	fmt.Print(` + scenario.argument + `)
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
			if scenario.refusal != "" {
				if err == nil {
					t.Fatalf("source-selected authority was incorrectly admitted; want %s refusal", scenario.refusal)
				}
				if !strings.Contains(err.Error(), scenario.refusal) {
					t.Fatalf("wrong refusal cause: want %s, got %v", scenario.refusal, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("inert source-selected authority was refused: %v", err)
			}
			if len(effects.admitted) != 1 {
				t.Fatalf("successful control did not admit its opaque call: %+v", effects.admitted)
			}
		})
	}
}
