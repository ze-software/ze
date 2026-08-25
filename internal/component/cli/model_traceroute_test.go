package cli

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func TestIsTracerouteMonitorCommand(t *testing.T) {
	assert.True(t, isTracerouteMonitorCommand("monitor traceroute 8.8.8.8"))
	assert.True(t, isTracerouteMonitorCommand("  monitor traceroute 8.8.8.8  "))
	assert.True(t, isTracerouteMonitorCommand("monitor traceroute 8.8.8.8 max-hops 10"))
	assert.False(t, isTracerouteMonitorCommand("monitor bgp"))
	assert.False(t, isTracerouteMonitorCommand("traceroute 8.8.8.8"))
	assert.False(t, isTracerouteMonitorCommand("monitor traceroute"))
	assert.False(t, isTracerouteMonitorCommand("monitor traceroute "))
}

func TestParseTracerouteMonitorArgs(t *testing.T) {
	target, maxHops, errMsg := parseTracerouteMonitorArgs("monitor traceroute 8.8.8.8")
	assert.Equal(t, "8.8.8.8", target)
	assert.Equal(t, 16, maxHops)
	assert.Empty(t, errMsg)
}

func TestParseTracerouteMonitorArgs_AllOptions(t *testing.T) {
	target, maxHops, errMsg := parseTracerouteMonitorArgs("monitor traceroute 10.0.0.1 max-hops 10")
	assert.Equal(t, "10.0.0.1", target)
	assert.Equal(t, 10, maxHops)
	assert.Empty(t, errMsg)
}

func TestParseTracerouteMonitorArgs_InvalidMaxHops(t *testing.T) {
	_, maxHops, errMsg := parseTracerouteMonitorArgs("monitor traceroute 8.8.8.8 max-hops 100")
	assert.Equal(t, 16, maxHops)
	assert.Empty(t, errMsg)
}

func TestParseTracerouteMonitorArgs_KeywordsOnly(t *testing.T) {
	target, _, errMsg := parseTracerouteMonitorArgs("monitor traceroute max-hops 10")
	assert.Equal(t, "", target)
	assert.Empty(t, errMsg)
}

func TestParseTracerouteMonitorArgs_UnexpectedArg(t *testing.T) {
	_, _, errMsg := parseTracerouteMonitorArgs("monitor traceroute 1.1.1.1 resolve")
	assert.Contains(t, errMsg, "unexpected argument: resolve")
	assert.Contains(t, errMsg, "| for pipe operators")
}

func TestParsePositiveInt(t *testing.T) {
	assert.Equal(t, 42, parsePositiveInt("42"))
	assert.Equal(t, 0, parsePositiveInt("abc"))
	assert.Equal(t, 0, parsePositiveInt(""))
	assert.Equal(t, 10, parsePositiveInt("10"))
}

func TestPathStats_Loss(t *testing.T) {
	p := traceroutePathStats{sent: 10, recv: 8}
	assert.InDelta(t, 20.0, p.loss(), 0.01)

	p2 := traceroutePathStats{sent: 10, recv: 10}
	assert.InDelta(t, 0.0, p2.loss(), 0.01)

	p3 := traceroutePathStats{sent: 0, recv: 0}
	assert.InDelta(t, 0.0, p3.loss(), 0.01)
}

func TestPathStats_Avg(t *testing.T) {
	p := traceroutePathStats{recv: 3, sum: 30.0}
	assert.InDelta(t, 10.0, p.avg(), 0.01)

	p2 := traceroutePathStats{recv: 0, sum: 0}
	assert.InDelta(t, 0.0, p2.avg(), 0.01)
}

func TestPathStats_Stddev(t *testing.T) {
	p := traceroutePathStats{recv: 1}
	assert.InDelta(t, 0.0, p.stddev(), 0.01)

	p2 := traceroutePathStats{
		recv:  2,
		sum:   30.0,
		sumSq: 500.0,
	}
	assert.InDelta(t, math.Sqrt(50), p2.stddev(), 0.01)
}

func testFactory(hops []map[string]any) TracerouteFactory {
	return func(_ context.Context, _ string, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		ch := make(chan map[string]any, len(hops)+1)
		for _, h := range hops {
			ch <- h
		}
		close(ch)
		_, cancel := context.WithCancel(context.Background())
		return ch, cancel, nil
	}
}

// feedRounds feeds multiple rounds of hop data into a tracerouteState
// and returns the final state. Each round is a slice of hop maps.
func feedRounds(t *testing.T, target string, maxHops int, rounds [][]map[string]any) *tracerouteState { //nolint:unparam // maxHops varies in future tests
	t.Helper()
	ts := &tracerouteState{
		target:  target,
		maxHops: maxHops,
	}
	for _, round := range rounds {
		for _, hop := range round {
			applyHop(ts, hop)
		}
		ts.rounds++
		ts.lastPollTime = time.Now()
	}
	return ts
}

// assertPath checks a single path's stats at a given TTL index.
func assertPath(t *testing.T, ts *tracerouteState, ttlIdx, pathIdx int, addr string, sent, recv int) {
	t.Helper()
	require.Greater(t, len(ts.hops), ttlIdx, "TTL %d not present", ttlIdx+1)
	require.Greater(t, len(ts.hops[ttlIdx].paths), pathIdx, "path %d at TTL %d not present", pathIdx, ttlIdx+1)
	p := &ts.hops[ttlIdx].paths[pathIdx]
	assert.Equal(t, addr, p.addr, "TTL %d path %d addr", ttlIdx+1, pathIdx)
	assert.Equal(t, sent, p.sent, "TTL %d path %d sent", ttlIdx+1, pathIdx)
	assert.Equal(t, recv, p.recv, "TTL %d path %d recv", ttlIdx+1, pathIdx)
}

// assertRender checks that the rendered output contains all expected substrings.
func assertRender(t *testing.T, ts *tracerouteState, expected []string) {
	t.Helper()
	m := NewCommandModel()
	m.width = 120
	m.activeView = &tracerouteView{st: ts}
	output := m.renderTraceroute()
	for _, s := range expected {
		assert.Contains(t, output, s, "render missing: %q", s)
	}
}

// --- Scenario: single path, no ECMP ---

func TestScenario_SinglePath(t *testing.T) {
	ts := feedRounds(t, "1.1.1.1", 16, [][]map[string]any{
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.0},
			{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 2.0},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.0},
		},
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.5},
			{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 2.5},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.5},
		},
	})

	assert.Equal(t, 2, ts.rounds)
	require.Len(t, ts.hops, 3)

	assertPath(t, ts, 0, 0, "10.0.0.1", 2, 2)
	assertPath(t, ts, 1, 0, "10.0.0.2", 2, 2)
	assertPath(t, ts, 2, 0, "1.1.1.1", 2, 2)

	assert.InDelta(t, 1.25, ts.hops[0].paths[0].avg(), 0.01)
	assert.InDelta(t, 1.0, ts.hops[0].paths[0].best, 0.01)
	assert.InDelta(t, 1.5, ts.hops[0].paths[0].worst, 0.01)

	assertRender(t, ts, []string{
		"10.0.0.1", "10.0.0.2", "1.1.1.1",
		"rounds 2",
	})
}

// --- Scenario: ECMP at hop 2, two paths ---

func TestScenario_ECMP_TwoPaths(t *testing.T) {
	ts := feedRounds(t, "1.1.1.1", 16, [][]map[string]any{
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.0},
			{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 2.0},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.0},
		},
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.2},
			{"ttl": 2, "addr": "10.0.0.3", "rtt-ms": 2.5},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.2},
		},
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.1},
			{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 2.1},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.1},
		},
	})

	assert.Equal(t, 3, ts.rounds)
	require.Len(t, ts.hops, 3)

	// Hop 1: single path
	require.Len(t, ts.hops[0].paths, 1)
	assertPath(t, ts, 0, 0, "10.0.0.1", 3, 3)

	// Hop 2: two paths (ECMP)
	require.Len(t, ts.hops[1].paths, 2)
	assertPath(t, ts, 1, 0, "10.0.0.2", 2, 2)
	assertPath(t, ts, 1, 1, "10.0.0.3", 1, 1)

	// Hop 3: single path (destination)
	require.Len(t, ts.hops[2].paths, 1)
	assertPath(t, ts, 2, 0, "1.1.1.1", 3, 3)

	// Render must show both IPs at hop 2
	assertRender(t, ts, []string{
		"10.0.0.1",
		"10.0.0.2",
		"10.0.0.3",
		"1.1.1.1",
	})
}

// --- Scenario: ECMP with three paths ---

func TestScenario_ECMP_ThreePaths(t *testing.T) {
	ts := feedRounds(t, "8.8.8.8", 16, [][]map[string]any{
		{{"ttl": 2, "addr": "A", "rtt-ms": 1.0}},
		{{"ttl": 2, "addr": "B", "rtt-ms": 2.0}},
		{{"ttl": 2, "addr": "C", "rtt-ms": 3.0}},
		{{"ttl": 2, "addr": "A", "rtt-ms": 1.5}},
		{{"ttl": 2, "addr": "B", "rtt-ms": 2.5}},
	})

	require.Len(t, ts.hops[1].paths, 3)
	assertPath(t, ts, 1, 0, "A", 2, 2)
	assertPath(t, ts, 1, 1, "B", 2, 2)
	assertPath(t, ts, 1, 2, "C", 1, 1)
}

// --- Scenario: packet loss at one hop ---

func TestScenario_PacketLoss(t *testing.T) {
	ts := feedRounds(t, "1.1.1.1", 16, [][]map[string]any{
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.0},
			{"ttl": 2, "addr": "*", "rtt-ms": nil},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.0},
		},
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.2},
			{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 2.0},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.2},
		},
		{
			{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.1},
			{"ttl": 2, "addr": "*", "rtt-ms": nil},
			{"ttl": 3, "addr": "1.1.1.1", "rtt-ms": 3.1},
		},
	})

	// Hop 2: "*" was absorbed into "10.0.0.2" when the real IP appeared.
	// Total: 3 sent (2 from "*" + 1 from "10.0.0.2"), 1 recv.
	require.Len(t, ts.hops[1].paths, 1)
	assertPath(t, ts, 1, 0, "10.0.0.2", 3, 1)
	assert.InDelta(t, 66.7, ts.hops[1].paths[0].loss(), 0.1)
}

// --- Scenario: ECMP with loss on one path ---

func TestScenario_ECMP_WithLoss(t *testing.T) {
	ts := feedRounds(t, "1.1.1.1", 16, [][]map[string]any{
		{{"ttl": 2, "addr": "A", "rtt-ms": 1.0}},
		{{"ttl": 2, "addr": "B", "rtt-ms": nil}},
		{{"ttl": 2, "addr": "A", "rtt-ms": 1.5}},
		{{"ttl": 2, "addr": "B", "rtt-ms": 2.0}},
	})

	require.Len(t, ts.hops[1].paths, 2)
	assertPath(t, ts, 1, 0, "A", 2, 2)
	assertPath(t, ts, 1, 1, "B", 2, 1)

	assert.InDelta(t, 0.0, ts.hops[1].paths[0].loss(), 0.01)
	assert.InDelta(t, 50.0, ts.hops[1].paths[1].loss(), 0.01)
}

// --- Scenario: render shows hop number only on first path ---

func TestRender_HopNumberOnFirstPathOnly(t *testing.T) {
	ts := &tracerouteState{
		target:  "1.1.1.1",
		maxHops: 16,
		rounds:  3,
		hops: []tracerouteHop{
			{paths: []traceroutePathStats{
				{addr: "10.0.0.1", sent: 3, recv: 3, last: 1.0, best: 0.8, worst: 1.2, sum: 3.0, sumSq: 3.08},
			}},
			{paths: []traceroutePathStats{
				{addr: "10.0.0.2", sent: 2, recv: 2, last: 2.0, best: 1.8, worst: 2.0, sum: 3.8, sumSq: 7.24},
				{addr: "10.0.0.3", sent: 1, recv: 1, last: 2.5, best: 2.5, worst: 2.5, sum: 2.5, sumSq: 6.25},
			}},
			{paths: []traceroutePathStats{
				{addr: "1.1.1.1", sent: 3, recv: 3, last: 3.0, best: 2.8, worst: 3.2, sum: 9.0, sumSq: 27.08},
			}},
		},
		lastPollTime: time.Now(),
	}

	m := NewCommandModel()
	m.width = 120
	m.activeView = &tracerouteView{st: ts}
	output := m.renderTraceroute()

	lines := strings.Split(output, "\n")

	// Find lines containing our addresses
	var hop2Lines []string
	for _, line := range lines {
		if strings.Contains(line, "10.0.0.2") || strings.Contains(line, "10.0.0.3") {
			hop2Lines = append(hop2Lines, line)
		}
	}

	require.Len(t, hop2Lines, 2)
	assert.Contains(t, hop2Lines[0], "  2 ")
	assert.True(t, strings.HasPrefix(strings.TrimLeft(hop2Lines[1], " "), "10.0.0.3"),
		"second path line should not have hop number, got: %s", hop2Lines[1])
}

// --- Helpers test ---

func TestFindOrCreate(t *testing.T) {
	h := tracerouteHop{}
	p1 := h.findOrCreate("10.0.0.1")
	assert.Equal(t, "10.0.0.1", p1.addr)
	assert.Equal(t, math.MaxFloat64, p1.best)

	p2 := h.findOrCreate("10.0.0.2")
	assert.Equal(t, "10.0.0.2", p2.addr)

	p1again := h.findOrCreate("10.0.0.1")
	assert.Equal(t, "10.0.0.1", p1again.addr)
	require.Len(t, h.paths, 2)
}

func TestApplyHop(t *testing.T) {
	ts := &tracerouteState{maxHops: 16}

	applyHop(ts, map[string]any{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.5})
	require.Len(t, ts.hops, 1)
	require.Len(t, ts.hops[0].paths, 1)
	assert.Equal(t, "10.0.0.1", ts.hops[0].paths[0].addr)
	assert.Equal(t, 1, ts.hops[0].paths[0].sent)
	assert.Equal(t, 1, ts.hops[0].paths[0].recv)
	assert.InDelta(t, 1.5, ts.hops[0].paths[0].last, 0.01)

	applyHop(ts, map[string]any{"ttl": 1, "addr": "10.0.0.2", "rtt-ms": 2.0})
	require.Len(t, ts.hops[0].paths, 2)
	assert.Equal(t, "10.0.0.2", ts.hops[0].paths[1].addr)
}

// --- Poll and lifecycle tests ---

func TestHandleTraceroutePoll(t *testing.T) {
	ch := make(chan map[string]any, 4)
	ch <- map[string]any{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.5}
	ch <- map[string]any{"ttl": 2, "addr": "10.0.0.2", "rtt-ms": 5.0}
	ch <- map[string]any{"ttl": 3, "addr": "*", "rtt-ms": nil}
	close(ch)

	m := NewCommandModel()
	m.activeView = &tracerouteView{st: &tracerouteState{
		target:  "8.8.8.8",
		maxHops: 16,
		hopChan: ch,
		poller:  testFactory(nil),
	}}

	result, cmd := m.handleTraceroutePoll()
	model, _ := result.(Model) //nolint:errcheck // test assertion follows
	require.NotNil(t, cmd)

	ts := model.activeTraceroute()
	require.NotNil(t, ts)
	assert.Equal(t, 1, ts.rounds)
	require.Len(t, ts.hops, 3)
	assertPath(t, ts, 0, 0, "10.0.0.1", 1, 1)
	assertPath(t, ts, 1, 0, "10.0.0.2", 1, 1)
	assertPath(t, ts, 2, 0, "*", 1, 0)
}

func TestHandleTraceroutePoll_Partial(t *testing.T) {
	ch := make(chan map[string]any, 3)
	ch <- map[string]any{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 2.0}

	m := NewCommandModel()
	m.activeView = &tracerouteView{st: &tracerouteState{
		target:  "8.8.8.8",
		maxHops: 16,
		hopChan: ch,
		poller:  testFactory(nil),
	}}

	result, cmd := m.handleTraceroutePoll()
	model, _ := result.(Model) //nolint:errcheck // test assertion follows
	require.NotNil(t, cmd)

	ts := model.activeTraceroute()
	assert.Equal(t, 0, ts.rounds)
	require.Len(t, ts.hops, 1)
	assertPath(t, ts, 0, 0, "10.0.0.1", 1, 1)
}

func TestHandleTraceroutePoll_MultiRound(t *testing.T) {
	roundCount := 0
	factory := func(_ context.Context, _ string, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		roundCount++
		ch := make(chan map[string]any, 2)
		ch <- map[string]any{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": float64(roundCount)}
		close(ch)
		_, cancel := context.WithCancel(context.Background())
		return ch, cancel, nil
	}

	m := NewCommandModel()
	m.setTracerouteFactory(factory)
	cmd := m.startTraceroute("monitor traceroute 8.8.8.8")
	require.NotNil(t, cmd)

	// startTraceroute already called startTracerouteRound (round 1 factory call).
	assert.Equal(t, 1, roundCount)

	// Drain round 1: channel closes, startTracerouteRound chains round 2.
	result, cmd := m.handleTraceroutePoll()
	m, _ = result.(Model) //nolint:errcheck // test assertion follows
	require.NotNil(t, cmd, "round 1 close should chain to next round")
	assert.Equal(t, 1, m.activeTraceroute().rounds)
	assert.Equal(t, 2, roundCount)

	// Drain round 2: channel closes, startTracerouteRound chains round 3.
	result, cmd = m.handleTraceroutePoll()
	m, _ = result.(Model) //nolint:errcheck // test assertion follows
	require.NotNil(t, cmd, "round 2 close should chain to next round")
	assert.Equal(t, 2, m.activeTraceroute().rounds)
	assert.Equal(t, 3, roundCount)

	assertPath(t, m.activeTraceroute(), 0, 0, "10.0.0.1", 2, 2)
}

func TestHandleTraceroutePoll_NilSession(t *testing.T) {
	m := NewCommandModel()
	_, cmd := m.handleTraceroutePoll()
	assert.Nil(t, cmd)
}

func TestRenderTraceroute_Empty(t *testing.T) {
	m := NewCommandModel()
	m.width = 80
	m.activeView = &tracerouteView{st: &tracerouteState{target: "8.8.8.8", rounds: 0, maxHops: 30}}
	output := m.renderTraceroute()
	assert.Contains(t, output, "waiting for data...")
}

func TestRenderTraceroute_Nil(t *testing.T) {
	m := NewCommandModel()
	assert.Equal(t, "", m.renderTraceroute())
}

func TestFormatFloat1(t *testing.T) {
	assert.Equal(t, "1.5", formatFloat1(1.5))
	assert.Equal(t, "0.0", formatFloat1(0.0))
	assert.Equal(t, "100.3", formatFloat1(100.3))
}

func padL(s string, w int) string { var b textbuf.Buffer; tbPadLeft(&b, s, w); return b.String() }
func padR(s string, w int) string { var b textbuf.Buffer; tbPadRight(&b, s, w); return b.String() }

func TestWritePadLeft(t *testing.T) {
	assert.Equal(t, "  hi", padL("hi", 4))
	assert.Equal(t, "hello", padL("hello", 3))
	assert.Equal(t, "hi", padL("hi", 2))
}

func TestWritePadRight(t *testing.T) {
	assert.Equal(t, "hi  ", padR("hi", 4))
	assert.Equal(t, "hello", padR("hello", 3))
	assert.Equal(t, "hi", padR("hi", 2))
}

func TestHandleTracerouteKey(t *testing.T) {
	m := NewCommandModel()
	m.activeView = &tracerouteView{st: &tracerouteState{target: "8.8.8.8"}}
	assert.True(t, m.handleTracerouteKey("q"))
	assert.Nil(t, m.activeTraceroute())
	assert.Equal(t, "traceroute stopped", m.statusMessage)
}

func TestHandleTracerouteKey_Esc(t *testing.T) {
	m := NewCommandModel()
	m.activeView = &tracerouteView{st: &tracerouteState{target: "8.8.8.8"}}
	assert.True(t, m.handleTracerouteKey("esc"))
	assert.Nil(t, m.activeTraceroute())
}

func TestHandleTracerouteKey_NilSession(t *testing.T) {
	m := NewCommandModel()
	assert.False(t, m.handleTracerouteKey("q"))
}

func TestStartTraceroute_NoFactory(t *testing.T) {
	m := NewCommandModel()
	cmd := m.startTraceroute("monitor traceroute 8.8.8.8")
	assert.Nil(t, cmd)
	assert.Equal(t, "traceroute not available (no daemon connection)", m.statusMessage)
}

func TestStartTraceroute_NoTarget(t *testing.T) {
	m := NewCommandModel()
	m.setTracerouteFactory(testFactory(nil))
	cmd := m.startTraceroute("monitor traceroute max-hops 10")
	assert.Nil(t, cmd)
	assert.Equal(t, "monitor traceroute: missing target address", m.statusMessage)
}

func TestStartTraceroute_OK(t *testing.T) {
	m := NewCommandModel()
	m.setTracerouteFactory(testFactory([]map[string]any{{"ttl": 1, "addr": "10.0.0.1", "rtt-ms": 1.0}}))
	cmd := m.startTraceroute("monitor traceroute 8.8.8.8")
	assert.NotNil(t, cmd)
	assert.NotNil(t, m.activeTraceroute())
	assert.Equal(t, "8.8.8.8", m.activeTraceroute().target)
}

func TestDrainTracerouteHops(t *testing.T) {
	ch := make(chan map[string]any, 3)
	ch <- map[string]any{"ttl": 1}
	ch <- map[string]any{"ttl": 2}

	hops, closed := drainTracerouteHops(ch)
	assert.False(t, closed)
	assert.Len(t, hops, 2)

	close(ch)
	hops2, closed2 := drainTracerouteHops(ch)
	assert.True(t, closed2)
	assert.Empty(t, hops2)
}

func TestHopInt(t *testing.T) {
	assert.Equal(t, 5, hopInt(5))
	assert.Equal(t, 5, hopInt(float64(5)))
	assert.Equal(t, 0, hopInt("five"))
	assert.Equal(t, 0, hopInt(nil))
}

func TestHopFloat(t *testing.T) {
	v, ok := hopFloat(1.5)
	assert.True(t, ok)
	assert.InDelta(t, 1.5, v, 0.01)

	v2, ok2 := hopFloat(3)
	assert.True(t, ok2)
	assert.InDelta(t, 3.0, v2, 0.01)

	_, ok3 := hopFloat(nil)
	assert.False(t, ok3)
}

func TestFormatTracerouteLogHeader(t *testing.T) {
	hops := []tracerouteHop{
		{paths: []traceroutePathStats{{addr: "10.0.0.1"}}},
		{paths: []traceroutePathStats{{addr: "10.0.0.2"}}},
		{paths: []traceroutePathStats{{addr: "192.0.2.1"}}},
	}
	hdr := formatTracerouteLogHeader(hops)
	assert.Contains(t, hdr, "Rnd")
	assert.Contains(t, hdr, "1")
	assert.Contains(t, hdr, "2")
	assert.Contains(t, hdr, "3")
}

func TestFormatTracerouteLogMap(t *testing.T) {
	hops := []tracerouteHop{
		{paths: []traceroutePathStats{{addr: "10.0.0.1"}}},
		{},
		{paths: []traceroutePathStats{{addr: "192.0.2.1"}}},
	}
	m := formatTracerouteLogMap(hops, false, false)
	assert.Contains(t, m, "1: 10.0.0.1")
	assert.Contains(t, m, "2:        *")
	assert.Contains(t, m, "3:192.0.2.1")
	assert.NotContains(t, m, "hop ")
}

func TestFormatTracerouteLogLine(t *testing.T) {
	hops := []tracerouteHop{
		{paths: []traceroutePathStats{{addr: "10.0.0.1", recv: 1, last: 0.5}}},
		{paths: []traceroutePathStats{{addr: "10.0.0.2", recv: 1, last: 1.2}}},
		{paths: []traceroutePathStats{{addr: "192.0.2.1", recv: 0}}},
	}
	line := formatTracerouteLogLine(hops, 3)
	assert.Contains(t, line, "3")
	assert.Contains(t, line, "0.5ms")
	assert.Contains(t, line, "1.2ms")
	assert.Contains(t, line, "*")
}

func TestFormatTracerouteLogLineEmpty(t *testing.T) {
	hops := []tracerouteHop{{}, {}}
	line := formatTracerouteLogLine(hops, 1)
	assert.Contains(t, line, "1")
	assert.Equal(t, 2, strings.Count(line, "*"))
}

// TestTracerouteViewAnswersItsFaultRatherThanRenderingIt verifies a poll
// failure reaches the Model through the view interface instead of the
// traceroute output.
//
// VALIDATES: tracerouteView.problem answers the last poll fault, and neither
// the header nor the body carries it. The Model renders every view's fault in
// one error zone (docs/architecture/cli/error-surface.md).
//
// PREVENTS: the same failure printed twice in one frame. The header appended
// the fault to "Traceroute to X rounds N", and the no-hops branch rendered the
// same string again as "error: ...". A reader saw one failure as two, and
// neither copy was where the faults of any other command appear.
func TestTracerouteViewAnswersItsFaultRatherThanRenderingIt(t *testing.T) {
	m := NewCommandModel()
	m.width = 120
	view := &tracerouteView{st: &tracerouteState{
		target:    "192.0.2.1",
		rounds:    2,
		pollError: "connection lost",
	}}
	m.activeView = view

	if got := view.problem(&m); got != "connection lost" {
		t.Errorf("problem: got %q, want %q", got, "connection lost")
	}

	out := m.renderTraceroute()
	if strings.Contains(out, "connection lost") {
		t.Errorf("the traceroute output carries the fault, which belongs to the error zone: %q", out)
	}
	if !strings.Contains(out, "192.0.2.1") {
		t.Errorf("the traceroute output lost its own subject: %q", out)
	}
}
