package dnsserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

type fakeResponseWriter struct {
	dns.ResponseWriter
	written *dns.Msg
}

func (f *fakeResponseWriter) WriteMsg(m *dns.Msg) error {
	f.written = m
	return nil
}

func (f *fakeResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 5353}
}

// VALIDATES: the authoritative wrapper sets Authoritative=true and never
// advertises recursion, owns the single wire write, and re-asserts the shape
// after fn so an answer func that sets RecursionAvailable=true cannot make it
// onto the wire; a panic inside fn is recovered via onPanic and no reply is
// written for that query (AC-5, the child-2 R-3 single-source recursion guard).
// PREVENTS: a consumer accidentally (or via its own WriteMsg) advertising
// recursion, or one bad query crashing the listener goroutine.
func TestAuthoritativeWrapper_SetsBitsAndRecovers(t *testing.T) {
	q := new(dns.Msg)
	q.SetQuestion("example.test.", dns.TypeA)

	var gotMsg *dns.Msg
	var gotSrc netip.Addr
	handler := Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
		gotMsg = msg
		gotSrc = RemoteAddr(p)
		msg.RecursionAvailable = true // fn tries to advertise recursion...
		msg.Compress = true           // ...and to enable compression
		return true
	}, nil)

	fw := &fakeResponseWriter{}
	handler(fw, q)

	if gotMsg == nil {
		t.Fatal("fn was not called")
	}
	if gotSrc.String() != "192.0.2.1" {
		t.Errorf("src passed to fn = %v, want 192.0.2.1 (the packet source)", gotSrc)
	}
	if !gotMsg.Authoritative {
		t.Error("Authoritative = false, want true")
	}
	if gotMsg.Compress {
		t.Error("Compress = true, want false")
	}
	// ...and the wrapper must have forced it back off before the wire write.
	if gotMsg.RecursionAvailable {
		t.Error("RecursionAvailable = true, want false (wrapper must re-assert the guard even when the answer func sets it)")
	}
	if fw.written == nil {
		t.Fatal("no reply written")
	}
	if fw.written.RecursionAvailable {
		t.Error("written reply advertised recursion; the guard must be unbypassable")
	}
	if fw.written.Compress {
		t.Error("written reply enabled compression; the wrapper must re-assert the full shape")
	}

	// send=false must drop the query with no reply.
	dropped := Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
		return false
	}, nil)
	fwDrop := &fakeResponseWriter{}
	dropped(fwDrop, q)
	if fwDrop.written != nil {
		t.Error("a reply was written for a query the answer func dropped (send=false), want none")
	}

	var panicked any
	handler = Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
		panic("boom")
	}, func(rec any) { panicked = rec })

	fw2 := &fakeResponseWriter{}
	handler(fw2, q) // must not propagate the panic to the caller
	if panicked != "boom" {
		t.Errorf("onPanic received %v, want %q", panicked, "boom")
	}
	if fw2.written != nil {
		t.Error("a reply was written for a query whose answer func panicked, want none")
	}
}
