// Design: docs/architecture/isis/isis-5-adjacency.md -- per-level Hello timers.
//
// VALIDATES: a circuit runs one Hello timer for each level it forms, so the
// Level-1 and the Level-2 LAN IIH leave at periods of their own and each one
// carries the holding time of its own level (spec-isis-per-level-hello-timers
// AC-2 and AC-3). A point-to-point circuit publishes exactly ONE schedule,
// because its single IIH serves both levels, and the holding time it advertises
// is the one of the level that schedule runs at.
// PREVENTS: the regression this spec closes -- one circuit-wide Hello timer
// driving both levels, so a Level-1 override changes neither the period nor the
// holding time of the Level-1 IIH, and a Level-2 IIH advertises a holding time
// computed from the Level-1 period it does not go out at.

package circuit

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// levelTimerCircuit builds a circuit of kind forming levels, with the Hello
// timers l1 and l2. Nothing else varies across the tests below.
func levelTimerCircuit(t *testing.T, s Sender, kind adjacency.CircuitKind, levels []adjacency.Level, l1, l2 LevelTimers) *Circuit {
	t.Helper()
	return New(Config{
		Name:     "eth0",
		IfIndex:  3,
		SystemID: types.SystemID{0, 0, 0, 0, 0, 1},
		SNPA:     adjacency.SNPA{0x02, 0, 0, 0, 0, 1},
		Areas:    []types.AreaID{testArea(t)},
		IPv4:     netip.MustParseAddr("192.0.2.1"),
		Kind:     kind,
		Levels:   levels,
		Level1:   l1,
		Level2:   l2,
		Priority: 64,
	}, s, nil)
}

// lanHoldingTime sends the IIH for level and returns the PDU type and holding
// time the circuit put on the wire.
func lanHoldingTime(t *testing.T, c *Circuit, s *fakeSender, level adjacency.Level) (packet.PDUType, uint16) {
	t.Helper()
	if err := c.SendHello(level); err != nil {
		t.Fatalf("SendHello(%v): %v", level, err)
	}
	p := decodeSent(t, s)
	if p.LANHello == nil {
		t.Fatalf("SendHello(%v) sent %v, want a LAN IIH", level, p.Header.PDUType)
	}
	return p.LANHello.PDUType, uint16(p.LANHello.HoldingTime)
}

// TestISISPerLevelHelloSchedulesOnBroadcast: a broadcast circuit forming both
// levels publishes one schedule per level, each at that level's own period.
func TestISISPerLevelHelloSchedulesOnBroadcast(t *testing.T) {
	c := levelTimerCircuit(t, &fakeSender{mtu: 1500}, adjacency.KindBroadcast,
		[]adjacency.Level{adjacency.Level1, adjacency.Level2},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	got := c.HelloSchedules()
	want := []HelloSchedule{
		{Level: adjacency.Level1, Period: 3 * time.Second},
		{Level: adjacency.Level2, Period: 30 * time.Second},
	}
	if len(got) != len(want) {
		t.Fatalf("HelloSchedules() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("HelloSchedules()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestISISPerLevelHelloScheduleSingleLevel: a circuit forming one level
// publishes one schedule, and it is that level's period -- not the other
// level's, which the operator may have left at the circuit-wide value.
func TestISISPerLevelHelloScheduleSingleLevel(t *testing.T) {
	c := levelTimerCircuit(t, &fakeSender{mtu: 1500}, adjacency.KindBroadcast,
		[]adjacency.Level{adjacency.Level2},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	got := c.HelloSchedules()
	want := HelloSchedule{Level: adjacency.Level2, Period: 30 * time.Second}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("HelloSchedules() = %+v, want [%+v]", got, want)
	}
}

// TestISISPerLevelHoldingTimeInLANIIH: each LAN IIH advertises the holding time
// of ITS OWN level (ISO/IEC 10589 section 8.2: hello interval * hold
// multiplier), so a neighbor at one level is never told a holding time derived
// from the period the other level runs at.
func TestISISPerLevelHoldingTimeInLANIIH(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := levelTimerCircuit(t, s, adjacency.KindBroadcast,
		[]adjacency.Level{adjacency.Level1, adjacency.Level2},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	if pt, hold := lanHoldingTime(t, c, s, adjacency.Level1); pt != packet.PDUTypeL1LANHello || hold != 9 {
		t.Errorf("L1 IIH = type %v hold %d, want type %v hold 9", pt, hold, packet.PDUTypeL1LANHello)
	}
	if pt, hold := lanHoldingTime(t, c, s, adjacency.Level2); pt != packet.PDUTypeL2LANHello || hold != 60 {
		t.Errorf("L2 IIH = type %v hold %d, want type %v hold 60", pt, hold, packet.PDUTypeL2LANHello)
	}
}

// TestISISSendHelloRefusesUnformedLevel: a level the circuit does not form is an
// error and sends nothing. A silent no-op would hide a caller that reached a
// schedule the circuit never published; a silent send would put a level on the
// wire the operator did not configure.
func TestISISSendHelloRefusesUnformedLevel(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := levelTimerCircuit(t, s, adjacency.KindBroadcast,
		[]adjacency.Level{adjacency.Level1},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	if err := c.SendHello(adjacency.Level2); err == nil {
		t.Error("SendHello(Level2) on an L1-only circuit returned nil, want an error")
	}
	if len(s.sent) != 0 {
		t.Errorf("SendHello(Level2) on an L1-only circuit sent %d PDUs, want 0", len(s.sent))
	}
}

// TestISISP2PRunsOneHelloSchedule: a point-to-point circuit sends ONE IIH that
// serves both levels (RFC 5303 section 3: the P2P IIH carries no level bit), so
// it runs one Hello timer and the holding time it advertises is the one of the
// level that timer runs at. Two periods would have nothing to apply to.
func TestISISP2PRunsOneHelloSchedule(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := levelTimerCircuit(t, s, adjacency.KindP2P,
		[]adjacency.Level{adjacency.Level1, adjacency.Level2},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	got := c.HelloSchedules()
	want := HelloSchedule{Level: adjacency.Level1, Period: 3 * time.Second}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("P2P HelloSchedules() = %+v, want [%+v]", got, want)
	}

	if err := c.SendHello(got[0].Level); err != nil {
		t.Fatalf("SendHello: %v", err)
	}
	p := decodeSent(t, s)
	if p.P2PHello == nil {
		t.Fatalf("P2P circuit sent %v, want a P2P IIH", p.Header.PDUType)
	}
	if hold := uint16(p.P2PHello.HoldingTime); hold != 9 {
		t.Errorf("P2P IIH holding time = %d, want 9 (the period the IIH goes out at)", hold)
	}
}

// TestISISP2PL2OnlyTakesItsOwnTimers: an L2-only point-to-point circuit runs its
// one timer at the Level-2 period, not at the Level-1 pair the operator left
// unused.
func TestISISP2PL2OnlyTakesItsOwnTimers(t *testing.T) {
	s := &fakeSender{mtu: 1500}
	c := levelTimerCircuit(t, s, adjacency.KindP2P,
		[]adjacency.Level{adjacency.Level2},
		LevelTimers{HelloInterval: 3, HoldMult: 3},
		LevelTimers{HelloInterval: 30, HoldMult: 2})

	got := c.HelloSchedules()
	want := HelloSchedule{Level: adjacency.Level2, Period: 30 * time.Second}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("L2-only P2P HelloSchedules() = %+v, want [%+v]", got, want)
	}
	if err := c.SendHello(got[0].Level); err != nil {
		t.Fatalf("SendHello: %v", err)
	}
	p := decodeSent(t, s)
	if p.P2PHello == nil {
		t.Fatalf("P2P circuit sent %v, want a P2P IIH", p.Header.PDUType)
	}
	if hold := uint16(p.P2PHello.HoldingTime); hold != 60 {
		t.Errorf("L2-only P2P IIH holding time = %d, want 60", hold)
	}
}

// TestISISHelloScheduleNeverZeroPeriod: a Config that resolved no timers still
// yields a positive period. time.NewTicker panics on a non-positive period, and
// the engine builds its tickers straight from these schedules, so a zero here
// takes the daemon down rather than mis-timing one circuit.
func TestISISHelloScheduleNeverZeroPeriod(t *testing.T) {
	c := levelTimerCircuit(t, &fakeSender{mtu: 1500}, adjacency.KindBroadcast,
		[]adjacency.Level{adjacency.Level1, adjacency.Level2},
		LevelTimers{}, LevelTimers{})

	for _, s := range c.HelloSchedules() {
		if s.Period <= 0 {
			t.Errorf("schedule %+v has a non-positive period", s)
		}
	}
}
