package ifacevpp

import (
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/gre"
	"go.fd.io/govpp/binapi/ipip"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// newTunnelBackend returns a backend wired to a programmable channel with a
// deterministic SwIfIndex for add replies.
func newTunnelBackend(ch *progChannel) *vppBackendImpl {
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {}) // short-circuit ensureChannel populate
	return b
}

// TestCreateTunnelGRE verifies AC-2: a gre tunnel under the vpp backend issues
// a gre_tunnel_add_del with type L3, the resolved endpoints, and registers the
// name->SwIfIndex mapping.
// VALIDATES: AC-2 -- GRE tunnel programmed on VPP.
// PREVENTS: regression to the errNotSupported stub.
func TestCreateTunnelGRE(t *testing.T) {
	ch := &progChannel{swIfIndex: 7}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel gre: %v", err)
	}
	req, ok := ch.requests[0].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *gre.GreTunnelAddDel", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Tunnel.Type != gre.GRE_API_TUNNEL_TYPE_L3 {
		t.Errorf("Type: got %v, want L3", req.Tunnel.Type)
	}
	if got := req.Tunnel.Src.ToIP().String(); got != "192.0.2.1" {
		t.Errorf("Src: got %s, want 192.0.2.1", got)
	}
	if got := req.Tunnel.Dst.ToIP().String(); got != "192.0.2.2" {
		t.Errorf("Dst: got %s, want 192.0.2.2", got)
	}
	if idx, ok := b.names.LookupIndex("gre0"); !ok || idx != 7 {
		t.Errorf("name map: got (%d,%v), want (7,true)", idx, ok)
	}
}

// TestCreateTunnelGRETap verifies AC-2: gretap maps to the TEB (transparent
// ethernet bridging) gre tunnel type.
// VALIDATES: AC-2 -- GRETAP programmed as TEB.
// PREVENTS: gretap silently becoming an L3 gre tunnel.
func TestCreateTunnelGRETap(t *testing.T) {
	ch := &progChannel{swIfIndex: 9}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRETap,
		Name:          "gt0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel gretap: %v", err)
	}
	req, ok := ch.requests[0].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("request type: got %T, want *gre.GreTunnelAddDel", ch.requests[0])
	}
	if req.Tunnel.Type != gre.GRE_API_TUNNEL_TYPE_TEB {
		t.Errorf("Type: got %v, want TEB", req.Tunnel.Type)
	}
}

// TestCreateTunnelGRERejectsKey verifies the honest exact-or-reject behavior:
// the v0.13.0 gre_tunnel API has no key field, so a configured key is rejected
// rather than silently dropped.
// VALIDATES: AC-2 -- no silent GRE-key drop.
// PREVENTS: an operator believing a keyed GRE tunnel was programmed when it was not.
func TestCreateTunnelGRERejectsKey(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
		Key:           42,
		KeySet:        true,
	})
	if err == nil {
		t.Fatal("expected error for GRE key on VPP backend, got nil")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("expected 'key' in error, got: %v", err)
	}
}

// TestCreateTunnelIPIP verifies AC-2: an ipip tunnel issues ipip_add_tunnel
// with the resolved endpoints.
// VALIDATES: AC-2 -- IPIP tunnel programmed on VPP.
// PREVENTS: regression to the errNotSupported stub.
func TestCreateTunnelIPIP(t *testing.T) {
	ch := &progChannel{swIfIndex: 3}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindIPIP,
		Name:          "ipip0",
		LocalAddress:  "203.0.113.1",
		RemoteAddress: "203.0.113.2",
	})
	if err != nil {
		t.Fatalf("CreateTunnel ipip: %v", err)
	}
	req, ok := ch.requests[0].(*ipip.IpipAddTunnel)
	if !ok {
		t.Fatalf("request type: got %T, want *ipip.IpipAddTunnel", ch.requests[0])
	}
	if got := req.Tunnel.Dst.ToIP().String(); got != "203.0.113.2" {
		t.Errorf("Dst: got %s, want 203.0.113.2", got)
	}
	if idx, ok := b.names.LookupIndex("ipip0"); !ok || idx != 3 {
		t.Errorf("name map: got (%d,%v), want (3,true)", idx, ok)
	}
}

// TestCreateTunnelLocalInterfaceRejected verifies a local-interface source is
// rejected on VPP (VPP terminates tunnels on an address, not an ifindex).
// VALIDATES: AC-2 -- no silent ignoring of local-interface source.
// PREVENTS: a tunnel created with the wrong source when local { interface } is set.
func TestCreateTunnelLocalInterfaceRejected(t *testing.T) {
	ch := &progChannel{}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:           iface.TunnelKindGRE,
		Name:           "gre0",
		LocalInterface: "xe0",
		RemoteAddress:  "192.0.2.2",
	})
	if err == nil {
		t.Fatal("expected error for local-interface source on VPP, got nil")
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected on rejection, got %d", len(ch.requests))
	}
}

// TestCreateTunnelUnsupportedKind verifies a netlink-only kind is rejected
// (defense in depth behind the ze:backend commit gate).
// VALIDATES: AC-2/R-2 -- exact-or-reject for unwired kinds.
// PREVENTS: a widened annotation silently no-oping sit/ip6tnl on VPP.
func TestCreateTunnelUnsupportedKind(t *testing.T) {
	ch := &progChannel{}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindSIT,
		Name:          "sit0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	})
	if err == nil {
		t.Fatal("expected error for sit tunnel on VPP, got nil")
	}
}

// TestDeleteTunnelGRE verifies the delete path issues gre_tunnel_add_del with
// IsAdd=false and clears the name map.
// VALIDATES: AC-2 -- clean tunnel delete path.
// PREVENTS: stale VPP tunnels / name-map entries after a config removal.
func TestDeleteTunnelGRE(t *testing.T) {
	ch := &progChannel{swIfIndex: 5}
	b := newTunnelBackend(ch)

	if err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	}); err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if err := b.DeleteInterface("gre0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*gre.GreTunnelAddDel)
	if !ok {
		t.Fatalf("delete request type: got %T, want *gre.GreTunnelAddDel", ch.requests[len(ch.requests)-1])
	}
	if last.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	if _, ok := b.names.LookupIndex("gre0"); ok {
		t.Error("name map still has gre0 after delete")
	}
}
