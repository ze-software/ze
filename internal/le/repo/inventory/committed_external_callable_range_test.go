// Design: docs/architecture/plugin/feature-gates.md -- unknown execution cannot disappear from a union.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalMixedKnownUnknownCallableRefuses keeps an inert callable
// in both programs. Discarding an unsupported yielded value grants no authority;
// invoking its union with the inert callable must retain the unknown owner.
func TestCommittedExternalMixedKnownUnknownCallableRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, callback := range []struct{ name, body string }{
			{"inert", ""},
			{"installer", "helper.Install(0)"},
		} {
			t.Run(kind+"/"+callback.name, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				body := "func Fire() { cb := func() {}; for _, item := range []func(){func() { BODY }} { TARGET = item }; cb() }"
				body = strings.Replace(body, "BODY", callback.body, 1)
				control := strings.Replace(body, "TARGET", "_", 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", control))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				// Only the assignment target changes. The last invocation now has
				// unknown range provenance as well as the known inert literal.
				changed := strings.Replace(body, "TARGET", "cb", 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", changed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "unknown")
			})
		}
	}
}

// TestCommittedExternalFunctionRangeExecutionRefuses distinguishes a private
// dormant iterator from its execution by an exposed public function. This
// bounded model cannot omit that execution even when yielded data is discarded.
func TestCommittedExternalFunctionRangeExecutionRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, iterator := range []struct{ name, body string }{
			{"inert", ""},
			{"installer", "helper.Install(0);"},
		} {
			t.Run(kind+"/"+iterator.name, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				each := "func each(yield func(uint32) bool) { " + iterator.body + " _ = yield(0) }\n"
				fire := "func Fire() { for item := range RANGE_SOURCE { _ = item } }"
				control := each + strings.Replace(fire, "RANGE_SOURCE", "[]uint32{0}", 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", control))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				changed := each + strings.Replace(fire, "RANGE_SOURCE", "each", 1)
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", changed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "range")
			})
		}
	}
}
