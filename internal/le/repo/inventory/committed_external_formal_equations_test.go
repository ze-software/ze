// Design: docs/architecture/plugin/feature-gates.md -- complete retained caller provenance.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalFormalCallerEquations pairs exact ordinary inventories
// with dormant argument, forwarding and unknown-caller changes. The dormant
// functions are source equations under the later-write contract, not claims
// that these bodies execute during actual initialization.
func TestCommittedExternalFormalCallerEquations(t *testing.T) {
	const stat = "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
	const stash = "func stash(cb func()) { store.Callback = cb }\n"
	for _, kind := range []string{"plugin", "yang"} {
		for _, scenario := range []struct {
			name         string
			neutral      string
			declarations string
			calls        string
			changed      string
			changedCalls string
			reason       string
		}{
			{
				name: "forwarded dormant argument",
				declarations: stash + "func forward(cb func()) { stash(cb) }\n" +
					"func later() { forward(func() { BODY }) }\n",
				calls:  "forward(func() {}); " + stat,
				reason: "registration",
			},
			{
				name:         "explicit nil versus absent authority",
				declarations: stash,
				calls:        "stash(nil); " + stat,
				changed:      stash,
				changedCalls: stat,
				reason:       "unknown",
			},
			{
				name: "grounded nil versus ungrounded recursion",
				declarations: stash + "func forward(cb func()) { stash(cb); forward(cb) }\n" +
					"func later() { forward(nil) }\n",
				calls:   stat,
				changed: stash + "func forward(cb func()) { stash(cb); forward(cb) }\n",
				reason:  "unknown",
			},
			{
				name:         "primitive scalar versus unbound callable",
				neutral:      externalNeutralSource + "var Count uint32\n",
				declarations: "func stashScalar(number uint32) { store.Count = number }\n",
				calls:        stat,
				changed:      stash,
				reason:       "unknown",
			},
			{
				name: "direct caller versus unresolved alias caller",
				declarations: stash +
					"func later() { stash(func() {}) }\n",
				calls: "stash(func() {}); " + stat,
				changed: stash +
					"func later() { alias := stash; alias(func() { install(0) }) }\n",
				reason: "unknown",
			},
			{
				name: "dormant factory execution stays possible",
				declarations: stash + "func produce() func() { BODY; return func() {} }\n" +
					"func later() { stash(produce()) }\n",
				calls:  stat,
				reason: "registration",
			},
		} {
			t.Run(kind+"/"+scenario.name, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				if scenario.neutral != "" {
					fixture.goFile(t, externalNeutralFile, scenario.neutral)
				}
				positive := strings.ReplaceAll(scenario.declarations, "BODY", "")
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(positive, scenario.calls))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				negative := scenario.changed
				if negative == "" {
					negative = strings.ReplaceAll(scenario.declarations, "BODY", "install(0)")
				}
				calls := scenario.changedCalls
				if calls == "" {
					calls = scenario.calls
				}
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(negative, calls))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, scenario.reason)
			})
		}
	}
}
