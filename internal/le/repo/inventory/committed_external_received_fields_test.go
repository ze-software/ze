// Design: docs/architecture/plugin/feature-gates.md -- unknown whole values retain field obligations.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalReceivedStructFieldRemainsUnknown keeps the receiver's
// static type and known initial callback unchanged. Receiving an unknown whole
// value must invalidate that field summary before the callback is invoked.
func TestCommittedExternalReceivedStructFieldRemainsUnknown(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			body := "type box struct { Cb func() }\nfunc Fire(ch <-chan box) { var s box = box{Cb: func(){}}; RECEIVE s.Cb() }\n"
			control := strings.Replace(body, "RECEIVE", "", 1)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", control))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			// Only the transfer changes. The same declared box type cannot prove
			// that the newly received value still holds the known inert callback.
			changed := strings.Replace(body, "RECEIVE", "s = <-ch;", 1)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", changed))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "channel")
		})
	}
}
