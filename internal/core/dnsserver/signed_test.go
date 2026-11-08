package dnsserver

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// signedWire packs a query for name, signed with a test key when signed is
// true, the way a client puts it on the wire.
func signedWire(t testing.TB, name string, signed bool) []byte {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(name, dns.TypeA)
	m.SetEdns0(1232, false)
	if !signed {
		wire, err := m.Pack()
		if err != nil {
			t.Fatal(err)
		}
		return wire
	}
	m.SetTsig("k.example.", dns.HmacSHA256, 300, time.Now().Unix())
	wire, _, err := dns.TsigGenerate(m, "c2VjcmV0", "", false)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

// TestLastIsTSIG proves the reader's cheap walk finds a TSIG RR in the last
// position and nothing else. Method: a signed query, the same query
// unsigned, the signed one cut short, and a header alone.
func TestLastIsTSIG(t *testing.T) {
	signed := signedWire(t, "a.example.", true)
	if !lastIsTSIG(signed) {
		t.Error("a signed query: not found signed")
	}
	if lastIsTSIG(signedWire(t, "a.example.", false)) {
		t.Error("an unsigned query with an OPT: found signed")
	}
	if lastIsTSIG(signed[:len(signed)-3]) {
		t.Error("a signed query cut short: found signed")
	}
	if lastIsTSIG(signed[:dnsHeaderOctets]) {
		t.Error("a header alone: found signed")
	}
}

// TestSignedQueriesTake proves a kept copy is handed to the query from the
// same address with the same MAC, once, and to no other. Method: keep one
// signed query, take it with a wrong address, then the right one twice.
func TestSignedQueriesTake(t *testing.T) {
	wire := signedWire(t, "a.example.", true)
	parsed := new(dns.Msg)
	if err := parsed.Unpack(wire); err != nil {
		t.Fatal(err)
	}
	from := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 40000}
	other := &net.UDPAddr{IP: net.IPv4(192, 0, 2, 2), Port: 40000}
	s := NewSignedQueries()
	s.keep(from, wire)
	if _, ok := s.take(other, parsed.IsTsig().MAC); ok {
		t.Fatal("another client's query took the copy")
	}
	got, ok := s.take(from, parsed.IsTsig().MAC)
	if !ok || string(got) != string(wire) {
		t.Fatalf("take = %v, want the bytes kept", ok)
	}
	if _, ok := s.take(from, parsed.IsTsig().MAC); ok {
		t.Fatal("a copy was handed out twice")
	}
}

// FuzzLastIsTSIG proves the walk never panics on hostile bytes and never
// finds a TSIG the DNS library does not. The other direction is not
// asserted: the library unpacks a message that ends before its counts say,
// and the walk refuses it, so that query keeps no copy and is answered
// SERVFAIL, never forwarded from its parsed form.
func FuzzLastIsTSIG(f *testing.F) {
	f.Add(signedWire(f, "a.example.", true))
	f.Add(signedWire(f, "a.example.", false))
	f.Fuzz(func(t *testing.T, m []byte) {
		got := lastIsTSIG(m)
		parsed := new(dns.Msg)
		if parsed.Unpack(m) != nil {
			return
		}
		if got && parsed.IsTsig() == nil {
			t.Fatal("lastIsTSIG found a TSIG the library does not")
		}
	})
}
