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
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
)

// VALIDATES: AC-3 for the peers. ze and the LAC in every scenario hold
// exactly NET_ADMIN and --device /dev/ppp, FRR exactly NET_ADMIN and
// SYS_ADMIN, every other peer no grant,
// and none runs --privileged or mounts the host's /lib/modules.
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
			for _, mount := range peer.Mounts {
				if mount.Source == "/lib/modules" {
					t.Errorf("%s: peer %s mounts the host's /lib/modules", plan.Source.Name, peer.Name)
				}
			}
			// ze and the LAC open PPP units; FRR holds the NET_ADMIN and
			// SYS_ADMIN of the shared FRR peer; every other peer holds none.
			var want grantcheck.Grants
			switch peer.Name {
			case peerZe, peerLAC:
				want = grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
			case peerFRR:
				want = grantcheck.Grants{Capabilities: []string{"NET_ADMIN", "SYS_ADMIN"}}
			}
			for _, problem := range grantcheck.Problems(grantcheck.ParsePeer(peer, grantcheck.StagedRoots(t)...), want) {
				t.Errorf("%s: peer %s %s", plan.Source.Name, peer.Name, problem)
			}
		}
	}
}

// VALIDATES: AC-3 for the preflight. The probe container holds the peers'
// grants, mounts no module tree, and loads no module.
// PREVENTS: the preflight passing because it loaded a module itself, on a
// host where the lab's unprivileged peers would then fail.
func TestL2TPPreflightRunsWithoutPrivilege(t *testing.T) {
	container := preflightContainer()
	want := grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
	for _, problem := range grantcheck.Problems(grantcheck.Parse(nil, container.Arguments), want) {
		t.Errorf("the preflight container %s: %v", problem, container.Arguments)
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
	for _, want := range []string{"/dev/ppp", "l2tp_ppp", "L2TP Generic Netlink"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q does not name %q", message, want)
		}
	}
	// The command ends the message on a line of its own, with no trailing
	// newline: the caller prints the error and adds its own.
	if !strings.HasSuffix(message, "\n  sudo modprobe -a ppp_generic l2tp_ppp l2tp_netlink") {
		t.Errorf("refusal %q does not end with the host command on a line of its own", message)
	}
}

// VALIDATES: AC-3 for the images. No L2TP lab image installs kmod or runs
// modprobe; the twin of TestPPPoELabImagesLoadNoModule.
// PREVENTS: an entrypoint or a daemon start that loads a module on a host that
// granted it, hiding the host's missing setup from the preflight. ze's own
// probe (probeKernelModules) reads /sys/module first, so on a host the
// preflight passed it never reaches modprobe.
func TestL2TPLabImagesLoadNoModule(t *testing.T) {
	root := l2tpCheckout(t)
	plans, _, err := plansAt(root, "", testEnvironment())
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	builds := imageBuilds(root, plans, "frr-image")
	if len(builds) == 0 {
		t.Fatal("the L2TP lab builds no image")
	}
	for _, image := range builds {
		if image.Dockerfile == "" {
			continue
		}
		content, err := os.ReadFile(image.Dockerfile)
		if err != nil {
			t.Fatalf("read %s: %v", image.Dockerfile, err)
		}
		for line := range strings.SplitSeq(string(content), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, banned := range []string{"modprobe", "kmod"} {
				if strings.Contains(line, banned) {
					t.Errorf("%s still uses %s: %s", image.Dockerfile, banned, line)
				}
			}
		}
	}
}
