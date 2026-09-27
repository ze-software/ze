// VALIDATES: RFC 5881 sec 4 UDP encapsulation for single-hop BFD. Control
// packets use UDP destination port 3784 and Echo packets use 3785; a single
// bound socket per (vrf,mode) loop fixes the source port for every Control
// packet in a session. The port is selected by role (Control vs Echo), so the
// two never collapse onto one number.
// PREVENTS: a Control transport binding a wrong or Echo port, an Echo transport
// binding the Control port, and a design that could vary the Control source
// port packet-to-packet.
package bfd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
)

// RFC requirement: RFC5881-4-1 positive -- BFD Control packets MUST be
// transmitted in UDP with destination port 3784. newUDPTransport
// (internal/component/bfd/bfd.go:356,360) binds the single-hop Control socket
// to transport.UDPPortSingleHopControl, and UDP.Send
// (internal/component/bfd/transport/udp.go:225) targets Bind.Port, so 3784 is
// both the bound and the destination port.
func TestRFC5881ControlDestPort3784(t *testing.T) {
	tr := newUDPTransport(api.SingleHop, "", "")
	if got := tr.Bind.Port(); got != transport.UDPPortSingleHopControl {
		t.Fatalf("single-hop Control bind port = %d, want UDPPortSingleHopControl", got)
	}
	if transport.UDPPortSingleHopControl != 3784 {
		t.Fatalf("UDPPortSingleHopControl = %d, want 3784 (RFC 5881 sec 4)", transport.UDPPortSingleHopControl)
	}
}

// RFC requirement: RFC5881-4-1 negative -- 3784 is the Control port
// specifically, not a constant applied to every BFD socket. The Echo transport
// (internal/component/bfd/bfd.go:394) binds transport.UDPPortEcho, which is not
// 3784, so a non-Control socket does not carry the Control destination port.
// Without this the positive could pass on code that hard-coded 3784 everywhere.
func TestRFC5881ControlPortNotUsedForEcho(t *testing.T) {
	echo := newEchoTransport("", "")
	if echo.Bind.Port() == transport.UDPPortSingleHopControl {
		t.Fatalf("Echo transport bound the Control port %d; RFC 5881 sec 4 assigns Echo a distinct port", echo.Bind.Port())
	}
}

// RFC requirement: RFC5881-4-5 positive -- BFD Echo packets MUST be transmitted
// in UDP with destination port 3785. newEchoTransport
// (internal/component/bfd/bfd.go:394) binds the Echo socket to
// transport.UDPPortEcho, and UDP.Send (transport/udp.go:225) targets Bind.Port.
func TestRFC5881EchoDestPort3785(t *testing.T) {
	echo := newEchoTransport("", "")
	if got := echo.Bind.Port(); got != transport.UDPPortEcho {
		t.Fatalf("Echo bind port = %d, want UDPPortEcho", got)
	}
	if transport.UDPPortEcho != 3785 {
		t.Fatalf("UDPPortEcho = %d, want 3785 (RFC 5881 sec 4)", transport.UDPPortEcho)
	}
}

// RFC requirement: RFC5881-4-5 negative -- 3785 is the Echo port specifically.
// The Control transport (internal/component/bfd/bfd.go:356) binds 3784, not the
// Echo port, so the Echo destination port is not applied to the Control socket.
// Without this the positive could pass on code that hard-coded 3785 everywhere.
func TestRFC5881EchoPortNotUsedForControl(t *testing.T) {
	tr := newUDPTransport(api.SingleHop, "", "")
	if tr.Bind.Port() == transport.UDPPortEcho {
		t.Fatalf("Control transport bound the Echo port %d; the two ports must differ", tr.Bind.Port())
	}
}

// RFC requirement: RFC5881-4-3 positive -- the same UDP source port MUST be
// used for all Control packets in a session. Each (vrf,mode) loop binds ONE UDP
// socket (internal/component/bfd/bfd.go:355-367) and UDP.Send reuses that one
// socket for every transmission (internal/component/bfd/transport/udp.go:218-228,
// conn.WriteToUDP), so the source port is the single fixed Bind.Port for the
// whole session -- there is no per-packet source-port selection that could make
// two Control packets differ.
func TestRFC5881SingleSourcePortPerSession(t *testing.T) {
	tr := newUDPTransport(api.SingleHop, "", "")
	// A single AddrPort, not a range: one socket, one source port.
	if !tr.Bind.IsValid() {
		t.Fatal("single-hop Control transport has no bound address")
	}
	if got := tr.Bind.Port(); got != transport.UDPPortSingleHopControl {
		t.Fatalf("Control source port = %d, want the single fixed port %d", got, transport.UDPPortSingleHopControl)
	}
	// A second transport for the same mode binds the same fixed port: the
	// source port is a property of the socket, not chosen per packet.
	if other := newUDPTransport(api.SingleHop, "", "").Bind.Port(); other != tr.Bind.Port() {
		t.Fatalf("Control source port varied between transports (%d vs %d); it must be the fixed bound port", other, tr.Bind.Port())
	}
}

// passiveProfileConfig builds a pluginConfig whose one session, in mode,
// references a profile with the passive leaf set as given.
func passiveProfileConfig(mode api.HopMode, iface string, passive bool) *pluginConfig {
	return &pluginConfig{
		profiles: map[string]profileConfig{
			"p": {name: "p", passive: passive},
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

// RFC requirement: RFC5881-3-1 negative -- both sides of a single-hop session
// MUST take the Active role, so pluginConfig.validate
// (internal/component/bfd/config.go) refuses a single-hop session that
// references a profile with passive set, and the configuration never reaches
// the engine.
func TestRFC5881SingleHopPassiveProfileRejected(t *testing.T) {
	cfg := passiveProfileConfig(api.SingleHop, "eth0", true)
	if err := cfg.validate(); err == nil {
		t.Fatal("validate accepted a Passive single-hop session; RFC 5881 Section 3 requires the Active role")
	}
}

// RFC requirement: RFC5881-3-1 positive -- a single-hop session whose profile
// leaves passive unset validates and asks the engine for the Active role
// (toSessionRequest carries Passive false). The refusal is scoped to single
// hop: a multi-hop session may still take the Passive role (RFC 5883 Section
// 4.3), so a blanket refusal of the leaf would fail here.
func TestRFC5881SingleHopActiveProfileAccepted(t *testing.T) {
	cfg := passiveProfileConfig(api.SingleHop, "eth0", false)
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate rejected an Active single-hop session: %v", err)
	}
	if req := cfg.sessions[0].toSessionRequest(cfg.profiles); req.Passive {
		t.Fatal("single-hop session request carries Passive")
	}
	multi := passiveProfileConfig(api.MultiHop, "", true)
	if err := multi.validate(); err != nil {
		t.Fatalf("validate rejected a Passive multi-hop session, which RFC 5883 Section 4.3 permits: %v", err)
	}
}

// RFC requirement: RFC5880-6.7.3-13 positive -- the config parser accepts a
// 16-byte secret for keyed-md5 and meticulous-keyed-md5 (parseAuthConfig,
// internal/component/bfd/config.go).
// RFC requirement: RFC5880-6.7.3-13 negative -- parseAuthConfig refuses a
// 17-byte secret for the two MD5 types.
// RFC requirement: RFC5880-6.7.4-7 positive -- parseAuthConfig accepts a
// 20-byte secret for keyed-sha1 and meticulous-keyed-sha1.
// RFC requirement: RFC5880-6.7.4-7 negative -- parseAuthConfig refuses a
// 21-byte secret for the two SHA1 types.
//
// VALIDATES: parseAuthConfig refuses a keyed secret longer than its Auth
// Key/Digest slot, 16 bytes for the MD5 types (RFC 5880 Section 6.7.3) and 20
// for the SHA1 types (Section 6.7.4), and accepts one that fills it exactly.
// PREVENTS: a load that silently truncates the configured key.
func TestParseAuthConfigKeyedSecretLength(t *testing.T) {
	cases := []struct {
		authType string
		slot     int
	}{
		{"keyed-md5", 16},
		{"meticulous-keyed-md5", 16},
		{"keyed-sha1", 20},
		{"meticulous-keyed-sha1", 20},
	}
	for _, tc := range cases {
		fields := map[string]any{"type": tc.authType, "key-id": "1", "secret": strings.Repeat("k", tc.slot)}
		if _, err := parseAuthConfig("p", fields); err != nil {
			t.Fatalf("%s: a %d-byte secret rejected: %v", tc.authType, tc.slot, err)
		}
		fields["secret"] = strings.Repeat("k", tc.slot+1)
		if _, err := parseAuthConfig("p", fields); err == nil {
			t.Fatalf("%s: a %d-byte secret accepted", tc.authType, tc.slot+1)
		}
	}
}
