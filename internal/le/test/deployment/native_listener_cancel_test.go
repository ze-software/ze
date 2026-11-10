//go:build linux || darwin

// Design: docs/functional-tests.md -- native proof cancellation owns daemon descendants.
package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNativeListenerStartupCancellationJoinsDescendants reaches the actual
// native PPP observer's listener wait. A real daemon descendant publishes its
// socket before cancellation; no listener log is fabricated. A per-run wait
// boundary rendezvous pins cancellation after step's root check. The observer must
// return cancellation without a proven verdict or starting xl2tpd, with its
// process watcher joined and output descriptor closed. EOF additionally checks
// that the descendant no longer holds its socket.
// MUTATION: detach nsCommand from the root or omit observe's closeNative defer.
func TestNativeListenerStartupCancellationJoinsDescendants(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // The test owns this listening socket.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	peerRecord := filepath.Join(work, "peer-started")
	t.Setenv("NATIVE_LISTENER_SELF", self)
	t.Setenv("NATIVE_LISTENER_PEER", peerRecord)
	t.Setenv("SCALE_LIFETIME_ADDRESS", listener.Addr().String())
	// Only privileged namespace listing and execution are replaced. Empty
	// baseline listings are valid; the daemon runs the existing real descendant
	// fixture, and any later peer attempt leaves an observable record.
	const ip = `#!/bin/sh
if [ "$1" != netns ] || [ "$2" != exec ]; then exit 93; fi
shift 3
case "$1" in
ip) exit 0 ;;
owned-listener-daemon)
  export SCALE_LIFETIME_ROLE=parent
  exec "$NATIVE_LISTENER_SELF" -test.run='^TestScaleLifetimeProcess$' ;;
xl2tpd) : > "$NATIVE_LISTENER_PEER"; exit 94 ;;
*) exit 95 ;;
esac
`
	if err := os.WriteFile(filepath.Join(work, "ip"), []byte(ip), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", work+string(os.PathListSeparator)+os.Getenv("PATH"))
	foreign := filepath.Join(work, "foreign-owner")
	if err := os.WriteFile(foreign, []byte("preserve\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := os.Stat(foreign)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	proof := NewL2TPPPP(work)
	proof.Progress = io.Discard
	proof.ListenerWait = 30 * time.Second
	entered := make(chan *running, 1)
	proof.wait = func(seen *collector, wanted, fatal []string, process *running, timeout time.Duration) (bool, error) {
		entered <- process
		<-ctx.Done()
		return awaitAll(seen, wanted, fatal, process, timeout)
	}
	observation := nativeListenerObservation{done: make(chan struct{})}
	go observation.run(ctx, proof, work)
	// MUST cancel and join the observer before releasing its fixture directory.
	defer func() {
		cancel()
		select {
		case <-observation.done:
		case <-time.After(5 * time.Second):
			t.Error("native listener observer did not join after fixture release")
		}
	}()
	owned := acceptScaleLifetime(t, listener)
	// Cancellation unblocks the wait wrapper. Closing the socket then releases
	// the fixture even if production cancellation is broken, before the join.
	defer func() {
		cancel()
		owned.Close() //nolint:errcheck // Owned test socket; EOF is asserted below.
	}()
	var pid int
	if err := json.NewDecoder(owned).Decode(&pid); err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatalf("daemon descendant did not identify its startup: %d", pid)
	}
	var process *running
	select {
	case process = <-entered:
	case <-observation.done:
		t.Fatalf("observer returned before its listener wait: %v", observation.err)
	case <-time.After(5 * time.Second):
		t.Fatal("observer did not reach its listener wait after daemon startup")
	}
	// MUST reclaim captured resources even if an omitted producer close makes
	// the assertions below fail. closeNative shares the owner's sole reaper.
	defer process.closeNative() //nolint:errcheck // The producer's cleanup result is asserted through observation.err.
	cancel()
	select {
	case <-observation.done:
	case <-time.After(5 * time.Second):
		t.Fatal("listener startup ignored root cancellation")
	}
	if !errors.Is(observation.err, context.Canceled) {
		t.Errorf("native observer lost startup cancellation: %v", observation.err)
	}
	if observation.report.Proven {
		t.Error("canceled listener startup reported proven")
	}
	select {
	case <-process.done:
	default:
		t.Error("observer returned before its sole process watcher joined")
	}
	if _, err := process.read.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("observer returned before closing its owned output descriptor: %v", err)
	}
	// EOF supplements the already-observed watcher/descriptor closure; it is
	// not the proof that cleanup completed before the observer returned.
	if err := owned.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := owned.Read(one[:]); !errors.Is(err, io.EOF) {
		t.Errorf("daemon descendant retained its socket after cleanup: %v", err)
	}
	if _, err := os.Stat(peerRecord); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("startup advanced to peer execution or its absence is unknown: %v", err)
	}
	after, err := os.Stat(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(identity, after) {
		t.Error("observer replaced the foreign owner's sentinel")
	}
	body, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "preserve\n" {
		t.Errorf("observer changed foreign bytes: %q", body)
	}
}

// nativeListenerObservation publishes one observer result when done closes.
// Its zero value is uninitialized; the test supplies done before starting run.
type nativeListenerObservation struct {
	done   chan struct{}
	report L2TPPPPReport
	err    error
}

func (o *nativeListenerObservation) run(ctx context.Context, proof *L2TPPPP, work string) {
	defer close(o.done)
	o.report, o.err = proof.observe(ctx, L2TPPPPReport{}, "owned-listener-daemon", work)
}
