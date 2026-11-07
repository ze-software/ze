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

// VALIDATES: AC-4 for the peers. Every peer of every scenario holds exactly
// NET_ADMIN and --device /dev/ppp, plus the IPv6 grants peerGrants names only
// where the scenario carries the ipv6-session marker, runs no --privileged,
// and mounts no /lib/modules.
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
		ipv6, err := ipv6SessionScenario(source)
		if err != nil {
			t.Fatalf("plan %s: %v", source.Name, err)
		}
		for _, peer := range plan.Peers {
			want := peerGrants(peer.Name, ipv6)
			for _, problem := range grantcheck.Problems(grantcheck.ParsePeer(peer, grantcheck.StagedRoots(t)...), want) {
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

// peerGrants is the exact set of grants a peer of a PPPoE scenario holds:
// NET_ADMIN and /dev/ppp for every peer, and, only in a scenario carrying the
// ipv6-session marker, the namespace's IPv6 default kept enabled
// (ipv6SessionArguments) on both peers, plus the generic net-sysctl write
// grant on Ze alone, because the client writes accept_ra, autoconf and
// forwarding on each session device it creates. accel-ppp writes no sysctl
// the scenario relies on, so it never gets the write grant.
func peerGrants(peer string, ipv6 bool) grantcheck.Grants {
	grants := grantcheck.Grants{Capabilities: []string{"NET_ADMIN"}, Devices: []string{"/dev/ppp"}}
	if !ipv6 {
		return grants
	}
	grants.Sysctls = grantcheck.Parse(nil, ipv6SessionArguments()).Sysctls
	if peer == zeImageName {
		grants.SecurityOptions = grantcheck.Parse(nil, interoplab.NetSysctlWriteArguments()).SecurityOptions
		grants.AppArmorProfile = interoplab.NetSysctlAppArmorProfileName
	}
	return grants
}

// VALIDATES: the IPv6 grant is exact and confined. Some scenario carries the
// ipv6-session marker (so the grant above is exercised, not dead), and every
// sysctl the Ze client writes on its session device is one the
// ze-lab-net-sysctl profile admits.
// PREVENTS: a marker that grants writes the profile then denies, and a grant
// list that no scenario reaches.
func TestPPPoEIPv6SessionGrantIsConfined(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := interoplab.Discover(filepath.Join(root, suitePath, "scenarios"), "", checkers())
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, source := range sources {
		ipv6, err := ipv6SessionScenario(source)
		if err != nil {
			t.Fatalf("%s: %v", source.Name, err)
		}
		if ipv6 {
			marked++
		}
	}
	if marked == 0 {
		t.Fatal("no PPPoE scenario carries the ipv6-session marker")
	}
	for _, suffix := range []string{"accept_ra", "autoconf", "forwarding"} {
		key := "net.ipv6.conf.pppoe0." + suffix
		if !interoplab.NetSysctlGranted(key) {
			t.Errorf("%s is denied by %s", key, interoplab.NetSysctlAppArmorProfileName)
		}
	}
}

// VALIDATES: AC-4 for the preflight. The probe container holds the peers'
// grants, mounts no module tree, and loads no module.
// PREVENTS: the preflight passing because it loaded a module itself.
func TestPPPoEPreflightRunsWithoutPrivilege(t *testing.T) {
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
	err := validatePreflightOutput("DEV_PPP=missing\nPPPOE=missing\n")
	if err == nil || !strings.HasSuffix(err.Error(), "\n  sudo modprobe -a ppp_generic pppoe") {
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
