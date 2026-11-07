// Design: docs/architecture/testing/interop.md -- the freeRouter RSVP-TE suite
// Related: zenode.go -- readZeNodeNet, zeNodeScript, freeRtrScript
// Related: rsvpte.go -- labLinks, linkDevices

package rsvpte

import (
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: a Ze node's <role>-net.txt becomes the routes its container
// adds, in the order the kernel needs: the MPLS sysctls are written, and the
// routes are in place before ze starts. A node on no link builds no device.
// PREVENTS: a scenario losing the route its checker depends on.
func TestZeNodeScriptFromNetFile(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{
		"transit.conf":    "",
		"transit-net.txt": "# a comment\n\n" + "route 198.51.100.4/32 via 172.29.81.14\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := scenarioPlan("t", interoplab.ScenarioSource{Name: "s", Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Peers) != 1 {
		t.Fatalf("peers %d, want 1", len(plan.Peers))
	}
	if len(plan.Links) != 0 {
		t.Fatalf("a scenario with no labLinks entry plans links %+v", plan.Links)
	}
	command := plan.Peers[0].Command
	if len(command) != 3 || command[0] != "sh" || command[1] != "-c" {
		t.Fatalf("command %q is not sh -c <script>", command)
	}
	want := "mkdir -p /run/fr\n" +
		"tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > " + captureFile + " 2> /run/fr/tcpdump.err &\n" +
		"set -eu\n" +
		"sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1\n" +
		"ip route add 198.51.100.4/32 via 172.29.81.14 dev eth0\n" +
		"exec ze start /etc/ze/ze.conf\n"
	if command[2] != want {
		t.Fatalf("script:\n%s\nwant:\n%s", command[2], want)
	}
}

// VALIDATES: plr-backup-path-to-egress-merge-point's protected link reaches
// the Ze PLR through the core's link declaration with the device and address
// the hand-written VLAN line gave it (prot0, VLAN 100, 10.0.14.3/24), the
// freeRouter egress is its external end and holds no device, the core accepts
// the declaration against the plan's own peers, the composed start builds
// prot0 before the node script, and that script enables MPLS input on prot0
// (AC-7b).
// Method: plan the real scenario directory, validate its links, compose the
// PLR's start against the Ze image's tini entrypoint, and read the wrapper.
// PREVENTS: the move to the core renaming or renumbering the device that
// transit.conf and the checker name, a sysctl naming a device that does not
// exist yet, and a refusal of the one-sided link before any container starts.
func TestBackupPathProtectedLinkFromCore(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, filepath.FromSlash(scenarioDirectory), scenarioBackupPathToMP)
	plan, err := scenarioPlan("t", interoplab.ScenarioSource{Name: scenarioBackupPathToMP, Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	if err := interoplab.ValidateLinks(scenarioBackupPathToMP, plan.Links, plan.Peers); err != nil {
		t.Fatal(err)
	}
	transit := slices.IndexFunc(plan.Peers, func(peer interoplab.PeerConfig) bool { return peer.Name == peerTransit })
	if transit < 0 {
		t.Fatalf("the scenario plans no %s: %+v", peerTransit, plan.Peers)
	}
	peer := plan.Peers[transit]
	want := []interoplab.LinkMembership{{Link: "protected", Device: deviceProtected, IPv4: netip.MustParseAddr("10.0.14.3")}}
	if got := interoplab.LinkMemberships(plan.Links, peer.Name, peer.Host); !slices.Equal(got, want) {
		t.Fatalf("PLR memberships = %+v, want %+v", got, want)
	}
	if got := interoplab.LinkMemberships(plan.Links, peerEgress, 4); len(got) != 0 {
		t.Fatalf("the freeRouter egress holds %+v, want nothing: its jar terminates VLAN 100", got)
	}
	composed := interoplab.ComposeStart(peer, plan.Links, interoplab.ImageStart{Entrypoint: []string{"tini", "--"}})
	if len(composed.Command) < 2 {
		t.Fatalf("composed command %q carries no wrapper script", composed.Command)
	}
	wrapper := composed.Command[1]
	built := "ip link add link eth0 name prot0 type vlan id 100\n" +
		"ip address add 10.0.14.3/24 dev prot0\n" +
		"ip link set prot0 up\n"
	if !strings.HasPrefix(wrapper, built) {
		t.Fatalf("wrapper:\n%s\nwant it to open with\n%s", wrapper, built)
	}
	if !strings.HasSuffix(wrapper, `exec "$@"`) {
		t.Fatalf("wrapper:\n%s\nwant it to exec the node's start last", wrapper)
	}
	if composed.Command[len(composed.Command)-1] != peer.Command[2] {
		t.Fatalf("composed command %q does not end with the node script", composed.Command)
	}
	if !strings.Contains(peer.Command[2], " net.mpls.conf.prot0.input=1 ") {
		t.Fatalf("node script does not enable MPLS input on prot0:\n%s", peer.Command[2])
	}
}

// VALIDATES: every malformed <role>-net.txt line refuses the plan, a VLAN
// line included: a VLAN is a link of the scenario's labLinks entry now.
// PREVENTS: a typo silently running a scenario without its route, and a VLAN
// declared beside the core's links, which the core would never validate.
func TestReadZeNodeNetRefusesMalformedLines(t *testing.T) {
	for _, line := range []string{
		"bridge br0",
		"route 198.51.100.4/32",
		"route 198.51.100.4/32 through 172.29.81.14",
		"route 198.51.100.4 via 172.29.81.14",
		"route 198.51.100.5/24 via 172.29.81.14",
		"route 198.51.100.4/32 via gateway",
		"vlan prot0 id 100 address 10.0.14.3/24",
	} {
		path := filepath.Join(t.TempDir(), "transit-net.txt")
		if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readZeNodeNet(path); err == nil {
			t.Errorf("%q: accepted", line)
		}
	}
}

// VALIDATES: the freeRouter command binds the jar's UDP socket before rawInt
// starts and execs rawInt last, as the container's long-lived process.
// PREVENTS: rawInt exiting on ICMP port-unreachable because it started first.
func TestFreeRtrPeerRunsTheStartScript(t *testing.T) {
	peer := freeRtrPeer("egress", 4, "t", t.TempDir())
	if len(peer.Command) != 3 || peer.Command[2] != freeRtrScript {
		t.Fatalf("command %q does not run freeRtrScript", peer.Command)
	}
	wait := strings.Index(freeRtrScript, "0100007F:6591")
	raw := strings.Index(freeRtrScript, "exec /peer/rawInt.bin")
	if wait < 0 || raw < wait {
		t.Fatalf("rawInt does not start after the jar's socket is bound:\n%s", freeRtrScript)
	}
}

// VALIDATES: a Ze role with no routes still writes the MPLS sysctls and starts
// ze, with no VLAN and no route (links now come from the core start wrapper).
// PREVENTS: a scenario that declares no network starting ze without MPLS input.
func TestZeNodeScriptWithoutNetFile(t *testing.T) {
	script := zeNodeScript(zeNodeNet{}, nil)
	for _, line := range []string{
		"sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.ipv4.ip_forward=1\n",
		"exec ze start /etc/ze/ze.conf\n",
	} {
		if !strings.Contains(script, line) {
			t.Errorf("script lacks %q:\n%s", line, script)
		}
	}
	for _, absent := range []string{"ip link add", "ip route add"} {
		if strings.Contains(script, absent) {
			t.Errorf("script runs %q with no net file:\n%s", absent, script)
		}
	}
}
