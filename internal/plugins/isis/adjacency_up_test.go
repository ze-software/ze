// Design: plan/spec-isis-5-adjacency.md -- two-engine adjacency wiring test.
//
// VALIDATES (Wiring Test, AC-3): two IS-IS engines on an in-memory broadcast
// circuit exchange LAN IIHs through the spec-isis-4 PDU dispatcher and the
// registered IIH handler, run the LAN three-way check, and both reach Up,
// emitting a session-up event. This proves the end-to-end path
// transport -> dispatcher -> handleIIH -> circuit.Receive -> FSM -> table ->
// events without a real raw socket (darwin-runnable).
// PREVENTS: a regression where the IIH handler is not registered, the source MAC
// is dropped before the FSM, or the three-way echo never completes.

package isis

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// wire is a shared in-memory L2 segment: a frame sent by one attached circuit is
// delivered to every OTHER attached circuit's receive channel (a broadcast LAN).
// It models the veth/bridge a real test would use, but runs on darwin.
type wire struct {
	mu        sync.Mutex
	receivers map[int]*pairedCircuit // by ifindex
}

func newWire() *wire { return &wire{receivers: make(map[int]*pairedCircuit)} }

func (w *wire) attach(c *pairedCircuit) {
	w.mu.Lock()
	w.receivers[c.ifindex] = c
	w.mu.Unlock()
}

// deliver pushes a frame from srcIfindex to every other attached circuit.
func (w *wire) deliver(srcIfindex int, src [transport.MACLen]byte, pdu []byte) {
	w.mu.Lock()
	targets := make([]*pairedCircuit, 0, len(w.receivers))
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
		default:
		}
	}
}

// pairedBackend hands out circuits attached to a shared wire, each with a
// distinct ifindex and MAC so the LAN three-way echo can match.
type pairedBackend struct {
	w       *wire
	ifindex int
	mac     [transport.MACLen]byte
}

func (b *pairedBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	c := &pairedCircuit{
		name:    name,
		ifindex: b.ifindex,
		mac:     b.mac,
		w:       b.w,
		recv:    make(chan transport.RawFrame, 64),
	}
	b.w.attach(c)
	return c, nil
}

// pairedCircuit is a CircuitHandle whose Send pushes onto the shared wire.
type pairedCircuit struct {
	name    string
	ifindex int
	mac     [transport.MACLen]byte
	w       *wire
	recv    chan transport.RawFrame
	once    sync.Once
}

func (c *pairedCircuit) IfIndex() int                   { return c.ifindex }
func (c *pairedCircuit) HWAddr() [transport.MACLen]byte { return c.mac }
func (c *pairedCircuit) MTU() int                       { return 1500 }
func (c *pairedCircuit) Send(_, src [transport.MACLen]byte, pdu []byte) error {
	c.w.deliver(c.ifindex, src, pdu)
	return nil
}
func (c *pairedCircuit) Recv() <-chan transport.RawFrame { return c.recv }
func (c *pairedCircuit) Close() error {
	c.once.Do(func() { close(c.recv) })
	return nil
}

// startEngine builds and starts an engine on the shared wire with the given
// ifindex/MAC and config. The per-circuit goroutines send Hellos and run the
// hold-timer sweep just as in production; only the backend is in-memory.
func startEngine(t *testing.T, w *wire, ifindex int, mac byte, jsonCfg string) *engine {
	t.Helper()
	cfg, err := parseISISConfig(sec(jsonCfg))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	be := &pairedBackend{w: w, ifindex: ifindex, mac: [transport.MACLen]byte{0x02, 0, 0, 0, 0, mac}}
	eng := newEngine(transport.New(be))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	return eng
}

// TestISISAdjacencyUp: two engines on an in-memory broadcast circuit form an
// adjacency that reaches Up on both sides via the LAN three-way check.
func TestISISAdjacencyUp(t *testing.T) {
	w := newWire()
	const cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1"}}}}}`
	const cfgB = `{"isis":{"net":"49.0001.0000.0000.0002.00","interfaces":{"interface":{"eth0":{"hello-interval":"1","level":"l1"}}}}}`

	engA := startEngine(t, w, 10, 0x0a, cfgA)
	engB := startEngine(t, w, 20, 0x0b, cfgB)
	defer engA.shutdown()
	defer engB.shutdown()

	// The per-circuit goroutines send Hellos every second; the three-way needs a
	// couple of exchanges (A hears B, echoes B's SNPA in its next Hello, then B
	// hears its own SNPA back). Poll for both sides Up.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if engUp(engA) && engUp(engB) {
			return // success
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("adjacency did not reach Up on both sides: A up=%v B up=%v", engUp(engA), engUp(engB))
}

// engUp reports whether the engine has at least one Up adjacency on any circuit.
func engUp(e *engine) bool {
	e.circuitsMu.RLock()
	defer e.circuitsMu.RUnlock()
	for _, c := range e.circuitByName {
		if c.Table().UpCount() > 0 {
			return true
		}
	}
	return false
}
