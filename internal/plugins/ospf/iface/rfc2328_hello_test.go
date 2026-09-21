// VALIDATES: RFC 2328 Section 10.5 Hello reception -- the Network Mask, HelloInterval
// and RouterDeadInterval of a received Hello are checked against the receiving
// interface, and bidirectional communication is declared only once this router's
// Router ID appears in the neighbor's Hello.
// PREVENTS: an adjacency forming across mismatched network parameters, and a
// neighbor reaching 2-Way before it has heard this router.
package iface

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// RFC requirement: RFC2328-10.5-1 positive -- a Hello whose Network Mask, HelloInterval and RouterDeadInterval equal the receiving interface's is accepted, and the neighbor becomes bidirectional exactly when the Hello lists this router's Router ID (validateHelloLocked accepts the matching values and receiveHello sets TwoWay on the listing, iface.go).
func TestRFC2328HelloMatchingParametersAndTwoWay(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg), time.Now()); reason != "" {
		t.Fatalf("matching Hello refused: %s", reason)
	}
	if n, known := ifc.neighbors[peer]; !known || n.TwoWay {
		t.Fatalf("neighbor = %+v known=%v, want known and not yet bidirectional (this router not listed)", n, known)
	}
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("Hello listing this router refused: %s", reason)
	}
	if !ifc.neighbors[peer].TwoWay {
		t.Fatal("neighbor not bidirectional after a Hello listing this router's Router ID")
	}
}

// RFC requirement: RFC2328-10.5-1 negative -- a Hello whose Network Mask, HelloInterval or RouterDeadInterval differs from the receiving interface's is rejected with the mismatching field named and creates no neighbor, and a Hello that does not list this router's Router ID never makes the neighbor bidirectional (validateHelloLocked, receiveHello, iface.go).
func TestRFC2328HelloMismatchRejectedAndNoTwoWayUnlisted(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	cases := []struct {
		name   string
		mutate func(h *packet.Hello)
		want   string
	}{
		{"network-mask", func(h *packet.Hello) { h.NetworkMask = [4]byte{255, 255, 0, 0} }, DropReasonNetworkMask},
		{"hello-interval", func(h *packet.Hello) { h.HelloInterval++ }, "hello-interval"},
		{"dead-interval", func(h *packet.Hello) { h.DeadInterval++ }, "dead-interval"},
	}
	for _, c := range cases {
		h := helloFor(cfg, cfg.RouterID)
		c.mutate(&h)
		if got := ifc.ReceiveHello(peer, h, time.Now()); got != c.want {
			t.Fatalf("%s: reason = %q, want %q", c.name, got, c.want)
		}
		if n, known := ifc.neighbors[peer]; known {
			t.Fatalf("%s: a rejected Hello created neighbor %+v", c.name, n)
		}
	}
	other := rid(t, "10.0.0.3")
	for range 3 {
		if reason := ifc.ReceiveHello(peer, helloFor(cfg, other), time.Now()); reason != "" {
			t.Fatalf("Hello listing another router refused: %s", reason)
		}
	}
	if n, known := ifc.neighbors[peer]; !known || n.TwoWay {
		t.Fatalf("neighbor = %+v known=%v, want known and never bidirectional while this router is unlisted", n, known)
	}
}
