// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched.go -- installUnmatchedAtStart and unmatchedStartRefusal, under test
// Related: apply.go -- assertUnmatched, which takes the start and the reload apart
// Related: doctor_spd_install.go -- the ipsec-spd-install check the start-time refusal feeds
// RFC: rfc/short/rfc4301.md -- the final SPD entry (Section 4.4.1)

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// VALIDATES: owner decision 2026-11-20 (5c), the start-up rule of decision 3 (a)
// applied to the SPD catch-all. A start whose catch-all the dataplane partly refuses is
// not refused: it installs every catch-all entry the dataplane accepts and the operator
// entries, logs the refusal at Error under the doctor code, enters no discard in force,
// and ipsec-spd-install reports that the catch-all (for discard, the default discard)
// is NOT in force, with the dataplane's answer and the restart as its fix. The same
// refusal on a reload of that engine still fails the apply and keeps the finding, and a
// reload with the cause gone installs the catch-all and clears it.
// METHOD: refuse only the inbound catch-all entries, start, read the database, the log,
// discardInForce and the check; reload under the same refusal; lift it and reload.
// PREVENTS: the start refusing the whole IPsec block over its catch-all (the
// access-security door trap "A refusal added for reload also ran at startup", in
// reverse), a missing catch-all no operator is told of, the implicit-discard line
// claiming a discard that is not in force, and the reload losing its refusal.
func TestUnmatchedInstallFailureAtStartIsReportedByDoctor(t *testing.T) {
	refuseCatchAllIn := func(p dataplane.SPParams) error {
		if p.Role == dataplane.PolicyRoleCatchAll && p.Dir == dataplane.SADirIn {
			return errSPDRefusedByTest
		}
		return nil
	}
	for _, action := range []dataplane.SPAction{dataplane.SPActionDiscard, dataplane.SPActionBypass} {
		t.Run(unmatchedActionName(action), func(t *testing.T) {
			resetSPDLeft(t)
			dp := &refusingSPDDP{refuse: refuseCatchAllIn}
			loadFailClosedBackend(t, dp)
			var logged bytes.Buffer
			state := applyTestState(t)
			state.log = slog.New(slog.NewTextHandler(&logged, nil))
			cfg := lifecycleConfig(t)
			cfg.Unmatched = action
			cfg.UnmatchedByDefault = action == dataplane.SPActionDiscard

			if err := state.applyConfig(cfg, applyStartup); err != nil {
				t.Fatalf("the start refused the IPsec block over its catch-all: %v", err)
			}
			wantEntryHeld(t, dp, cfg.Policies["keep"])
			wantCatchAll(t, dp, action, func(dir dataplane.SADir) bool { return dir != dataplane.SADirIn })
			if state.discardInForce.Load() {
				t.Error("the engine records a discard in force although its inbound catch-all is not installed")
			}
			out := logged.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "NOT in force") {
				t.Fatalf("the start did not log the missing catch-all at Error:\n%s", out)
			}
			if !strings.Contains(out, "code="+diagnostic.CodeDoctorIPsecSPDCatchAllMissing) {
				t.Fatalf("the start's Error line does not name its doctor code:\n%s", out)
			}
			if strings.Contains(out, "catch-all DISCARD is in force") {
				t.Fatalf("the start logged an implicit discard in force that it could not install:\n%s", out)
			}

			findings := answerSPDInstall(state)
			if len(findings) != 1 {
				t.Fatalf("the doctor check answered %d findings, want 1 for the catch-all: %+v", len(findings), findings)
			}
			finding := findings[0]
			if finding.Code != diagnostic.CodeDoctorIPsecSPDCatchAllMissing || finding.Severity != severityError {
				t.Fatalf("the finding is %s/%s, want %s/%s",
					finding.Severity, finding.Code, severityError, diagnostic.CodeDoctorIPsecSPDCatchAllMissing)
			}
			wantText := "`vpn ipsec unmatched bypass` is NOT in force"
			if action == dataplane.SPActionDiscard {
				wantText = "The default discard is NOT in force"
			}
			if !strings.Contains(finding.Message, wantText) {
				t.Fatalf("the finding does not say %q: %q", wantText, finding.Message)
			}
			if !strings.Contains(finding.Message, errSPDRefusedByTest.Error()) {
				t.Fatalf("the finding does not carry the dataplane's answer: %q", finding.Message)
			}
			if strings.Contains(finding.Message, "\n") {
				t.Fatalf("the finding spans several lines: %q", finding.Message)
			}
			if finding.Fix != spdInstallFixRestart {
				t.Fatalf("the finding's fix is %q, want %q", finding.Fix, spdInstallFixRestart)
			}

			// The negative: the same refusal on a reload is still a refused apply.
			err := state.applyConfig(cfg, applyReload)
			if err == nil {
				t.Fatal("a reload whose catch-all the dataplane refused was applied")
			}
			if !errors.Is(err, errSPDRefusedByTest) || !strings.Contains(err.Error(), "catch-all") {
				t.Fatalf("the reload's refusal does not name the catch-all and the dataplane's answer: %v", err)
			}
			if findings := answerSPDInstall(state); len(findings) != 1 {
				t.Fatalf("a refused reload changed the catch-all finding: %+v", findings)
			}

			dp.refuse = nil
			if err := state.applyConfig(cfg, applyReload); err != nil {
				t.Fatalf("the reload with the cause gone was refused: %v", err)
			}
			wantCatchAll(t, dp, action, func(dataplane.SADir) bool { return true })
			if findings := answerSPDInstall(state); len(findings) != 0 {
				t.Fatalf("the doctor still reports a catch-all the reload installed: %+v", findings)
			}
			if got, want := state.discardInForce.Load(), action == dataplane.SPActionDiscard; got != want {
				t.Errorf("discardInForce is %v after the catch-all went in, want %v", got, want)
			}
		})
	}
}

// VALIDATES: a start with no dataplane loaded is still refused, the one catch-all
// failure decision 5c leaves refusing.
// METHOD: apply at start with no backend loaded and read the error and the record.
// PREVENTS: the later SPD writers, which read a nil dataplane as "nothing to do" on the
// strength of this refusal (installXFRMInterfaceEntries, installSPDPoliciesAtStart),
// recording entries as installed that nothing holds.
func TestUnmatchedWithNoDataplaneRefusesTheStart(t *testing.T) {
	// applyTestState loads a backend that holds every entry when none is loaded, so the
	// absence is made after it. Its cleanup closes the backend again, which is then a
	// no-op (dataplane.CloseBackend answers nil with none active).
	state := applyTestState(t)
	if err := dataplane.CloseBackend(); err != nil {
		t.Fatalf("close the backend applyTestState loaded: %v", err)
	}
	cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
	if err := state.applyConfig(cfg, applyStartup); !errors.Is(err, errUnmatchedNoDataplane) {
		t.Fatalf("a start with no dataplane answered %v, want %v", err, errUnmatchedNoDataplane)
	}
}

// VALIDATES: owner decision 2026-11-20 (5c) for a bound XFRM interface's own SPD
// catch-all. A start whose interface catch-all the backend refuses is not refused: the
// node catch-all goes in, no exemption hands traffic to the interface, the refusal is
// logged at Error under the doctor code naming the interface, and ipsec-spd-install
// reports that the interface's default discard is NOT in force, with the restart as its
// fix. The same refusal on a reload still fails the apply and keeps the finding; a
// reload with the cause gone installs the entry and clears it.
// METHOD: over a backend that refuses only interface catch-all entries, start a
// vti0-bound discard configuration, read the database, the log and the check; reload
// under the refusal; lift it and reload.
// PREVENTS: the start refusing the IPsec block over one interface's final entry, and
// an interface SPD without its final entry that no operator is told of.
func TestInterfaceSPDCatchAllInstallFailureAtStartIsReportedByDoctor(t *testing.T) {
	dp := &failInterfaceCatchAllInstallDP{refuse: true}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))
	cfg := vtiBoundConfig(dataplane.SPActionDiscard)

	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block over an interface catch-all: %v", err)
	}
	if got := liveCatchAll(t, &dp.catchAllDP); got != dataplane.SPActionDiscard {
		t.Errorf("node catch-all action %d after the start, want discard", got)
	}
	if got := liveInterfaceCatchAll(t, &dp.catchAllDP); len(got) != 0 {
		t.Errorf("interface catch-all entries live although the backend refused them: %q", got)
	}
	if got := liveExemptions(&dp.catchAllDP); len(got) != 0 {
		t.Errorf("exemptions hand traffic to an interface SPD with no final entry: %q", got)
	}
	out := logged.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "XFRM interface vti0 is NOT in force") {
		t.Fatalf("the start did not log the interface's missing catch-all at Error:\n%s", out)
	}
	if !strings.Contains(out, "code="+diagnostic.CodeDoctorIPsecSPDCatchAllMissing) {
		t.Fatalf("the start's Error line does not name its doctor code:\n%s", out)
	}

	findings := answerSPDInstall(state)
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for vti0: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != diagnostic.CodeDoctorIPsecSPDCatchAllMissing || finding.Severity != severityError {
		t.Fatalf("the finding is %s/%s, want %s/%s",
			finding.Severity, finding.Code, severityError, diagnostic.CodeDoctorIPsecSPDCatchAllMissing)
	}
	if !strings.Contains(finding.Message, "The default discard of XFRM interface vti0 is NOT in force") {
		t.Fatalf("the finding does not say vti0's default discard is not in force: %q", finding.Message)
	}
	if !strings.Contains(finding.Message, "operation not permitted") {
		t.Fatalf("the finding does not carry the dataplane's answer: %q", finding.Message)
	}
	if finding.Fix != spdInstallFixRestart {
		t.Fatalf("the finding's fix is %q, want %q", finding.Fix, spdInstallFixRestart)
	}

	// The negative: the same refusal on a reload is still a refused apply.
	err := state.applyConfig(cfg, applyReload)
	if err == nil {
		t.Fatal("a reload whose interface SPD catch-all the backend refused was applied")
	}
	if !strings.Contains(err.Error(), "vti0") || !strings.Contains(err.Error(), "catch-all") {
		t.Fatalf("the reload's refusal does not name the interface and its catch-all: %v", err)
	}
	if findings := answerSPDInstall(state); len(findings) != 1 {
		t.Fatalf("a refused reload changed the interface finding: %+v", findings)
	}

	dp.refuse = false
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("the reload with the cause gone was refused: %v", err)
	}
	if got := liveInterfaceCatchAll(t, &dp.catchAllDP); len(got) == 0 {
		t.Error("the reload with the cause gone installed no interface catch-all")
	}
	if findings := answerSPDInstall(state); len(findings) != 0 {
		t.Fatalf("the doctor still reports an interface catch-all the reload installed: %+v", findings)
	}
}

// VALIDATES: a start whose interface catch-all the backend refuses logs the interface at
// Error exactly once, however many entries of its catch-all were refused, and its
// finding cites the RFC 4301 obligation at its own level: the Section 5 MUST that an
// unmatched packet be discarded, quoted, and the Section 4.4.1 final entry as a SHOULD.
// METHOD: start the vti0-bound discard configuration of
// TestInterfaceSPDCatchAllInstallFailureAtStartIsReportedByDoctor over a backend that
// refuses every interface catch-all entry; count the Error lines naming vti0 and read
// the finding.
// PREVENTS: one failure logged once per refused entry and again for the interface, and a
// finding that says Section 4.4.1 forbids what that section only recommends against.
func TestInterfaceSPDCatchAllAtStartLogsOnceAndCitesTheMust(t *testing.T) {
	dp := &failInterfaceCatchAllInstallDP{refuse: true}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block over an interface catch-all: %v", err)
	}
	errorLines := 0
	for line := range strings.SplitSeq(logged.String(), "\n") {
		if strings.Contains(line, "level=ERROR") && strings.Contains(line, "vti0") {
			errorLines++
		}
	}
	if errorLines != 1 {
		t.Fatalf("the start logged vti0's missing catch-all on %d Error lines, want 1:\n%s", errorLines, logged.String())
	}

	findings := answerSPDInstall(state)
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for vti0: %+v", len(findings), findings)
	}
	const must = `"If no policy is found in the SPD that matches a packet (for either inbound or outbound traffic), the packet MUST be discarded."`
	if !strings.Contains(findings[0].Message, "RFC 4301 Section 5 requires it dropped: "+must) {
		t.Fatalf("the finding does not quote the Section 5 MUST: %q", findings[0].Message)
	}
	if strings.Contains(findings[0].Message, "4.4.1 forbids") || !strings.Contains(findings[0].Message, "SHOULD (Section 4.4.1)") {
		t.Fatalf("the finding does not cite Section 4.4.1 at its SHOULD level: %q", findings[0].Message)
	}
}

// failInterfaceExemptionInstallDP refuses, while refuse is set, the install of any if_id
// 0 exemption of a bound XFRM interface, and installs every other entry.
type failInterfaceExemptionInstallDP struct {
	catchAllDP
	refuse bool
}

func (d *failInterfaceExemptionInstallDP) InstallPolicy(p dataplane.SPParams) error {
	if d.refuse && p.Role == dataplane.PolicyRoleInterfaceExemption {
		return errors.New("test: operation not permitted")
	}
	return d.catchAllDP.InstallPolicy(p)
}

// VALIDATES: `ze doctor` reports a bound XFRM interface whose if_id 0 exemption the
// backend refused (ai/META-DECISIONS.md: doctor reports every problem an operator can
// hit), naming the interface, the dataplane's answer and the restart, under its own
// code; the start is not refused for it, and the finding clears once a retry places the
// exemption.
// METHOD: over a backend that refuses only interface exemptions, start a vti0-bound
// discard configuration and read the log and the check; lift the refusal, reload and
// read them again.
// PREVENTS: a tunnel whose routed traffic is dropped at the node-wide catch-all with
// only a log line to say so, and a finding that outlives the fix.
func TestInterfaceSPDExemptionInstallFailureIsReportedByDoctor(t *testing.T) {
	dp := &failInterfaceExemptionInstallDP{refuse: true}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	var logged bytes.Buffer
	state := applyTestState(t)
	state.log = slog.New(slog.NewTextHandler(&logged, nil))
	cfg := vtiBoundConfig(dataplane.SPActionDiscard)

	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("the start refused the IPsec block over an exemption: %v", err)
	}
	if got := liveExemptions(&dp.catchAllDP); len(got) != 0 {
		t.Fatalf("exemptions live although the backend refused them: %q", got)
	}
	if out := logged.String(); !strings.Contains(out, "code="+diagnostic.CodeDoctorIPsecSPDExemptionMissing) {
		t.Fatalf("the refused exemption was not logged under its doctor code:\n%s", out)
	}
	findings := answerSPDInstall(state)
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for vti0's exemption: %+v", len(findings), findings)
	}
	finding := findings[0]
	if finding.Code != diagnostic.CodeDoctorIPsecSPDExemptionMissing || finding.Severity != severityError {
		t.Fatalf("the finding is %s/%s, want %s/%s",
			finding.Severity, finding.Code, severityError, diagnostic.CodeDoctorIPsecSPDExemptionMissing)
	}
	if !strings.Contains(finding.Message, "XFRM interface vti0") || !strings.Contains(finding.Message, "operation not permitted") {
		t.Fatalf("the finding does not name vti0 and the dataplane's answer: %q", finding.Message)
	}
	if finding.Fix != spdInstallFixRestart {
		t.Fatalf("the finding's fix is %q, want %q", finding.Fix, spdInstallFixRestart)
	}

	// The negative: once a retry places the exemption, nothing is reported.
	dp.refuse = false
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("the reload with the cause gone was refused: %v", err)
	}
	if got := liveExemptions(&dp.catchAllDP); len(got) == 0 {
		t.Fatal("the reload with the cause gone installed no exemption")
	}
	if findings := answerSPDInstall(state); len(findings) != 0 {
		t.Fatalf("the doctor still reports an exemption the reload installed: %+v", findings)
	}
}

// wantCatchAll fails unless the backend holds the catch-all entry of each family in each
// direction want accepts, carrying action, and holds none in the others.
func wantCatchAll(t *testing.T, dp *refusingSPDDP, action dataplane.SPAction, want func(dataplane.SADir) bool) {
	t.Helper()
	for _, family := range spdFamilies {
		policies, err := unmatchedPolicies(family, action)
		if err != nil {
			t.Fatalf("build the catch-all: %v", err)
		}
		for i := range policies {
			p := &policies[i]
			held, ok := dp.live[keyOf(*p)]
			if !want(p.Dir) {
				if ok {
					t.Fatalf("the dataplane holds the refused %s catch-all %s entry", family, dirLabel(p.Dir))
				}
				continue
			}
			if !ok {
				t.Fatalf("the dataplane does not hold the %s catch-all %s entry", family, dirLabel(p.Dir))
			}
			if held.Action != action {
				t.Fatalf("the %s catch-all %s entry is action %d, want %d", family, dirLabel(p.Dir), held.Action, action)
			}
		}
	}
}
