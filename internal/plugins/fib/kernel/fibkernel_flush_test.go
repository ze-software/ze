// Design: docs/architecture/core-design.md -- FIB shutdown flush.
// Related: fibkernel.go -- flushRoutes, flushOneLocked, flushResidueLocked, reportFlushLocked.

package fibkernel

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
)

// tableRecordingBackend records the table each rich delete names. Its
// constructor supplies the plain backend; its zero value is invalid.
type tableRecordingBackend struct {
	*mockBackend
	richDeleted map[string]uint32
}

func newTableRecordingBackend() *tableRecordingBackend {
	return &tableRecordingBackend{mockBackend: newMockBackend(), richDeleted: make(map[string]uint32)}
}

func (backend *tableRecordingBackend) addRichRoute(RichRoute) error     { return nil }
func (backend *tableRecordingBackend) replaceRichRoute(RichRoute) error { return nil }

func (backend *tableRecordingBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error { //nolint:unparam // richRouteBackend method: the interface returns an error, this double never fails
	backend.richDeleted[prefix.String()] = tableID
	return nil
}

// failingDeleteBackend refuses every plain delete, as a kernel that answers
// an error other than ESRCH does.
type failingDeleteBackend struct {
	*mockBackend
	attempts int
}

func (backend *failingDeleteBackend) delRoute(string) error {
	backend.attempts++
	return errors.New("operation not permitted")
}

func flushIssues() []report.Issue {
	var issues []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == reportCodeFIBFlushIncomplete && issue.Subject == "flush" {
			issues = append(issues, issue)
		}
	}
	return issues
}

func installFlushControls(t *testing.T, fib *fibKernel, prefixes ...string) {
	t.Helper()
	changes := make([]incomingChange, 0, len(prefixes))
	for _, prefix := range prefixes {
		changes = append(changes, incomingChange{
			Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix),
			NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "static",
		})
	}
	fib.processEvent(makeSysribPayload(changes))
	if len(fib.installed) != len(prefixes) {
		t.Fatalf("control installation retained %d routes, want %d", len(fib.installed), len(prefixes))
	}
}

// Goal: the stop flush removes a route from the table it was installed in.
// Method: install a route carrying table 100 through processEvent, flush, and
// read the table the rich delete named. A flush that deletes by prefix alone
// asks the main table and leaves the table-100 route forwarding.
func TestFIBKernelFlushDeletesInTheInstalledTable(t *testing.T) {
	backend := newTableRecordingBackend()
	fib := newFIBKernel(backend)
	prefix := "198.18.91.0/24"
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix),
		NextHop: netip.MustParseAddr("192.0.2.1"), TableID: 100, Protocol: "static",
	}}))
	if _, installed := fib.installed[prefix]; !installed {
		t.Fatalf("control route %s was not installed", prefix)
	}

	fib.flushRoutes()

	table, deleted := backend.richDeleted[prefix]
	if !deleted {
		t.Fatalf("flush did not remove %s through the rich delete; plain deletes: %v", prefix, backend.deleted)
	}
	if table != 100 {
		t.Errorf("flush removed %s from table %d, want the installed table 100", prefix, table)
	}
}

// Goal: a flush whose deletes the kernel acknowledged but whose routes the
// kernel still lists reaches the operator. Method: install two routes, make
// the backend's listing still hold one of them after the deletes, flush, and
// read the report bus: one doctor-fib-flush-incomplete for the flush, naming the residue,
// carrying the command that removes it. A clean flush is the positive control.
func TestFIBKernelFlushReportsRoutesTheKernelKept(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	clean := newFIBKernel(newMockBackend())
	installFlushControls(t, clean, "198.18.92.0/24")
	clean.flushRoutes()
	if issues := flushIssues(); len(issues) != 0 {
		t.Fatalf("a clean flush raised %d flush failures: %+v", len(issues), issues)
	}

	backend := newMockBackend()
	fib := newFIBKernel(backend)
	installFlushControls(t, fib, "198.18.93.0/24", "198.18.94.0/24")
	backend.zeRoutes = []installedRoute{{prefix: "198.18.94.0/24"}, {prefix: "203.0.113.0/24"}}

	fib.flushRoutes()

	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("flush raised %d flush failures, want one for the whole flush: %+v", len(issues), issues)
	}
	issue := issues[0]
	if issue.Detail["residue"] != 1 {
		t.Errorf("residue = %v, want 1 (203.0.113.0/24 was never Ze's to flush)", issue.Detail["residue"])
	}
	if issue.Detail["first-residue"] != "198.18.94.0/24" {
		t.Errorf("first residue = %v, want 198.18.94.0/24", issue.Detail["first-residue"])
	}
	if issue.Detail["failed"] != 0 {
		t.Errorf("failed = %v, want 0: every delete was acknowledged", issue.Detail["failed"])
	}
	if issue.Fix != flushFix {
		t.Errorf("fix = %q, want %q", issue.Fix, flushFix)
	}
	for _, other := range report.Errors(0) {
		if other.Code == reportCodeFIBSyncFailure {
			t.Errorf("the flush also raised %s, which names a per-prefix sync failure: %+v", reportCodeFIBSyncFailure, other)
		}
	}
}

// Goal: deletes the kernel refuses at stop are reported once, with their
// count, never one line per route and never not at all. Method: a backend
// that refuses every delete, three installed routes, one flush.
func TestFIBKernelFlushReportsFailedDeletes(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &failingDeleteBackend{mockBackend: newMockBackend()}
	fib := newFIBKernel(backend)
	installFlushControls(t, fib, "198.18.95.0/24", "198.18.96.0/24", "198.18.97.0/24")

	fib.flushRoutes()

	if backend.attempts != 3 {
		t.Fatalf("flush attempted %d deletes, want all three installed routes", backend.attempts)
	}
	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("flush raised %d flush failures, want exactly one: %+v", len(issues), issues)
	}
	if issues[0].Detail["failed"] != 3 {
		t.Errorf("failed = %v, want 3", issues[0].Detail["failed"])
	}
	if issues[0].Detail["first-error"] != "operation not permitted" {
		t.Errorf("first error = %v, want the backend's refusal", issues[0].Detail["first-error"])
	}
}

// Goal: flush-on-stop removes a route a previous run left that this run
// retained at startup and has not swept yet. Method: the backend lists one Ze
// route at startup, startupSweep retains it, sysrib never refreshes it, and the
// flush runs before the sweep would.
func TestFIBKernelFlushRemovesRetainedRoutes(t *testing.T) {
	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{{prefix: "198.18.98.0/24", nextHop: "192.0.2.1"}}
	fib := newFIBKernel(backend)
	if stale := fib.startupSweep(); len(stale) != 1 {
		t.Fatalf("startup retained %d routes, want the one listed", len(stale))
	}
	backend.zeRoutes = nil

	fib.flushRoutes()

	if !slices.Contains(backend.deleted, "198.18.98.0/24") {
		t.Errorf("flush deleted %v, want the retained route 198.18.98.0/24", backend.deleted)
	}
	if len(fib.retained) != 0 {
		t.Errorf("flush kept %d retained records", len(fib.retained))
	}
}
