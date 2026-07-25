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
	p.Control = func(cmd web.ControlCommand) error {
		dispatched = cmd
		return nil
	}

	args, err := json.Marshal(map[string]string{"action": "pause"})
	require.NoError(t, err)
	result := p.CallTool("chaos_control", args)
	require.NotNil(t, result)
	assert.Nil(t, result["isError"])
	assert.Equal(t, "pause", dispatched.Type)
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
