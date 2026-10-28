package rib

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRIBFamilyWithNoSplitterWarnsOnce proves routes of a family the RIB
// cannot split are reported at Warn, naming the peer and the fix, and only once
// per peer and family, rather than dropped behind one Debug line.
// Method: a capturing logger at Info, two UPDATEs of a family no splitter
// serves from one peer, then one from a second peer.
func TestRIBFamilyWithNoSplitterWarnsOnce(t *testing.T) {
	unstorable := family.Family{AFI: family.AFIIPv4, SAFI: 250}
	if nlrisplit.Supported(unstorable) {
		t.Fatalf("fixture family %s has a splitter; pick one that has none", unstorable)
	}
	buf := captureRIBLog(t)
	r := newTestRIBManager(t)

	first := storage.NewPeerRIB("192.0.2.1")
	r.insertPoolNLRIs(first, unstorable, []byte{8, 10}, nil, false, 1, 0, false)
	r.insertPoolNLRIs(first, unstorable, []byte{8, 11}, nil, false, 2, 0, false)
	r.insertPoolNLRIs(storage.NewPeerRIB("192.0.2.2"), unstorable, []byte{8, 10}, nil, false, 3, 0, false)

	var warns []string
	for line := range strings.SplitSeq(buf.String(), "\n") {
		if strings.Contains(line, "level=WARN") {
			warns = append(warns, line)
		}
	}
	if len(warns) != 2 {
		t.Fatalf("want one Warn per peer and family (2), got %d: %q", len(warns), buf.String())
	}
	for i, peer := range []string{"192.0.2.1", "192.0.2.2"} {
		for _, want := range []string{"not stored", "peer=" + peer, "family"} {
			if !strings.Contains(warns[i], want) {
				t.Fatalf("warn %d %q lacks %q", i, warns[i], want)
			}
		}
	}
}
