// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: prepare.go -- the VRRP scenario's ze container these tests read
//
// A VRRP scenario runs ze as a router that writes network sysctls at run time,
// which docker-default denies. The lab runs that container under the one
// generic lab profile, ze-lab-net-sysctl, never unconfined (owner decision
// D-7). That the profile grants every sysctl VRRP writes is proved next to the
// producer, by TestVRRPWritesOnlyLabGrantedSysctls in internal/plugins/vrrp.

package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: D-7. A scenario carrying keepalived.conf starts ze under the
// generic net sysctl profile with /proc/sys writable, and no argument lifts
// AppArmor confinement.
// PREVENTS: the lab silently running ze with apparmor=unconfined.
func TestVRRPLabZeRunsUnderItsAppArmorProfile(t *testing.T) {
	scenario := t.TempDir()
	writeFixture(t, filepath.Join(scenario, "ze.conf"), "vrrp {}\n")
	writeFixture(t, filepath.Join(scenario, "keepalived.conf"), "vrrp_instance VI_1 {}\n")
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}
	peers, err := scenarioPeers(t.TempDir(), scenario, "fixture", network)
	if err != nil {
		t.Fatal(err)
	}
	index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == "ze" })
	if index < 0 {
		t.Fatalf("no ze peer in %v", peers)
	}
	ze := peers[index]
	if ze.AppArmorProfile != interoplab.NetSysctlAppArmorProfileName {
		t.Errorf("ze runs under AppArmor profile %q, want %q", ze.AppArmorProfile, interoplab.NetSysctlAppArmorProfileName)
	}
	joined := strings.Join(ze.Arguments, " ")
	if strings.Contains(joined, "apparmor=") {
		t.Errorf("ze's arguments set AppArmor themselves: %s", joined)
	}
	if want := strings.Join(interoplab.NetSysctlWriteArguments(), " "); !strings.Contains(joined, want) {
		t.Errorf("ze's arguments %q lack %q, so /proc/sys stays read-only", joined, want)
	}
}

// VALIDATES: D-7. A scenario without keepalived.conf names no profile, so ze
// runs under the daemon's default confinement.
func TestNonVRRPLabZeNamesNoAppArmorProfile(t *testing.T) {
	scenario := t.TempDir()
	writeFixture(t, filepath.Join(scenario, "ze.conf"), "bgp {}\n")
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.31.22.0/24")}
	peers, err := scenarioPeers(t.TempDir(), scenario, "fixture", network)
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range peers {
		if peer.AppArmorProfile != "" {
			t.Errorf("peer %s runs under %q in a scenario that needs no profile", peer.Name, peer.AppArmorProfile)
		}
	}
}
