// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- native publication fence
// Related: pppoe_admission.go -- existing registry's short admission boundary

package iface

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

type gatedAdmissionWriter struct {
	logicalNativeWriter
	entered chan struct{}
	release chan struct{}
	mu      *sync.Mutex
}

func (w *gatedAdmissionWriter) SetMTU(context.Context, int) error {
	if !w.mu.TryLock() {
		panic("registry lock spans native call")
	}
	w.mu.Unlock()
	close(w.entered)
	<-w.release
	return ErrNativePPPPending
}

// TestPPPoEAdmissionWaitsForActualWorkerOutcome pauses a real worker inside its
// original writer while cleanupErr still says nothing. Reconcile must not use
// that stale observation to start a new Dial. The desired resource-free client
// remains published, and the SAME registry retains original custody across
// another reconcile. Terminal cleanup then permits a useful replacement,
// proving the fence does not reject all work.
func TestPPPoEAdmissionWaitsForActualWorkerOutcome(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		var mu sync.Mutex
		writer := &gatedAdmissionWriter{entered: make(chan struct{}), release: make(chan struct{}), mu: &mu}
		pending := true
		cleanupCalls := 0
		originalDialer := &nativeConsumerDialer{session: PPPoESession{
			NativeWriter: writer, NegMTU: 1492,
			Cleanup: func() error {
				if !mu.TryLock() {
					t.Fatal("registry lock spans cleanup callback")
				}
				mu.Unlock()
				cleanupCalls++
				if pending {
					return ErrNativePPPPending
				}
				return nil
			},
		}}
		entry := pppoeClientEntry{Name: "old", Timing: testPPPoETiming}
		original := NewPPPoEClient(pppoeClientConfigFrom(&entry), originalDialer, log)
		active := map[string]*PPPoEClient{"old": original}
		original.registry = &pppoeClientSet{mu: &mu, active: active}
		released := false
		t.Cleanup(func() {
			if !released {
				close(writer.release)
			}
			pending = false
			if err := original.Stop(); err != nil {
				t.Error(err)
			}
			if replacement := active["new"]; replacement != nil {
				if err := replacement.Stop(); err != nil {
					t.Error(err)
				}
			}
		})
		original.Start()
		<-writer.entered
		original.mu.Lock()
		stale := original.cleanupErr
		original.mu.Unlock()
		if stale != nil {
			t.Fatalf("control already published cleanup: %v", stale)
		}
		newWriter := &nativeConsumerWriter{}
		newDialer := &nativeConsumerDialer{session: PPPoESession{
			NativeWriter: newWriter, NegMTU: 1492, Done: make(chan struct{}),
			Cleanup: func() error { return nil },
		}}
		saved := pppoeDialerVar
		pppoeDialerVar = newDialer
		defer func() { pppoeDialerVar = saved }()
		cfg := &ifaceConfig{PPPoEClient: []pppoeClientEntry{entry, {Name: "new", Timing: testPPPoETiming}}}
		reconcilePPPoEClients(cfg, active, &mu, log, nil)
		synctest.Wait()
		if active["new"] == nil {
			t.Fatal("transient work dropped desired client config")
		}
		if newDialer.calls != 0 {
			t.Fatal("new Dial overlapped unresolved original call")
		}
		close(writer.release)
		released = true
		synctest.Wait()
		if cleanupCalls != 1 {
			t.Fatalf("real worker cleanup calls = %d", cleanupCalls)
		}
		if err := original.Stop(); !errors.Is(err, ErrNativePPPPending) {
			t.Fatalf("stop lost Pending: %v", err)
		}
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: cfg.PPPoEClient[1:]}, active, &mu, log, nil)
		if active["old"] != original {
			t.Fatal("restart reconciliation lost original custody")
		}
		if newDialer.calls != 0 {
			t.Fatal("restart admitted native Dial during Pending")
		}
		pending = false
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: cfg.PPPoEClient[1:]}, active, &mu, log, nil)
		// sleep(test-clock): release the already-published client's first bounded
		// reconnect wait; no second config event creates or starts that client.
		time.Sleep(testPPPoETiming.ReconnectDelayMin)
		synctest.Wait()
		if active["old"] != nil {
			t.Fatal("terminal original still owns registry entry")
		}
		if active["new"] == nil {
			t.Fatal("terminal cleanup failed to admit replacement")
		}
		if newDialer.calls != 1 {
			t.Fatalf("replacement Dial = %d", newDialer.calls)
		}
		if len(newWriter.calls) != 2 {
			t.Fatalf("replacement did no useful MTU/up work: %+v", newWriter.calls)
		}
		if err := active["new"].Stop(); err != nil {
			t.Fatal(err)
		}
	})
}
