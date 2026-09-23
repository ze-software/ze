//go:build integration && linux

// Design: docs/guide/l2tp.md -- kernel-to-PPP descriptor ownership

package l2tp

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"golang.org/x/sys/unix"
)

func ownedKernelResult(t *testing.T) (*kernelWorker, kernelSetupSucceeded) {
	t.Helper()
	fds := &pppSessionFDs{pppoxFD: -1, chanFD: -1, unitFD: -1}
	worker := &kernelWorker{
		logger: discardLoggerForTest(),
		sessions: map[sessionKey]*pppSessionFDs{{1, 2}: fds},
		ops: kernelOps{
			closeFD: unix.Close,
			sessionDelete: func(uint16, uint16) error { return nil },
		},
	}
	t.Cleanup(func() { worker.teardownSession(kernelTeardownEvent{localTID: 1, localSID: 2}) })
	for _, target := range []*int{&fds.pppoxFD, &fds.chanFD, &fds.unitFD} {
		fd, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
		if err != nil {
			t.Fatal(err)
		}
		*target = fd
	}
	return worker, kernelSetupSucceeded{localTID: 1, localSID: 2, fds: *fds, owner: fds, session: &L2TPSession{localSID: 2}}
}

// A natural PPP exit can recycle both descriptor numbers before kernel teardown.
// The worker must disconnect only the PPPoX transport, not the replacement files.
func TestKernelTeardownDoesNotCloseTransferredDescriptors(t *testing.T) {
	worker, event := ownedKernelResult(t)
	channel, unit, ok := worker.takePPPDescriptors(event)
	if !ok {
		t.Fatal("live setup was not transferred")
	}
	for _, fd := range []int{channel, unit} {
		file := ppp.NewFDFile(fd, "transferred-ppp")
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	source, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	if source != channel && source != unit {
		defer func() { _ = unix.Close(source) }()
	}
	var replacements []*os.File
	for _, fd := range []int{channel, unit} {
		if source != fd {
			if err := unix.Dup3(source, fd, unix.O_CLOEXEC); err != nil {
				t.Fatal(err)
			}
		}
		file := os.NewFile(uintptr(fd), "replacement")
		t.Cleanup(func() { _ = file.Close() })
		replacements = append(replacements, file)
	}
	worker.teardownSession(kernelTeardownEvent{localTID: 1, localSID: 2})
	for _, file := range replacements {
		if _, err := file.Write([]byte{1}); err != nil {
			t.Fatalf("kernel teardown closed a recycled descriptor: %v", err)
		}
	}
	if _, _, ok := worker.takePPPDescriptors(event); ok {
		t.Fatal("a stale success transferred descriptors twice")
	}
}

func TestKernelTransferRejectsReplacedOwner(t *testing.T) {
	worker, event := ownedKernelResult(t)
	replacement := event.fds
	worker.sessions[sessionKey{1, 2}] = &replacement
	if _, _, ok := worker.takePPPDescriptors(event); ok {
		t.Fatal("old setup claimed a replacement's descriptors")
	}
	if _, err := unix.Write(replacement.chanFD, []byte{1}); err != nil {
		t.Fatalf("stale claim damaged replacement channel: %v", err)
	}
}

func TestKernelSuccessCancellationReleasesClaimedDescriptors(t *testing.T) {
	worker, event := ownedKernelResult(t)
	stop := make(chan struct{})
	close(stop)
	fake := newFakePPPDriver()
	fake.sessionsIn = make(chan ppp.StartSession)
	reactor := &l2tpReactor{
		logger: discardLoggerForTest(), kernelWorker: worker, pppDriver: fake, stop: stop,
		tunnelsByLocalID: map[uint16]*L2TPTunnel{1: {sessions: map[uint16]*L2TPSession{2: event.session}}},
	}
	reactor.handleKernelSuccess(event)
	for _, fd := range []int{event.fds.chanFD, event.fds.unitFD} {
		if _, err := unix.Write(fd, []byte{1}); !errors.Is(err, unix.EBADF) {
			t.Fatalf("cancelled transfer retained descriptor %d: %v", fd, err)
		}
	}
}

func TestKernelSuccessForRemovedSessionRetainsWorkerOwnership(t *testing.T) {
	worker, event := ownedKernelResult(t)
	fake := newFakePPPDriver()
	reactor := &l2tpReactor{
		logger: discardLoggerForTest(), kernelWorker: worker, pppDriver: fake,
		tunnelsByLocalID: map[uint16]*L2TPTunnel{1: {sessions: map[uint16]*L2TPSession{2: {localSID: 2}}}},
	}
	reactor.handleKernelSuccess(event)
	select {
	case <-fake.sessionsIn:
		t.Fatal("old setup was delivered to a replacement logical session")
	default:
	}
	worker.teardownSession(kernelTeardownEvent{localTID: 1, localSID: 2})
	for _, fd := range []int{event.fds.chanFD, event.fds.unitFD} {
		if _, err := unix.Write(fd, []byte{1}); !errors.Is(err, unix.EBADF) {
			t.Fatalf("worker failed to release unclaimed descriptor %d: %v", fd, err)
		}
	}
}

// Reporting can block on either queue while the reactor claims an earlier
// setup. Neither queue may keep the ownership mutex locked.
func TestKernelClaimProceedsWhileReportingBlocked(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "error"
		}
		t.Run(name, func(t *testing.T) {
			worker, event := ownedKernelResult(t)
			worker.tunnels = map[uint16]*kernelTunnelState{1: {localTID: 1, connFD: -1, sessionCount: 1}}
			failure := errors.New("session setup refused")
			created := make(chan struct{})
			worker.ops.sessionCreate = func(sessionCreateParams) error {
				close(created)
				if fail {
					return failure
				}
				return nil
			}
			worker.ops.pppSetup = func(kernelSetupEvent) (pppSessionFDs, error) {
				return pppSessionFDs{pppoxFD: -1, chanFD: -1, unitFD: -1}, nil
			}
			worker.ops.tunnelDelete = func(uint16) error { return nil }
			successes := make(chan kernelSetupSucceeded)
			failures := make(chan kernelSetupFailed)
			worker.successCh, worker.errCh = successes, failures
			done := make(chan struct{})
			go func() {
				defer close(done)
				worker.setupSession(kernelSetupEvent{localTID: 1, localSID: 3})
			}()
			<-created
			type claim struct {
				channel, unit int
				ok bool
			}
			claimed := make(chan claim, 1)
			go func() {
				channel, unit, ok := worker.takePPPDescriptors(event)
				claimed <- claim{channel, unit, ok}
			}()
			var result claim
			late := false
			select {
			case result = <-claimed:
			case <-time.After(time.Second):
				late = true
			}
			// Unblock the report even on failure, so both goroutines and
			// their owned descriptors can be released before the assertion.
			if fail {
				if report := <-failures; !errors.Is(report.err, failure) {
					t.Errorf("wrong setup failure: %v", report.err)
				}
			} else {
				<-successes
			}
			<-done
			if late {
				result = <-claimed
			}
			if result.ok {
				_ = ppp.NewFDFile(result.channel, "claimed-channel").Close()
				_ = ppp.NewFDFile(result.unit, "claimed-unit").Close()
			}
			if late || !result.ok {
				t.Fatalf("claim could not complete independently of reporting: late=%v acquired=%v", late, result.ok)
			}
		})
	}
}
