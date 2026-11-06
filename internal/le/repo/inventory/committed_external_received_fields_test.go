// Design: docs/architecture/plugin/feature-gates.md -- unknown whole values retain field obligations.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalReceivedStructFieldRemainsUnknown keeps the receiver's
// static type and known initial callback unchanged. Fire is a public entry, so
// its channel holds what external code sends: the received whole value carries
// the ambient origins and dispatches as external, never as the earlier inert
// field. Method: both ch forms, the control and the receiving form, publish,
// because the ambient pool holds no registering callable.
// A package channel q has no ambient origins, so its receive keeps the field
// obligation and the call refuses, while the same Fire without it publishes.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// "result.unresolvedFields = \"received channel element field ownership requires a retained element summary\"",
// "result.unresolvedFields = \"\"") lets the received value borrow the inert
// field, so the q form publishes.
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
			// The received value dispatches over the inert literal and the
			// ambient callables external code holds; none registers here.
			changed := strings.Replace(body, "RECEIVE", "s = <-ch;", 1)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", changed))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			// A package channel is no parameter, so it holds no ambient
			// origins: the received whole value keeps its field obligation and
			// cannot borrow the earlier inert field.
			const queue = "type box struct { Cb func() }\nvar q = make(chan box, 1)\nfunc Fire() { var s box = box{Cb: func(){}}; RECEIVE s.Cb() }\n"
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", strings.Replace(queue, "RECEIVE", "", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", externalPublicHelperSource("publicentry", strings.Replace(queue, "RECEIVE", "s = <-q;", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "received channel element field ownership")
		})
	}
}
