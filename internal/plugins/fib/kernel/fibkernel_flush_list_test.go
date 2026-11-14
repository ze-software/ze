// Design: docs/architecture/core-design.md -- FIB shutdown flush.
// Related: fibkernel.go -- flushResidueLocked, reportFlushLocked: the listing that verifies a flush.

package fibkernel

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// listFailingBackend accepts every route operation through the embedded mock
// and refuses its route listing, as a kernel whose route dump fails does.
type listFailingBackend struct {
	*mockBackend
	listErr error
}

func (backend *listFailingBackend) listZeRoutes() ([]installedRoute, error) {
	return nil, backend.listErr
}

// Goal: a flush whose deletes the kernel acknowledged, but whose result the
// kernel could not be listed to confirm, reaches the operator as an
// unverified flush: one doctor-fib-flush-incomplete error for the flush,
// carrying the listing's error, the number of routes the flush owned, and the
// command that removes Ze's routes. It does not also claim an incomplete
// flush, since every delete succeeded and no residue was observed.
//
// Method: a backend whose deletes succeed and whose listing fails, two routes
// installed through processEvent, one flushRoutes. The deletes and the
// cleared state are the controls that the flush itself ran.
func TestFIBKernelFlushReportsUnverifiedKernel(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &listFailingBackend{mockBackend: newMockBackend(), listErr: errors.New("route list: dump interrupted")}
	fib := newFIBKernel(backend)
	installFlushControls(t, fib, "198.18.99.0/24", "198.18.100.0/24")

	fib.flushRoutes()

	deleted := slices.Sorted(slices.Values(backend.deleted))
	if !slices.Equal(deleted, []string{"198.18.100.0/24", "198.18.99.0/24"}) {
		t.Fatalf("flush deleted %v, want both installed routes", deleted)
	}
	if len(fib.installed) != 0 {
		t.Fatalf("flush kept %d installed records", len(fib.installed))
	}
	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("flush raised %d flush errors, want the one unverified-flush error: %+v", len(issues), issues)
	}
	issue := issues[0]
	if want := "FIB flush could not verify the kernel: route list: dump interrupted"; issue.Message != want {
		t.Errorf("message = %q, want %q", issue.Message, want)
	}
	if issue.Fix != flushFix {
		t.Errorf("fix = %q, want %q", issue.Fix, flushFix)
	}
	if issue.Detail[keyOperation] != "flush" {
		t.Errorf("operation = %v, want flush", issue.Detail[keyOperation])
	}
	if issue.Detail["owned"] != 2 {
		t.Errorf("owned = %v, want the 2 installed routes", issue.Detail["owned"])
	}
	if _, counted := issue.Detail["residue"]; counted {
		t.Errorf("the unverified flush also reported a residue count: %+v", issue.Detail)
	}
}

// deleteAndListFailingBackend refuses every plain delete and its listing.
type deleteAndListFailingBackend struct {
	*failingDeleteBackend
	listErr error
}

func (backend *deleteAndListFailingBackend) listZeRoutes() ([]installedRoute, error) {
	return nil, backend.listErr
}

// Goal: a flush whose deletes failed and whose kernel could not be listed
// reports the failed deletes and that the kernel could not be read, and
// prints no count of routes left, because none was taken. Method: a backend
// that refuses every delete and its listing, two installed routes, one
// flushRoutes, then read the report bus: exactly one flush error, its message
// naming the failed deletes and the listing's error and holding no "still
// listed" figure, its detail holding no residue.
func TestFIBKernelFlushCountsNothingItCouldNotList(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &deleteAndListFailingBackend{
		failingDeleteBackend: &failingDeleteBackend{mockBackend: newMockBackend()},
		listErr:              errors.New("route list: dump interrupted"),
	}
	fib := newFIBKernel(backend)
	installFlushControls(t, fib, "198.18.101.0/24", "198.18.102.0/24")

	fib.flushRoutes()

	if backend.attempts != 2 {
		t.Fatalf("flush attempted %d deletes, want both installed routes", backend.attempts)
	}
	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("flush raised %d flush errors, want one: %+v", len(issues), issues)
	}
	issue := issues[0]
	want := "FIB flush left Ze routes in the kernel, which keeps forwarding after Ze stopped: " +
		"2 of 2 deletes failed, and the kernel could not be read to count what is left: route list: dump interrupted"
	if issue.Message != want {
		t.Errorf("message = %q, want %q", issue.Message, want)
	}
	if strings.Contains(issue.Message, "still listed") {
		t.Errorf("the message prints a count of routes left that was never taken: %q", issue.Message)
	}
	if _, counted := issue.Detail["residue"]; counted {
		t.Errorf("the detail carries a residue count that was never taken: %+v", issue.Detail)
	}
	if issue.Detail["failed"] != 2 {
		t.Errorf("failed = %v, want 2", issue.Detail["failed"])
	}
	if issue.Detail["list-error"] != "route list: dump interrupted" {
		t.Errorf("list error = %v, want the listing's error", issue.Detail["list-error"])
	}
	if issue.Fix != flushFix {
		t.Errorf("fix = %q, want %q", issue.Fix, flushFix)
	}
}

// Goal: the startup sweep removes a stale route from the table the kernel
// listed it in. Method: a backend whose listing holds one Ze route in table
// 100, startupSweep, no refresh, sweepStale, then read the table the rich
// delete named. A sweep that deletes by prefix alone asks the main table.
func TestFIBKernelSweepDeletesInTheListedTable(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := newTableRecordingBackend()
	backend.zeRoutes = []installedRoute{{prefix: "198.18.103.0/24", table: 100}}
	fib := newFIBKernel(backend)
	stale := fib.startupSweep()

	fib.sweepStale(stale)

	table, deleted := backend.richDeleted["198.18.103.0/24"]
	if !deleted {
		t.Fatalf("the sweep did not remove 198.18.103.0/24 through the rich delete; plain deletes: %v", backend.deleted)
	}
	if table != 100 {
		t.Errorf("the sweep removed 198.18.103.0/24 from table %d, want the listed table 100", table)
	}
	if len(fib.staleTables) != 0 {
		t.Errorf("the sweep kept %d stale table records", len(fib.staleTables))
	}
}

// Goal: a startup sweep whose delete the kernel refuses says so on the report
// bus, with the remedy, and does not count the route as removed. Method: a
// backend that refuses every delete, one stale prefix no protocol refreshed,
// one sweepStale, then read the report bus: one sweep error naming the prefix
// and the kernel's error, and no orphan warning claiming a removal.
func TestFIBKernelSweepReportsRefusedDelete(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &failingDeleteBackend{mockBackend: newMockBackend()}
	fib := newFIBKernel(backend)

	fib.sweepStale(map[string]string{"198.18.104.0/24": ""})

	if backend.attempts != 1 {
		t.Fatalf("the sweep attempted %d deletes, want one", backend.attempts)
	}
	var issues []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == reportCodeFIBSweepIncomplete && issue.Subject == "sweep" {
			issues = append(issues, issue)
		}
	}
	if len(issues) != 1 {
		t.Fatalf("the sweep raised %d sweep errors, want one: %+v", len(issues), issues)
	}
	issue := issues[0]
	want := "FIB startup sweep left stale Ze routes in the kernel, which keep forwarding with no route behind them: " +
		"1 of 1 stale prefixes could not be deleted, first 198.18.104.0/24: operation not permitted"
	if issue.Message != want {
		t.Errorf("message = %q, want %q", issue.Message, want)
	}
	if issue.Fix != sweepFix {
		t.Errorf("fix = %q, want %q", issue.Fix, sweepFix)
	}
	if issue.Detail["failed"] != 1 || issue.Detail["first-failure"] != "198.18.104.0/24" {
		t.Errorf("detail = %+v, want one failure naming 198.18.104.0/24", issue.Detail)
	}
	for _, warning := range report.Warnings() {
		if warning.Code == reportCodeFIBOrphan {
			t.Errorf("the sweep claimed removed orphans after a refused delete: %+v", warning)
		}
	}
}
