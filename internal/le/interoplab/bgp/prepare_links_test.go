package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: a scenario's declared links (scenarioLinks) are built in each
// member's start wrapper before its daemon starts, each member on its own
// eth<N> at its host number, and only members join a link.
// Method: prepare the committed isis-import-policy-frr scenario (FRR-A -- Ze --
// FRR-B) and read each wrapped start in order.
// PREVENTS: FRR-A and FRR-B sharing a segment (FRR-B would learn FRR-A's
// prefixes directly, so the flooding assertion would pass with Ze flooding
// nothing), a link built after the daemon's first hello, and a wrapper that
// assigns a link-local the scenario never asked for.
func TestScenarioLinksBuildEachMemberBeforeItsDaemon(t *testing.T) {
	network := interoplab.Network{Name: "lab", IPv4: netip.MustParsePrefix("172.30.44.0/24")}
	source := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", isisImportPolicyScenario)
	prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
		Source:  interoplab.ScenarioSource{Name: isisImportPolicyScenario, Directory: source},
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
	for _, want := range []struct {
		peer   string
		steps  []string
		absent []string
	}{
		{"ze", []string{
			"ip link add link eth0 name eth1 type vlan id 101\n",
			"ip address add 10.101.0.2/24 dev eth1\n",
			"ip link set eth1 up\n",
			"ip link add link eth0 name eth2 type vlan id 102\n",
			"ip address add 10.102.0.2/24 dev eth2\n",
			"ip link set eth2 up\n",
			"exec ze ",
		}, []string{"dev eth0 nodad"}},
		{peerFRR, []string{
			"ip link add link eth0 name eth1 type vlan id 101\n",
			"ip address add 10.101.0.3/24 dev eth1\n",
			"exec /sbin/tini ",
		}, []string{"vlan id 102", "dev eth0 nodad"}},
		{peerFRRSink, []string{
			"ip link add link eth0 name eth1 type vlan id 102\n",
			"ip address add 10.102.0.15/24 dev eth1\n",
			"exec /sbin/tini ",
		}, []string{"vlan id 101", "dev eth0 nodad"}},
	} {
		index := slices.IndexFunc(prepared.Peers, func(p interoplab.PeerConfig) bool { return p.Name == want.peer })
		if index < 0 {
			t.Fatalf("no %s peer prepared", want.peer)
		}
		command := prepared.Peers[index].Command
		if len(command) < 2 || command[0] != shellErrexitCommand {
			t.Fatalf("%s start = %q, want a %s wrapper", want.peer, command, shellErrexitCommand)
		}
		script := command[1]
		at := 0
		for _, step := range want.steps {
			found := strings.Index(script[at:], step)
			if found < 0 {
				t.Fatalf("%s wrapper lacks %q after offset %d:\n%s", want.peer, step, at, script)
			}
			at += found + len(step)
		}
		for _, absent := range want.absent {
			if strings.Contains(script, absent) {
				t.Fatalf("%s wrapper carries %q:\n%s", want.peer, absent, script)
			}
		}
	}
}

// VALIDATES: renderLinks refuses every link it cannot build as declared, each
// with its own reason.
// Method: one declaration per refusal, each checked for the reason's words.
// PREVENTS: a link on a reserved VLAN, a host address as the subnet, a link
// with one member, or a member whose container never starts, each of which
// leaves a lab that runs and proves nothing.
func TestRenderLinksRefusals(t *testing.T) {
	peers := []interoplab.PeerConfig{{Name: "ze"}, {Name: peerFRR}}
	both := []int{zeLinkLocalOwner, frrLinkLocalOwner}
	subnet := netip.MustParsePrefix("10.101.0.0/24")
	for _, tc := range []struct {
		name   string
		links  []labLink
		reason string
	}{
		{"vlan zero", []labLink{{vlan: 0, ipv4: subnet, members: both}}, "not a usable VLAN"},
		{"vlan reserved", []labLink{{vlan: 4095, ipv4: subnet, members: both}}, "not a usable VLAN"},
		{"vlan twice", []labLink{{vlan: 101, ipv4: subnet, members: both}, {vlan: 101, ipv4: subnet, members: both}}, "declared twice"},
		{"no subnet", []labLink{{vlan: 101, members: both}}, "has no subnet"},
		{"ipv6 subnet", []labLink{{vlan: 101, ipv4: netip.MustParsePrefix("fd00::/64"), members: both}}, "needs an IPv4 subnet"},
		{"host not network", []labLink{{vlan: 101, ipv4: netip.MustParsePrefix("10.101.0.1/24"), members: both}}, "names a host"},
		{"too long", []labLink{{vlan: 101, ipv4: netip.MustParsePrefix("10.101.0.0/28"), members: both}}, "longer than a /24"},
		{"one member", []labLink{{vlan: 101, ipv4: subnet, members: []int{zeLinkLocalOwner}}}, "fewer than two members"},
		{"member twice", []labLink{{vlan: 101, ipv4: subnet, members: []int{zeLinkLocalOwner, zeLinkLocalOwner}}}, "twice; name each member once"},
		{"unknown owner", []labLink{{vlan: 101, ipv4: subnet, members: []int{zeLinkLocalOwner, len(linkLocalOwners)}}}, "outside linkLocalOwners"},
		{"member not started", []labLink{{vlan: 101, ipv4: subnet, members: []int{zeLinkLocalOwner, frrSinkLinkLocalOwner}}}, "does not start that container"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := renderLinks("fixture", tc.links, peers)
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.reason)
			}
		})
	}
	t.Run("valid", func(t *testing.T) {
		rendered, err := renderLinks("fixture", []labLink{{vlan: 101, ipv4: subnet, members: both}}, peers)
		if err != nil {
			t.Fatal(err)
		}
		if rendered[frrSinkLinkLocalOwner] != "" {
			t.Fatalf("frr-sink is on no link, its script = %q", rendered[frrSinkLinkLocalOwner])
		}
		if !strings.Contains(rendered[frrLinkLocalOwner], "10.101.0.3/24 dev eth1") {
			t.Fatalf("frr script = %q", rendered[frrLinkLocalOwner])
		}
	})
}
