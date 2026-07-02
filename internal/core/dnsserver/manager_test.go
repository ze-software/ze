package dnsserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func freePort(t *testing.T) uint16 {
	t.Helper()
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	udpAddr, ok := pc.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("expected *net.UDPAddr, got %T", pc.LocalAddr())
	}
	port := uint16(udpAddr.Port)
	if cerr := pc.Close(); cerr != nil {
		t.Fatalf("close probe socket: %v", cerr)
	}
	return port
}

// echoHandler is a minimal dns.Handler with no consumer-plugin involvement,
// proving the Manager is generic over any handler.
func echoHandler(ip string) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if len(r.Question) > 0 {
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(ip),
			})
		}
		_ = w.WriteMsg(m)
	}
}

func exchangeA(t *testing.T, proto, addr, qname string) *dns.Msg {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(qname), dns.TypeA)
	c := &dns.Client{Net: proto, Timeout: 2 * time.Second}
	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("%s exchange: %v", proto, err)
	}
	return resp
}

// VALIDATES: a Manager binds UDP+TCP for a plain dns.Handler and serves a
// query, standalone -- no specific consumer plugin involved (AC-1,
// End-to-End User Story #2: a second DNS plugin can reuse the harness
// without importing a sibling plugin).
// PREVENTS: the harness silently depending on a single consumer's construction.
func TestManager_BindsAndServes(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.9"), Options{})
	if err := mgr.Apply(true, []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	t.Cleanup(mgr.Stop)

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	for _, proto := range []string{"udp", "tcp"} {
		resp := exchangeA(t, proto, addr, "example.test.")
		if len(resp.Answer) != 1 {
			t.Fatalf("%s: got %d answers, want 1", proto, len(resp.Answer))
		}
		a, ok := resp.Answer[0].(*dns.A)
		if !ok || a.A.String() != "10.0.0.9" {
			t.Errorf("%s: answer = %v, want A 10.0.0.9", proto, resp.Answer[0])
		}
	}
}

// VALIDATES: Apply is a no-op (no rebind) when the endpoint set and enabled
// flag are unchanged; a second Apply with the same endpoints does not error
// and the listener keeps serving on the same port (ported from a consumer
// plugin's reload-without-rebind coverage).
// PREVENTS: a spurious rebind (and a listen-address race) on every config
// reload even when nothing about the bound addresses changed.
func TestManager_RebindOnlyOnEndpointChange(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.1"), Options{})
	eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	t.Cleanup(mgr.Stop)

	firstApplied := mgr.applied
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if mgr.applied != firstApplied {
		t.Errorf("applied signature changed on unchanged endpoint set")
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	resp := exchangeA(t, "udp", addr, "still.test.")
	if len(resp.Answer) != 1 {
		t.Fatalf("got %d answers after no-op apply, want 1", len(resp.Answer))
	}
}

// VALIDATES: a failed Apply (every endpoint unbindable) does not stick the
// signature -- a later Apply with the IDENTICAL endpoint set must retry the
// bind rather than silently no-op forever, once the obstruction clears.
// PREVENTS: a transient bind failure (port momentarily taken, address not
// yet present) permanently wedging the manager into "already applied" for
// an endpoint set that in fact has zero listeners bound.
func TestManager_RetriesAfterFailedApply(t *testing.T) {
	port := freePort(t)
	eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	// Occupy the UDP port so the first Apply's bind fails outright.
	var lc net.ListenConfig
	blocker, err := lc.ListenPacket(context.Background(), "udp", addr)
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}

	mgr := New(testLogger(), echoHandler("10.0.0.2"), Options{})
	t.Cleanup(mgr.Stop)

	if err := mgr.Apply(true, eps); err == nil {
		t.Fatal("first Apply on an occupied port succeeded, want error")
	}

	if cerr := blocker.Close(); cerr != nil {
		t.Fatalf("release occupied port: %v", cerr)
	}

	// Same endpoint set, now bindable: a second identical Apply must not be
	// treated as already-applied -- it must actually retry and succeed.
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("second Apply after obstruction cleared: %v", err)
	}

	resp := exchangeA(t, "udp", addr, "retried.test.")
	if len(resp.Answer) != 1 {
		t.Fatalf("got %d answers after retry, want 1 (listener never actually came up)", len(resp.Answer))
	}
}

// VALIDATES: a good->bad->revert-to-good Apply sequence retries the revert,
// rather than getting stuck believing the reverted-to signature is already
// applied. Apply(A) succeeds; Apply(B) tears down A's listeners via Stop and
// then fails to bind entirely; Apply(A) again must actually rebind A -- not
// short-circuit as a no-op against a stale "applied" signature left over
// from the original success, while zero listeners are actually up.
// PREVENTS: an operator reverting a bad config change back to the last-good
// one silently ending up with no listeners at all.
func TestManager_RetriesAfterRevertToPreviouslyGoodSignature(t *testing.T) {
	portA := freePort(t)
	portB := freePort(t)
	epsA := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: portA}}
	epsB := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: portB}}
	addrA := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(portA)))
	addrB := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(portB)))

	mgr := New(testLogger(), echoHandler("10.0.0.4"), Options{})
	t.Cleanup(mgr.Stop)

	if err := mgr.Apply(true, epsA); err != nil {
		t.Fatalf("Apply(A): %v", err)
	}

	// Occupy B's port so Apply(B) tears down A (via Stop, inside Apply) and
	// then fails to bind B at all.
	var lc net.ListenConfig
	blocker, err := lc.ListenPacket(context.Background(), "udp", addrB)
	if err != nil {
		t.Fatalf("occupy port B: %v", err)
	}
	if err := mgr.Apply(true, epsB); err == nil {
		t.Fatal("Apply(B) on an occupied port succeeded, want error")
	}
	if cerr := blocker.Close(); cerr != nil {
		t.Fatalf("release occupied port B: %v", cerr)
	}

	// Revert to A: A's listeners were torn down by Apply(B)'s Stop() call,
	// so this must actually rebind, not no-op against the stale signature.
	if err := mgr.Apply(true, epsA); err != nil {
		t.Fatalf("revert Apply(A): %v", err)
	}
	resp := exchangeA(t, "udp", addrA, "reverted.test.")
	if len(resp.Answer) != 1 {
		t.Fatalf("got %d answers after revert to A, want 1 (A never actually rebound)", len(resp.Answer))
	}
}

// VALIDATES: an unexpected listener crash (socket closed out from under the
// server, not via Stop) fires OnListenerChange(..., false) and invalidates
// the applied signature so a later Apply with the SAME endpoint set actually
// retries the bind instead of silently no-op'ing forever.
// PREVENTS: a dead listener (e.g. an anycast address withdrawn under it)
// permanently reporting "up" in metrics/logs with no way to recover short of
// an unrelated config change that happens to alter the endpoint signature.
func TestManager_UnexpectedListenerCrashInvalidatesApplied(t *testing.T) {
	port := freePort(t)
	eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	var mu sync.Mutex
	var changes []string
	mgr := New(testLogger(), echoHandler("10.0.0.5"), Options{
		OnListenerChange: func(proto, a string, up bool) {
			mu.Lock()
			changes = append(changes, fmt.Sprintf("%s:%s:%v", proto, a, up))
			mu.Unlock()
		},
	})
	t.Cleanup(mgr.Stop)

	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Simulate an external crash: close the UDP socket out from under the
	// server without going through Stop (e.g. the bound address is withdrawn
	// from the interface underneath a live listener).
	if len(mgr.servers) == 0 {
		t.Fatal("no servers bound after Apply")
	}
	udpServer := mgr.servers[0]
	if err := udpServer.PacketConn.Close(); err != nil {
		t.Fatalf("simulate crash: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		found := false
		for _, c := range changes {
			if c == "udp:127.0.0.1:false" {
				found = true
			}
		}
		mu.Unlock()
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("OnListenerChange(udp, ..., false) never fired after simulated crash")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A later Apply with the SAME endpoint set must actually retry -- not
	// silently no-op believing the crashed listener is still up.
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("Apply after crash: %v", err)
	}
	resp := exchangeA(t, "udp", addr, "recovered.test.")
	if len(resp.Answer) != 1 {
		t.Fatalf("got %d answers after recovery, want 1 (listener never actually came back up)", len(resp.Answer))
	}
}

// VALIDATES: bind() does not return until the spawned listener goroutines
// have genuinely reached dns.Server's "started" state -- closing the race
// where a caller's immediate Stop() (e.g. a rapid Apply->Apply sequence)
// calls ShutdownContext before ActivateAndServe has set srv.started=true.
// ShutdownContext on a not-yet-started *dns.Server returns immediately
// ("server not started") WITHOUT closing the underlying socket, so the
// listener goroutine -- unaware any shutdown was requested -- proceeds to
// serve indefinitely on a socket the Manager has already forgotten about
// (m.servers was cleared), permanently occupying the port and leaking a
// live, unreachable-by-the-Manager listener.
// PREVENTS: a leaked live listener that a later bind() on the same port can
// never displace (port already in use), even though the Manager itself
// correctly believes nothing is bound.
func TestManager_BindThenImmediateStopNeverLeaksListener(t *testing.T) {
	for i := range 30 {
		port := freePort(t)
		eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

		mgr := New(testLogger(), echoHandler("10.0.0.6"), Options{})
		if err := mgr.Apply(true, eps); err != nil {
			t.Fatalf("iteration %d: Apply: %v", i, err)
		}
		// Stop immediately -- the race window is between bind()'s spawned
		// goroutines starting and ActivateAndServe marking the server
		// started. No sleep: if bind() correctly waits for the started
		// signal before returning, this Stop() always sees fully-started
		// servers and ShutdownContext actually closes the sockets.
		mgr.Stop()

		// The port must be immediately reusable -- if the previous
		// listener leaked, this bind fails with "address already in use".
		var lc net.ListenConfig
		pc, err := lc.ListenPacket(context.Background(), "udp", addr)
		if err != nil {
			t.Fatalf("iteration %d: port %s not released after Stop (leaked listener): %v", i, addr, err)
		}
		if cerr := pc.Close(); cerr != nil {
			t.Fatalf("iteration %d: close probe: %v", i, cerr)
		}
	}
}

// VALIDATES: Freebind:true installs a net.ListenConfig.Control hook on Linux
// (where IP_FREEBIND exists), Freebind:false never installs one on any
// platform, and Freebind:true is a safe no-op (no hook) on a non-Linux host,
// matching a bare net.ListenConfig (AC-4).
// PREVENTS: the default (Freebind:false) silently gaining a Control hook it
// never had, and documents the cross-platform contract for Freebind:true.
func TestFreebindOptionInstallsControlHook(t *testing.T) {
	if listenConfig(false).Control != nil {
		t.Error("Freebind:false must not install a Control hook")
	}
	hasHook := listenConfig(true).Control != nil
	wantHook := runtime.GOOS == "linux"
	if hasHook != wantHook {
		t.Errorf("Freebind:true on %s: Control hook installed = %v, want %v", runtime.GOOS, hasHook, wantHook)
	}
}
