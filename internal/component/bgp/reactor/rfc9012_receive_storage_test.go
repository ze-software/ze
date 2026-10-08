// Design: docs/architecture/wire/attributes.md -- carrier-aware tunnel validation.
// Related: rfc9830_tunnel_receive_test.go -- structural verdict and forwarding cases.
// RFC 9012 Sections 6 and 13 -- see rfc/short/rfc9012.md.
package reactor

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgprib "github.com/ze-software/ze/internal/component/bgp/plugins/rib"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC9012ReceiveRemovesInstalledNativeRoutes observes actual received-route
// storage after malformed replacements, not a validation enum or a mock RIB.
// RFC 9012 Section 13: "If this is not the case, the TLV MUST be considered to be
// malformed, and the \"Treat-as-withdraw\" procedure of [RFC7606] is applied."
// RFC 9012 Section 13: "Within a Tunnel Encapsulation attribute that is carried
// by a BGP UPDATE whose AFI/SAFI is one of those explicitly listed in the first
// paragraph of Section 6, a TLV that does not contain exactly one Tunnel Egress
// Endpoint sub-TLV MUST be treated as if it contained a malformed Tunnel Egress
// Endpoint sub-TLV."
// RFC requirement: RFC9012-13-2 positive -- malformed framing removes an already installed route on each Section 6 carrier while a distinct control route survives.
// RFC requirement: RFC9012-13-2 negative -- framed unknown sub-TLVs install and reinstall the native route on the same established session.
// RFC requirement: RFC9012-13-15 positive -- a valid endpoint TLV survives beside a missing or duplicate endpoint TLV in actual received-route storage.
// RFC requirement: RFC9012-13-15 negative -- missing and duplicate endpoints with no surviving TLV remove the installed native route, retaining the independent control.
// MUTATION: returning the original UPDATE from applyTunnelEncap leaves malformed replacements installed; forcing requireEndpoint false retains missing and duplicate endpoints on MP carriers.
// RFC requirement: RFC9012-3.1-8 positive -- exactly one endpoint installs a real received route on every Section 6 carrier.
// RFC requirement: RFC9012-3.1-8 negative -- missing or duplicate endpoints remove their TLVs; a marked valid sibling survives, while no survivor withdraws only the installed target before same-session reinstall.
func TestRFC9012ReceiveRemovesInstalledNativeRoutes(t *testing.T) {
	families := []family.Family{
		family.IPv4Unicast, family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
		{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
	}
	for _, fam := range families {
		t.Run(fam.String(), func(t *testing.T) {
			peer, client := tunnelStoragePeer(t, fam)
			session := peer.currentSession()
			route := ownershipRailNative(fam, 0, 1, false, 0)[4:]
			if fam.SAFI == family.SAFIEVPN {
				// RFC 7432 Section 7.3: type 3, RD, Ethernet Tag, IP length, IP.
				route = []byte{3, 17, 0, 0, 0xfd, 0xe8, 0, 0, 0, 7, 0, 0, 0, 0, 32, 192, 0, 2, 1}
			}
			control := bytes.Clone(route)
			control[len(control)-1]++
			endpoint := teSub(6, 0, 0, 0, 0, 0, 0)
			valid := teTLV(2, endpoint, teSub(99, 1))
			tunnelStorageSend(t, client, fam, control, valid)
			for _, tc := range []struct {
				name    string
				bad     []byte
				sibling bool
			}{
				{"framing", teTLV(2, endpoint, []byte{99, 2, 1}), false},
				{"missing", teTLV(2, teSub(99, 1)), true},
				{"duplicate", teTLV(2, endpoint, endpoint), true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					tunnelStorageSend(t, client, fam, route, valid)
					tunnelStorageAwait(t, fam, route, control, valid, true)
					if tc.sibling {
						// A changed valid sibling is a causal marker: an old valid
						// snapshot cannot satisfy the receive-rewrite assertion.
						marked := teTLV(2, endpoint, teSub(99, 2))
						mixed := append(bytes.Clone(tc.bad), marked...)
						tunnelStorageSend(t, client, fam, route, mixed)
						tunnelStorageAwait(t, fam, route, control, marked, true)
					}
					tunnelStorageSend(t, client, fam, route, tc.bad)
					tunnelStorageAwait(t, fam, route, control, nil, false)
					tunnelStorageSend(t, client, fam, route, valid)
					tunnelStorageAwait(t, fam, route, control, valid, true)
					if peer.currentSession() != session || session.State() != fsm.StateEstablished {
						t.Fatal("treat-as-withdraw replaced or reset the session")
					}
				})
			}
		})
	}
}

// TestRFC9012EndpointAddressRegistryStorage replaces an installed native route
// over the established socket, observing the real RIB after each transition.
// A marked valid sibling proves the mixed UPDATE was processed, not an old
// snapshot; an independent route must survive all-invalid treat-as-withdraw.
// RFC 9012 Section 13: "If a Tunnel Encapsulation attribute does not have any
// valid TLVs, or it does not have the transitive bit set, the \"Treat-as-withdraw\"
// procedure of [RFC7606] is applied."
// RFC requirement: RFC9012-13-14 positive -- IANA-prohibited endpoints are removed from stored received routes on every Section 6 carrier, or withdraw the target when no TLV survives.
// RFC requirement: RFC9012-13-14 negative -- a marked valid sibling and independent control route survive, and the same established session accepts a subsequent valid replacement.
// MUTATION: bypassing endpoint address classification retains the bad sibling
// and prevents removal of the already installed target route.
// RFC requirement: RFC9012-13-13 positive -- registry-malformed endpoint TLVs are removed wholesale before native received-route storage on every Section 6 carrier.
// RFC requirement: RFC9012-13-13 negative -- marked valid siblings and independent routes survive; allowed endpoint controls remain installed.
func TestRFC9012EndpointAddressRegistryStorage(t *testing.T) {
	for _, fam := range []family.Family{
		family.IPv4Unicast, family.IPv6Unicast,
		{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel},
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
		{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN},
	} {
		t.Run(fam.String(), func(t *testing.T) {
			peer, client := tunnelStoragePeer(t, fam)
			session := peer.currentSession()
			route := ownershipRailNative(fam, 0, 1, false, 0)[4:]
			if fam.SAFI == family.SAFIEVPN {
				route = []byte{3, 17, 0, 0, 0xfd, 0xe8, 0, 0, 0, 7, 0, 0, 0, 0, 32, 192, 0, 2, 1}
			}
			control := bytes.Clone(route)
			control[len(control)-1]++
			controlValue := teTLV(2, teSub(6, 0, 0, 0, 0, 0, 0), teSub(99, 1))
			tunnelStorageSend(t, client, fam, control, controlValue)
			for _, address := range []string{"192.0.2.1", "169.254.1.1", "2001:db8::1", "fe80::1"} {
				t.Run(address, func(t *testing.T) {
					valid := teTLV(2, teRegistryEndpoint("fd00::1"), teSub(99, 1))
					tunnelStorageSend(t, client, fam, route, valid)
					tunnelStorageAwait(t, fam, route, control, valid, true)
					marked := teTLV(2, teRegistryEndpoint("10.0.0.77"), teSub(99, 2))
					bad := teTLV(8, teRegistryEndpoint(address), teSub(99, 3))
					mixed := append(bytes.Clone(bad), marked...)
					mixed = append(mixed, bad...)
					// RFC 9012 Sections 3.1 and 13.
					tunnelStorageSend(t, client, fam, route, mixed)
					tunnelStorageAwait(t, fam, route, control, marked, true)
					// RFC 9012 Section 13: no survivor removes only this NLRI.
					tunnelStorageSend(t, client, fam, route, bad)
					tunnelStorageAwait(t, fam, route, control, nil, false)
					tunnelStorageSend(t, client, fam, route, valid)
					tunnelStorageAwait(t, fam, route, control, valid, true)
					if peer.currentSession() != session || session.State() != fsm.StateEstablished {
						t.Fatal("address treat-as-withdraw replaced or reset the session")
					}
				})
			}
		})
	}
}

// tunnelStoragePeer MUST be called before sending; cleanup MUST close the pipe
// before stopping the reactor, whose owned session reader otherwise waits on it.
func tunnelStoragePeer(t *testing.T, fam family.Family) (*Peer, net.Conn) {
	t.Helper()
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	settings := lowLiveSettings("192.0.2.1", 65000, 65002)
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI},
	}
	if err := EnsureProcessBinding(settings, "bgp-rib", "update state refresh", "update"); err != nil {
		t.Fatal(err)
	}
	if err := r.AddPeer(settings); err != nil {
		t.Fatal(err)
	}
	srv := newBorrowedPluginServer(t, r, "bgp-rib")
	r.SetPluginServer(srv)
	if err := r.StartWithContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopAndWait(t, r) })
	startBorrowedPeers(t, r, srv)
	peer := r.peers[settings.PeerKey()]
	lowEventually(t, func() bool { return peer.currentSession() != nil && peer.SessionState() == fsm.StateActive }, "tunnel peer active")
	server, client := net.Pipe()
	// MUST close both pipe endpoints before the reactor cleanup joins readers.
	t.Cleanup(func() { _ = client.Close(); _ = server.Close() })
	startDrain(t, client)
	if err := peer.acceptConnection(server); err != nil {
		t.Fatal(err)
	}
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x0a000001,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xfd, 0xea, 2, 6, 1, 4, byte(fam.AFI >> 8), byte(fam.AFI), 0, byte(fam.SAFI)}}
	if _, err := client.Write(message.PackTo(open, nil)); err != nil {
		t.Fatal(err)
	}
	lowEventually(t, func() bool { return peer.SessionState() == fsm.StateOpenConfirm }, "tunnel OPEN accepted")
	if _, err := client.Write(message.PackTo(message.NewKeepalive(), nil)); err != nil {
		t.Fatal(err)
	}
	lowEventually(t, func() bool { return peer.State() == PeerStateEstablished && !peer.pendingSync() }, "tunnel peer established")
	if !peer.currentSession().Negotiated().SupportsFamily(fam) {
		t.Fatalf("fixture failed to negotiate %s", fam)
	}
	return peer, client
}

func tunnelStorageSend(t *testing.T, client net.Conn, fam family.Family, route, tunnel []byte) {
	t.Helper()
	body := ownershipRailBody(fam, false, route, 0)
	update, err := message.UnpackUpdate(body)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 4271 Section 5.1.2: match the real external peer's first AS.
	_, _, path, found := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
	if !found {
		t.Fatal("fixture has no ASN4 path")
	}
	if len(path) != 6 {
		t.Fatal("fixture has no single-AS ASN4 path")
	}
	copy(path, []byte{2, 1, 0, 0, 0xfd, 0xea})
	update.PathAttributes = append(update.PathAttributes, makeAttr(0xc0, 23, tunnel)...)
	if _, err := client.Write(message.PackTo(update, nil)); err != nil {
		t.Fatal(err)
	}
}

func tunnelStorageAwait(t *testing.T, fam family.Family, route, control, want []byte, present bool) {
	t.Helper()
	// A failed inventory assertion must expose what the real store contained,
	// including another family, rather than report an opaque polling timeout.
	defer func() {
		if !t.Failed() {
			return
		}
		t.Logf("wanted family=%s target=%x control=%x tunnel=%x present=%v", fam, route, control, want, present)
		bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
			OnPeer: func(address string, asn uint32, id [4]byte, ipv6 bool) uint16 {
				t.Logf("stored peer=%s asn=%d id=%x ipv6=%v", address, asn, id, ipv6)
				return 1
			},
			OnRoute: func(peer, afi, safi uint16, bits uint8, native, attrs []byte) {
				t.Logf("stored peer=%d family=%d/%d bits=%d native=%x attrs=%x", peer, afi, safi, bits, native, attrs)
			},
		})
	}()
	lowEventually(t, func() bool {
		seenRoute, seenControl, exact := false, false, false
		count := 0
		bgprib.RIBDumpBridge.DumpRIB(registry.RIBDumpVisitor{
			OnPeer: func(address string, _ uint32, _ [4]byte, _ bool) uint16 {
				if address == "192.0.2.1" {
					return 1
				}
				return 0
			},
			OnRoute: func(peer, afi, safi uint16, bits uint8, native, attrs []byte) {
				if peer != 1 {
					return
				}
				if afi != uint16(fam.AFI) {
					return
				}
				if safi != uint16(fam.SAFI) {
					return
				}
				count++
				key := native
				if fam.SAFI == family.SAFIUnicast {
					key = append([]byte{bits}, native...)
				}
				if tunnelStorageMatches(fam, key, attrs, control) {
					_, _, value, found := attribute.AttrFind(attrs, attribute.AttrTunnelEncap)
					seenControl = found && bytes.Equal(value, teTLV(2, teSub(6, 0, 0, 0, 0, 0, 0), teSub(99, 1)))
				}
				if tunnelStorageMatches(fam, key, attrs, route) {
					seenRoute = true
					_, _, value, found := attribute.AttrFind(attrs, attribute.AttrTunnelEncap)
					exact = found && bytes.Equal(value, want)
				}
			},
		})
		if present {
			return count == 2 && seenRoute && seenControl && exact
		}
		return count == 1 && !seenRoute && seenControl
	}, "exact native received-route inventory after tunnel processing")
}

// tunnelStorageMatches observes the RIB snapshot, not the MRT file format.
// insertLabeledEntry stores SAFI 4 under a CIDR key with separate label bindings.
// Its original MP_REACH still has to carry this exact family and labeled NLRI;
// accepting a matching CIDR alone would lose the native label assertion.
func tunnelStorageMatches(fam family.Family, key, attrs, wire []byte) bool {
	if fam.SAFI != family.SAFIMPLSLabel {
		return bytes.Equal(key, wire)
	}
	// This fixture announces one three-octet label, with no ADD-PATH header.
	if len(wire) < 4 {
		return false
	}
	cidr := append([]byte{wire[0] - 24}, wire[4:]...)
	if !bytes.Equal(key, cidr) {
		return false
	}
	_, _, reach, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return false
	}
	if len(reach) < 5 {
		return false
	}
	if reach[0] != byte(fam.AFI>>8) {
		return false
	}
	if reach[1] != byte(fam.AFI) {
		return false
	}
	if reach[2] != byte(fam.SAFI) {
		return false
	}
	nlriOffset := 5 + int(reach[3])
	if nlriOffset > len(reach) {
		return false
	}
	return bytes.Equal(reach[nlriOffset:], wire)
}
