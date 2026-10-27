// Design: docs/architecture/provisioning/dhcp-server.md -- panic containment in the request task
// Related: panic_guard.go -- requestTask, the boundary under test
// Related: register.go -- serveMulti and serveRequests, the loop the guard wraps

package dhcpserver

import (
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestDHCPRequestPanicContained proves a listener's request task contains a Ze
// defect met while it answers one request: the listener carries on, the next
// request is served, and the panic is counted and crash-recorded on the real
// guard.
// METHOD: serveMulti runs over an unprivileged loopback socket with two
// handlers. The first owns 10.0.0.0/24 and has no pool, a state only a defect
// produces since newDHCPHandler always builds one. A DISCOVER whose ciaddr is in
// that subnet reaches the nil pool and panics; a second DISCOVER whose ciaddr is
// in 127.0.0.0/8 must then be answered by the healthy handler, which allocates.
// PREVENTS: a request-driven panic killing the daemon, or ending the listener
// and with it every later DHCP answer on that interface.
func TestDHCPRequestPanicContained(t *testing.T) {
	before, declared := dhcpGuard.Panics(taskRequest)
	if !declared {
		t.Fatalf("task %q is not declared on the dhcp boundary", taskRequest)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	defective := &dhcpHandler{subnet: subnetConfig{Prefix: netip.MustParsePrefix("10.0.0.0/24")}}
	healthy := newDHCPHandler(subnetConfig{
		Prefix:       netip.MustParsePrefix("127.0.0.0/8"),
		Ranges:       []addressRange{{Name: "r", Start: netip.MustParseAddr("127.0.0.50"), Stop: netip.MustParseAddr("127.0.0.60")}},
		LeaseTimeSec: 600,
	}, netip.MustParseAddr("127.0.0.1"), pxeConfig{})

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveMulti(conn, []*dhcpHandler{defective, healthy}, slogutil.DiscardLogger())
	}()
	t.Cleanup(func() {
		_ = conn.Close()
		<-done
	})

	server, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener address %T is not a *net.UDPAddr", conn.LocalAddr())
	}
	client, err := net.DialUDP("udp4", nil, server)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	first := buildMsg(msgDiscover, net.HardwareAddr{2, 0, 0, 0, 0, 1}, 1, netip.MustParseAddr("10.0.0.5"), 0, nil)
	if _, err := client.Write(first); err != nil {
		t.Fatalf("send the DISCOVER that panics: %v", err)
	}
	waitFor(t, "the panic to be counted", func() bool {
		count, _ := dhcpGuard.Panics(taskRequest)
		return count == before+1
	})

	second := buildMsg(msgDiscover, net.HardwareAddr{2, 0, 0, 0, 0, 2}, 2, netip.MustParseAddr("127.0.0.2"), 0, nil)
	if _, err := client.Write(second); err != nil {
		t.Fatalf("send the DISCOVER after the panic: %v", err)
	}
	waitFor(t, "the healthy handler to allocate: the listener did not carry on", func() bool {
		_, allocated, _ := healthy.pool.stats()
		return allocated > 0
	})

	if count, _ := dhcpGuard.Panics(taskRequest); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d", taskRequest, count, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "dhcp " + taskRequest + ", unit request from " + client.LocalAddr().String()
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

// waitFor polls cond until it holds, failing the test after five seconds with
// what it waited for.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
