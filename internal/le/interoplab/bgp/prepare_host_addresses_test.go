package bgp

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: a declared lab host address is rendered on the run's selected
// IPv4 subnet, at the head of its peer's Prestart, ahead of what a registered
// preparer already put there; the other peers are untouched.
// PREVENTS: the address put on the 172.30.0.0/24 the scenario files spell when
// a concurrent run moved the lab, and a declaration that names a peer the
// scenario does not start, or a run with no IPv4 network, passing silently.
func TestAddHostAddresses(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.4.0/24")}
	peers := []interoplab.PeerConfig{{Name: "ze", Prestart: "existing-setup\n"}, {Name: peerFRR}}
	peers, err := addHostAddresses(peers, "scenario", []hostAddress{{peer: "ze", host: 250}}, network)
	if err != nil {
		t.Fatalf("addHostAddresses: %v", err)
	}
	if want := "ip address add 172.31.4.250/32 dev eth0\nexisting-setup\n"; peers[0].Prestart != want {
		t.Errorf("ze Prestart = %q, want %q", peers[0].Prestart, want)
	}
	if peers[1].Prestart != "" {
		t.Errorf("FRR Prestart = %q, want none", peers[1].Prestart)
	}

	if _, err := addHostAddresses(peers, "scenario", []hostAddress{{peer: "absent", host: 250}}, network); err == nil || !strings.Contains(err.Error(), "does not start") {
		t.Errorf("an address for a peer the scenario does not start was not refused: %v", err)
	}
	if _, err := addHostAddresses(peers, "scenario", []hostAddress{{peer: "ze", host: 250}}, interoplab.Network{}); err == nil || !strings.Contains(err.Error(), "no IPv4 network") {
		t.Errorf("a run with no IPv4 network was not refused: %v", err)
	}
	unchanged, err := addHostAddresses([]interoplab.PeerConfig{{Name: "ze"}}, "scenario", nil, interoplab.Network{})
	if err != nil || unchanged[0].Prestart != "" {
		t.Errorf("a scenario with no declaration changed its peers: %v %q", err, unchanged[0].Prestart)
	}
}

// VALIDATES: ldp-backoff-clear-frr declares ze's LDP transport address, host
// ldpTransportHost, so it is on eth0 before ze dials.
// PREVENTS: the declaration dropped and the startup bind race back.
func TestLDPBackoffClearDeclaresTheTransportAddress(t *testing.T) {
	addresses := scenarioHostAddresses[ldpBackoffClearScenario]
	if len(addresses) != 1 || addresses[0] != (hostAddress{peer: "ze", host: ldpTransportHost}) {
		t.Fatalf("%s declares %v, want ze's transport address host %d", ldpBackoffClearScenario, addresses, ldpTransportHost)
	}
}
