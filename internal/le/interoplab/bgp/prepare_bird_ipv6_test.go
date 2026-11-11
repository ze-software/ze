// Design: docs/architecture/testing/interop.md -- Container Addresses
// Related: prepare.go -- scenarioPeers, renderScenario, scenarioRendering
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

// VALIDATES: BIRD starts with the IPv6 sysctls exactly when the render wrote a
// lab IPv6 address into its bird.conf, on a lab network other than the base
// 172.30.0.0/24, and without them when bird.conf names no IPv6 address on the
// same dual-stack network.
// Method: prepare a fixture scenario on 172.31.22.0/24 with its IPv6 prefix,
// once with a bird.conf naming the base IPv6 prefix and once with an
// IPv4-only bird.conf, and read the BIRD peer's arguments.
// PREVENTS: the decision re-reading the rendered bird.conf for the base
// prefix, which rendering has already replaced with the lab's own, so BIRD
// (bgp-vpn-frr) started without IPv6 on every lab subnet but the base one.
func TestBIRDIPv6SysctlsFollowTheRender(t *testing.T) {
	ipv4 := netip.MustParsePrefix("172.31.22.0/24")
	network := interoplab.Network{Name: "lab", IPv4: ipv4, IPv6: ipv6For(ipv4)}
	for _, scenario := range []struct {
		name string
		bird string
		want []string
	}{
		{"ipv6", "protocol bgp ze {\n\tlocal fd00:1e:0::4 as 65004;\n\tneighbor fd00:1e:0::2 as 65001;\n}\n", ipv6Sysctls()},
		{"ipv4-only", "protocol bgp ze {\n\tlocal 172.30.0.4 as 65004;\n\tneighbor 172.30.0.2 as 65001;\n}\n", nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := t.TempDir()
			writeFixture(t, filepath.Join(source, zeConfigFile), "bgp {\n}\n")
			writeFixture(t, filepath.Join(source, "bird.conf"), scenario.bird)
			prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
				Source:  interoplab.ScenarioSource{Name: "bird-ipv6-fixture", Directory: source},
				Network: network,
			})
			if prepared.Cleanup != nil {
				t.Cleanup(func() {
					if cleanupErr := prepared.Cleanup(); cleanupErr != nil {
						t.Error(cleanupErr)
					}
				})
			}
			if err != nil {
				t.Fatal(err)
			}
			index := slices.IndexFunc(prepared.Peers, func(peer interoplab.PeerConfig) bool { return peer.Name == peerBIRD })
			if index < 0 {
				t.Fatal("no BIRD peer was prepared for a scenario carrying bird.conf")
			}
			bird := prepared.Peers[index]
			if !slices.Equal(bird.Arguments, scenario.want) {
				t.Fatalf("BIRD arguments = %q, want %q", bird.Arguments, scenario.want)
			}
			if scenario.want == nil {
				return
			}
			rendered, readErr := os.ReadFile(bird.Mounts[0].Source)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if strings.Contains(string(rendered), baseIPv6Prefix) {
				t.Fatalf("the rendered bird.conf still names the base IPv6 prefix, so this lab network does not exercise the defect:\n%s", rendered)
			}
		})
	}
}
