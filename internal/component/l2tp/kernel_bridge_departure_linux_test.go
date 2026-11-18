//go:build linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- worker custody of deferred transport close.
package l2tp

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/report"
)

// These are private physical-outcome ports, not PPP native grants. The
// production OpenedSession mints the retained token and caches the real call
// to this exact file owner. The worker runs its production cleanup producer.
type workerDepartureFrames struct {
	ppp.SessionHandle
	bytes.Buffer
}

func (frames *workerDepartureFrames) Control() io.ReadWriter { return &frames.Buffer }
func (*workerDepartureFrames) Opened() io.Reader             { return nil }
func (*workerDepartureFrames) InterruptRead() error          { return nil }
func (*workerDepartureFrames) Close() error                  { return nil }

type workerDepartureTransport struct {
	original   *os.File
	cause      error
	panicClose bool
	closes     int
}

func (transport *workerDepartureTransport) WithOriginalFile(use func(*os.File) error) error {
	return use(transport.original)
}
func (transport *workerDepartureTransport) Close() error {
	transport.closes++
	err := transport.original.Close()
	if transport.panicClose {
		panic(errors.Join(err, transport.cause))
	}
	return errors.Join(err, transport.cause)
}

// A pending departure is fulfilled by the final actual token release reached
// through worker teardown, including a physical owner error or panic. Removing
// errors.Join(... owners.relayFile.Close()) from closeOriginalKernelPPP must
// lose this exact cause and fail these assertions, not merely a lease test.
func TestWorkerBridgeDepartureRetainsPhysicalCloseFailure(t *testing.T) {
	for _, panics := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic"}[panics], func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			original, peer, err := os.Pipe()
			require.NoError(t, err)
			t.Cleanup(func() {
				if _, err := original.Stat(); err == nil {
					require.NoError(t, original.Close())
				}
			})
			t.Cleanup(func() { require.NoError(t, peer.Close()) })
			cause := errors.New("exact deferred physical transport close failed")
			physical := &workerDepartureTransport{original: original, cause: cause, panicClose: panics}
			frames := &workerDepartureFrames{}
			opened, err := dataplane.NewOpenedSession(frames, frames, physical)
			require.NoError(t, err)
			lease, err := opened.RetainOriginalFile()
			require.NoError(t, err)
			require.NoError(t, lease.WithOriginalFile(func(actual *os.File) error { require.Same(t, original, actual); return nil }))
			require.Error(t, opened.CloseTransport(), "this producer requests departure while the exact bridge token is live")
			require.Equal(t, 0, physical.closes)
			_, err = original.Stat()
			require.NoError(t, err)
			bridge := &cleanupBridgeModel{pending: true, failure: errors.New("original bridge still pending")}
			owners := &kernelPPPOwners{bridge: bridge, relayFile: lease}
			key := sessionKey{tunnelID: 601, sessionID: 602}
			tunnel := &kernelTunnelState{localTID: key.tunnelID, connFD: -1, sessionCount: 1}
			owned := &pppSessionFDs{ppp: owners, pppoxFD: -1, chanFD: 772, unitFD: -1, tunnel: tunnel, bridged: true}
			var logs bytes.Buffer
			worker := newKernelWorker(kernelOps{
				closePPP: func(resources *kernelPPPOwners, transport *os.File, _ [3]int) [3]error {
					return closeOriginalKernelPPP(resources, transport, (*os.File).Close)
				},
				sessionDelete: func(tid, sid uint16) error {
					require.Equal(t, key.tunnelID, tid)
					require.Equal(t, key.sessionID, sid)
					return nil
				},
			}, nil, nil, slog.New(slog.NewTextHandler(&logs, nil)))
			worker.sessions[key], worker.tunnels[key.tunnelID] = owned, tunnel
			teardown := func() { worker.mu.Lock(); defer worker.mu.Unlock(); worker.teardownSessionFDsLocked(key, owned) }
			teardown()
			require.Same(t, lease, owners.relayFile)
			require.Same(t, owners, owned.ppp)
			require.Equal(t, 0, physical.closes)
			require.NoError(t, owned.closeErr)
			bridge.pending, bridge.failure = false, nil
			teardown()
			require.Equal(t, 1, physical.closes)
			require.ErrorIs(t, owned.closeErr, cause)
			require.Same(t, owned, worker.sessions[key], "terminal physical cause must retain its report owner")
			require.Nil(t, owners.relayFile)
			require.Nil(t, owners.bridge)
			require.ErrorIs(t, lease.Close(), cause)
			require.ErrorIs(t, opened.CloseTransport(), cause)
			_, err = original.Stat()
			require.ErrorIs(t, err, os.ErrClosed)
			require.Len(t, report.Warnings(), 1)
			require.Contains(t, report.Warnings()[0].Message, cause.Error())
			require.Contains(t, report.Warnings()[0].Subject, kernelCleanupSessionClose)
			teardown()
			require.Equal(t, 1, physical.closes, "consumed original physical file must never be retried")
			require.ErrorIs(t, owned.closeErr, cause)
			require.Len(t, report.Warnings(), 1)
			require.Equal(t, 1, strings.Count(logs.String(), "L2TP session descriptor close is unconfirmed"), "worker must report the physical close once")
			require.NoError(t, frames.Close())
		})
	}
}
