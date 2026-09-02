// Overview: nlrisplit.go — the Splitter walk every family registers
// Related: cidr.go, typelen.go, labeled.go, flowspec.go, vpls.go, bgpls.go — the walks measured here

package nlrisplit

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// splitAll collects every NLRI a splitter visits. Tests that name a splitter
// function rather than its family use it to assert the slice form, which is
// what Split gives a test that goes through the registry.
func splitAll(t *testing.T, split Splitter, data []byte, addPath bool) ([][]byte, error) {
	t.Helper()
	var out [][]byte
	_, err := split(data, addPath, func(nlri []byte) {
		out = append(out, nlri)
	})
	return out, err
}

// TestEveryWalkAllocatesNothing is the reason a Splitter is a walk and not a
// builder. checkPrefixLimits walks one of these for every NLRI section of every
// inbound UPDATE (forEachPrefixEntry, internal/component/bgp/reactor/session_prefix.go),
// so a single heap allocation inside one of them is paid millions of times a
// second on a full table.
//
// VALIDATES: each registered walk allocates nothing, with a visitor and without.
// PREVENTS: a splitter reintroducing an intermediate slice, which is exactly the
// regression that put three checkPrefixLimits benchmarks over their ceiling in
// perf.AllocCeilings (internal/perf/allocgate.go).
func TestEveryWalkAllocatesNothing(t *testing.T) {
	cases := map[string]struct {
		split Splitter
		data  []byte
	}{
		"cidr":     {splitCIDR, []byte{24, 10, 0, 0, 24, 10, 0, 1}},
		"vpn":      {splitVPN, concat(vpnNLRI(100, false), vpnNLRI(200, false))},
		"evpn":     {splitEVPN, []byte{2, 3, 0x11, 0x22, 0x33, 1, 2, 0xaa, 0xbb}},
		"mup":      {SplitMUP, concat(mupNLRI(1, 0xaa), mupNLRI(2, 0xbb, 0xcc))},
		"labeled":  {SplitLabeled, []byte{48, 0x06, 0x40, 0x01, 10, 0, 0, 48, 0x0C, 0x80, 0x01, 192, 168, 1}},
		"flowspec": {SplitFlowSpec, []byte{3, 0xaa, 0xbb, 0xcc, 2, 0xdd, 0xee}},
		"vpls":     {SplitVPLS, []byte{0x00, 0x03, 0xaa, 0xbb, 0xcc, 0x00, 0x02, 0xdd, 0xee}},
		"bgpls":    {SplitBGPLS, []byte{0x00, 0x01, 0x00, 0x03, 0xaa, 0xbb, 0xcc, 0x00, 0x02, 0x00, 0x02, 0xdd, 0xee}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// The visitor is built once, outside the measured call, because a
			// caller builds it once too: the receive path holds one per section.
			visited := 0
			var walkErr error
			visit := func(nlri []byte) { visited += len(nlri) }

			withVisitor := testing.AllocsPerRun(100, func() {
				_, walkErr = tc.split(tc.data, false, visit)
			})
			if walkErr != nil {
				t.Fatalf("walk with a visitor returned %v", walkErr)
			}
			if withVisitor != 0 {
				t.Errorf("walk with a visitor allocated %v times per run, want 0", withVisitor)
			}
			if visited == 0 {
				t.Fatal("the visitor was never called, so the measurement is vacuous")
			}

			counted := 0
			countOnly := testing.AllocsPerRun(100, func() {
				counted, walkErr = tc.split(tc.data, false, nil)
			})
			if walkErr != nil {
				t.Fatalf("count pass returned %v", walkErr)
			}
			if countOnly != 0 {
				t.Errorf("count pass allocated %v times per run, want 0", countOnly)
			}
			if counted != 2 {
				t.Errorf("count pass counted %d NLRIs, want 2", counted)
			}
		})
	}
}

// TestSplitSizesItsResultFromTheCount pins the two-pass shape of Split: the
// count pass fixes the capacity, so the fill pass never grows the slice.
//
// VALIDATES: Split returns exactly the NLRIs the walk visits, in wire order,
// in a slice whose capacity is the count.
// PREVENTS: a fill pass that disagrees with its own count pass, which would
// leave the result short or over-allocated.
func TestSplitSizesItsResultFromTheCount(t *testing.T) {
	data := []byte{24, 10, 0, 0, 24, 10, 0, 1, 24, 10, 0, 2}

	got, err := Split(family.IPv4Unicast, data, false)
	if err != nil {
		t.Fatalf("Split returned %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Split gave %d NLRIs, want 3", len(got))
	}
	if cap(got) != 3 {
		t.Errorf("Split sized the result at capacity %d, want 3", cap(got))
	}
	for i, want := range [][]byte{{24, 10, 0, 0}, {24, 10, 0, 1}, {24, 10, 0, 2}} {
		if !bytes.Equal(got[i], want) {
			t.Errorf("NLRI %d is % x, want % x", i, got[i], want)
		}
	}
}
