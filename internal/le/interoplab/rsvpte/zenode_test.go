// Design: docs/architecture/testing/interop.md -- the freeRouter RSVP-TE suite
// Related: zenode.go -- readZeNodeNet, zeNodeScript, freeRtrScript

package rsvpte

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: a Ze node's <role>-net.txt becomes the VLANs and routes its
// container creates, in the order the kernel needs: the VLAN exists before the
// sysctl naming its MPLS input, and the routes are in place before ze starts.
// PREVENTS: a scenario losing the route or VLAN its checker depends on, or a
// sysctl write against a link that does not exist yet.
func TestZeNodeScriptFromNetFile(t *testing.T) {
	directory := t.TempDir()
	for name, content := range map[string]string{
		"transit.conf": "",
		"transit-net.txt": "# a comment\n\n" +
			"vlan prot0 id 100 address 10.0.14.3/24\n" +
			"route 198.51.100.4/32 via 172.29.81.14\n",
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
	command := plan.Peers[0].Command
	if len(command) != 3 || command[0] != "sh" || command[1] != "-c" {
		t.Fatalf("command %q is not sh -c <script>", command)
	}
	want := "mkdir -p /run/fr\n" +
		"tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > " + captureFile + " 2> /run/fr/tcpdump.err &\n" +
		"set -eu\n" +
		"ip link add link eth0 name prot0 type vlan id 100\n" +
		"ip addr add 10.0.14.3/24 dev prot0\n" +
		"ip link set prot0 up\n" +
		"sysctl -w net.mpls.platform_labels=1048575 net.mpls.conf.eth0.input=1 net.mpls.conf.prot0.input=1 net.ipv4.ip_forward=1\n" +
		"ip route add 198.51.100.4/32 via 172.29.81.14 dev eth0\n" +
		"exec ze start /etc/ze/ze.conf\n"
	if command[2] != want {
		t.Fatalf("script:\n%s\nwant:\n%s", command[2], want)
	}
}

// VALIDATES: a Ze role with no <role>-net.txt still writes the MPLS sysctls
// and starts ze, with no VLAN and no route.
// PREVENTS: a scenario that declares no network starting ze without MPLS input.
func TestZeNodeScriptWithoutNetFile(t *testing.T) {
	script := zeNodeScript(zeNodeNet{})
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

// VALIDATES: every malformed <role>-net.txt line refuses the plan.
// PREVENTS: a typo silently running a scenario without its route or VLAN, or
// a VLAN name that would need shell quoting.
func TestReadZeNodeNetRefusesMalformedLines(t *testing.T) {
	for _, line := range []string{
		"bridge br0",
		"route 198.51.100.4/32",
		"route 198.51.100.4/32 through 172.29.81.14",
		"route 198.51.100.4 via 172.29.81.14",
		"route 198.51.100.5/24 via 172.29.81.14",
		"route 198.51.100.4/32 via gateway",
		"vlan prot0 id 100",
		"vlan prot0 tag 100 address 10.0.14.3/24",
		"vlan prot0 id 100 addr 10.0.14.3/24",
		"vlan prot0 id 0 address 10.0.14.3/24",
		"vlan prot0 id 4095 address 10.0.14.3/24",
		"vlan prot0 id x address 10.0.14.3/24",
		"vlan prot0 id 100 address 10.0.14.3",
		"vlan Prot0 id 100 address 10.0.14.3/24",
		"vlan prot0;reboot id 100 address 10.0.14.3/24",
		"vlan abcdefghijklmnop id 100 address 10.0.14.3/24",
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
