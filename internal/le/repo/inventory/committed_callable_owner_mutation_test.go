// Design: docs/architecture/plugin/feature-gates.md -- a callable initializer is not its owner after rebinding.
package repoinventory

import "testing"

// Distinct A/B installers distinguish the invoked B from both an incomplete
// ordinary-only report and a report that newly publishes the uncalled A.
func TestCommittedCallableMutationOwnersCannotFallBack(t *testing.T) {
	cases := []struct {
		name         string
		declarations string
		call         string
	}{
		{"reused-short-declaration", "", "invoke := uncalled; invoke, marker := install, 0; _ = marker; invoke(ownerProbe())"},
		{"value-spec-pointer-alias", "", "invoke := uncalled; var pointer = &invoke; *pointer = install; invoke(ownerProbe())"},
		{"helper-parameter-address-escape", "func replaceOwner(pointer *func(uint32)) { *pointer = install }", "invoke := uncalled; replaceOwner(&invoke); invoke(ownerProbe())"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			for _, kind := range []string{"plugin", "yang"} {
				t.Run(kind, func(t *testing.T) {
					other := "func uncalled(_ uint32) { _ = r.Register(r.Registration{Name: \"uncalled-plugin\", Description: \"uncalled executable owner\", RunEngine: engine}) }\n"
					imported := "r \"github.com/ze-software/ze/internal/component/plugin/registry\""
					if kind == "yang" {
						other = "func uncalled(_ uint32) { y.RegisterModule(\"uncalled.yang\", \"module uncalled { namespace urn:uncalled; prefix u; }\") }\n"
						imported = "y \"github.com/ze-software/ze/internal/component/config/yang\""
					}
					candidate := callableOwnerCase{
						declarations: other + scenario.declarations,
						extraImport:  imported,
						call:         scenario.call,
					}
					fixture := callableOwnerFixture(t, kind, candidate)
					fixture.goFile(t, callableOwnerCaller, callableOwnerSource(candidate, candidate.call))
					commitInventoryFixture(t, fixture.root)
					callableOwnerOutcome(t, fixture, kind, true)
				})
			}
		})
	}
}
