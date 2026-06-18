package reactor

import (
	"bytes"
	"testing"
)

// TestPackNLRIs verifies that packNLRIs groups NLRIs into size-bounded batches:
// everything in one batch when it fits, split when adding the next would exceed
// maxSize, and a lone oversized NLRI in its own batch.
//
// VALIDATES: the generic plugin-route send path enforces the negotiated max
// message size -- the guard that sendFlowSpecRoutesVia (BuildFlowSpecWithMaxSize)
// and sendMVPNRoutesVia (BuildGroupedMVPN) used to provide before the migration.
// PREVENTS: oversized FlowSpec rules / MVPN groups being emitted unguarded
// (regression found in /ze-review of spec-route-config-plugin-migration).
func TestPackNLRIs(t *testing.T) {
	nlris := [][]byte{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	// measure: a fixed 10-byte overhead plus the NLRI bytes.
	measure := func(b []byte) int { return 10 + len(b) }

	// All fit (10 + 9 = 19 <= 100): one concatenated batch, in order.
	got := packNLRIs(nlris, 100, measure)
	if len(got) != 1 {
		t.Fatalf("fit-all: got %d batches, want 1", len(got))
	}
	if !bytes.Equal(got[0], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Errorf("fit-all: batch = %v, want concatenation in order", got[0])
	}

	// maxSize 16: a pair (10+6=16) fits but a third (10+9=19) does not -> split.
	got = packNLRIs(nlris, 16, measure)
	if len(got) != 2 {
		t.Fatalf("split: got %d batches, want 2", len(got))
	}
	if !bytes.Equal(got[0], []byte{1, 2, 3, 4, 5, 6}) || !bytes.Equal(got[1], []byte{7, 8, 9}) {
		t.Errorf("split: batches = %v", got)
	}

	// Each NLRI alone exceeds maxSize: still one batch per NLRI (best effort).
	got = packNLRIs(nlris, 5, measure)
	if len(got) != 3 {
		t.Fatalf("oversize: got %d batches, want 3 (one per NLRI)", len(got))
	}

	// Empty input -> no batches.
	if got := packNLRIs(nil, 100, measure); len(got) != 0 {
		t.Errorf("empty: got %d batches, want 0", len(got))
	}
}
