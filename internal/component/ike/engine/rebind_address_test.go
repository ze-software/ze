// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT traversal, the rebind
// Related: rebind.go -- rebindChildSAs, rebindRekeyReason
// Related: create_child.go -- peerTunnelAddress, takePeerTunnelAddress

package engine

import (
	"errors"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// rbAddressGone is an address no fixture binds, standing for where the peer was before a
// NAT rebinding moved it.
var rbAddressGone = net.ParseIP("192.0.2.9")

// VALIDATES: a tunnel-mode Child SA built while the IKE SA holds an authenticated peer
// endpoint takes that endpoint's address as its tunnel remote, and a transport-mode one
// keeps the configured address its selectors name.
// PREVENTS: a CREATE_CHILD_SA Child SA built after a rebind sending ESP to the
// configured address IKE no longer uses (RFC 4555 Section 3.3).
// METHOD: newChildSA over an established SA whose peer endpoint differs from the
// configured remote, in both modes; no dataplane.
func TestNewChildSATunnelRemoteIsTheAuthenticatedPeerAddress(t *testing.T) {
	ini, _, _ := establishPSK(t)
	endpoint := &net.UDPAddr{IP: net.ParseIP("198.51.100.7"), Port: 500}
	ini.peerEndpoint = endpoint
	for name, tc := range map[string]struct {
		mode uint8
		want net.IP
	}{
		"tunnel":    {mode: modeTunnel, want: endpoint.IP},
		"transport": {mode: modeTransport, want: net.ParseIP(ini.PeerCfg.RemoteAddress)},
	} {
		t.Run(name, func(t *testing.T) {
			child, err := newChildSA(ini, ini.PeerCfg.LocalAddress, ini.PeerCfg.RemoteAddress,
				&childSpec{inSPI: 1, outSPI: 2, mode: tc.mode})
			if err != nil {
				t.Fatalf("newChildSA: %v", err)
			}
			if !child.RemoteAddr.Equal(tc.want) {
				t.Fatalf("Child SA remote = %s, want %s", child.RemoteAddr, tc.want)
			}
		})
	}
}

// VALIDATES: an owed rekey after a rebind installs the replacement pair toward the IKE
// SA's peer address, not the address the retired pair was keyed on.
// PREVENTS: newRekeyedChild's copy of the retired pair's RemoteAddr sending ESP to an
// address the peer left, on every backend that rekeys instead of migrating.
// METHOD: the rebind fixture without a migrator; the installed pair is marked as keyed on
// an address the peer has left, the peer's DPD from the moved socket triggers the rekey,
// and the replacement states are read off the mock dataplane.
func TestRebindRekeyTakesTheIKESAPeerAddress(t *testing.T) {
	f := rbOwner(t, rbOptions{})
	f.child.RemoteAddr = append(net.IP(nil), rbAddressGone...)
	f.dpd(t, f.movedTr, false)
	f.tick()
	f.answerRekey(t)
	f.wantReplacementOnMovedPort(t)

	want := f.local.peerEndpoint.IP
	for i := 2; i < len(f.mock.sas); i++ {
		state := f.mock.sas[i]
		peer := state.Dst
		if state.Dir == dataplane.SADirIn {
			peer = state.Src
		}
		if !peer.Equal(want) {
			t.Fatalf("replacement state %d (dir %d) peer address = %s, want the IKE SA's %s",
				i, state.Dir, peer, want)
		}
	}
	if replacement := f.ps.getChildSA(); !replacement.RemoteAddr.Equal(want) {
		t.Fatalf("replacement Child SA remote = %s, want %s", replacement.RemoteAddr, want)
	}
}

// VALIDATES: a transport-mode Child SA whose peer changed ADDRESS closes the IKE SA for
// re-establishment: no migration, no owed rekey, no state installed.
// PREVENTS: a rekey that keeps selectors naming the address the peer left (RFC 7296
// Section 2.23.1: the transport-mode TSi "MUST match the source address of the IKE SA").
// METHOD: rebindChildSAs driven directly, since every fixture socket shares 127.0.0.1.
func TestRebindTransportAddressMoveReestablishes(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true, transport: true})
	old := &net.UDPAddr{IP: rbAddressGone, Port: int(f.oldPort)}
	out := f.ps.rebindChildSAs(f.local, f.dp, old, f.log)
	if !out.reestablish {
		t.Fatal("a transport-mode address move did not ask for re-establishment")
	}
	if f.local.State != StateDead {
		t.Fatalf("IKE SA state = %s, want dead", f.local.State)
	}
	if f.ps.rebindRekeyOwed {
		t.Fatal("a transport-mode address move owes a rekey it cannot make")
	}
	if len(f.migrator.migrations) != 0 {
		t.Fatalf("a transport-mode Child SA was migrated %d times", len(f.migrator.migrations))
	}
	if len(f.mock.sas) != 2 {
		t.Fatalf("a transport-mode address move installed states: %d, want the first pair only", len(f.mock.sas))
	}
	f.wantLog(t, "a transport-mode Child SA cannot follow it")
	f.wantLog(t, "set vpn ipsec site-to-site peer "+f.local.PeerName+" remote-address "+f.local.peerEndpoint.IP.String())
}

// VALIDATES: the retire branch reads the INSTALLED pair's mode. A pair that runs tunnel
// mode on an SA configured for transport (the peer declined transport mode) follows an
// address move by the owed rekey, and the IKE SA stays up.
// PREVENTS: closing a working tunnel-mode SA because of the configured mode.
// METHOD: rebindChildSAs driven directly over the transport fixture with its pair
// marked tunnel mode.
func TestRebindTunnelPairOnTransportConfigIsNotRetired(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true, transport: true})
	f.child.Mode = modeTunnel
	old := &net.UDPAddr{IP: rbAddressGone, Port: int(f.oldPort)}
	if out := f.ps.rebindChildSAs(f.local, f.dp, old, f.log); out.reestablish {
		t.Fatal("a tunnel-mode pair's address move closed the IKE SA")
	}
	if f.local.State == StateDead {
		t.Fatal("a tunnel-mode pair's address move killed the IKE SA")
	}
	if !f.ps.rebindRekeyOwed {
		t.Fatal("the move left no rekey owed")
	}
	f.wantLog(t, `reason="`+rebindReasonTransportConfigured+`"`)
}

// VALIDATES: a rekeyed Child SA of a MOBIKE SA keeps the retired pair's address.
// PREVENTS: a rekey installing toward a MOBIKE path that has not passed the return
// routability check (RFC 4555 Section 3.7), which setMobikePath records first.
// METHOD: takePeerTunnelAddress over a tunnel child of an SA with MOBIKE enabled and a
// peer endpoint elsewhere.
func TestTakePeerTunnelAddressLeavesAMobikeChildAlone(t *testing.T) {
	ini, _, _ := establishPSK(t)
	ini.mobike.enabled = true
	ini.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("198.51.100.7"), Port: 4500}
	child := &ChildSA{Mode: modeTunnel, RemoteAddr: append(net.IP(nil), rbAddressGone...)}
	child.takePeerTunnelAddress(ini)
	if !child.RemoteAddr.Equal(rbAddressGone) {
		t.Fatalf("MOBIKE Child SA remote = %s, want the retired pair's %s", child.RemoteAddr, rbAddressGone)
	}
	ini.mobike.enabled = false
	child.takePeerTunnelAddress(ini)
	if !child.RemoteAddr.Equal(ini.peerEndpoint.IP) {
		t.Fatalf("non-MOBIKE Child SA remote = %s, want %s", child.RemoteAddr, ini.peerEndpoint.IP)
	}
}

// VALIDATES: a tunnel-mode Child SA that does not encapsulate ESP in UDP is rekeyed after
// a move, never handed to the migrator.
// PREVENTS: a migration that encapsulates a bare-ESP pair (migrateMobikePair sets
// UDPEncap) or that the kernel refuses for its port pair, logged as a failed migration.
// METHOD: rebindChildSAs driven directly over a migrator fixture whose pair is bare ESP.
func TestRebindBareESPChildSAIsRekeyed(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true})
	f.child.UDPEncap = false
	old := &net.UDPAddr{IP: f.local.peerEndpoint.IP, Port: int(f.oldPort)}
	if out := f.ps.rebindChildSAs(f.local, f.dp, old, f.log); out.reestablish {
		t.Fatal("a bare-ESP rebind closed the IKE SA")
	}
	if len(f.migrator.migrations) != 0 {
		t.Fatalf("a bare-ESP Child SA was migrated %d times", len(f.migrator.migrations))
	}
	if !f.ps.rebindRekeyOwed {
		t.Fatal("the move left no rekey owed")
	}
	f.wantLog(t, `reason="`+rebindReasonBareESP+`"`)
}

// VALIDATES: installChildSA refuses a Child SA that needs UDP encapsulation and carries
// no port, before anything reaches the dataplane.
// PREVENTS: an ESP-in-UDP template on port 0 from a builder that skipped takeEncapPorts.
// METHOD: the rebind fixture's installed child, ports cleared, installed again on a
// fresh mock dataplane.
func TestInstallChildSARefusesEncapWithoutPorts(t *testing.T) {
	f := rbOwner(t, rbOptions{})
	for name, zero := range map[string]func(c *ChildSA){
		"local":  func(c *ChildSA) { c.udpLocalPort = 0 },
		"remote": func(c *ChildSA) { c.udpRemotePort = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			child := *f.child
			zero(&child)
			dp := &mockDP{}
			err := installChildSA(&child, testESPGroup().Proposals[0], dp, f.log)
			if !errors.Is(err, errChildEncapPort) {
				t.Fatalf("install error = %v, want errChildEncapPort", err)
			}
			if len(dp.sas) != 0 || len(dp.policies) != 0 {
				t.Fatalf("refused install left %d states and %d policies", len(dp.sas), len(dp.policies))
			}
		})
	}
}
