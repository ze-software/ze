// Design: docs/architecture/l2tp/followup-l2tp-call.md -- access ownership at bridge preparation
// Related: session.go -- sessionLease, serviceBridgePreparation

package pppoe

import (
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// fixtureRelayOpened supplies the same synthetic descriptor used by the old
// sink-only fixtures, through the real validated and serialized owner boundary.
// No fixture invokes an operating-system operation on this descriptor.
func fixtureRelayOpened(t *testing.T, descriptor int) dataplane.OpenedSession {
	t.Helper()
	opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, &fixtureSessionHandle{},
		&relayFixtureTransport{descriptor: descriptor})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := opened.Dataplane().Close(); err != nil {
			t.Error(err)
		}
		if err := opened.CloseTransport(); err != nil {
			t.Error(err)
		}
	})
	return opened
}

type relayFixtureTransport struct{ descriptor int }

func (owner *relayFixtureTransport) RelayDescriptor() (int, error) { return owner.descriptor, nil }
func (*relayFixtureTransport) Close() error                        { return nil }

type countedRelayLease struct {
	ppp.SessionHandle
	mu        sync.Mutex
	closes    int
	closeOnce sync.Once
	entered   chan struct{}
	release   chan struct{}
	err       error
}

func (lease *countedRelayLease) Close() error {
	lease.mu.Lock()
	lease.closes++
	lease.mu.Unlock()
	lease.closeOnce.Do(func() {
		if lease.entered != nil {
			close(lease.entered)
			<-lease.release
		}
	})
	return lease.err
}

func newPreparationOwner(t *testing.T, lease *countedRelayLease) (*InterfaceServer, dataplane.OpenedSession) {
	t.Helper()
	opened, err := dataplane.NewOpenedSession(&fixtureFrames{}, lease,
		&relayFixtureTransport{descriptor: 37})
	if err != nil {
		t.Fatal(err)
	}
	server := &InterfaceServer{sessions: newSessionTable("eth0", 10), logger: slog.Default()}
	if err := server.sessions.Add(&Session{SID: 7, State: StateSession}); err != nil {
		t.Fatal(err)
	}
	if !fixtureCurrentOpened(t, server, server.sessions.Lookup(7), opened) {
		t.Fatal("session owner was not admitted")
	}
	return server, opened
}

// TestRelayPreparationAndTeardownShareLease blocks the actual handle close,
// removes the admission concurrently, and proves the table is not held while
// both paths share exactly one release and its original failure.
func TestRelayPreparationAndTeardownShareLease(t *testing.T) {
	lease := &countedRelayLease{entered: make(chan struct{}), release: make(chan struct{}), err: errors.New("lease close failed")}
	server, opened := newPreparationOwner(t, lease)
	preparation := &serviceBridgePreparation{sessions: server.sessions, sid: 7, opened: opened}
	prepared := make(chan error, 1)
	var workers sync.WaitGroup
	release := sync.OnceFunc(func() { close(lease.release) })
	t.Cleanup(func() { release(); workers.Wait() })
	workers.Go(func() { prepared <- preparation.PrepareBridge() })
	select {
	case <-lease.entered:
	case err := <-prepared:
		t.Fatalf("preparation returned without releasing the lease: %v", err)
	}
	removed := server.sessions.removeAdmitted(7)
	if removed.opened != opened {
		t.Fatal("teardown lost the exact admitted transport")
	}
	if removed.unusedLease == nil {
		t.Fatal("teardown lost the shared unused lease")
	}
	closed := make(chan error, 1)
	workers.Go(func() { closed <- removed.unusedLease.Close() })
	release()
	if err := <-prepared; !errors.Is(err, lease.err) {
		t.Errorf("preparation error = %v", err)
	}
	if err := <-closed; !errors.Is(err, lease.err) {
		t.Errorf("teardown error = %v", err)
	}
	if lease.closes != 1 {
		t.Errorf("handle closes = %d, want 1", lease.closes)
	}
	if err := removed.opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
}

// TestStaleRelayPreparationKeepsReplacement opens another admission under the
// same SID and proves a delayed worker cannot release the replacement's lease.
func TestStaleRelayPreparationKeepsReplacement(t *testing.T) {
	oldLease := &countedRelayLease{}
	server, old := newPreparationOwner(t, oldLease)
	preparation := &serviceBridgePreparation{sessions: server.sessions, sid: 7, opened: old}
	server.closeAdmitted(server.sessions.Lookup(7))
	replacementLease := &countedRelayLease{}
	replacement, err := dataplane.NewOpenedSession(&fixtureFrames{}, replacementLease, &relayFixtureTransport{descriptor: 38})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.sessions.Add(&Session{SID: 7, State: StateSession}); err != nil {
		t.Fatal(err)
	}
	if !fixtureCurrentOpened(t, server, server.sessions.Lookup(7), replacement) {
		t.Fatal("replacement owner was not admitted")
	}
	if err := preparation.PrepareBridge(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("stale preparation = %v", err)
	}
	if replacementLease.closes != 0 {
		t.Fatal("stale preparation closed the replacement lease")
	}
	if err := replacement.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatal(err)
	}
	server.closeAdmitted(server.sessions.Lookup(7))
	if oldLease.closes != 1 {
		t.Errorf("original lease closes = %d, want 1", oldLease.closes)
	}
	if replacementLease.closes != 1 {
		t.Errorf("replacement lease closes = %d, want 1", replacementLease.closes)
	}
}

// TestAccessTeardownKeepsTransferredPPPLease gives the local lease to PPP,
// then removes access ownership. Only the PPP owner can close that lease.
func TestAccessTeardownKeepsTransferredPPPLease(t *testing.T) {
	lease := &countedRelayLease{}
	server, opened := newPreparationOwner(t, lease)
	if server.sessions.takePPPLease(server.sessions.Lookup(7)) == nil {
		t.Fatal("local PPP did not receive its lease")
	}
	server.closeAdmitted(server.sessions.Lookup(7))
	server.closeAdmitted(server.sessions.Lookup(7))
	if lease.closes != 0 {
		t.Fatal("access teardown closed a PPP-owned lease")
	}
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if lease.closes != 1 {
		t.Errorf("PPP lease closes = %d, want 1", lease.closes)
	}
}

// TestMissingAdmissionCleanupKeepsLiveOwner exercises the idempotent absence
// guards twice and proves they leave another admitted transport and lease live.
func TestMissingAdmissionCleanupKeepsLiveOwner(t *testing.T) {
	lease := &countedRelayLease{}
	server, opened := newPreparationOwner(t, lease)
	server.closeAdmitted(nil)
	server.closeTransport(server.sessions.transport(8))
	server.closeAdmitted(nil)
	if lease.closes != 0 {
		t.Fatal("missing admission cleanup closed a live lease")
	}
	if err := opened.WithDescriptor(func(int) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if server.sessions.Lookup(7) == nil {
		t.Fatal("missing admission cleanup removed another session")
	}
	server.closeAdmitted(server.sessions.Lookup(7))
	if lease.closes != 1 {
		t.Errorf("live lease closes = %d, want 1", lease.closes)
	}
}
