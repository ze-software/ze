// VALIDATES: RFC 5883 encapsulation and Echo rules for multihop BFD.
// Multihop Control packets use UDP destination port 4784, distinct from
// the single-hop 3784 port (RFC 5883 sec 5), so the two coexist on
// separate sockets; and the Echo function MUST NOT run on a multihop
// path (RFC 5883 sec 3), so config validation rejects an echo profile on
// a multi-hop session while permitting it on single-hop.
// PREVENTS: a multihop session binding the single-hop port (blending the
// two demux domains) and an operator enabling Echo on a multihop path
// (whose reflected packet proves nothing about end-to-end liveness).
package bfd

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// RFC requirement: RFC5883-5-1 positive -- multihop BFD Control packets MUST
// use UDP destination port 4784 (RFC 5883 sec 5). The producer
// newUDPTransport (internal/component/bfd/bfd.go:355-359) selects
// transport.UDPPortMultiHopControl (=4784, internal/component/bfd/transport/udp.go:48)
// when the loop's HopMode is api.MultiHop.
func TestRFC5883MultiHopControlPort(t *testing.T) {
	tr := newUDPTransport(api.MultiHop, "", "")
	if got := tr.Bind.Port(); got != transport.UDPPortMultiHopControl {
		t.Fatalf("multihop bind port = %d, want UDPPortMultiHopControl", got)
	}
	if transport.UDPPortMultiHopControl != 4784 {
		t.Fatalf("UDPPortMultiHopControl = %d, want 4784 (RFC 5883 sec 5)", transport.UDPPortMultiHopControl)
	}
}

// RFC requirement: RFC5883-5-1 negative -- a multihop session MUST NOT reuse
// the single-hop 3784 port; the mode selector in newUDPTransport
// (internal/component/bfd/bfd.go:357-359) diverts MultiHop away from
// UDPPortSingleHopControl. The single-hop path keeps 3784 as the contrast, so
// this pins port selection to the mode rather than a constant either way.
func TestRFC5883MultiHopControlPortNotSingleHop(t *testing.T) {
	mh := newUDPTransport(api.MultiHop, "", "").Bind.Port()
	if mh == transport.UDPPortSingleHopControl {
		t.Fatalf("multihop bound the single-hop port %d; RFC 5883 sec 5 requires 4784", mh)
	}
	if sh := newUDPTransport(api.SingleHop, "", "").Bind.Port(); sh != transport.UDPPortSingleHopControl {
		t.Fatalf("single-hop bind port = %d, want 3784 (RFC 5881); contrast to multihop is what pins the selector", sh)
	}
}

// RFC requirement: RFC5883-5-2 positive -- single-hop and multihop bind
// SEPARATE UDP ports so RX sockets demux by session type; the single-hop
// socket binds the RFC 5881 port 3784. Producer newUDPTransport
// (internal/component/bfd/bfd.go:356) defaults to
// transport.UDPPortSingleHopControl for api.SingleHop.
func TestRFC5883SingleHopControlPort(t *testing.T) {
	sh := newUDPTransport(api.SingleHop, "", "").Bind.Port()
	if sh != transport.UDPPortSingleHopControl {
		t.Fatalf("single-hop bind port = %d, want UDPPortSingleHopControl", sh)
	}
	if transport.UDPPortSingleHopControl != 3784 {
		t.Fatalf("UDPPortSingleHopControl = %d, want 3784 (RFC 5881)", transport.UDPPortSingleHopControl)
	}
}

// RFC requirement: RFC5883-5-2 negative -- the multihop socket binds a port
// DISTINCT from single-hop, so a single-hop and a multihop session never
// collapse onto one socket. newUDPTransport (internal/component/bfd/bfd.go:355-359)
// yields a different port per mode; equal ports would mean one shared socket
// and is the failure this negative excludes.
func TestRFC5883SeparatePortsPerMode(t *testing.T) {
	sh := newUDPTransport(api.SingleHop, "", "").Bind.Port()
	mh := newUDPTransport(api.MultiHop, "", "").Bind.Port()
	if sh == mh {
		t.Fatalf("single-hop and multihop share port %d; RFC 5883 sec 5 requires separate sockets per mode", sh)
	}
	if mh != 4784 {
		t.Fatalf("multihop bind port = %d, want 4784 distinct from single-hop 3784", mh)
	}
}

// echoProfileConfig returns a pluginConfig with one profile that has Echo
// enabled and one session bound to it in the requested mode. It is the
// minimal input to exercise pluginConfig.validate's multihop-echo guard.
func echoProfileConfig(mode api.HopMode, iface string) *pluginConfig {
	return &pluginConfig{
		profiles: map[string]profileConfig{
			"p": {name: "p", echo: &echoConfig{desiredMinEchoTxUs: 50_000}},
		},
		sessions: []sessionConfig{
			{
				mode:    mode,
				peer:    netip.MustParseAddr("203.0.113.1"),
				iface:   iface,
				profile: "p",
			},
		},
	}
}

// RFC requirement: RFC5883-3-1 negative -- Echo MUST NOT run on a multihop
// path (RFC 5883 sec 3), so pluginConfig.validate
// (internal/component/bfd/config.go:483-496) rejects a multi-hop session
// referencing an echo-enabled profile.
// RFC requirement: RFC5883-x-1 negative -- same prohibition restated in the
// Pitfalls: a multihop implementation MUST reject enabling Echo on a
// multihop session; the single guard in config.go:492-493 enforces both.
func TestRFC5883MultiHopEchoRejected(t *testing.T) {
	cfg := echoProfileConfig(api.MultiHop, "")
	if err := cfg.validate(); err == nil {
		t.Fatalf("validate accepted Echo on a multi-hop session; RFC 5883 sec 3 forbids it")
	}
}

// RFC requirement: RFC5883-3-1 positive -- the Echo prohibition is scoped to
// multihop: pluginConfig.validate (internal/component/bfd/config.go:485)
// skips single-hop sessions, so an echo-enabled profile on a single-hop
// session is ACCEPTED (Echo is valid over a single hop, RFC 5881).
// RFC requirement: RFC5883-x-1 positive -- same scoping: the Pitfalls
// prohibition applies only to multihop, so single-hop Echo passes validation.
func TestRFC5883SingleHopEchoAccepted(t *testing.T) {
	cfg := echoProfileConfig(api.SingleHop, "eth0")
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate rejected Echo on a single-hop session, which RFC 5881 permits: %v", err)
	}
}
