package rfc

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// VALIDATES: a tagged Go unit is placed where its own build constraints hold: on the
// host when the host run compiles it, in the Linux guest when only the guest does, and
// nowhere, as an error, when neither does.
// METHOD: fixture files carrying each constraint shape the tree uses, asked of a
// darwin host context and the guest context placementContexts builds.
// PREVENTS: the gap journal row 155 records -- an `integration && linux` unit run by host
// `go test`, which never compiles it, so no break could ever be observed on it -- and the
// opposite error, a unit that compiles nowhere being reported as green or as red.
func TestUnitPlacementFollowsBuildConstraints(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"plain_test.go":        "package widget\n",
		"integration_test.go":  "//go:build integration && linux\n\npackage widget\n",
		"linuxonly_test.go":    "//go:build linux\n\npackage widget\n",
		"widget_linux_test.go": "package widget\n",
		"feature_test.go":      "//go:build ze_widget\n\npackage widget\n",
		"windows_test.go":      "//go:build windows\n\npackage widget\n",
		"notlinux_test.go":     "//go:build !linux\n\npackage widget\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	host, guest := placementContexts([]string{"ze_widget"})
	// A darwin host, whatever this test runs on, so the linux cases are asked of
	// the guest on every machine.
	host.GOOS = "darwin"

	cases := []struct {
		file  string
		guest bool
		fails bool
	}{
		{file: "plain_test.go"},
		{file: "feature_test.go"},
		{file: "notlinux_test.go"},
		{file: "integration_test.go", guest: true},
		{file: "linuxonly_test.go", guest: true},
		{file: "widget_linux_test.go", guest: true},
		{file: "windows_test.go", fails: true},
	}
	for _, c := range cases {
		inGuest, err := unitNeedsGuest(&host, &guest, filepath.Join(directory, c.file), false)
		if c.fails {
			if err == nil {
				t.Errorf("%s: placed (guest=%v), want the refusal for a file nothing compiles", c.file, inGuest)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		if inGuest != c.guest {
			t.Errorf("%s: guest=%v, want %v", c.file, inGuest, c.guest)
		}
	}
	if !slices.Contains(guest.BuildTags, testqemu.IntegrationTag) {
		t.Errorf("guest tags %v lack %q", guest.BuildTags, testqemu.IntegrationTag)
	}
	if guest.GOOS != "linux" {
		t.Errorf("guest GOOS %q, want linux", guest.GOOS)
	}
}

// VALIDATES: a guest observation demands Ze's runtime kernel, and a host observation
// refuses one.
// METHOD: requireGuestKernel over the four combinations.
// PREVENTS: a guest proof recorded over the Alpine ISO's own kernel, which
// ai/rules/platform-linux.md forbids, and a kernel argument silently read by nothing.
func TestRequireGuestKernel(t *testing.T) {
	const unit = "internal/widget/widget_linux_test.go::TestWidget"
	if err := requireGuestKernel(unit, true, ""); err == nil || !strings.Contains(err.Error(), keyKernel) {
		t.Errorf("guest with no kernel: %v, want a refusal naming %q", err, keyKernel)
	}
	if err := requireGuestKernel(unit, true, "tmp/kernel/build/vmlinuz"); err != nil {
		t.Errorf("guest with a kernel: %v", err)
	}
	if err := requireGuestKernel(unit, false, "tmp/kernel/build/vmlinuz"); err == nil {
		t.Error("host unit with a kernel: accepted, want a refusal")
	}
	if err := requireGuestKernel(unit, false, ""); err != nil {
		t.Errorf("host unit with no kernel: %v", err)
	}
}

// VALIDATES: a guest observation compiles the unit's package for the guest on the host,
// under the integration tag and the overlay, and runs the ONE unit inside the guest on
// the named kernel, verbose and with the coverage profile written through the checkout.
// METHOD: the three commands a guest run is made of, over a fixture runner.
// PREVENTS: a guest run that selects every test in the package (no attribution), that
// drops the overlay (a green that never saw the break), or whose profile the host cannot
// read (a reachability judgement over nothing).
func TestGuestObservationCommands(t *testing.T) {
	tree := t.TempDir()
	runner := &observationRunner{
		tree: tree, toolchain: gotoolchain.Toolchain{Timeout: "20m"},
		carrier: Carrier{Kind: kindUnit}, tag: Tag{File: "internal/widget/widget_linux_test.go"},
		record:   DiscriminationRecord{Producer: "internal/widget/widget.go::SendWidget"},
		unitName: "TestWidget", self: "/bin/le", kernel: "tmp/kernel/build/vmlinuz", guest: true,
	}
	scratch := filepath.Join(tree, "tmp", "s")
	binary := filepath.Join(scratch, "guest-1.test")
	profile := filepath.Join(scratch, "cover-1.out")

	build := runner.guestBuildArgv(binary, filepath.Join(scratch, "overlay.json"), profile)
	joined := strings.Join(build, " ")
	for _, want := range []string{" test ", " -c -o " + binary, " -overlay " + filepath.Join(scratch, "overlay.json"),
		" -cover -coverpkg ./internal/widget"} {
		if !strings.Contains(joined, want) {
			t.Errorf("build argv %q lacks %q", joined, want)
		}
	}
	if tags := build[slices.Index(build, "-tags")+1]; !slices.Contains(strings.Fields(tags), testqemu.IntegrationTag) {
		t.Errorf("build tags %q lack %q", tags, testqemu.IntegrationTag)
	}
	if build[len(build)-1] != "./internal/widget" {
		t.Errorf("build argv ends %q, want the unit's package", build[len(build)-1])
	}

	script := runner.guestScript(binary, profile)
	// The binary runs from a guest-local copy: a unit that re-executes itself in a
	// new user namespace cannot exec a path on the 9p checkout, whose owner that
	// namespace does not map (guestScript). A coverage run names a guest-local
	// counter directory with -test.gocoverdir=, so a child the unit re-executes
	// writes its counters where the parent's profile is built from.
	want := "cp '" + filepath.ToSlash(binary) + "' '/tmp/guest-1.test' && mkdir -p '/tmp/cover-guest-1.test'" +
		" && cd '" + filepath.ToSlash(filepath.Join(tree, "internal/widget")) + "'" +
		" && exec '/tmp/guest-1.test' -test.run '^TestWidget$'" +
		" -test.count=1 -test.v -test.timeout 20m -test.coverprofile '" + filepath.ToSlash(profile) + "'" +
		" -test.gocoverdir='/tmp/cover-guest-1.test'\n"
	if script != want {
		t.Errorf("guest script\n got %q\nwant %q", script, want)
	}

	run := runner.guestRunArgv(filepath.Join(scratch, "guest-1.sh"))
	wantRun := []string{"/bin/le", "test", "qemu", "docker-lab", "kernel", "tmp/kernel/build/vmlinuz",
		"command", "sh '" + filepath.ToSlash(filepath.Join(scratch, "guest-1.sh")) + "'"}
	if !slices.Equal(run, wantRun) {
		t.Errorf("guest run argv\n got %q\nwant %q", run, wantRun)
	}
}

// VALIDATES: a .ci carrier runs in the guest exactly when the functional runner says
// this host skips it, and an ExaBGP compatibility file never asks.
// METHOD: functionalNeedsGuest over a stub of testrunner.NeedsGuest answering each way.
// PREVENTS: the gap of the 2026-09-30 journal row -- a caps-gated .ci observed through a
// host run that skipped it -- and a guest boot for a file the host runs.
func TestFunctionalPlacementIsTheRunnersAnswer(t *testing.T) {
	plugin := Carrier{Kind: kindFunctional, Name: "functional-plugin"}
	for _, skipped := range []bool{true, false} {
		guest, err := functionalNeedsGuest(plugin, "/tree/test/plugin/a.ci",
			func(string) (bool, error) { return skipped, nil })
		if err != nil {
			t.Fatalf("skipped=%v: %v", skipped, err)
		}
		if guest != skipped {
			t.Errorf("host skips it=%v: guest=%v, want %v", skipped, guest, skipped)
		}
	}
	if _, err := functionalNeedsGuest(plugin, "/tree/test/plugin/a.ci",
		func(string) (bool, error) { return false, os.ErrNotExist }); err == nil {
		t.Error("an unreadable .ci was placed, want a refusal")
	}
	asked := false
	guest, err := functionalNeedsGuest(Carrier{Kind: kindFunctional, Name: exabgpCarrierName},
		"/tree/test/exabgp-compat/x.ci", func(string) (bool, error) { asked = true; return true, nil })
	if err != nil || guest || asked {
		t.Errorf("exabgp carrier: guest=%v err=%v asked=%v, want the host and no question", guest, err, asked)
	}
}

// VALIDATES: a guest .ci observation boots Ze's own system on the named runtime kernel
// under the harness, and runs the ONE .ci against this run's own cross-built daemons.
// METHOD: guestCIArgv over a fixture runner.
// PREVENTS: a guest proof over the Alpine kernel, a run of the shared guest daemons
// rather than the ones built under the overlay, and a whole-suite run with no attribution.
func TestGuestCIObservationCommand(t *testing.T) {
	runner := &observationRunner{
		tree: t.TempDir(), carrier: Carrier{Kind: kindFunctional, Name: "functional-plugin"},
		tag: Tag{File: "test/plugin/gated.ci"}, self: "/bin/le",
		kernel: "tmp/kernel/build/vmlinuz", guest: true,
	}
	binaries := "tmp/qemu/rfc-discriminate/pid-1/linux-amd64/bin"
	got := runner.guestCIArgv(binaries, testqemu.ArchAMD64)
	want := []string{"/bin/le", "test", "qemu", "docker-lab", "kernel", "tmp/kernel/build/vmlinuz",
		testqemu.KeywordHarness, "timeout", guestCICommandTimeout,
		"command", testqemu.OneCICommand(binaries, "test/plugin/gated.ci", testqemu.ArchAMD64)}
	if !slices.Equal(got, want) {
		t.Errorf("guest .ci argv\n got %q\nwant %q", got, want)
	}
}

// VALIDATES: a .ci observation is judged by the runner's result line for that .ci: the
// clean run must show PASS, a run under the break must show FAIL or TIME, and a SKIP or a
// missing line is refused as unobserved, whatever the exit code said.
// METHOD: the clean-run check, judgeRed and attribution over runner output fixtures, for
// an ordinary suite and for the ExaBGP carrier that prints no such line.
// PREVENTS: the 2026-09-30 journal row: a skipped .ci (exit 0) read as a clean green, and
// its skip under the break reported as "stayed GREEN" rather than as nothing observed.
func TestCIResultLineDecidesWhatWasObserved(t *testing.T) {
	const stem = "gated"
	line := func(outcome string) string {
		return "  1.2s  1/1  " + outcome + "  A  " + stem + "\n"
	}
	runner := &observationRunner{carrier: Carrier{Kind: kindFunctional, Name: "functional-plugin"},
		tag: Tag{File: "test/plugin/gated.ci"}, names: stem,
		record: DiscriminationRecord{Unit: "test/plugin/gated.ci"}}

	if why := runner.cleanRunUnobserved(line(ciPass)); why != "" {
		t.Errorf("clean PASS refused: %s", why)
	}
	for _, output := range []string{line(ciSkip), "pass 0/0\n"} {
		if why := runner.cleanRunUnobserved(output); why == "" {
			t.Errorf("clean run %q accepted as green", output)
		}
	}
	for _, outcome := range []string{ciFail, ciTimeout} {
		if err := runner.judgeRed(false, line(outcome)); err != nil {
			t.Errorf("red %s refused: %v", outcome, err)
		}
	}
	err := runner.judgeRed(true, line(ciSkip))
	if err == nil || !strings.Contains(err.Error(), "did not run under the break") {
		t.Errorf("skip under the break: %v, want the unobserved refusal", err)
	}
	if err := runner.judgeRed(false, "boot failed\n"); err == nil {
		t.Error("a failing run with no result line was accepted as the red")
	}
	if err := runner.judgeRed(false, line(ciPass)); err == nil {
		t.Error("a failing exit over a PASS line was accepted as the red")
	}

	exabgp := *runner
	exabgp.carrier = Carrier{Kind: kindFunctional, Name: exabgpCarrierName}
	if why := exabgp.cleanRunUnobserved("compat report\n"); why != "" {
		t.Errorf("exabgp clean run judged by a line it never prints: %s", why)
	}
	if err := exabgp.judgeRed(false, "case "+stem+" failed\n"); err != nil {
		t.Errorf("exabgp red naming the file refused: %v", err)
	}
}

// VALIDATES: a unit's own doc comment decides whether its guest run is elevated: the
// guestRootDirective line selects root, its absence keeps the ordinary owner, and a
// function the file does not declare is refused rather than answered unprivileged.
// METHOD: unitDeclaresGuestRoot over a fixture file, then guestRunArgv for each answer.
// PREVENTS: a root-gated unit (a cgroup BPF attach) run unprivileged in the guest, where
// it skips and the record reads the skip as "stayed GREEN under the break".
func TestGuestRootIsDeclaredByTheUnit(t *testing.T) {
	file := filepath.Join(t.TempDir(), "guard_linux_test.go")
	source := "package widget\n\nimport \"testing\"\n\n" +
		"// TestGuardDenied attaches a cgroup program.\n//\n//ze:guest-root\nfunc TestGuardDenied(t *testing.T) {}\n\n" +
		"// TestGuardUnprivileged means something only without root.\nfunc TestGuardUnprivileged(t *testing.T) {}\n\n" +
		"type fixture struct{}\n\n//ze:guest-root\nfunc (fixture) TestGuardMethod(t *testing.T) {}\n"
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	for unit, want := range map[string]bool{"TestGuardDenied": true, "TestGuardUnprivileged": false} {
		got, err := unitDeclaresGuestRoot(file, unit)
		if err != nil || got != want {
			t.Errorf("%s: root %v, %v; want %v", unit, got, err, want)
		}
	}
	// A method of the same name is not the test function: the go command runs no method.
	if _, err := unitDeclaresGuestRoot(file, "TestGuardMethod"); err == nil {
		t.Error("a method was read as the test function")
	}
	if _, err := unitDeclaresGuestRoot(file, "TestAbsent"); err == nil {
		t.Error("a function the file does not declare was answered")
	}

	runner := &observationRunner{self: "/bin/le", kernel: "vmlinuz", guest: true, root: true}
	wantRun := []string{"/bin/le", "test", "qemu", "docker-lab", "kernel", "vmlinuz",
		testqemu.KeywordRoot, "command", "sh 'guest-1.sh'"}
	if run := runner.guestRunArgv("guest-1.sh"); !slices.Equal(run, wantRun) {
		t.Errorf("root guest run argv\n got %q\nwant %q", run, wantRun)
	}
	runner.root = false
	if run := runner.guestRunArgv("guest-1.sh"); slices.Contains(run, testqemu.KeywordRoot) {
		t.Errorf("an undeclared unit was elevated: %q", run)
	}
}

// VALIDATES: a root-declared unit placed on the host is refused before any run, with
// the kernel fix, and a guest placement or an undeclared unit passes.
// PREVENTS: an unprivileged host run of a root-only unit, which can only skip.
func TestRequireRootInGuest(t *testing.T) {
	if err := requireRootInGuest("u", false, true); err == nil ||
		!strings.Contains(err.Error(), guestRootDirective) || !strings.Contains(err.Error(), keyKernel) {
		t.Errorf("root on the host answered %v, want the refusal naming the directive and kernel", err)
	}
	if err := requireRootInGuest("u", true, true); err != nil {
		t.Errorf("root in the guest refused: %v", err)
	}
	if err := requireRootInGuest("u", false, false); err != nil {
		t.Errorf("an undeclared host unit refused: %v", err)
	}
}

// VALIDATES: a Go unit that skips observes nothing in either run: the clean run is no
// green and a skip under the break is no "stayed GREEN", and the refusal carries the
// skip reason. A skipped subtest counts, a sibling's skip does not, and a pass is a pass.
// METHOD: cleanRunUnobserved and judgeRed over `go test -v` output shapes.
// PREVENTS: the 2026-11-07 refusal of RFC4301-4.4.1-8, where a root-gated unit run
// unprivileged in the guest skipped twice and was reported as not discriminating.
func TestGoUnitSkipObservesNothing(t *testing.T) {
	const reason = "requires privileged QEMU: CAP_BPF"
	skipped := "=== RUN   TestGuard\n    guard_linux_test.go:265: " + reason + "\n--- SKIP: TestGuard (0.00s)\nPASS\n"
	runner := &observationRunner{carrier: Carrier{Kind: kindUnit}, unitName: "TestGuard", guest: true,
		record: DiscriminationRecord{Unit: "internal/widget/guard_linux_test.go::TestGuard",
			Break: "return nil", Producer: "internal/widget/guard.go::Attach"}}

	why := runner.cleanRunUnobserved(skipped)
	if !strings.Contains(why, "the unit skipped: guard_linux_test.go:265: "+reason) {
		t.Errorf("clean skip answered %q, want the skip refusal with its reason", why)
	}
	if !strings.Contains(why, guestRootDirective) {
		t.Errorf("guest skip refusal %q does not name the root declaration", why)
	}
	err := runner.judgeRed(true, skipped)
	if err == nil || !strings.Contains(err.Error(), "the unit skipped") || strings.Contains(err.Error(), "stayed GREEN") {
		t.Errorf("skip under the break answered %v, want the skip refusal, never stayed GREEN", err)
	}
	subtest := "=== RUN   TestGuard\n=== RUN   TestGuard/v6\n        guard_linux_test.go:9: no IPv6\n" +
		"    --- SKIP: TestGuard/v6 (0.00s)\n--- PASS: TestGuard (0.01s)\nPASS\n"
	if why := runner.cleanRunUnobserved(subtest); !strings.Contains(why, "the unit skipped: guard_linux_test.go:9: no IPv6") {
		t.Errorf("subtest skip answered %q, want the skip refusal", why)
	}
	for _, output := range []string{
		"=== RUN   TestGuard\n--- PASS: TestGuard (0.01s)\nPASS\n",
		"=== RUN   TestGuardSibling\n--- SKIP: TestGuardSibling (0.00s)\n=== RUN   TestGuard\n--- PASS: TestGuard (0.01s)\n",
	} {
		if why := runner.cleanRunUnobserved(output); why != "" {
			t.Errorf("run %q refused: %s", output, why)
		}
	}
	if got := skipReason("=== RUN   TestGuard\n"); got != "it printed no reason" {
		t.Errorf("a skip with no log line answered %q", got)
	}
}
