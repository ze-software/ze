//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// adoptRecorder captures the socket-hook calls the kernel worker makes, in order,
// so a test can assert both that they happen and when.
type adoptRecorder struct {
	mu       sync.Mutex
	adopted  []int // fds passed to adopt, in call order
	released []uint16
	order    []string
	failWith error
}

func (a *adoptRecorder) adopt(_ uint16, fd int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failWith != nil {
		a.order = append(a.order, "adopt-fail")
		return a.failWith
	}
	a.adopted = append(a.adopted, fd)
	a.order = append(a.order, "adopt")
	return nil
}

func (a *adoptRecorder) release(tid uint16) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.released = append(a.released, tid)
	a.order = append(a.order, "release")
}

func (a *adoptRecorder) snapshot() ([]int, []uint16, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]int(nil), a.adopted...), append([]uint16(nil), a.released...), append([]string(nil), a.order...)
}

// connFDOps returns kernelOps whose tunnelCreate hands back a fixed connFD and
// which records closeFD calls, so a test can order release against close.
func connFDOps(connFD int, closes *[]int, mu *sync.Mutex) kernelOps {
	return kernelOps{
		tunnelCreate: func(_, _ uint16, _ int, _ netip.AddrPort) (int, error) { return connFD, nil },
		tunnelDelete: func(uint16) error { return nil },
		sessionCreate: func(sessionCreateParams) error {
			return nil
		},
		sessionDelete: func(uint16, uint16) error { return nil },
		pppSetup: func(kernelSetupEvent) (pppSessionFDs, error) {
			return pppSessionFDs{pppoxFD: 10, chanFD: 11, unitFD: 12}, nil
		},
		closeFD: func(fd int) error {
			mu.Lock()
			defer mu.Unlock()
			*closes = append(*closes, fd)
			return nil
		},
		probeModules: func() error { return nil },
	}
}

// VALIDATES: on kernel tunnel create the worker hands the CONNECTED socket fd to
// the listener (adopt), and on tunnel delete it tells the listener to stop
// reading (release) BEFORE closing that fd.
// PREVENTS: the regression this wiring exists to fix -- the connected socket
// outranks the listener for the peer's datagrams, so an unadopted tunnel makes ze
// deaf to that peer's control plane (no second ICRQ, CDN, HELLO or StopCCN). The
// release-before-close ordering prevents the reader waking on a stale descriptor.
func TestKernelWorkerAdoptsAndReleasesTunnelSocket(t *testing.T) {
	const connFD = 4242
	var closes []int
	var closeMu sync.Mutex

	rec := &adoptRecorder{}
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(connFDOps(connFD, &closes, &closeMu), errCh, successCh, discardLogger())
	w.setSocketHooks(rec.adopt, rec.release)
	w.Start()
	defer w.Stop()

	w.Enqueue(kernelSetupEvent{
		localTID: 77, remoteTID: 88, localSID: 1, remoteSID: 2,
		peerAddr: netip.MustParseAddrPort("10.0.0.1:1701"), socketFD: 3, lnsMode: true,
	})

	select {
	case <-successCh:
	case f := <-errCh:
		t.Fatalf("setup failed: %v", f.err)
	case <-time.After(5 * time.Second):
		t.Fatal("no kernel setup result")
	}

	adopted, _, _ := rec.snapshot()
	require.Equal(t, []int{connFD}, adopted,
		"the CONNECTED socket fd must be adopted, not the listener fd")

	// Tear the tunnel down and assert release happens before the fd is closed.
	w.teardownAll()

	deadline := time.Now().Add(5 * time.Second)
	for {
		_, released, order := rec.snapshot()
		if len(released) > 0 {
			require.Equal(t, []uint16{77}, released)
			require.Equal(t, []string{"adopt", "release"}, order,
				"release must precede the close of the tunnel fd")
			closeMu.Lock()
			gotCloses := append([]int(nil), closes...)
			closeMu.Unlock()
			require.Contains(t, gotCloses, connFD, "tunnel fd must still be closed")
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("tunnel socket was never released")
		}
		// poll interval; the loop returns as soon as the worker records the release
		time.Sleep(10 * time.Millisecond)
	}
}

// VALIDATES: when the listener cannot adopt the tunnel socket, setup is rolled
// back -- the tunnel is deleted, the fd closed, and the failure reported -- rather
// than leaving a kernel tunnel whose control plane nobody can read.
// PREVENTS: a half-live tunnel that silently swallows the peer's control
// messages, which is harder to diagnose than a clean setup failure.
func TestKernelWorkerRollsBackWhenAdoptFails(t *testing.T) {
	const connFD = 5150
	var closes []int
	var closeMu sync.Mutex

	adoptErr := errors.New("listener stopped")
	rec := &adoptRecorder{failWith: adoptErr}
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(connFDOps(connFD, &closes, &closeMu), errCh, successCh, discardLogger())
	w.setSocketHooks(rec.adopt, rec.release)
	w.Start()
	defer w.Stop()

	w.Enqueue(kernelSetupEvent{
		localTID: 91, remoteTID: 92, localSID: 5, remoteSID: 6,
		peerAddr: netip.MustParseAddrPort("10.0.0.2:1701"), socketFD: 3, lnsMode: true,
	})

	select {
	case f := <-errCh:
		require.ErrorIs(t, f.err, adoptErr)
		require.Equal(t, uint16(91), f.localTID)
	case <-successCh:
		t.Fatal("setup reported success despite an unreadable tunnel socket")
	case <-time.After(5 * time.Second):
		t.Fatal("no kernel setup result")
	}

	closeMu.Lock()
	gotCloses := append([]int(nil), closes...)
	closeMu.Unlock()
	require.Contains(t, gotCloses, connFD, "rollback must close the tunnel fd")
}
