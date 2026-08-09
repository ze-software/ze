// Design: docs/architecture/isis/isis-9-spf-rib.md step 5 -- the ORIGINATION side of L1<->L2
// inter-level leaking (RFC 2966). The receiving-side preference is in
// route_test.go (TestISISLeakUpDownBit); this file validates which prefixes an
// L1L2 router RE-ORIGINATES into each level and with which up/down state.
//
// VALIDATES: LeakPrefixes on a mixed L1L2 topology -- an L1-only prefix is leaked
// UP into L2 (up/down bit clear), an L2-derived prefix is leaked DOWN into L1 with
// the up/down bit SET, a prefix already carrying the up/down bit is NOT re-leaked
// (loop prevention), the root's own prefix is never leaked, the leaked metric is
// the source-level path cost, and a single-level node leaks nothing.
// PREVENTS: a routing loop from re-leaking a down-bit prefix back up (R-1), and a
// re-origination thrash from a non-fixpoint leak (the leak is idempotent).

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// leakNode builds a one-node Result + Graph for `level`: the root plus a reachable
// neighbor `to` (one first-hop, the given metric) that advertises `prefix` at
// prefixMetric (up/down bit clear -- a level-native prefix). The root itself
// advertises rootPrefix (to prove the root's own prefixes are never leaked). A
// zero-value rootPrefix adds no root prefix.
func leakNode(level Level, root types.SystemID, to types.SourceID, metric uint64, prefix netip.Prefix, prefixMetric uint32, rootPrefix netip.Prefix) (*Result, *Graph) {
	g := NewGraph()
	r := g.node(types.NewSourceID(root, 0))
	if rootPrefix.IsValid() {
		r.Prefixes = append(r.Prefixes, Prefix{Prefix: rootPrefix, Metric: 0})
	}
	n := g.node(to)
	n.Prefixes = append(n.Prefixes, Prefix{Prefix: prefix, Metric: prefixMetric, UpDown: false})

	res := &Result{
		Root:  root,
		Level: level,
		Nodes: map[types.SourceID]*NodeResult{
			types.NewSourceID(root, 0): {ID: types.NewSourceID(root, 0), Metric: 0},
			to:                         {ID: to, Metric: metric, FirstHops: []types.SystemID{to.SystemID()}},
		},
	}
	return res, g
}

// TestISISLeakOriginationL1L2 is the mixed-L1L2 origination-leak regression: it
// proves an L1L2 router re-originates the OTHER level's reachable prefixes with
// the correct RFC 2966 up/down state and never re-leaks a down-bit prefix.
//
// RFC requirement: RFC2966-2-1 positive -- an L2-derived prefix advertised DOWN
// into L1 is stamped with the up/down bit set (leakInto setDownBit=true for the
// L2->L1 direction, internal/plugins/isis/spf/leak.go): assertion 2 requires
// hasLeak(IntoL1, l2Derived, true).
// RFC requirement: RFC2966-2-1 negative -- a prefix leaked in any OTHER direction
// has the up/down bit CLEAR: the L1-native prefix leaked UP into L2 carries
// up/down=false (assertion 1, hasLeak(IntoL2, l1Only, false)).
// RFC requirement: RFC2966-2-2 positive -- a prefix that already carries the
// up/down (down) bit is NEVER re-advertised back up into L2 (leak.go leakInto
// skips p.UpDown): assertion 3 requires alreadyDown be absent from IntoL2.
// RFC requirement: RFC2966-2-2 negative -- the down-bit suppression is scoped, not
// a blanket drop: a clear-bit L1-native prefix IS still leaked up into L2
// (assertion 1), so ordinary L1 prefixes are not withheld.
func TestISISLeakOriginationL1L2(t *testing.T) {
	root := sysID(1)
	rootPfx := netip.MustParsePrefix("10.0.1.0/24") // root's own connected prefix

	// L1 side: neighbor 2 advertises an L1-only prefix (up), metric 10 + prefix 5.
	l1Only := netip.MustParsePrefix("10.1.0.0/24")
	// L2 side: neighbor 3 advertises an L2-derived prefix (up in L2), metric 20 +
	// prefix 7 -> leaked DOWN into L1 with the up/down bit set, total 27.
	l2Derived := netip.MustParsePrefix("10.2.0.0/24")
	// A prefix already carrying the up/down (down) bit in L1: it came from a lower
	// leak and MUST NOT be re-leaked UP into L2 (loop prevention). Advertised by
	// neighbor 2 in L1.
	alreadyDown := netip.MustParsePrefix("10.9.0.0/24")

	resL1, gL1 := leakNode(Level1, root, srcID(2), 10, l1Only, 5, rootPfx)
	// Add the already-down prefix to L1's neighbor-2 node.
	gL1.Nodes[srcID(2)].Prefixes = append(gL1.Nodes[srcID(2)].Prefixes,
		Prefix{Prefix: alreadyDown, Metric: 1, UpDown: true})

	resL2, gL2 := leakNode(Level2, root, srcID(3), 20, l2Derived, 7, netip.Prefix{})

	leak := LeakPrefixes(
		[]*Result{resL1, resL2},
		map[Level]*Graph{Level1: gL1, Level2: gL2},
	)

	// --- Assertion 1: the L1-only prefix is leaked UP into L2 (up/down clear). ---
	if !hasLeak(leak.IntoL2, l1Only, false) {
		t.Errorf("IntoL2 missing %s up=false; got %+v", l1Only, leak.IntoL2)
	}
	// The L1-only prefix must NOT appear in IntoL1 (a prefix is leaked into the
	// OTHER level, never its own).
	if hasPrefix(leak.IntoL1, l1Only) {
		t.Errorf("IntoL1 must not contain the L1-native prefix %s; got %+v", l1Only, leak.IntoL1)
	}

	// --- Assertion 2: the L2-derived prefix is leaked DOWN into L1 with the bit. ---
	if !hasLeak(leak.IntoL1, l2Derived, true) {
		t.Errorf("IntoL1 missing %s up=true (down leak); got %+v", l2Derived, leak.IntoL1)
	}
	// Its leaked metric is the L2 source-level path cost: 20 (distance) + 7 = 27.
	if m, ok := leakMetric(leak.IntoL1, l2Derived); !ok || m != 27 {
		t.Errorf("leaked %s metric = %d (ok=%v), want 27 (20 dist + 7 prefix)", l2Derived, m, ok)
	}

	// --- Assertion 3: a prefix already carrying the down bit is NOT re-leaked. ---
	if hasPrefix(leak.IntoL2, alreadyDown) {
		t.Errorf("loop prevention: down-bit prefix %s must NOT be leaked UP into L2; got %+v", alreadyDown, leak.IntoL2)
	}
	if hasPrefix(leak.IntoL1, alreadyDown) {
		t.Errorf("down-bit prefix %s must NOT be re-leaked into L1 either; got %+v", alreadyDown, leak.IntoL1)
	}

	// --- The root's own prefix is never leaked into either level. ---
	if hasPrefix(leak.IntoL2, rootPfx) || hasPrefix(leak.IntoL1, rootPfx) {
		t.Errorf("root's own prefix %s must never be leaked; IntoL1=%+v IntoL2=%+v", rootPfx, leak.IntoL1, leak.IntoL2)
	}
}

// TestISISLeakFixpoint proves the leak is a one-pass fixpoint: feeding the leaked
// DOWN prefix back in as an L1 advertisement (with the up/down bit, exactly as the
// engine would re-originate it) yields NO further leak of that prefix. This is the
// loop-termination guarantee for the engine's SPF->re-originate->SPF feedback.
//
// RFC requirement: RFC2966-2-3 positive -- an L1L2 router never advertises an
// L2->L1 inter-area route (a re-originated prefix carrying the up/down/down bit in
// L1) back into L2: round 2 requires the re-originated down-bit l2Derived be absent
// from IntoL2 (leak.go leakInto skips p.UpDown).
// RFC requirement: RFC2966-2-3 negative -- the block is specific to down-bit
// re-advertisement, not a suppression of legitimate leaking: round 1 (the same
// L2-derived prefix WITHOUT the down bit) DOES leak down into L1
// (hasLeak(r1.IntoL1, l2Derived, true)).
func TestISISLeakFixpoint(t *testing.T) {
	root := sysID(1)
	l2Derived := netip.MustParsePrefix("10.2.0.0/24")

	resL2, gL2 := leakNode(Level2, root, srcID(3), 20, l2Derived, 7, netip.Prefix{})
	// Round 1: L1 carries nothing of its own; the L2 prefix leaks DOWN.
	resL1, gL1 := leakNode(Level1, root, srcID(2), 10, netip.MustParsePrefix("10.1.0.0/24"), 5, netip.Prefix{})
	r1 := LeakPrefixes([]*Result{resL1, resL2}, map[Level]*Graph{Level1: gL1, Level2: gL2})
	if !hasLeak(r1.IntoL1, l2Derived, true) {
		t.Fatalf("round 1: expected %s leaked down into L1; got %+v", l2Derived, r1.IntoL1)
	}

	// Round 2: simulate the re-origination -- the leaked DOWN prefix is now an L1
	// advertisement carrying the up/down bit. It must NOT be leaked back UP into L2.
	gL1.Nodes[srcID(2)].Prefixes = append(gL1.Nodes[srcID(2)].Prefixes,
		Prefix{Prefix: l2Derived, Metric: 27, UpDown: true})
	r2 := LeakPrefixes([]*Result{resL1, resL2}, map[Level]*Graph{Level1: gL1, Level2: gL2})
	if hasPrefix(r2.IntoL2, l2Derived) {
		t.Errorf("fixpoint violated: the re-originated down-bit %s leaked back UP into L2; got %+v", l2Derived, r2.IntoL2)
	}
}

// TestISISLeakSingleLevelNoLeak proves a single-level node (only L1 OR only L2 in
// the result set) leaks nothing -- leaking is an L1L2-router behavior.
func TestISISLeakSingleLevelNoLeak(t *testing.T) {
	root := sysID(1)
	pfx := netip.MustParsePrefix("10.1.0.0/24")
	resL1, gL1 := leakNode(Level1, root, srcID(2), 10, pfx, 5, netip.Prefix{})

	leak := LeakPrefixes([]*Result{resL1}, map[Level]*Graph{Level1: gL1})
	if !leak.Empty() {
		t.Errorf("single-level node must leak nothing; got %+v", leak)
	}
}

// TestISISLeakIPv6 proves the IPv6 (TLV 236) leak mirrors IPv4 (RFC 5308 sec 5):
// an L1-only IPv6 prefix leaks UP into L2, an L2-derived one leaks DOWN into L1
// with the up/down bit set.
func TestISISLeakIPv6(t *testing.T) {
	root := sysID(1)
	l1v6 := netip.MustParsePrefix("2001:db8:1::/64")
	l2v6 := netip.MustParsePrefix("2001:db8:2::/64")

	gL1 := NewGraph()
	gL1.node(types.NewSourceID(root, 0))
	gL1.node(srcID(2)).PrefixesV6 = []Prefix{{Prefix: l1v6, Metric: 5, UpDown: false}}
	resL1 := &Result{Root: root, Level: Level1, Nodes: map[types.SourceID]*NodeResult{
		types.NewSourceID(root, 0): {ID: types.NewSourceID(root, 0)},
		srcID(2):                   {ID: srcID(2), Metric: 10, FirstHops: []types.SystemID{sysID(2)}},
	}}

	gL2 := NewGraph()
	gL2.node(types.NewSourceID(root, 0))
	gL2.node(srcID(3)).PrefixesV6 = []Prefix{{Prefix: l2v6, Metric: 7, UpDown: false}}
	resL2 := &Result{Root: root, Level: Level2, Nodes: map[types.SourceID]*NodeResult{
		types.NewSourceID(root, 0): {ID: types.NewSourceID(root, 0)},
		srcID(3):                   {ID: srcID(3), Metric: 20, FirstHops: []types.SystemID{sysID(3)}},
	}}

	leak := LeakPrefixes([]*Result{resL1, resL2}, map[Level]*Graph{Level1: gL1, Level2: gL2})
	if !hasLeak(leak.IntoL2V6, l1v6, false) {
		t.Errorf("IPv6: IntoL2V6 missing %s up=false; got %+v", l1v6, leak.IntoL2V6)
	}
	if !hasLeak(leak.IntoL1V6, l2v6, true) {
		t.Errorf("IPv6: IntoL1V6 missing %s up=true (down leak); got %+v", l2v6, leak.IntoL1V6)
	}
}

// hasLeak reports whether leaked contains prefix with the given up/down bit.
func hasLeak(leaked []LeakedPrefix, prefix netip.Prefix, upDown bool) bool {
	for _, lp := range leaked {
		if lp.Prefix == prefix && lp.UpDown == upDown {
			return true
		}
	}
	return false
}

// hasPrefix reports whether leaked contains prefix (any up/down state).
func hasPrefix(leaked []LeakedPrefix, prefix netip.Prefix) bool {
	for _, lp := range leaked {
		if lp.Prefix == prefix {
			return true
		}
	}
	return false
}

// leakMetric returns the leaked metric for prefix and whether it was found.
func leakMetric(leaked []LeakedPrefix, prefix netip.Prefix) (uint32, bool) {
	for _, lp := range leaked {
		if lp.Prefix == prefix {
			return lp.Metric, true
		}
	}
	return 0, false
}
