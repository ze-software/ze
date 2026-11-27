package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/validation"
	"github.com/ze-software/ze/internal/chaos/web"
)

func newTestProvider() *Provider {
	state := web.NewDashboardState(2, 2, 100)
	state.Seed = 42
	state.TotalAnnounced = 100
	state.TotalReceived = 95
	state.TotalMissing = 5
	state.Peers[0].RoutesSent = 50
	state.Peers[0].RoutesRecv = 47
	state.Peers[0].Missing = 3
	state.Peers[1].RoutesSent = 50
	state.Peers[1].RoutesRecv = 48
	state.Peers[1].Missing = 2

	return &Provider{
		State:       state,
		Convergence: validation.NewConvergence(2, 30*time.Second),
		Seed:        42,
		StartTime:   time.Now(),
		PeerCount:   2,
	}
}

func extractText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	require.True(t, ok, "result missing content array")
	require.NotEmpty(t, content)
	text, ok := content[0]["text"].(string)
	require.True(t, ok, "content[0] missing text string")
	return text
}

func TestChaosToolStatus(t *testing.T) {
	p := newTestProvider()
	result := p.CallTool("chaos_status", nil)
	require.NotNil(t, result)

	text := extractText(t, result)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &status))

	assert.Equal(t, float64(42), status["seed"])
	assert.Equal(t, float64(100), status["routes-announced"])
	assert.Equal(t, float64(95), status["routes-received"])
	assert.Equal(t, float64(5), status["routes-missing"])
	assert.NotNil(t, status["convergence"])
	assert.Contains(t, status, "control")
}

func TestChaosToolProblems(t *testing.T) {
	p := newTestProvider()

	result := p.CallTool("chaos_problems", nil)
	require.NotNil(t, result)

	text := extractText(t, result)
	var problems []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &problems))

	found := false
	for _, prob := range problems {
		if prob["type"] == "missing-routes" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected missing-routes problem when TotalMissing > 0")
}

func TestChaosToolProblemsHealthy(t *testing.T) {
	p := newTestProvider()
	p.State.TotalMissing = 0
	for _, ps := range p.State.Peers {
		ps.Missing = 0
	}

	result := p.CallTool("chaos_problems", nil)
	require.NotNil(t, result)

	text := extractText(t, result)
	var problems []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &problems))
	assert.Empty(t, problems)
}

func TestChaosToolPeers(t *testing.T) {
	p := newTestProvider()

	result := p.CallTool("chaos_peers", nil)
	require.NotNil(t, result)

	text := extractText(t, result)
	var peers []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &peers))
	assert.Len(t, peers, 2)
	assert.Equal(t, float64(0), peers[0]["index"])
	assert.Equal(t, float64(1), peers[1]["index"])
}

func TestChaosToolPeersSingle(t *testing.T) {
	p := newTestProvider()

	args, err := json.Marshal(map[string]int{"peer": 0})
	require.NoError(t, err)
	result := p.CallTool("chaos_peers", args)
	require.NotNil(t, result)

	text := extractText(t, result)
	var peer map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &peer))
	assert.Equal(t, float64(0), peer["index"])
	assert.Contains(t, peer, "recent-chaos")
}

func TestChaosToolScenario(t *testing.T) {
	p := newTestProvider()

	result := p.CallTool("chaos_scenario", nil)
	require.NotNil(t, result)

	text := extractText(t, result)
	var scenario map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &scenario))
	assert.Equal(t, float64(42), scenario["seed"])
	assert.Equal(t, float64(2), scenario["peer-count"])
}

func TestChaosToolControl(t *testing.T) {
	var dispatched web.ControlCommand
	p := newTestProvider()
	p.Control = func(cmd web.ControlCommand) (web.ControlResult, error) {
		dispatched = cmd
		return web.ControlResult{Action: cmd.Type}, nil
	}

	args, err := json.Marshal(map[string]string{"action": "pause"})
	require.NoError(t, err)
	result := p.CallTool("chaos_control", args)
	require.NotNil(t, result)
	assert.Nil(t, result["isError"])
	assert.Equal(t, "pause", dispatched.Type)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, result)), &body))
	assert.Equal(t, true, body["ok"])
	assert.Equal(t, "pause", body["action"])
}

// TestChaosToolControlTriggerParamsAndOutcomes verifies trigger passes params and
// peer lists through and surfaces synchronous dispatch/block/drop outcomes.
//
// VALIDATES: Programmatic callers can trigger parameterized actions and tell
// whether a fault was dispatched or blocked.
// PREVENTS: Fire-and-forget triggers hiding blocked or dropped faults behind "ok".
func TestChaosToolControlTriggerParamsAndOutcomes(t *testing.T) {
	var dispatched web.ControlCommand
	p := newTestProvider()
	p.Control = func(cmd web.ControlCommand) (web.ControlResult, error) {
		dispatched = cmd
		return web.ControlResult{
			Action:      cmd.Type,
			ChaosAction: cmd.Trigger.ActionType,
			Dispatched:  []int{0},
			Blocked:     []web.ControlTargetOutcome{{Peer: 1, Reason: "peer not established"}},
		}, nil
	}

	args, err := json.Marshal(map[string]any{
		"action":       "trigger",
		"peers":        []int{0, 1},
		"chaos-action": "route-flap",
		"params":       map[string]string{"count": "10", "cycles": "2", "interval": "50ms"},
	})
	require.NoError(t, err)
	result := p.CallTool("chaos_control", args)
	require.NotNil(t, result)
	assert.Nil(t, result["isError"])
	require.NotNil(t, dispatched.Trigger)
	assert.Equal(t, []int{0, 1}, dispatched.Trigger.Peers)
	assert.Equal(t, map[string]string{"count": "10", "cycles": "2", "interval": "50ms"}, dispatched.Trigger.Params)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, result)), &body))
	assert.Equal(t, true, body["ok"])
	assert.Equal(t, "route-flap", body["chaos-action"])
	assert.Equal(t, []any{float64(0)}, body["dispatched"])
	assert.Len(t, body["blocked"], 1)

	// When zero peers receive the trigger, MCP must report an error rather than ok.
	p.Control = func(cmd web.ControlCommand) (web.ControlResult, error) {
		return web.ControlResult{
			Action:      cmd.Type,
			ChaosAction: cmd.Trigger.ActionType,
			Blocked:     []web.ControlTargetOutcome{{Peer: 0, Reason: "hold-timer expiry already pending"}},
		}, nil
	}
	blockedArgs, err := json.Marshal(map[string]any{
		"action":       "trigger",
		"peer":         0,
		"chaos-action": "hold-timer-expiry",
	})
	require.NoError(t, err)
	blockedRes := p.CallTool("chaos_control", blockedArgs)
	require.NotNil(t, blockedRes)
	assert.Equal(t, true, blockedRes["isError"])
	assert.Contains(t, extractText(t, blockedRes), "hold-timer expiry already pending")

	// Supplying both peer and peers must be rejected.
	conflictArgs, err := json.Marshal(map[string]any{
		"action":       "trigger",
		"peer":         0,
		"peers":        []int{1},
		"chaos-action": "tcp-disconnect",
	})
	require.NoError(t, err)
	conflictRes := p.CallTool("chaos_control", conflictArgs)
	require.NotNil(t, conflictRes)
	assert.Equal(t, true, conflictRes["isError"])
	assert.Contains(t, extractText(t, conflictRes), "not both")

	// Unknown chaos-action must be rejected before dispatch.
	unknownArgs, err := json.Marshal(map[string]any{
		"action":       "trigger",
		"peer":         0,
		"chaos-action": "bogus-fault",
	})
	require.NoError(t, err)
	unknownRes := p.CallTool("chaos_control", unknownArgs)
	require.NotNil(t, unknownRes)
	assert.Equal(t, true, unknownRes["isError"])
	assert.Contains(t, extractText(t, unknownRes), "unknown chaos-action")
}

func TestChaosToolErrors(t *testing.T) {
	p := newTestProvider()

	result := p.CallTool("nonexistent", nil)
	assert.Nil(t, result)

	args, err := json.Marshal(map[string]int{"peer": 999})
	require.NoError(t, err)
	result = p.CallTool("chaos_peers", args)
	require.NotNil(t, result)
	isErr, ok := result["isError"].(bool)
	require.True(t, ok)
	assert.True(t, isErr)

	args, err = json.Marshal(map[string]string{"action": "destroy"})
	require.NoError(t, err)
	result = p.CallTool("chaos_control", args)
	require.NotNil(t, result)
	isErr, ok = result["isError"].(bool)
	require.True(t, ok)
	assert.True(t, isErr)

	p.Control = nil
	args, err = json.Marshal(map[string]string{"action": "pause"})
	require.NoError(t, err)
	result = p.CallTool("chaos_control", args)
	require.NotNil(t, result)
	isErr, ok = result["isError"].(bool)
	require.True(t, ok)
	assert.True(t, isErr)
}
