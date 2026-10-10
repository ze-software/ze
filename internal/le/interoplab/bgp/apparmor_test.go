// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: register_apparmor.go -- the VRRP lab profile these tests read
//
// A VRRP scenario runs ze as a router that writes per-device sysctls, which
// docker-default denies. The lab runs that container under Ze's own profile,
// never unconfined (owner decision D-7).

package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: D-7. A scenario carrying keepalived.conf starts ze under the VRRP
// lab profile, and no argument lifts AppArmor confinement.
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
	if ze.AppArmorProfile != vrrpLabAppArmorProfileName {
		t.Errorf("ze runs under AppArmor profile %q, want %q", ze.AppArmorProfile, vrrpLabAppArmorProfileName)
	}
	if joined := strings.Join(ze.Arguments, " "); strings.Contains(joined, "apparmor=") {
		t.Errorf("ze's arguments set AppArmor themselves: %s", joined)
	}
	if _, registered := interoplab.LookupAppArmorProfile(vrrpLabAppArmorProfileName); !registered {
		t.Errorf("profile %q is not registered, so no host can load it", vrrpLabAppArmorProfileName)
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

// VALIDATES: D-7. The VRRP lab profile is docker-default with its /proc/sys
// write rules narrowed to the per-device conf trees VRRP writes
// (ipv4Conf and ipv6Conf, internal/plugins/vrrp/dataplane_linux.go), and keeps
// docker-default's blanket mount deny.
// PREVENTS: a profile that grants a mount or any other sysctl write.
func TestVRRPLabAppArmorProfileGrantsOnlyTheConfWrites(t *testing.T) {
	profile, registered := interoplab.LookupAppArmorProfile(vrrpLabAppArmorProfileName)
	if !registered {
		t.Fatalf("profile %q is not registered", vrrpLabAppArmorProfileName)
	}
	text := profile.Text()
	for _, want := range []string{
		"profile " + vrrpLabAppArmorProfileName + " flags=",
		"  deny mount,\n",
		"  deny @{PROC}/sys/[^n]** w,\n",
		"  deny @{PROC}/sys/net/ipv[^46]** w,\n",
		"  deny @{PROC}/sys/net/ipv[46]/conf[^/]** w,\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the profile lacks %q:\n%s", want, text)
		}
	}
	for _, refused := range []string{"mount options", "remount", "unconfined)"} {
		if strings.Contains(text, refused) {
			t.Errorf("the profile carries %q:\n%s", refused, text)
		}
	}
}
