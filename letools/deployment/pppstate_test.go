package deployment

// Tests for the kernel questions and the interpretation of their answers.
//
// Goal: pin the four judgements that the on-host proofs make from `ip` output.
// The judgements identify the session interface, verify the negotiated address
// pair, verify that state returned to its initial value, and define an ambiguous
// answer. Method: put a recording `ip` on PATH and read each judgement's answer.
// No namespace is present.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stateStub answers each `ip` question out of an environment variable, so a
// case says what the kernel holds by setting one.
//
// It is a program on PATH rather than an injected seam for the reason
// deployment.go states: the proof IS the argv that reaches ip.
const stateStub = `#!/bin/bash
case "$*" in
  *"ip -o link show type ppp") printf '%s' "$ZE_TEST_LINKS" ; exit ${ZE_TEST_LINKS_EXIT:-0} ;;
  *"ip -o addr show dev "*)    printf '%s' "$ZE_TEST_ADDRS" ; exit ${ZE_TEST_ADDRS_EXIT:-0} ;;
  *"ip l2tp show tunnel")      printf '%s' "$ZE_TEST_TUNNEL" ; exit 0 ;;
  *"ip l2tp show session")     printf '%s' "$ZE_TEST_SESSION" ; exit 0 ;;
esac
exit 0
`

// withStateStub puts the recording ip in front of PATH for one case.
func withStateStub(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(stateStub), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the ip stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The listing the stub answers when one PPP interface exists, and the one it
// answers when two appeared at once.
const (
	oneNewLink  = "3: ppp0: <POINTOPOINT,UP> mtu 1400\n"
	twoNewLinks = "3: ppp0: <POINTOPOINT,UP> mtu 1400\n4: ppp1: <POINTOPOINT,UP> mtu 1400\n"
)

// VALIDATES: the daemon's own log names the interface, and the set difference
// is only the fallback.
// PREVENTS: a proof asserting about an interface something else on the machine
// created. The daemon says which interface it programmed, and that answer has
// evidence behind it where a difference of two listings has none.
func TestTheDaemonsLogNamesTheInterfaceTheSessionCameUpOn(t *testing.T) {
	withStateStub(t)
	t.Setenv("ZE_TEST_LINKS", twoNewLinks)

	lines := []string{"l2tp: PPP session up tunnel-id=1 interface=ppp1"}
	got, err := discoverPPPIface("ns", map[string]bool{}, lines, "Ze")
	if err != nil {
		t.Fatalf("discover the interface: %v", err)
	}
	if got != "ppp1" {
		t.Errorf("the interface is %q, want the one the daemon named (ppp1)", got)
	}
}

// VALIDATES: with no log line naming one, the interface that APPEARED is the
// answer, and two that appeared is refused.
// PREVENTS: a guess the proof then asserts about. The peer's side logs nothing,
// so the fallback is the only answer there -- and an ambiguous fallback picked
// silently would send every later assertion at an interface nobody chose.
func TestAnAmbiguousInterfaceIsRefusedRatherThanGuessed(t *testing.T) {
	withStateStub(t)

	t.Setenv("ZE_TEST_LINKS", oneNewLink)
	got, err := discoverPPPIface("ns", map[string]bool{}, nil, "LAC")
	if err != nil || got != "ppp0" {
		t.Errorf("one new interface answered %q, %v; want ppp0 and no error", got, err)
	}

	t.Setenv("ZE_TEST_LINKS", twoNewLinks)
	if _, err := discoverPPPIface("ns", map[string]bool{}, nil, "LAC"); err == nil ||
		!strings.Contains(err.Error(), "more than one") {
		t.Errorf("two new interfaces answered %v, want a refusal naming both", err)
	}

	t.Setenv("ZE_TEST_LINKS", "")
	if _, err := discoverPPPIface("ns", map[string]bool{}, nil, "LAC"); err == nil ||
		!strings.Contains(err.Error(), "no new pppN interface") {
		t.Errorf("no new interface answered %v, want a refusal", err)
	}

	t.Setenv("ZE_TEST_LINKS", oneNewLink)
	t.Setenv("ZE_TEST_LINKS_EXIT", "1")
	if _, err := discoverPPPIface("ns", map[string]bool{}, nil, "LAC"); err == nil {
		t.Error("a listing that could not be read answered an interface")
	}
}

// VALIDATES: an interface already present before the session is not read as a
// new one.
// PREVENTS: a proof that asserts about the machine's existing PPP interface,
// which on a developer's laptop is a live VPN.
func TestAnInterfaceThatWasAlreadyThereIsNotTheSessions(t *testing.T) {
	withStateStub(t)
	t.Setenv("ZE_TEST_LINKS", twoNewLinks)

	before := map[string]bool{"ppp0": true}
	got, err := discoverPPPIface("ns", before, nil, "LAC")
	if err != nil {
		t.Fatalf("discover the interface: %v", err)
	}
	if got != "ppp1" {
		t.Errorf("the interface is %q, want the one that appeared (ppp1)", got)
	}
}

// VALIDATES: an address is matched as a WHOLE field, bare or with a prefix
// length, and never as a substring of another address.
// PREVENTS: the defect the script carries. This proof's pool runs to
// 10.100.0.10, and its gateway is 10.100.0.1. A substring test reads an
// interface on the wrong end of the pool as the correct interface. As a result,
// the assertion can never come out red on its own.
func TestAnAddressIsMatchedWholeRatherThanAsASubstring(t *testing.T) {
	const listing = "3: ppp0    inet 10.100.0.10 peer 10.100.0.20/32 scope global ppp0"

	if addressPresent(listing, "10.100.0.1") {
		t.Error("10.100.0.1 was found in a listing that only carries 10.100.0.10")
	}
	if addressPresent(listing, "10.100.0.2") {
		t.Error("10.100.0.2 was found in a listing that only carries 10.100.0.20")
	}
	if !addressPresent(listing, "10.100.0.10") {
		t.Error("a bare address the listing carries was not found")
	}
	if !addressPresent(listing, "10.100.0.20") {
		t.Error("an address carrying its prefix length was not found")
	}
}

// VALIDATES: the address assertion needs BOTH ends, and reports the listing it
// judged.
// PREVENTS: a half-finished IPCP passing. An interface with a local address and
// no peer address is what a control plane that programmed an address without
// its peer leaves behind.
func TestTheAddressAssertionNeedsBothEnds(t *testing.T) {
	withStateStub(t)

	t.Setenv("ZE_TEST_ADDRS", "3: ppp0    inet 10.100.0.1 peer 10.100.0.2/32 scope global ppp0")
	if err := verifyPPPAddress("ns", "ppp0", "10.100.0.1", "10.100.0.2"); err != nil {
		t.Errorf("a negotiated pair was refused: %v", err)
	}

	t.Setenv("ZE_TEST_ADDRS", "3: ppp0    inet 10.100.0.1 scope global ppp0")
	err := verifyPPPAddress("ns", "ppp0", "10.100.0.1", "10.100.0.2")
	if err == nil {
		t.Fatal("an interface with no peer address was accepted")
	}
	if !strings.Contains(err.Error(), "10.100.0.2") || !strings.Contains(err.Error(), "ppp0") {
		t.Errorf("the refusal names neither the address nor the interface: %v", err)
	}

	t.Setenv("ZE_TEST_ADDRS_EXIT", "1")
	if err := verifyPPPAddress("ns", "ppp0", "10.100.0.1", "10.100.0.2"); err == nil {
		t.Error("a listing that could not be read was accepted")
	}
}

// VALIDATES: the teardown assertion needs the session's interface gone, the
// interface set back where it was, AND the L2TP state back where it was.
// PREVENTS: the leak this assertion exists for. A tunnel left behind after its
// session went away is invisible to a check that only asks about the interface.
func TestTheTeardownAssertionReadsBothTheInterfacesAndTheTunnels(t *testing.T) {
	withStateStub(t)
	t.Setenv("ZE_TEST_LINKS", "")
	t.Setenv("ZE_TEST_TUNNEL", "")
	t.Setenv("ZE_TEST_SESSION", "")

	baselines, err := readPPPBaselines([]string{"ns"})
	if err != nil {
		t.Fatalf("read the baseline: %v", err)
	}
	baselines[0].iface = "ppp0"

	if err := awaitTeardown(baselines, 100*time.Millisecond); err != nil {
		t.Errorf("a namespace back where it started was refused: %v", err)
	}

	t.Setenv("ZE_TEST_LINKS", oneNewLink)
	if err := awaitTeardown(baselines, 100*time.Millisecond); err == nil {
		t.Error("a namespace still holding the session's interface was accepted")
	}

	t.Setenv("ZE_TEST_LINKS", "")
	t.Setenv("ZE_TEST_TUNNEL", "Tunnel 1, encap UDP\n")
	err = awaitTeardown(baselines, 100*time.Millisecond)
	if err == nil {
		t.Fatal("a namespace still holding an L2TP tunnel was accepted")
	}
	if !strings.Contains(err.Error(), "l2tp-changed=true") {
		t.Errorf("the refusal does not say the L2TP state moved: %v", err)
	}
}
