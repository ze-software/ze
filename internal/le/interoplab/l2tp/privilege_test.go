// Design: docs/architecture/testing/interop.md -- the L2TP lab's peers
// Related: l2tp.go -- zePeer, lacPeer, preflightContainer, preflightRefusal
//
// No L2TP lab container runs privileged or loads a kernel module
// (spec-lab-containers-least-privilege, AC-3): ze and the xl2tpd LAC hold
// NET_ADMIN and the PPP device, and the preflight probes with the same grants.
// The tests prepare every real scenario and read the docker run inputs; no
// container starts.

package l2tp

import (
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

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

// VALIDATES: AC-3 for the peers. ze and the LAC in every scenario hold
// NET_ADMIN and --device /dev/ppp, and neither runs --privileged nor mounts the
// host's /lib/modules.
// PREVENTS: a peer silently regaining --privileged or a module tree.
func TestL2TPLabPeersRunWithoutPrivilege(t *testing.T) {
	root := l2tpCheckout(t)
	plans, _, err := plansAt(root, "", testEnvironment())
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	network := interoplab.Network{Name: "n", IPv4: netip.MustParsePrefix("172.29.0.0/24")}
	for _, plan := range plans {
		scenario, err := plan.Prepare(t.Context(), interoplab.PrepareContext{Source: plan.Source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", plan.Source.Name, err)
		}
		if scenario.Cleanup != nil {
			cleanup := scenario.Cleanup
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Errorf("cleanup %s: %v", plan.Source.Name, err)
				}
			})
		}
		for _, peer := range scenario.Peers {
			if slices.Contains(peer.Arguments, "--privileged") {
				t.Errorf("%s: peer %s runs --privileged", plan.Source.Name, peer.Name)
			}
			for _, mount := range peer.Mounts {
				if mount.Source == "/lib/modules" {
					t.Errorf("%s: peer %s mounts the host's /lib/modules", plan.Source.Name, peer.Name)
				}
			}
			if peer.Name != peerZe && peer.Name != peerLAC {
				continue
			}
			if !slices.Contains(peer.Capabilities, "NET_ADMIN") {
				t.Errorf("%s: peer %s lacks NET_ADMIN: %v", plan.Source.Name, peer.Name, peer.Capabilities)
			}
			if !slices.Contains(argumentPairs(peer.Arguments, "--device"), "/dev/ppp") {
				t.Errorf("%s: peer %s lacks --device /dev/ppp: %v", plan.Source.Name, peer.Name, peer.Arguments)
			}
		}
	}
}

// VALIDATES: AC-3 for the preflight. The probe container holds the peers'
// grants, mounts no module tree, and loads no module.
// PREVENTS: the preflight passing because it loaded a module itself, on a
// host where the lab's unprivileged peers would then fail.
func TestL2TPPreflightRunsWithoutPrivilege(t *testing.T) {
	container := preflightContainer("test")
	if slices.Contains(container.Arguments, "--privileged") {
		t.Errorf("preflight runs --privileged: %v", container.Arguments)
	}
	if !slices.Contains(argumentPairs(container.Arguments, "--cap-add"), "NET_ADMIN") {
		t.Errorf("preflight lacks --cap-add NET_ADMIN: %v", container.Arguments)
	}
	if !slices.Contains(argumentPairs(container.Arguments, "--device"), "/dev/ppp") {
		t.Errorf("preflight lacks --device /dev/ppp: %v", container.Arguments)
	}
	for _, argument := range container.Arguments {
		if strings.Contains(argument, "/lib/modules") {
			t.Errorf("preflight mounts the host's module tree: %v", container.Arguments)
		}
	}
	script := strings.Join(container.Command, " ")
	for _, banned := range []string{"modprobe", "kmod"} {
		if strings.Contains(script, banned) {
			t.Errorf("preflight script still uses %s: %s", banned, script)
		}
	}
}

// VALIDATES: AC-3, R-4. A host that lacks a module refuses with the missing
// part named and the host command that loads it on a line of its own.
// PREVENTS: a refusal that names the symptom and leaves the operator to find
// the fix.
func TestL2TPPreflightRefusalNamesTheHostFix(t *testing.T) {
	if err := preflightRefusal(map[string]string{"DEV_PPP": "ok", "L2TP_PPP": "ok", "IP_L2TP": "ok"}); err != nil {
		t.Fatalf("a host with every part refused: %v", err)
	}
	err := preflightRefusal(map[string]string{"DEV_PPP": "missing", "L2TP_PPP": "missing", "IP_L2TP": "missing"})
	if err == nil {
		t.Fatal("a host with no part passed")
	}
	message := err.Error()
	for _, want := range []string{"/dev/ppp", "l2tp_ppp", "L2TP Generic Netlink", "\n  sudo modprobe -a ppp_generic l2tp_ppp l2tp_netlink\n"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q does not name %q", message, want)
		}
	}
}
