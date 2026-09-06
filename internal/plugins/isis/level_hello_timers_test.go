// Design: docs/architecture/isis/isis-5-adjacency.md -- per-level Hello timers.
//
// VALIDATES the wiring of spec-isis-per-level-hello-timers: the LEVEL-NESTED
// `interfaces/interface/level-1/hello-interval` and `hold-multiplier` leaves
// (and their level-2 twins) are parsed, resolved against the circuit-wide
// leaves, carried into the circuit the engine builds, and reach the wire as the
// Hello period and the advertised holding time of that level alone.
// PREVENTS: the regression the spec closes -- the four leaves parsed into
// LevelInterfaceConfig with no reader, so the circuit-wide value drove every
// IIH and the committed help promising an override was false. A test that sets
// `hello-interval` directly under `interface` cannot see this: that is the
// circuit-wide leaf, which already worked.

package isis

import (
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/transport"
)

// capturingBackend hands out capturingCircuits that keep every frame they are
// asked to send, so a test can decode the IIH the engine really produced.
type capturingBackend struct {
	mu      sync.Mutex
	circuit *capturingCircuit
}

func (b *capturingBackend) OpenCircuit(name string) (transport.CircuitHandle, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.circuit = &capturingCircuit{name: name, recv: make(chan transport.RawFrame)}
	return b.circuit, nil
}

type capturingCircuit struct {
	name string
	recv chan transport.RawFrame
	once sync.Once

	mu    sync.Mutex
	sent  [][]byte
	dests [][transport.MACLen]byte
}

func (c *capturingCircuit) IfIndex() int { return 7 }
func (c *capturingCircuit) HWAddr() [transport.MACLen]byte {
	return [transport.MACLen]byte{0x02, 0, 0, 0, 0, 1}
}
func (c *capturingCircuit) MTU() int { return 1500 }

func (c *capturingCircuit) Send(dst, _ [transport.MACLen]byte, pdu []byte) error {
	c.mu.Lock()
	c.sent = append(c.sent, append([]byte(nil), pdu...))
	c.dests = append(c.dests, dst)
	c.mu.Unlock()
	return nil
}

func (c *capturingCircuit) Recv() <-chan transport.RawFrame { return c.recv }

func (c *capturingCircuit) Close() error {
	c.once.Do(func() { close(c.recv) })
	return nil
}

// lastSent returns the most recent PDU and the multicast group it went to.
func (c *capturingCircuit) lastSent() ([]byte, [transport.MACLen]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.sent) == 0 {
		return nil, [transport.MACLen]byte{}, false
	}
	return c.sent[len(c.sent)-1], c.dests[len(c.dests)-1], true
}

// levelTimerConfig is one isis config subtree with a single broadcast L1L2
// circuit on eth0, carrying whatever per-interface leaves ifaceLeaves adds.
func levelTimerConfig(ifaceLeaves string) string {
	return `{"isis":{"net":"49.0001.0000.0000.0001.00","level":"l1-l2",` +
		`"interfaces":{"interface":{"eth0":{"level":"l1-l2","circuit-type":"broadcast"` +
		ifaceLeaves + `}}}}}`
}

// parseOneInterface resolves data and returns its single interface config.
func parseOneInterface(t *testing.T, data string) InterfaceConfig {
	t.Helper()
	cfg, err := parseISISConfig(sec(data))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("Interfaces = %d, want 1", len(cfg.Interfaces))
	}
	return cfg.Interfaces[0]
}

// TestISISLevelHelloTimersResolution: the per-level leaf replaces the
// circuit-wide value at its own level and leaves the other level alone; an unset
// leaf keeps the circuit-wide value; an unset circuit-wide leaf keeps the YANG
// default. Each case starts from the CONFIG TEXT, so the parser and the resolver
// are both under test.
func TestISISLevelHelloTimersResolution(t *testing.T) {
	cases := []struct {
		name        string
		ifaceLeaves string
		wantL1      circuit.LevelTimers
		wantL2      circuit.LevelTimers
	}{
		{
			name:        "no leaf set anywhere takes the YANG defaults",
			ifaceLeaves: ``,
			wantL1:      circuit.LevelTimers{HelloInterval: DefaultHelloInterval, HoldMult: DefaultHoldMultiplier},
			wantL2:      circuit.LevelTimers{HelloInterval: DefaultHelloInterval, HoldMult: DefaultHoldMultiplier},
		},
		{
			name:        "circuit-wide leaves govern both levels when no override is set",
			ifaceLeaves: `,"hello-interval":"5","hold-multiplier":"4"`,
			wantL1:      circuit.LevelTimers{HelloInterval: 5, HoldMult: 4},
			wantL2:      circuit.LevelTimers{HelloInterval: 5, HoldMult: 4},
		},
		{
			name: "a level-1 override replaces the circuit-wide pair at Level-1 only",
			ifaceLeaves: `,"hello-interval":"5","hold-multiplier":"4",` +
				`"level-1":{"hello-interval":"3","hold-multiplier":"6"}`,
			wantL1: circuit.LevelTimers{HelloInterval: 3, HoldMult: 6},
			wantL2: circuit.LevelTimers{HelloInterval: 5, HoldMult: 4},
		},
		{
			name: "each level takes its own override",
			ifaceLeaves: `,"level-1":{"hello-interval":"3","hold-multiplier":"6"},` +
				`"level-2":{"hello-interval":"30","hold-multiplier":"2"}`,
			wantL1: circuit.LevelTimers{HelloInterval: 3, HoldMult: 6},
			wantL2: circuit.LevelTimers{HelloInterval: 30, HoldMult: 2},
		},
		{
			name:        "a half override keeps the circuit-wide value for the leaf it omits",
			ifaceLeaves: `,"hello-interval":"5","hold-multiplier":"4","level-2":{"hello-interval":"30"}`,
			wantL1:      circuit.LevelTimers{HelloInterval: 5, HoldMult: 4},
			wantL2:      circuit.LevelTimers{HelloInterval: 30, HoldMult: 4},
		},
		{
			name:        "a level override with no circuit-wide leaf falls back to the YANG default",
			ifaceLeaves: `,"level-1":{"hello-interval":"1"}`,
			wantL1:      circuit.LevelTimers{HelloInterval: 1, HoldMult: DefaultHoldMultiplier},
			wantL2:      circuit.LevelTimers{HelloInterval: DefaultHelloInterval, HoldMult: DefaultHoldMultiplier},
		},
		{
			name:        "the range maxima resolve unchanged",
			ifaceLeaves: `,"level-1":{"hello-interval":"65535","hold-multiplier":"255"}`,
			wantL1:      circuit.LevelTimers{HelloInterval: 65535, HoldMult: 255},
			wantL2:      circuit.LevelTimers{HelloInterval: DefaultHelloInterval, HoldMult: DefaultHoldMultiplier},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ic := parseOneInterface(t, levelTimerConfig(tc.ifaceLeaves))
			if got := levelHelloTimers(ic, adjacency.Level1); got != tc.wantL1 {
				t.Errorf("Level-1 timers = %+v, want %+v", got, tc.wantL1)
			}
			if got := levelHelloTimers(ic, adjacency.Level2); got != tc.wantL2 {
				t.Errorf("Level-2 timers = %+v, want %+v", got, tc.wantL2)
			}
		})
	}
}

// TestISISPerLevelHelloTimersReachTheWire is the wiring test: a config that sets
// the LEVEL-NESTED leaves reaches the circuit the engine builds, which then
// publishes one Hello schedule per level at that level's period, and puts that
// level's holding time in the IIH it sends to that level's multicast group.
func TestISISPerLevelHelloTimersReachTheWire(t *testing.T) {
	cfg, err := parseISISConfig(sec(levelTimerConfig(
		`,"hello-interval":"10","hold-multiplier":"3",` +
			`"level-1":{"hello-interval":"3","hold-multiplier":"3"},` +
			`"level-2":{"hello-interval":"30","hold-multiplier":"2"}`)))
	if err != nil {
		t.Fatalf("parseISISConfig: %v", err)
	}
	fb := &capturingBackend{}
	eng := newEngine(transport.New(fb))
	eng.setConfig(cfg)
	if err := eng.openCircuits(); err != nil {
		t.Fatalf("openCircuits: %v", err)
	}
	defer eng.shutdown()

	c := eng.buildCircuit(cfg.Interfaces[0])
	if c == nil {
		t.Fatal("buildCircuit returned nil for the configured eth0 circuit")
	}

	// One schedule per level, each at the period its own override asked for. The
	// circuit-wide 10s governs neither level now that both carry an override.
	want := []circuit.HelloSchedule{
		{Level: adjacency.Level1, Period: 3 * time.Second},
		{Level: adjacency.Level2, Period: 30 * time.Second},
	}
	got := c.HelloSchedules()
	if len(got) != len(want) {
		t.Fatalf("HelloSchedules() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("HelloSchedules()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Each schedule's IIH carries its own holding time and goes to its own group.
	holds := map[adjacency.Level]uint16{adjacency.Level1: 9, adjacency.Level2: 60}
	for _, s := range got {
		if err := c.SendHello(s.Level); err != nil {
			t.Fatalf("SendHello(%v): %v", s.Level, err)
		}
		pdu, dst, ok := fb.circuit.lastSent()
		if !ok {
			t.Fatalf("SendHello(%v) sent nothing", s.Level)
		}
		p, err := packet.DecodePDU(pdu)
		if err != nil {
			t.Fatalf("DecodePDU after SendHello(%v): %v", s.Level, err)
		}
		if p.LANHello == nil {
			t.Fatalf("SendHello(%v) sent %v, want a LAN IIH", s.Level, p.Header.PDUType)
		}
		if hold := uint16(p.LANHello.HoldingTime); hold != holds[s.Level] {
			t.Errorf("%v IIH holding time = %d, want %d", s.Level, hold, holds[s.Level])
		}
		wantMAC, _ := transport.MulticastMACForLevel(transportLevel(s.Level))
		if dst != wantMAC {
			t.Errorf("%v IIH went to %v, want %v", s.Level, dst, wantMAC)
		}
	}
}

// transportLevel maps an adjacency level onto the transport level that selects
// the multicast group, so the assertion above names the group the IIH is owed.
func transportLevel(l adjacency.Level) transport.Level {
	if l == adjacency.Level2 {
		return transport.Level2
	}
	return transport.Level1
}

// TestISISLevelTimerChangeIsNotUnchanged: reconcile must not read a config whose
// only edit is a per-level Hello timer as identical to the running one. The
// per-level containers now select what the circuit sends, so a reconcile that
// called them equal would leave the operator's committed change invisible.
func TestISISLevelTimerChangeIsNotUnchanged(t *testing.T) {
	before := parseOneInterface(t, levelTimerConfig(`,"level-1":{"hello-interval":"3"}`))
	after := parseOneInterface(t, levelTimerConfig(`,"level-1":{"hello-interval":"5"}`))

	if circuitParamsEqual(before, after) {
		t.Error("circuitParamsEqual called a level-1 hello-interval change unchanged")
	}
	if !circuitParamsEqual(before, before) {
		t.Error("circuitParamsEqual called an unedited config changed")
	}
}
