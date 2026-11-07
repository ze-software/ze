package bgp

import (
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: isis-import-policy-frr's links, declared to the core
// (linkDeclarations), are built in each member's start wrapper before its
// daemon starts, with the devices and addresses the scenario's files name, each member on its own
// eth<N> at its host number, and only members join a link.
// Method: prepare the committed isis-import-policy-frr scenario (FRR-A -- Ze --
// FRR-B) and read each wrapped start in order.
// PREVENTS: FRR-A and FRR-B sharing a segment (FRR-B would learn FRR-A's
// prefixes directly, so the flooding assertion would pass with Ze flooding
// nothing), a link built after the daemon's first hello, and a wrapper that
// assigns a link-local the scenario never asked for.
func TestBGPImportPolicyLinksUnchanged(t *testing.T) {
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
	peers := composedPeers(t, prepared.Peers, linkDeclarations[isisImportPolicyScenario])
	if err := interoplab.ValidateLinks(isisImportPolicyScenario, linkDeclarations[isisImportPolicyScenario], prepared.Peers); err != nil {
		t.Fatal(err)
	}
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
			wrapperExec,
		}, []string{"dev eth0 nodad"}},
		{peerFRR, []string{
			"ip link add link eth0 name eth1 type vlan id 101\n",
			"ip address add 10.101.0.3/24 dev eth1\n",
			wrapperExec,
		}, []string{"vlan id 102", "dev eth0 nodad"}},
		{peerFRRSink, []string{
			"ip link add link eth0 name eth1 type vlan id 102\n",
			"ip address add 10.102.0.15/24 dev eth1\n",
			wrapperExec,
		}, []string{"vlan id 101", "dev eth0 nodad"}},
	} {
		index := slices.IndexFunc(peers, func(p interoplab.PeerConfig) bool { return p.Name == want.peer })
		if index < 0 {
			t.Fatalf("no %s peer prepared", want.peer)
		}
		command := peers[index].Command
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
