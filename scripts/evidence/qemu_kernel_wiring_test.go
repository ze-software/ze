package main

// Guards the wiring that puts the two functional QEMU targets on ze's own
// runtime kernel and stops them skipping the firewall suite.
//
// None of this is reachable from a Go entry point: it is make wiring, a shell
// default and a set of .ci markers. Each one can regress silently, and the
// regression looks exactly like success -- a green run that booted the wrong
// kernel, or that ran zero firewall tests. These tests read the files.
//
// Rule: ai/rules/platform-linux.md, "Both targets boot ze's own runtime
// kernel". Origin: spec-fixit-qemu-runtime-kernel, closed 2026-08-24. That
// spec file is no longer in the tree, so the rule is the durable reference.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	integrationMk = "../../mk/test-integration.mk"
	allTestsSh    = "qemu-all-tests.sh"
	firewallDir   = "../../test/firewall"
)

func readOrFail(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(b) == 0 {
		t.Fatalf("%s is empty; this test must not pass vacuously", path)
	}
	return string(b)
}

// target returns the RECIPE of one make target: the tab-indented lines that
// follow its `name:` line, plus the recipe of every target it delegates to
// through $(MAKE).
//
// Only tab-indented lines count. An earlier version also walked blank and `#`
// lines, which let a comment block BELOW the recipe be read as part of it -- so
// a comment mentioning `--kernel $(ZE_QEMU_KERNEL)` would have kept this file
// green with the flag deleted from the real command. make itself ends a recipe
// at the first line that is not tab-indented, so matching make is both stricter
// and more accurate.
//
// Delegation is followed because a target split into a thin `ze-run.sh` wrapper
// plus an `-impl` body carries neither the flag nor the guard on the wrapper.
// Without following it, every check below reports "not wired" for a tree that
// is correctly wired, which is how these guards went red while the wiring they
// pin was intact.
func target(t *testing.T, mk, name string) string {
	t.Helper()
	body, found := recipeOf(mk, name)
	if !found {
		t.Fatalf("make target %q not found in %s", name, integrationMk)
	}
	if len(body) == 0 {
		t.Fatalf("make target %q has an empty recipe in %s; the parser or the layout changed", name, integrationMk)
	}
	return strings.Join(withDelegated(mk, body, map[string]bool{name: true}), "\n")
}

// recipeOf returns the tab-indented lines that follow a target's `name:` line,
// and reports whether the file declares that target at all. The two are kept
// separate so a caller can tell "no such target" from "declared with an empty
// recipe": they are different failures of this file's assumptions.
func recipeOf(mk, name string) ([]string, bool) {
	lines := strings.Split(mk, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, name+":") {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	var body []string
	for _, l := range lines[start+1:] {
		if !strings.HasPrefix(l, "\t") {
			break
		}
		body = append(body, l)
	}
	return body, true
}

// withDelegated appends the recipe of every target the body invokes through
// $(MAKE). The delegating line stays, because a check may be asking about the
// delegation itself. `seen` stops a cycle and keeps each recipe to one copy.
func withDelegated(mk string, body []string, seen map[string]bool) []string {
	out := append([]string(nil), body...)
	for _, ln := range body {
		for _, name := range makeInvocationTargets(ln) {
			if seen[name] {
				continue
			}
			seen[name] = true
			sub, ok := recipeOf(mk, name)
			if !ok || len(sub) == 0 {
				continue
			}
			out = append(out, withDelegated(mk, sub, seen)...)
		}
	}
	return out
}

// makeInvocationTargets returns the target names one recipe line invokes
// through $(MAKE): the tokens after it that are neither flags nor variable
// assignments.
func makeInvocationTargets(line string) []string {
	_, rest, ok := strings.Cut(line, "$(MAKE)")
	if !ok {
		return nil
	}
	var names []string
	for tok := range strings.FieldsSeq(rest) {
		if strings.HasPrefix(tok, "-") || strings.Contains(tok, "=") {
			continue
		}
		names = append(names, tok)
	}
	return names
}

// prerequisites returns the text after the colon on a target's own line.
func prerequisites(t *testing.T, mk, name string) string {
	t.Helper()
	for l := range strings.SplitSeq(mk, "\n") {
		if rest, ok := strings.CutPrefix(l, name+":"); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatalf("make target %q not found in %s", name, integrationMk)
	return ""
}

// guardUsers returns every make target whose recipe runs ze-qemu-kernel-guard,
// read from the file. A hand-written list would have missed ze-qemu-pppoe-test,
// which adopted the guard without the ze-host-build prerequisite it needs.
func guardUsers(t *testing.T, mk string) []string {
	t.Helper()
	var names []string
	var current string
	for l := range strings.SplitSeq(mk, "\n") {
		if !strings.HasPrefix(l, "\t") && !strings.HasPrefix(l, " ") && strings.Contains(l, ":") &&
			!strings.HasPrefix(l, "#") && !strings.Contains(l, ":=") && !strings.HasPrefix(l, ".") {
			current = strings.TrimSpace(strings.SplitN(l, ":", 2)[0])
			continue
		}
		if current != "" && strings.HasPrefix(l, "\t") && strings.Contains(l, "$(ze-qemu-kernel-guard)") {
			names = append(names, current)
			current = ""
		}
	}
	return names
}

// makeDefine returns the body of a `define <name> ... endef` block.
func makeDefine(t *testing.T, mk, name string) string {
	t.Helper()
	_, after, ok := strings.Cut(mk, "define "+name+"\n")
	if !ok {
		t.Fatalf("make define %q not found in %s", name, integrationMk)
	}
	body, _, ok := strings.Cut(after, "\nendef")
	if !ok {
		t.Fatalf("make define %q is not closed by endef in %s", name, integrationMk)
	}
	return body
}

// TestQemuFunctionalTargetsBootTheRuntimeKernel is the AC-3 guard.
//
// VALIDATES: `make ze-qemu-test-all` and `make ze-qemu-needs-linux-test` both
// pass --kernel to qemu-run.py, so both boot ze's 7.x runtime kernel.
// PREVENTS: a silent return to the stock Alpine 6.12.13-0-virt kernel, which
// ze itself refuses to support (tools/kernel-builder/build.py requires >= 7.0)
// and which is not the kernel ze ships. Dropping one flag leaves the target
// looking like it runs while it judges ze on a kernel no operator gets, which
// is how CONFIG_NF_TABLES_INET, the qdisc set and CONFIG_DUMMY sat missing
// from the appliance config unseen.
//
// It does NOT prevent an nft set-element-timeout crash. That claim was measured
// FALSE on 2026-08-24: stock 6.12.13-0-virt runs the whole firewall suite
// 24/24, firewall-set-element-timeout included.
func TestQemuFunctionalTargetsBootTheRuntimeKernel(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	for _, name := range []string{"ze-qemu-test-all", "ze-qemu-needs-linux-test"} {
		body := target(t, mk, name)
		if !strings.Contains(body, "--kernel $(ZE_QEMU_KERNEL)") {
			t.Errorf("%s does not pass --kernel $(ZE_QEMU_KERNEL) to qemu-run.py; it would boot the stock Alpine kernel", name)
		}
	}
}

// TestQemuTargetsGuardTheStagedKernel is the AC-4 and AC-11 guard.
//
// VALIDATES: both targets run ze-qemu-kernel-guard before the VM starts, and
// that guard exits non-zero on a missing or mismatched kernel rather than
// continuing.
// PREVENTS: a silent fall back to stock when tmp/kernel/vmlinuz is absent
// (AC-4), and the arch trap where GOKRAZY_ARCH defaults to amd64 while
// QEMU_GOARCH follows uname, so a bare `make ze-kernel-build` on an arm64 host
// stages an unbootable amd64 vmlinuz that an existence-only `test -f` accepts
// (AC-11).
func TestQemuTargetsGuardTheStagedKernel(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	for _, name := range []string{"ze-qemu-test-all", "ze-qemu-needs-linux-test"} {
		if !strings.Contains(target(t, mk, name), "$(ze-qemu-kernel-guard)") {
			t.Errorf("%s does not run $(ze-qemu-kernel-guard); a missing or wrong-arch kernel would reach QEMU", name)
		}
	}

	// Every user of the guard needs ze-host-build, because the guard's first
	// command execs the ze-host binary. Derived from the file, so a target that
	// adopts the guard later is covered without editing this test.
	users := guardUsers(t, mk)
	if len(users) < 2 {
		t.Fatalf("found %d users of ze-qemu-kernel-guard (%v); the parser or the layout changed, and this test must not pass vacuously", len(users), users)
	}
	for _, name := range users {
		if !strings.Contains(prerequisites(t, mk, name), "ze-host-build") {
			t.Errorf("%s runs $(ze-qemu-kernel-guard) but does not declare `: ze-host-build`. On a clean checkout the guard execs a binary that is not there, so it denies while reporting the cache branch's message, whose hint does not fix the real cause", name)
		}
	}

	guard := makeDefine(t, mk, "ze-qemu-kernel-guard")
	if !strings.Contains(guard, "exit 1") {
		t.Error("ze-qemu-kernel-guard never exits non-zero; a guard that does not deny does not exist (ai/rules/evidence.md)")
	}
	// The arch must come from QEMU_GOARCH (uname), never from GOKRAZY_ARCH,
	// whose default is amd64 on every host.
	if !strings.Contains(guard, "--arch $(QEMU_GOARCH)") {
		t.Error("ze-qemu-kernel-guard does not key the cache lookup on $(QEMU_GOARCH); it cannot tell an amd64 vmlinuz from an arm64 one")
	}
	if strings.Contains(guard, "GOKRAZY_ARCH") {
		t.Error("ze-qemu-kernel-guard reads GOKRAZY_ARCH, which defaults to amd64 regardless of the host (mk/build-gokrazy.mk)")
	}
	if !strings.Contains(guard, "cmp -s") {
		t.Error("ze-qemu-kernel-guard does not compare the staged kernel against the arch-keyed cache entry; existence alone cannot see the architecture")
	}
}

// TestFirewallNotInDefaultQemuSkips is the AC-2 guard.
//
// VALIDATES: `firewall` is absent from the default skip list in BOTH places
// that carry one.
// PREVENTS: fixing only the makefile default. qemu-all-tests.sh keeps its own
// default for direct invocation, so a half fix leaves the suite skipped for
// anyone who runs the script rather than the target, and the run still reports
// green.
func TestFirewallNotInDefaultQemuSkips(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	if !strings.Contains(mk, "ZE_QEMU_SKIP_SUITES ?= web\n") {
		t.Errorf("%s: expected `ZE_QEMU_SKIP_SUITES ?= web`; firewall must not be skipped by default", integrationMk)
	}

	sh := readOrFail(t, allTestsSh)
	if !strings.Contains(sh, `SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web}"`) {
		t.Errorf("%s: expected the script default to be `web`; firewall must not be skipped by default", allTestsSh)
	}
}

// TestFirewallCiTestsAreNeedsLinux is the AC-10 guard.
//
// VALIDATES: every firewall .ci that boots the daemon carries
// option=needs-linux, and none of them uses option=skip-os:value=darwin.
// PREVENTS: the silent gap this spec exists to close. Under
// ZE_QEMU_LINUX_ONLY=1 the runner skips every test NOT marked needs-linux
// (internal/test/runner/record_parse.go), so a suite marked skip-os reports
// SKIP for all of its tests while ze-qemu-needs-linux-test reports green. That
// is indistinguishable from success and it is how 23 firewall tests ran nowhere
// for months.
func TestFirewallCiTestsAreNeedsLinux(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(firewallDir, "*.ci"))
	if err != nil {
		t.Fatalf("glob %s: %v", firewallDir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .ci found under %s; layout changed. This test must not pass vacuously.", firewallDir)
	}

	daemonTests := 0
	for _, f := range files {
		body := readOrFail(t, f)
		if strings.Contains(body, "option=skip-os:value=darwin") {
			t.Errorf("%s uses option=skip-os:value=darwin. skip-os hides the test from macOS and keeps it OUT of the QEMU needs-linux loop; use option=needs-linux (ai/rules/platform-linux.md)", f)
		}
		// A test that launches the daemon applies real config to a real
		// kernel. A foreground-only test (`ze config validate`, `ze firewall
		// help`) touches no kernel and correctly carries no marker.
		if !strings.Contains(body, "cmd=background:") {
			continue
		}
		daemonTests++
		if !strings.Contains(body, "option=needs-linux") {
			t.Errorf("%s boots the daemon against the kernel firewall but carries no option=needs-linux; it will SKIP in ze-qemu-needs-linux-test", f)
		}
	}
	if daemonTests == 0 {
		t.Fatalf("no daemon-booting firewall test found among %d files; the discriminator broke and this test proves nothing", len(files))
	}
}
