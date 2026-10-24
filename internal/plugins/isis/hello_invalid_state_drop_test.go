// Design: docs/architecture/isis/isis-5-adjacency.md -- a discarded Hello is counted.
// Related: server.go -- engine.handleIIH, transport/transport.go -- Transport.CountDrop
//
// VALIDATES: a Hello with an invalid TLV 240 state reaching the engine's IIH
// handler is discarded, creates no adjacency, and is counted by reason.
// PREVENTS: an RFC 5303 Section 3.2 discard that no operator can see.

package isis

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// dropRegistry records every increment of ze_isis_frames_dropped_total by its
// label values; every other series is a no-op.
type dropRegistry struct {
	metrics.NopRegistry
	mu    sync.Mutex
	drops map[string]int // "interface/reason" -> count
}

type dropVec struct{ reg *dropRegistry }

type dropCounter struct {
	reg *dropRegistry
	key string
}

func (r *dropRegistry) CounterVec(name, help string, labelNames []string) metrics.CounterVec {
	if name == "ze_isis_frames_dropped_total" {
		return dropVec{reg: r}
	}
	return r.NopRegistry.CounterVec(name, help, labelNames)
}

func (v dropVec) With(labelValues ...string) metrics.Counter {
	return dropCounter{reg: v.reg, key: labelValues[0] + "/" + labelValues[1]}
}

func (v dropVec) Delete(...string) bool { return false }

func (c dropCounter) Inc() { c.Add(1) }

func (c dropCounter) Add(n float64) {
	c.reg.mu.Lock()
	defer c.reg.mu.Unlock()
	c.reg.drops[c.key] += int(n)
}

// TestISISHelloInvalidThreeWayStateDiscarded feeds a point-to-point IIH whose
// TLV 240 carries the unnamed Adjacency Three-Way State 3 through the engine's
// real IIH handler. The Hello must be discarded with no adjacency created, and
// counted in ze_isis_frames_dropped_total under invalid-three-way-state. A legacy
// Hello from the same neighbor afterwards still brings the adjacency Up, so the
// discard took no other action.
//
// RFC 5303 Section 3.2: "If the option is present and contains invalid
// Adjacency Three-Way State, the PDU SHALL be discarded and no further action
// is taken."
func TestISISHelloInvalidThreeWayStateDiscarded(t *testing.T) {
	cfg, err := parseISISConfig(sec(`{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"circuit-type":"point-to-point"}}}}}`))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	reg := &dropRegistry{drops: make(map[string]int)}
	tr := transport.New(&fakeBackend{})
	tr.SetMetrics(reg)
	eng := newEngine(tr)
	defer eng.shutdown()
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}

	eng.circuitsMu.RLock()
	c := eng.circuitByName["eth0"]
	ifindex := -1
	for idx, byIndex := range eng.circuits {
		if byIndex == c {
			ifindex = idx
		}
	}
	eng.circuitsMu.RUnlock()
	if c == nil || ifindex < 0 {
		t.Fatal("eth0 circuit not built")
	}

	neighbor := types.SystemID{0, 0, 0, 0, 0, 2}
	area := eng.cfg.NETs[0].AreaID()
	invalid := invalidThreeWayHelloPDU(t, neighbor, area)
	eng.handleIIH(transport.RawFrame{IfIndex: ifindex, PDU: invalid})

	if got := c.Table().UpCount(); got != 0 {
		t.Fatalf("an invalid three-way Hello brought %d adjacencies Up", got)
	}
	reg.mu.Lock()
	dropped := reg.drops["eth0/invalid-three-way-state"]
	reg.mu.Unlock()
	if dropped != 1 {
		t.Fatalf("ze_isis_frames_dropped_total{eth0,invalid-three-way-state} = %d, want 1 (drops: %v)", dropped, reg.drops)
	}

	if up := c.Receive(adjacency.SNPA{}, p2pHelloPDU(t, neighbor, area)); !up.SessionUp {
		t.Fatalf("a valid Hello after the discard did not bring the adjacency Up: %+v", up)
	}
}

// invalidThreeWayHelloPDU builds the same IIH as p2pHelloPDU plus a TLV 240
// whose one-octet value is state 3, which RFC 5303 Section 3.1 does not define.
func invalidThreeWayHelloPDU(t *testing.T, sys types.SystemID, area types.AreaID) []byte {
	t.Helper()
	var areaScratch [13]byte
	an := area.WriteTo(areaScratch[:], 0)
	areaVal := append([]byte{byte(an)}, areaScratch[:an]...)

	h := &packet.P2PHello{
		CircuitType:    packet.CircuitL1L2,
		SystemID:       sys,
		HoldingTime:    30,
		LocalCircuitID: 1,
		TLVs: []packet.TLV{
			{Type: packet.TLVAreaAddresses, Value: areaVal},
			{Type: packet.TLVProtocolsSupported, Value: []byte{packet.NLPIDIPv4}},
			{Type: packet.TLVIPInterfaceAddress, Value: []byte{192, 0, 2, 2}},
			{Type: packet.TLVP2PThreeWay, Value: []byte{3}},
		},
	}
	buf := make([]byte, h.EncodedLen())
	n := h.WriteTo(buf, 0)
	return buf[:n]
}
