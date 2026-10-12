// Design: docs/guide/bfd.md -- Editing live
// Related: bfd.go -- applyPinned, reprofileClients
//
// VALIDATES: one commit that deletes a bfd profile together with the last
// client naming it applies, whichever of the two the daemon handles first:
// the BFD apply does not fail on a profile its leaving client still names.
// PREVENTS: a commit removing a BGP peer or static route and the profile it
// named failing its BFD apply with "profile not defined".
package bfd

import "testing"

// clientDeleteSection is clientEditSection without profile "p".
const clientDeleteSection = `{"bfd":{"profile":{` +
	`"q":{` + pinnedEditSlow + `},` +
	`"secure":{"auth":{"type":"keyed-sha1","key-id":"7","secret":"EDIT-SECRET"}}}}}`

// TestClientProfileDeletedBeforeItsClientLeaves has a client hold a session
// under "p", then applies a config with "p" deleted, before the client
// releases: the order a commit takes when the BFD plugin applies before the
// BGP peer or static route naming "p" goes away. The apply succeeds, the
// client keeps the session it holds until it releases it, and the release
// then succeeds.
// MUTATION: reprofileClients answering the "not defined" error for the
// leaving client fails the apply, so this test goes red.
func TestClientProfileDeletedBeforeItsClientLeaves(t *testing.T) {
	svc, loop := pinnedEditService(t, clientEditSection(pinnedEditFast, pinnedEditSlow))
	h, err := svc.EnsureSession(clientEditRequest("p"))
	if err != nil {
		t.Fatalf("EnsureSession(p): %v", err)
	}
	before := onlySession(t, loop)

	applyPinnedEdit(t, svc, clientDeleteSection)

	after := onlySession(t, loop)
	if after.LocalDiscr != before.LocalDiscr || after.DetectMult != before.DetectMult {
		t.Fatalf("after the delete: discriminator %d Detect Mult %d, want the held %d and %d",
			after.LocalDiscr, after.DetectMult, before.LocalDiscr, before.DetectMult)
	}
	if err := svc.ReleaseSession(h); err != nil {
		t.Fatalf("release after the delete: %v", err)
	}
	applyPinnedEdit(t, svc, clientDeleteSection)
}
