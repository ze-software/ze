package main

// Guards the wiring that puts EVERY QEMU target on ze's own runtime kernel and
// stops the functional ones skipping the firewall suite.
//
// None of this is reachable from a Go entry point: it is make wiring, a shell
// default and a set of .ci markers. Each one can regress silently, and the
// regression looks exactly like success -- a green run that booted the wrong
// kernel, or that ran zero firewall tests. These tests read the files.
//
// Three properties travel together. Each is checked SEPARATELY:
//
//   - the --kernel flag,
//   - the guard that proves what it points at,
//   - the ze-host-build prerequisite the guard needs to name its own cause.
//
// A target carrying one but not the others is the shape this file refuses.
// One compound check cannot say which of the three went missing.
//
// Rule: ai/rules/platform-linux.md, "Every QEMU target boots ze's own runtime
// kernel".

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	integrationMk = "../../mk/test-integration.mk"
	allTestsSh    = "qemu-all-tests.sh"
	firewallDir   = "../../test/firewall"
	// The recipe spelling every QEMU target uses to launch a VM. It is the
	// population every check below is derived from.
	qemuRunScript = "python3 scripts/evidence/qemu-run.py"
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

// recipeOf returns the tab-indented lines that follow a target's `name:` line,
// and reports whether the file declares that target at all. The two are kept
// separate so a caller can tell "no such target" from "declared with an empty
// recipe": they are different failures of this file's assumptions.
//
// A make CONDITIONAL DIRECTIVE inside a recipe is not tab-indented, and it does
// not end the recipe. ze-qemu-debug and ze-qemu-shell both wrap their
// cross-compile in `ifneq ($(NOBUILD),1) ... endif`. Everything after the
// `endif` is recipe. That includes the qemu-run.py invocation and `--kernel`.
//
// Stopping at the `ifneq` reported those two targets as unwired while make ran
// the flag. That is a guard that lies in the direction of a false RED. The
// directives are walked over rather than collected: they are parse-time control
// flow, never commands.
//
// Every rule line naming the target is read, not the first. make lets a target
// be declared more than once. It puts the recipe after whichever line carries
// one. ze-qemu-debug declares a target-specific variable on its first line, and
// its prerequisites and recipe on its second. Reading the first line alone
// found an empty recipe, and dropped ze-qemu-debug out of the derived
// population. That is silent, because a target that vanishes is a target
// nothing checks.
func recipeOf(mk, name string) ([]string, bool) {
	lines := strings.Split(mk, "\n")
	declared := false
	var body []string
	for i, l := range lines {
		if !strings.HasPrefix(l, name+":") {
			continue
		}
		declared = true
		for _, r := range lines[i+1:] {
			if isMakeConditional(r) {
				continue
			}
			if !strings.HasPrefix(r, "\t") {
				break
			}
			body = append(body, r)
		}
	}
	if !declared {
		return nil, false
	}
	return body, true
}

// isMakeConditional reports whether a line is one of make's conditional
// directives. Only these, and only at the start of the line. A blank line or a
// comment still ends the recipe. Otherwise a comment block BELOW a recipe reads
// as part of it, and a check stays green with the command it looks for
// deleted.
func isMakeConditional(line string) bool {
	for _, kw := range []string{"ifeq", "ifneq", "ifdef", "ifndef", "else", "endif"} {
		if line == kw || strings.HasPrefix(line, kw+" ") || strings.HasPrefix(line, kw+"(") {
			return true
		}
	}
	return false
}

// prerequisites returns the prerequisites make sees for a target, joined from
// EVERY rule line that names it.
//
// One target CAN be declared on several lines, and make unions what they
// declare. ze-qemu-debug is declared twice. `ze-qemu-debug: export
// ZE_QEMU_DEBUG_RUN = $(RUN)` sets a target-specific variable, and
// `ze-qemu-debug: ze-host-build` carries the prerequisite. Returning the first
// match alone read the export line, and reported the prerequisite missing from
// a target that declares it.
func prerequisites(t *testing.T, mk, name string) string {
	t.Helper()
	var found []string
	for l := range strings.SplitSeq(mk, "\n") {
		if rest, ok := strings.CutPrefix(l, name+":"); ok {
			found = append(found, strings.TrimSpace(rest))
		}
	}
	if len(found) == 0 {
		t.Fatalf("make target %q not found in %s", name, integrationMk)
	}
	return strings.Join(found, " ")
}

// targetsWhoseRecipeContains returns every make target whose own recipe carries
// a substring, in file order and without repeats.
//
// Derived from the file, never listed. A hand-written list missed
// ze-qemu-pppoe-test, which adopted the guard without the ze-host-build
// prerequisite it needs, and a list of the targets that boot the runtime kernel
// would have to be edited by the same person who forgets the flag.
func targetsWhoseRecipeContains(mk, needle string) []string {
	var names []string
	for _, name := range declaredTargets(mk) {
		body, ok := recipeOf(mk, name)
		if !ok {
			continue
		}
		if strings.Contains(strings.Join(body, "\n"), needle) && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// declaredTargets returns every target name the makefile declares, in file
// order. A `define ... endef` body is skipped: its lines are tab-indented and
// its first line can carry a colon, so a body reads as a target with a recipe.
func declaredTargets(mk string) []string {
	var names []string
	inDefine := false
	for l := range strings.SplitSeq(mk, "\n") {
		switch {
		case strings.HasPrefix(l, "define "):
			inDefine = true
			continue
		case strings.HasPrefix(l, "endef"):
			inDefine = false
			continue
		case inDefine:
			continue
		}
		name, ok := targetName(l)
		if ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

// targetName returns the target a rule line declares. It rejects an indented
// line, a comment, a special target (`.PHONY`), and a variable assignment.
//
// The assignment test is positional. A `=` BEFORE the first `:` makes the line
// an assignment whose value carries a colon. `ZE_NETNS_PORT_LOCK_RESTORE = sudo
// chown -R $$(id -u):$$(id -g) ...` is one such line.
// `ze-qemu-debug: export X = $(RUN)` puts the colon first, so it is a rule.
func targetName(line string) (string, bool) {
	if line == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") ||
		strings.HasPrefix(line, "#") || strings.HasPrefix(line, ".") || strings.Contains(line, ":=") {
		return "", false
	}
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return "", false
	}
	if eq := strings.Index(line, "="); eq >= 0 && eq < colon {
		return "", false
	}
	return strings.TrimSpace(line[:colon]), true
}

// qemuRunInvocations returns every qemu-run.py command line in a target's
// recipe, each with its backslash continuations joined into one string.
//
// Per INVOCATION, not per target. The properties under test are properties of
// the command. Take a target with two invocations, one without --kernel. It
// satisfies a `strings.Contains` over the whole recipe. It also boots stock
// Alpine half the time.
func qemuRunInvocations(recipe string) []string {
	joined := strings.ReplaceAll(recipe, "\\\n", " ")
	var out []string
	for l := range strings.SplitSeq(joined, "\n") {
		if strings.Contains(l, qemuRunScript) {
			out = append(out, l)
		}
	}
	return out
}

// qemuRunTargets returns every target that invokes qemu-run.py, and fails when
// the parser attributed fewer invocations than the file holds.
//
// The vacuity check is self-calibrating. It compares what the parser found
// against a raw count of the recipe lines that invoke the script. A parser that
// stops walking a recipe early therefore takes this file red, instead of
// reporting a shrunken population as fully wired.
func qemuRunTargets(t *testing.T, mk string) []string {
	t.Helper()
	targets := targetsWhoseRecipeContains(mk, qemuRunScript)
	if len(targets) == 0 {
		t.Fatalf("no target in %s invokes %s; the parser or the layout changed, and this test must not pass vacuously", integrationMk, qemuRunScript)
	}
	attributed := 0
	for _, name := range targets {
		body, _ := recipeOf(mk, name)
		attributed += len(qemuRunInvocations(strings.Join(body, "\n")))
	}
	raw := 0
	for l := range strings.SplitSeq(mk, "\n") {
		if strings.HasPrefix(l, "\t") && strings.Contains(l, qemuRunScript) {
			raw++
		}
	}
	if attributed != raw {
		t.Fatalf("attributed %d %s invocations to %d targets, but %s holds %d recipe lines invoking it; the parser lost some, or one target now carries two invocations and the per-invocation checks below need to say which", attributed, qemuRunScript, len(targets), integrationMk, raw)
	}
	return targets
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

// TestQemuFunctionalTargetsBootTheRuntimeKernel
//
// VALIDATES: EVERY qemu-run.py invocation in mk/test-integration.mk passes
// --kernel $(ZE_QEMU_KERNEL), so every QEMU target boots ze's 7.x runtime
// kernel.
// PREVENTS: a target judging ze on the stock Alpine 6.12.13-0-virt kernel. ze
// itself refuses to support that release (tools/kernel-builder/build.py wants
// 7.0 or later), and it is not the kernel ze ships. The target looks like it
// runs while its verdict is about a kernel no operator gets. That is how
// CONFIG_NF_TABLES_INET, the qdisc set and CONFIG_DUMMY sat missing from the
// appliance config unseen. Seven of the thirteen invocations were in that state
// until 2026-08-24. ze-qemu-debug was among them: the target whose entire job
// is reproducing a failure, reproducing it on a different kernel.
//
// The population is DERIVED, so a fourteenth target is checked the day it is
// written rather than the day somebody remembers to add it here.
//
// It does NOT prevent an nft set-element-timeout crash. That claim was measured
// FALSE on 2026-08-24: stock 6.12.13-0-virt runs the whole firewall suite
// 24/24, firewall-set-element-timeout included.
func TestQemuFunctionalTargetsBootTheRuntimeKernel(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	for _, name := range qemuRunTargets(t, mk) {
		body, _ := recipeOf(mk, name)
		for _, invocation := range qemuRunInvocations(strings.Join(body, "\n")) {
			if !strings.Contains(invocation, "--kernel $(ZE_QEMU_KERNEL)") {
				t.Errorf("%s invokes qemu-run.py without --kernel $(ZE_QEMU_KERNEL); that VM boots the stock Alpine kernel and its verdict is not about ze", name)
			}
		}
	}
}

// TestQemuTargetsDependOnHostBuild
//
// VALIDATES: every target that invokes qemu-run.py declares `: ze-host-build`.
// PREVENTS: the guard denying while naming the wrong cause. Its first command
// execs $(CURDIR)/ze-host, which a clean checkout does not have. Without the
// prerequisite it reports the CACHE branch's message, and hints at
// `make ze-kernel-build`, which does not fix a missing ze-host.
//
// Separate from the guard check on purpose. The flag, the guard and the
// prerequisite travel together, and each has its own failure. Removing one must
// produce one named red rather than a compound one.
func TestQemuTargetsDependOnHostBuild(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	for _, name := range qemuRunTargets(t, mk) {
		if !strings.Contains(prerequisites(t, mk, name), "ze-host-build") {
			t.Errorf("%s invokes qemu-run.py but does not declare `: ze-host-build`; the kernel guard it runs execs ze-host, so on a clean checkout it denies while naming the wrong cause", name)
		}
	}
}

// TestQemuTargetsGuardTheStagedKernel
//
// VALIDATES: every target that invokes qemu-run.py runs ze-qemu-kernel-guard
// before the VM starts, and that guard exits non-zero on a missing or
// mismatched kernel rather than continuing.
// PREVENTS: a silent fall back to stock when tmp/kernel/vmlinuz is absent, and
// the arch trap where GOKRAZY_ARCH defaults to amd64 while QEMU_GOARCH follows
// uname, so a bare `make ze-kernel-build` on an arm64 host stages an unbootable
// amd64 vmlinuz that an existence-only `test -f` accepts.
//
// The flag alone is not enough, which is why this is a separate property. A
// target passing --kernel at a path nothing staged hands QEMU a file that is
// not there, or one built for another architecture. The failure names
// neither.
func TestQemuTargetsGuardTheStagedKernel(t *testing.T) {
	mk := readOrFail(t, integrationMk)
	for _, name := range qemuRunTargets(t, mk) {
		body, _ := recipeOf(mk, name)
		if !strings.Contains(strings.Join(body, "\n"), "$(ze-qemu-kernel-guard)") {
			t.Errorf("%s invokes qemu-run.py without running $(ze-qemu-kernel-guard); an absent or wrong-arch kernel would reach QEMU and the failure would name neither", name)
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
