package local

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// firewallCall is one call the sweep made to the firewall registry, in order.
// kind is the entry point, and tables is how many tables a register carried. A
// reconcile records the applyAll call itself.
type firewallCall struct {
	kind   string
	tables int
}

// recordingFirewall replaces the firewall entry points and records the ORDER of
// what a caller asked of them. The sweep's correctness is an ordering property
// rather than a count. The name has to be in the desired set for one reconcile
// and out of it for the next, and a recorder is the only witness of that.
//
// applyErr is returned by the Nth reconcile, counting from one, so a test can
// drive the failure arm. Zero means every reconcile succeeds.
func recordingFirewall(failOnReconcile int, applyErr error) (calls func() []firewallCall, restore func()) {
	origReg := registerTables
	origApply := applyAll
	var recorded []firewallCall
	reconciles := 0
	registerTables = func(_ string, tables []firewall.Table) error {
		recorded = append(recorded, firewallCall{kind: "register", tables: len(tables)})
		return nil
	}
	applyAll = func() error {
		reconciles++
		recorded = append(recorded, firewallCall{kind: "reconcile"})
		if reconciles == failOnReconcile {
			return applyErr
		}
		return nil
	}
	return func() []firewallCall { return recorded },
		func() {
			registerTables = origReg
			applyAll = origApply
		}
}

// TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt proves the start-of-process
// sweep does the one thing that can reach a table this process did not write.
//
// VALIDATES: a ddos-local drop rule is never persistent state -- it does not
// survive the process that installed it, whatever `firewall flush-on-shutdown`
// says (owner directive, 2026-09-09).
// PREVENTS: the sweep collapsing into one reconcile. shouldDeleteTable
// (internal/plugins/firewall/nft/backend_linux.go) deletes a ze_ table only when
// the name is in the desired set or in this backend instance's applied map, and a
// fresh process's map is empty. So a reconcile that does not CLAIM the name
// leaves the stale table where it was. A claim that is never withdrawn leaves an
// empty table of ze's own behind. Both halves are the mechanism, so both are
// asserted here, and test/plugin/ddos-local-stale-table-swept.ci reads the kernel
// back.
func TestStaleDropRuleSweepClaimsTheNameThenWithdrawsIt(t *testing.T) {
	calls, restore := recordingFirewall(0, nil)
	defer restore()

	if err := clearStaleDropRule(); err != nil {
		t.Fatalf("the sweep must succeed when every reconcile does: %v", err)
	}

	want := []firewallCall{
		{kind: "register", tables: 1},
		{kind: "reconcile"},
		{kind: "register", tables: 0},
		{kind: "reconcile"},
	}
	got := calls()
	if len(got) != len(want) {
		t.Fatalf("the sweep must claim the table name, reconcile, withdraw it and reconcile again; got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call %d is %v, want %v (whole sequence %v)", i, got[i], want[i], got)
		}
	}
}

// TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName proves the sweep
// claims the name the responder installs under, which is the only name that
// reaches a stale rule.
//
// VALIDATES: the same directive.
// PREVENTS: a sweep of some other name. RegisterTables refuses a name without the
// ze_ ownership prefix (internal/component/firewall/registry.go), and the backend
// matches a kernel table by name, so a claim under any other spelling deletes
// nothing and reports success.
func TestStaleDropRuleSweepClaimsTheTableUnderTheResponderName(t *testing.T) {
	origReg := registerTables
	origApply := applyAll
	defer func() {
		registerTables = origReg
		applyAll = origApply
	}()

	var claimed []firewall.Table
	var owner string
	registerTables = func(name string, tables []firewall.Table) error {
		if len(tables) > 0 {
			owner = name
			claimed = tables
		}
		return nil
	}
	applyAll = func() error { return nil }

	if err := clearStaleDropRule(); err != nil {
		t.Fatalf("the sweep must succeed when every reconcile does: %v", err)
	}

	if owner != tableName {
		t.Errorf("the sweep claimed owner %q, want %q: any other key leaves the responder's own table unclaimed", owner, tableName)
	}
	if len(claimed) != 1 {
		t.Fatalf("the sweep must claim exactly one table, got %d", len(claimed))
	}
	if claimed[0].Name != tableName {
		t.Errorf("the sweep claimed table %q, want %q: the backend matches a kernel table by name", claimed[0].Name, tableName)
	}
	if len(claimed[0].Chains) != 0 {
		t.Errorf("the claimed table carries %d chains, want none: it exists for one reconcile so the backend counts the name as one it owns, and a chain would enforce something nobody asked for",
			len(claimed[0].Chains))
	}
}

// TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile proves a sweep that
// cannot reach the kernel does not arm the next owner's reconcile to install an
// empty table of ze's own.
//
// VALIDATES: the same directive, on the arm where the firewall backend is
// unusable -- the daemon must still detect and report.
// PREVENTS: a registration left standing after the reconcile that was meant to
// consume it failed. The registry is process-wide and keyed by owner
// (internal/component/firewall/registry.go). A claim nobody withdraws is merged
// into every later ApplyAll by any owner, so the empty ze_ddos-local table
// appears in the kernel with no attack and no responder behind it.
func TestStaleDropRuleSweepLeavesNoClaimBehindOnAFailedReconcile(t *testing.T) {
	wedged := errors.New("the kernel is wedged")
	calls, restore := recordingFirewall(1, wedged)
	defer restore()

	err := clearStaleDropRule()
	if !errors.Is(err, wedged) {
		t.Fatalf("a failed reconcile must be reported to the caller, got %v", err)
	}

	got := calls()
	if len(got) == 0 {
		t.Fatal("the sweep asked the firewall registry for nothing")
	}
	last := got[len(got)-1]
	if last.kind != "register" || last.tables != 0 {
		t.Errorf("the sweep left %v as its last act, want a withdraw carrying no table (whole sequence %v)", last, got)
	}
}

// captureLog routes the package logger into a buffer for one test, and restores
// what was there. It reads at Debug so no line the responder writes is filtered
// out by level.
func captureLog(t *testing.T) (lines func() string) {
	t.Helper()
	buf := &bytes.Buffer{}
	previous := loggerPtr.Load()
	loggerPtr.Store(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return buf.String
}

// TestRemoveMitigationDoesNotReportARemovalTheKernelRefused proves a withdrawal
// the kernel refused is reported as the failure it is.
//
// VALIDATES: the operator can tell a rule that went out from one that did not.
// PREVENTS: "drop rule removed" printed on the line after "failed to remove drop
// rule". Most callers have a later reconcile that repairs a failed withdrawal,
// because the detector re-fires about once a second. The engine's exit path has
// none. There this line is the only witness the operator gets, and it said the
// opposite of what happened (ai/rules/evidence.md).
func TestRemoveMitigationDoesNotReportARemovalTheKernelRefused(t *testing.T) {
	lines := captureLog(t)

	origReg := registerTables
	origApply := applyAll
	defer func() {
		registerTables = origReg
		applyAll = origApply
	}()
	registerTables = func(string, []firewall.Table) error { return nil }
	applyAll = func() error { return errors.New("the kernel is wedged") }

	r := newResponder(enforcing(), nil)
	r.mu.Lock()
	r.setStatus(true, floodVictim(), firewall.HookInput)
	r.removeMitigation()
	r.mu.Unlock()

	log := lines()
	if !strings.Contains(log, "failed to remove drop rule") {
		t.Errorf("a refused withdrawal must be reported, log was:\n%s", log)
	}
	if strings.Contains(log, "drop rule removed") {
		t.Errorf("a refused withdrawal must not also report a removal: the rule is still in the kernel and this line is what an operator acts on. Log was:\n%s", log)
	}
}
