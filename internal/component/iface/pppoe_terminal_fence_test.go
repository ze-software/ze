// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- terminal admission fence
// Related: pppoe_client.go -- stopped, Start, Stop and reconciliation

package iface

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
)

// TestPPPoEStoppedPredicateAlwaysObservesClosure checks every observation of a
// closed stop signal and repeated Start/Stop on the same actual client. It does
// not accept a lucky single select between two ready channels as a fence.
func TestPPPoEStoppedPredicateAlwaysObservesClosure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dialer := &nativeConsumerDialer{}
		client := NewPPPoEClient(PPPoEClientConfig{Name: "wan"}, dialer,
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err := client.Stop(); err != nil {
			t.Fatal(err)
		}
		for range 1024 {
			if !client.stopped() {
				t.Fatal("closed stop signal reported running")
			}
			client.Start()
		}
		synctest.Wait()
		if err := client.Stop(); err != nil {
			t.Fatal(err)
		}
		if dialer.calls != 0 {
			t.Fatalf("terminal client invoked Dial %d times", dialer.calls)
		}
	})
}

// TestPPPoEReconcileNeverRestartsStoppedInstance proves a desired entry replaces
// a terminal instance rather than resurrecting it. The independently recorded
// replacement does useful work through the actual worker and its own writer.
func TestPPPoEReconcileNeverRestartsStoppedInstance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		entry := pppoeClientEntry{Name: "wan", Timing: testPPPoETiming}
		originalDialer := &nativeConsumerDialer{}
		original := NewPPPoEClient(pppoeClientConfigFrom(&entry), originalDialer, log)
		if err := original.Stop(); err != nil {
			t.Fatal(err)
		}
		writer := &nativeConsumerWriter{}
		replacementDialer := &nativeConsumerDialer{session: PPPoESession{
			NativeWriter: writer, NegMTU: 1492, Done: make(chan struct{}),
			Cleanup: func() error { return nil },
		}}
		saved := pppoeDialerVar
		pppoeDialerVar = replacementDialer
		defer func() { pppoeDialerVar = saved }()
		active := map[string]*PPPoEClient{"wan": original}
		var mu sync.Mutex
		reconcilePPPoEClients(&ifaceConfig{PPPoEClient: []pppoeClientEntry{entry}}, active, &mu, log, nil)
		synctest.Wait()
		if active["wan"] == original {
			t.Fatal("reconcile retained terminal instance")
		}
		if originalDialer.calls != 0 {
			t.Fatal("reconcile resurrected original Dial")
		}
		if replacementDialer.calls != 1 {
			t.Fatalf("replacement Dial calls = %d", replacementDialer.calls)
		}
		if len(writer.calls) != 2 {
			t.Fatalf("replacement did not apply MTU/admin-up: %+v", writer.calls)
		}
		if err := active["wan"].Stop(); err != nil {
			t.Fatal(err)
		}
	})
}
