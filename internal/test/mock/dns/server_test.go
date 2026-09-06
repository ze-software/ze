// Goal: prove the server serves the TTL the zone declares, answers the value a
// test last Set, binds the port it was asked for, and keeps serving after a
// message it cannot answer. Method: a real UDP server on an ephemeral port,
// queried with the same client library the daemon's resolver uses.
//
// VALIDATES: AC-1, AC-2, AC-6.
// PREVENTS: a stub whose answers are right but uncacheable-by-accident, or one
// that reprograms a name and keeps serving the old value, either of which makes
// a consumer test assert the wrong thing.

package dns

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

// startForTest starts a stub on an OS-chosen port and stops it when the test
// ends.
func startForTest(t *testing.T) *Server {
	t.Helper()
	server, err := Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return server
}

// ask sends one query and returns the reply. It builds the query the way
// Resolver.query does (internal/component/resolve/dns/resolver.go), so the test
// exercises what the daemon sends rather than a simpler message.
func ask(t *testing.T, server *Server, name string, qtype uint16) *mdns.Msg {
	t.Helper()
	client := &mdns.Client{Net: "udp", Timeout: 2 * time.Second}
	query := new(mdns.Msg)
	query.SetQuestion(mdns.Fqdn(name), qtype)
	query.RecursionDesired = true
	query.SetEdns0(4096, false)

	reply, _, err := client.Exchange(query, server.Addr().String())
	if err != nil {
		t.Fatalf("query %s %s: %v", name, mdns.TypeToString[qtype], err)
	}
	return reply
}

func TestAnswerCarriesTheDeclaredTTL(t *testing.T) {
	server := startForTest(t)

	for _, test := range []struct {
		name  string
		qtype uint16
		ttl   uint32
	}{
		// Zero is the value that matters most: cache.put
		// (internal/component/resolve/dns/cache.go) returns before storing a
		// zero-TTL answer, so a stub that substituted a default here would hide
		// every later change behind the daemon's own cache.
		{"web.example.test", mdns.TypeA, 0},
		{"web.example.test", mdns.TypeAAAA, 0},
		{"cached.example.test", mdns.TypeA, 300},
	} {
		reply := ask(t, server, test.name, test.qtype)
		if len(reply.Answer) == 0 {
			t.Errorf("%s %s: no records", test.name, mdns.TypeToString[test.qtype])
			continue
		}
		for _, rr := range reply.Answer {
			if rr.Header().Ttl != test.ttl {
				t.Errorf("%s %s: ttl %d, want %d", test.name, mdns.TypeToString[test.qtype], rr.Header().Ttl, test.ttl)
			}
		}
	}
}

func TestSetChangesTheNextAnswer(t *testing.T) {
	server := startForTest(t)

	before := ask(t, server, "web.example.test", mdns.TypeA)
	if got := addresses(before); len(got) != 1 || got[0] != "203.0.113.10" {
		t.Fatalf("before Set: %v, want [203.0.113.10]", got)
	}

	server.Set("web.example.test", mdns.TypeA, Answer{Addresses: []netip.Addr{netip.MustParseAddr("203.0.113.11")}})

	after := ask(t, server, "web.example.test", mdns.TypeA)
	if got := addresses(after); len(got) != 1 || got[0] != "203.0.113.11" {
		t.Fatalf("after Set: %v, want [203.0.113.11]", got)
	}

	// The other family is untouched: Set replaces one name and one type.
	sibling := ask(t, server, "web.example.test", mdns.TypeAAAA)
	if got := addresses(sibling); len(got) != 1 || got[0] != "2001:db8::10" {
		t.Fatalf("AAAA after setting A: %v, want [2001:db8::10]", got)
	}
}

// Set on a name the zone does not carry adds it, so a fixture can serve a name
// this package never declared.
func TestSetAddsANameTheZoneDoesNotCarry(t *testing.T) {
	server := startForTest(t)

	if reply := ask(t, server, "new.example.test", mdns.TypeA); reply.Rcode != mdns.RcodeNameError {
		t.Fatalf("before Set: rcode %s, want NXDOMAIN", mdns.RcodeToString[reply.Rcode])
	}

	server.Set("new.example.test", mdns.TypeA, Answer{Addresses: []netip.Addr{netip.MustParseAddr("203.0.113.30")}})

	reply := ask(t, server, "new.example.test", mdns.TypeA)
	if reply.Rcode != mdns.RcodeSuccess {
		t.Fatalf("after Set: rcode %s, want NOERROR", mdns.RcodeToString[reply.Rcode])
	}
	if got := addresses(reply); len(got) != 1 || got[0] != "203.0.113.30" {
		t.Fatalf("after Set: %v, want [203.0.113.30]", got)
	}
}

func TestStartBindsChosenAndEphemeralPort(t *testing.T) {
	// Port 0 binds a port the OS chooses, and Addr reports the real one.
	ephemeral := startForTest(t)
	if ephemeral.Addr().Port() == 0 {
		t.Fatal("Addr reports port 0 after binding an ephemeral port")
	}

	// A port already in use returns an error naming it. It is never downgraded
	// to a server that accepts nothing: a test reading a silent stub's answers
	// would report the daemon's own cache back to itself.
	busy := ephemeral.Addr().String()
	second, err := Start(busy)
	if err == nil {
		if closeErr := second.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
		t.Fatalf("Start on the busy address %s returned a server", busy)
	}
	if !strings.Contains(err.Error(), busy) {
		t.Fatalf("Start error %q does not name the address it could not bind", err)
	}
}

// A message the server cannot answer is answered, and the server keeps serving.
// Two shapes reach it: a query with no question section, which the handler
// refuses, and bytes that are not a DNS message at all, which the library drops
// before the handler.
func TestMalformedQueryIsAnsweredNotDropped(t *testing.T) {
	server := startForTest(t)

	writer := &recordingWriter{}
	server.handle(writer, new(mdns.Msg))
	if writer.reply == nil {
		t.Fatal("a query with no question section was dropped, not answered")
	}
	if writer.reply.Rcode != mdns.RcodeRefused {
		t.Fatalf("question-less query: rcode %s, want REFUSED", mdns.RcodeToString[writer.reply.Rcode])
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(t.Context(), "udp", server.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := conn.Write([]byte("not a dns message")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if reply := ask(t, server, "web.example.test", mdns.TypeA); len(reply.Answer) != 1 {
		t.Fatalf("after a malformed datagram: %d records, want 1 -- the server stopped serving", len(reply.Answer))
	}
}

// addresses reads the address out of every A and AAAA record in a reply.
func addresses(reply *mdns.Msg) []string {
	out := make([]string, 0, len(reply.Answer))
	for _, rr := range reply.Answer {
		switch record := rr.(type) {
		case *mdns.A:
			out = append(out, record.A.String())
		case *mdns.AAAA:
			out = append(out, record.AAAA.String())
		}
	}
	return out
}

// recordingWriter keeps the reply the handler wrote, so a test can drive the
// handler without a socket.
type recordingWriter struct {
	mdns.ResponseWriter
	reply *mdns.Msg
}

func (w *recordingWriter) WriteMsg(m *mdns.Msg) error {
	w.reply = m
	return nil
}

func (w *recordingWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
}

func (w *recordingWriter) Write([]byte) (int, error) { return 0, errors.New("not supported") }
