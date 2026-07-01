package dnsserver

import (
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

// VALIDATES: the authoritative wrapper sets Authoritative=true, never sets
// RecursionAvailable, and never compresses, then delegates to fn; a panic
// inside fn is recovered via onPanic and no reply is written for that query
// (ported behavior from a consumer plugin's single-defer query handler --
// AC-5, the child-2 R-3 single-source recursion guard).
// PREVENTS: a consumer accidentally advertising recursion, or one bad query
// crashing the listener goroutine.
func TestAuthoritativeWrapper_SetsBitsAndRecovers(t *testing.T) {
	q := new(dns.Msg)
	q.SetQuestion("example.test.", dns.TypeA)

	var gotMsg *dns.Msg
	handler := Authoritative(func(msg, r *dns.Msg, w dns.ResponseWriter) {
		gotMsg = msg
		_ = w.WriteMsg(msg)
	}, nil)

	fw := &fakeResponseWriter{}
	handler(fw, q)

	if gotMsg == nil {
		t.Fatal("fn was not called")
	}
	if !gotMsg.Authoritative {
		t.Error("Authoritative = false, want true")
	}
	if gotMsg.RecursionAvailable {
		t.Error("RecursionAvailable = true, want false")
	}
	if gotMsg.Compress {
		t.Error("Compress = true, want false")
	}
	if fw.written == nil {
		t.Error("no reply written")
	}

	var panicked any
	handler = Authoritative(func(msg, r *dns.Msg, w dns.ResponseWriter) {
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
