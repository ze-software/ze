// Tests for the /dev/ppp step: how its state is read, which root commands an
// install run takes for each absent piece, and what a probe-only run and a host
// with no route to root print.
//
// Every case drives a temporary file in place of the device and the udev rule,
// and a recorder in place of the shell, so no case changes the machine.

package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// pppHost is a temporary device node, a rule path, and the seams the step reads.
type pppHost struct {
	dev  string
	rule string
}

// newPppHost makes a device node with this mode, owned by the test's own user
// and group, and a rule path that holds the rule when withRule is set.
func newPppHost(t *testing.T, mode os.FileMode, withRule bool) pppHost {
	t.Helper()
	dir := t.TempDir()
	host := pppHost{dev: filepath.Join(dir, "ppp"), rule: filepath.Join(dir, "60-ze-ppp.rules")}
	if err := os.WriteFile(host.dev, nil, 0o600); err != nil {
		t.Fatalf("write device: %v", err)
	}
	if err := os.Chmod(host.dev, mode); err != nil {
		t.Fatalf("chmod device: %v", err)
	}
	if withRule {
		if err := os.WriteFile(host.rule, []byte(pppRuleBody), 0o600); err != nil {
			t.Fatalf("write rule: %v", err)
		}
	}
	return host
}

// setup answers a Setup over this host. dipIsOurs makes the dip group the test
// user's own group, so the node counts as granted to it.
func (h pppHost) setup(t *testing.T, rec *recorder, dipIsOurs, member bool) *Setup {
	t.Helper()
	info, err := os.Stat(h.dev)
	if err != nil {
		t.Fatalf("stat device: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat device: no owner ids on this platform")
	}
	gid := int(stat.Gid)
	if !dipIsOurs {
		gid++
	}
	return &Setup{
		GOOS:           "linux",
		PppDev:         h.dev,
		PppRule:        h.rule,
		Shell:          rec.shell(),
		User:           func() string { return "thomas" },
		DipGroupID:     func() (int, bool) { return gid, true },
		DipGroupMember: func() bool { return member },
	}
}

// TestPppTellsApartUsablePendingAndNotGranted pins the three answers.
// VALIDATES: a node this process can open is ok; rule, node and membership all
// in place with an unusable node is a stale login; any absent piece is not
// granted.
// PREVENTS: a host whose session predates the group being told to run the
// install again, or a host missing the rule being told only to log back in.
func TestPppTellsApartUsablePendingAndNotGranted(t *testing.T) {
	usable := newPppHost(t, 0o600, false)
	if got, _, err := usable.setup(t, rootShell(), false, false).pppState(); err != nil || got != pppOK {
		t.Errorf("a node this process opens answered %s (%v), want %s", got, err, pppOK)
	}

	if os.Geteuid() == 0 {
		t.Skip("root opens a node of any mode, so the unusable states are unreachable here")
	}

	// Mode 0060 grants the group and denies the owner, which is this test: the
	// node is granted to dip and still closed to this process.
	granted := newPppHost(t, 0o060, true)
	if got, _, err := granted.setup(t, rootShell(), true, true).pppState(); err != nil || got != pppPendingLogin {
		t.Errorf("a member with a stale session answered %s (%v), want %s", got, err, pppPendingLogin)
	}
	if got, _, err := granted.setup(t, rootShell(), true, false).pppState(); err != nil || got != pppNotGranted {
		t.Errorf("a non-member answered %s (%v), want %s", got, err, pppNotGranted)
	}

	noRule := newPppHost(t, 0o060, false)
	if got, _, err := noRule.setup(t, rootShell(), true, true).pppState(); err != nil || got != pppNotGranted {
		t.Errorf("a member with no rule answered %s (%v), want %s", got, err, pppNotGranted)
	}

	otherGroup := newPppHost(t, 0o060, true)
	if got, _, err := otherGroup.setup(t, rootShell(), false, true).pppState(); err != nil || got != pppNotGranted {
		t.Errorf("a node granted to another group answered %s (%v), want %s", got, err, pppNotGranted)
	}
}

// TestGrantingPppRunsEveryStepForABareHost pins the order and the argv of each
// root command when nothing is in place.
// VALIDATES: the group exists before the node and the user reference it, the
// rule is written and reloaded, the module is loaded before the node it creates
// is changed.
// PREVENTS: a chgrp to a group that does not exist yet, or a node changed now
// and reset by the next module load because no rule names it.
func TestGrantingPppRunsEveryStepForABareHost(t *testing.T) {
	facts := pppFacts{}
	got := stepLines(pppSteps(facts, "/dev/ppp", pppRuleDefault, "thomas"))
	want := []string{
		"groupadd --system dip",
		"tee /etc/udev/rules.d/60-ze-ppp.rules",
		"udevadm control --reload",
		"modprobe ppp_generic",
		"chgrp dip /dev/ppp",
		"chmod 0660 /dev/ppp",
		"usermod -aG dip thomas",
	}
	if !slices.Equal(got, want) {
		t.Errorf("a bare host runs %v, want %v", got, want)
	}

	onlyMember := pppFacts{group: true, rule: true, node: true, nodeGranted: true}
	got = stepLines(pppSteps(onlyMember, "/dev/ppp", pppRuleDefault, "thomas"))
	if !slices.Equal(got, []string{"usermod -aG dip thomas"}) {
		t.Errorf("a host missing only the membership runs %v", got)
	}
}

// stepLines answers each step's argv joined, for comparison.
func stepLines(steps []privilegedStep) []string {
	lines := make([]string, 0, len(steps))
	for _, step := range steps {
		lines = append(lines, strings.Join(step.argv, " "))
	}
	return lines
}

// TestTheRuleIsWrittenWithItsContent pins the stdin of the tee and the line a
// human copies.
// VALIDATES: the rule names dip, mode 0660 and the static node, and the copied
// line writes the same two lines the tee receives.
// PREVENTS: a recorded `tee <path>` with no content, which a human cannot run.
func TestTheRuleIsWrittenWithItsContent(t *testing.T) {
	steps := pppSteps(pppFacts{group: true, node: true}, "/dev/ppp", pppRuleDefault, "thomas")
	if len(steps) == 0 || steps[0].argv[0] != "tee" {
		t.Fatalf("the first step is %v, want the tee", steps)
	}
	if string(steps[0].stdin) != pppRuleBody {
		t.Errorf("the tee receives %q", steps[0].stdin)
	}
	for _, part := range []string{`KERNEL=="ppp"`, `GROUP="dip"`, `MODE="0660"`, `OPTIONS+="static_node=ppp"`} {
		if !strings.Contains(pppRuleBody, part) {
			t.Errorf("the rule lacks %s:\n%s", part, pppRuleBody)
		}
		if !strings.Contains(steps[0].shown, part) {
			t.Errorf("the copied line lacks %s: %s", part, steps[0].shown)
		}
	}
}

// TestAProbeOnlyRunNeverGrantsPpp pins check mode: it reports and runs nothing.
func TestAProbeOnlyRunNeverGrantsPpp(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a node of any mode, so the step is ok here")
	}
	host := newPppHost(t, 0o060, false)
	rec := rootShell()
	rec.euid = 0
	setup := host.setup(t, rec, true, false)
	setup.Check = true

	outcome := setup.visitPpp(&Report{})
	if outcome.State != StateMissing {
		t.Errorf("an ungranted node in check mode is %s, want %s", outcome.State, StateMissing)
	}
	if !strings.Contains(outcome.Detail, "no udev rule") || !strings.Contains(outcome.Detail, "not in the dip group") {
		t.Errorf("the detail does not name what is absent: %q", outcome.Detail)
	}
	if len(rec.calls) != 0 {
		t.Errorf("check mode ran %v", rec.ran())
	}
}

// TestAStaleLoginNamesTheGroupCommand pins the pending answer in check mode.
func TestAStaleLoginNamesTheGroupCommand(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a node of any mode, so the step is ok here")
	}
	host := newPppHost(t, 0o060, true)
	setup := host.setup(t, rootShell(), true, true)
	setup.Check = true

	outcome := setup.visitPpp(&Report{})
	if outcome.State != StatePending {
		t.Errorf("a stale login is %s, want %s", outcome.State, StatePending)
	}
	if !strings.Contains(outcome.Detail, "sg dip -c") {
		t.Errorf("the pending detail names no command: %q", outcome.Detail)
	}
}

// TestPppWithNoRouteToRootPrintsEveryCommand pins the manual fallback.
// VALIDATES: with no password-free sudo and no terminal, the run changes
// nothing and prints each command with its sudo prefix.
// PREVENTS: an agent session reporting pending with no line to give the owner.
func TestPppWithNoRouteToRootPrintsEveryCommand(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a node of any mode, so the step is ok here")
	}
	host := newPppHost(t, 0o060, false)
	rec := rootShell()
	rec.euid = 1000
	// dip is not the node's group, so the node change is owed too.
	setup := host.setup(t, rec, false, false)
	report := &Report{}

	outcome := setup.visitPpp(report)
	if outcome.State != StatePending {
		t.Errorf("an unreachable root is %s, want %s", outcome.State, StatePending)
	}
	text := report.Text()
	for _, line := range []string{
		"Run: printf '%s\\n' ",
		"| sudo tee " + host.rule,
		"Run: sudo udevadm control --reload",
		"Run: sudo chgrp dip " + host.dev,
		"Run: sudo chmod 0660 " + host.dev,
		"Run: sudo usermod -aG dip thomas",
	} {
		if !strings.Contains(text, line) {
			t.Errorf("the fallback lacks %q:\n%s", line, text)
		}
	}
	if len(rec.calls) != 0 {
		t.Errorf("it ran %v with no route to root", rec.ran())
	}
}

// TestPppIsLinuxOnly pins the macOS answer: skipped, with the reason.
func TestPppIsLinuxOnly(t *testing.T) {
	rec := rootShell()
	setup := &Setup{GOOS: "darwin", Shell: rec.shell()}
	outcome := setup.visitPpp(&Report{})
	if outcome.State != StateSkipped {
		t.Errorf("macOS is %s, want %s", outcome.State, StateSkipped)
	}
	if !strings.Contains(outcome.Detail, "Linux only") {
		t.Errorf("the skip gives no reason: %q", outcome.Detail)
	}
	if len(rec.calls) != 0 {
		t.Errorf("macOS ran %v", rec.ran())
	}
}
