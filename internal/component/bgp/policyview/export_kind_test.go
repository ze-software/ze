// Design: docs/architecture/core-design.md -- Policy Filter Chain, "The BGP route view".

package policyview

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// TestExportSessionKindRequiresExplicitContext proves missing source facts
// cannot classify a route as local, and known source facts cannot classify it
// as ordinary learned output. Method: build both forms, then supply the
// independently derived engine kind without mutating the original session.
func TestExportSessionKindRequiresExplicitContext(t *testing.T) {
	peer := sessionPeer()
	peer.IsIBGP = true
	for _, sourceKnown := range []bool{false, true} {
		base := ExportSession(peer, peer, sourceKnown)
		if got := base.ExportKind(); got != filterapi.ExportUnknown {
			t.Fatalf("sourceKnown=%v inferred kind=%d", sourceKnown, got)
		}
		if base.UnconfiguredAccepts() {
			t.Fatalf("sourceKnown=%v granted unknown export provenance", sourceKnown)
		}
		for _, kind := range []filterapi.ExportKind{filterapi.ExportOrdinaryLearned, filterapi.ExportExplicit} {
			derived := base.WithExportKind(kind)
			if got := derived.ExportKind(); got != kind {
				t.Fatalf("supplied kind=%d, got %d", kind, got)
			}
			if got := base.ExportKind(); got != filterapi.ExportUnknown {
				t.Fatalf("derived session changed original kind=%d", got)
			}
			if got := derived.UnconfiguredAccepts(); got != (kind == filterapi.ExportExplicit) {
				t.Fatalf("kind=%d: internal export accept=%v", kind, got)
			}
		}
	}
}

// TestExportKindPerRunDoesNotLeak proves a reused route view never carries an
// earlier route's export authority. Method: reset the same view across both
// known kinds and unknown, then clear it and inspect the released context.
func TestExportKindPerRunDoesNotLeak(t *testing.T) {
	var view View
	update := wireu.NewWireUpdate(fullUpdate(), fourOctet(t))
	base := ExportSession(sessionPeer(), filterapi.PeerFilterInfo{}, false)
	for _, kind := range []filterapi.ExportKind{
		filterapi.ExportOrdinaryLearned, filterapi.ExportExplicit, filterapi.ExportUnknown,
	} {
		view.Reset(update, base.WithExportKind(kind))
		if got := view.Session().ExportKind(); got != kind {
			t.Fatalf("reset kind=%d, got %d", kind, got)
		}
	}
	view.Reset(update, base.WithExportKind(filterapi.ExportExplicit))
	view.Clear()
	if got := view.Session().ExportKind(); got != filterapi.ExportUnknown {
		t.Fatalf("clear retained export authority kind=%d", got)
	}
}

// TestImportSessionDefaultIgnoresExportKind proves import keeps its existing
// session-type disposition. Method: attach both kinds to an import context;
// neither may make external import accept or internal import reject.
func TestImportSessionDefaultIgnoresExportKind(t *testing.T) {
	for _, internal := range []bool{false, true} {
		peer := sessionPeer()
		peer.IsIBGP = internal
		for _, kind := range []filterapi.ExportKind{
			filterapi.ExportUnknown, filterapi.ExportOrdinaryLearned, filterapi.ExportExplicit,
		} {
			session := ImportSession(peer).WithExportKind(kind)
			if got := session.UnconfiguredAccepts(); got != internal {
				t.Errorf("internal=%v kind=%d: import accept=%v", internal, kind, got)
			}
		}
	}
}
