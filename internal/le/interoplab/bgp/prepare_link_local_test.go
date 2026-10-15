// Design: docs/architecture/testing/interop.md -- Container Addresses
// Related: prepare.go -- prepareScenario, renderScenario, linkLocalOwners
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

// VALIDATES: the link-local an interop container owns is decided by the
// placeholders the scenario's files carry, never by the scenario's name.
// Method: prepare four scenarios that differ only in which placeholders their
// files hold, and read each peer's start: a wrapped start runs
// `ip -6 address add <address>/64 dev eth0 nodad` and then execs the start it
// replaced.
// PREVENTS: a second link-local scenario editing the harness, or FRR never
// owning the address its peers are configured with.
func TestLinkLocalAssignmentFollowsThePlaceholders(t *testing.T) {
	network := interoplab.Network{
		Name: "lab",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	zeAddress := linkLocalAt(network, zeHostNumber).String()
	frrAddress := linkLocalAt(network, frrHostNumber).String()
	if zeAddress != "fe80::fd00:1e:2c:2" {
		t.Fatalf("Ze's link-local = %s, want fe80::fd00:1e:2c:2 (prefix in the identifier, host 2 last)", zeAddress)
	}
	if frrAddress != "fe80::fd00:1e:2c:3" {
		t.Fatalf("FRR's link-local = %s, want fe80::fd00:1e:2c:3", frrAddress)
	}
	for _, scenario := range []struct {
		name      string
		zeConfig  string
		frrConfig string
		wrapZe    bool
		wrapFRR   bool
	}{
		{"ze-only", "neighbor " + zeLinkLocalPlaceholder + "\n", "bfd\n", true, false},
		{"frr-only", "bfd {}\n", "peer " + frrLinkLocalPlaceholder + "\n", false, true},
		{"both", "local " + zeLinkLocalPlaceholder + "\n", "peer " + frrLinkLocalPlaceholder + " " + zeLinkLocalPlaceholder + "\n", true, true},
		// The name the harness used to compare against: carrying no placeholder,
		// it gets no assignment, so the name no longer decides.
		{"bgp-rfc2545-linklocal-nexthop-frr", "bgp {}\n", "router bgp 65002\n", false, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := t.TempDir()
			writeFixture(t, filepath.Join(source, zeConfigFile), scenario.zeConfig)
			writeFixture(t, filepath.Join(source, frrConfigFile), scenario.frrConfig)
			ipv6, err := needsIPv6(source)
			if err != nil {
				t.Fatal(err)
			}
			if wantIPv6 := scenario.wrapZe || scenario.wrapFRR; ipv6 != wantIPv6 {
				t.Fatalf("needsIPv6 = %v, want %v: a link-local renders from the lab's IPv6 prefix", ipv6, wantIPv6)
			}
			prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
				Source:  interoplab.ScenarioSource{Name: scenario.name, Directory: source},
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
			requireLinkLocalStart(t, prepared.Peers, "ze", zeAddress, scenario.wrapZe, "exec ze ", nil)
			requireLinkLocalStart(t, prepared.Peers, peerFRR, frrAddress, scenario.wrapFRR,
				"exec /sbin/tini ", []string{"--", "/usr/lib/frr/docker-start"})
		})
	}
}

// requireLinkLocalStart checks one peer's start: wrapped with the assignment of
// address and an exec of the replaced entrypoint, or left untouched.
func requireLinkLocalStart(t *testing.T, peers []interoplab.PeerConfig, name, address string, wrapped bool, exec string, imageCommand []string) {
	t.Helper()
	index := slices.IndexFunc(peers, func(peer interoplab.PeerConfig) bool { return peer.Name == name })
	if index < 0 {
		t.Fatalf("no %s peer in the prepared scenario", name)
	}
	peer := peers[index]
	start := strings.Join(peer.Command, " ")
	if !wrapped {
		if strings.Contains(start, "ip -6 address add") {
			t.Fatalf("%s's start assigns a link-local its scenario's files never name: %q", name, start)
		}
		return
	}
	if !slices.Equal(peer.Arguments[len(peer.Arguments)-2:], []string{dockerEntrypointFlag, "/bin/sh"}) {
		t.Fatalf("%s's arguments %q do not end with %s /bin/sh", name, peer.Arguments, dockerEntrypointFlag)
	}
	if strings.Count(strings.Join(peer.Arguments, " "), dockerEntrypointFlag) != 1 {
		t.Fatalf("%s's arguments %q name more than one entrypoint", name, peer.Arguments)
	}
	if len(peer.Command) < 3 {
		t.Fatalf("%s's command %q is not a shell script and its arguments", name, peer.Command)
	}
	if peer.Command[0] != shellErrexitCommand {
		t.Fatalf("%s's command opens %q, want %q", name, peer.Command[0], shellErrexitCommand)
	}
	if want := "ip -6 address add " + address + "/64 dev eth0 nodad\n"; !strings.HasPrefix(peer.Command[1], want) {
		t.Fatalf("%s's script = %q, want it to open with %q", name, peer.Command[1], want)
	}
	if !strings.Contains(peer.Command[1], exec) {
		t.Fatalf("%s's script = %q, want it to %q", name, peer.Command[1], exec)
	}
	if imageCommand != nil && !slices.Equal(peer.Command[3:], imageCommand) {
		t.Fatalf("%s's start passes %q, want the image's own command %q", name, peer.Command[3:], imageCommand)
	}
}

// VALIDATES: the RFC 2545 scenario prepares exactly as it did when the harness
// chose its link-local by the scenario's name: Ze's start assigns
// fe80::<prefix>:2 and execs ze with its own arguments, FRR's start is
// untouched, and ze.conf's `link-local` leaf names that same address.
// Method: the expected values are those the name-keyed code produced for this
// network, captured on 2026-10-15 before it was deleted.
// PREVENTS: the placeholder mapping moving Ze's next hop to another host
// number, or wrapping FRR in a scenario that names no FRR link-local.
func TestRFC2545ScenarioPreparesAsBefore(t *testing.T) {
	const scenarioName = "bgp-rfc2545-linklocal-nexthop-frr"
	network := interoplab.Network{
		Name: "lab",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	source := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", scenarioName)
	ipv6, err := needsIPv6(source)
	if err != nil {
		t.Fatal(err)
	}
	if !ipv6 {
		t.Fatal("the RFC 2545 scenario no longer selects an IPv6 lab network")
	}
	root := t.TempDir()
	prepared, err := prepareScenario(root, t.TempDir(), "fixture", interoplab.PrepareContext{
		Source:  interoplab.ScenarioSource{Name: scenarioName, Directory: source},
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
	ze := slices.IndexFunc(prepared.Peers, func(peer interoplab.PeerConfig) bool { return peer.Name == "ze" })
	if ze < 0 {
		t.Fatal("no ze peer in the prepared scenario")
	}
	wantArguments := []string{"--sysctl", "net.ipv6.conf.all.disable_ipv6=0", "--sysctl", "net.ipv6.conf.default.disable_ipv6=0", dockerEntrypointFlag, "/bin/sh"}
	if !slices.Equal(prepared.Peers[ze].Arguments, wantArguments) {
		t.Fatalf("ze's arguments = %q, want %q", prepared.Peers[ze].Arguments, wantArguments)
	}
	wantCommand := []string{shellErrexitCommand, "ip -6 address add fe80::fd00:1e:2c:2/64 dev eth0 nodad\nexec ze \"$@\"", "--", "start", "/etc/ze/bgp.conf"}
	if !slices.Equal(prepared.Peers[ze].Command, wantCommand) {
		t.Fatalf("ze's command = %q, want %q", prepared.Peers[ze].Command, wantCommand)
	}
	requireLinkLocalStart(t, prepared.Peers, peerFRR, "", false, "", nil)
	rendered := filepath.Join(root, "tmp", interoplab.RenderedConfigDirectory, scenarioName+"-fixture", zeConfigFile)
	config, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "\tlink-local fe80::fd00:1e:2c:2;\n") {
		t.Fatalf("rendered ze.conf does not name Ze's link-local fe80::fd00:1e:2c:2 in its link-local leaf:\n%s", config)
	}
	if strings.Contains(string(config), "@") {
		t.Fatalf("rendered ze.conf still carries a placeholder:\n%s", config)
	}
}

// VALIDATES: a scenario's registered prestart (scenarioPrestarts) runs in each
// owner's start wrapper, rendered, after the eth0 link-local and before the
// exec of the daemon, and a prestart that would never run is refused by name.
// Method: prepare the committed bfd-link-local-two-links-frr scenario and read
// each wrapped start in order; then render a prestart for an owner whose
// placeholder no scenario file carries.
// PREVENTS: the two links or the Your-Discriminator-zero drop being built after
// the daemon's first packet, an unrendered placeholder reaching `ip`, and a
// prestart silently skipped.
func TestPrestartRunsBeforeTheDaemon(t *testing.T) {
	network := interoplab.Network{
		Name: "lab",
		IPv4: netip.MustParsePrefix("172.30.44.0/24"),
		IPv6: netip.MustParsePrefix("fd00:1e:2c::/64"),
	}
	source := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", bfdLinkLocalTwoLinksScenario)
	prepared, err := prepareScenario(t.TempDir(), t.TempDir(), "fixture", interoplab.PrepareContext{
		Source:  interoplab.ScenarioSource{Name: bfdLinkLocalTwoLinksScenario, Directory: source},
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
	scripts := map[string]string{}
	for _, want := range []struct {
		peer  string
		steps []string
	}{
		{"ze", []string{
			"ip -6 address add fe80::fd00:1e:2c:2/64 dev eth0 nodad\n",
			"ip link add link eth0 name eth1 type vlan id 101\n",
			"ip link add link eth0 name eth2 type vlan id 102\n",
			"ip -6 address add fe80::fd00:1e:2c:2/64 dev eth1 nodad\n",
			"ip -6 address add fe80::fd00:1e:2c:2/64 dev eth2 nodad\n",
			"udp dport 3784 @th,128,32 0 drop\n",
			"exec ze ",
		}},
		{peerFRR, []string{
			"ip -6 address add fe80::fd00:1e:2c:3/64 dev eth0 nodad\n",
			"ip link add link eth0 name eth1 type vlan id 101\n",
			"ip link add link eth0 name eth2 type vlan id 102\n",
			"ip -6 address add fe80::fd00:1e:2c:3/64 dev eth1 nodad\n",
			"ip -6 address add fe80::fd00:1e:2c:3/64 dev eth2 nodad\n",
			"exec /sbin/tini ",
		}},
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
		scripts[want.peer] = script
		if strings.Contains(script, "_LINK_LOCAL@") {
			t.Fatalf("%s wrapper carries an unrendered placeholder:\n%s", want.peer, script)
		}
		at := 0
		for _, step := range want.steps {
			found := strings.Index(script[at:], step)
			if found < 0 {
				t.Fatalf("%s wrapper lacks %q after offset %d:\n%s", want.peer, step, at, script)
			}
			at += found + len(step)
		}
	}
	if strings.Contains(scripts[peerFRR], "nft") {
		t.Fatalf("FRR's wrapper carries the nft drop, which belongs to ze's sends alone:\n%s", scripts[peerFRR])
	}

	t.Run("owner without its placeholder", func(t *testing.T) {
		var linkLocals renderedLinkLocals
		linkLocals[frrLinkLocalOwner] = true
		_, err := renderPrestarts("fixture", ownerPrestarts{zeLinkLocalOwner: "true\n"}, linkLocals, network)
		want := "no file of the scenario carries " + zeLinkLocalPlaceholder
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to contain %q", err, want)
		}
	})
}
