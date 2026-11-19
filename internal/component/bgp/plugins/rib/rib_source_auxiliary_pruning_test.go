// Design: docs/architecture/plugin/rib-storage-design.md -- live auxiliary debt without completed history.
package rib

import (
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A genuine untouched received reference stays live across repeated successful
// cleanup of other receipts from its original physical connection. Completed
// batches and their owned commands must disappear from EVERY live lease while
// ready, held and UNKNOWN work remains. Real registered commands and SDK output
// prove useful independent progress despite those earlier unfinished writes.
// MUTATION: omit forgetPhysicalSentOutput after a positive batch join; the
// survivor then retains completed history even though no source is released.
func TestGRSourceAuxiliaryDebtPrunesOnlyCompletedBatches(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "wire", true: "bridge"}[native], func(t *testing.T) {
			ctx := selectedDispatchContext(t)
			entered, unblock := make(chan rpc.UpdateRouteInput, 1), make(chan struct{})
			var once sync.Once
			var mu sync.Mutex
			var written []string
			writer := func(input rpc.UpdateRouteInput) (*rpc.UpdateRouteOutput, error) {
				mu.Lock()
				written = append(written, input.Command)
				mu.Unlock()
				if strings.Contains(input.Command, "82.0.0.0/8") {
					return nil, errors.New("original completion remains unknown")
				}
				if strings.Contains(input.Command, "83.0.0.0/8") {
					entered <- input
					<-unblock
				}
				return &rpc.UpdateRouteOutput{Withdrawn: 1}, nil
			}
			r, calls := sourceLeaseTestRIBWithOutput(t, native, true, writer)
			done := make(chan error, 1)
			joined := true
			t.Cleanup(func() {
				once.Do(func() { close(unblock) })
				if !joined {
					selectedDispatchAwait(t, ctx, done)
				}
				r.sourceLeases.stop()
				r.sourceLeases = nil
				for _, destinations := range r.ribOut {
					for _, routes := range destinations {
						for _, entry := range routes {
							entry.release()
						}
					}
				}
				r.ribOut = nil
			})
			source := netip.MustParseAddr("192.0.2.253")
			grSourceCommandRoute(t, r, source, 1280, 41, 8280, 80, []byte{0xff, 0xff, 0, 7})
			grSourceCommandRoute(t, r, source, 1280, 41, 8281, 81, nil)
			grSourceCommandRun(t, r, "retain-routes", 1280, 41, "on-down", "families", "ipv4/unicast")
			grSourceCommandRun(t, r, "mark-stale", 1280, 41, "restart-time", "0")
			grAuxSent(t, r, !native, 2280, 91, 1280, 41, 9280, 180, 80)
			r.updateRefuseHook = func(input rpc.UpdateRouteInput) error {
				if strings.Contains(input.Command, "80.0.0.0/8") {
					return rpc.CachedWriteNotDispatched(errors.New("original request was refused locally before dispatch"))
				}
				return nil
			}
			args := []string{"owner", "1280", "connection", "41", "family", "ipv4/unicast", "community", "ffff0007"}
			retire := func() error {
				_, _, err := r.handleCommand("request bgp rib source delete-with-community", "*", args)
				return err
			}
			require.Error(t, retire())
			ready := grAuxLeaseOutputs(t, r.sourceLeases, 8281)
			require.Len(t, ready, 1)
			readyBatch := ready[0]
			require.Equal(t, ribSourceSentReady, grAuxBatchSnapshot(t, readyBatch)[0].outcome)

			grSourceCommandRoute(t, r, source, 1280, 41, 8282, 82, []byte{0xff, 0xff, 0, 7})
			grSourceCommandRun(t, r, "mark-stale", 1280, 41, "restart-time", "0")
			grAuxSent(t, r, !native, 2280, 91, 1280, 41, 9282, 182, 82)
			require.Error(t, retire())
			pending := grAuxLeaseOutputs(t, r.sourceLeases, 8281)
			require.Len(t, pending, 2)
			require.Same(t, readyBatch, pending[0])
			unknownBatch := pending[1]
			require.Equal(t, ribSourceSentUnknown, grAuxBatchSnapshot(t, unknownBatch)[0].outcome)

			grSourceCommandRoute(t, r, source, 1280, 41, 8283, 83, []byte{0xff, 0xff, 0, 7})
			grSourceCommandRun(t, r, "mark-stale", 1280, 41, "restart-time", "0")
			grAuxSent(t, r, !native, 2280, 91, 1280, 41, 9283, 183, 83)
			joined = false
			go func() { done <- retire() }()
			captured := selectedDispatchAwait(t, ctx, entered)
			require.Equal(t, uint64(2280), captured.DestinationOwner)
			require.Equal(t, uint64(91), captured.DestinationSession)
			held := grAuxLeaseOutputs(t, r.sourceLeases, 8281)
			require.Len(t, held, 3, "held output must not be pruned")
			completedBatch := held[2]
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8281))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8283))
			once.Do(func() { close(unblock) })
			require.Error(t, selectedDispatchAwait(t, ctx, done), "other original pending writes still refuse overall completion")
			joined = true
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 8283), "independent completed output releases only its own ended receipt")
			require.Empty(t, grAuxBatchSnapshot(t, completedBatch), "completed owned commands and metadata are cleared")
			for receipt := range map[uint64]bool{8280: true, 8281: true, 8282: true} {
				require.NotContains(t, grAuxLeaseOutputs(t, r.sourceLeases, receipt), completedBatch, "completed batch is detached even from live untouched references")
			}

			// Another successful cleanup must not accumulate history on 8281.
			grSourceCommandRoute(t, r, source, 1280, 41, 8284, 84, []byte{0xff, 0xff, 0, 7})
			grSourceCommandRun(t, r, "mark-stale", 1280, 41, "restart-time", "0")
			grAuxSent(t, r, !native, 2280, 91, 1280, 41, 9284, 184, 84)
			require.Error(t, retire(), "ready and UNKNOWN originals still refuse")
			require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 8284))
			remaining := grAuxLeaseOutputs(t, r.sourceLeases, 8281)
			require.Equal(t, []*ribSourceSentOutput{readyBatch, unknownBatch}, remaining)
			readyAttempt := grAuxBatchSnapshot(t, readyBatch)
			unknownAttempt := grAuxBatchSnapshot(t, unknownBatch)
			require.Len(t, readyAttempt, 1)
			require.Len(t, unknownAttempt, 1)
			require.Equal(t, ribSourceSentReady, readyAttempt[0].outcome)
			require.Equal(t, ribSourceSentUnknown, unknownAttempt[0].outcome)
			require.Contains(t, readyAttempt[0].input.Command, "80.0.0.0/8")
			require.Contains(t, unknownAttempt[0].input.Command, "82.0.0.0/8")
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8280))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8281))
			require.Zero(t, calls.count(rpc.SourceLeaseRelease, 8282))
			survivor := grSourceCommandRead(t, r, 1280, 81)
			require.Equal(t, uint64(41), survivor.SourceConnectionID())
			mu.Lock()
			observed := append([]string(nil), written...)
			mu.Unlock()
			require.Len(t, observed, 3, "UNKNOWN is not retried and locally refused Ready never dispatches")
			require.Contains(t, observed[0], "82.0.0.0/8")
			require.Contains(t, observed[1], "83.0.0.0/8")
			require.Contains(t, observed[2], "84.0.0.0/8")
		})
	}
}

// Snapshots hold only their own owner mutex, and are used after command join
// except the lease membership snapshot that deliberately observes held output.
func grAuxLeaseOutputs(t *testing.T, leases *ribSourceLeases, receipt uint64) []*ribSourceSentOutput {
	t.Helper()
	leases.mu.Lock()
	defer leases.mu.Unlock()
	lease := leases.leases[receipt]
	require.NotNil(t, lease)
	return append([]*ribSourceSentOutput(nil), lease.sentOutputs...)
}

func grAuxBatchSnapshot(t *testing.T, output *ribSourceSentOutput) []ribSourceSentAttempt {
	t.Helper()
	output.mu.Lock()
	defer output.mu.Unlock()
	require.Nil(t, output.busy, "attempt payload is inspected only after its actual command caller joined")
	return append([]ribSourceSentAttempt(nil), output.attempts...)
}
