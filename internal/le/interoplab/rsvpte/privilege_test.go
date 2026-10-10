// Design: docs/architecture/testing/interop.md -- the RSVP-TE lab's peers
// Related: rsvpte.go -- zePeer, mplsPreflightContainer, mplsPreflightRefusal
//
// No RSVP-TE lab container runs privileged or loads a kernel module
// (spec-lab-containers-least-privilege, AC-5). A Ze node holds NET_ADMIN for
// its MPLS routes and VLANs, and writes its MPLS sysctls at run time under the
// generic net sysctl profile; the preflight reads the MPLS sysctl tree with no
// grant at all. The tests prepare every real scenario and read the docker run
// inputs and the setup scripts; no container starts.

package rsvpte

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// setupSysctlKeys answers every key a setup script writes with `sysctl -w`.
func setupSysctlKeys(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path) // #nosec G304 -- a repository-owned scenario script.
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close %s: %v", path, err)
		}
	}()

	var keys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] != "sysctl" || fields[1] != "-w" {
			continue
		}
		for _, assignment := range fields[2:] {
			key, _, found := strings.Cut(assignment, "=")
			if found {
				keys = append(keys, key)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

// VALIDATES: AC-5 for the peers. No peer of any scenario runs --privileged; a
// Ze node holds NET_ADMIN, gets /proc/sys writable under the generic net
// sysctl profile, and that profile grants every key its setup script writes.
// PREVENTS: a node silently regaining --privileged, or a scenario script
// writing a sysctl the profile denies.
func TestRSVPTELabPeersRunWithoutPrivilege(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := interoplab.Discover(filepath.Join(root, "test", "interop-rsvpte", "scenarios"), "", scenarioCheckerMap(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) == 0 {
		t.Fatal("no RSVP-TE scenario discovered")
	}
	for _, source := range sources {
		plan, err := scenarioPlan("privilege-test", source)
		if err != nil {
			t.Fatalf("plan %s: %v", source.Name, err)
		}
		for _, peer := range plan.Peers {
			if slices.Contains(peer.Arguments, "--privileged") {
				t.Errorf("%s: peer %s runs --privileged", source.Name, peer.Name)
			}
			if peer.Image != imageZe {
				continue
			}
			if !slices.Contains(peer.Capabilities, "NET_ADMIN") {
				t.Errorf("%s: Ze node %s lacks NET_ADMIN: %v", source.Name, peer.Name, peer.Capabilities)
			}
			for _, argument := range interoplab.NetSysctlWriteArguments() {
				if !slices.Contains(peer.Arguments, argument) {
					t.Errorf("%s: Ze node %s lacks %s: %v", source.Name, peer.Name, argument, peer.Arguments)
				}
			}
			if peer.AppArmorProfile != interoplab.NetSysctlAppArmorProfileName {
				t.Errorf("%s: Ze node %s runs under %q, want %s", source.Name, peer.Name, peer.AppArmorProfile, interoplab.NetSysctlAppArmorProfileName)
			}
			for _, key := range setupSysctlKeys(t, filepath.Join(source.Directory, peer.Name+"-setup.sh")) {
				if !interoplab.NetSysctlGranted(key) {
					t.Errorf("%s: %s-setup.sh writes %s, which %s denies", source.Name, peer.Name, key, interoplab.NetSysctlAppArmorProfileName)
				}
			}
		}
	}
}

// VALIDATES: AC-5 for the preflight. The probe runs with no grant, mounts no
// module tree and loads no module; a host without MPLS routing is refused with
// the host command that loads it on a line of its own.
// PREVENTS: the preflight passing because it loaded mpls_router itself.
func TestRSVPTEPreflightRunsWithoutPrivilege(t *testing.T) {
	container := mplsPreflightContainer("test")
	for _, argument := range container.Arguments {
		if argument == "--privileged" || argument == "--cap-add" || strings.Contains(argument, "/lib/modules") {
			t.Errorf("preflight holds a grant or the module tree: %v", container.Arguments)
		}
	}
	script := strings.Join(container.Command, " ")
	for _, banned := range []string{"modprobe", "kmod"} {
		if strings.Contains(script, banned) {
			t.Errorf("preflight script still uses %s: %s", banned, script)
		}
	}
	if err := mplsPreflightRefusal("MPLS=ok\n"); err != nil {
		t.Errorf("a host with MPLS routing refused: %v", err)
	}
	err := mplsPreflightRefusal("MPLS=missing\n")
	if err == nil || !strings.Contains(err.Error(), "\n  sudo modprobe -a mpls_router mpls_iptunnel\n") {
		t.Errorf("refusal %v does not print the host command on a line of its own", err)
	}
}
