package testqemu

// The appliance hostname proof's seed, readers, forward and verdict.
//
// Goal: pin the decisions this proof makes without a VM: the seed keeps the
// default appliance configuration and adds `system host`, the image's own name
// and the kernel host name are read from parsed documents and never answered
// empty, the web forward lands on the -nic, and a name that never arrived is a
// failure, except an appliance that never answered under software emulation.
// Method: call each function over fixture files, fixture documents and a
// fixture console. No VM is ever started.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: the seed is the default seed followed by `set system host <name>`.
// PREVENTS: a seed that replaced the default (an overlay replaces it, see
// resolveSeedConfig), which boots an appliance with no SSH or web server, or a
// seed whose host line is absent, which would prove nothing about the applier.
func TestTheHostnameSeedKeepsTheDefaultAndSetsTheHost(t *testing.T) {
	base := "set environment ssh enabled true\n"
	seed := seedWithHost(base, hostnameBooted)
	if !strings.HasPrefix(seed, base) {
		t.Errorf("the seed dropped the default appliance seed: %q", seed)
	}
	if !strings.Contains(seed, "\nset system host cpe-lab\n") {
		t.Errorf("the seed lacks the host line: %q", seed)
	}
}

// VALIDATES: the image's own name is the gokrazy instance config's Hostname,
// and a config that names none, or does not parse, is an error.
// PREVENTS: the unset step comparing the kernel with an empty or guessed name.
func TestTheImageHostNameIsReadFromTheGokrazyConfig(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		tree := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tree, "gokrazy", "ze"), 0o750); err != nil {
			t.Fatalf("make the fixture tree: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tree, imageConfigPath), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture config: %v", err)
		}
		return tree
	}

	name, err := imageHostName(write(t, `{"Hostname": "ze", "Packages": []}`))
	if err != nil || name != "ze" {
		t.Errorf("imageHostName = %q, %v; want ze, nil", name, err)
	}
	if _, err := imageHostName(write(t, `{"Packages": []}`)); err == nil {
		t.Error("a config naming no Hostname answered a name")
	}
	if _, err := imageHostName(write(t, `not json`)); err == nil {
		t.Error("a config that does not parse answered a name")
	}
	if _, err := imageHostName(t.TempDir()); err == nil {
		t.Error("a checkout with no gokrazy config answered a name")
	}
}

// VALIDATES: the kernel host name is host.hostname of `show host all | json`,
// and a document that is not JSON, or carries no name, is an error.
// PREVENTS: a CLI error text or an inventory with no host section being read
// as an empty name, which an unset step could mistake for a kernel answer.
func TestTheKernelHostNameIsReadFromTheHostInventory(t *testing.T) {
	name, err := hostnameOf(`{"kernel": {"release": "6.12"}, "host": {"hostname": "cpe-lab", "uptime-seconds": 9}}`)
	if err != nil || name != "cpe-lab" {
		t.Errorf("hostnameOf = %q, %v; want cpe-lab, nil", name, err)
	}
	for _, document := range []string{`error: unknown command`, `{"kernel": {}}`, `{"host": {"hostname": ""}}`} {
		if got, err := hostnameOf(document); err == nil {
			t.Errorf("hostnameOf(%q) answered %q, want an error", document, got)
		}
	}
}

// VALIDATES: the web forward is appended to the -nic value, the caller's argv
// is left unchanged, and an argv with no -nic is an error.
// PREVENTS: a commit step that posts to a port nothing forwards, which reads as
// a refused commit rather than a harness defect.
func TestTheWebForwardLandsOnTheNIC(t *testing.T) {
	argv := []string{"-m", "1024", "-nic", "user,model=e1000,hostfwd=tcp:127.0.0.1:2223-:22", "-nographic"}
	original := slices.Clone(argv)

	got, err := withGuestForward(argv, 2224, webGuestPort)
	if err != nil {
		t.Fatalf("withGuestForward: %v", err)
	}
	want := "user,model=e1000,hostfwd=tcp:127.0.0.1:2223-:22,hostfwd=tcp:127.0.0.1:2224-:8080"
	if got[3] != want {
		t.Errorf("the -nic value is %q, want %q", got[3], want)
	}
	if !slices.Equal(argv, original) {
		t.Errorf("the caller's argv was edited: %q", argv)
	}
	if _, err := withGuestForward([]string{"-m", "1024", "-nic"}, 2224, webGuestPort); err == nil {
		t.Error("an argv whose -nic has no value answered a forward")
	}
}

// VALIDATES: a name that never arrived is a FAILURE carrying the console under
// a hypervisor; under software emulation an appliance that never answered is
// a SKIP, and one that answered the wrong name is still a FAILURE.
// PREVENTS: a broken applier reported as a slow machine, the way the hugepage
// proof once stayed green over an appliance that never answered.
func TestAHostnameMismatchIsASkipOnlyWhenNothingAnsweredUnderEmulation(t *testing.T) {
	console := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(console, []byte("kernel host name not set\n"), 0o600); err != nil {
		t.Fatalf("write the fixture console: %v", err)
	}
	step := func(got string) []HostnameStep {
		return []HostnameStep{{Name: "at boot", Want: hostnameBooted, Got: got}}
	}

	hard := hostnameMismatch(HostnameReport{Accelerator: acceleratorKVM, Steps: step("")}, console, "connection refused")
	if hard.Verdict != VerdictFail {
		t.Errorf("a hardware-accelerated boot that never answered is %v, want fail", hard.Verdict)
	}
	if len(hard.ConsoleTail) == 0 {
		t.Error("the failure carries no serial console")
	}
	if !strings.Contains(hard.Reason, "connection refused") {
		t.Errorf("the failure does not name the last error: %q", hard.Reason)
	}
	if !strings.HasPrefix(hard.Text(), HostnameReportPrefix+"FAIL ") {
		t.Errorf("the failure renders as %q", hard.Text())
	}

	silent := hostnameMismatch(HostnameReport{Accelerator: acceleratorTCG, Steps: step("")}, console, "connection refused")
	if silent.Verdict != VerdictSkip {
		t.Errorf("a software-emulated boot that never answered is %v, want skip", silent.Verdict)
	}
	if !strings.HasPrefix(silent.Text(), HostnameReportPrefix+"SKIP ") {
		t.Errorf("the skip renders as %q", silent.Text())
	}

	wrong := hostnameMismatch(HostnameReport{Accelerator: acceleratorTCG, Steps: step("ze")}, console, "the kernel host name is ze")
	if wrong.Verdict != VerdictFail {
		t.Errorf("a software-emulated boot that answered the wrong name is %v, want fail", wrong.Verdict)
	}
	if !strings.Contains(wrong.Text(), "at boot: want cpe-lab, got ze") {
		t.Errorf("the failure does not show the step: %q", wrong.Text())
	}
}
