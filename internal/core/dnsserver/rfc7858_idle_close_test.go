// Design: docs/architecture/dns/secure-transports.md -- DoT idle connections
// RFC: rfc/short/rfc7858.md -- RFC7858-3.4-5, robust to idle termination by either party

package dnsserver

import (
	"crypto/tls"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// dotIdleWaitMax bounds how long the test waits for the server to drop an idle
// DoT connection. The listener's dns.Server keeps the miekg/dns default idle
// timeout of 8 seconds, so twice that leaves room for a loaded machine.
const dotIdleWaitMax = 16 * time.Second

// VALIDATES: RFC 7858 section 3.4, the half of "either party" that
// TestDoTRobustToIdleConnectionClose leaves open: the SERVER terminates an idle
// connection. Method: one query on a DoT connection, then no traffic. The
// server must close the connection (the client read sees EOF, never a timeout)
// and must go on answering: a query on a fresh connection is answered.
// Ze runs no DoT client, so the client half of the sentence binds no Ze code.
// PREVENTS: a server that holds idle connections forever, and a server whose
// own idle close leaves it unable to accept the next client.
// RFC requirement: RFC7858-3.4-5 positive -- after the server itself terminates an idle DoT connection (the client sees EOF within 16 s), the server answers a query on a new connection.
func TestRFC7858ServerIdleCloseLeavesTheServerServing(t *testing.T) {
	port, roots := startDoT(t, echoHandler("10.0.0.7"))
	tlsCfg := &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	c := &dns.Client{Net: "tcp-tls", Timeout: 3 * time.Second, TLSConfig: tlsCfg}

	conn, err := c.Dial(hostPort(port))
	if err != nil {
		t.Fatalf("dial DoT: %v", err)
	}
	defer func() { _ = conn.Close() }()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("idle-server1.test"), dns.TypeA)
	if _, _, err := c.ExchangeWithConn(m, conn); err != nil {
		t.Fatalf("first DoT exchange: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(dotIdleWaitMax)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 1)
	_, readErr := conn.Read(buf)
	if !errors.Is(readErr, io.EOF) {
		t.Fatalf("idle DoT connection: read = %v, want EOF from a server-side close within %s", readErr, dotIdleWaitMax)
	}

	m2 := new(dns.Msg)
	m2.SetQuestion(dns.Fqdn("idle-server2.test"), dns.TypeA)
	resp, _, err := c.Exchange(m2, hostPort(port))
	if err != nil {
		t.Fatalf("DoT exchange after the server closed an idle connection: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer after the server-side idle close, got %d", len(resp.Answer))
	}
}
