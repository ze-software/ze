// Design: plan/spec-isis-7-flooding.md -- engine-layer flooding wiring tests (Wiring Test table).
//
// VALIDATES: the end-to-end glue this spec owns at the engine layer, on darwin
// via the in-memory wire (no raw socket):
//   - a newer LSP received on a circuit floods onward and an LSDB converges across
//     a three-node line A-B-C (TestISISLSDBSync, the umbrella Wiring Test row);
//   - the periodic flood timer transmits an SRM-armed LSP to a P2P neighbor and
//     SRM clears after the send (TestISISFloodSRMTimer, AC-5);
//   - a CSNP listing an LSP the receiver does not hold records a pending-request
//     and a PSNP requesting it is emitted (TestISISCSNPGapRequest, AC-15);
//   - a PSNP acknowledging our LSP at our sequence clears SRM (TestISISPSNPAck,
//     AC-9).
// PREVENTS: a regression where the LSP/CSNP/PSNP dispatcher handlers are not
// registered (the stub no-ops persist), the flood timer never runs, or the P2P
// initial CSNP is not wired.

package isis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// relWire is a lossless in-memory L2 segment for flooding tests: a frame sent by
// one attached circuit is delivered to every OTHER attached circuit, blocking
// briefly rather than dropping (a real point-to-point/LAN link does not randomly
// drop, and flooding convergence assertions need determinism, unlike the Hello
// path which self-heals via 1s repeats). Delivery respects the receiver's stop so
// shutdown never deadlocks.
type relWire struct {
	mu        sync.Mutex
	receivers map[int]*relCircuit
}

func newRelWire() *relWire { return &relWire{receivers: map[int]*relCircuit{}} }

func (w *relWire) attach(c *relCircuit) {
	w.mu.Lock()
	w.receivers[c.ifindex] = c
	w.mu.Unlock()
}

func (w *relWire) detach(ifindex int) {
	w.mu.Lock()
	delete(w.receivers, ifindex)
	w.mu.Unlock()
}

// deliver pushes a frame from srcIfindex to every other attached circuit,
// blocking on a full buffer (lossless) but unblocking if the receiver stops.
func (w *relWire) deliver(srcIfindex int, src [transport.MACLen]byte, pdu []byte) {
	w.mu.Lock()
	targets := make([]*relCircuit, 0, len(w.receivers))
	for ifx, c := range w.receivers {
		if ifx != srcIfindex {
			targets = append(targets, c)
		}
	}
	w.mu.Unlock()
	for _, c := range targets {
		cp := append([]byte(nil), pdu...)
		select {
		case c.recv <- transport.RawFrame{IfIndex: c.ifindex, SrcMAC: src, PDU: cp}:
		case <-c.stop:
		}
	}
}

// relCircuit is a lossless CircuitHandle on a relWire.
type relCircuit struct {
	name    string
	ifindex int
	mac     [transport.MACLen]byte
	w       *relWire
	recv    chan transport.RawFrame
	stop    chan struct{}
	once    sync.Once
}

func (c *relCircuit) IfIndex() int                   { return c.ifindex }
func (c *relCircuit) HWAddr() [transport.MACLen]byte { return c.mac }
func (c *relCircuit) MTU() int                       { return 1500 }
func (c *relCircuit) Send(_, src [transport.MACLen]byte, pdu []byte) error {
	c.w.deliver(c.ifindex, src, pdu)
	return nil
}
func (c *relCircuit) Recv() <-chan transport.RawFrame { return c.recv }
func (c *relCircuit) Close() error {
	c.once.Do(func() {
		close(c.stop)
		c.w.detach(c.ifindex)
		close(c.recv)
	})
	return nil
}

// multiBackend hands out per-interface lossless circuits each attached to a named
// wire, so one engine can have several circuits on distinct segments (a line
// topology). It maps interface name -> (wire, ifindex, mac).
type multiBackend struct {
	wires   map[string]*relWire
	ifindex map[string]int
	mac     map[string][transport.MACLen]byte
}

func newMultiBackend() *multiBackend {
	return &multiBackend{
		wires:   map[string]*relWire{},
		ifindex: map[string]int{},
		mac:     map[string][transport.MACLen]byte{},
	}
}

func (b *multiBackend) addLink(name string, w *relWire, ifindex int, mac byte) {
	b.wires[name] = w
	b.ifindex[name] = ifindex
	b.mac[name] = [transport.MACLen]byte{0x02, 0, 0, 0, 0, mac}
}

func (b *multiBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	c := &relCircuit{
		name:    name,
		ifindex: b.ifindex[name],
		mac:     b.mac[name],
		w:       b.wires[name],
		recv:    make(chan transport.RawFrame, 256),
		stop:    make(chan struct{}),
	}
	b.wires[name].attach(c)
	return c, nil
}

// startEngineMulti builds and starts an engine with a multi-link backend.
func startEngineMulti(t *testing.T, be *multiBackend, jsonCfg string) *engine {
	t.Helper()
	return startEngineMultiWithMetrics(t, be, jsonCfg, nil)
}

// startEngineMultiWithMetrics builds and starts an engine, optionally wiring a
// metrics registry BEFORE the engine's loops start (the production order in
// register.go: newEngine -> setMetrics -> setConfig -> openCircuits). Wiring
// metrics before openCircuits is required: setMetrics rebinds per-subsystem
// metric handles, and once the receive/aging/flood/SPF loops run, a concurrent
// setMetrics would race those goroutines (the engine never re-binds metrics after
// start in production). A nil reg skips metric wiring.
func startEngineMultiWithMetrics(t *testing.T, be *multiBackend, jsonCfg string, reg metrics.Registry) *engine {
	t.Helper()
	cfg, err := parseISISConfig(sec(jsonCfg))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	eng := newEngine(transport.New(be))
	if reg != nil {
		eng.setMetrics(reg)
	}
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	return eng
}

// engHoldsForeignL1 reports whether engine e holds, at Level 1, an LSP
// originated by the System ID sys (fragment 0), i.e. a foreign LSP it learned by
// flooding. Used to assert LSDB convergence across the L1 line.
func engHoldsForeignL1(e *engine, sys types.SystemID) bool {
	id := types.NewLSPID(types.NewSourceID(sys, 0), 0)
	return e.lsdb.Lookup(lsdb.Level1, id) != nil
}

// TestISISLSDBSync wires a three-node line A-B-C over two point-to-point segments
// (A<->B and B<->C). Each node originates its own LSP at start; once adjacencies
// form, flooding propagates every node's LSP across the line so A's LSP reaches C
// (two hops, via B) and C's reaches A. This is the umbrella Wiring Test row
// "adjacency Up on two nodes -> LSPs exchanged, LSDB synced", extended to a line
// so the onward re-flood (B floods A's LSP to C) is exercised. L1-only keeps the
// area match simple.
func TestISISLSDBSync(t *testing.T) {
	wAB := newRelWire()
	wBC := newRelWire()

	// A: one P2P circuit on wAB.
	beA := newMultiBackend()
	beA.addLink("eth0", wAB, 10, 0x0a)
	// B: two P2P circuits, eth0 on wAB and eth1 on wBC.
	beB := newMultiBackend()
	beB.addLink("eth0", wAB, 20, 0x0b)
	beB.addLink("eth1", wBC, 21, 0x1b)
	// C: one P2P circuit on wBC.
	beC := newMultiBackend()
	beC.addLink("eth0", wBC, 30, 0x0c)

	const cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	const cfgB = `{"isis":{"net":"49.0001.0000.0000.0002.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"},"eth1":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	const cfgC = `{"isis":{"net":"49.0001.0000.0000.0003.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`

	engA := startEngineMulti(t, beA, cfgA)
	engB := startEngineMulti(t, beB, cfgB)
	engC := startEngineMulti(t, beC, cfgC)
	defer engA.shutdown()
	defer engB.shutdown()
	defer engC.shutdown()

	sysA := engA.cfg.SystemID
	sysC := engC.cfg.SystemID

	// Poll for convergence: A's LSP must reach C and C's must reach A (proving the
	// two-hop onward flood through B), and B holds both. The flood timer is 5 s and
	// adjacencies need a few Hello exchanges, so allow a generous deadline.
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		if engHoldsForeignL1(engC, sysA) &&
			engHoldsForeignL1(engA, sysC) &&
			engHoldsForeignL1(engB, sysA) &&
			engHoldsForeignL1(engB, sysC) {
			return // converged
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("LSDB did not converge across the line: C<-A=%v A<-C=%v B<-A=%v B<-C=%v",
		engHoldsForeignL1(engC, sysA),
		engHoldsForeignL1(engA, sysC),
		engHoldsForeignL1(engB, sysA),
		engHoldsForeignL1(engB, sysC))
}

// TestISISFloodSRMTimer wires two nodes over a P2P link and asserts the periodic
// flood timer transmits A's originated LSP to B (B's LSDB gains A's LSP) and A's
// SRM for that LSP on the circuit clears once B acknowledges it (AC-5/AC-9; the
// flood timer leaves SRM set on send and B's reciprocal PSNP ack clears it, per
// ISO/IEC 10589 clause 7.3.15.1 -- NOT a blind clear-on-send).
func TestISISFloodSRMTimer(t *testing.T) {
	w := newRelWire()
	beA := newMultiBackend()
	beA.addLink("eth0", w, 10, 0x0a)
	beB := newMultiBackend()
	beB.addLink("eth0", w, 20, 0x0b)

	const cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	const cfgB = `{"isis":{"net":"49.0001.0000.0000.0002.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`

	engA := startEngineMulti(t, beA, cfgA)
	engB := startEngineMulti(t, beB, cfgB)
	defer engA.shutdown()
	defer engB.shutdown()

	sysA := engA.cfg.SystemID
	idA := types.NewLSPID(types.NewSourceID(sysA, 0), 0)
	cidA := engA.circuitIDFor("eth0")

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		// B has A's LSP and A's SRM on the circuit is cleared by B's PSNP ack.
		if engB.lsdb.Lookup(lsdb.Level1, idA) != nil && !engA.lsdb.SRM(lsdb.Level1, idA, cidA) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("flood timer did not deliver A's LSP / clear SRM: B-has-A=%v A-SRM=%v",
		engB.lsdb.Lookup(lsdb.Level1, idA) != nil, engA.lsdb.SRM(lsdb.Level1, idA, cidA))
}

// probe is a second circuit attached to an engine's wire so a test can inject
// crafted PDUs (a CSNP/PSNP) into the engine's receive path and observe the
// reaction, without a second full engine.
type probe struct {
	c *relCircuit
}

// newProbe attaches a probe circuit to wire w with a distinct ifindex/mac.
func newProbe(w *relWire, ifindex int, mac byte) *probe {
	c := &relCircuit{
		name:    "probe",
		ifindex: ifindex,
		mac:     [transport.MACLen]byte{0x02, 0, 0, 0, 0, mac},
		w:       w,
		recv:    make(chan transport.RawFrame, 64),
		stop:    make(chan struct{}),
	}
	w.attach(c)
	return &probe{c: c}
}

// send pushes a crafted PDU from the probe onto the wire (delivered to the
// engine's circuit on the same segment).
func (p *probe) send(pdu []byte) { p.c.w.deliver(p.c.ifindex, p.c.mac, pdu) }

// TestISISCSNPGapRequest wires a single engine and injects, from a probe on the
// same segment, a CSNP listing an LSP the engine does NOT hold. The engine must
// record a pending-request on that circuit and (on the PSNP timer) emit a PSNP
// requesting the missing LSP. We assert the pending-request is recorded (the
// observable engine-side effect of the gap detection wiring, AC-15).
func TestISISCSNPGapRequest(t *testing.T) {
	w := newRelWire()
	be := newMultiBackend()
	be.addLink("eth0", w, 10, 0x0a)
	const cfg = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	eng := startEngineMulti(t, be, cfg)
	defer eng.shutdown()

	pr := newProbe(w, 99, 0x99)

	// A CSNP listing an LSP ID (System ID 0x42) the engine does not hold.
	missing := types.NewLSPID(types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 0x42}, 0), 0)
	csnp := buildProbeCSNP(t, missing)

	cid := eng.circuitIDFor("eth0")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pr.send(csnp)
		if eng.flooder.PendingCount(cid, lsdb.Level1) > 0 {
			return // gap recorded as a pending-request (AC-15)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("CSNP gap for a not-held LSP did not record a pending-request (AC-15)")
}

// TestISISPSNPAck wires a single engine, arms SRM for its own LSP on the circuit,
// then injects a PSNP (from a probe) acknowledging that LSP at the engine's
// sequence. The engine must clear SRM on the circuit (AC-9).
func TestISISPSNPAck(t *testing.T) {
	w := newRelWire()
	be := newMultiBackend()
	be.addLink("eth0", w, 10, 0x0a)
	const cfg = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	eng := startEngineMulti(t, be, cfg)
	defer eng.shutdown()

	pr := newProbe(w, 99, 0x99)

	// The engine's own L1 fragment 0 (originated at start).
	sys := eng.cfg.SystemID
	id := types.NewLSPID(types.NewSourceID(sys, 0), 0)
	cid := eng.circuitIDFor("eth0")
	// Wait for origination, then arm SRM (as if we owe the LSP on the circuit).
	waitForLSP(t, eng, lsdb.Level1, id)
	seq := eng.lsdb.Lookup(lsdb.Level1, id).Sequence()
	eng.lsdb.SetSRM(lsdb.Level1, id, cid)

	psnp := buildProbePSNP(t, id, seq, eng.lsdb.Lookup(lsdb.Level1, id).Lifetime(), eng.lsdb.Lookup(lsdb.Level1, id).Checksum())

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		pr.send(psnp)
		if !eng.lsdb.SRM(lsdb.Level1, id, cid) {
			return // SRM cleared by the PSNP ack (AC-9)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("PSNP ack at our sequence did not clear SRM (AC-9)")
}

// waitForLSP blocks until the engine holds (level, id) or fails after a timeout.
func waitForLSP(t *testing.T, e *engine, level lsdb.Level, id types.LSPID) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if e.lsdb.Lookup(level, id) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("engine never originated/stored LSP %s at %s", id, level)
}

// buildProbeCSNP builds an L1 CSNP PDU listing one LSP entry (seq 7, lifetime
// 1000) over the whole LSP-ID range, sourced from a probe Source ID.
func buildProbeCSNP(t *testing.T, id types.LSPID) []byte {
	t.Helper()
	var maxID types.LSPID
	for i := range maxID {
		maxID[i] = 0xff
	}
	entry := packet.LSPEntry{LSPID: id, SequenceNumber: 7, RemainingLifetime: 1000, Checksum: 0x1234}
	ev := packet.LSPEntriesTLV{Entries: []packet.LSPEntry{entry}}
	tbuf := make([]byte, ev.EncodedLen())
	tn := packet.WriteLSPEntriesTLV(tbuf, 0, ev)
	c := packet.CSNP{
		PDUType:    packet.PDUTypeL1CSNP,
		SourceID:   types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 0x99}, 0),
		StartLSPID: types.LSPID{},
		EndLSPID:   maxID,
		TLVs:       []packet.TLV{{Type: packet.TLVLSPEntries, Value: tbuf[packet.TLVHeaderLen:tn]}},
	}
	buf := make([]byte, c.EncodedLen())
	n := c.WriteTo(buf, 0)
	return buf[:n]
}

// scrapeEngine renders the Prometheus text exposition for reg (engine-layer
// metric assertions).
func scrapeEngine(t *testing.T, reg *metrics.PrometheusRegistry) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/metrics", http.NoBody)
	w := httptest.NewRecorder()
	reg.Handler().ServeHTTP(w, req)
	return w.Body.String()
}

// buildForeignLSP builds an L1 LSP for a foreign System ID (one octet sys) with a
// VALID Fletcher checksum, returning the LSP ID and the on-wire bytes.
func buildForeignLSP(t *testing.T, sys byte, seq types.SequenceNumber) (types.LSPID, []byte) {
	t.Helper()
	id := types.NewLSPID(types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, sys}, 0), 0)
	lsp := &packet.LSP{
		PDUType:           packet.PDUTypeL1LSP,
		RemainingLifetime: 1000,
		LSPID:             id,
		SequenceNumber:    seq,
		TLVs:              []packet.TLV{{Type: 222, Value: []byte{0xAA, 0xBB}}},
	}
	buf := make([]byte, lsp.EncodedLen())
	n := lsp.WriteTo(buf, 0) // WriteTo computes and backfills the Fletcher checksum
	return id, buf[:n]
}

// TestISISReceivedLSPBadChecksumDropped injects, from a probe on the engine's
// segment, an LSP whose Fletcher checksum does NOT verify (a body byte flipped
// after the checksum was computed). The engine MUST discard it: never store it in
// the LSDB and never flood it, bumping ze_isis_lsps_dropped_total{reason=
// "bad-checksum"} (ISO/IEC 10589 sec 7.3.14.2). A control LSP with a valid
// checksum, identical otherwise, IS stored -- proving the checksum is the only
// difference and the drop is real. Regression for finding B2-1: received LSPs
// were never checksum-validated before being stored/re-flooded.
func TestISISReceivedLSPBadChecksumDropped(t *testing.T) {
	w := newRelWire()
	be := newMultiBackend()
	be.addLink("eth0", w, 10, 0x0a)
	const cfg = `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1","circuit-type":"point-to-point"}}}}}`
	// Wire metrics BEFORE the engine starts (production order) so the scrape sees
	// the bad-checksum counter without racing the engine's running goroutines.
	reg := metrics.NewPrometheusRegistry()
	eng := startEngineMultiWithMetrics(t, be, cfg, reg)
	defer eng.shutdown()

	pr := newProbe(w, 99, 0x99)

	// (a) A corrupt LSP: build a valid one, then flip the LAST body byte (inside a
	// TLV value) so the stored checksum no longer matches the data. DecodePDU does
	// not verify the checksum, so the engine must reject it in handleLSP.
	badID, bad := buildForeignLSP(t, 0x55, 7)
	bad[len(bad)-1] ^= 0xFF
	// Sanity: the corrupted bytes must indeed fail verification (else the test is
	// vacuous).
	if dec, err := packet.DecodePDU(bad); err != nil || dec.LSP == nil || dec.LSP.VerifyChecksum() {
		t.Fatalf("corrupt LSP did not fail VerifyChecksum (test fixture invalid): err=%v", err)
	}

	// Send the corrupt LSP repeatedly for a window; it must NEVER appear in the LSDB.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pr.send(bad)
		if eng.lsdb.Lookup(lsdb.Level1, badID) != nil {
			t.Fatal("a corrupt-checksum LSP was stored in the LSDB (must be dropped, finding B2-1)")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// The drop is observable on the canonical counter with reason="bad-checksum".
	out := scrapeEngine(t, reg)
	if !strings.Contains(out, `ze_isis_lsps_dropped_total{level="l1",reason="bad-checksum"}`) {
		t.Errorf("corrupt LSP drop not counted on reason=\"bad-checksum\":\n%s", out)
	}

	// (b) Control: the SAME LSP-ID with a VALID checksum is accepted and stored,
	// proving the only difference was the checksum (and the receive path otherwise
	// works on this circuit).
	goodID, good := buildForeignLSP(t, 0x55, 8)
	if goodID != badID {
		t.Fatalf("control LSP-ID %v != corrupt LSP-ID %v", goodID, badID)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pr.send(good)
		if eng.lsdb.Lookup(lsdb.Level1, goodID) != nil {
			return // stored: a valid-checksum LSP on the same circuit IS accepted.
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("a valid-checksum LSP on the same circuit was not stored (the drop was not checksum-specific)")
}

// buildProbePSNP builds an L1 PSNP PDU acknowledging one LSP at the given
// sequence/lifetime/checksum, sourced from a probe Source ID.
func buildProbePSNP(t *testing.T, id types.LSPID, seq types.SequenceNumber, lifetime types.RemainingLifetime, checksum uint16) []byte {
	t.Helper()
	entry := packet.LSPEntry{LSPID: id, SequenceNumber: seq, RemainingLifetime: lifetime, Checksum: checksum}
	ev := packet.LSPEntriesTLV{Entries: []packet.LSPEntry{entry}}
	tbuf := make([]byte, ev.EncodedLen())
	tn := packet.WriteLSPEntriesTLV(tbuf, 0, ev)
	p := packet.PSNP{
		PDUType:  packet.PDUTypeL1PSNP,
		SourceID: types.NewSourceID(types.SystemID{0, 0, 0, 0, 0, 0x99}, 0),
		TLVs:     []packet.TLV{{Type: packet.TLVLSPEntries, Value: tbuf[packet.TLVHeaderLen:tn]}},
	}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	return buf[:n]
}
