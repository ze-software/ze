// Design: docs/guide/configuration.md -- SSH binds every listener or none, at start and on reload
// Related: ssh.go -- Start, Reconfigure, bindListeners

package ssh

import (
	"bufio"
	"context"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// unbindableIPv4 is TEST-NET-1 (RFC 5737): no host holds it, so a bind fails
// with EADDRNOTAVAIL on Linux and macOS alike.
const unbindableIPv4 = "192.0.2.1"

// freePort answers a TCP port that 127.0.0.1 and, when the host has IPv6,
// ::1 both have free right now.
func freePort(t *testing.T, needIPv6 bool) string {
	t.Helper()
	for range 20 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port) //nolint:forcetypeassert // A TCP listener answers a TCPAddr.
		if !needIPv6 {
			_ = ln.Close()
			return port
		}
		ln6, err6 := net.Listen("tcp", net.JoinHostPort("::1", port))
		_ = ln.Close()
		if err6 != nil {
			continue
		}
		_ = ln6.Close()
		return port
	}
	t.Fatal("no port free on both 127.0.0.1 and ::1")
	return ""
}

// requireIPv6Loopback skips a test on a host without ::1.
func requireIPv6Loopback(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("host has no IPv6 loopback: %v", err)
	}
	_ = ln.Close()
}

// startListenerServer starts a real server on addrs and stops it at cleanup.
func startListenerServer(t *testing.T, addrs ...string) *Server {
	t.Helper()
	srv := newListenerServer(t, addrs...)
	if err := srv.Start(context.Background(), nil, nil); err != nil {
		t.Fatalf("Start(%v): %v", addrs, err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	return srv
}

func newListenerServer(t *testing.T, addrs ...string) *Server {
	t.Helper()
	srv, err := NewServer(Config{
		Listen: addrs[0], ListenAddrs: addrs,
		HostKeyPath: filepath.Join(t.TempDir(), "host-key"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

// requireZeBanner dials addr and requires the SSH identification line Ze's
// server sends, which proves this server, not another process, answers there.
func requireZeBanner(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer conn.Close() //nolint:errcheck // Test connection.
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read banner from %s: %v", addr, err)
	}
	if !strings.HasPrefix(line, "SSH-2.0-ze") {
		t.Fatalf("banner from %s = %q, want the ze SSH identification", addr, line)
	}
}

// requireRefused requires that nothing listens on addr.
func requireRefused(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("%s still accepts connections", addr)
	}
}

// TestStartFailsWhenExtraListenerFails starts a server whose second address is
// not on the host.
// VALIDATES: AC-6 -- Start answers an error naming the address and the bind
// error, and leaves no listener of the set bound: the first port binds again.
// PREVENTS: the old log-and-continue path, which served half the config and
// reported a started server.
func TestStartFailsWhenExtraListenerFails(t *testing.T) {
	port := freePort(t, false)
	first := net.JoinHostPort("127.0.0.1", port)
	bad := net.JoinHostPort(unbindableIPv4, port)
	srv := newListenerServer(t, first, bad)

	err := srv.Start(context.Background(), nil, nil)
	if err == nil {
		_ = srv.Stop(context.Background())
		t.Fatal("Start succeeded with an address the host does not hold")
	}
	if !strings.Contains(err.Error(), bad) {
		t.Fatalf("Start error %q does not name %s", err, bad)
	}
	if got := srv.Addresses(); got != nil {
		t.Fatalf("Addresses after a failed Start = %v, want none", got)
	}
	ln, err := net.Listen("tcp", first)
	if err != nil {
		t.Fatalf("first listener %s left bound after the failed Start: %v", first, err)
	}
	_ = ln.Close()
}

// TestStartServesDualStackWildcardOverBothFamilies starts a server on `::`
// alone and reaches it over IPv4 and IPv6.
// VALIDATES: AC-3 -- `::` alone serves both families, which is why validation
// tells the operator to list it instead of `::` beside 0.0.0.0.
// PREVENTS: the refusal's fix being a config that drops IPv4.
func TestStartServesDualStackWildcardOverBothFamilies(t *testing.T) {
	requireIPv6Loopback(t)
	port := freePort(t, true)
	startListenerServer(t, net.JoinHostPort("::", port))

	requireZeBanner(t, net.JoinHostPort("127.0.0.1", port))
	requireZeBanner(t, net.JoinHostPort("::1", port))
}

// TestStartBindsSpecificAddressesOfBothFamilies starts a server on 127.0.0.1
// and ::1 on one port.
// VALIDATES: AC-4 -- distinct specific addresses of the two families both bind.
// PREVENTS: the all-or-none bind refusing a set the kernel accepts.
func TestStartBindsSpecificAddressesOfBothFamilies(t *testing.T) {
	requireIPv6Loopback(t)
	port := freePort(t, true)
	v4 := net.JoinHostPort("127.0.0.1", port)
	v6 := net.JoinHostPort("::1", port)
	srv := startListenerServer(t, v4, v6)

	if got := srv.Addresses(); len(got) != 2 || got[0] != v4 || got[1] != v6 {
		t.Fatalf("Addresses = %v, want [%s %s]", got, v4, v6)
	}
	requireZeBanner(t, v4)
	requireZeBanner(t, v6)
}

// TestReconfigureAllOrNone moves a running server to a set holding an address
// the host does not have.
// VALIDATES: AC-11, server half -- Reconfigure fails naming the address,
// Addresses is unchanged, and the old listener still answers.
// PREVENTS: a reload that drops the working listener for a set it cannot bind.
func TestReconfigureAllOrNone(t *testing.T) {
	port := freePort(t, false)
	old := net.JoinHostPort("127.0.0.1", port)
	bad := net.JoinHostPort(unbindableIPv4, port)
	srv := startListenerServer(t, old)

	err := srv.Reconfigure(context.Background(), []string{old, bad})
	if err == nil {
		t.Fatal("Reconfigure succeeded with an address the host does not hold")
	}
	if !strings.Contains(err.Error(), bad) {
		t.Fatalf("Reconfigure error %q does not name %s", err, bad)
	}
	if got := srv.Addresses(); len(got) != 1 || got[0] != old {
		t.Fatalf("Addresses after a failed Reconfigure = %v, want [%s]", got, old)
	}
	requireZeBanner(t, old)
}

// TestReconfigureMovesListener moves a running server from 127.0.0.1 to ::1.
// VALIDATES: AC-12, server half -- the server answers on the new address and
// no longer on the old one, with no restart.
// PREVENTS: a removed listener left serving, or a move that needs a restart.
func TestReconfigureMovesListener(t *testing.T) {
	requireIPv6Loopback(t)
	port := freePort(t, true)
	old := net.JoinHostPort("127.0.0.1", port)
	moved := net.JoinHostPort("::1", port)
	srv := startListenerServer(t, old)

	if err := srv.Reconfigure(context.Background(), []string{moved}); err != nil {
		t.Fatalf("Reconfigure to %s: %v", moved, err)
	}
	if got := srv.Addresses(); len(got) != 1 || got[0] != moved {
		t.Fatalf("Addresses = %v, want [%s]", got, moved)
	}
	requireZeBanner(t, moved)
	requireRefused(t, old)
}
