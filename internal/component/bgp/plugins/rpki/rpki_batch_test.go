package rpki

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func newTestBatchPlugin(t *testing.T) *RPKIPlugin {
	t.Helper()
	pluginEnd, remoteEnd := net.Pipe()
	require.NoError(t, remoteEnd.Close())
	p := sdk.NewWithConn("rpki-test", pluginEnd)
	t.Cleanup(func() { _ = p.Close() })
	return &RPKIPlugin{
		plugin:     p,
		cache:      NewROACache(),
		aspaCache:  NewASPACache(),
		validateCh: make(chan validationRequest, 4096),
		stopCh:     make(chan struct{}),
	}
}

// TestDrainAndDispatchFiltersNotValidated verifies drainAndDispatch
// skips NotValidated entries and drains the channel.
func TestDrainAndDispatchFiltersNotValidated(t *testing.T) {
	rp := newTestBatchPlugin(t)

	rp.validateCh <- validationRequest{
		peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
		pathID: 0, state: ValidationValid,
	}
	rp.validateCh <- validationRequest{
		peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.1.0/24",
		pathID: 1, state: ValidationInvalid,
	}
	rp.validateCh <- validationRequest{
		peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.2.0/24",
		pathID: 2, state: ValidationNotValidated,
	}

	batch := make([]validationRequest, 0, maxBatchSize)
	rp.drainAndDispatch(batch)

	assert.Empty(t, rp.validateCh, "channel should be empty after drain")
}

// TestDrainAndDispatchChunks verifies drainAndDispatch dispatches in
// maxBatchSize chunks when the channel has more than maxBatchSize items.
func TestDrainAndDispatchChunks(t *testing.T) {
	rp := newTestBatchPlugin(t)

	for i := range 200 {
		rp.validateCh <- validationRequest{
			peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
			pathID: uint32(i), state: ValidationValid,
		}
	}

	batch := make([]validationRequest, 0, maxBatchSize)
	rp.drainAndDispatch(batch)

	assert.Empty(t, rp.validateCh, "all 200 items should be drained")
}

// TestBuildDecisionsASPAOverride verifies that ASPA reject policy
// flips an origin-valid accept into a reject in the typed decisions.
func TestBuildDecisionsASPAOverride(t *testing.T) {
	rp := &RPKIPlugin{
		cache:     NewROACache(),
		aspaCache: NewASPACache(),
		stopCh:    make(chan struct{}),
	}
	rp.aspaInvalidAction.Store(uint32(ASPAPolicyReject))
	rp.aspaUnknownAction.Store(uint32(ASPAPolicyAccept))

	batch := []validationRequest{
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
			pathID: 0, state: ValidationValid, aspaState: aspaStateNone},
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.1.0/24",
			pathID: 1, state: ValidationValid, aspaState: ASPAInvalid},
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.2.0/24",
			pathID: 2, state: ValidationValid, aspaState: ASPAUnknown},
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.3.0/24",
			pathID: 3, state: ValidationInvalid, aspaState: aspaStateNone},
	}

	decisions := rp.buildDecisions(batch)
	require.Len(t, decisions, 4)

	assert.True(t, decisions[0].Accept, "origin valid, no ASPA -> accept")
	assert.False(t, decisions[1].Accept, "origin valid, ASPA invalid + reject policy -> reject")
	assert.True(t, decisions[2].Accept, "origin valid, ASPA unknown + accept policy -> accept")
	assert.False(t, decisions[3].Accept, "origin invalid -> reject regardless of ASPA")
}

// TestBuildDecisionsOriginInvalidAction verifies the RFC 6811 origin-validation disposition is an
// operator-configurable, per-state policy choice rather than an automatic side effect.
//
// RFC requirement: RFC6811-2-2 positive -- exclusion of an Invalid route is explicitly configured:
// with invalid-action=reject the Invalid route is excluded (Accept=false).
// RFC requirement: RFC6811-2-2 negative -- absent that configuration the route is NOT excluded as
// a side effect of its state: with invalid-action=accept the Invalid route stays in the
// Adj-RIB-In (Accept=true), still marked with its Invalid validation state.
// RFC requirement: RFC6811-3-1 positive -- the operator can match a validation state and set its
// disposition: the configured invalid-action determines whether the Invalid route is rejected or
// accepted, and the accepted route retains its Invalid state marker.
// RFC requirement: RFC6811-3-1 negative -- the action is state-specific, not a blanket rule: a
// Valid route is accepted regardless of the invalid-action, so the policy is keyed on the state.
func TestBuildDecisionsOriginInvalidAction(t *testing.T) {
	newRP := func(action uint8) *RPKIPlugin {
		rp := &RPKIPlugin{cache: NewROACache(), aspaCache: NewASPACache(), stopCh: make(chan struct{})}
		rp.originInvalidAction.Store(uint32(action))
		return rp
	}
	batch := []validationRequest{
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
			pathID: 0, state: ValidationInvalid, aspaState: aspaStateNone},
		{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.1.0/24",
			pathID: 1, state: ValidationValid, aspaState: aspaStateNone},
	}

	// invalid-action=reject: the Invalid route is excluded; the Valid route is unaffected.
	rej := newRP(ASPAPolicyReject).buildDecisions(batch)
	assert.False(t, rej[0].Accept, "invalid-action=reject excludes the Invalid route")
	assert.True(t, rej[1].Accept, "Valid route is accepted regardless of invalid-action")

	// invalid-action=accept: the Invalid route is retained and still marked Invalid.
	acc := newRP(ASPAPolicyAccept).buildDecisions(batch)
	assert.True(t, acc[0].Accept, "invalid-action=accept keeps the Invalid route in the Adj-RIB-In")
	assert.Equal(t, ValidationInvalid, acc[0].ValState, "accepted Invalid route retains its Invalid state marker")
	assert.True(t, acc[1].Accept, "Valid route is accepted under either policy")
}

// TestBatchSizeBound verifies maxBatchSize is explicit and bounded.
func TestBatchSizeBound(t *testing.T) {
	assert.Equal(t, 128, maxBatchSize, "maxBatchSize should be 128")
}

// TestBatchWaitBound verifies batchWait is explicit and bounded.
func TestBatchWaitBound(t *testing.T) {
	assert.Equal(t, 1*time.Millisecond, batchWait, "batchWait should be 1ms")
}

// TestDispatchBatchEmpty verifies dispatchBatch is a no-op for empty batch.
func TestDispatchBatchEmpty(t *testing.T) {
	rp := &RPKIPlugin{
		cache:     NewROACache(),
		aspaCache: NewASPACache(),
		stopCh:    make(chan struct{}),
	}
	rp.dispatchBatch(nil)
}

// TestWorkerShutdownCompletes verifies the worker goroutine exits
// when stopCh is closed, even with a partially filled batch.
func TestWorkerShutdownCompletes(t *testing.T) {
	rp := newTestBatchPlugin(t)

	rp.validateCh <- validationRequest{
		peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24",
		pathID: 0, state: ValidationValid,
	}

	done := make(chan struct{})
	go func() {
		rp.validationWorker()
		close(done)
	}()

	time.Sleep(5 * time.Millisecond)
	close(rp.stopCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		require.Fail(t, "worker did not exit within timeout")
	}

	assert.Empty(t, rp.validateCh, "channel should be drained on shutdown")
}
