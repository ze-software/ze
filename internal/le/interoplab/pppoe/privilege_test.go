// Design: docs/architecture/testing/interop.md -- the PPPoE lab's peers
// Related: scenarios.go -- prepareZeClient, prepareZeAccessConcentrator
// Related: pppoe.go -- preflightContainer, validatePreflightOutput
//
// No PPPoE lab container runs privileged or loads a kernel module
// (spec-lab-containers-least-privilege, AC-4): ze, accel-ppp and the pppd
// client hold NET_ADMIN and the PPP device, the preflight probes with the same
// grants, and no image's entrypoint runs modprobe. The tests prepare every real
// scenario and read the docker run inputs and the Dockerfiles; no container
// starts.

package pppoe

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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

// VALIDATES: AC-4 for the peers. Every peer of every scenario holds NET_ADMIN
// and --device /dev/ppp, runs no --privileged, and mounts no /lib/modules.
// PREVENTS: a peer silently regaining --privileged or the host's module tree.
func TestPPPoELabPeersRunWithoutPrivilege(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := interoplab.Discover(filepath.Join(root, suitePath, "scenarios"), "", checkers())
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) == 0 {
		t.Fatal("no PPPoE scenario discovered")
	}
	for _, source := range sources {
		plan, err := scenarioPlan(source, "privilege-test")
		if err != nil {
			t.Fatalf("plan %s: %v", source.Name, err)
		}
		for _, peer := range plan.Peers {
			if slices.Contains(peer.Arguments, "--privileged") {
				t.Errorf("%s: peer %s runs --privileged", source.Name, peer.Name)
			}
			for _, mount := range peer.Mounts {
				if mount.Source == "/lib/modules" {
					t.Errorf("%s: peer %s mounts the host's /lib/modules", source.Name, peer.Name)
				}
			}
			if !slices.Contains(peer.Capabilities, "NET_ADMIN") {
				t.Errorf("%s: peer %s lacks NET_ADMIN: %v", source.Name, peer.Name, peer.Capabilities)
			}
			if !slices.Contains(argumentPairs(peer.Arguments, "--device"), "/dev/ppp") {
				t.Errorf("%s: peer %s lacks --device /dev/ppp: %v", source.Name, peer.Name, peer.Arguments)
			}
		}
	}
}

// VALIDATES: AC-4 for the preflight. The probe container holds the peers'
// grants, mounts no module tree, and loads no module.
// PREVENTS: the preflight passing because it loaded a module itself.
func TestPPPoEPreflightRunsWithoutPrivilege(t *testing.T) {
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
	err := validatePreflightOutput("DEV_PPP=missing\nPPPOE=missing\n")
	if err == nil || !strings.Contains(err.Error(), "\n  sudo modprobe -a ppp_generic pppoe\n") {
		t.Errorf("refusal %v does not print the host command on a line of its own", err)
	}
}

// VALIDATES: AC-4 for the images. No PPPoE lab image installs kmod or runs
// modprobe.
// PREVENTS: an entrypoint that loads a module on a host that granted it,
// hiding the host's missing setup from the preflight.
func TestPPPoELabImagesLoadNoModule(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, image := range imageBuilds(root) {
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
