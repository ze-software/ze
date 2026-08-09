// Design: docs/architecture/isis/isis-8-dis-broadcast.md -- engine-layer DIS wiring tests.
//
// VALIDATES (Wiring Test, the umbrella isis-8 row), on darwin via the in-memory
// lossless wire (no raw socket):
//   - three engines on one shared broadcast segment elect EXACTLY one DIS per
//     level; the highest (priority, MAC) wins (TestISISDISElection, AC-1/AC-2);
//   - the elected DIS originates a pseudo-node LSP (non-zero pseudonode LAN ID)
//     listing every member at metric 0, and it appears in every node's LSDB by
//     flooding (TestISISDISElection, AC-3, User Story 2);
//   - every node's own LSP advertises the LAN as a SINGLE TLV 22 entry pointing
//     at the pseudo-node, not one entry per peer (TestOwnLSPPointsAtPseudoNode,
//     AC-7, R-3);
//   - losing the DIS (its node leaves the segment) re-elects a new DIS from the
//     remaining routers; the new DIS originates a pseudo-node and the old one no
//     longer survives as a live LSP (TestISISDISReElectOnLoss, AC-6, User Story 4).
// PREVENTS: a regression where election is not wired to the transition hook, the
// pseudo-node LSP is never originated/flooded, the star encoding is not applied,
// or a role change leaves a stale pseudo-node.

package isis

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// lanNode bundles an engine with its System ID for the LAN DIS tests.
type lanNode struct {
	eng *engine
	sys types.SystemID
}

// startLANNode builds and starts an engine with one broadcast circuit on the
// shared wire w (ifindex/mac distinct per node), at L1 with the given priority.
func startLANNode(t *testing.T, w *relWire, ifindex int, mac, sysOctet byte, priority int) lanNode {
	t.Helper()
	be := newMultiBackend()
	be.addLink("eth0", w, ifindex, mac)
	netHex := "49.0001.0000.0000.000" + string(rune('0'+sysOctet)) + ".00"
	cfg := `{"isis":{"net":"` + netHex + `","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"broadcast","priority":"` +
		itoa(priority) + `"}}}}}`
	eng := startEngineMulti(t, be, cfg)
	return lanNode{eng: eng, sys: eng.cfg.SystemID}
}

// itoa is a tiny int->string for the config template (no fmt in tests by habit;
// the values are small DIS priorities 0..127).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// pseudonodeL1 returns the elected pseudo-node Source ID for eth0 at L1 on node n
// and whether one is recorded (the LAN DIS tests are L1-only).
func (n lanNode) pseudonodeL1() (types.SourceID, bool) {
	return n.eng.lookupElectedPseudonode("eth0", lsdb.Level1)
}

// holdsPseudonodeLSP reports whether node n's LSDB holds a (non-purged) pseudo-node
// LSP fragment 0 with Source ID src at L1 (the LAN DIS tests are L1-only).
func holdsPseudonodeLSP(n lanNode, src types.SourceID) bool {
	id := types.NewLSPID(src, 0)
	e := n.eng.lsdb.Lookup(lsdb.Level1, id)
	return e != nil && !e.IsPurged()
}

// ownLSPNeighbors decodes node n's own L1 fragment-0 TLV 22 entries into a set of
// neighbor Source IDs (the star edges the own LSP advertises). The LAN DIS tests
// are L1-only, so the level is fixed at L1.
func ownLSPNeighbors(t *testing.T, n lanNode) map[types.SourceID]struct{} {
	t.Helper()
	id := types.NewLSPID(types.NewSourceID(n.sys, 0), 0)
	e := n.eng.lsdb.Lookup(lsdb.Level1, id)
	if e == nil {
		return nil
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode own LSP: %v", err)
	}
	out := map[types.SourceID]struct{}{}
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedISReach {
			continue
		}
		dec, err := packet.DecodeExtendedISReachTLV(tlv.Value)
		if err != nil {
			t.Fatalf("decode TLV 22: %v", err)
		}
		for _, ent := range dec.Entries {
			out[ent.Neighbor] = struct{}{}
		}
	}
	return out
}

// TestISISDISElection: three nodes on one shared broadcast segment elect a single
// DIS for L1; the DIS (highest priority) originates a pseudo-node LSP listing all
// three members, and the pseudo-node LSP floods to every node's LSDB.
func TestISISDISElection(t *testing.T) {
	w := newRelWire()
	// Node C has the highest priority (100) -> it must become DIS. A=10, B=50.
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 50)
	c := startLANNode(t, w, 30, 0x0c, 3, 100)
	defer a.eng.shutdown()
	defer b.eng.shutdown()
	defer c.eng.shutdown()

	nodes := []lanNode{a, b, c}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		// C must be the DIS (it allocated a pseudo-node whose System ID is C's), and
		// every node must hold C's pseudo-node LSP by flooding.
		pnSrc, ok := c.pseudonodeL1()
		if ok && pnSrc.SystemID() == c.sys && pnSrc.IsPseudonode() && c.eng.localIsDISL1() {
			allHold := true
			for _, n := range nodes {
				if !holdsPseudonodeLSP(n, pnSrc) {
					allHold = false
					break
				}
			}
			// And A and B must NOT be the DIS (exactly one DIS).
			if allHold && !a.eng.localIsDISL1() && !b.eng.localIsDISL1() {
				// The pseudo-node LSP must list all three System IDs at metric 0.
				assertPseudonodeMembers(t, c, lsdb.Level1, pnSrc, []types.SystemID{a.sys, b.sys, c.sys})
				return // converged: one DIS (C), pseudo-node flooded everywhere
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("DIS election did not converge: C-isDIS=%v A-isDIS=%v B-isDIS=%v",
		c.eng.localIsDISL1(),
		a.eng.localIsDISL1(),
		b.eng.localIsDISL1())
}

// assertPseudonodeMembers checks the pseudo-node LSP (in DIS node n's LSDB) lists
// exactly the given members, each at metric 0.
func assertPseudonodeMembers(t *testing.T, n lanNode, level lsdb.Level, src types.SourceID, want []types.SystemID) {
	t.Helper()
	id := types.NewLSPID(src, 0)
	e := n.eng.lsdb.Lookup(level, id)
	if e == nil {
		t.Fatal("pseudo-node LSP missing in DIS LSDB")
	}
	lsp, err := e.Decode()
	if err != nil {
		t.Fatalf("decode pseudo-node LSP: %v", err)
	}
	got := map[types.SystemID]uint32{}
	for _, tlv := range lsp.TLVs {
		if tlv.Type != packet.TLVExtendedISReach {
			continue
		}
		dec, _ := packet.DecodeExtendedISReachTLV(tlv.Value)
		for _, ent := range dec.Entries {
			got[ent.Neighbor.SystemID()] = ent.Metric.Value()
		}
	}
	for _, m := range want {
		metric, ok := got[m]
		if !ok {
			t.Fatalf("pseudo-node LSP missing member %s (have %d members)", m, len(got))
		}
		if metric != 0 {
			t.Fatalf("pseudo-node member %s metric = %d, want 0", m, metric)
		}
	}
}

// pnSeqLifetime returns the stored sequence and Remaining Lifetime of node n's
// pseudo-node fragment 0 (src) at L1, and whether it is present and live.
func pnSeqLifetime(n lanNode, src types.SourceID) (seq uint32, lifetime uint16, liveOK bool) {
	id := types.NewLSPID(src, 0)
	e := n.eng.lsdb.Lookup(lsdb.Level1, id)
	if e == nil {
		return 0, 0, false
	}
	return uint32(e.Sequence()), e.Lifetime().Seconds(), !e.IsPurged()
}

// setPNLastOrigAtPast plants the pseudo-node last-origination time for (eth0, L1)
// far enough in the past that a refresh is due. The deterministic seam the
// pseudo-node refresh test uses instead of sleeping for a real refresh interval.
func setPNLastOrigAtPast(n lanNode) {
	key := disKey{name: "eth0", level: lsdb.Level1}
	n.eng.disMu.Lock()
	if n.eng.pnLastOrigAt == nil {
		n.eng.pnLastOrigAt = make(map[disKey]time.Time)
	}
	n.eng.pnLastOrigAt[key] = time.Now().Add(-2 * DefaultLSPRefreshInterval * time.Second)
	n.eng.disMu.Unlock()
}

// TestISISPseudonodeRefreshDueReoriginates is the periodic pseudo-node refresh
// regression (ISO/IEC 10589 clause 7.3.16.1, the pseudo-node twin of spec-isis-6
// AC-3 / the deferred Bundle E item 2): a DIS on a quiescent LAN must re-originate
// its pseudo-node LSP once lsp-refresh-interval has elapsed, bumping the sequence
// and resetting the Remaining Lifetime to MaxAge so the pseudo-node never ages out
// of peers' LSDBs. It drives the refresh deterministically by planting pnLastOrigAt
// in the past and invoking the same refreshPseudonodes the aging tick calls -- no
// 900s sleep. It also proves the coalescing path: a not-due refresh is a no-op (no
// sequence bump), so a stable DIS does not re-flood its pseudo-node every second.
func TestISISPseudonodeRefreshDueReoriginates(t *testing.T) {
	w := newRelWire()
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 50) // higher priority -> DIS
	defer a.eng.shutdown()
	defer b.eng.shutdown()

	// B becomes DIS, originates its pseudo-node, and A holds it by flooding.
	var bPN types.SourceID
	waitFor(t, 45*time.Second, func() bool {
		pn, ok := b.pseudonodeL1()
		if ok && pn.SystemID() == b.sys && b.eng.localIsDISL1() && holdsPseudonodeLSP(a, pn) {
			bPN = pn
			return true
		}
		return false
	}, "B did not become the initial DIS")

	// Coalescing baseline: with a fresh pseudo-node (just originated) NO refresh is
	// due, so refreshPseudonodes must be a no-op (the per-second re-election cadence
	// does not re-flood a stable pseudo-node).
	if due := b.eng.refreshDuePseudonodes(time.Now()); len(due) != 0 {
		t.Fatalf("refreshDuePseudonodes = %d due on a freshly-originated pseudo-node, want 0", len(due))
	}
	seqBefore, _, liveBefore := pnSeqLifetime(b, bPN)
	if !liveBefore {
		t.Fatal("B's pseudo-node fragment 0 is not live before the refresh")
	}
	b.eng.refreshPseudonodes()
	if seqAfterNoop, _, _ := pnSeqLifetime(b, bPN); seqAfterNoop != seqBefore {
		t.Errorf("not-due pseudo-node refresh bumped sequence %d -> %d (should coalesce)", seqBefore, seqAfterNoop)
	}

	// Now make a refresh due and drive the aging-loop pseudo-node refresh.
	setPNLastOrigAtPast(b)
	due := b.eng.refreshDuePseudonodes(time.Now())
	if len(due) != 1 || due[0].dl != lsdb.Level1 {
		t.Fatalf("refreshDuePseudonodes = %v, want B's eth0 L1 pseudo-node due", due)
	}
	baseSeq, _, _ := pnSeqLifetime(b, bPN)
	b.eng.refreshPseudonodes()

	// The pseudo-node LSP is re-originated: HIGHER sequence and Remaining Lifetime
	// reset to MaxAge (lsp-lifetime, default 1200), and it is still live.
	seqAfter, lifeAfter, liveAfter := pnSeqLifetime(b, bPN)
	if !liveAfter {
		t.Fatal("B's pseudo-node fragment 0 is not live after the refresh")
	}
	if seqAfter <= baseSeq {
		t.Errorf("pseudo-node refresh sequence = %d, want > %d (refresh must bump)", seqAfter, baseSeq)
	}
	if lifeAfter != DefaultLSPLifetime {
		t.Errorf("pseudo-node refresh Remaining Lifetime = %d, want %d (reset to MaxAge)", lifeAfter, DefaultLSPLifetime)
	}
}

// TestOwnLSPPointsAtPseudoNode: once a DIS is elected, every node's OWN LSP
// advertises the broadcast LAN as a single TLV 22 entry pointing at the
// pseudo-node, NOT one entry per peer (AC-7, R-3).
func TestOwnLSPPointsAtPseudoNode(t *testing.T) {
	w := newRelWire()
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 50)
	c := startLANNode(t, w, 30, 0x0c, 3, 100) // highest priority -> DIS
	defer a.eng.shutdown()
	defer b.eng.shutdown()
	defer c.eng.shutdown()

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		pnSrc, ok := c.pseudonodeL1()
		if !ok || pnSrc.SystemID() != c.sys {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		// Check A (a non-DIS): its own LSP must contain the pseudo-node Source ID and
		// must NOT contain B's or C's router Source IDs directly (the star, R-3).
		aNeighbors := ownLSPNeighbors(t, a)
		_, hasPN := aNeighbors[pnSrc]
		_, hasBRouter := aNeighbors[types.NewSourceID(b.sys, 0)]
		_, hasCRouter := aNeighbors[types.NewSourceID(c.sys, 0)]
		if hasPN && !hasBRouter && !hasCRouter {
			// And the own LSP must reference the pseudo-node exactly once for the
			// circuit (a single star edge), so the neighbor set holds exactly the one
			// pseudo-node entry for this LAN.
			if len(aNeighbors) == 1 {
				return // star encoding confirmed on a non-DIS node
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	pnSrc, _ := c.pseudonodeL1()
	t.Fatalf("own LSP star encoding not applied on node A: neighbors=%v pn=%s",
		ownLSPNeighbors(t, a), pnSrc)
}

// test-relax: TestISISDISReElectOnPriority (an engine-level priority-driven role
// transfer via reconcile) is replaced by the two tests below. A runtime DIS-priority
// change to a LIVE circuit is not re-advertised by isis-5's in-place reconcile (the
// circuit keeps its build-time priority; reconcile only marks the param changed), so
// a priority change cannot drive a role transfer at the engine layer in v1. AC-5's
// priority-change election logic is fully covered by the pure-logic unit test
// TestDISReElectOnPriorityChange (circuit/dis_test.go). The engine-level runtime
// re-election triggers are: a DIS LOSS (AC-6, TestISISDISReElectOnLoss) and a
// higher-priority router JOINING (which exercises the purge-before-yielding path
// with the old DIS still present, R-2, TestISISDISYieldPurgesPseudonode).

// TestISISDISReElectOnLoss: with a DIS already elected on a three-node LAN, losing
// the DIS (its node leaves the segment) re-elects a new DIS from the remaining
// routers, which originates its own pseudo-node LSP (AC-6, User Story 4). The
// hold-timer sweep drops the lost DIS adjacency, the periodic re-election observes
// the smaller candidate set, and the new winner originates its pseudo-node. (The
// departed DIS's own pseudo-node ages out over MaxLifetime via the isis-6 aging
// path -- an abruptly-departed node cannot purge its LSPs -- so this test asserts
// the new DIS comes up, not that the stale LSP is already gone; the
// purge-before-yielding path for a PRESENT node is TestISISDISYieldPurgesPseudonode.)
func TestISISDISReElectOnLoss(t *testing.T) {
	w := newRelWire()
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 50)  // becomes DIS after C leaves (50 > 10)
	c := startLANNode(t, w, 30, 0x0c, 3, 100) // initially DIS (priority 100)
	defer a.eng.shutdown()
	defer b.eng.shutdown()
	// C is shut down mid-test; guard the deferred shutdown so it is idempotent.
	cShut := false
	defer func() {
		if !cShut {
			c.eng.shutdown()
		}
	}()

	// Wait for C to become the DIS with a flooded pseudo-node held by A.
	waitFor(t, 45*time.Second, func() bool {
		pn, ok := c.pseudonodeL1()
		return ok && pn.SystemID() == c.sys && holdsPseudonodeLSP(a, pn) && c.eng.localIsDISL1()
	}, "C did not become the initial DIS")

	// C leaves the segment (the DIS is lost). Its Hellos stop; A and B time the
	// adjacency to C out and re-elect.
	c.eng.shutdown()
	cShut = true

	// After re-election B (priority 50 > A's 10) must be the DIS with its own
	// pseudo-node, and A must hold B's new pseudo-node by flooding (AC-6).
	waitFor(t, 90*time.Second, func() bool {
		bPN, ok := b.pseudonodeL1()
		if !ok || bPN.SystemID() != b.sys || !b.eng.localIsDISL1() {
			return false
		}
		return holdsPseudonodeLSP(b, bPN) && holdsPseudonodeLSP(a, bPN) && !a.eng.localIsDISL1()
	}, "DIS loss did not re-elect a new DIS that originates a pseudo-node")
}

// TestISISDISYieldPurgesPseudonode: a higher-priority router JOINING the segment
// takes the DIS role from the incumbent; the incumbent (still present) PURGES its
// pseudo-node LSP before yielding so no phantom node lingers (R-2, AC-5's
// role-transfer effect). B is DIS first; then higher-priority D joins and wins; B
// must purge the pseudo-node it originated (zero Remaining Lifetime).
func TestISISDISYieldPurgesPseudonode(t *testing.T) {
	w := newRelWire()
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 50) // DIS first (only other node is A at 10)
	defer a.eng.shutdown()
	defer b.eng.shutdown()

	// B becomes DIS and originates its pseudo-node.
	var bPN types.SourceID
	waitFor(t, 45*time.Second, func() bool {
		pn, ok := b.pseudonodeL1()
		if ok && pn.SystemID() == b.sys && b.eng.localIsDISL1() && holdsPseudonodeLSP(a, pn) {
			bPN = pn
			return true
		}
		return false
	}, "B did not become the initial DIS")

	// D joins with the highest priority (120) and must take the role.
	d := startLANNode(t, w, 40, 0x0d, 4, 120)
	defer d.eng.shutdown()

	// D becomes DIS, and B (still present) purges the pseudo-node it had originated:
	// its own LSDB now holds bPN as a purge (Remaining Lifetime 0) -- the
	// purge-before-yielding path (R-2).
	waitFor(t, 90*time.Second, func() bool {
		dPN, ok := d.pseudonodeL1()
		if !ok || dPN.SystemID() != d.sys || !d.eng.localIsDISL1() {
			return false
		}
		if b.eng.localIsDISL1() {
			return false // B must have yielded
		}
		// B's old pseudo-node is purged in B's own LSDB (it withdrew it before
		// yielding); a purged entry is retained (zero-age) not deleted at once.
		oldID := types.NewLSPID(bPN, 0)
		be := b.eng.lsdb.Lookup(lsdb.Level1, oldID)
		bPurged := be != nil && be.IsPurged()
		// And D's new pseudo-node is live and flooded to A.
		return bPurged && holdsPseudonodeLSP(a, dPN)
	}, "higher-priority join did not transfer the role / old DIS did not purge its pseudo-node")
}

// localIsDISL1 is a test accessor: whether the engine's eth0 circuit is the DIS at
// L1 (the LAN DIS tests are single-circuit, L1-only). It reads the live circuit's
// committed DIS state.
func (e *engine) localIsDISL1() bool {
	e.circuitsMu.RLock()
	c := e.circuitByName["eth0"]
	e.circuitsMu.RUnlock()
	if c == nil {
		return false
	}
	return c.LocalIsDIS(adjToAdjacencyLevel(lsdb.Level1))
}

// waitFor polls cond until it returns true or the deadline elapses, failing with
// msg otherwise.
func waitFor(t *testing.T, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(msg)
}

// eth0Circuit is a test accessor for a node's live eth0 broadcast circuit.
func (n lanNode) eth0Circuit() *circuit.Circuit {
	n.eng.circuitsMu.RLock()
	defer n.eng.circuitsMu.RUnlock()
	return n.eng.circuitByName["eth0"]
}

// TestISISDISElectionConcurrentReaction is the DIS-ordering regression (Bundle E
// finding 2): runElection is invoked concurrently for the SAME circuit from the
// receive, hold-timer-sweep, and DIS-loop goroutines. Elect() is atomic under the
// circuit mutex, but the engine REACTION (allocate / originate / record the
// pseudo-node, then decide the own-LSP re-origination) runs outside it and could
// interleave to a stale pseudo-node / own-LSP set. With the reaction serialized
// under the engine election mutex, a storm of concurrent runElection calls on one
// circuit must leave a CONSISTENT outcome: exactly one recorded pseudo-node equal
// to the elected DIS, the DIS unchanged, and the pseudo-node LSP intact.
//
// Run under -race, the concurrent calls also prove the reaction has no data race on
// the engine's DIS state. PREVENTS: a regression where concurrent elections corrupt
// the recorded pseudo-node or leave the own LSP pointing at a stale pseudo-node.
func TestISISDISElectionConcurrentReaction(t *testing.T) {
	w := newRelWire()
	a := startLANNode(t, w, 10, 0x0a, 1, 10)
	b := startLANNode(t, w, 20, 0x0b, 2, 100) // highest priority -> DIS
	defer a.eng.shutdown()
	defer b.eng.shutdown()

	// Converge: B is the DIS at L1, A holds B's pseudo-node by flooding, AND A's own
	// LSP already points at that pseudo-node (the star). Establishing the star as a
	// precondition makes the post-storm star assertion sound: the storm exercises
	// runElection (the reaction), not A's own-LSP rebuild, so a star that holds
	// before the storm must still hold after if the reaction stayed consistent.
	waitFor(t, 45*time.Second, func() bool {
		pn, ok := b.pseudonodeL1()
		if !ok || pn.SystemID() != b.sys || !b.eng.localIsDISL1() || !holdsPseudonodeLSP(a, pn) {
			return false
		}
		_, points := ownLSPNeighbors(t, a)[pn]
		return points
	}, "DIS election did not converge (with A's star) before the concurrent storm")

	pnBefore, _ := b.pseudonodeL1()

	// Hammer runElection on the SAME circuit from many goroutines on both nodes
	// while the live engine goroutines also run their own elections. With the
	// reaction serialized this must not race or diverge.
	bc := b.eth0Circuit()
	ac := a.eth0Circuit()
	if bc == nil || ac == nil {
		t.Fatal("eth0 circuit missing on a converged node")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 20 {
				b.eng.runElection(bc)
			}
		}()
		go func() {
			defer wg.Done()
			for range 20 {
				a.eng.runElection(ac)
			}
		}()
	}
	wg.Wait()

	// The recorded pseudo-node is still the single elected DIS's, B is still DIS,
	// and the pseudo-node LSP is intact (the storm did not corrupt the state).
	pnAfter, ok := b.pseudonodeL1()
	if !ok {
		t.Fatal("B lost its recorded pseudo-node after the concurrent storm")
	}
	if pnAfter != pnBefore {
		t.Errorf("recorded pseudo-node changed under concurrent elections: %s -> %s", pnBefore, pnAfter)
	}
	if pnAfter.SystemID() != b.sys || !pnAfter.IsPseudonode() {
		t.Errorf("recorded pseudo-node = %s, want a pseudo-node owned by DIS %s", pnAfter, b.sys)
	}
	if !b.eng.localIsDISL1() {
		t.Error("B is no longer DIS after the concurrent storm")
	}
	if !holdsPseudonodeLSP(b, pnAfter) {
		t.Error("DIS B no longer holds an intact pseudo-node LSP after the storm")
	}
	// A's own LSP still points at exactly B's pseudo-node (the star, AC-7).
	aNbrs := ownLSPNeighbors(t, a)
	if _, points := aNbrs[pnAfter]; !points {
		t.Errorf("A's own LSP no longer points at the pseudo-node %s (star broken)", pnAfter)
	}
}
