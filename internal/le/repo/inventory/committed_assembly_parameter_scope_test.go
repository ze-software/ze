// Design: docs/architecture/plugin/feature-gates.md -- generated aliases cannot escape lexical type-parameter scope.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedAssemblyGeneratedParameterScopeCannotFallBack establishes each
// generic declaration WITHOUT an instantiation as an independent full positive.
// Adding an alias of that generic parameter must either resolve its real struct
// field and collide, or explicitly refuse unknown Alias membership. A package
// type or predeclared scalar with the same spelling is never a valid fallback.
func TestCommittedAssemblyGeneratedParameterScopeCannotFallBack(t *testing.T) {
	cases := []struct {
		name         string
		declarations string
	}{
		{"package-scalar", "package headerprobe\ntype T int\ntype Box[T any] = T\n"},
		{"package-struct", "package headerprobe\ntype T struct { Wrong int }\ntype Box[T any] = T\n"},
		{"predeclared-scalar", "package headerprobe\ntype Box[int any] = int\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			declarations := test.declarations + "type Direct[Value any] struct { Field Value }\ntype DirectAlias = Direct[int]\ntype Holder struct { Item Direct[int] }\n"
			unit := "#include \"go_asm.h\"\n#define Alias_Field 0\n" + asmHeaderNumericBody
			fixture := asmHeaderFixture(t, declarations, unit)
			// Retain resolved direct-generic-struct and nongeneric-holder
			// positives while the SAME parameter-dependent Box is unused.
			// Without Alias, its Alias_Field source definition is valid.
			fixture.goFile(t, asmHeaderProbeFile, declarations+"type Alias = Box[struct { Field int }]\n")
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err == nil {
				t.Fatalf("lexical parameter fallback silently omitted Alias_Field and accepted its source definition: %+v", inv)
			}
			if inv.Plugins != nil {
				t.Fatalf("parameter-dependent refusal leaked partial plugins: %+v: %v", inv.Plugins, err)
			}
			asmHeaderParameterScopeCause(t, err)
			snapshot, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatalf("source capture failed before parameter-scope admission: %v", err)
			}
			plugins, err := snapshot.Plugins()
			if plugins != nil {
				t.Fatalf("retained parameter-dependent refusal leaked partial plugins: %+v", plugins)
			}
			asmHeaderParameterScopeCause(t, err)
		})
	}
}

func asmHeaderParameterScopeCause(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("parameter-dependent generated Alias membership was falsely accepted")
	}
	message := err.Error()
	if strings.Contains(message, "unsupported generated assembly type Alias") {
		t.Logf("explicit parameter-dependent Alias membership refusal: %v", err)
		return
	}
	if strings.Contains(message, "Alias_Field") {
		if strings.Contains(message, "redefined") {
			t.Logf("proved instantiated Alias_Field collision: %v", err)
			return
		}
	}
	t.Fatalf("parameter-scope discriminator is masked or wrong: %v; need explicit unsupported Alias or proved Alias_Field redefinition", err)
}
