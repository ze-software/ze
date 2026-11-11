// Design: docs/architecture/testing/interop.md -- Ze's readiness probe.
// Related: ze_ready.go -- zePassiveListenPorts, zeReadyProbe.
// Related: extended_relay.go -- prepareExtendedRelayPeers.
package bgp

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// zeReadyFixturePeer is a BGP peer block for the readiness fixtures, with the
// connection leaves the case under test sets.
func zeReadyFixturePeer(name, remote, local string) string {
	return "\tpeer " + name + " {\n\t\tconnection {\n\t\t\tremote { " + remote + " }\n\t\t\tlocal  { " + local +
		" }\n\t\t}\n\t\tsession { asn { local 65001; remote 65002; } }\n\t}\n"
}

// VALIDATES: the Ze under test is ready only once ss lists a listener on each
// port Ze answers a peer it never dials (`connect false`, a dynamic group, a
// group's `connect false` inherited by its peer at its own local port), and
// a Ze that dials every peer keeps the probe that only runs a command, even
// when a comment names `connect false`.
// Method: prepare each fixture's peers through scenarioPeers and read Ze's
// ReadyProbe.
// PREVENTS: the lab starting BIRD or FRR while a passive Ze is not yet
// listening, so the daemon's first dial is refused and its 120 s
// connect-retry outlasts the session budget (bgp-wire-edit-api-origin-bird,
// bgp-redist-late-join-dynamic-frr, bgp-bfd-dynamic-enable-live-frr).
func TestZeReadinessWaitsForPassiveListener(t *testing.T) {
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.30.44.0/24")}
	for _, scenario := range []struct {
		name     string
		zeConfig string
		contains []string
	}{
		{"passive-peer", "bgp {\n" + zeReadyFixturePeer("bird", "ip 172.30.44.4; connect false;", "ip 172.30.44.2;") + "}\n", []string{":179 "}},
		{"dynamic-group", "bgp {\n\tgroup dynpeers {\n\t\tconnection {\n\t\t\tremote { ip dynamic; connect false; range 172.30.44.0/24; }\n" +
			"\t\t\tlocal  { ip 172.30.44.2; accept true; }\n\t\t}\n\t\tsession { asn { local 65001; } }\n\t}\n}\n", []string{":179 "}},
		{"group-inherits", "bgp {\n\tgroup observers {\n\t\tconnection {\n\t\t\tremote { connect false; }\n\t\t\tlocal  { port 1790; }\n\t\t}\n" +
			zeReadyFixturePeer("observer", "ip 172.30.44.10;", "ip 172.30.44.2;") + "\t}\n}\n", []string{":1790 "}},
		{"all-active", "bgp {\n" + zeReadyFixturePeer("frr", "ip 172.30.44.3;", "ip 172.30.44.2;") + "}\n", nil},
		{"comment-only", "# connect false is not configured here.\nbgp {\n" + zeReadyFixturePeer("frr", "ip 172.30.44.3;", "ip 172.30.44.2;") + "}\n", nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := t.TempDir()
			writeFixture(t, filepath.Join(source, zeConfigFile), scenario.zeConfig)
			peers, err := scenarioPeers(t.TempDir(), source, "fixture", network)
			if err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == "ze" })
			if index < 0 {
				t.Fatal("no Ze peer prepared")
			}
			requireZeReadiness(t, peers[index].Ready, scenario.contains)
		})
	}
}

// requireZeReadiness checks a Ze probe: the ss listener barrier requiring
// contains, or, for a nil contains, the probe that only runs a command.
func requireZeReadiness(t *testing.T, ready *interoplab.ReadyProbe, contains []string) {
	t.Helper()
	if ready == nil || ready.Timeout <= 0 || ready.Interval <= 0 {
		t.Fatalf("Ze readiness is missing or unbounded: %+v", ready)
	}
	if contains == nil {
		if !slices.Equal(ready.Command, []string{"true"}) || len(ready.Contains) != 0 {
			t.Fatalf("a Ze that dials every peer got the probe %+v, want the bare start", ready)
		}
		return
	}
	if !slices.Equal(ready.Command, []string{"ss", "-ltn"}) {
		t.Fatalf("passive Ze readiness runs %q, want ss -ltn: nothing checks the listener", ready.Command)
	}
	if !slices.Equal(ready.Contains, contains) {
		t.Fatalf("passive Ze readiness requires %q, want %q", ready.Contains, contains)
	}
}

// VALIDATES: a scenario's second Ze (ze-peer.conf) gets the same listener
// barrier when it only listens, while the Ze under test, which dials, keeps
// the bare start. Method: prepare ze.conf active and ze-peer.conf passive.
// PREVENTS: the barrier reading only ze.conf, so a passive second Ze races
// the daemon that dials it.
func TestZePeerReadinessReadsItsOwnConfig(t *testing.T) {
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.30.44.0/24")}
	source := t.TempDir()
	writeFixture(t, filepath.Join(source, zeConfigFile), "bgp {\n"+zeReadyFixturePeer("peer", "ip 172.30.44.3;", "ip 172.30.44.2;")+"}\n")
	writeFixture(t, filepath.Join(source, zePeerConfigFile), "bgp {\n"+zeReadyFixturePeer("ze", "ip 172.30.44.2; connect false;", "ip 172.30.44.3;")+"}\n")
	peers, err := scenarioPeers(t.TempDir(), source, "fixture", network)
	if err != nil {
		t.Fatal(err)
	}
	for name, contains := range map[string][]string{"ze": nil, peerZePeer: {":179 "}} {
		index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == name })
		if index < 0 {
			t.Fatalf("no %s peer prepared", name)
		}
		requireZeReadiness(t, peers[index].Ready, contains)
	}
}

// VALIDATES: an extended relay's configuration barrier keeps Ze's listener
// barrier: one probe runs ss and then the peer list on one stdout, and
// requires the relay neighbor and the listener port.
// Method: plan a relay to FRR with Ze carrying zeReadyProbe's listener probe.
// PREVENTS: prepareExtendedRelayPeers replacing the listener barrier with a
// peer list that names the relay before Ze listens.
func TestExtendedRelayKeepsZeListenerBarrier(t *testing.T) {
	network := interoplab.Network{IPv4: netip.MustParsePrefix("172.31.71.0/24")}
	peers := []interoplab.PeerConfig{
		{Name: "ze", Host: zeHostNumber, Ready: zeReadyProbe([]string{zeDefaultListenPort})},
		{Name: peerFRR, Host: frrHostNumber},
	}
	relays := []extendedRelayPeer{{peer: interoplab.PeerConfig{Name: peerSpeaker, Host: 10}, destination: peerFRR}}
	planned, err := prepareExtendedRelayPeers(peers, relays, network)
	if err != nil {
		t.Fatal(err)
	}
	ready := planned[0].Ready
	peerList := zeCommand("show bgp peer list")
	want := append([]string{"sh", "-c", `ss -ltn && exec "$@"`, "sh"}, peerList...)
	if !slices.Equal(ready.Command, want) {
		t.Fatalf("Ze relay readiness runs %q, want %q", ready.Command, want)
	}
	if !slices.Equal(ready.Contains, []string{`"172.31.71.10"`, ":179 "}) {
		t.Fatalf("Ze relay readiness requires %q, want the relay neighbor and the listener", ready.Contains)
	}
	if !slices.Equal(ready.Environment, queryEnvironment("ze", peerList)) {
		t.Fatalf("Ze relay readiness environment %+v is not the peer list's", ready.Environment)
	}
	if ready.Timeout != 30*time.Second || ready.Interval != time.Second {
		t.Fatalf("Ze relay readiness bounds changed: %+v", ready)
	}
}

// VALIDATES: every committed scenario's Ze config, rendered as a run renders
// it, parses for the readiness probe, and the three scenarios whose
// third-party daemon dials a passive Ze require the 179 listener.
// Method: render each scenario under test/interop/scenarios and read its
// ze.conf and ze-peer.conf through readZeConfig and zePassiveListenPorts.
// PREVENTS: a scenario the readiness parse refuses, which no lab could start.
func TestEveryScenarioZeConfigFeedsReadiness(t *testing.T) {
	network := interoplab.Network{
		Name: "lab",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	root := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	exposed := map[string]bool{
		"bgp-wire-edit-api-origin-bird":    false,
		"bgp-redist-late-join-dynamic-frr": false,
		"bgp-bfd-dynamic-enable-live-frr":  false,
	}
	for _, entry := range entries {
		if !entry.IsDir() || !regularFile(filepath.Join(root, entry.Name(), zeConfigFile)) {
			continue
		}
		target := t.TempDir()
		if _, renderErr := renderScenario(filepath.Join(root, entry.Name()), target, network); renderErr != nil {
			t.Errorf("%s: render: %v", entry.Name(), renderErr)
			continue
		}
		for _, file := range []string{zeConfigFile, zePeerConfigFile} {
			path := filepath.Join(target, file)
			if !regularFile(path) {
				continue
			}
			tree, readErr := readZeConfig(path)
			if readErr != nil {
				t.Errorf("%s: %v", entry.Name(), readErr)
				continue
			}
			ports, portsErr := zePassiveListenPorts(tree, file)
			if portsErr != nil {
				t.Errorf("%s: %v", entry.Name(), portsErr)
				continue
			}
			if _, ok := exposed[entry.Name()]; ok && file == zeConfigFile {
				exposed[entry.Name()] = slices.Equal(ports, []string{zeDefaultListenPort})
			}
		}
	}
	for name, listens := range exposed {
		if !listens {
			t.Errorf("%s: Ze's readiness does not wait for the 179 listener its dialing daemon needs", name)
		}
	}
}
