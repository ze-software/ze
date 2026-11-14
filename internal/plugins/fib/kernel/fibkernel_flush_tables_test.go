// Design: docs/architecture/core-design.md -- FIB startup sweep and shutdown flush.
// Related: fibkernel.go -- startupSweep, sweepStale, flushOneLocked, delListedLocked.

package fibkernel

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/report"
)

// deleteLogBackend is a rich backend that records every rich delete, in
// order, with the table it named. Its constructor supplies the plain backend;
// its zero value is invalid.
type deleteLogBackend struct {
	*mockBackend
	deletes []installedRoute
}

func (backend *deleteLogBackend) addRichRoute(RichRoute) error     { return nil }
func (backend *deleteLogBackend) replaceRichRoute(RichRoute) error { return nil }

func (backend *deleteLogBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error { //nolint:unparam // richRouteBackend method: the interface returns an error, this double never refuses
	backend.deletes = append(backend.deletes, installedRoute{prefix: prefix.String(), table: tableID})
	return nil
}

func sweepIssues() []report.Issue {
	var issues []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == reportCodeFIBSweepIncomplete && issue.Subject == "sweep" {
			issues = append(issues, issue)
		}
	}
	return issues
}

// Goal: a prefix startup listed in two tables, which this run then installed
// in one of them, is deleted from each table exactly once by the flush.
// Method: the listing holds the prefix in main and in table 100, a change
// installs it in main through processEvent, and flushRoutes runs. The described
// delete removes main; the listed-table delete MUST skip main and remove table
// 100. Without the skip, main is deleted twice, and a second delete of a key
// with no priority removes whichever Ze route the kernel matches next.
func TestFIBKernelFlushDeletesEachListedTableOnce(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := "198.18.107.0/24"
	backend := &deleteLogBackend{mockBackend: newMockBackend()}
	backend.zeRoutes = []installedRoute{{prefix: prefix}, {prefix: prefix, table: 100}}
	fib := newFIBKernel(backend)
	fib.startupSweep()
	backend.zeRoutes = nil
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix(prefix),
		Interface: "ze-scope0", RouteType: sysribevents.RouteTypeUnicast, Protocol: "static",
	}}))
	if _, installed := fib.installed[prefix]; !installed {
		t.Fatalf("control route %s was not installed", prefix)
	}

	fib.flushRoutes()

	want := []installedRoute{{prefix: prefix}, {prefix: prefix, table: 100}}
	if !slices.Equal(backend.deletes, want) {
		t.Errorf("flush deleted %+v, want %+v: main once through the installed change, then table 100", backend.deletes, want)
	}
	if issues := flushIssues(); len(issues) != 0 {
		t.Errorf("a complete flush raised flush failures: %+v", issues)
	}
}

// Goal: a backend that can name no table, asked by the flush to remove a route
// startup listed in a named table, refuses, and the operator sees the refusal
// with the prefix and the table. Method: a plain mock backend whose listing
// holds one Ze route in table 100, startupSweep, flushRoutes, then the report
// bus. No plain (main-table) delete may be made for it, because that would
// remove a different route or none.
func TestFIBKernelFlushReportsPlainBackendNamedTable(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := "198.18.108.0/24"
	backend := newMockBackend()
	backend.zeRoutes = []installedRoute{{prefix: prefix, table: 100}}
	fib := newFIBKernel(backend)
	fib.startupSweep()
	backend.zeRoutes = nil

	fib.flushRoutes()

	if len(backend.deleted) != 0 {
		t.Errorf("the plain backend made main-table deletes %v for a route listed in table 100", backend.deleted)
	}
	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("flush raised %d flush errors, want one: %+v", len(issues), issues)
	}
	if issues[0].Detail["failed"] != 1 {
		t.Errorf("failed = %v, want 1", issues[0].Detail["failed"])
	}
	if issues[0].Detail["first-failure"] != prefix {
		t.Errorf("first failure = %v, want %s", issues[0].Detail["first-failure"], prefix)
	}
	want := "route " + prefix + " in table 100: this backend removes main-table routes only"
	if issues[0].Detail["first-error"] != want {
		t.Errorf("first error = %v, want %q", issues[0].Detail["first-error"], want)
	}
}

// Goal: a startup listing that fails reaches the operator, because nothing is
// then marked stale and no route a crashed run left is ever swept. Method: a
// backend whose listing fails, one startupSweep, then the report bus: one
// sweep error carrying the listing's error, the remedy, and no stale count.
func TestFIBKernelStartupReportsUnlistedKernel(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &listFailingBackend{mockBackend: newMockBackend(), listErr: errors.New("route list: permission denied")}
	fib := newFIBKernel(backend)

	if stale := fib.startupSweep(); stale != nil {
		t.Fatalf("a failed listing answered stale routes %v", stale)
	}

	issues := sweepIssues()
	if len(issues) != 1 {
		t.Fatalf("startup raised %d sweep errors, want one: %+v", len(issues), issues)
	}
	issue := issues[0]
	want := "FIB startup sweep could not list the kernel's Ze routes, so no stale route a previous run left " +
		"is removed and each keeps forwarding with no route behind it: route list: permission denied"
	if issue.Message != want {
		t.Errorf("message = %q, want %q", issue.Message, want)
	}
	if issue.Fix != sweepFix {
		t.Errorf("fix = %q, want %q", issue.Fix, sweepFix)
	}
	if issue.Detail[keyOperation] != "list" {
		t.Errorf("operation = %v, want list", issue.Detail[keyOperation])
	}
	if issue.Detail["list-error"] != "route list: permission denied" {
		t.Errorf("list error = %v, want the listing's error", issue.Detail["list-error"])
	}
	if _, counted := issue.Detail["stale"]; counted {
		t.Errorf("the report carries a stale count no listing produced: %+v", issue.Detail)
	}
}

// Goal: when several stale deletes fail, the sweep error counts them all and
// names the lowest prefix, so the report does not depend on map order.
// Method: eight stale prefixes, a backend that refuses every delete, one
// sweepStale. Map iteration is random, so a sweep that kept whichever failure
// came first names 198.18.110.0/24 only one run in eight.
func TestFIBKernelSweepNamesTheLowestFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	stale := map[string]string{}
	for _, prefix := range []string{
		"198.18.117.0/24", "198.18.113.0/24", "198.18.110.0/24", "198.18.116.0/24",
		"198.18.111.0/24", "198.18.115.0/24", "198.18.112.0/24", "198.18.114.0/24",
	} {
		stale[prefix] = ""
	}
	backend := &failingDeleteBackend{mockBackend: newMockBackend()}
	fib := newFIBKernel(backend)

	fib.sweepStale(stale)

	if backend.attempts != 8 {
		t.Fatalf("the sweep attempted %d deletes, want all eight", backend.attempts)
	}
	issues := sweepIssues()
	if len(issues) != 1 {
		t.Fatalf("the sweep raised %d sweep errors, want one: %+v", len(issues), issues)
	}
	if issues[0].Detail["failed"] != 8 {
		t.Errorf("failed = %v, want 8", issues[0].Detail["failed"])
	}
	if issues[0].Detail["first-failure"] != "198.18.110.0/24" {
		t.Errorf("first failure = %v, want the lowest prefix 198.18.110.0/24", issues[0].Detail["first-failure"])
	}
}

// Goal: a stale route the startup sweep could not delete is still Ze's to
// remove, so the stop flush tries it again and reports it when the kernel
// still refuses. Method: a backend that refuses every delete, one stale
// prefix no protocol refreshed, sweepStale, then flushRoutes: the delete is
// attempted a second time and the flush raises one error counting it.
func TestFIBKernelFlushRetriesARouteTheSweepCouldNotDelete(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &failingDeleteBackend{mockBackend: newMockBackend()}
	fib := newFIBKernel(backend)
	const prefix = "198.18.130.0/24"

	fib.sweepStale(map[string]string{prefix: ""})
	if backend.attempts != 1 {
		t.Fatalf("the sweep attempted %d deletes, want one", backend.attempts)
	}

	fib.flushRoutes()

	if backend.attempts != 2 {
		t.Fatalf("after a failed sweep the flush attempted %d deletes in all, want the stale route tried again", backend.attempts)
	}
	issues := flushIssues()
	if len(issues) != 1 {
		t.Fatalf("the flush raised %d flush errors, want one for the route still in the kernel: %+v", len(issues), issues)
	}
	if issues[0].Detail["failed"] != 1 || issues[0].Detail["first-failure"] != prefix {
		t.Errorf("flush detail = %+v, want one failed delete naming %s", issues[0].Detail, prefix)
	}
}

// refusingRichBackend is a rich backend that records every rich delete and
// refuses it, or, when onlyTable is set, refuses the deletes in that table
// alone. Its constructor supplies the plain backend; its zero value is invalid.
type refusingRichBackend struct {
	*mockBackend
	deletes   []installedRoute
	onlyTable uint32
}

func (backend *refusingRichBackend) addRichRoute(RichRoute) error     { return nil }
func (backend *refusingRichBackend) replaceRichRoute(RichRoute) error { return nil }

func (backend *refusingRichBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error {
	backend.deletes = append(backend.deletes, installedRoute{prefix: prefix.String(), table: tableID})
	if backend.onlyTable != 0 && tableID != backend.onlyTable {
		return nil
	}
	return errors.New("operation not permitted")
}

// Goal: a stale route in a named table that the sweep could not delete is
// retried by the stop flush in that same table. Method: a rich backend that
// refuses every delete and lists one Ze route in table 100, startupSweep,
// sweepStale, then flushRoutes: both delete attempts name table 100, and no
// plain main-table delete is made.
func TestFIBKernelFlushRetriesTheSweepsRefusedTable(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &refusingRichBackend{mockBackend: newMockBackend()}
	const prefix = "198.18.131.0/24"
	backend.zeRoutes = []installedRoute{{prefix: prefix, table: 100}}
	fib := newFIBKernel(backend)

	fib.sweepStale(fib.startupSweep())
	fib.flushRoutes()

	want := []installedRoute{{prefix: prefix, table: 100}, {prefix: prefix, table: 100}}
	if len(backend.deletes) != len(want) || backend.deletes[0] != want[0] || backend.deletes[1] != want[1] {
		t.Fatalf("rich deletes = %+v, want the sweep's attempt and the flush's retry, both in table 100", backend.deletes)
	}
	if len(backend.deleted) != 0 {
		t.Errorf("the flush made plain main-table deletes %v for a route listed in table 100", backend.deleted)
	}
}

// Goal: a refused stale table is still flushed when the prefix was installed
// again in another table and then withdrawn before stop. Method: a rich
// backend that refuses the deletes in table 100 only, startup listing the prefix in main and in
// table 100, an Add for it in main inside the sweep window, sweepStale (the
// table 100 delete is refused), a Withdraw, then flushRoutes: the flush tries
// table 100 again.
func TestFIBKernelFlushRetriesARefusedTableAfterAWithdraw(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	backend := &refusingRichBackend{mockBackend: newMockBackend(), onlyTable: 100}
	prefix := netip.MustParsePrefix("198.18.132.0/24")
	backend.zeRoutes = []installedRoute{{prefix: prefix.String(), table: 0}, {prefix: prefix.String(), table: 100}}
	fib := newFIBKernel(backend)
	stale := fib.startupSweep()

	fib.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("192.168.32.1"), Protocol: "bgp"},
	}))
	fib.sweepStale(stale)
	sweepAttempts := len(backend.deletes)
	if sweepAttempts != 1 || backend.deletes[0].table != 100 {
		t.Fatalf("control: the sweep's rich deletes = %+v, want one attempt in table 100", backend.deletes)
	}
	fib.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Withdraw, Prefix: prefix, Protocol: "bgp"},
	}))
	afterWithdraw := len(backend.deletes)
	if _, installed := fib.installed[prefix.String()]; installed {
		t.Fatalf("control: the withdraw left %s installed", prefix)
	}

	fib.flushRoutes()

	retried := false
	for _, del := range backend.deletes[afterWithdraw:] {
		if del.prefix == prefix.String() && del.table == 100 {
			retried = true
		}
	}
	if !retried {
		t.Fatalf("the flush did not try table 100 again after the withdraw; rich deletes = %+v", backend.deletes)
	}
}
