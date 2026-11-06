// Design: docs/architecture/plugin/feature-gates.md -- returned concrete types retain reflection authority.
package repoinventory

import (
	"strings"
	"testing"
)

// TestExternalSignatureReturnedValueAuthoritySolver distinguishes invoking a
// retained producer from exposing its result. Its body returns no callable node
// carrying the declared concrete result type. Controls still invoke that body.
func TestExternalSignatureReturnedValueAuthoritySolver(t *testing.T) {
	const mutator = "type root string\nfunc (root) Mutate() { label = \"mutated registration metadata\" }\n"
	for _, scenario := range []struct {
		name, declarations, before, argument string
		mutates                              bool
	}{
		{
			name: "direct-nil-function-result", declarations: "func produce() func(root) { return nil }",
			argument: "produce()", mutates: true,
		},
		{
			name: "direct-nil-pointer-result", declarations: "func produce() *root { return nil }",
			argument: "produce()", mutates: true,
		},
		{
			name: "inferred-nil-function-result", declarations: "func produce() func(root) { return nil }",
			before: "produced := produce(); _ = produced", argument: "produced", mutates: true,
		},
		{
			name: "inferred-nil-pointer-result", declarations: "func produce() *root { return nil }",
			before: "produced := produce(); _ = produced", argument: "produced", mutates: true,
		},
		{
			name: "aliased-nil-function-result", declarations: "type signature = func(root)\nfunc produce() signature { return nil }",
			argument: "produce()", mutates: true,
		},
		{
			name: "instantiated-nil-function-result", declarations: "type signature[T any] func(T)\nfunc produce() signature[root] { return nil }",
			argument: "produce()", mutates: true,
		},
		{
			name: "named-scalar-constant-result", declarations: "func produce() root { return \"\" }",
			argument: "produce()", mutates: true,
		},
		{
			name: "nil-interface-result", declarations: "type opaque interface { Anchor(root) }\nfunc produce() opaque { return nil }",
			argument: "produce()",
		},
		{
			name: "nil-instantiated-interface-result", declarations: "type opaque[T any] interface { Anchor(T) }\nfunc produce() opaque[root] { return nil }",
			argument: "produce()",
		},
		{
			name: "inert-nil-function-result", declarations: "func produce() func(int) { return nil }",
			argument: "produce()",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, exposure := range []struct {
				name   string
				expose bool
			}{
				{name: "scalar-control"},
				{name: "returned-value", expose: true},
			} {
				t.Run(exposure.name, func(t *testing.T) {
					argument := `"scalar copy"`
					if exposure.expose {
						argument = scenario.argument
					}
					// The control invokes the same producer and creates the same
					// inferred local. Only the final opaque argument changes.
					statements := "_ = produce()\n" + scenario.before + "\nfmt.Print(" + argument + ")"
					effects, file := externalTerminalSelectedSource(t, mutator+scenario.declarations, statements)
					err := effects.solve()
					metadata := file.Scope.Lookup("label")
					if metadata == nil || !effects.metadata[metadata] {
						t.Fatal("selected return fixture lost its actual registration metadata")
					}
					if !exposure.expose || !scenario.mutates {
						if err != nil {
							t.Fatalf("unexposed or inert returned type was refused: %v", err)
						}
						externalTerminalAccepted(t, effects, file)
						return
					}
					if err == nil {
						t.Fatal("typed returned value lost its metadata-mutating reflection authority")
					}
					if !strings.Contains(err.Error(), "metadata") {
						t.Fatalf("returned type refusal lost its metadata cause: %v", err)
					}
					if len(effects.admitted) != 0 {
						t.Fatalf("returned type refusal admitted operations: %+v", effects.admitted)
					}
				})
			}
		})
	}
}

// TestExternalSignatureReturnedNilCallableKeepsOwnership prevents a declared
// result descriptor from inventing an executable implementation for nil.
func TestExternalSignatureReturnedNilCallableKeepsOwnership(t *testing.T) {
	for _, invoke := range []bool{false, true} {
		name := "discarded-result"
		call := ""
		if invoke {
			name = "nil-result-call"
			call = "produce()()\n"
		}
		t.Run(name, func(t *testing.T) {
			declarations := "type root string\nfunc (root) Mutate() { label = \"mutated registration metadata\" }\nfunc produce() func() *root { return nil }\n"
			effects, file := externalTerminalSelectedSource(t, declarations, "_ = produce()\n"+call+`fmt.Print("scalar copy")`)
			err := effects.solve()
			if !invoke {
				if err != nil {
					t.Fatalf("discarding the typed result exposed its descriptor: %v", err)
				}
				externalTerminalAccepted(t, effects, file)
				return
			}
			if err == nil || !strings.Contains(err.Error(), "callable ownership is unresolved") {
				t.Fatalf("nil result borrowed a reflected method as its implementation: %v", err)
			}
			if !effects.metadata[file.Scope.Lookup("label")] {
				t.Fatal("nil-result call lost its actual registration metadata")
			}
			if len(effects.admitted) != 0 {
				t.Fatalf("nil-result call refusal admitted operations: %+v", effects.admitted)
			}
		})
	}
}
