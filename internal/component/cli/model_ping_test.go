// Design: docs/architecture/api/commands.md -- monitor ping argument and stats tests

package cli

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePingMonitorArgs(t *testing.T) {
	target, interval, timeout, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4")
	require.Empty(t, errMsg)
	assert.Equal(t, "1.2.3.4", target)
	assert.Equal(t, defaultPingMonitorInterval, interval)
	assert.Equal(t, defaultPingMonitorTimeout, timeout)
}

func TestParsePingMonitorArgsWithInterval(t *testing.T) {
	target, interval, _, errMsg := parsePingMonitorArgs("monitor ping 10.0.0.1 interval 500ms")
	require.Empty(t, errMsg)
	assert.Equal(t, "10.0.0.1", target)
	assert.Equal(t, 500*time.Millisecond, interval)
}

func TestParsePingMonitorArgsWithTimeout(t *testing.T) {
	_, _, timeout, errMsg := parsePingMonitorArgs("monitor ping 10.0.0.1 timeout 2s")
	require.Empty(t, errMsg)
	assert.Equal(t, 2*time.Second, timeout)
}

func TestParsePingMonitorArgsOnlyKeywords(t *testing.T) {
	target, _, _, errMsg := parsePingMonitorArgs("monitor ping interval 1s")
	assert.Empty(t, target)
	assert.Empty(t, errMsg)
}

func TestParsePingMonitorArgsIntervalMissingValue(t *testing.T) {
	_, _, _, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 interval")
	assert.Contains(t, errMsg, "interval requires a value")
}

func TestParsePingMonitorArgsTimeoutMissingValue(t *testing.T) {
	_, _, _, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 timeout")
	assert.Contains(t, errMsg, "timeout requires a value")
}

func TestParsePingMonitorArgsIntervalTooLow(t *testing.T) {
	_, _, _, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 interval 10ms")
	assert.Contains(t, errMsg, "interval must be")
}

func TestParsePingMonitorArgsTimeoutTooHigh(t *testing.T) {
	_, _, _, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 timeout 60s")
	assert.Contains(t, errMsg, "timeout must be")
}

func TestParsePingMonitorArgsUnexpected(t *testing.T) {
	_, _, _, errMsg := parsePingMonitorArgs("monitor ping 1.2.3.4 extra")
	assert.Contains(t, errMsg, "unexpected argument")
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
	return func(_ context.Context, _ string, _, _ time.Duration) (<-chan map[string]any, context.CancelFunc, error) {
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
	m := NewCommandModel()
	m.pingFactory = pingTestFactory(nil)

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | resolve | origin | log")
	require.NotNil(t, cmd)
	require.NotNil(t, m.pingMonitorPiped)
	assert.True(t, m.pingMonitorPiped.logMode)
	assert.True(t, m.pingMonitorPiped.pipeResolve)
	assert.True(t, m.pingMonitorPiped.pipeOrigin)
	assert.False(t, m.pingMonitorPiped.hasFormatPipe)
}

// VALIDATES: monitor ping | resolve | log enriches the target legend with the
// PTR name (pipe-completeness: data-transform pipes apply in | log mode).
// PREVENTS: | log rendering bypassing | resolve enrichment.
func TestHandlePingPipedPoll_LogResolveEnrichesLegend(t *testing.T) {
	setStubResolvers(t)

	m := NewCommandModel()
	m.pingFactory = pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
		{"seq": 1, "status": "timeout"},
	})

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

	m := NewCommandModel()
	m.pingFactory = pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
	})

	cmd := m.startPingMonitorPiped("monitor ping 192.0.2.1 | origin | log")
	require.NotNil(t, cmd)

	_, pollCmd := m.handlePingPipedPoll()
	assert.Nil(t, pollCmd, "closed reply channel must end the piped session")

	assert.Contains(t, m.outputBuf.String(), "--- 192.0.2.1 TEST-NET-AS ---")
}

// VALIDATES: without | resolve / | origin the legend stays the plain target.
func TestHandlePingPipedPoll_LogPlainLegend(t *testing.T) {
	setStubResolvers(t)

	m := NewCommandModel()
	m.pingFactory = pingTestFactory([]map[string]any{
		{"seq": 0, "status": "ok", "rtt-ms": 1.5},
	})

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
