// Design: docs/architecture/testing/interop.md -- the BMP collector a scenario starts.
// Related: ze_ready.go -- readZeConfig, zeSendsBMP.
// Related: prepare.go -- scenarioPeers.
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

// zeBMPSenderFixture is a ze.conf whose BGP block runs the BMP sender toward
// the lab collector, as bmp-frr's does.
const zeBMPSenderFixture = "bgp {\n\tbmp {\n\t\tsender {\n\t\t\tcollector local {\n" +
	"\t\t\t\taddress 172.30.44.6;\n\t\t\t\tport 11019;\n\t\t\t}\n\t\t}\n\t}\n}\n"

// VALIDATES: a scenario's Ze gets the lab BMP collector exactly when its
// parsed ze.conf runs the BMP sender (`bgp bmp sender`): written with the
// usual spacing or with none, and not when `bmp {` appears only in a comment
// or when Ze only receives BMP (`environment bmp`).
// Method: prepare each fixture's peers through scenarioPeers and look for the
// collector peer.
// PREVENTS: a substring match on the raw file deciding the collector, which
// started one for a commented-out block and missed `bmp{`.
func TestScenarioBMPCollectorFollowsParsedSender(t *testing.T) {
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.30.44.0/24")}
	for _, scenario := range []struct {
		name      string
		zeConfig  string
		collector bool
	}{
		{"sender", zeBMPSenderFixture, true},
		{"sender-no-spacing", "bgp{bmp{sender{collector local{address 172.30.44.6;port 11019;}}}}\n", true},
		{"comment-only", "# bmp {\n#   sender { collector local { address 172.30.44.6; } }\n# }\nbgp {\n" +
			zeReadyFixturePeer("frr", "ip 172.30.44.3;", "ip 172.30.44.2;") + "}\n", false},
		{"receiver-only", "environment {\n\tbmp {\n\t\tenabled true;\n\t\tserver main {\n\t\t\tip 0.0.0.0;\n" +
			"\t\t\tport 11019;\n\t\t}\n\t}\n}\n", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := t.TempDir()
			writeFixture(t, filepath.Join(source, zeConfigFile), scenario.zeConfig)
			peers, err := scenarioPeers(t.TempDir(), source, "fixture", network, scenarioRendering{})
			if err != nil {
				t.Fatal(err)
			}
			started := slices.ContainsFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == peerBMP })
			if started != scenario.collector {
				t.Fatalf("BMP collector started = %v, want %v", started, scenario.collector)
			}
		})
	}
}

// VALIDATES: a ze.conf the lab cannot read, or one the schema refuses, fails
// the scenario's preparation with an error naming ze.conf.
// Method: prepare a scenario whose ze.conf does not parse, and one whose
// ze.conf the user cannot read, through scenarioPeers.
// PREVENTS: an unreadable or invalid ze.conf reading as "no BMP" and the lab
// starting without the collector the scenario needs.
func TestScenarioPeersRefusesUnusableZeConfig(t *testing.T) {
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.30.44.0/24")}
	t.Run("invalid", func(t *testing.T) {
		source := t.TempDir()
		writeFixture(t, filepath.Join(source, zeConfigFile), "bgp {\n\tbmp {\n\t\tsender {\n")
		_, err := scenarioPeers(t.TempDir(), source, "fixture", network, scenarioRendering{})
		if err == nil || !strings.Contains(err.Error(), zeConfigFile) {
			t.Fatalf("scenarioPeers error = %v, want a parse error naming %s", err, zeConfigFile)
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a mode 000 file, so the read cannot fail")
		}
		source := t.TempDir()
		path := filepath.Join(source, zeConfigFile)
		writeFixture(t, path, zeBMPSenderFixture)
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		_, err := scenarioPeers(t.TempDir(), source, "fixture", network, scenarioRendering{})
		if err == nil || !strings.Contains(err.Error(), zeConfigFile) {
			t.Fatalf("scenarioPeers error = %v, want a read error naming %s", err, zeConfigFile)
		}
	})
}
