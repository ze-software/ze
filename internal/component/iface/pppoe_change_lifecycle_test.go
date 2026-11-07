// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Session change ownership.
// Related: pppoe_changes_test.go -- Recording backend for the actual consumer.

package iface

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// TestPPPoEClientChangeRouteRetry proves a failed route write does not advance
// ownership or repeat successful address and MTU writes on the next record.
func TestPPPoEClientChangeRouteRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, PPPoEClientConfig{})
		var logs bytes.Buffer
		h.client.logger = slog.New(slog.NewTextHandler(&logs, nil))
		h.backend.fail = "route-add"
		next := changedResult("192.0.2.3", "192.0.2.9", 1400)
		h.changes <- next
		synctest.Wait()
		if len(h.ended) != 0 {
			t.Fatal("route failure ended the session")
		}
		if h.cleanups != 0 {
			t.Fatal("route failure released the unit")
		}
		for _, text := range []string{"level=WARN", "0.0.0.0/0", "192.0.2.9", "wan"} {
			if !strings.Contains(logs.String(), text) {
				t.Errorf("warning lacks %q: %s", text, logs.String())
			}
		}
		for _, call := range h.backend.calls {
			if call.op == "route-remove" {
				t.Fatal("failed replacement removed the owned route")
			}
		}
		h.backend.calls = nil
		h.backend.fail = ""
		h.changes <- next
		synctest.Wait()
		if len(h.backend.calls) != 2 {
			t.Fatalf("retry calls = %+v, want add and remove only", h.backend.calls)
		}
		if h.backend.calls[0].op != "route-add" {
			t.Fatal("retry did not add the new route first")
		}
		if h.backend.calls[0].peer != "192.0.2.9" {
			t.Fatal("retry added the wrong gateway")
		}
		if h.backend.calls[1].op != "route-remove" {
			t.Fatal("retry did not remove the previously owned route")
		}
		if h.backend.calls[1].peer != "192.0.2.1" {
			t.Fatal("failed attempt replaced route ownership")
		}
		h.backend.calls = nil
		h.changes <- next
		synctest.Wait()
		if len(h.backend.calls) != 0 {
			t.Fatalf("successful retry was applied again: %+v", h.backend.calls)
		}
	})
}

// changeDialer records real run-loop attempts inside a synctest bubble.
type changeDialer struct {
	session PPPoESession
	times   chan time.Time
}

func (d *changeDialer) Dial(_ PPPoEClientConfig, _ <-chan struct{}, _ *slog.Logger) (PPPoESession, error) {
	d.times <- time.Now()
	return d.session, nil
}

// TestPPPoEClientChangeReconnect drives Start through a fatal change and checks
// Cleanup precedes the next Dial and the existing reconnect delay is observed.
func TestPPPoEClientChangeReconnect(t *testing.T) {
	for _, operation := range []string{"mtu", "address-add"} {
		t.Run(operation, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				changes := make(chan PPPoEResult, 1)
				var cleanups atomic.Int32
				b := &changeBackend{}
				initial := changedResult("192.0.2.2", "192.0.2.1", 1492)
				d := &changeDialer{session: PPPoESession{
					SessionID: 42, UnitNum: 7, NegMTU: initial.NegMTU,
					LocalIP: initial.LocalIP, PeerIP: initial.PeerIP,
					Changes: changes, Done: make(chan struct{}), Cleanup: func() {
						b.fail = ""
						cleanups.Add(1)
					},
				}, times: make(chan time.Time, 4)}
				c := NewPPPoEClient(PPPoEClientConfig{Name: "wan", Timing: testPPPoETiming}, d, b, slog.New(slog.NewTextHandler(io.Discard, nil)))
				c.Start()
				t.Cleanup(c.Stop)
				synctest.Wait()
				<-d.times
				b.fail = operation
				failedAt := time.Now()
				changes <- changedResult("192.0.2.3", "192.0.2.1", 1400)
				synctest.Wait()
				if cleanups.Load() != 1 {
					t.Fatalf("cleanup before backoff = %d", cleanups.Load())
				}
				if len(d.times) != 0 {
					t.Fatal("reconnected without backoff")
				}
				// sleep(test-clock): Advance the real run loop's reconnect timer.
				time.Sleep(testPPPoETiming.ReconnectDelayMin)
				synctest.Wait()
				if len(d.times) != 1 {
					t.Fatalf("new dial attempts = %d, want 1", len(d.times))
				}
				if delay := (<-d.times).Sub(failedAt); delay != testPPPoETiming.ReconnectDelayMin {
					t.Fatalf("reconnect delay = %s", delay)
				}
			})
		})
	}
}

// TestPPPoEClientInitialRouteRetry requires an initial failed route install to
// remain unowned and retry on an otherwise unchanged completion record.
func TestPPPoEClientInitialRouteRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		changes := make(chan PPPoEResult, 1)
		initial := changedResult("192.0.2.2", "192.0.2.1", 1492)
		b := &changeBackend{fail: "route-add"}
		var logs bytes.Buffer
		cleanups := 0
		d := &fakeDialer{sess: PPPoESession{
			SessionID: 42, UnitNum: 7, LocalIP: initial.LocalIP,
			PeerIP: initial.PeerIP, NegMTU: initial.NegMTU,
			Changes: changes, Done: make(chan struct{}), Cleanup: func() { cleanups++ },
		}}
		c := NewPPPoEClient(PPPoEClientConfig{Name: "wan", Timing: testPPPoETiming}, d, b, slog.New(slog.NewTextHandler(&logs, nil)))
		c.Start()
		t.Cleanup(c.Stop)
		synctest.Wait()
		if c.status().State != "up" {
			t.Fatal("initial route failure ended PPP")
		}
		for _, text := range []string{"level=WARN", "0.0.0.0/0", "192.0.2.1", "wan"} {
			if !strings.Contains(logs.String(), text) {
				t.Errorf("initial warning lacks %q: %s", text, logs.String())
			}
		}
		b.fail = ""
		b.calls = nil
		changes <- initial
		synctest.Wait()
		if len(b.calls) != 1 {
			t.Fatalf("initial route retry calls = %+v", b.calls)
		}
		if b.calls[0].op != "route-add" {
			t.Fatalf("initial failure claimed a route: %+v", b.calls)
		}
		if cleanups != 0 {
			t.Fatal("initial route failure invoked Cleanup")
		}
	})
}

// heldChangeBackend pauses initial setup so two completed outcomes can overtake
// each other before the real runSession reaches its Changes receive.
type heldChangeBackend struct {
	*changeBackend
	release chan struct{}
}

func (b *heldChangeBackend) SetAdminUp(device string) error {
	<-b.release
	return b.changeBackend.SetAdminUp(device)
}

// TestPPPoEClientNewestPendingChange drives the real consumer after replacement
// of its unread one-slot result and requires only the newest pair and MTU.
func TestPPPoEClientNewestPendingChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		changes := make(chan PPPoEResult, 1)
		initial := changedResult("192.0.2.2", "192.0.2.1", 1492)
		b := &heldChangeBackend{changeBackend: &changeBackend{}, release: make(chan struct{})}
		d := &fakeDialer{sess: PPPoESession{
			SessionID: 42, UnitNum: 7, LocalIP: initial.LocalIP,
			PeerIP: initial.PeerIP, NegMTU: initial.NegMTU,
			Changes: changes, Done: make(chan struct{}), Cleanup: func() {},
		}}
		c := NewPPPoEClient(PPPoEClientConfig{Name: "wan", Timing: testPPPoETiming}, d, b, slog.New(slog.NewTextHandler(io.Discard, nil)))
		c.Start()
		synctest.Wait()
		changes <- changedResult("192.0.2.3", "192.0.2.8", 1400)
		// Model the producer's already-proven one-slot replacement contract.
		<-changes
		latest := changedResult("192.0.2.4", "192.0.2.9", 1300)
		changes <- latest
		close(b.release)
		t.Cleanup(c.Stop)
		synctest.Wait()
		if len(changes) != 0 {
			t.Fatal("newest pending result was not consumed")
		}
		mtuChanges := 0
		addressChanges := 0
		for _, call := range b.calls {
			if call.peer == "192.0.2.8" {
				t.Fatal("applied superseded route")
			}
			if call.local == "192.0.2.3/32" {
				t.Fatal("applied superseded address")
			}
			if call.mtu == 1400 {
				t.Fatal("applied superseded MTU")
			}
			if call.mtu == 1300 {
				mtuChanges++
			}
			if call.op == "address-add" {
				if call.local == "192.0.2.4/32" {
					addressChanges++
				}
			}
		}
		if mtuChanges != 1 {
			t.Fatalf("newest MTU applications = %d", mtuChanges)
		}
		if addressChanges != 1 {
			t.Fatalf("newest address applications = %d", addressChanges)
		}
		if c.status().LocalIP != latest.LocalIP {
			t.Fatal("status did not advance to the newest result")
		}
	})
}

// TestPPPoEClientOldRouteRemovalRetry keeps failed old-route removal owned,
// retries it on an unchanged record and never removes the new gateway.
func TestPPPoEClientOldRouteRemovalRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, PPPoEClientConfig{})
		h.backend.fail = "route-remove"
		next := changedResult("192.0.2.3", "192.0.2.9", 1400)
		h.changes <- next
		synctest.Wait()
		if len(h.ended) != 0 {
			t.Fatal("route removal failure ended PPP")
		}
		h.backend.calls = nil
		h.backend.fail = ""
		h.changes <- next
		synctest.Wait()
		if len(h.backend.calls) != 1 {
			t.Fatalf("removal retry calls = %+v", h.backend.calls)
		}
		if h.backend.calls[0].op != "route-remove" {
			t.Fatal("removal retry rewrote successful state")
		}
		if h.backend.calls[0].peer != "192.0.2.1" {
			t.Fatal("removal retry targeted the new route")
		}
	})
}

// TestPPPoEClientPendingRouteRemovalBoundsOwnership checks exact backend calls
// for immediate replacement, one retained cleanup, recovery and idempotence.
func TestPPPoEClientPendingRouteRemovalBoundsOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, PPPoEClientConfig{})
		h.backend.fail = "route-remove"
		h.changes <- changedResult("192.0.2.3", "192.0.2.9", 1400)
		synctest.Wait()
		h.backend.calls = nil
		latest := changedResult("192.0.2.4", "192.0.2.10", 1300)
		h.changes <- latest
		synctest.Wait()
		wantCalls := []changeCall{
			{op: "mtu", device: "wan", mtu: 1300},
			{op: "address-add", device: "wan", local: "192.0.2.4/32", peer: "192.0.2.10/32"},
			{op: "route-add", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.10", metric: 73, proto: rtproto.Iface},
			{op: "route-remove", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.1", metric: 73, proto: rtproto.Iface},
			{op: "address-remove", device: "wan", local: "192.0.2.3/32", peer: "192.0.2.9/32"},
		}
		if len(h.backend.calls) != len(wantCalls) {
			t.Fatalf("replacement calls = %+v, want %+v", h.backend.calls, wantCalls)
		}
		for i, want := range wantCalls {
			if h.backend.calls[i] != want {
				t.Errorf("replacement call %d = %+v, want %+v", i, h.backend.calls[i], want)
			}
		}
		if len(h.ended) != 0 {
			t.Fatal("old-route cleanup failure ended PPP")
		}
		h.backend.fail = ""
		h.backend.calls = nil
		h.changes <- latest
		synctest.Wait()
		if len(h.backend.calls) != 1 {
			t.Fatalf("recovery calls = %+v", h.backend.calls)
		}
		wantCleanup := changeCall{
			op: "route-remove", device: "wan", local: "0.0.0.0/0",
			peer: "192.0.2.1", metric: 73, proto: rtproto.Iface,
		}
		if h.backend.calls[0] != wantCleanup {
			t.Errorf("recovery call = %+v, want %+v", h.backend.calls[0], wantCleanup)
		}
		if len(h.ended) != 0 {
			t.Fatal("route cleanup recovery ended PPP")
		}
		h.backend.calls = nil
		h.changes <- latest
		synctest.Wait()
		if len(h.backend.calls) != 0 {
			t.Fatalf("unchanged applied result made backend calls: %+v", h.backend.calls)
		}
		if len(h.ended) != 0 {
			t.Fatal("unchanged applied result ended PPP")
		}
	})
}

// TestPPPoEClientClosedChangesFails ends a malformed dialer lifecycle instead of
// spinning on a closed change channel or applying its zero result.
func TestPPPoEClientClosedChangesFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, PPPoEClientConfig{})
		close(h.changes)
		synctest.Wait()
		if len(h.ended) != 1 {
			t.Fatal("closed source did not end consumption")
		}
		if err := <-h.ended; err == nil {
			t.Fatal("closed source returned no error")
		}
		if len(h.backend.calls) != 0 {
			t.Fatal("closed source applied a zero result")
		}
	})
}
