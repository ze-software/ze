// Design: docs/architecture/testing/interop.md -- Container Addresses
// Related: prepare.go -- scenarioPeers, zeDaemonPeer, renderScenario
package bgp

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: a scenario carrying ze-peer.conf starts a second Ze daemon, the
// peer named ze-peer, at FRR's host number, from the same daemon start the
// Ze under test uses: the rendered ze-peer.conf mounted at the daemon's config
// path, NET_ADMIN, IPv6 on eth0, and the CLI user the checkers log in as.
// Method: prepare a fixture scenario with ze.conf and ze-peer.conf and read the
// prepared peers and the rendered file.
// PREVENTS: a Ze<->Ze scenario (ospf-multiaf-v4-frr, where FRR ospf6d has no
// IPv4-unicast address family) whose second router is never started, so its
// checker meets "unknown peer".
func TestZePeerConfigStartsSecondZe(t *testing.T) {
	network := interoplab.Network{
		Name: "lab",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	source := t.TempDir()
	writeFixture(t, filepath.Join(source, zeConfigFile), "ospf {\n\trouter-id 172.30.0.2\n}\n")
	writeFixture(t, filepath.Join(source, zePeerConfigFile), "ospf {\n\trouter-id 172.30.0.3\n}\n")
	prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
		Source:  interoplab.ScenarioSource{Name: "ze-peer-fixture", Directory: source},
		Network: network,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := prepared.Cleanup(); cleanupErr != nil {
			t.Error(cleanupErr)
		}
	})
	index := slices.IndexFunc(prepared.Peers, func(peer interoplab.PeerConfig) bool { return peer.Name == peerZePeer })
	if index < 0 {
		t.Fatalf("no %s peer prepared: %+v", peerZePeer, prepared.Peers)
	}
	peer := prepared.Peers[index]
	zeIndex := slices.IndexFunc(prepared.Peers, func(peer interoplab.PeerConfig) bool { return peer.Name == "ze" })
	if zeIndex < 0 {
		t.Fatal("the Ze under test is no longer prepared")
	}
	ze := prepared.Peers[zeIndex]
	if peer.Host != frrHostNumber {
		t.Fatalf("%s host = %d, want %d: ze-peer.conf's router-id names FRR's slot", peerZePeer, peer.Host, frrHostNumber)
	}
	if peer.Image != ze.Image || !slices.Equal(peer.Command, ze.Command) || !slices.Equal(peer.Capabilities, ze.Capabilities) || !slices.Equal(peer.Arguments, ze.Arguments) {
		t.Fatalf("%s does not start the way the Ze under test does:\n%s: %+v\nze: %+v", peerZePeer, peerZePeer, peer, ze)
	}
	if peer.Container == ze.Container {
		t.Fatalf("%s shares the container name %s", peerZePeer, ze.Container)
	}
	if len(peer.Mounts) != 1 || peer.Mounts[0].Target != zeMountedConfig || filepath.Base(peer.Mounts[0].Source) != zePeerConfigFile {
		t.Fatalf("%s mounts = %+v, want the rendered %s at %s", peerZePeer, peer.Mounts, zePeerConfigFile, zeMountedConfig)
	}
	rendered, err := os.ReadFile(peer.Mounts[0].Source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered), "user interop") {
		t.Fatalf("rendered %s lacks the CLI user the checkers log in as:\n%s", zePeerConfigFile, rendered)
	}
	if !strings.Contains(string(rendered), "router-id 172.30.44.3") {
		t.Fatalf("rendered %s did not move the lab addresses to the selected network:\n%s", zePeerConfigFile, rendered)
	}
	if env := queryEnvironment(peerZePeer, zeCommand("show version")); len(env) != 1 || env[0].Name != "ZE_SSH_PASSWORD" {
		t.Fatalf("queryEnvironment(%s) = %+v, want the CLI password", peerZePeer, env)
	}
}

// VALIDATES: a scenario carrying both frr.conf and ze-peer.conf is refused,
// because both peers would take host 3.
// PREVENTS: two containers started on one address.
func TestZePeerConfigRefusesFRRBeside(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, filepath.Join(source, zeConfigFile), "bgp {\n}\n")
	writeFixture(t, filepath.Join(source, zePeerConfigFile), "bgp {\n}\n")
	writeFixture(t, filepath.Join(source, frrConfigFile), "router bgp 65002\n")
	_, err := scenarioPeers(t.TempDir(), source, "fixture", interoplab.Network{IPv4: netip.MustParsePrefix("172.30.44.0/24")})
	if err == nil {
		t.Fatal("a scenario with frr.conf and ze-peer.conf was prepared")
	}
	if !strings.Contains(err.Error(), zePeerConfigFile) || !strings.Contains(err.Error(), frrConfigFile) {
		t.Fatalf("refusal does not name both files: %v", err)
	}
}

// VALIDATES: the second Ze's OSPFv3 config alone selects an IPv6 lab network.
// PREVENTS: a ze-peer.conf running OSPFv3 starting without a link-local.
func TestZePeerOSPFv3SelectsIPv6(t *testing.T) {
	source := t.TempDir()
	writeFixture(t, filepath.Join(source, zeConfigFile), "bgp {\n}\n")
	writeFixture(t, filepath.Join(source, zePeerConfigFile), "ospf {\n\taddress-family {\n\t\tipv4-unicast { instance-id 64 }\n\t}\n}\n")
	got, err := needsIPv6(source)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("an OSPFv3 ze-peer.conf did not select an IPv6 lab network")
	}
}
