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
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

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
			want := grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
			for _, problem := range grantcheck.Problems(grantcheck.Parse(peer.Capabilities, peer.Arguments), want) {
				t.Errorf("%s: peer %s %s", source.Name, peer.Name, problem)
			}
			for _, mount := range peer.Mounts {
				if mount.Source == "/lib/modules" {
					t.Errorf("%s: peer %s mounts the host's /lib/modules", source.Name, peer.Name)
				}
			}
		}
	}
}

// VALIDATES: AC-4 for the preflight. The probe container holds the peers'
// grants, mounts no module tree, and loads no module.
// PREVENTS: the preflight passing because it loaded a module itself.
func TestPPPoEPreflightRunsWithoutPrivilege(t *testing.T) {
	container := preflightContainer("test")
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
	err := validatePreflightOutput("DEV_PPP=missing\nPPPOE=missing\n")
	if err == nil || !strings.HasSuffix(err.Error(), "\n  sudo modprobe -a ppp_generic pppoe") {
		t.Errorf("refusal %v does not print the host command on a line of its own", err)
	}
}

// VALIDATES: R-4 for a host with no /dev/ppp at all. Docker refuses the
// preflight's --device before the probe runs, and the refusal names the module
// that provides the device on a line of its own, as the L2TP lab's does; any
// other failure keeps Docker's words and names no module.
// PREVENTS: an operator reading only Docker's "error gathering device
// information" with no fix.
func TestPPPoEPreflightWithoutDevPPPNamesTheHostFix(t *testing.T) {
	stderr := `docker: Error response from daemon: error gathering device information while adding custom device "/dev/ppp": no such file or directory.`
	err := preflightRunRefusal(125, stderr+"\n")
	if err == nil {
		t.Fatal("a refused container was not reported")
	}
	if !strings.HasSuffix(err.Error(), "\n  sudo modprobe ppp_generic") {
		t.Errorf("refusal %q does not end with the host command on a line of its own", err)
	}
	if !strings.Contains(err.Error(), stderr) {
		t.Errorf("refusal %q drops Docker's own words", err)
	}

	other := preflightRunRefusal(1, "pull access denied")
	if other == nil || strings.Contains(other.Error(), "modprobe") {
		t.Errorf("a failure unrelated to /dev/ppp answered %v", other)
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
