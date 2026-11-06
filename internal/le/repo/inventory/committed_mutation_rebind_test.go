// Design: docs/architecture/plugin/feature-gates.md -- a local rebind never writes the declaration it was initialized from.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedMutationQueriesSeparateRebindFromWrite proves an assignment to a
// bare local identifier is a rebind of that local, while a write through the
// alias and a direct assignment to the declaration stay refused. The fixture is
// the flowspec family selection shape: a struct-typed package declaration
// copied into a local, which a branch then rebinds to a sibling declaration.
func TestCommittedMutationQueriesSeparateRebindFromWrite(t *testing.T) {
	root := committedFixture(t)
	const directory = "internal/plugins/example"
	const name = directory + "/rebind_controls.go"
	const declarations = `package example
import names "github.com/ze-software/ze/internal/shared"
type Family struct{ AFI uint16 }
var FlowV4 = Family{AFI: 1}
var FlowV6 = Family{AFI: 2}
var Counter = 1
var RangeSeed = Family{AFI: 3}
var AliasStruct = Family{AFI: 4}
var AliasPointer = &Family{AFI: 5}
var DirectLocal = Family{AFI: 6}
func selectFamily(isIPv6 bool) Family {
 fam := FlowV4
 if isIPv6 {
  fam = FlowV6
 }
 return fam
}
func countAlias() int { count := Counter; count++; return count }
func rangeAlias() Family {
 seed := RangeSeed
 for _, seed = range []Family{{AFI: 7}} {}
 return seed
}
func writeStructAlias() { alias := AliasStruct; alias.AFI = 8; _ = alias }
func writePointerAlias() { alias := AliasPointer; alias.AFI = 9 }
func writeDirect() { DirectLocal = Family{AFI: 10} }
func writeRemote() { names.DirectRemote = 11 }
`
	write(t, root, name, declarations)
	write(t, root, "internal/shared/rebind_controls.go", "package shared\nvar DirectRemote = 1\n")
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "rebind controls")
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	reader := snapshot.evaluator()

	// MUTATION: delete the `continue` under `if rebindsLocal(target, reference, file.file)`
	// in declarationMutationFile.writes, and each rebind below reads as a write.
	for _, binding := range []string{"FlowV4", "Counter", "RangeSeed"} {
		source := mutationQueryGlobal(t, snapshot, directory, binding)
		if err := reader.immutable(source); err != nil {
			t.Errorf("rebinding a local alias of %s was read as its mutation: %v", binding, err)
		}
	}

	// MUTATION: replace `if rebindsLocal(target, reference, file.file) {` with
	// `if true {` in declarationMutationFile.writes, and every write below is
	// skipped. For DirectLocal alone, replacing
	// `return file.Scope.Lookup(name.Name) != object` with `return true` in
	// rebindsLocal also turns it red: the package declaration reads as a local.
	for _, scenario := range []struct {
		directory string
		binding   string
	}{
		{directory, "AliasStruct"},
		{directory, "AliasPointer"},
		{directory, "DirectLocal"},
		{"internal/shared", "DirectRemote"},
	} {
		source := mutationQueryGlobal(t, snapshot, scenario.directory, scenario.binding)
		failure := reader.immutable(source)
		if failure == nil {
			t.Errorf("a write to %s was certified immutable", scenario.binding)
			continue
		}
		if !strings.Contains(failure.Error(), "declaration input "+scenario.binding+" is mutated after initialization") {
			t.Errorf("%s lost its mutation cause: %v", scenario.binding, failure)
		}
	}
}
