package dnsserver

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"strconv"
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
