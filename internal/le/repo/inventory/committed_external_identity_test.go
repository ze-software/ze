// Design: docs/architecture/plugin/feature-gates.md -- selected import identity precedes external effects.
package repoinventory

import "testing"

// TestCommittedExternalProvenanceRefusals first establishes an inert opaque
// call with both sink witnesses, then changes only the committed resolution
// topology. These are explicit unsupported-identity refusals, not claims that
// every valid workspace or vendor layout changes Go's actual selected package.
// All Go source stays syntactically valid; the cycle invalidates the graph
// theorem, while the deliberately malformed manifest invalidates its authority.
func TestCommittedExternalProvenanceRefusals(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		reason string
		change func(*testing.T, *asmInventoryFixture)
	}{
		{
			name: "replacement module", reason: "identity",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				// A selected import now has an explicit module alternative. The
				// original root package remains present, so basename/prefix
				// resolution cannot establish one authoritative declaration.
				write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\n"+
					"require github.com/ze-software/ze/internal/callableowner v0.0.0\n"+
					"replace github.com/ze-software/ze/internal/callableowner => ./replacement\n")
				write(t, fixture.root, "replacement/go.mod", "module github.com/ze-software/ze/internal/callableowner\n\ngo 1.26\n")
				fixture.goFile(t, "replacement/owner.go", "package ownerhelper\nfunc Install(uint32) {}\nfunc ArgumentInstall() uint32 { return 0 }\n")
			},
		},
		{
			name: "workspace authority", reason: "identity",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				// Even an otherwise ordinary workspace needs an explicit reader
				// contract; ignoring a retained workspace is not that contract.
				write(t, fixture.root, "go.work", "go 1.26\n\nuse .\n")
			},
		},
		{
			name: "selected nested module", reason: "identity",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				write(t, fixture.root, callableOwnerDir+"/go.mod", "module github.com/ze-software/ze/internal/callableowner\n\ngo 1.26\n")
			},
		},
		{
			name: "nested vendor topology", reason: "identity",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				fixture.goFile(t, asmOrdinaryDir+"/vendor/github.com/ze-software/ze/internal/callableowner/owner.go",
					"package ownerhelper\nfunc Install(uint32) {}\nfunc ArgumentInstall() uint32 { return 0 }\n")
			},
		},
		{
			name: "malformed vendor manifest", reason: "identity",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				write(t, fixture.root, "vendor/modules.txt", "example.invalid/orphan/package\n")
			},
		},
		{
			name: "retained import cycle", reason: "cycle",
			change: func(t *testing.T, fixture *asmInventoryFixture) {
				fixture.goFile(t, registrationPackage+"/cycle.go", "package registry\nimport _ \"github.com/ze-software/ze/"+asmOrdinaryDir+"\"\n")
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			scenario.change(t, fixture)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, scenario.reason)
		})
	}
}
