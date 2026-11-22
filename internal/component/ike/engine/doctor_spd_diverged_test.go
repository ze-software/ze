// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: spd_policy.go -- reconcileSPDPolicies, undoSPDInstall and removeReplacedSPDPolicies, which answer what diverged
// Related: doctor_spd_install.go -- recordSPDReconcile and answerSPDInstall, which keep and report it

package engine

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// installSPDPolicies is a test helper, not production code: no production function of
// this name exists since the reconciler became reconcileSPDPolicies. It reconciles
// entries onto dp the way a reload does and answers only the refusal. It serves the
// tests that judge what the reconcile writes, which read the
// dataplane itself; the divergence the reconcile also answers is judged here, through
// applyConfig and answerSPDInstall.
func installSPDPolicies(dp dataplane.Dataplane, previous, next map[string]ipsec.SPDPolicy, log *slog.Logger) error {
	_, err := reconcileSPDPolicies(dp, previous, next, log)
	return err
}

// errSPDRemoveRefusedByTest is the dataplane's answer to a refused removal. It is none
// of the answers policyGone reads as a policy already gone.
var errSPDRemoveRefusedByTest = errors.New("netlink: operation not permitted")

// outboundOf answers the outbound policy of entry.
func outboundOf(t *testing.T, entry ipsec.SPDPolicy) dataplane.SPParams {
	t.Helper()
	policies := spdPolicyParams(entry)
	for i := range policies {
		if policies[i].Dir == dataplane.SADirOut {
			return policies[i]
		}
	}
	t.Fatalf("entry %s installs no outbound policy", entry.Name)
	return dataplane.SPParams{}
}

// wantOneDivergence fails unless findings is exactly one undo-incomplete error naming
// prefix and the dataplane's answer, with the restart as its fix.
func wantOneDivergence(t *testing.T, findings []rpc.DoctorCheckDiagnostic, prefix string, cause error) {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("the doctor check answered %d findings, want 1 for the divergence: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Code != codeDoctorIPsecSPDUndoIncomplete || f.Severity != severityError {
		t.Fatalf("the finding is %s/%s, want %s/%s", f.Severity, f.Code, severityError, codeDoctorIPsecSPDUndoIncomplete)
	}
	if !strings.Contains(f.Message, prefix) || !strings.Contains(f.Message, cause.Error()) || !strings.Contains(f.Message, "dir out") {
		t.Fatalf("the finding does not name the direction, the selector %s and the dataplane's answer: %q", prefix, f.Message)
	}
	if f.Fix != spdInstallFixRestart() {
		t.Fatalf("the finding's fix is %q, want %q", f.Fix, spdInstallFixRestart())
	}
}

// VALIDATES: an accepted reload whose removal of an entry's previous form the
// dataplane refuses is reported by ipsec-spd-install as a divergence, with the
// direction, the selector, the dataplane's answer and the restart; the finding outlives
// a later reload that leaves the selector alone, and a reload that settles the selector
// clears it.
// METHOD: start with a discard entry, reload it moved to a new prefix while the backend
// refuses removing the old outbound policy, and read the check; reload an unrelated
// change and read it again; then lift the refusal, reload the old form and read it.
// PREVENTS: the previous form staying in the dataplane while only a Warn log line said
// so, and the record being dropped by an apply that never touched the selector.
func TestSPDPreviousFormNotRemovedIsReportedByDoctor(t *testing.T) {
	resetSPDLeft(t)
	dp := &refusingSPDDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	old := spdDiscardEntry(t)
	cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{"drop": old})
	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("start: %v", err)
	}
	if findings := answerSPDInstall(state); len(findings) != 0 {
		t.Fatalf("the doctor reports a clean start: %+v", findings)
	}

	dp.removeErr = map[spKey]error{keyOf(outboundOf(t, old)): errSPDRemoveRefusedByTest}
	moved := old
	moved.LocalPrefix = mustCIDR(t, "192.0.2.0/25")
	movedCfg := withPolicies(cfg, map[string]ipsec.SPDPolicy{"drop": moved})
	if err := state.applyConfig(movedCfg, applyReload); err != nil {
		t.Fatalf("the reload whose old form could not be removed was refused: %v", err)
	}
	wantOneDivergence(t, answerSPDInstall(state), "192.0.2.0/24", errSPDRemoveRefusedByTest)

	other := failClosedBypassEntry(t, "keep", "203.0.113.0/24")
	if err := state.applyConfig(withPolicies(cfg, map[string]ipsec.SPDPolicy{"drop": moved, "keep": other}), applyReload); err != nil {
		t.Fatalf("the unrelated reload was refused: %v", err)
	}
	wantOneDivergence(t, answerSPDInstall(state), "192.0.2.0/24", errSPDRemoveRefusedByTest)

	dp.removeErr = nil
	if err := state.applyConfig(withPolicies(cfg, map[string]ipsec.SPDPolicy{"drop": old, "keep": other}), applyReload); err != nil {
		t.Fatalf("the reload back to the old form was refused: %v", err)
	}
	if findings := answerSPDInstall(state); len(findings) != 0 {
		t.Fatalf("the doctor still reports a selector the reload settled: %+v", findings)
	}
}

// VALIDATES: a refused reload whose undo cannot take back a policy it wrote is
// reported by ipsec-spd-install as a divergence, beside the refusal of the commit.
// METHOD: start with a discard entry, reload it moved to a new prefix while the backend
// refuses the inbound half and refuses removing the outbound half already written, and
// read the error and the check.
// PREVENTS: the dataplane holding a policy the running configuration does not, while
// only an Error log line said so.
func TestSPDUndoNotTakenBackIsReportedByDoctor(t *testing.T) {
	resetSPDLeft(t)
	dp := &refusingSPDDP{}
	loadFailClosedBackend(t, dp)
	state := applyTestState(t)
	old := spdDiscardEntry(t)
	cfg := withPolicies(lifecycleConfig(t), map[string]ipsec.SPDPolicy{"drop": old})
	if err := state.applyConfig(cfg, applyStartup); err != nil {
		t.Fatalf("start: %v", err)
	}

	moved := old
	moved.LocalPrefix = mustCIDR(t, "192.0.2.0/25")
	dp.refuse = func(p dataplane.SPParams) error {
		if p.Role == dataplane.PolicyRoleOperator && p.Dir == dataplane.SADirIn {
			return errSPDRefusedByTest
		}
		return nil
	}
	dp.removeErr = map[spKey]error{keyOf(outboundOf(t, moved)): errSPDRemoveRefusedByTest}
	if err := state.applyConfig(withPolicies(cfg, map[string]ipsec.SPDPolicy{"drop": moved}), applyReload); !errors.Is(err, errSPDRefusedByTest) {
		t.Fatalf("the reload whose entry the dataplane refused answered %v, want the refusal", err)
	}
	wantOneDivergence(t, answerSPDInstall(state), "192.0.2.0/25", errSPDRemoveRefusedByTest)
}

// VALIDATES: the divergence record's rule: a settled selector clears its record, and a
// step that diverges after a write at the same selector keeps the divergence.
// METHOD: fold reconciles into an empty state and read the record.
// PREVENTS: the undo of a write being overwritten by the write it undid.
func TestRecordSPDReconcileAppliesSettledFirst(t *testing.T) {
	state := applyTestState(t)
	sp := outboundOf(t, spdDiscardEntry(t))
	sel := selectorOf(&sp)
	state.recordSPDReconcile(spdReconcile{
		settled:  []spdSelector{sel},
		diverged: []spdDivergence{newSPDDivergence(spdDivergenceNotTakenBack, &sp, errSPDRemoveRefusedByTest)},
	})
	if _, held := state.spdDiverged[sel]; !held {
		t.Fatal("the undo's divergence was lost to the write it undid")
	}
	state.recordSPDReconcile(spdReconcile{settled: []spdSelector{sel}})
	if state.spdDiverged != nil {
		t.Fatalf("a settled selector kept its record: %+v", state.spdDiverged)
	}
}
