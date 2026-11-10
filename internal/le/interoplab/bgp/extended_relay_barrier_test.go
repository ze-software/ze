// Design: docs/architecture/testing/interop.md -- native configuration barriers before a relay dials.
// Related: extended_relay.go -- prepareExtendedRelayPeers, nativeRelayReady.
package bgp

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestExtendedRelayGoBGPBarrier proves a relay dialing GoBGP waits until GoBGP
// has configured the relay as its neighbor. GoBGP opens its listener before it
// adds its neighbors and closes a connection from an unconfigured address, and
// the relay never reconnects, so a relay started on a listening port alone
// loses its one session (bgp-local-pref-strip-gobgp). Method: plan Ze, FRR,
// GoBGP and one GoBGP-facing relay, then read the plan the lab runs in order.
//
// MUTATION: drop the peerGoBGP case in nativeRelayReady (extended_relay.go)
// and a GoBGP destination has no barrier: preparation fails here. Drop the
// peerGoBGP case in scenarioPeers (prepare.go) and the relay never reaches
// prepareExtendedRelayPeers: TestExtendedRelayGoBGPPreparation goes red.
func TestExtendedRelayGoBGPBarrier(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
	frrReady := &interoplab.ReadyProbe{Command: []string{cmdVtysh, "-c", frrShowVersion}, Timeout: time.Second, Interval: time.Second}
	peers := []interoplab.PeerConfig{
		{Name: "ze", Host: zeHostNumber},
		{Name: peerFRR, Host: frrHostNumber, Ready: frrReady},
		{Name: peerGoBGP, Host: gobgpHostNumber},
	}
	relays := []extendedRelayPeer{{peer: interoplab.PeerConfig{Name: peerSpeaker, Host: 10}, destination: peerGoBGP}}
	planned, err := prepareExtendedRelayPeers(peers, relays, network)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(planned))
	for index := range planned {
		names = append(names, planned[index].Name)
	}
	if !slices.Equal(names, []string{"ze", peerFRR, peerGoBGP, peerSpeaker}) {
		t.Fatalf("relay does not start after GoBGP: %v", names)
	}
	barrier := planned[2].Ready
	if barrier == nil {
		t.Fatal("GoBGP has no readiness barrier, so the relay can dial before AddPeer")
	}
	if !slices.Equal(barrier.Command, []string{cmdGoBGP, gobgpNeighbor, "172.31.71.10"}) {
		t.Fatalf("GoBGP barrier command %v does not ask for the relay neighbor", barrier.Command)
	}
	if !slices.Equal(barrier.Contains, []string{"BGP neighbor is 172.31.71.10"}) {
		t.Fatalf("GoBGP barrier requires %v, want the relay neighbor's header", barrier.Contains)
	}
	if barrier.Timeout <= 0 || barrier.Interval <= 0 {
		t.Fatalf("GoBGP barrier is unbounded: %+v", barrier)
	}
	if planned[1].Ready != frrReady {
		t.Fatal("a GoBGP-facing relay replaced FRR's readiness probe")
	}
	if !slices.Equal(planned[0].Ready.Contains, []string{`"172.31.71.10"`}) {
		t.Fatalf("Ze readiness does not list the relay peer: %+v", planned[0].Ready)
	}
}

// TestExtendedRelayUnknownDestinationRefused proves a relay whose destination
// has no known configuration barrier is refused, never started on a listening
// port alone. Method: plan a relay to BIRD, which nativeRelayReady does not
// cover.
//
// MUTATION: return a nil probe and nil error after the switch in
// nativeRelayReady, and the relay is planned with no barrier: this goes red.
func TestExtendedRelayUnknownDestinationRefused(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
	peers := []interoplab.PeerConfig{{Name: "ze", Host: zeHostNumber}, {Name: peerBIRD, Host: 4}}
	relays := []extendedRelayPeer{{peer: interoplab.PeerConfig{Name: peerSpeaker, Host: 10}, destination: peerBIRD}}
	if _, err := prepareExtendedRelayPeers(peers, relays, network); err == nil {
		t.Fatal("relay to a daemon with no configuration barrier was planned")
	}
}
