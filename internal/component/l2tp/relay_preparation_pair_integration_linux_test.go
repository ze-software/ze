// Design: docs/architecture/l2tp/followup-l2tp-call.md -- original attachment forwarding
// Related: relay_preparation_test.go -- preparation order and refusal; kernel_counter_transport_integration_linux_test.go -- real native carrier

//go:build integration && linux

package l2tp

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestRelayPreparationForwardsOriginalPair replaces the old portable scalar
// 21/22/23 return oracle with exact genuine pointer and unit-number witnesses.
// It drives the same prepareAndSetupPPP continuation both with and without
// preparation, and consumes the result through the real public kernel owner.
// No fake descriptor, counterfeit public owner, or counter callback is used.
func TestRelayPreparationForwardsOriginalPair(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, prepared := range []bool{false, true} {
		name := "unprepared"
		if prepared {
			name = "prepared"
		}
		t.Run(name, func(t *testing.T) {
			backend, err := iface.OpenBackend("kernel")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, backend.Close()) })
			creator, err := pppkernel.NewUnitCreator(backend)
			require.NoError(t, err)
			transport := kernelCounterTransport(t)
			var trace []string
			event := kernelSetupEvent{}
			if prepared {
				event.beforeBridge = &relayPreparationForTest{prepare: func() error {
					trace = append(trace, "release-local")
					return nil
				}}
			}
			var originalChannel *pppkernel.CreatedChannel
			var originalUnit *pppkernel.CreatedUnit
			transferred := false
			channel, unit, err := prepareAndSetupPPP(event, transport,
				func(original *os.File, requested string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
					require.Same(t, transport, original)
					require.Empty(t, requested)
					trace = append(trace, "attach-worker")
					var setupErr error
					originalChannel, originalUnit, setupErr = pppkernel.DevPPPSetup(creator, original, requested)
					// Register custody before an assertion can stop the callback.
					// Once NewSession consumes the pair these fallback cleanup
					// closures relinquish custody on every constructor outcome.
					if originalChannel != nil {
						t.Cleanup(func() {
							if !transferred {
								require.NoError(t, originalChannel.Close())
							}
						})
					}
					if originalUnit != nil {
						t.Cleanup(func() {
							if !transferred {
								require.NoError(t, originalUnit.Close())
							}
						})
					}
					return originalChannel, originalUnit, setupErr
				})
			require.NoError(t, err)
			require.NotNil(t, channel)
			require.NotNil(t, unit)
			require.Same(t, originalChannel, channel)
			require.Same(t, originalUnit, unit)
			if prepared {
				require.Equal(t, []string{"release-local", "attach-worker"}, trace)
			} else {
				require.Equal(t, []string{"attach-worker"}, trace)
			}
			cfg, err := unit.SessionConfig(channel)
			require.NoError(t, err)
			require.Equal(t, originalUnit.UnitNumber(), cfg.UnitNum)
			transferred = true
			_, owner, err := pppkernel.NewSession(cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, owner.Close()) })
			require.Equal(t, originalUnit.Identity(), owner.Identity())
			counters, err := owner.Counters()
			require.NoError(t, err)
			require.Equal(t, ppp.SessionCounters{
				RxErrors: ppp.MeasuredCounter(0), TxErrors: ppp.MeasuredCounter(0),
				RxDropped: ppp.MeasuredCounter(0), TxDropped: ppp.MeasuredCounter(0),
			}, counters)
		})
	}
}

// TestBridgeRollbackClosesOriginalPair complements the retained logical FD
// cleanup questions with actual native ownership. Deliberately wrong equal
// numeric labels must neither replace nor alias the two genuine originals.
// The transport remains with its carrier fixture; this operation owns no raw
// transport slot. Both original files must refuse a later native operation.
func TestBridgeRollbackClosesOriginalPair(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	backend, err := iface.OpenBackend("kernel")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	creator, err := pppkernel.NewUnitCreator(backend)
	require.NoError(t, err)
	transport := kernelCounterTransport(t)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "")
	owners := &kernelPPPOwners{channel: channel, unit: unit}
	closeTransport := func(*os.File) error {
		t.Error("rollback closed a transport it did not own")
		return errors.New("rollback closed a transport it did not own")
	}
	t.Cleanup(func() {
		failures := closeOriginalKernelPPP(owners, nil, closeTransport)
		for _, failure := range failures {
			require.NoError(t, failure)
		}
	})
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.NotNil(t, unit)
	// Both original files are live immediately before the producing cleanup.
	require.NoError(t, channel.WithDescriptor(func(int) error { return nil }))
	require.NoError(t, unit.WithDescriptor(func(int) error { return nil }))
	failures := closeOriginalKernelPPP(owners, nil, closeTransport)
	for _, failure := range failures {
		require.NoError(t, failure)
	}
	require.ErrorIs(t, channel.WithDescriptor(func(int) error {
		t.Error("closed original channel granted an operation")
		return nil
	}), os.ErrClosed)
	require.ErrorIs(t, unit.WithDescriptor(func(int) error {
		t.Error("closed original unit granted an operation")
		return nil
	}), os.ErrClosed)
}
