//go:build linux

package l2tp

import (
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeKernelOps records calls and lets tests inject failures at any
// step of the setup sequence.
type fakeKernelOps struct {
	mu sync.Mutex

	tunnelCreated []uint16
	tunnelDeleted []uint16
	sessionAdded  []sessionCreateParams
	sessionGone   []sessionKey
	pppSetups     []kernelSetupEvent
	closes        []int

	failTunnelCreate  error
	failSessionCreate error
	failPPPSetup      error

	// pppSetupResult overrides the default (pppoxFD=10, chanFD=11, unitFD=12).
	pppSetupResult *pppSessionFDs
}

func (f *fakeKernelOps) ops() kernelOps {
	return kernelOps{
		tunnelCreate: func(local, remote uint16, fd int, _ netip.AddrPort) (int, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failTunnelCreate != nil {
				return -1, f.failTunnelCreate
			}
			f.tunnelCreated = append(f.tunnelCreated, local)
			return -1, nil
		},
		tunnelDelete: func(local uint16) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.tunnelDeleted = append(f.tunnelDeleted, local)
			return nil
		},
		sessionCreate: func(p sessionCreateParams) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failSessionCreate != nil {
				return f.failSessionCreate
			}
			f.sessionAdded = append(f.sessionAdded, p)
			return nil
		},
		sessionDelete: func(tid, sid uint16) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.sessionGone = append(f.sessionGone, sessionKey{tunnelID: tid, sessionID: sid})
			return nil
		},
		pppSetup: func(ev kernelSetupEvent) (pppSessionFDs, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failPPPSetup != nil {
				return pppSessionFDs{}, f.failPPPSetup
			}
			f.pppSetups = append(f.pppSetups, ev)
			if f.pppSetupResult != nil {
				return *f.pppSetupResult, nil
			}
			return pppSessionFDs{pppoxFD: 10, chanFD: 11, unitFD: 12, unitNum: 0}, nil
		},
		closeFD: func(fd int) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.closes = append(f.closes, fd)
			return nil
		},
		probeModules: func() error { return nil },
	}
}

// discardLogger returns a logger that writes to io.Discard.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestWorker builds a kernelWorker around the given fake ops and
// returns (worker, errCh). Caller must Stop the worker. successCh is
// allocated here so callers wanting to assert success events can receive
// on it; exposed via newTestWorkerWithSuccess.
func newTestWorker(t *testing.T, fake *fakeKernelOps) (*kernelWorker, chan kernelSetupFailed) {
	t.Helper()
	w, errCh, _ := newTestWorkerWithSuccess(t, fake)
	return w, errCh
}

// newTestWorkerWithSuccess is like newTestWorker but also returns the
// success channel. Tests that assert kernelSetupSucceeded events receive
// on the returned successCh.
func newTestWorkerWithSuccess(t *testing.T, fake *fakeKernelOps) (*kernelWorker, chan kernelSetupFailed, chan kernelSetupSucceeded) {
	t.Helper()
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(fake.ops(), errCh, successCh, discardLogger())
	w.Start()
	t.Cleanup(w.Stop)
	return w, errCh, successCh
}

// setupEvent builds a synthetic kernelSetupEvent with sensible defaults.
func setupEvent(tid, sid uint16) kernelSetupEvent {
	return kernelSetupEvent{
		localTID:  tid,
		remoteTID: tid + 1000,
		peerAddr:  netip.MustParseAddrPort("192.0.2.1:1701"),
		localSID:  sid,
		remoteSID: sid + 1000,
		socketFD:  7,
		lnsMode:   true,
	}
}

// waitFor polls until cond returns true, or fails the test after 2s.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting: %s", msg)
}

func TestKernelWorkerSetupSequence(t *testing.T) {
	// VALIDATES: AC-3, AC-5, AC-8, AC-9 -- a successful setup creates
	// kernel tunnel, kernel session, and PPP fds in order.
	// PREVENTS: missing steps silently break kernel data plane.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))

	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.pppSetups) == 1
	}, "pppSetup called")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.tunnelCreated) != 1 || fake.tunnelCreated[0] != 100 {
		t.Fatalf("tunnelCreated = %v, want [100]", fake.tunnelCreated)
	}
	if len(fake.sessionAdded) != 1 {
		t.Fatalf("sessionAdded len = %d, want 1", len(fake.sessionAdded))
	}
	sa := fake.sessionAdded[0]
	if sa.tunnelID != 100 || sa.localSID != 1001 || sa.remoteSID != 2001 {
		t.Fatalf("sessionAdded[0] = %+v", sa)
	}
	if !sa.lnsMode {
		t.Fatal("sessionAdded[0].lnsMode = false, want true (LNS)")
	}
	// No teardowns on happy path.
	if len(fake.tunnelDeleted) != 0 || len(fake.sessionGone) != 0 {
		t.Fatalf("unexpected teardowns: tun=%v sess=%v", fake.tunnelDeleted, fake.sessionGone)
	}
}

func TestKernelWorkerIdempotentTunnel(t *testing.T) {
	// VALIDATES: AC-4 -- second session on the same tunnel does NOT
	// recreate the kernel tunnel.
	// PREVENTS: kernel EEXIST or double-create on multi-session tunnels.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))
	w.Enqueue(setupEvent(100, 1002))

	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sessionAdded) == 2
	}, "two sessions created")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.tunnelCreated) != 1 {
		t.Fatalf("tunnelCreated = %v, want exactly one", fake.tunnelCreated)
	}
}

func TestKernelWorkerTeardownOrder(t *testing.T) {
	// VALIDATES: AC-10 -- teardown closes fds in strict reverse order
	// (unit, channel, pppox) then deletes the kernel session.
	// PREVENTS: fd leak and premature session delete that could
	// confuse the kernel's l2tp module.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))
	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.pppSetups) == 1
	}, "setup done")

	w.Enqueue(kernelTeardownEvent{localTID: 100, localSID: 1001})
	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sessionGone) == 1
	}, "session deleted")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	// closeFD is called for unitFD(12), chanFD(11), pppoxFD(10) in that order.
	if len(fake.closes) != 3 {
		t.Fatalf("closes = %v, want 3 entries", fake.closes)
	}
	if fake.closes[0] != 12 || fake.closes[1] != 11 || fake.closes[2] != 10 {
		t.Fatalf("close order = %v, want [12 11 10]", fake.closes)
	}
}

func TestKernelWorkerTeardownLastSession(t *testing.T) {
	// VALIDATES: AC-11 -- kernel tunnel is deleted after the last
	// session on it is torn down.
	// PREVENTS: orphaned kernel tunnels on a heavily-churned LNS.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))
	w.Enqueue(setupEvent(100, 1002))
	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sessionAdded) == 2
	}, "two sessions up")

	w.Enqueue(kernelTeardownEvent{localTID: 100, localSID: 1001})
	// After one of two sessions tears down, tunnel must NOT be deleted.
	time.Sleep(10 * time.Millisecond)
	fake.mu.Lock()
	if len(fake.tunnelDeleted) != 0 {
		fake.mu.Unlock()
		t.Fatalf("tunnel deleted prematurely: %v", fake.tunnelDeleted)
	}
	fake.mu.Unlock()

	w.Enqueue(kernelTeardownEvent{localTID: 100, localSID: 1002})
	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.tunnelDeleted) == 1
	}, "tunnel deleted after last session")

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.tunnelDeleted[0] != 100 {
		t.Fatalf("tunnelDeleted = %v, want [100]", fake.tunnelDeleted)
	}
}

func TestKernelWorkerTeardownUnknownSession(t *testing.T) {
	// VALIDATES: teardown event for a session that was never set up is
	// a no-op (e.g., setup failed partway, reactor still emits teardown).
	// PREVENTS: panic or double-delete on racy teardown paths.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(kernelTeardownEvent{localTID: 42, localSID: 7})
	time.Sleep(10 * time.Millisecond)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sessionGone) != 0 || len(fake.closes) != 0 {
		t.Fatalf("unexpected activity: sessions=%v closes=%v", fake.sessionGone, fake.closes)
	}
}

func TestKernelWorkerPartialFailureTunnel(t *testing.T) {
	// VALIDATES: AC-13 -- tunnel create failure reports error to reactor.
	// PREVENTS: silent setup failure; session stuck in established state
	// without kernel resources.
	fake := &fakeKernelOps{failTunnelCreate: errors.New("boom")}
	w, errCh := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))

	select {
	case kerr := <-errCh:
		if kerr.localTID != 100 || kerr.localSID != 1001 {
			t.Fatalf("error IDs wrong: %+v", kerr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kernel error")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	// Tunnel create failed; nothing else should have run.
	if len(fake.sessionAdded) != 0 || len(fake.pppSetups) != 0 {
		t.Fatalf("partial setup continued: sessions=%v ppp=%v",
			fake.sessionAdded, fake.pppSetups)
	}
}

func TestKernelWorkerPartialFailureSession(t *testing.T) {
	// VALIDATES: AC-13 -- session create failure rolls back the freshly
	// created kernel tunnel.
	// PREVENTS: orphaned kernel tunnel when the first session fails.
	fake := &fakeKernelOps{failSessionCreate: errors.New("boom")}
	w, errCh := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kernel error")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.tunnelCreated) != 1 || len(fake.tunnelDeleted) != 1 {
		t.Fatalf("tunnel rollback missing: created=%v deleted=%v",
			fake.tunnelCreated, fake.tunnelDeleted)
	}
}

func TestKernelWorkerPartialFailurePPP(t *testing.T) {
	// VALIDATES: AC-13 -- pppSetup failure rolls back session + tunnel.
	// PREVENTS: orphaned kernel session when PPPoL2TP socket creation
	// or /dev/ppp setup fails.
	fake := &fakeKernelOps{failPPPSetup: errors.New("boom")}
	w, errCh := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kernel error")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sessionAdded) != 1 || len(fake.sessionGone) != 1 {
		t.Fatalf("session rollback missing: added=%v gone=%v",
			fake.sessionAdded, fake.sessionGone)
	}
	if len(fake.tunnelCreated) != 1 || len(fake.tunnelDeleted) != 1 {
		t.Fatalf("tunnel rollback missing: created=%v deleted=%v",
			fake.tunnelCreated, fake.tunnelDeleted)
	}
}

func TestKernelWorkerTeardownAll(t *testing.T) {
	// VALIDATES: AC-14 -- TeardownAll tears down every live session and
	// tunnel before the worker stops.
	// PREVENTS: leaked kernel resources on subsystem Stop().
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Enqueue(setupEvent(100, 1001))
	w.Enqueue(setupEvent(100, 1002))
	w.Enqueue(setupEvent(200, 3001))
	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.sessionAdded) == 3
	}, "all sessions up")

	w.TeardownAll()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sessionGone) != 3 {
		t.Fatalf("sessionGone len = %d, want 3", len(fake.sessionGone))
	}
	if len(fake.tunnelDeleted) != 2 {
		t.Fatalf("tunnelDeleted len = %d, want 2", len(fake.tunnelDeleted))
	}
	// 3 sessions * 3 fd closes each = 9.
	if len(fake.closes) != 9 {
		t.Fatalf("closes len = %d, want 9", len(fake.closes))
	}
}

func TestKernelWorkerStopIdempotent(t *testing.T) {
	// VALIDATES: Stop is safe to call more than once.
	// PREVENTS: close-of-closed-channel panic during subsystem unwind.
	fake := &fakeKernelOps{}
	w, _ := newTestWorker(t, fake)

	w.Stop()
	w.Stop() // must not panic
}

func TestKernelWorkerEmitsSucceeded(t *testing.T) {
	// VALIDATES: AC-1 -- after setupSession completes, the worker writes
	// a kernelSetupSucceeded event carrying the fds, IDs, lnsMode,
	// sequencing, and the proxy LCP bytes that arrived on the setup event.
	// PREVENTS: kernel setup succeeding silently with no downstream PPP
	// dispatch, leaving sessions established at L2TP but never reaching
	// LCP negotiation.
	fake := &fakeKernelOps{
		pppSetupResult: &pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
	}
	w, _, successCh := newTestWorkerWithSuccess(t, fake)

	ev := setupEvent(100, 1001)
	ev.sequencing = true
	ev.proxyInitialRecvLCPConfReq = []byte{0x01, 0x02, 0x03}
	ev.proxyLastSentLCPConfReq = []byte{0x04, 0x05}
	ev.proxyLastRecvLCPConfReq = []byte{0x06}
	w.Enqueue(ev)

	select {
	case succ := <-successCh:
		require.Equal(t, uint16(100), succ.localTID)
		require.Equal(t, uint16(1001), succ.localSID)
		require.True(t, succ.lnsMode)
		require.True(t, succ.sequencing)
		require.Equal(t, 30, succ.fds.pppoxFD)
		require.Equal(t, 31, succ.fds.chanFD)
		require.Equal(t, 32, succ.fds.unitFD)
		require.Equal(t, 7, succ.fds.unitNum)
		require.Equal(t, []byte{0x01, 0x02, 0x03}, succ.proxyInitialRecvLCPConfReq)
		require.Equal(t, []byte{0x04, 0x05}, succ.proxyLastSentLCPConfReq)
		require.Equal(t, []byte{0x06}, succ.proxyLastRecvLCPConfReq)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for kernelSetupSucceeded")
	}
}

func TestKernelWorkerSucceededCoexistsWithNilChannel(t *testing.T) {
	// VALIDATES: when successCh is nil (e.g., tests that only care about
	// failure paths), a successful setup does not panic and does not
	// block the worker's run loop.
	// PREVENTS: regressions in the existing teardown/failure tests that
	// still use newTestWorker (no successCh exposed).
	fake := &fakeKernelOps{}
	errCh := make(chan kernelSetupFailed, 4)
	w := newKernelWorker(fake.ops(), errCh, nil, discardLogger())
	w.Start()
	t.Cleanup(w.Stop)

	w.Enqueue(setupEvent(100, 1001))

	waitFor(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.pppSetups) == 1
	}, "pppSetup called even without successCh")
}
