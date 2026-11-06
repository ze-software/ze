// Design: docs/architecture/plugin/feature-gates.md -- range stores are retained writes.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalRangeAssignmentRetainsCallback distinguishes discarding
// callbacks from storing them through a range-assignment target in a public root.
func TestCommittedExternalRangeAssignmentRetainsCallback(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, writer := range []struct{ name, file, pkg, function string }{
			{"public", externalPublicEntryDir + "/entry.go", "publicentry", "Fire"},
			{"dormant internal", "internal/rangewriter/write.go", "writer", "later"},
		} {
			t.Run(kind+"/"+writer.name, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				body := "func " + writer.function + "() { _ = store.Callback; for _, callback := range []func(){func() { helper.Install(0) }} { _ = callback } }"
				ordinary := externalPublicHelperSource(writer.pkg, body)
				ordinary = strings.Replace(ordinary, "import helper", "import store \"github.com/ze-software/ze/"+externalNeutralDir+"\"\nimport helper", 1)
				fixture.goFile(t, writer.file, ordinary)
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				changed := "func " + writer.function + "() { _ = store.Callback; for _, store.Callback = range []func(){func() { helper.Install(0) }} {} }"
				changedSource := externalPublicHelperSource(writer.pkg, changed)
				changedSource = strings.Replace(changedSource, "import helper", "import store \"github.com/ze-software/ze/"+externalNeutralDir+"\"\nimport helper", 1)
				fixture.goFile(t, writer.file, changedSource)
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "range")
			})
		}
	}
}
