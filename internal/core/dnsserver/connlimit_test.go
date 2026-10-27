// Design: docs/architecture/dns/server-harness.md -- the TCP connection cap

package dnsserver

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// connCapTestMax is the cap the test configures: small, so the test opens few
// sockets.
const connCapTestMax = 2

// holdTCPConn dials addr, answers one query on the connection so the server
// moves it to the idle timeout, and returns it still open.
func holdTCPConn(t *testing.T, addr string) *dns.Conn {
	t.Helper()
	c := &dns.Client{Net: "tcp", Timeout: 2 * time.Second}
	conn, err := c.Dial(addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	m := new(dns.Msg)
	m.SetQuestion("held.test.", dns.TypeA)
	if _, _, err := c.ExchangeWithConn(m, conn); err != nil {
		t.Fatalf("exchange on a held connection: %v", err)
	}
	return conn
}

// VALIDATES: Options.TCPConnMax caps the connections one plain TCP listener
// holds. Method: hold the cap's worth of connections open, then dial one more;
// the server must close the extra one at once (the client reads EOF, never its
// own timeout). Closing a held connection frees its slot: a fresh query is then
// answered over TCP.
// PREVENTS: any source, inside the consumer's ACL or not, holding an unbounded
// number of TCP connections and with them the daemon's descriptors.
func TestManager_TCPConnMaxRefusesPastTheCap(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.3"), Options{TCPConnMax: connCapTestMax})
	if err := mgr.reconcile(true, Listeners{Plain: []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}}, BindLenient); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	t.Cleanup(mgr.Stop)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))

	held := make([]*dns.Conn, 0, connCapTestMax)
	for range connCapTestMax {
		held = append(held, holdTCPConn(t, addr))
	}

	dialer := net.Dialer{Timeout: 2 * time.Second}
	extra, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial past the cap: %v", err)
	}
	t.Cleanup(func() { _ = extra.Close() })
	if err := extra.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 1)
	if _, readErr := extra.Read(buf); !errors.Is(readErr, io.EOF) {
		t.Fatalf("connection past the cap of %d: read = %v, want EOF from a server-side close", connCapTestMax, readErr)
	}

	if err := held[0].Close(); err != nil {
		t.Fatalf("close a held connection: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		m := new(dns.Msg)
		m.SetQuestion("after.test.", dns.TypeA)
		c := &dns.Client{Net: "tcp", Timeout: 500 * time.Millisecond}
		resp, _, exErr := c.Exchange(m, addr)
		if exErr == nil && len(resp.Answer) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("after a held connection closed, a new TCP query was not answered: %v", exErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// VALIDATES: a zero TCPConnMax leaves the listener uncapped, the behavior the
// authoritative services keep. Method: hold one connection more than the test
// cap and see every one answered.
// PREVENTS: the cap leaking onto a consumer that never asked for one.
func TestManager_TCPConnMaxZeroIsUncapped(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.4"), Options{})
	if err := mgr.reconcile(true, Listeners{Plain: []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}}, BindLenient); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	t.Cleanup(mgr.Stop)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	for range connCapTestMax + 1 {
		holdTCPConn(t, addr)
	}
}
