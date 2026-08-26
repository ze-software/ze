package deployment

// Tests for the on-host L2TP PPP proof through direct function calls.
//
// Goal: pin the run behavior that
// scripts/evidence/l2tp_ppp_parity_test.go cannot reach. This test covers both
// daemon configurations, the ze startup environment, and a kernel listing. It
// distinguishes a proof that the test was unable to perform from a failed
// proof. Method: call each piece and read its answer. No namespace or daemon is
// present.

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// fixtureL2TPPPP answers a run over a checkout carrying the one file the build
// tags derive from.
func fixtureL2TPPPP(t *testing.T) *L2TPPPP {
	t.Helper()

	tree := t.TempDir()
	manifest := "ze_bgp internal/component/bgp\nze_l2tp internal/component/l2tp\n"
	if err := os.WriteFile(filepath.Join(tree, "feature-gates.txt"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write the fixture manifest: %v", err)
	}

	run := NewL2TPPPP(tree)
	run.Progress = io.Discard
	return run
}

// VALIDATES: both spellings of ze's kernel-probe escape are refused before
// anything is started.
// PREVENTS: the one answer this proof must never give. The escape makes ze skip
// the kernel path the proof exists to exercise, so a run that honored it would
// report a pass over a user-space session.
func TestTheProofRefusesEitherKernelProbeEscape(t *testing.T) {
	for _, key := range []string{SkipKernelProbeEnv, SkipKernelProbeKey} {
		t.Setenv(key, "true")
		err := refuseSkipKernelProbe()
		if err == nil {
			t.Fatalf("a run with %s set was allowed to start", key)
		}
		if !strings.Contains(err.Error(), key) {
			t.Errorf("the refusal does not name %s: %v", key, err)
		}
		os.Unsetenv(key) //nolint:errcheck // t.Setenv restores the outer value at cleanup
	}

	if err := refuseSkipKernelProbe(); err != nil {
		t.Errorf("a run with neither variable set was refused: %v", err)
	}
}

// VALIDATES: the daemon's environment carries the six settings the proof needs
// and neither spelling of the escape, whatever this process inherited.
// PREVENTS: the refusal above being the whole guarantee. A run started from a
// shell that exports the escape would otherwise hand it to the daemon through
// os.Environ, which no check in front of the run can see.
func TestTheDaemonEnvironmentStripsTheEscapeAndCarriesTheSettings(t *testing.T) {
	t.Setenv(SkipKernelProbeEnv, "true")
	t.Setenv(SkipKernelProbeKey, "true")

	run := fixtureL2TPPPP(t)
	work := t.TempDir()
	entries := run.daemonEnv(work)

	for _, entry := range entries {
		key, _, _ := strings.Cut(entry, "=")
		if key == SkipKernelProbeEnv || key == SkipKernelProbeKey {
			t.Errorf("the daemon would have inherited %s", key)
		}
	}

	wanted := []string{
		"ZE_LOG_L2TP=debug",
		"ZE_STORAGE_BLOB=false",
		"ZE_CONFIG_DIR=" + filepath.Join(work, "ze"),
		"ze.l2tp.ncp.enable-ipv6cp=false",
		"ze.l2tp.ncp.ip-timeout=15s",
		"ze.l2tp.auth.timeout=15s",
	}
	for _, want := range wanted {
		if !slices.Contains(entries, want) {
			t.Errorf("the daemon environment lacks %q", want)
		}
	}
}

// VALIDATES: the peer's configuration dials the address and port ze binds, out
// of the run's own scratch directory.
// PREVENTS: a peer pointed at a port nothing listens on, which reads as an L2TP
// failure and is a configuration error.
func TestThePeerConfigurationDialsTheAddressZeBinds(t *testing.T) {
	run := fixtureL2TPPPP(t)
	work := t.TempDir()

	config := run.peerConfig(work)
	for _, want := range []string{
		"port = " + L2TPPPPPeerPort,
		"lns = " + L2TPPPPZeIP,
		"auth file = " + filepath.Join(work, "l2tp-secrets"),
		"pppoptfile = " + filepath.Join(work, "ppp-options"),
		"require authentication = no",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("the peer configuration lacks %q:\n%s", want, config)
		}
	}

	daemon := run.daemonConfig()
	for _, want := range []string{
		"gateway " + L2TPPPPLocalAddr + ";",
		"start " + L2TPPPPPeerAddr + ";",
		"end " + L2TPPPPPoolEnd + ";",
		"ip " + L2TPPPPZeIP + ";",
		"port " + L2TPPPPListenPort + ";",
	} {
		if !strings.Contains(daemon, want) {
			t.Errorf("the ze configuration lacks %q:\n%s", want, daemon)
		}
	}
}

// VALIDATES: the four input files are written, and the secrets file is written
// narrowly enough for xl2tpd to accept it.
// PREVENTS: a run that dies at the peer with "secrets file is world readable",
// which reads as a peer failure and is a permission.
func TestTheRunWritesFourInputsAndNarrowsTheSecrets(t *testing.T) {
	run := fixtureL2TPPPP(t)
	work := t.TempDir()

	if err := run.writeInputs(work); err != nil {
		t.Fatalf("write the inputs: %v", err)
	}
	for _, name := range []string{"xl2tpd.conf", "l2tp-secrets", "ppp-options", "ze.conf"} {
		if _, err := os.Stat(filepath.Join(work, name)); err != nil {
			t.Errorf("the run did not write %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(work, "ze")); err != nil {
		t.Errorf("the run did not make ze's configuration directory: %v", err)
	}

	info, err := os.Stat(filepath.Join(work, "l2tp-secrets"))
	if err != nil {
		t.Fatalf("stat the secrets: %v", err)
	}
	// The mode is SPELLED here instead of compared with the writer's constant.
	// That constant would not provide an independent assertion. If the constant
	// widens, both sides widen. The case stays green for a secrets file that
	// anybody on the machine can read.
	const ownerOnly os.FileMode = 0o600
	if info.Mode().Perm() != ownerOnly {
		t.Errorf("the secrets file is mode %v, want %v", info.Mode().Perm(), ownerOnly)
	}
}

// VALIDATES: an `ip -o link show` line answers the interface it names, and a
// line that names none answers nothing.
// PREVENTS: a paired link's peer suffix being read as part of the name, which
// would leave the teardown assertion comparing names that never match.
func TestALinkListingLineAnswersItsInterfaceName(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"3: ppp0: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1500", "ppp0"},
		{"7: ppp1@if5: <POINTOPOINT> mtu 1400", "ppp1"},
		{"1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536", "lo"},
		{"", ""},
		{"Cannot open network namespace: No such file or directory", ""},
		{"ppp0: no index in front of it", ""},
	}
	for _, one := range cases {
		if got := linkName(one.line); got != one.want {
			t.Errorf("linkName(%q) = %q, want %q", one.line, got, one.want)
		}
	}
}

// VALIDATES: an `interface=` field is read out of a log line, quoted or not.
// PREVENTS: the discovery reading a quoted value with its quotes, which never
// matches a kernel interface name and sends every run down the set-difference
// fallback.
func TestALogLineAnswersTheInterfaceItNames(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{`time=... msg="l2tp: PPP session up" tunnel-id=1 interface=ppp0`, "ppp0"},
		{`interface="ppp3" session-id=2`, "ppp3"},
		{"interface=ppp7", "ppp7"},
		{"no field here", ""},
	}
	for _, one := range cases {
		if got := interfaceField(one.line); got != one.want {
			t.Errorf("interfaceField(%q) = %q, want %q", one.line, got, one.want)
		}
	}
}

// VALIDATES: a report renders as structured data with kebab-case keys, so the
// pipe operators have something to render.
// PREVENTS: a payload nobody can pipe, which is the CLI contract this port
// inherits (ai/rules/cli.md).
func TestTheL2TPPPPReportIsStructuredData(t *testing.T) {
	report := L2TPPPPReport{
		Peer: PeerName, ZeNamespace: "ze-ns", LACNamespace: "lac-ns",
		ZeInterface: "ppp0", LACInterface: "ppp1",
		LocalAddress: L2TPPPPLocalAddr, PeerAddress: L2TPPPPPeerAddr, Proven: true,
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("render the report: %v", err)
	}
	for _, key := range []string{
		`"ze-namespace"`, `"lac-namespace"`, `"ze-interface"`, `"lac-interface"`,
		`"local-address"`, `"peer-address"`, `"proven"`, `"log-tail"`,
	} {
		if !strings.Contains(string(body), key) {
			t.Errorf("the rendered report lacks %s: %s", key, body)
		}
	}
}

// VALIDATES: the text rendering names both interfaces on a pass and the reason
// on a failure.
// PREVENTS: a person who typed no pipe operator reading "FAIL" with nothing
// behind it.
func TestTheL2TPPPPReportTextSaysWhatHappened(t *testing.T) {
	pass := L2TPPPPReport{Peer: PeerName, ZeInterface: "ppp0", LACInterface: "ppp1", Proven: true}.Text()
	for _, want := range []string{"OK:", PeerName, "ppp0", "ppp1", "clean teardown"} {
		if !strings.Contains(pass, want) {
			t.Errorf("the pass line lacks %q: %s", want, pass)
		}
	}

	fail := L2TPPPPReport{Reason: "ncp: timeout", LogTail: []string{"ze> one", "ze> two"}}.Text()
	for _, want := range []string{"FAIL: ncp: timeout", "ze log tail:", "ze> two"} {
		if !strings.Contains(fail, want) {
			t.Errorf("the failure lines lack %q: %s", want, fail)
		}
	}
}

// VALIDATES: an override naming a path that is not there, or one that cannot be
// executed, is refused before anything is built.
// PREVENTS: a run that gets as far as starting a daemon inside a namespace and
// fails there, where the reason is much harder to read than it is here.
func TestTheProofRefusesAnUnusableBinaryOverride(t *testing.T) {
	tree := t.TempDir()

	t.Setenv("ZE_EVIDENCE_ZE_BINARY", filepath.Join(tree, "absent"))
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if _, err := hostDaemon(tree, daemonBinaryName, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "does not exist") {
		t.Errorf("an override naming nothing answered %v, want a refusal naming the path", err)
	}

	unreadable := filepath.Join(tree, "not-executable")
	if err := os.WriteFile(unreadable, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatalf("write the fixture binary: %v", err)
	}
	t.Setenv("ZE_EVIDENCE_ZE_BINARY", unreadable)
	env.ResetCache()

	if _, err := hostDaemon(tree, daemonBinaryName, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "is not executable") {
		t.Errorf("an override that cannot be executed answered %v, want a refusal", err)
	}
}

// VALIDATES: the daemon this proof builds is built for THIS machine, with every
// gate the manifest declares, and lands under the tree's scratch directory.
// PREVENTS: the two regressions this build has already had in this package. A
// cross-compiled binary cannot be executed by the run that built it, and a
// hand-written tag list goes stale the day a feature becomes a gate.
func TestTheHostDaemonIsBuiltForThisMachineWithEveryGate(t *testing.T) {
	run := fixtureL2TPPPP(t)

	argv, err := daemonBuildArgsTo(run.Tree, hostDaemonRel(daemonBinaryName))
	if err != nil {
		t.Fatalf("derive the build: %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, tag := range []string{"ze_core", "ze_distro", "ze_bgp", "ze_l2tp"} {
		if !strings.Contains(joined, tag) {
			t.Errorf("the build carries no %s: %s", tag, joined)
		}
	}
	if strings.Contains(joined, "GOARCH") || strings.Contains(joined, "GOOS") {
		t.Errorf("the host build names a target platform: %s", joined)
	}
	want := filepath.Join(run.Tree, "tmp", "evidence", "bin", daemonBinaryName)
	if !strings.Contains(joined, want) {
		t.Errorf("the build does not land at %s: %s", want, joined)
	}
}

// VALIDATES: two namespaces and a veth pair are named after this process, and
// the two link names fit the kernel's 15-character bound.
// PREVENTS: two runs on one machine colliding on a namespace, and a veth name
// the kernel refuses, which reads as "cannot create veth pair" with no reason.
func TestEachRunNamesItsOwnNamespacesAndLinks(t *testing.T) {
	run := fixtureL2TPPPP(t)

	if run.ZeNamespace == run.LACNamespace {
		t.Errorf("both namespaces are called %s", run.ZeNamespace)
	}
	if run.ZeVeth == run.LACVeth {
		t.Errorf("both veth ends are called %s", run.ZeVeth)
	}
	const linkMax = 15
	for _, link := range []string{run.ZeVeth, run.LACVeth} {
		if len(link) > linkMax {
			t.Errorf("the link name %q is %d characters, over the kernel's %d", link, len(link), linkMax)
		}
	}
	suffix := namespaceSuffix()
	for _, name := range []string{run.ZeNamespace, run.LACNamespace} {
		if !strings.HasSuffix(name, suffix) {
			t.Errorf("the namespace %q is not named after this process (%s)", name, suffix)
		}
	}
}

// VALIDATES: a run whose listen address was not named binds the underlay
// address ze holds, and one that names another binds that.
// PREVENTS: the default being written down twice. The Python original defaults
// the listen address to the ze underlay address, so a port carrying its own
// literal would diverge the day either one is changed.
func TestTheListenAddressDefaultsToTheZeUnderlayAddress(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if run := fixtureL2TPPPP(t); run.ListenIP != run.ZeIP {
		t.Errorf("the listener binds %s and the underlay is %s", run.ListenIP, run.ZeIP)
	}

	t.Setenv("ZE_L2TP_PPP_ZE_UNDERLAY_IP", "10.9.9.1")
	t.Setenv("ZE_L2TP_PPP_LISTEN_IP", "10.9.9.2")
	env.ResetCache()

	run := fixtureL2TPPPP(t)
	if run.ZeIP != "10.9.9.1" || run.ListenIP != "10.9.9.2" {
		t.Errorf("the run took underlay=%s listen=%s, want 10.9.9.1 and 10.9.9.2", run.ZeIP, run.ListenIP)
	}
}
