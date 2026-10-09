//go:build integration && linux

// Design: docs/architecture/update-building.md -- configured routes and the Session writer.
// Related: ../config/peers.go -- native configuration supplies the static route set.
package reactor_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/vpn"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4659ConfiguredIPv6TransportNextHopWire parses native configuration,
// starts its Peer, and inspects UPDATE bodies delivered to a real recipient
// Session over TCP6. No builder supplies the oracle or the route input.
//
// The fixture creates independent speaker and recipient network namespaces with
// a veth path and three global IPv6 /128 addresses. Interface enumeration proves
// ownership independently of configuration; the recipient cannot own either
// speaker address. The accepted TCP endpoint identifies next-hop self, while an
// additional speaker loopback address proves explicit selection is not self.
// Distinct /128s exclude the common-subnet/link-local branch. This proves no
// tunnel creation, mapped transport, 48-octet origination, or PE dataplane.
//
// RFC requirement: RFC4659-3.2.1.1-1 positive -- operator-configured IPv6 VPN
// transport origination emits exactly zero RD plus the speaker-owned global
// IPv6 address, with self and explicit selection under both grouping modes.
// MUTATION: in buildMPReachVPN, only for afi == 2 and nhLen == 24, replace
// value[12:28] with another valid global IPv6 address after its copy; alternatively
// set value[4] = 1 after clearing the RD. The recipient must reject either wire
// contract violation, even if the Session admission layer accepts the UPDATE.
func TestRFC4659ConfiguredIPv6TransportNextHopWire(t *testing.T) {
	speaker := netip.MustParseAddr("2001:db8:4659:1::1")
	recipient := netip.MustParseAddr("2001:db8:4659:2::2")
	selected := netip.MustParseAddr("2001:db8:4659:3::1")
	for _, grouped := range []bool{false, true} {
		for _, selection := range []string{"self", "explicit"} {
			t.Run("grouped="+strconv.FormatBool(grouped)+"/"+selection, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				senderNS, listener := rfc4659TransportFixture(t, speaker, recipient, selected)
				endpoint, ok := listener.Addr().(*net.TCPAddr)
				if !ok {
					t.Fatal("recipient listener has no TCP endpoint")
				}
				hop := "self"
				if selection == "explicit" {
					hop = selected.String()
				}
				settings := rfc4659ConfiguredSettings(t, speaker, recipient, endpoint.Port, hop, grouped)
				peer := reactor.NewPeer(settings)
				peer.SetDialer(&rfc4659NamespaceDialer{namespace: senderNS, local: settings.LocalAddress})
				peer.StartWithContext(ctx)
				defer func() {
					peer.Stop()
					waitCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
					defer stop()
					if err := peer.Wait(waitCtx); err != nil {
						t.Error(err)
					}
				}()
				// The listener deadline bounds establishment without timing a sleep.
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("fixture context has no deadline")
				}
				tcpListener, ok := listener.(*net.TCPListener)
				if !ok {
					t.Fatal("recipient listener is not TCP")
				}
				if err := tcpListener.SetDeadline(deadline); err != nil {
					t.Fatal(err)
				}
				conn, err := listener.Accept()
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close() //nolint:errcheck // The Session may already have closed this accepted socket.
				remoteEndpoint, ok := conn.RemoteAddr().(*net.TCPAddr)
				if !ok {
					t.Fatal("accepted speaker has no TCP endpoint")
				}
				connected := remoteEndpoint.AddrPort().Addr()
				if connected != speaker {
					t.Fatalf("connected speaker = %v, want owned address %v", connected, speaker)
				}
				wantHop := connected
				if selection == "explicit" {
					wantHop = selected
				}
				remoteSettings := reactor.NewPeerSettings(speaker, 65001, 65000, 0x05060708)
				remoteSettings.Connection = reactor.ConnectionPassive
				remoteSettings.Capabilities = []capability.Capability{&capability.Multiprotocol{AFI: 2, SAFI: 128}}
				remote := reactor.NewSession(remoteSettings)
				updates := make(chan []byte, 16)
				remote.SetMessageCallback(func(_ netip.Addr, kind msgtype.MessageType, body []byte,
					_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
					_ reactor.BufHandle, _ map[string]any, _ string, _ uint64) bool {
					if direction != rpc.DirectionReceived {
						return false
					}
					if kind == msgtype.TypeUPDATE {
						select {
						case updates <- bytes.Clone(body):
						case <-ctx.Done():
						}
					}
					return false
				})
				if err := remote.Start(); err != nil {
					t.Fatal(err)
				}
				if err := remote.Accept(conn); err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() { done <- remote.Run(ctx) }()
				defer func() {
					cancel()
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("recipient Session did not stop")
					}
				}()
				rfc4659ReadConfiguredRoutes(t, ctx, updates, wantHop)
			})
		}
	}
}

// rfc4659RequireOwnedHostAddresses excludes invented transport identities and
// shared connected subnets by reading the namespace's actual interfaces.
func rfc4659RequireOwnedHostAddresses(t *testing.T, addresses ...netip.Addr) {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	owned := make(map[netip.Addr]bool, len(addresses))
	for _, iface := range interfaces {
		prefixes, err := iface.Addrs()
		if err != nil {
			t.Fatal(err)
		}
		for _, address := range prefixes {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				t.Fatal(err)
			}
			for _, wanted := range addresses {
				if prefix.Addr() == wanted {
					if prefix.Bits() != 128 {
						t.Fatalf("fixture address %v must have a /128 prefix, got %v", wanted, prefix)
					}
					owned[wanted] = true
				}
				if prefix.Contains(wanted) && prefix.Bits() < 128 {
					t.Fatalf("fixture address %v lies on connected subnet %v", wanted, prefix)
				}
			}
		}
	}
	for _, address := range addresses {
		if !owned[address] {
			t.Fatalf("fixture failed to install owned /128 address %v", address)
		}
	}
}

// rfc4659ConfiguredSettings keeps grouping, next-hop selection, labels and NLRI
// on the operator's parser path rather than patching reactor settings afterward.
func rfc4659ConfiguredSettings(t *testing.T, speaker, recipient netip.Addr, port int, hop string, grouped bool) *reactor.PeerSettings {
	t.Helper()
	input := `bgp {
    router-id 1.2.3.4;
    session { asn { local 65000; } }
    peer vpn-recipient {
        connection {
            local { ip ` + speaker.String() + `; accept false; }
            remote { ip ` + recipient.String() + `; port ` + strconv.Itoa(port) + `; }
        }
        session { asn { remote 65001; } family { ipv6/mpls-vpn { prefix { maximum 1000; } } } }
        behavior { group-updates ` + strconv.FormatBool(grouped) + `; }
        update {
            attribute { origin igp; next-hop ` + hop + `; }
            nlri {
                ipv6/mpls-vpn add rd 65000:100 label 1000 2001:db8:1::/48;
                ipv6/mpls-vpn add rd 65000:200 label 2000 2001:db8:2::/48;
            }
        }
    }
}`
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.NewParser(schema).Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := bgpconfig.PeersFromConfigTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 {
		t.Fatalf("configured peers = %d, want 1", len(peers))
	}
	if peers[0].GroupUpdates != grouped {
		t.Fatalf("group-updates = %v, want %v", peers[0].GroupUpdates, grouped)
	}
	if len(peers[0].StaticRoutes) != 2 {
		t.Fatalf("configured routes = %d, want 2", len(peers[0].StaticRoutes))
	}
	return peers[0]
}

// rfc4659ReadConfiguredRoutes uses literal VPN NLRI bytes, independent of the
// production NLRI encoder, and ends only at the recipient's VPNv6 End-of-RIB.
func rfc4659ReadConfiguredRoutes(t *testing.T, ctx context.Context, updates <-chan []byte, wantHop netip.Addr) {
	t.Helper()
	want := map[string]bool{
		"88003e810000fde80000006420010db80001": false,
		"88007d010000fde8000000c820010db80002": false,
	}
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("configured VPNv6 synchronization did not complete: %v", ctx.Err())
		case body := <-updates:
			mp, endOfRIB := rfc4659ConfiguredMPReach(t, body)
			if endOfRIB {
				for nlri, seen := range want {
					if !seen {
						t.Fatalf("VPNv6 End-of-RIB preceded intended NLRI %s", nlri)
					}
				}
				return
			}
			if len(mp) < 29 {
				t.Fatalf("short MP_REACH: %x", mp)
			}
			if !bytes.Equal(mp[:4], []byte{0, 2, 128, 24}) {
				t.Fatalf("AFI/SAFI/next-hop length = %x, want 00028018", mp[:4])
			}
			for index, value := range mp[4:12] {
				if value != 0 {
					t.Fatalf("next-hop RD byte %d = %02x, want zero", index, value)
				}
			}
			if !bytes.Equal(mp[12:28], wantHop.AsSlice()) {
				t.Fatalf("advertised global = %x, want independently owned %v", mp[12:28], wantHop)
			}
			if mp[28] != 0 {
				t.Fatalf("MP_REACH reserved byte = %02x, want zero", mp[28])
			}
			key := hex.EncodeToString(mp[29:])
			seen, exists := want[key]
			if !exists {
				t.Fatalf("unexpected VPN NLRI/labels/RD: %s", key)
			}
			if seen {
				t.Fatalf("duplicate configured VPN NLRI: %s", key)
			}
			want[key] = true
		}
	}
}

// rfc4659ConfiguredMPReach checks framing before exposing the single MP_REACH
// value or recognizing the sole empty VPNv6 MP_UNREACH as End-of-RIB. Attribute
// lengths may use either legal header form; neither changes the family marker.
// RFC 4724 Section 2: "For any other address family, it is an UPDATE message
// that contains only the MP_UNREACH_NLRI attribute [BGP-MP] with no withdrawn
// routes for that <AFI, SAFI>."
func rfc4659ConfiguredMPReach(t *testing.T, body []byte) ([]byte, bool) {
	t.Helper()
	if len(body) < 4 {
		t.Fatalf("short UPDATE: %x", body)
	}
	if binary.BigEndian.Uint16(body[:2]) != 0 {
		t.Fatalf("unexpected withdrawn routes: %x", body)
	}
	length := int(binary.BigEndian.Uint16(body[2:4]))
	if length != len(body)-4 {
		t.Fatalf("unexpected legacy NLRI or invalid attribute length: %x", body)
	}
	attrs := body[4:]
	var reach []byte
	endOfRIB := false
	attributeCount := 0
	for len(attrs) != 0 {
		if len(attrs) < 3 {
			t.Fatalf("short attribute header: %x", attrs)
		}
		header, size := 3, int(attrs[2])
		if attrs[0]&0x10 != 0 {
			if len(attrs) < 4 {
				t.Fatalf("short extended attribute header: %x", attrs)
			}
			header, size = 4, int(binary.BigEndian.Uint16(attrs[2:4]))
		}
		if size > len(attrs)-header {
			t.Fatalf("truncated attribute: %x", attrs)
		}
		if attrs[1] == 14 {
			// The announcement carries exactly one MP_REACH.
			if reach != nil {
				t.Fatal("duplicate MP_REACH")
			}
			reach = attrs[header : header+size]
		}
		if attrs[1] == 15 {
			if !bytes.Equal(attrs[header:header+size], []byte{0, 2, 128}) {
				t.Fatalf("unexpected MP_UNREACH: %x", attrs[header:header+size])
			}
			endOfRIB = true
		}
		attributeCount++
		attrs = attrs[header+size:]
	}
	if endOfRIB {
		if attributeCount != 1 {
			t.Fatalf("VPNv6 End-of-RIB must contain only MP_UNREACH: %x", body)
		}
		return nil, true
	}
	if reach == nil {
		t.Fatalf("UPDATE has no MP_REACH: %x", body)
	}
	return reach, false
}

// rfc4659TransportFixture follows the network package's newTCPEgress namespace
// lifecycle. The caller MUST stop and join both Sessions before this fixture's
// registered cleanup closes their namespace handles. Only socket creation needs setns.
func rfc4659TransportFixture(t *testing.T, speaker, recipient, selected netip.Addr) (netns.NsHandle, net.Listener) {
	t.Helper()
	runtime.LockOSThread()
	original, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	defer func() {
		if err := netns.Set(original); err != nil {
			t.Fatalf("restore original namespace: %v", err)
		}
		if err := original.Close(); err != nil {
			t.Error(err)
		}
		runtime.UnlockOSThread()
	}()
	senderNS, err := netns.New()
	if err != nil {
		t.Skipf("network namespaces require CAP_SYS_ADMIN: %v", err)
	}
	t.Cleanup(func() {
		if err := senderNS.Close(); err != nil {
			t.Error(err)
		}
	})
	receiverNS, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := receiverNS.Close(); err != nil {
			t.Error(err)
		}
	})
	pair := &netlink.Veth{
		LinkAttrs:     netlink.LinkAttrs{Name: "vpn-receiver"},
		PeerName:      "vpn-sender",
		PeerNamespace: netlink.NsFd(int(senderNS)),
	}
	if err := netlink.LinkAdd(pair); err != nil {
		t.Fatal(err)
	}
	rfc4659NamespaceAddress(t, "vpn-receiver", recipient)
	rfc4659NamespaceRoute(t, "vpn-receiver", speaker)
	rfc4659NamespaceRoute(t, "vpn-receiver", selected)
	rfc4659RequireOwnedHostAddresses(t, recipient)
	rfc4659RefuseLocalAddress(t, speaker, selected)
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp6", net.JoinHostPort(recipient.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := netns.Set(senderNS); err != nil {
		t.Fatal(err)
	}
	rfc4659NamespaceAddress(t, "vpn-sender", speaker)
	rfc4659NamespaceAddress(t, "lo", selected)
	rfc4659NamespaceRoute(t, "vpn-sender", recipient)
	rfc4659RequireOwnedHostAddresses(t, speaker, selected)
	rfc4659RefuseLocalAddress(t, recipient)
	return senderNS, listener
}

// rfc4659NamespaceAddress installs an immediately usable host address. NODAD
// avoids a wall-clock DAD wait on these fresh, independently owned namespaces.
func rfc4659NamespaceAddress(t *testing.T, name string, address netip.Addr) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	value := &netlink.Addr{
		IPNet: &net.IPNet{IP: address.AsSlice(), Mask: net.CIDRMask(128, 128)},
		Flags: unix.IFA_F_NODAD,
	}
	if err := netlink.AddrAdd(link, value); err != nil {
		t.Fatal(err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
}

// rfc4659NamespaceRoute connects distinct /128 endpoints without manufacturing
// a shared subnet that would require the absent 48-octet next-hop branch.
func rfc4659NamespaceRoute(t *testing.T, name string, target netip.Addr) {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: target.AsSlice(), Mask: net.CIDRMask(128, 128)},
		Scope:     netlink.SCOPE_LINK,
	}); err != nil {
		t.Fatal(err)
	}
}

// rfc4659RefuseLocalAddress proves that no local route can shortcut the veth.
func rfc4659RefuseLocalAddress(t *testing.T, addresses ...netip.Addr) {
	t.Helper()
	local, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range local {
		prefix, err := netip.ParsePrefix(value.String())
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range addresses {
			if prefix.Addr() == forbidden {
				t.Fatalf("remote endpoint %v is owned by the wrong namespace", forbidden)
			}
		}
	}
}

// rfc4659NamespaceDialer creates real TCP sockets in the speaker namespace.
// Its fields are immutable while the Peer runs; the caller MUST join the Peer
// before closing namespace. The zero value is not a usable fixture dialer.
type rfc4659NamespaceDialer struct {
	namespace netns.NsHandle
	local     netip.Addr
}

func (d *rfc4659NamespaceDialer) DialContext(ctx context.Context, transport, address string) (conn net.Conn, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		return nil, err
	}
	defer func() {
		restoreErr := netns.Set(original)
		closeErr := original.Close()
		if restoreErr != nil {
			err = fmt.Errorf("restore dialer namespace: %w", restoreErr)
		} else if closeErr != nil {
			err = fmt.Errorf("close dialer namespace handle: %w", closeErr)
		}
		if err != nil && conn != nil {
			conn.Close() //nolint:errcheck // The original failure is authoritative.
			conn = nil
		}
	}()
	if err := netns.Set(d.namespace); err != nil {
		return nil, err
	}
	dialer := network.RealDialer{LocalAddr: &net.TCPAddr{IP: d.local.AsSlice()}}
	return dialer.DialContext(ctx, transport, address)
}
