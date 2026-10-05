// Design: docs/architecture/route-selection.md -- whole-set MED elimination.
package rib

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// wholeSetMEDCandidates recreates the saved six-permutation red: A removes C
// at MED, then B removes A at Router ID. C must never remove B first.
func wholeSetMEDCandidates() [3]Candidate {
	return [3]Candidate{
		{PeerAddr: "192.0.2.3", PeerIP: netip.MustParseAddr("192.0.2.3"),
			PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 1,
			FirstAS: 65001, Origin: OriginIGP, MED: 0, IGPCost: 10,
			OriginatorIP: netip.MustParseAddr("198.51.100.3")},
		{PeerAddr: "192.0.2.2", PeerIP: netip.MustParseAddr("192.0.2.2"),
			PeerASN: 65002, LocalASN: 65000, LocalPref: 100, ASPathLen: 1,
			FirstAS: 65002, Origin: OriginIGP, MED: 0, IGPCost: 10,
			OriginatorIP: netip.MustParseAddr("198.51.100.2")},
		{PeerAddr: "192.0.2.1", PeerIP: netip.MustParseAddr("192.0.2.1"),
			PeerASN: 65001, LocalASN: 65000, LocalPref: 100, ASPathLen: 1,
			FirstAS: 65001, Origin: OriginIGP, MED: 100, IGPCost: 10,
			OriginatorIP: netip.MustParseAddr("198.51.100.1")},
	}
}

func wholeSetMEDOrders() [6][3]int {
	return [6][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
}

// TestRFC4271WholeSetMEDBeforeLaterCriteria preserves the historical red and
// checks the explanation's actual removal witnesses in the original indices.
// RFC 4271 Section 9.1.2.2: "The criteria MUST be applied in the order specified."
// RFC requirement: RFC4271-9.1.2.2-1 positive -- every permutation of three routes removes the higher MED path within its neighbor AS before comparing the surviving paths' BGP Identifiers; both selection APIs choose B and the explanation names A removing C at MED then B removing A at Router ID.
// MUTATION: Restore the insertion-order tournament in selectBestCandidates.
func TestRFC4271WholeSetMEDBeforeLaterCriteria(t *testing.T) {
	for _, order := range wholeSetMEDOrders() {
		c := wholeSetMEDCandidates()
		input := []*Candidate{&c[order[0]], &c[order[1]], &c[order[2]]}
		original := [3]*Candidate{input[0], input[1], input[2]}
		// RFC 4271 Section 9.1.2.2(c), then (f).
		exp := SelectBestExplain(input)
		if exp == nil || exp.Winner != &c[1] {
			t.Fatalf("order %v: explanation = %+v, want B", order, exp)
		}
		if len(exp.Steps) != 2 {
			t.Fatalf("order %v: steps = %+v, want two removals", order, exp.Steps)
		}
		for i, candidate := range original {
			if exp.Candidates[i] != candidate || input[i] != candidate {
				t.Fatalf("order %v: explanation changed original index %d", order, i)
			}
		}
		for i, want := range []struct {
			winner *Candidate
			loser  *Candidate
			step   BestStep
		}{{&c[0], &c[2], BestStepMED}, {&c[1], &c[0], BestStepRouterID}} {
			step := exp.Steps[i]
			loser := step.ChallengerIdx
			if step.WinnerIdx == loser {
				loser = step.IncumbentIdx
			}
			if exp.Candidates[step.WinnerIdx] != want.winner || exp.Candidates[loser] != want.loser || step.Step != want.step {
				t.Fatalf("order %v step %d: %+v, want %s removes %s at %s", order, i, step, want.winner.PeerAddr, want.loser.PeerAddr, want.step)
			}
			if !strings.Contains(step.Reason, want.step.String()) {
				t.Fatalf("order %v: missing reason values in %+v", order, step)
			}
			if i == 0 && (!strings.Contains(step.Reason, "100") || !strings.Contains(step.Reason, "0") || !strings.Contains(step.Reason, "65001")) {
				t.Fatalf("order %v: MED reason omits compared values/neighbor: %q", order, step.Reason)
			}
		}
		// RFC 4271 Section 9.1.2.2: the hot path must elect the same route.
		if got := SelectBest(input); got != &c[1] {
			t.Fatalf("order %v: SelectBest = %+v, want B", order, got)
		}
		for _, candidate := range original {
			found := 0
			for _, retained := range input {
				if retained == candidate {
					found++
				}
			}
			if found != 1 {
				t.Fatalf("order %v: candidate %s retained %d times", order, candidate.PeerAddr, found)
			}
		}
	}
}

// TestRFC4271WholeSetMEDEarlierCriteria removes the route that would otherwise
// suppress C at MED. C then wins, proving that an earlier loser cannot remove it.
// RFC requirement: RFC4271-9.1.2.2-1 negative -- stale depreference, LOCAL_PREF, AIGP, AS_PATH length and ORIGIN remove A before MED in all six orders, so its lower MED cannot suppress C and both selection APIs choose C with A's truthful earlier loss reason.
// MUTATION: Group and eliminate on MED before filtering earlier-criteria losers.
func TestRFC4271WholeSetMEDEarlierCriteria(t *testing.T) {
	for _, tc := range []struct {
		name string
		step BestStep
		bias func(c *[3]Candidate)
	}{
		{"stale", BestStepStale, func(c *[3]Candidate) { c[0].StaleLevel = 2 }},
		{"local-pref", BestStepLocalPref, func(c *[3]Candidate) { c[0].LocalPref = 90 }},
		{"aigp", BestStepAIGP, func(c *[3]Candidate) { c[1].HasAIGP, c[2].HasAIGP = true, true }},
		{"as-path", BestStepASPathLen, func(c *[3]Candidate) { c[0].ASPathLen = 2 }},
		{"origin", BestStepOrigin, func(c *[3]Candidate) { c[0].Origin = OriginIncomplete }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, order := range wholeSetMEDOrders() {
				c := wholeSetMEDCandidates()
				tc.bias(&c)
				input := []*Candidate{&c[order[0]], &c[order[1]], &c[order[2]]}
				// RFC 4271 Section 9.1.2.2, with RFC 7311 Section 4.1 AIGP ordering.
				exp := SelectBestExplain(input)
				if exp == nil || exp.Winner != &c[2] || len(exp.Steps) != 2 {
					t.Fatalf("order %v: explanation = %+v, want C and two removals", order, exp)
				}
				step := exp.Steps[0]
				loser := step.ChallengerIdx
				if loser == step.WinnerIdx {
					loser = step.IncumbentIdx
				}
				if exp.Candidates[loser] != &c[0] || step.Step != tc.step {
					t.Fatalf("order %v: A removal = %+v, want %s", order, step, tc.step)
				}
				if got := SelectBest(input); got != &c[2] {
					t.Fatalf("order %v: best = %+v, want C", order, got)
				}
			}
		})
	}
}

// TestWholeSetMEDComparabilityAndMultipath keeps MED local to a known neighbor
// and refuses a MED-eliminated route as an equal-cost sibling of another AS.
func TestWholeSetMEDComparabilityAndMultipath(t *testing.T) {
	for _, order := range wholeSetMEDOrders() {
		c := wholeSetMEDCandidates()
		c[1].MED = 50 // Lower MED from a different AS must not suppress B.
		input := []*Candidate{&c[order[0]], &c[order[1]], &c[order[2]]}
		primary, siblings := SelectMultipath(input, 3, true)
		if primary != &c[1] || len(siblings) != 1 || siblings[0] != &c[0] {
			t.Fatalf("order %v: multipath = %+v / %+v, want B / [A]", order, primary, siblings)
		}
		// Unknown neighboring AS does not put unrelated paths in a MED group.
		c[0].FirstAS, c[2].FirstAS = 0, 0
		if got := SelectBest(input); got != &c[2] {
			t.Fatalf("order %v: unknown-neighbor winner = %+v, want C", order, got)
		}
		// Empty/AS_SET-led iBGP paths use the local AS, not an unknown group.
		c[0].PeerASN, c[2].PeerASN = 65000, 65000
		c[1].PeerASN = 65000
		if got := SelectBest(input); got != &c[1] {
			t.Fatalf("order %v: local-AS MED winner = %+v, want B", order, got)
		}
	}
}

// TestWholeSetMEDNoSelectionAllocation covers one, two and many candidates,
// including many MED groups, without a per-election map or scratch allocation.
func TestWholeSetMEDNoSelectionAllocation(t *testing.T) {
	var candidates [192]Candidate
	var input [192]*Candidate
	for i := range candidates {
		candidates[i] = Candidate{LocalPref: 100, FirstAS: uint32(i%17 + 1), MED: uint32(i), PathID: uint32(i)}
		input[i] = &candidates[i]
	}
	for _, count := range []int{1, 2, 3, 192} {
		if allocations := testing.AllocsPerRun(100, func() { SelectBest(input[:count]) }); allocations != 0 {
			t.Fatalf("%d candidates: allocations = %v, want zero", count, allocations)
		}
	}
}

// TestRFC4271WholeSetMEDRIBBestChange drives stored UPDATE attributes through
// candidate extraction, selection, best-change publication and the Loc-RIB.
// Removing A revives C, even though A itself was not the installed best.
// RFC requirement: RFC4271-9.1.2.2-1 positive -- the real RIB best-change workflow installs B after whole-set MED elimination, replaces it with C when non-best A is withdrawn, restores B when A returns and keeps one exact BGP next hop in the Loc-RIB throughout.
// MUTATION: Restore pairwise selection or ignore removal of a non-best MED witness.
func TestRFC4271WholeSetMEDRIBBestChange(t *testing.T) {
	for _, order := range wholeSetMEDOrders() {
		bus := newTestEventBus()
		r := newTestRIBManagerWithBus(bus)
		r.locRIB.Store(locrib.NewRIB())
		c := wholeSetMEDCandidates()
		prefix := []byte{24, 203, 0, 113}
		for _, i := range order {
			// RFC 4271 Section 9.1.2.2: actual received attributes feed extraction.
			wholeSetMEDReceive(t, r, &c[i], false)
		}
		assertWholeSetMEDLocRIB(t, r, c[1].PeerIP)
		// An unchanged election must not publish a spurious best-change.
		if _, changed := r.checkBestPathChange(famV4, prefix, false, nil); changed {
			t.Fatalf("order %v: unchanged whole-set winner republished", order)
		}
		before := bus.eventCount()
		wholeSetMEDReceive(t, r, &c[0], true)
		assertWholeSetMEDLocRIB(t, r, c[2].PeerIP)
		assertWholeSetMEDChange(t, bus, before, c[2].PeerIP)
		before = bus.eventCount()
		wholeSetMEDReceive(t, r, &c[0], false)
		assertWholeSetMEDLocRIB(t, r, c[1].PeerIP)
		assertWholeSetMEDChange(t, bus, before, c[1].PeerIP)
	}
}

func assertWholeSetMEDLocRIB(t *testing.T, r *RIBManager, nextHop netip.Addr) {
	t.Helper()
	group, found := r.locRIB.Load().Lookup(famV4, netip.MustParsePrefix("203.0.113.0/24"))
	if !found || len(group.Paths) != 1 || group.Paths[0].NextHop != nextHop {
		t.Fatalf("Loc-RIB = %+v, found %t, want one path through %s", group, found, nextHop)
	}
}

func assertWholeSetMEDChange(t *testing.T, bus *testEventBus, before int, nextHop netip.Addr) {
	t.Helper()
	batch := sweepBestChange(t, bus, before)
	if len(batch.Changes) != 1 {
		t.Fatalf("best-change payload = %+v, want one change", batch)
	}
	change := batch.Changes[0]
	if change.Action != ribevents.BestChangeUpdate || change.NextHop != nextHop || change.Prefix != netip.MustParsePrefix("203.0.113.0/24") {
		t.Fatalf("best-change = %+v, want replacement through %s", change, nextHop)
	}
}

// wholeSetMEDReceive sends literal RFC 4271 Section 4.3 UPDATE fields through
// the production receive entry point. The three paths have distinct next hops.
func wholeSetMEDReceive(t *testing.T, r *RIBManager, c *Candidate, withdraw bool) {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	if err != nil {
		t.Fatal(err)
	}
	nextHop := c.PeerIP.As4()
	body := []byte{
		0, 0, 0, 34, // Withdrawn length 0, path attribute length 34.
		0x40, 1, 1, 0, // ORIGIN IGP.
		0x40, 2, 6, 2, 1, byte(c.FirstAS >> 24), byte(c.FirstAS >> 16), byte(c.FirstAS >> 8), byte(c.FirstAS),
		0x40, 3, 4, nextHop[0], nextHop[1], nextHop[2], nextHop[3],
		0x80, 4, 4, byte(c.MED >> 24), byte(c.MED >> 16), byte(c.MED >> 8), byte(c.MED),
		0x40, 5, 4, 0, 0, 0, 100,
		24, 203, 0, 113,
	}
	if withdraw {
		body = []byte{0, 4, 24, 203, 0, 113, 0, 0}
	}
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	if err != nil {
		t.Fatal(err)
	}
	// RFC 4271 Section 9.1.2.2: selection follows the received UPDATE.
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType: rpc.EventKindUpdate, PeerAddress: c.PeerAddr,
		PeerAS: c.PeerASN, LocalAS: c.LocalASN,
		RemoteRouterID: identifierFor(t, c.OriginatorIP.String()),
		RawMessage: &types.RawMessage{
			Type: msgtype.TypeUPDATE, RawBytes: body, WireUpdate: wu, AttrsWire: attrs,
		},
	})
}

// TestWholeSetMEDLaterCriteria opposes MED with each later criterion across
// all six orders. The final Path Identifier case uses three paths of one peer.
func TestWholeSetMEDLaterCriteria(t *testing.T) {
	for _, tc := range []struct {
		name string
		step BestStep
		bias func(c *[3]Candidate)
	}{
		{"ebgp", BestStepEBGPOverIBGP, func(c *[3]Candidate) { c[0].PeerASN = 65000 }},
		{"igp-cost", BestStepIGPCost, func(c *[3]Candidate) {
			c[0].IGPCost, c[1].IGPCost, c[2].IGPCost = 30, 20, 10
		}},
		{"cluster-list", BestStepClusterList, func(c *[3]Candidate) {
			c[0].OriginatorIP, c[2].OriginatorIP = c[1].OriginatorIP, c[1].OriginatorIP
			c[0].ClusterListEntries, c[1].ClusterListEntries, c[2].ClusterListEntries = 3, 2, 1
		}},
		{"peer-address", BestStepPeerAddr, func(c *[3]Candidate) {
			c[0].OriginatorIP, c[2].OriginatorIP = c[1].OriginatorIP, c[1].OriginatorIP
		}},
		{"path-id", BestStepPathID, func(c *[3]Candidate) {
			for i := range c {
				c[i].OriginatorIP = netip.MustParseAddr("198.51.100.1")
				c[i].PeerIP = netip.MustParseAddr("192.0.2.1")
				c[i].PeerAddr = "192.0.2.1"
				c[i].PeerASN = 65000
				c[i].AddPath = true
				c[i].PathID = uint32(2 - i)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, order := range wholeSetMEDOrders() {
				c := wholeSetMEDCandidates()
				tc.bias(&c)
				input := []*Candidate{&c[order[0]], &c[order[1]], &c[order[2]]}
				// RFC 4271 Section 9.1.2.2: MED removes C before later criteria.
				exp := SelectBestExplain(input)
				if exp == nil || exp.Winner != &c[1] || len(exp.Steps) != 2 {
					t.Fatalf("order %v: explanation = %+v, want B", order, exp)
				}
				if exp.Steps[0].Step != BestStepMED || exp.Steps[1].Step != tc.step {
					t.Fatalf("order %v: steps = %+v, want MED then %s", order, exp.Steps, tc.step)
				}
				if got := SelectBest(input); got != &c[1] {
					t.Fatalf("order %v: winner = %+v, want B", order, got)
				}
			}
		})
	}
}
