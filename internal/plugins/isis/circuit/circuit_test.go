// Design: plan/spec-isis-5-adjacency.md -- circuit RX dispatch + lifecycle.
//
// VALIDATES: a received LAN IIH whose TLV 6 echoes our SNPA drives the adjacency
// to Up via the circuit's Receive path (codec -> FSM -> table); the stored TLV
// 132 next-hop survives the round trip; the hold-timer Sweep times an adjacency
// out; and Teardown drops every adjacency on circuit-down. Uses a controllable
// clock so the timer behavior is deterministic.
// PREVENTS: a regression where the RX path does not parse the IIH TLVs, the
// three-way echo is ignored, or the sweep/teardown never fires.

package circuit

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// fakeClock is a controllable monotonic clock for deterministic timer tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// recordSink captures session events the circuit emits.
type recordSink struct {
	mu   sync.Mutex
	up   []adjacency.NeighborSnapshot
	down []adjacency.NeighborSnapshot
}

func (r *recordSink) SessionUp(s adjacency.NeighborSnapshot) {
	r.mu.Lock()
	r.up = append(r.up, s)
	r.mu.Unlock()
}

func (r *recordSink) SessionDown(s adjacency.NeighborSnapshot) {
	r.mu.Lock()
	r.down = append(r.down, s)
	r.mu.Unlock()
}

// buildPeerLANHello encodes a Level-1 LAN IIH from peerSys carrying TLV 1, 132,
// and a TLV 6 listing the supplied echoed SNPAs.
func buildPeerLANHello(t *testing.T, echoed [][packet.SNPALen]byte) []byte {
	t.Helper()
	area, _ := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	areaVal := []byte{byte(area.Len())}
	areaVal = append(areaVal, area.Bytes()...)
	v4 := netip.MustParseAddr("192.0.2.2").As4()

	snpaVal := make([]byte, 0, len(echoed)*packet.SNPALen)
	for _, s := range echoed {
		snpaVal = append(snpaVal, s[:]...)
	}

	h := packet.LANHello{
		PDUType:     packet.PDUTypeL1LANHello,
		CircuitType: packet.CircuitL1,
		SystemID:    types.SystemID{0, 0, 0, 0, 0, 2},
		HoldingTime: types.HoldingTime(30),
		Priority:    64,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaVal},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVIPInterfaceAddress, Value: v4[:]},
			{Type: packet.TLVISNeighbors, Value: snpaVal},
		},
	}
	buf := make([]byte, h.EncodedLen())
	n := h.WriteTo(buf, 0)
	return buf[:n]
}

func clockCircuit(t *testing.T, clk *fakeClock) (*Circuit, *recordSink) {
	t.Helper()
	area, _ := types.AreaIDFromBytes([]byte{0x49, 0x00, 0x01})
	c := New(Config{
		Name:          "eth0",
		IfIndex:       3,
		SystemID:      types.SystemID{0, 0, 0, 0, 0, 1},
		SNPA:          adjacency.SNPA{0x02, 0, 0, 0, 0, 1},
		Areas:         []types.AreaID{area},
		IPv4:          netip.MustParseAddr("192.0.2.1"),
		Kind:          adjacency.KindBroadcast,
		Levels:        []adjacency.Level{adjacency.Level1},
		HelloInterval: 10,
		HoldMult:      3,
	}, &fakeSender{mtu: 1500}, clk.now)
	sink := &recordSink{}
	c.SetEventSink(sink)
	return c, sink
}

// ourSNPA is the circuit's own MAC, echoed by a neighbor to complete the
// LAN three-way.
var ourSNPA = [packet.SNPALen]byte{0x02, 0, 0, 0, 0, 1}

// TestISISCircuitReceiveLANThreeWay: a LAN IIH that does NOT echo our SNPA keeps
// the adjacency Initializing; once it echoes our SNPA the adjacency reaches Up
// via Receive, the next-hop is stored, and a session-up event fires.
func TestISISCircuitReceiveLANThreeWay(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, sink := clockCircuit(t, clk)

	// First Hello: no echo of our SNPA -> Initializing.
	pdu := buildPeerLANHello(t, [][packet.SNPALen]byte{{0x02, 0, 0, 0, 0, 9}})
	tr := c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, pdu)
	if tr.State != adjacency.StateInitializing {
		t.Fatalf("no echo -> state %v, want initializing", tr.State)
	}

	// Second Hello: echoes our SNPA -> Up + session-up.
	pdu = buildPeerLANHello(t, [][packet.SNPALen]byte{ourSNPA})
	tr = c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, pdu)
	if tr.State != adjacency.StateUp {
		t.Fatalf("echo present -> state %v, want up", tr.State)
	}
	sink.mu.Lock()
	upCount := len(sink.up)
	sink.mu.Unlock()
	if upCount != 1 {
		t.Fatalf("session-up events = %d, want 1", upCount)
	}

	// The stored next-hop is the neighbor's TLV 132 address (AC-10).
	rows := c.Table().Snapshot()
	if len(rows) != 1 || rows[0].IPv4 != "192.0.2.2" {
		t.Fatalf("snapshot next-hop = %+v, want IPv4 192.0.2.2", rows)
	}
}

// TestISISHoldTimerExpiry: an Up adjacency that hears no Hello within the
// advertised hold time is swept to Down and a session-down event fires.
func TestISISHoldTimerExpiry(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, sink := clockCircuit(t, clk)

	pdu := buildPeerLANHello(t, [][packet.SNPALen]byte{ourSNPA})
	if tr := c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, pdu); tr.State != adjacency.StateUp {
		t.Fatalf("setup: state %v, want up", tr.State)
	}

	// Within the hold time (30s): Sweep keeps it Up.
	clk.advance(10 * time.Second)
	if dropped := c.Sweep(); dropped != 0 {
		t.Fatalf("Sweep within hold time dropped %d, want 0", dropped)
	}

	// Past the hold time: Sweep times it out.
	clk.advance(25 * time.Second)
	if dropped := c.Sweep(); dropped != 1 {
		t.Fatalf("Sweep past hold time dropped %d, want 1", dropped)
	}
	sink.mu.Lock()
	downCount := len(sink.down)
	sink.mu.Unlock()
	if downCount != 1 {
		t.Fatalf("session-down events = %d, want 1", downCount)
	}
}

// TestISISCircuitDownTeardown: Teardown drops every adjacency on the circuit and
// emits session-down (AC-8).
func TestISISCircuitDownTeardown(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, sink := clockCircuit(t, clk)

	pdu := buildPeerLANHello(t, [][packet.SNPALen]byte{ourSNPA})
	if tr := c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, pdu); tr.State != adjacency.StateUp {
		t.Fatalf("setup: state %v, want up", tr.State)
	}

	c.Teardown()
	sink.mu.Lock()
	downCount := len(sink.down)
	sink.mu.Unlock()
	if downCount != 1 {
		t.Fatalf("Teardown session-down events = %d, want 1", downCount)
	}
	for _, row := range c.Table().Snapshot() {
		if row.State == adjacency.StateUp.String() {
			t.Errorf("adjacency still Up after Teardown: %+v", row)
		}
	}
}

// TestISISCircuitReceiveSweepRace: Receive and Sweep run concurrently on one
// circuit over a single Up->Down edge, and EXACTLY one SessionDown is delivered.
//
// The B4 fix (Sweep stops after Teardown) made fireEvents safe only because the
// FSM transition functions are the single delivery point: classify() emits a
// SessionDown ONLY on the exact Up->non-Up edge, and table.Update / table.Each
// both hold the table write lock, so whichever path transitions the record away
// from Up first wins; the other observes a non-Up record and emits nothing.
//
// This is a -race regression test: run it with `go test ./.../circuit -race`.
// It drives one neighbor Up, advances the clock past the hold time, then races a
// non-echoing Receive (would drive Up->Initializing) against a Sweep (would drive
// Up->Down via the hold timer). Both are down edges out of Up; the lock must let
// only one observe the Up state. Repeated many times to surface a data race or a
// double delivery if fireEvents ever stopped depending on the single FSM edge.
func TestISISCircuitReceiveSweepRace(t *testing.T) {
	const iterations = 200
	for i := range iterations {
		clk := &fakeClock{t: time.Unix(int64(3_000_000+i*1000), 0)}
		c, sink := clockCircuit(t, clk)

		// Bring the neighbor Up (echoes our SNPA).
		up := buildPeerLANHello(t, [][packet.SNPALen]byte{ourSNPA})
		if tr := c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, up); tr.State != adjacency.StateUp {
			t.Fatalf("iter %d setup: state %v, want up", i, tr.State)
		}

		// Past the hold time so Sweep would expire the Up adjacency.
		clk.advance(40 * time.Second)

		// A non-echoing Hello: Receive would drive Up->Initializing (also a down
		// edge). Race it against Sweep (Up->Down). Exactly ONE down edge exists.
		noEcho := buildPeerLANHello(t, [][packet.SNPALen]byte{{0x02, 0, 0, 0, 0, 9}})

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Receive(adjacency.SNPA{0x02, 0, 0, 0, 0, 2}, noEcho)
		}()
		go func() {
			defer wg.Done()
			c.Sweep()
		}()
		wg.Wait()

		sink.mu.Lock()
		downCount := len(sink.down)
		sink.mu.Unlock()
		if downCount != 1 {
			t.Fatalf("iter %d: SessionDown events = %d, want exactly 1 for one Up->Down edge", i, downCount)
		}
	}
}

// TestISISCircuitReceiveMalformed: a malformed PDU is rejected without panicking
// and forms no adjacency.
func TestISISCircuitReceiveMalformed(t *testing.T) {
	clk := &fakeClock{t: time.Unix(3_000_000, 0)}
	c, _ := clockCircuit(t, clk)
	tr := c.Receive(adjacency.SNPA{}, []byte{0x00, 0x01, 0x02})
	if !tr.Rejected {
		t.Fatalf("malformed PDU not rejected: %+v", tr)
	}
	if c.Table().Len() != 0 {
		t.Errorf("malformed PDU created a table entry")
	}
}
