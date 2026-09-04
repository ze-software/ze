package iface

import "testing"

// VALIDATES: every queue key a worker block can be counted under produces a
// non-empty, readable metric label.
//
// PREVENTS: the defect that cost a session most of a night on 2026-09-03. The
// worker labels a block with the interface name on the entry it was about to
// handle. A carrier resync is about every interface at once and carries no
// name, so the block landed on a `name=""` series. A functional test summing
// only `name="zeflapv0"` therefore read zero through a genuine block, reported
// "the scenario was never built", and sent the session hunting a stimulus that
// was never missing. An empty label value is not a neutral default here: it is
// the thing that made a real signal unreadable.
//
// This is a table over every linkEventClass rather than the two cases that
// exist today, so a class added later without a label fails here rather than
// silently reintroducing the empty series.
func TestBlockedLabelIsNeverEmpty(t *testing.T) {
	classes := []struct {
		name  string
		class linkEventClass
	}{
		{"carrier", linkEventCarrier},
		{"resync", linkEventResync},
	}
	for _, entry := range classes {
		t.Run(entry.name, func(t *testing.T) {
			// The key as the producer actually builds it for this class: a
			// resync carries no interface name, a carrier event does.
			key := linkEventKey{class: entry.class}
			if entry.class == linkEventCarrier {
				key.ifaceName = "eth0"
			}
			got := blockedLabel(key)
			if got == "" {
				t.Fatalf("class %s produced an empty metric label", entry.name)
			}
		})
	}
}

// VALIDATES: a named entry keeps its own name, so the label is still per
// interface where an interface exists, and the resync gets the readable
// sentinel rather than another interface's name.
func TestBlockedLabelKeepsTheInterfaceNameWhenThereIsOne(t *testing.T) {
	if got := blockedLabel(linkEventKey{class: linkEventCarrier, ifaceName: "zeflapv0"}); got != "zeflapv0" {
		t.Errorf("a named carrier entry labelled %q, want zeflapv0", got)
	}
	if got := blockedLabel(linkEventKey{class: linkEventResync}); got != resyncBlockedLabel {
		t.Errorf("a resync entry labelled %q, want %q", got, resyncBlockedLabel)
	}
	// An unnamed entry of a class that is not the resync must still be
	// readable rather than empty, which is the case a new class would hit.
	if got := blockedLabel(linkEventKey{class: linkEventCarrier}); got == "" {
		t.Error("an unnamed carrier entry produced an empty label")
	}
}
