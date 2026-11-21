// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions, transit traffic
// Related: spd_forward_answer.go -- spdForwardAnswer and moveSPDForwardAnswer, under test
// Related: config_verify_commit.go -- verifyIPsecCommit, the plugin's OnConfigVerify callback under test

package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// sshHostsForward reports whether dp holds the two forward policies ssh-hosts
// (forwardBypassEntry over 10.1.0.0/16, local port 22) projects on a gathering kernel:
// its remote-to-local decision and its local-to-remote gate, both in FWD.
func sshHostsForward(dp *gatheringDP) (decision, gate bool) {
	_, decision = dp.policies[spdKey{src: "198.51.100.0/24", dst: "10.1.0.0/16", dir: dataplane.SADirFwd}]
	_, gate = dp.policies[spdKey{src: "10.1.0.0/16", dst: "198.51.100.0/24", dir: dataplane.SADirFwd}]
	return decision, gate
}

// VALIDATES: the engine keeps the fragment-gathering answer it installed the operator
// entries with. A start whose read of the answer failed installs a port-scoped bypass
// without its forward projection, and the ipsec-spd-forward-bypass doctor check keeps
// reporting it, with the read failure, after the kernel's answer becomes readable; the
// next reload, the entry unchanged, installs the projection, the check clears, and the
// release removes the projection again. While the answer stays unreadable a reload
// installs nothing.
// METHOD: a forward-lookup dataplane whose answer fails, then gathers; start with one
// port-scoped bypass for a protected subnet, read the dataplane and the check, clear
// the failure, read the check, reload the same configuration, read both again, release.
// PREVENTS: a backend that keeps no failed read answering true later while the engine
// re-reads it: the doctor check then reports nothing, spdPolicySame skips the unchanged
// entry on every reload, and forwarded traffic on its ports meets the next entry with
// nothing saying so.
func TestPortScopedBypassForwardProjectionFollowsALaterFragmentAnswer(t *testing.T) {
	resetSPDLeft(t)
	holdAddresses(t, zeHostAddress)
	errGather := errors.New("capability receipt read refused by test")
	dp := &gatheringDP{forwardPolicyDP: &forwardPolicyDP{spdDP: newSPDDP()}, gathers: true, err: errGather}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	state.log = slogutil.DiscardLogger()
	entry := forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true)
	cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{entry.Name: entry})
	cfg.Unmatched = dataplane.SPActionBypass
	// forwardPolicyDP cannot refuse per-socket IPsec policies, so the default deny would
	// refuse the start (applyApplicationOverride); allow keeps that guard out of the subject.
	cfg.ApplicationOverride = ipsec.ApplicationOverrideAllow

	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block: %v", err)
	}
	if decision, gate := sshHostsForward(dp); decision || gate {
		t.Fatalf("an unreadable fragment answer projected the port-scoped bypass: decision=%v gate=%v", decision, gate)
	}

	// The negative: the answer is still unreadable, so a reload has nothing to apply.
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("a reload under the same unreadable answer was refused: %v", err)
	}
	if decision, gate := sshHostsForward(dp); decision || gate {
		t.Fatalf("a reload projected the port-scoped bypass on an unreadable answer: decision=%v gate=%v", decision, gate)
	}

	dp.err = nil
	findings := answerSPDForwardBypass(state)
	if len(findings) != 1 {
		t.Fatalf("with the kernel now answering, the check reported %d findings for an entry installed without its "+
			"forward projection, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "ssh-hosts") || !strings.Contains(findings[0].Message, errGather.Error()) {
		t.Fatalf("the finding does not name the entry and the read failure it was installed under: %q", findings[0].Message)
	}

	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("the reload with the answer readable was refused: %v", err)
	}
	if decision, gate := sshHostsForward(dp); !decision || !gate {
		t.Fatalf("the reload left the unchanged port-scoped bypass without its forward projection: decision=%v gate=%v",
			decision, gate)
	}
	if findings := answerSPDForwardBypass(state); len(findings) != 0 {
		t.Fatalf("the check still reports an entry whose projection the reload installed: %+v", findings)
	}

	state.releaseSPD(dp, slogutil.DiscardLogger())
	if decision, gate := sshHostsForward(dp); decision || gate {
		t.Fatalf("the release left the forward projection installed: decision=%v gate=%v", decision, gate)
	}
}

// failForwardDP refuses every forward policy once the test sets refuse, and installs
// every other one.
type failForwardDP struct {
	*gatheringDP
	refuse bool
}

func (d *failForwardDP) InstallPolicy(p dataplane.SPParams) error {
	if d.refuse && p.Dir == dataplane.SADirFwd {
		return errors.New("test: operation not permitted")
	}
	return d.gatheringDP.InstallPolicy(p)
}

// VALIDATES: a reload that must install the forward projection a newly readable answer
// owes, and whose dataplane refuses it, is refused with an operator error naming the
// entry, and the doctor check keeps reporting the entry, so the next commit retries.
// METHOD: start under an unreadable answer, make it readable and refuse forward
// installs, reload, read the error and the check.
// PREVENTS: a refused projection install recorded as done, after which no reload would
// try again.
func TestForwardProjectionInstallRefusedOnReloadKeepsTheRecordedAnswer(t *testing.T) {
	resetSPDLeft(t)
	holdAddresses(t, zeHostAddress)
	gathering := &gatheringDP{forwardPolicyDP: &forwardPolicyDP{spdDP: newSPDDP()}, gathers: true,
		err: errors.New("capability receipt read refused by test")}
	dp := &failForwardDP{gatheringDP: gathering}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	state.log = slogutil.DiscardLogger()
	entry := forwardBypassEntry(t, "ssh-hosts", dataplane.SPActionBypass, "10.1.0.0/16", true)
	cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{entry.Name: entry})
	cfg.Unmatched = dataplane.SPActionBypass
	cfg.ApplicationOverride = ipsec.ApplicationOverrideAllow
	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block: %v", err)
	}

	gathering.err = nil
	dp.refuse = true
	err := state.applyConfig(cfg, applyReload)
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("a reload whose forward projection the dataplane refused answered %v, want an operator error", err)
	}
	if !strings.Contains(refusal.Problem(), "ssh-hosts") || refusal.Fix() != spdInstallFixCommit {
		t.Fatalf("the refusal does not name the entry and the retry: %q, fix %q", refusal.Problem(), refusal.Fix())
	}
	if findings := answerSPDForwardBypass(state); len(findings) != 1 {
		t.Fatalf("after the refused projection the check reported %d findings, want 1: %+v", len(findings), findings)
	}
}

// VALIDATES: the plugin's real OnConfigVerify callback refuses a commit holding a bypass
// with a port for a protected subnet while a forward-lookup dataplane that does not
// gather fragments is loaded, with the operator error naming the entry and the
// any-port fix, and stages nothing; the same configuration with the port any is
// accepted and staged.
// METHOD: load a forward-lookup fake backend that declares no fragment gathering, then
// hand verifyIPsecCommit, the function runEngine registers with OnConfigVerify, a `vpn`
// section as the daemon delivers it.
// PREVENTS: the refusal existing in verifyForwardBypass while the callback that a commit
// reaches stops calling it, which TestRFC4301PortScopedForwardBypassRefusedAtCommit,
// calling the helper directly, cannot see.
func TestOnConfigVerifyRefusesPortScopedForwardBypass(t *testing.T) {
	holdAddresses(t, zeHostAddress)
	loadFailClosedBackend(t, &forwardPolicyDP{spdDP: newSPDDP()})
	section := func(port string) string {
		return `{"vpn": {"ipsec": {
		  "unmatched": "bypass",
		  "policy": {"ssh-hosts": {
		    "action": "bypass", "protocol": "6",
		    "local": {"prefix": "10.1.0.0/16", "port": "` + port + `"},
		    "remote": {"prefix": "198.51.100.0/24"}
		  }}
		}}}`
	}
	state := &ikeEngineState{}
	staging := &ikeConfigStaging{}

	err := verifyIPsecCommit(state, staging, vpnSection(section("22")))
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("the commit of a port-scoped bypass on a non-gathering forward dataplane answered %v, want an operator error", err)
	}
	if !strings.Contains(refusal.Problem(), "ssh-hosts") || !strings.Contains(refusal.Why(), "RFC 4301 Section 7.4") {
		t.Fatalf("the refusal does not name the entry and the RFC section: %q / %q", refusal.Problem(), refusal.Why())
	}
	if want := "set vpn ipsec policy ssh-hosts local port any"; fixCommand(refusal.Fix()) != want {
		t.Fatalf("the refusal's fix is %q, want it to open with %q", refusal.Fix(), want)
	}
	if staging.pending != nil {
		t.Fatal("a refused commit was staged for the apply")
	}

	if err := verifyIPsecCommit(state, staging, vpnSection(section("any"))); err != nil {
		t.Fatalf("the any-port form of the entry was refused: %v", err)
	}
	if staging.pending == nil {
		t.Fatal("the accepted commit was not staged for the apply")
	}
}
