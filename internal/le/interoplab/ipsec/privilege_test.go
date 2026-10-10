// Design: docs/architecture/testing/interop.md -- the IPsec lab's peers
// Related: ipsec.go -- prepareScenario, the peers these tests read
// Related: nat.go -- natSetupScript and natSysctls
//
// No IPsec lab peer runs privileged (spec-lab-containers-least-privilege,
// AC-1): each is granted what its daemon does. The tests prepare every real
// scenario and read the peers' docker run inputs; no container starts.

package ipsec

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// preparedIPsecScenarios prepares every scenario under test/interop-ipsec.
func preparedIPsecScenarios(t *testing.T) map[string]interoplab.PreparedScenario {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	environment := interoplab.Environment{Image: defaultFRRImage, SessionTimeout: 90 * time.Second, Suffix: "privilege-test"}
	sources, err := interoplab.Discover(filepath.Join(root, "test", "interop-ipsec", "scenarios"), "", checkerAdapters())
	if err != nil {
		t.Fatal(err)
	}
	network := interoplab.Network{Name: "ze-ipsec-privilege-test", IPv4: networkPrefix}
	prepared := make(map[string]interoplab.PreparedScenario, len(sources))
	for _, source := range sources {
		plan := scenarioPlan(root, environment, source, &scenarioState{})
		scenario, err := plan.Prepare(t.Context(), interoplab.PrepareContext{Source: source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", source.Name, err)
		}
		if scenario.Cleanup != nil {
			t.Cleanup(func() {
				if err := scenario.Cleanup(); err != nil {
					t.Errorf("cleanup %s: %v", source.Name, err)
				}
			})
		}
		prepared[source.Name] = scenario
	}
	return prepared
}

// argumentPairs answers every value that follows flag in arguments.
func argumentPairs(arguments []string, flag string) []string {
	var values []string
	for i := 0; i+1 < len(arguments); i++ {
		if arguments[i] == flag {
			values = append(values, arguments[i+1])
		}
	}
	return values
}

// VALIDATES: AC-1. No peer is privileged; the NAT box, strongSwan and ze hold
// NET_ADMIN; the NAT box sets its sysctls through --sysctl and its script writes
// none; strongSwan, where the scenario's checker cuts its reassembly marks at
// run time (peerSysctlWriters) and only there, gets
// /proc/sys writable under the generic net sysctl profile, which grants both
// marks; ze gets neither.
// PREVENTS: a lab peer silently running --privileged again, or a grant drifting
// from the writes the lab makes.
func TestIPsecLabPeersRunWithoutPrivilege(t *testing.T) {
	for name, scenario := range preparedIPsecScenarios(t) {
		for _, peer := range scenario.Peers {
			if slices.Contains(peer.Arguments, "--privileged") {
				t.Errorf("%s: peer %s runs --privileged", name, peer.Name)
			}
			if peer.Name == frrPeer {
				continue
			}
			if !slices.Contains(peer.Capabilities, "NET_ADMIN") {
				t.Errorf("%s: peer %s lacks NET_ADMIN: %v", name, peer.Name, peer.Capabilities)
			}
			writable := slices.Contains(peer.Arguments, "systempaths=unconfined")
			switch peer.Name {
			case swanPeer:
				if !peerSysctlWriters[name] {
					if writable || peer.AppArmorProfile != "" {
						t.Errorf("%s: strongSwan writes no sysctl here, yet gets %v %q", name, peer.Arguments, peer.AppArmorProfile)
					}
					continue
				}
				if !writable || peer.AppArmorProfile != interoplab.NetSysctlAppArmorProfileName {
					t.Errorf("%s: strongSwan arguments %v profile %q, want /proc/sys writable under %s",
						name, peer.Arguments, peer.AppArmorProfile, interoplab.NetSysctlAppArmorProfileName)
				}
			case natPeer:
				sysctls := argumentPairs(peer.Arguments, "--sysctl")
				for _, sysctl := range natSysctls {
					if !slices.Contains(sysctls, sysctl) {
						t.Errorf("%s: NAT box --sysctl %v lacks %s", name, sysctls, sysctl)
					}
				}
				if strings.Contains(peer.Command[len(peer.Command)-1], "sysctl") {
					t.Errorf("%s: the NAT script still writes a sysctl on a read-only /proc/sys", name)
				}
				if writable || peer.AppArmorProfile != "" {
					t.Errorf("%s: the NAT box writes no sysctl at run time, yet gets %v %q", name, peer.Arguments, peer.AppArmorProfile)
				}
			default:
				if writable || peer.AppArmorProfile != "" {
					t.Errorf("%s: peer %s writes no sysctl, yet gets %v %q", name, peer.Name, peer.Arguments, peer.AppArmorProfile)
				}
			}
		}
	}
	for _, mark := range peerReassemblyMarks {
		if !interoplab.NetSysctlGranted(mark) {
			t.Errorf("strongSwan's profile does not grant %s, which dropFragmentsAtPeer writes", mark)
		}
	}
}
