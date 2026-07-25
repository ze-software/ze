package sdk

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configureWithClaims is the Stage-2 payload shape the engine sends
// (rpc.ConfigureInput). Declared locally so this test exercises the wire
// contract rather than the Go struct.
type configureWithClaims struct {
	Sections []ConfigSection `json:"sections"`
	Claims   []string        `json:"claims,omitempty"`
}

// TestStartupClaimsPrecedeReady pins the ORDERING guarantee that exclusive-role
// claims are delivered on, not merely the fact that they are delivered.
//
// VALIDATES: a claim delivered on the Stage-2 configure callback is readable
// via ClaimActive from inside the configure handler, and the engine has not yet
// received the Stage-5 ready RPC at that moment. Stage 5 is what releases the
// plugin's startup phase, and the engine calls SignalPluginStartupComplete ->
// StartPeers only after every phase completes -- so a decision taken at Stage 2
// is strictly ordered before the first session can establish.
//
// PREVENTS: the peer-up replay duplicate in test/plugin/rfc7606-relay-one-field
// and test/plugin/llgr-readvertise-multipeer, by preventing the regression that
// caused it: moving an exclusive-role decision back onto the post-startup
// callback. That callback is fanned out on detached goroutines immediately
// before StartPeers (internal/component/plugin/server/startup.go,
// sendPostStartupToAll -- waiting there deadlocks), so a role resolved from it
// races session establishment by 1-2 ms on an idle host. If the claim were
// moved there, ClaimActive would still be false at Stage 2 and this test fails.
func TestStartupClaimsPrecedeReady(t *testing.T) {
	t.Parallel()

	p, engine := newTestPair(t)

	const role = "bgp-peer-up-replay"

	type observation struct {
		claimedAtConfigure bool
		unrelatedRole      bool
	}
	observed := make(chan observation, 1)

	p.OnConfigure(func([]ConfigSection) error {
		observed <- observation{
			claimedAtConfigure: p.ClaimActive(role),
			unrelatedRole:      p.ClaimActive("some-role-nobody-claimed"),
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- p.Run(ctx, Registration{})
	}()

	// Stage 1.
	req := readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:declare-registration", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 2: the engine delivers the claim set alongside the config sections.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:configure",
		configureWithClaims{Claims: []string{role}}))

	var got observation
	select {
	case got = <-observed:
	case <-time.After(2 * time.Second):
		t.Fatal("configure callback not called")
	}

	assert.True(t, got.claimedAtConfigure,
		"the claimed role must be readable from the Stage-2 configure handler; "+
			"if this is false the decision has moved to a callback that races peer startup")
	assert.False(t, got.unrelatedRole,
		"an unclaimed role must read false -- the fail-closed direction")

	// The ordering assertion: Stage 5 has NOT been sent yet. Reading it here
	// proves the claim was in place strictly before the plugin declared itself
	// ready, which is strictly before the engine starts peers.
	assert.True(t, p.ClaimActive(role), "claim must still hold after configure returns")

	// Stage 3.
	req = readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:declare-capabilities", req.Method,
		"stage 3 must follow configure -- so configure completed before ready")
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	// Stage 4.
	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:share-registry",
		struct {
			Commands []RegistryCommand `json:"commands"`
		}{}))

	// Stage 5: only now does the plugin declare itself ready.
	req = readEngineRequest(t, ctx, engine.mux)
	require.Equal(t, "ze-plugin-engine:ready", req.Method)
	require.NoError(t, engine.mux.SendOK(ctx, req.ID))

	assert.True(t, p.ClaimActive(role),
		"the claim must still hold once the plugin is ready and events can arrive")

	require.NoError(t, callAndExpectOK(ctx, engine.mux, "ze-plugin-callback:bye",
		struct {
			Reason string `json:"reason"`
		}{Reason: "test-complete"}))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not exit after bye")
	}
}

// TestStartupClaimsAbsentMeansUnclaimed pins the fail-closed default.
//
// VALIDATES: a configure callback with no claims leaves every role unclaimed,
// so a plugin whose default behavior could be stood down keeps performing it.
// PREVENTS: a plugin standing down for an owner that does not exist -- for
// peer-up replay that is silent route loss, strictly worse than the duplicate
// (and idempotent) UPDATE the claim exists to prevent.
func TestStartupClaimsAbsentMeansUnclaimed(t *testing.T) {
	t.Parallel()

	p, _ := newTestPair(t)

	assert.False(t, p.ClaimActive("bgp-peer-up-replay"),
		"no configure delivered yet: nothing may read as claimed")

	// An engine that sends no claims at all (older engine, hub subsystem sink,
	// or simply a daemon with no claiming plugin loaded).
	require.NoError(t, p.handleConfigure(mustJSON(t, configureWithClaims{
		Sections: []ConfigSection{{Root: "bgp", Data: `{}`}},
	})))
	assert.False(t, p.ClaimActive("bgp-peer-up-replay"),
		"configure without claims must leave the role unclaimed")

	// A claim arrives, then a later configure (config reload) carries none.
	require.NoError(t, p.handleConfigure(mustJSON(t, configureWithClaims{
		Claims: []string{"bgp-peer-up-replay"},
	})))
	assert.True(t, p.ClaimActive("bgp-peer-up-replay"))

	require.NoError(t, p.handleConfigure(mustJSON(t, configureWithClaims{})))
	assert.False(t, p.ClaimActive("bgp-peer-up-replay"),
		"the SDK reports what the engine last said; consumers latch monotonically themselves")
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
