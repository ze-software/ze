// Design: docs/architecture/plugin/feature-gates.md -- generated and shadow evidence belongs to one retained revision.
package repoinventory

import "testing"

// TestCommittedAssemblyHeaderEvidenceRetainsSnapshot proves lazy header
// derivation cannot reopen dirty or later-committed declarations/presence.
// Its positive prerequisite needs the static export walk to expose the
// generated declarations' exported generic Box[T any] uninstantiated, with T
// reflected as Box's instantiations.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// `	if parameters != 0 && len(arguments) != 0 {`, `	if parameters != 0 {`):
// the bare walk rebinds T with no argument, and the prerequisite refuses as a
// reflected value parameter with no retained argument.
// MUTATION: (internal/le/repo/inventory/externalreflectionroots.go,
// `		if len(arguments) != 0 && len(arguments) != definition.TypeParams.NumFields() {`,
// `		if len(arguments) != definition.TypeParams.NumFields() {`): the bare
// root refuses its argument count.
// MUTATION: (internal/le/repo/inventory/externalreflectiontypes.go,
// `	if len(arguments) != 0 && len(arguments) != definition.TypeParams.NumFields() {`,
// `	if len(arguments) != definition.TypeParams.NumFields() {`): the bare
// descriptor refuses its argument count.
func TestCommittedAssemblyHeaderEvidenceRetainsSnapshot(t *testing.T) {
	for _, kind := range []string{"generated-declarations", "textflag.h", "funcdata.h", "go_asm.h"} {
		t.Run(kind, func(t *testing.T) {
			fixture := asmHeaderFixture(t, asmHeaderLZ4Go, "#include \"go_asm.h\"\n#include \"funcdata.h\"\n#include \"textflag.h\"\n"+asmHeaderNumericBody)
			expected := asmHeaderExpected(fixture)
			snapshot, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			revision := snapshot.Revision()
			freshPaths := asmHeaderPaths()
			cause := []string{kind, "shadows the compiler header"}
			if kind == "generated-declarations" {
				fixture.goFile(t, asmHeaderProbeFile, asmHeaderLZ4Go+"type GOOS struct { linux int }\n")
				later := asmHeaderProbeDir + "/later.go"
				fixture.goFile(t, later, "package headerprobe\nfunc LaterMarker() {}\n")
				freshPaths = asmHeaderPaths(later)
				cause = []string{"GOOS_linux", "redefined"}
			} else {
				write(t, fixture.root, asmHeaderProbeDir+"/"+kind, "#define dirty_or_later_shadow 1\n")
			}
			// Public Collect must ignore dirty content AND dirty new paths. Its
			// expected report corpus was frozen before fixture.goSources changed.
			asmHeaderPositive(t, expected, asmHeaderPaths())
			plugins, err := snapshot.Plugins()
			if err != nil {
				t.Fatalf("dirty input changed retained header evidence: %v", err)
			}
			expected.checkPlugins(t, plugins)
			asmHeaderSelectedPaths(t, snapshot, asmHeaderPaths())
			commitInventoryFixture(t, fixture.root)
			if snapshot.Revision() != revision {
				t.Fatal("captured header snapshot revision moved")
			}
			fresh, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			if fresh.Revision() == revision {
				t.Fatal("new committed header mutation did not produce a fresh revision")
			}
			asmHeaderSelectedPaths(t, fresh, freshPaths)
			asmHeaderRefusal(t, fixture, cause...)
			plugins, err = snapshot.Plugins()
			if err != nil {
				t.Fatalf("later HEAD changed retained header evidence: %v", err)
			}
			expected.checkPlugins(t, plugins)
			asmHeaderSelectedPaths(t, snapshot, asmHeaderPaths())
		})
	}
}
