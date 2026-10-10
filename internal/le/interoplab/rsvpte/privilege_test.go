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
	"github.com/ze-software/ze/internal/le/interoplab/grantcheck"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// setupSysctlKeys answers every key a setup script writes, and fails the test
// on a line that writes /proc/sys in a form scriptSysctlWrites cannot read,
// because a write it cannot name is a write the profile check never sees.
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

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	keys, unrecognized := scriptSysctlWrites(lines)
	for _, line := range unrecognized {
		t.Errorf("%s writes a sysctl in a form the privilege test cannot read: %q; "+
			"write it as `sysctl -w key=value` or `echo value > /proc/sys/<key path>`", path, line)
	}
	return keys
}

// scriptSysctlWrites answers the keys, spelled as sysctl spells them, that
// lines write with `sysctl -w key=value ...` or `echo value > /proc/sys/...`,
// and every other line that writes a sysctl: `sysctl` loading a file or given
// an assignment without -w, a -w argument that is no assignment, and a
// redirect, tee or dd into /proc/sys. A line that only reads is neither.
func scriptSysctlWrites(lines []string) (keys, unrecognized []string) {
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "sysctl" {
			lineKeys, recognized := sysctlCommandKeys(fields[1:])
			if !recognized {
				unrecognized = append(unrecognized, line)
			}
			keys = append(keys, lineKeys...)
			continue
		}
		if !strings.Contains(line, procSysRoot) {
			continue
		}
		if key, recognized := echoProcSysKey(fields); recognized {
			keys = append(keys, key)
			continue
		}
		if procSysWritten(line) {
			unrecognized = append(unrecognized, line)
		}
	}
	return keys, unrecognized
}

const procSysRoot = "/proc/sys/"

// sysctlCommandKeys answers the keys of `sysctl -w a=1 b=2`, and whether the
// arguments are that form or a read. A read answers no key and true.
func sysctlCommandKeys(arguments []string) (keys []string, recognized bool) {
	if len(arguments) > 0 && arguments[0] == "-w" {
		for _, assignment := range arguments[1:] {
			key, _, found := strings.Cut(assignment, "=")
			if !found {
				return nil, false
			}
			keys = append(keys, key)
		}
		return keys, len(keys) > 0
	}
	for _, argument := range arguments {
		if strings.Contains(argument, "=") {
			return nil, false
		}
		if argument == "-p" || argument == "--load" || argument == "--system" {
			return nil, false
		}
	}
	return nil, true
}

// echoProcSysKey answers the key `echo value > /proc/sys/a/b` writes (a.b),
// with the redirect spelled `> path` or `>path`.
func echoProcSysKey(fields []string) (string, bool) {
	if fields[0] != "echo" {
		return "", false
	}
	for index, field := range fields {
		target, redirect := strings.CutPrefix(field, ">")
		if !redirect {
			continue
		}
		if target == "" && index+1 < len(fields) {
			target = fields[index+1]
		}
		path, under := strings.CutPrefix(target, procSysRoot)
		if !under {
			return "", false
		}
		return strings.ReplaceAll(path, "/", "."), path != ""
	}
	return "", false
}

// procSysWritten reports whether a line that names /proc/sys writes there:
// through a redirect, tee or dd.
func procSysWritten(line string) bool {
	for _, writer := range []string{">", "tee ", "dd "} {
		if strings.Contains(line, writer) {
			return true
		}
	}
	return false
}

// VALIDATES: NOTE 13 of spec-lab-containers-least-privilege review round 1.
// The two write forms are read into keys, and every other form that writes a
// sysctl is refused rather than skipped.
// PREVENTS: a setup script writing a sysctl the profile denies through a form
// the privilege test does not parse, and the test staying green.
func TestScriptSysctlWritesRefusesFormsItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		line         string
		key          string
		unrecognized bool
	}{
		{line: "sysctl -w net.mpls.platform_labels=1048575 net.ipv4.ip_forward=1", key: "net.mpls.platform_labels net.ipv4.ip_forward"},
		{line: "echo 1 > /proc/sys/net/ipv4/ip_forward", key: "net.ipv4.ip_forward"},
		{line: "echo 1 >/proc/sys/kernel/panic", key: "kernel.panic"},
		{line: "cat /proc/sys/net/mpls/platform_labels"},
		{line: "sysctl net.ipv4.ip_forward"},
		{line: "echo MPLS=$(test -f /proc/sys/net/mpls/platform_labels && echo ok || echo missing)"},
		{line: "sysctl net.ipv4.ip_forward=1", unrecognized: true},
		{line: "sysctl -p /etc/sysctl.d/lab.conf", unrecognized: true},
		{line: "sysctl -w net.ipv4.ip_forward", unrecognized: true},
		{line: "printf 1 | tee /proc/sys/kernel/panic", unrecognized: true},
		{line: "echo 1 | dd of=/proc/sys/kernel/panic", unrecognized: true},
		{line: "printf 1 > /proc/sys/kernel/panic", unrecognized: true},
		{line: "echo 1 > /run/x; echo 2 > /proc/sys/kernel/panic", unrecognized: true},
	} {
		keys, unrecognized := scriptSysctlWrites([]string{tc.line})
		if got := strings.Join(keys, " "); got != tc.key {
			t.Errorf("%q: keys %q, want %q", tc.line, got, tc.key)
		}
		if got := len(unrecognized) == 1; got != tc.unrecognized {
			t.Errorf("%q: unrecognized = %v, want %v", tc.line, got, tc.unrecognized)
		}
	}
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
			// The scripted RSVP speakers send raw RSVP and program their
			// routes: NET_ADMIN and NET_RAW, nothing else.
			want := grantcheck.Grants{Capabilities: []string{"NET_ADMIN", "NET_RAW"}}
			if peer.Image == imageZe {
				want = grantcheck.Grants{
					Capabilities:    []string{"NET_ADMIN"},
					SecurityOptions: grantcheck.Parse(nil, interoplab.NetSysctlWriteArguments()).SecurityOptions,
				}
			}
			for _, problem := range grantcheck.Problems(grantcheck.Parse(peer.Capabilities, peer.Arguments), want) {
				t.Errorf("%s: peer %s %s", source.Name, peer.Name, problem)
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
	if err == nil || !strings.HasSuffix(err.Error(), "\n  sudo modprobe -a mpls_router mpls_iptunnel") {
		t.Errorf("refusal %v does not print the host command on a line of its own", err)
	}
}
