package testqemu

// The appliance MPLS boot proof's seed and verdict.
//
// Goal: pin the two decisions this proof makes without a VM: that the seed it
// writes keeps the default appliance configuration and adds the lines that put
// MPLS in use, and that an appliance whose LDP engine never answered is a
// failure under a hypervisor and a skip under software emulation.
// Method: call each function over a fixture checkout and a fixture console. No
// VM is ever started.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: the seed is the checkout's default appliance seed followed by
// `set fib kernel` and `set ldp`.
// PREVENTS: a seed that replaced the default (an overlay replaces it, see
// resolveSeedConfig) and so booted an appliance with no SSH server, or a seed
// without the lines that make MPLS count as in use, which would boot green
// without the kernel capability gate ever judging MPLS.
func TestTheMPLSSeedKeepsTheDefaultAndPutsMPLSInUse(t *testing.T) {
	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "gokrazy", "ze"), 0o750); err != nil {
		t.Fatalf("make the fixture tree: %v", err)
	}
	base := "set environment ssh enabled true\n"
	if err := os.WriteFile(filepath.Join(tree, defaultSeedPath), []byte(base), 0o600); err != nil {
		t.Fatalf("write the fixture seed: %v", err)
	}

	seed, err := (&MPLSBoot{run: &Hugepages{Tree: tree}}).seed()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !strings.HasPrefix(seed, base) {
		t.Errorf("the seed dropped the default appliance seed: %q", seed)
	}
	for _, line := range []string{"set fib kernel", "set ldp"} {
		if !strings.Contains(seed, line+"\n") {
			t.Errorf("the seed lacks %q: %q", line, seed)
		}
	}
}

// VALIDATES: a checkout with no default seed is an error, not an empty seed.
// PREVENTS: an image built with only the MPLS lines, which has no SSH server
// and so can only ever time out.
func TestTheMPLSSeedRefusesAMissingDefault(t *testing.T) {
	if _, err := (&MPLSBoot{run: &Hugepages{Tree: t.TempDir()}}).seed(); err == nil {
		t.Error("a checkout with no gokrazy/ze/ze.conf answered a seed")
	}
}

// VALIDATES: an LDP engine that never answered is a FAILURE carrying the serial
// console under a hypervisor, and a SKIP under software emulation.
// PREVENTS: a daemon the capability gate refused being reported as a slow
// machine, which is the way the hugepage proof once stayed green over an
// appliance that never answered.
func TestAnMPLSNoAnswerIsAFailureOnlyUnderAHypervisor(t *testing.T) {
	console := filepath.Join(t.TempDir(), "console.log")
	if err := os.WriteFile(console, []byte("ze: kernel capability refused\n"), 0o600); err != nil {
		t.Fatalf("write the fixture console: %v", err)
	}

	hard := mplsNoAnswer(MPLSBootReport{Accelerator: acceleratorKVM}, console, "connection refused")
	if hard.Verdict != VerdictFail {
		t.Errorf("a hardware-accelerated boot that never answered is %v, want fail", hard.Verdict)
	}
	if len(hard.ConsoleTail) == 0 {
		t.Error("the failure carries no serial console, which is where a refused start says why")
	}
	if !strings.Contains(hard.Reason, "connection refused") {
		t.Errorf("the failure does not name the last ssh error: %q", hard.Reason)
	}
	if !strings.HasPrefix(hard.Text(), MPLSReportPrefix+"FAIL ") {
		t.Errorf("the failure renders as %q", hard.Text())
	}

	soft := mplsNoAnswer(MPLSBootReport{Accelerator: acceleratorTCG}, console, "connection refused")
	if soft.Verdict != VerdictSkip {
		t.Errorf("a software-emulated boot that never answered is %v, want skip", soft.Verdict)
	}
	if !strings.HasPrefix(soft.Text(), MPLSReportPrefix+"SKIP ") {
		t.Errorf("the skip renders as %q", soft.Text())
	}
}
