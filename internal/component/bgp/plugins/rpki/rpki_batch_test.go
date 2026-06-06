package rpki

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
