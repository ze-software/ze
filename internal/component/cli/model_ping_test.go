// Design: docs/architecture/api/commands.md -- monitor ping argument and stats tests

package cli

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestPingMonitorDeclaresItsStreamShape verifies the owning registration admits
// enrichment only for the event's target field.
func TestPingMonitorDeclaresItsStreamShape(t *testing.T) {
	shape, declared := command.ShapeForCommand("monitor ping 192.0.2.1 count 2")
	require.True(t, declared)
	assert.Equal(t, command.ShapeTab, shape)
	assert.Equal(t, []string{"target"}, command.AddressFieldsForCommand("monitor ping 192.0.2.1"))

	_, _, flags, saves, errMsg := command.ProcessStreamPipes(
		"monitor ping 192.0.2.1 | resolve | origin | log", "",
	)
	t.Cleanup(func() { _ = saves.Abort() })
	require.Empty(t, errMsg)
	assert.True(t, flags.Resolve)
	assert.True(t, flags.Origin)
}

func TestParsePingMonitorArgs(t *testing.T) {
	mp, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4")
	target, interval, timeout := mp.Target, mp.Interval, mp.Timeout
	require.Empty(t, errMsg)
	assert.Equal(t, "1.2.3.4", target)
	assert.Equal(t, defaultPingMonitorInterval, interval)
	assert.Equal(t, defaultPingMonitorTimeout, timeout)
}

func TestParsePingMonitorArgsWithInterval(t *testing.T) {
	mp, errMsg := parsePingMonitorArgs("monitor ping 10.0.0.1 interval 500ms")
	target, interval := mp.Target, mp.Interval
	require.Empty(t, errMsg)
	assert.Equal(t, "10.0.0.1", target)
	assert.Equal(t, 500*time.Millisecond, interval)
}

func TestParsePingMonitorArgsWithTimeout(t *testing.T) {
	mp, errMsg := parsePingMonitorArgs("monitor ping 10.0.0.1 timeout 2s")
	timeout := mp.Timeout
	require.Empty(t, errMsg)
	assert.Equal(t, 2*time.Second, timeout)
}

func TestParsePingMonitorArgsOnlyKeywords(t *testing.T) {
	mp, errMsg := parsePingMonitorArgs("monitor ping interval 1s")
	target := mp.Target
	assert.Empty(t, target)
	assert.Empty(t, errMsg)
}

func TestParsePingMonitorArgsIntervalMissingValue(t *testing.T) {
	_, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 interval")
	assert.Contains(t, errMsg, "interval requires a value")
}

func TestParsePingMonitorArgsTimeoutMissingValue(t *testing.T) {
	_, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 timeout")
	assert.Contains(t, errMsg, "timeout requires a value")
}

func TestParsePingMonitorArgsIntervalTooLow(t *testing.T) {
	_, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 interval 10ms")
	assert.Contains(t, errMsg, "interval must be")
}

func TestParsePingMonitorArgsTimeoutTooHigh(t *testing.T) {
	_, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 timeout 60s")
	assert.Contains(t, errMsg, "timeout must be")
}

func TestParsePingMonitorArgsUnexpected(t *testing.T) {
	_, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 extra")
	assert.Contains(t, errMsg, "unexpected argument")
}

// TestParsePingMonitorArgsCountAndSize verifies the interactive CLI accepts both.
//
// VALIDATES: `monitor ping <t> count 5 size 1400` parses; omitting them leaves
// zero, which means stream-until-stopped with the default payload.
// PREVENTS: a regression to the previous behavior, where this parser had no
// count/size case and both fell to the default branch, rejecting them as
// "unexpected argument" while the offline path silently ignored them. The two
// paths are the same command and must accept the same input.
func TestParsePingMonitorArgsCountAndSize(t *testing.T) {
	mp, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 count 5 size 1400")
	require.Empty(t, errMsg)
	assert.Equal(t, "1.2.3.4", mp.Target)
	assert.Equal(t, 5, mp.Count)
	assert.Equal(t, 1400, mp.Size)

	mp, errMsg = parsePingMonitorArgs("monitor ping 1.2.3.4")
	require.Empty(t, errMsg)
	assert.Equal(t, 0, mp.Count, "no count streams until stopped")
	assert.Equal(t, 0, mp.Size, "no size uses the default payload")
}

// TestParsePingMonitorArgsCountSizeBounds pins the accepted ranges.
//
// VALIDATES: count 1-100 and size 1-65507, rejecting outside.
// PREVENTS: drift from the offline parser
// (internal/component/ping/cmd/ping.go parseMonitorPingArgs), which enforces the
// same limits. If these two disagree, `monitor ping` accepts different input
// depending on whether a daemon is running.
func TestParsePingMonitorArgsCountSizeBounds(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "count minimum", input: "monitor ping 1.2.3.4 count 1"},
		{name: "count maximum", input: "monitor ping 1.2.3.4 count 100"},
		{name: "count zero", input: "monitor ping 1.2.3.4 count 0", wantErr: "count must be"},
		{name: "count above", input: "monitor ping 1.2.3.4 count 101", wantErr: "count must be"},
		{name: "count missing", input: "monitor ping 1.2.3.4 count", wantErr: "count requires a value"},
		{name: "size minimum", input: "monitor ping 1.2.3.4 size 1"},
		{name: "size maximum", input: "monitor ping 1.2.3.4 size 65507"},
		{name: "size zero", input: "monitor ping 1.2.3.4 size 0", wantErr: "size must be"},
		{name: "size above", input: "monitor ping 1.2.3.4 size 65508", wantErr: "size must be"},
		{name: "size missing", input: "monitor ping 1.2.3.4 size", wantErr: "size requires a value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errMsg := parsePingMonitorArgs(tc.input)
			if tc.wantErr != "" {
				assert.Contains(t, errMsg, tc.wantErr)
				return
			}
			assert.Empty(t, errMsg)
		})
	}
}

// TestStartPingMonitorPassesCountAndSize verifies the parsed values reach the
// factory, not just the parser.
//
// VALIDATES: startPingMonitor threads count and size into PingFactory.
// PREVENTS: the exact class of bug being fixed -- arguments parsed correctly and
// then dropped before the probe. Unit-testing the parser alone would not catch a
// call site that forgets to pass them.
func TestStartPingMonitorPassesCountAndSize(t *testing.T) {
	var gotCount, gotSize int
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(func(_ context.Context, _ string, _, _ time.Duration, count, size int) (<-chan map[string]any, context.CancelFunc, error) {
		gotCount, gotSize = count, size
		ch := make(chan map[string]any)
		close(ch)
		_, cancel := context.WithCancel(context.Background())
		return ch, cancel, nil
	})

	m.startPingMonitor("monitor ping 192.0.2.1 count 7 size 512")
	assert.Equal(t, 7, gotCount)
	assert.Equal(t, 512, gotSize)
}

func TestIsPingMonitorCommand(t *testing.T) {
	assert.True(t, isPingMonitorCommand("monitor ping 1.2.3.4"))
	assert.True(t, isPingMonitorCommand("  monitor ping 1.2.3.4 "))
	assert.False(t, isPingMonitorCommand("monitor ping 1.2.3.4 | log"))
	assert.False(t, isPingMonitorCommand("show ping 1.2.3.4"))
}

func TestIsPipedPingMonitorCommand(t *testing.T) {
	assert.True(t, isPipedPingMonitorCommand("monitor ping 1.2.3.4 | log"))
	assert.True(t, isPipedPingMonitorCommand("monitor ping 1.2.3.4 | json"))
	assert.False(t, isPipedPingMonitorCommand("monitor ping 1.2.3.4"))
}

func TestPingStatsEmpty(t *testing.T) {
	s := pingStats{}
	assert.Equal(t, 0.0, s.loss())
	assert.Equal(t, 0.0, s.avg())
	assert.Equal(t, 0.0, s.stddev())
}

func TestPingStatsAccumulation(t *testing.T) {
	s := pingStats{min: math.MaxFloat64}

	applyPingReply(&s, map[string]any{"seq": 0, "status": "ok", "rtt-ms": 1.0})
	applyPingReply(&s, map[string]any{"seq": 1, "status": "ok", "rtt-ms": 3.0})
	applyPingReply(&s, map[string]any{"seq": 2, "status": "timeout"})
	applyPingReply(&s, map[string]any{"seq": 3, "status": "ok", "rtt-ms": 2.0})

	assert.Equal(t, 4, s.sent)
	assert.Equal(t, 3, s.recv)
	assert.Equal(t, 2.0, s.last)
	assert.Equal(t, 1.0, s.min)
	assert.Equal(t, 3.0, s.max)
	assert.InDelta(t, 25.0, s.loss(), 0.01)
	assert.InDelta(t, 2.0, s.avg(), 0.01)
	assert.InDelta(t, 1.0, s.stddev(), 0.01)
}

func TestFormatPingReplyLineOK(t *testing.T) {
	line := formatPingReplyLine(map[string]any{"seq": 5, "status": "ok", "rtt-ms": 1.234})
	assert.Contains(t, line, "seq=5")
	assert.Contains(t, line, "rtt=1.234ms")
}

func TestFormatPingReplyLineTimeout(t *testing.T) {
	line := formatPingReplyLine(map[string]any{"seq": 3, "status": "timeout"})
	assert.Contains(t, line, "seq=3")
	assert.Contains(t, line, "timeout")
}

func TestPingReplyToJSONOK(t *testing.T) {
	j := pingReplyToJSON("1.1.1.1", map[string]any{"seq": 5, "status": "ok", "rtt-ms": 1.234})
	assert.Equal(t, `{"target":"1.1.1.1","seq":5,"status":"ok","rtt-ms":1.234}`, j)
}

func TestPingReplyToJSONTimeout(t *testing.T) {
	j := pingReplyToJSON("1.1.1.1", map[string]any{"seq": 3, "status": "timeout"})
	assert.Equal(t, `{"target":"1.1.1.1","seq":3,"status":"timeout"}`, j)
}

func TestPingReplyToJSONEscapesStatus(t *testing.T) {
	j := pingReplyToJSON("1.1.1.1", map[string]any{"seq": 0, "status": `bad"quote`})
	assert.Equal(t, `{"target":"1.1.1.1","seq":0,"status":"bad\"quote"}`, j)
}

func TestPingReplyToJSONEscapesControlChars(t *testing.T) {
	j := pingReplyToJSON("1.1.1.1", map[string]any{"seq": 0, "status": "a\nb"})
	assert.Equal(t, "{\"target\":\"1.1.1.1\",\"seq\":0,\"status\":\"a\\u000ab\"}", j)
}

func TestPingReplyToJSONFloat64Seq(t *testing.T) {
	j := pingReplyToJSON("1.1.1.1", map[string]any{"seq": float64(7), "status": "ok", "rtt-ms": 0.5})
	assert.Equal(t, `{"target":"1.1.1.1","seq":7,"status":"ok","rtt-ms":0.500}`, j)
}

// pingTestFactory returns a PingFactory whose channel is pre-filled with the
// given replies and closed, so a single poll drains the whole session.
func pingTestFactory(replies []map[string]any) PingFactory {
	return func(_ context.Context, _ string, _, _ time.Duration, _, _ int) (<-chan map[string]any, context.CancelFunc, error) {
		ch := make(chan map[string]any, len(replies)+1)
		for _, r := range replies {
			ch <- r
		}
		close(ch)
		_, cancel := context.WithCancel(context.Background())
		return ch, cancel, nil
	}
}

// VALIDATES: | resolve and | origin flags are captured by startPingMonitorPiped.
// PREVENTS: pipe flags parsed but dropped before the | log render path.
func TestStartPingMonitorPiped_CapturesEnrichmentFlags(t *testing.T) {
	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingTestFactory(nil))

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | resolve | origin | log")
	require.NotNil(t, cmd)
	require.NotNil(t, m.activePingPiped())
	assert.True(t, m.activePingPiped().logMode)
	assert.True(t, m.activePingPiped().pipeResolve)
	assert.True(t, m.activePingPiped().pipeOrigin)
	assert.False(t, m.activePingPiped().hasFormatPipe)
}

// VALIDATES: monitor ping | resolve | log enriches the target legend with the
// PTR name (pipe-completeness: data-transform pipes apply in | log mode).
// PREVENTS: | log rendering bypassing | resolve enrichment.
func TestHandlePingPipedPoll_LogResolveEnrichesLegend(t *testing.T) {
	setStubResolvers(t)

	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
		{"seq": 1, "status": "timeout"},
	}))

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | resolve | log")
	require.NotNil(t, cmd)

	_, pollCmd := m.handlePingPipedPoll()
	assert.Nil(t, pollCmd, "closed reply channel must end the piped session")

	out := m.outputBuf.String()
	assert.Contains(t, out, "--- 192.0.2.1 ping-target.test ---")
	assert.Contains(t, out, "seq=0")
	assert.Contains(t, out, "seq=1")
}

// VALIDATES: monitor ping | origin | log enriches the target legend with AS data.
func TestHandlePingPipedPoll_LogOriginEnrichesLegend(t *testing.T) {
	setStubResolvers(t)

	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
	}))

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | origin | log")
	require.NotNil(t, cmd)

	_, pollCmd := m.handlePingPipedPoll()
	assert.Nil(t, pollCmd, "closed reply channel must end the piped session")

	assert.Contains(t, m.outputBuf.String(), "--- 192.0.2.1 TEST-NET-AS ---")
}

// VALIDATES: without | resolve / | origin the legend stays the plain target.
func TestHandlePingPipedPoll_LogPlainLegend(t *testing.T) {
	setStubResolvers(t)

	m := NewCommandModel(FilesystemAuthorityOperatorLocal)
	m.SetPingFactory(pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
	}))

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | log")
	require.NotNil(t, cmd)

	_, pollCmd := m.handlePingPipedPoll()
	assert.Nil(t, pollCmd, "closed reply channel must end the piped session")

	out := m.outputBuf.String()
	assert.Contains(t, out, "--- 192.0.2.1 ---")
	assert.NotContains(t, out, "ping-target.test")
	assert.NotContains(t, out, "TEST-NET-AS")
}

func TestRenderPingStatsPlain(t *testing.T) {
	s := pingStats{min: math.MaxFloat64}
	applyPingReply(&s, map[string]any{"seq": 0, "status": "ok", "rtt-ms": 1.5})
	applyPingReply(&s, map[string]any{"seq": 1, "status": "ok", "rtt-ms": 2.5})

	output := renderPingStatsPlain("10.0.0.1", &s)
	assert.Contains(t, output, "Ping 10.0.0.1")
	assert.Contains(t, output, "Sent 2")
	assert.Contains(t, output, "Recv 2")
	assert.Contains(t, output, "Loss 0.0%")
	assert.Contains(t, output, "Min 1.500ms")
	assert.Contains(t, output, "Max 2.500ms")
}
