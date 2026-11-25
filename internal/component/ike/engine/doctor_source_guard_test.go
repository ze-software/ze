// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_source_guard.go -- the ipsec-source-guard check under test
// Related: source_guard_test.go -- guardRecorder, the guard's kernel side

package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: a refused tunnel-source rule and a refused publish of the guard's nftables
// table each make ipsec-source-guard report one error under
// doctor-ipsec-source-guard-missing, naming the interface, the address, the refused
// step and the cause, with the fix the log line names (the same classifier); the log
// line carries the same code; a retry that installs the step clears the finding.
// METHOD: fail the recorder's rule install, then its publish, install the guard, read
// the findings and the log, then clear the failure, install again and read again.
// PREVENTS: a source exemption left uninstalled by its guard that ze doctor never
// reports, and a doctor finding that names another cause than the log line.
func TestSourceGuardCheckReportsEachRefusal(t *testing.T) {
	t.Cleanup(privilege.SetServiceMembershipForTest(privilege.MembershipZeService, nil))
	addr := netip.MustParseAddr("2001:db8::1")
	want := map[string]sourceGuardSpec{
		"vti0": {iface: "vti0", device: "vti0", ifIndex: 7, ifID: vti0IfID, sources: []netip.Addr{addr}},
	}
	ruleErr := fmt.Errorf("ip rule add from 2001:db8::1/128 lookup 3000: %w", syscall.EAFNOSUPPORT)
	publishErr := fmt.Errorf("firewallnft: flush: %w", syscall.EOPNOTSUPP)
	cases := []struct {
		name, step, option string
		fix                string
		arm                func(*guardRecorder)
		disarm             func(*guardRecorder)
	}{
		{
			name: "rule", step: "tunnel-source rule", option: "CONFIG_IPV6_MULTIPLE_TABLES",
			fix:    sourceRuleRequest.fix(ruleErr, ""),
			arm:    func(r *guardRecorder) { r.failRule = ruleErr },
			disarm: func(r *guardRecorder) { r.failRule = nil },
		},
		{
			name: "publish", step: "nft table " + sourceGuardTableName, option: "CONFIG_NF_TABLES",
			fix:    nftRequest.fix(publishErr, ""),
			arm:    func(r *guardRecorder) { r.failPublish = publishErr },
			disarm: func(r *guardRecorder) { r.failPublish = nil },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := newGuardRecorder()
			g := newSourceGuards(recorder)
			tc.arm(recorder)
			var logged bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logged, nil))
			if guarded := g.install(want, log); guarded[addr] {
				t.Fatalf("%s is answered guarded while its %s was refused", addr, tc.step)
			}
			if !strings.Contains(logged.String(), "code="+codeDoctorIPsecSourceGuardMissing) {
				t.Errorf("the refusal's log line carries no doctor code: %s", logged.String())
			}

			findings := g.findings()
			if len(findings) != 1 {
				t.Fatalf("findings = %d (%+v), want one for the refused %s", len(findings), findings, tc.step)
			}
			finding := findings[0]
			if finding.Code != codeDoctorIPsecSourceGuardMissing {
				t.Errorf("code = %q, want %q", finding.Code, codeDoctorIPsecSourceGuardMissing)
			}
			if finding.Severity != severityError {
				t.Errorf("severity = %q, want %q", finding.Severity, severityError)
			}
			for _, part := range []string{"vti0", addr.String(), tc.step, tc.option, "DISCARD catch-all"} {
				if !strings.Contains(finding.Message, part) {
					t.Errorf("the finding does not name %q: %s", part, finding.Message)
				}
			}
			if strings.Contains(finding.Message, "CAP_NET_ADMIN") {
				t.Errorf("a kernel without the feature is sent after a capability: %s", finding.Message)
			}
			if finding.Fix != tc.fix {
				t.Errorf("fix = %q, want the log line's %q", finding.Fix, tc.fix)
			}
			if !strings.Contains(finding.Fix, tc.option) {
				t.Errorf("the fix does not name %s: %q", tc.option, finding.Fix)
			}
			if !strings.HasSuffix(finding.Fix, "restart ze: sudo systemctl restart ze") {
				t.Errorf("the fix does not end in the restart under ze.service: %q", finding.Fix)
			}

			tc.disarm(recorder)
			if guarded := g.install(want, log); !guarded[addr] {
				t.Fatalf("%s is not guarded after the retry", addr)
			}
			if got := g.findings(); len(got) != 0 {
				t.Errorf("the retry that installed the %s left findings: %+v", tc.step, got)
			}
		})
	}
}

// VALIDATES: a recorded rule refusal leaves the record with the address: once prune
// drops the address, ipsec-source-guard reports nothing for it.
// METHOD: fail the rule, install, then prune with nothing wanted and read the findings.
// PREVENTS: a finding for an address the interface no longer holds.
func TestSourceGuardCheckForgetsAPrunedAddress(t *testing.T) {
	recorder := newGuardRecorder()
	recorder.failRule = errors.New("test: operation refused")
	g := newSourceGuards(recorder)
	log := slog.New(slog.DiscardHandler)
	want := map[string]sourceGuardSpec{
		"vti0": {iface: "vti0", device: "vti0", ifIndex: 7, ifID: vti0IfID, sources: []netip.Addr{netip.MustParseAddr("2001:db8::1")}},
	}
	g.install(want, log)
	if got := g.findings(); len(got) != 1 {
		t.Fatalf("findings before the prune = %+v, want the rule refusal", got)
	}
	g.prune(map[string]sourceGuardSpec{}, log)
	if got := g.findings(); len(got) != 0 {
		t.Errorf("findings after the address left = %+v, want none", got)
	}
	if len(g.ruleRefused) != 0 {
		t.Errorf("the rule refusal of a pruned address is still recorded: %v", g.ruleRefused)
	}
}

// VALIDATES: through the engine's doctor callback, a discard apply whose guard rule is
// refused reports the address under doctor-ipsec-source-guard-missing, the source
// exemption it kept out of the SPD, and the address event that installs the rule
// clears it.
// METHOD: fail the stubbed guard's rule, apply discard over a bound vti0, answer
// ipsec-source-guard through answerDoctorCheck, then clear the failure, run the address
// worker and answer again.
// PREVENTS: the refusal the install pass skips before the SPD (`!guarded[source]`),
// which never reaches interfaceExemptionRefused, going unreported.
func TestSourceGuardCheckThroughTheEngine(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	stub.guard.failRule = errors.New("test: operation refused")
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	findings, err := state.answerDoctorCheck(sourceGuardCheckName)
	if err != nil {
		t.Fatalf("answer %s: %v", sourceGuardCheckName, err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings with the rule refused = %+v, want one", findings)
	}
	if findings[0].Code != codeDoctorIPsecSourceGuardMissing {
		t.Errorf("code = %q, want %q", findings[0].Code, codeDoctorIPsecSourceGuardMissing)
	}
	if !strings.Contains(findings[0].Message, "2001:db8::1") {
		t.Errorf("the finding does not name 2001:db8::1: %s", findings[0].Message)
	}
	if findings[0].Fix == "" {
		t.Error("the finding names no fix")
	}

	stub.guard.failRule = nil
	state.followAddress()
	findings, err = state.answerDoctorCheck(sourceGuardCheckName)
	if err != nil {
		t.Fatalf("answer %s after the retry: %v", sourceGuardCheckName, err)
	}
	if len(findings) != 0 {
		t.Errorf("findings after the address event installed the rule = %+v, want none", findings)
	}
}

// VALIDATES: ipsec-source-guard is declared in the engine's Stage 1 registration at the
// runtime phase with doctor-ipsec-source-guard-missing, and the code has an explanation
// entry ze show diagnostic describes.
// PREVENTS: an Error line naming a code no check declares, which the doctor-coverage
// gate refuses, and a check run in a process that holds no guard.
func TestSourceGuardCheckRegistered(t *testing.T) {
	var found bool
	for _, decl := range engineDoctorCheckDecls() {
		if decl.Name != sourceGuardCheckName {
			continue
		}
		found = true
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("phase = %q, want %q", decl.Phase, rpc.DoctorPhaseRuntime)
		}
		if got := strings.Join(decl.Codes, ","); got != codeDoctorIPsecSourceGuardMissing {
			t.Errorf("codes = %q, want %q", got, codeDoctorIPsecSourceGuardMissing)
		}
	}
	if !found {
		t.Fatalf("the engine declares no %s doctor check", sourceGuardCheckName)
	}
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeDoctorIPsecSourceGuardMissing) == nil {
		t.Errorf("%s has no explanation entry", codeDoctorIPsecSourceGuardMissing)
	}
}
