// Design: docs/architecture/plugin/rib-storage-design.md -- retained receive gate.
package adj_rib_in

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func retainedAdjEnvelope(t *testing.T, event, payload string) string {
	t.Helper()
	body, err := json.Marshal(rpc.EventEnvelope{Namespace: retention.Namespace, Event: event, Payload: json.RawMessage(payload)})
	require.NoError(t, err)
	return string(body)
}

// Real typed/full-JSON input, retained original-family DOWN and actual decision
// application keep the old gate revalidatable without touching a useful sibling
// or fresh physical source. Retention never promotes Invalid or pending data.
func TestAdjRetainedGateKeepsOriginalValidationBothRails(t *testing.T) {
	for _, jsonRail := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "full-json"}[jsonRail], func(t *testing.T) {
			p := newAdjOwnerProducer(t, jsonRail)
			p.state(1701, 41, true)
			p.state(1702, 42, true)
			old := p.receive(1701, 41, 71, 0)
			sibling := p.receive(1702, 42, 72, 0)
			p.r.validationEnabled = true
			payload, err := retention.Encode(1701, 41, p.address.String(), []family.Family{family.IPv4Unicast})
			require.NoError(t, err)
			matched, err := p.r.handleRetainedValidationEvent(retainedAdjEnvelope(t, retention.EventType, payload))
			require.True(t, matched)
			require.NoError(t, err)
			p.state(1701, 41, false)
			require.True(t, p.r.routePresent(old))
			require.True(t, p.r.routeEligible(old, 71))
			require.True(t, p.r.routeEligible(sibling, 72))
			p.state(1701, 41, false)
			require.True(t, p.r.routeEligible(old, 71), "duplicate DOWN preserves the retained original lifetime")
			decision := rpc.ValidationDecision{PeerAddr: p.address.String(), PeerOwner: 1701, ConnectionID: 41, Family: "ipv4/unicast", Prefix: "10.77.0.0/24", MsgID: 71,
				Ineligible: true, ValState: ValidationInvalid}
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.True(t, p.r.routePresent(old))
			require.False(t, p.r.routeEligible(old, 71))
			require.True(t, p.r.routeEligible(sibling, 72))
			decision.Accept = true
			decision.Ineligible = false
			decision.ValState = ValidationValid
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			require.True(t, p.r.routeEligible(old, 71), "cache recovery reuses retained original bytes")
			_, _, err = p.r.handleCommand("request bgp adj-rib-in disable-validation", nil, "")
			require.NoError(t, err)
			_, snapshot, err := p.r.handleCommand("request bgp adj-rib-in enable-validation", []string{"refresh"}, "")
			require.NoError(t, err)
			rows := snapshot.(map[string]any)["routes"].([]map[string]any)
			retainedRows := 0
			for _, row := range rows {
				if row["gr-retained"] == true {
					retainedRows++
					require.Equal(t, "1701", row["peer-owner"])
					require.Equal(t, "41", row["connection-id"])
				}
			}
			require.Equal(t, 1, retainedRows, "reload preserves actual retained lifetime, not a fresh connection guess")
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			siblingDecision := decision
			siblingDecision.PeerOwner = 1702
			siblingDecision.ConnectionID = 42
			siblingDecision.MsgID = 72
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{siblingDecision})
			require.NoError(t, err)
			p.state(1701, 43, true)
			current := p.receive(1701, 43, 73, 0)
			decision.ConnectionID = 43
			decision.MsgID = 73
			_, err = p.r.handleBatchValidateTyped([]rpc.ValidationDecision{decision})
			require.NoError(t, err)
			payload, err = retention.EncodeCleanup(1701, 41, p.address.String(), nil, false)
			require.NoError(t, err)
			matched, err = p.r.handleRetainedValidationEvent(retainedAdjEnvelope(t, retention.CleanupEventType, payload))
			require.True(t, matched)
			require.NoError(t, err)
			require.False(t, p.r.routePresent(old))
			require.True(t, p.r.routeEligible(current, 73))
			require.True(t, p.r.routeEligible(sibling, 72))
			p.state(1701, 41, false)
			require.True(t, p.r.routeEligible(current, 73), "late original DOWN cannot remove successor")
			require.Equal(t, 2, len(p.r.ribIn))
		})
	}
}

// A pre-cleanup direct replay already escaped the store lock. Cleanup joins
// its actual relay before closing admission, so the following final source
// purge cannot be overtaken by those old bytes even with a shared receipt.
func TestAdjCleanupJoinsAdmittedDirectReplay(t *testing.T) {
	p := newAdjOwnerProducer(t, false)
	p.state(1701, 41, true)
	old := p.receive(1701, 41, 71, 0)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseReplay := sync.OnceFunc(func() { close(release) })
	defer releaseReplay()
	p.r.routeRelayer = func(_ string, routes []rpc.StoredRoute) error {
		once.Do(func() { close(entered); <-release })
		return nil
	}
	replay := make(chan error, 1)
	go func() { _, _, err := p.r.replayCommand([]string{"192.0.2.199"}); replay <- err }()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	free := p.r.replayGuard.TryLock()
	if free {
		p.r.replayGuard.Unlock()
	}
	require.False(t, free, "the actual relay owns admission through its engine handoff")
	payload, err := retention.EncodeCleanup(1701, 41, p.address.String(), nil, false)
	require.NoError(t, err)
	cleanup := make(chan error, 1)
	go func() {
		_, err := p.r.handleRetainedValidationEvent(retainedAdjEnvelope(t, retention.CleanupEventType, payload))
		cleanup <- err
	}()
	require.True(t, p.r.routePresent(old), "a blocked prior replay is still outstanding")
	releaseReplay()
	select {
	case err := <-replay:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-cleanup:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.False(t, p.r.routePresent(old))
	routes, _ := p.r.buildReplayRoutes(netip.MustParseAddr("192.0.2.199"), 0, unboundedReplay())
	require.Empty(t, routes)
}
