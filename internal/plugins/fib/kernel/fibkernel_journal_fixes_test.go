// Design: docs/architecture/core-design.md -- fib-kernel journal defect regression tests
// Related: fibkernel.go -- ownsSlotLocked, processEvent, sweepPrefixLocked, flushRoutes
// Related: pending_retry.go -- runRetryLoop, retryDuePending, retryPendingAfterProgressLocked

package fibkernel

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

func hasSyncFailureWarning(prefix string) bool {
	for _, w := range report.Warnings() {
		if w.Code == reportCodeFIBSyncFailure && w.Subject == prefix {
			return true
		}
	}
	return false
}

func countSyncFailureErrors(prefix string) int {
	n := 0
	for _, e := range report.Errors(0) {
		if e.Code == reportCodeFIBSyncFailure && e.Subject == prefix {
			n++
		}
	}
	return n
}

// esrchSweepBackend lets a test specify which prefixes were already absent
// (ESRCH -> removed=false, err=nil) versus actually removed (true, nil).
type esrchSweepBackend struct {
	*mockBackend
	absent  map[installedRoute]bool
	deletes []installedRoute
}

func (b *esrchSweepBackend) addRichRoute(RichRoute) error     { return nil }
func (b *esrchSweepBackend) replaceRichRoute(RichRoute) error { return nil }

func (b *esrchSweepBackend) delRichRoute(prefix netip.Prefix, tableID uint32) (bool, error) { //nolint:unparam // richRouteBackend method: test double never fails
	key := installedRoute{prefix: prefix.String(), table: tableID}
	b.deletes = append(b.deletes, key)
	if b.absent[key] {
		return false, nil
	}
	return true, nil
}

// VALIDATES: startup sweep does not count ESRCH (removed=false, err=nil) as a
// removed orphan and raises no fib-orphan warning when every stale route was
// already gone.
// PREVENTS: False fib-orphan warnings when the kernel already flushed stale
// routes before sweepStale ran.
func TestFIBKernelSweepDoesNotCountESRCHAsRemovedOrphan(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	r1 := installedRoute{prefix: "198.18.190.0/24", table: 0}
	r2 := installedRoute{prefix: "198.18.191.0/24", table: 100}
	backend := &esrchSweepBackend{
		mockBackend: newMockBackend(),
		absent:      map[installedRoute]bool{r1: true, r2: true},
	}
	backend.zeRoutes = []installedRoute{r1, r2}

	fib := newFIBKernel(backend)
	fib.sweepStale(fib.startupSweep())

	if len(backend.deletes) != 2 {
		t.Fatalf("sweep attempted %d deletes, want 2", len(backend.deletes))
	}
	for _, w := range report.Warnings() {
		if w.Code == reportCodeFIBOrphan {
			t.Fatalf("sweep raised fib-orphan warning when every delete answered ESRCH: %+v", w)
		}
	}
}

// VALIDATES: startup sweep counts only routes actually removed by the kernel
// (removed=true) in fib-orphan's orphan_count, excluding ESRCH responses.
// PREVENTS: Inflated orphan counts mixing real removals with already-absent
// routes.
func TestFIBKernelSweepCountsOnlyDeletedOrphans(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	present := installedRoute{prefix: "198.18.192.0/24", table: 0}
	absent := installedRoute{prefix: "198.18.193.0/24", table: 100}
	backend := &esrchSweepBackend{
		mockBackend: newMockBackend(),
		absent:      map[installedRoute]bool{absent: true},
	}
	backend.zeRoutes = []installedRoute{present, absent}

	fib := newFIBKernel(backend)
	fib.sweepStale(fib.startupSweep())

	found := false
	for _, w := range report.Warnings() {
		if w.Code != reportCodeFIBOrphan {
			continue
		}
		found = true
		if got := w.Detail["orphan_count"]; got != 1 {
			t.Fatalf("fib-orphan orphan_count = %v, want 1 (excluding ESRCH)", got)
		}
	}
	if !found {
		t.Fatal("sweep did not raise fib-orphan warning for the one removed orphan")
	}
}

type tableMoveOp struct {
	kind   string
	prefix string
	table  uint32
}

// tableMoveBackend records rich add/replace/delete operations in order and
// supports per-table failure injection.
type tableMoveBackend struct {
	*mockBackend
	ops            []tableMoveOp
	failAddTable   map[uint32]error
	failDelTable   map[uint32]error
	failAddOnceErr error
}

func newTableMoveBackend() *tableMoveBackend {
	return &tableMoveBackend{
		mockBackend:  newMockBackend(),
		failAddTable: make(map[uint32]error),
		failDelTable: make(map[uint32]error),
	}
}

func (b *tableMoveBackend) addRichRoute(r RichRoute) error {
	b.ops = append(b.ops, tableMoveOp{kind: "add", prefix: r.Prefix.String(), table: r.TableID})
	if b.failAddOnceErr != nil {
		err := b.failAddOnceErr
		b.failAddOnceErr = nil
		return err
	}
	if err := b.failAddTable[r.TableID]; err != nil {
		return err
	}
	return nil
}

func (b *tableMoveBackend) replaceRichRoute(r RichRoute) error { //nolint:unparam // richRouteBackend method: test double never fails
	b.ops = append(b.ops, tableMoveOp{kind: "replace", prefix: r.Prefix.String(), table: r.TableID})
	return nil
}

func (b *tableMoveBackend) delRichRoute(prefix netip.Prefix, tableID uint32) (bool, error) {
	b.ops = append(b.ops, tableMoveOp{kind: "del", prefix: prefix.String(), table: tableID})
	if err := b.failDelTable[tableID]; err != nil {
		return false, err
	}
	return true, nil
}

// VALIDATES: Moving an installed prefix from table 100 to table 200 uses
// make-before-break ordering (exclusive add in table 200 first, then delete in
// table 100) for both VerbInstall and VerbReplace, updating forwarding state
// cleanly with no leftover superseded tables.
// PREVENTS: Leaking the old-table route or replacing a foreign route in the
// destination table during a table move.
func TestFIBKernelTableMoveDeletesOldTableRoute(t *testing.T) {
	for _, verb := range []struct {
		name   string
		action routeaction.Action
	}{
		{name: "install", action: routeaction.Add},
		{name: "replace", action: routeaction.Update},
	} {
		t.Run(verb.name, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)

			prefix := netip.MustParsePrefix("198.18.200.0/24")
			backend := newTableMoveBackend()
			fib := newFIBKernel(backend)

			fib.processEvent(makeSysribPayload([]incomingChange{{
				Action:   routeaction.Add,
				Prefix:   prefix,
				NextHop:  netip.MustParseAddr("192.0.2.1"),
				TableID:  100,
				Protocol: "bgp",
			}}))
			backend.ops = nil

			fib.processEvent(makeSysribPayload([]incomingChange{{
				Action:   verb.action,
				Prefix:   prefix,
				NextHop:  netip.MustParseAddr("192.0.2.2"),
				TableID:  200,
				Protocol: "bgp",
			}}))

			wantOps := []tableMoveOp{
				{kind: "add", prefix: prefix.String(), table: 200},
				{kind: "del", prefix: prefix.String(), table: 100},
			}
			if !slices.Equal(backend.ops, wantOps) {
				t.Fatalf("ops = %+v, want %+v (exclusive add in table 200 before delete in table 100)", backend.ops, wantOps)
			}
			if got := fib.forwarding[prefix.String()].TableID; got != 200 {
				t.Fatalf("forwarding.TableID = %d, want 200", got)
			}
			if len(fib.supersededTables) != 0 {
				t.Fatalf("supersededTables = %v, want empty", fib.supersededTables)
			}
			if hasSyncFailureWarning(prefix.String()) {
				t.Fatal("clean table move raised fib-sync-failure")
			}
		})
	}
}

// VALIDATES: When installing into the destination table fails during a table
// move, fib-kernel keeps the previous table's route installed and does not
// delete it, while parking the move in pendingChanges.
// PREVENTS: Blackholing traffic by deleting the old-table route when the new
// table rejected the install.
func TestFIBKernelTableMovePreservesOldRouteWhenNewTableAddFails(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.201.0/24")
	backend := newTableMoveBackend()
	backend.failAddTable[200] = errors.New("file exists")
	fib := newFIBKernel(backend)

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		TableID:  100,
		Protocol: "bgp",
	}}))
	backend.ops = nil

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Update,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.2"),
		TableID:  200,
		Protocol: "bgp",
	}}))

	wantOps := []tableMoveOp{{kind: "add", prefix: prefix.String(), table: 200}}
	if !slices.Equal(backend.ops, wantOps) {
		t.Fatalf("ops = %+v, want only the failed add in table 200 (old table 100 must not be deleted)", backend.ops)
	}
	if got := fib.installed[prefix.String()]; got != "192.0.2.1" {
		t.Fatalf("installed[%s] = %q, want preserved next hop 192.0.2.1", prefix, got)
	}
	if got := fib.forwarding[prefix.String()].TableID; got != 100 {
		t.Fatalf("forwarding.TableID = %d, want preserved table 100", got)
	}
	if _, pending := fib.pending[prefix.String()]; !pending {
		t.Fatal("failed table move did not record pending entry")
	}
	if got := fib.pendingChanges[prefix.String()].change.TableID; got != 200 {
		t.Fatalf("pendingChanges.TableID = %d, want 200", got)
	}
	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("failed table move did not raise fib-sync-failure warning")
	}
}

// VALIDATES: When a table move installs cleanly in the new table but deleting
// the old table fails, fib-kernel records the new table in forwarding, retains
// the old table in supersededTables, and removes it on retry or flushRoutes.
// PREVENTS: Leaking the old-table route after a transient delete refusal.
func TestFIBKernelTableMoveRetainsOldTableOnDeleteFailureForFlushAndRetry(t *testing.T) {
	t.Run("retry clears old table and warning", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		prefix := netip.MustParsePrefix("198.18.202.0/24")
		backend := newTableMoveBackend()
		backend.failDelTable[100] = errors.New("operation not permitted")
		fib := newFIBKernel(backend)

		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action:   routeaction.Add,
			Prefix:   prefix,
			NextHop:  netip.MustParseAddr("192.0.2.1"),
			TableID:  100,
			Protocol: "bgp",
		}}))
		backend.ops = nil

		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action:   routeaction.Update,
			Prefix:   prefix,
			NextHop:  netip.MustParseAddr("192.0.2.2"),
			TableID:  200,
			Protocol: "bgp",
		}}))

		if got := fib.forwarding[prefix.String()].TableID; got != 200 {
			t.Fatalf("forwarding.TableID = %d, want 200", got)
		}
		if got := fib.supersededTables[prefix.String()]; !slices.Equal(got, []uint32{100}) {
			t.Fatalf("supersededTables = %v, want [100]", got)
		}
		if !hasSyncFailureWarning(prefix.String()) {
			t.Fatal("old-table delete failure did not raise fib-sync-failure warning")
		}

		delete(backend.failDelTable, 100)
		backend.ops = nil
		fib.retryDuePending(time.Now().Add(time.Hour))

		wantRetryOps := []tableMoveOp{
			{kind: "replace", prefix: prefix.String(), table: 200},
			{kind: "del", prefix: prefix.String(), table: 100},
		}
		if !slices.Equal(backend.ops, wantRetryOps) {
			t.Fatalf("retry ops = %+v, want %+v", backend.ops, wantRetryOps)
		}
		if len(fib.supersededTables) != 0 {
			t.Fatalf("supersededTables after retry = %v, want empty", fib.supersededTables)
		}
		if len(fib.pending) != 0 || len(fib.pendingChanges) != 0 {
			t.Fatalf("pending state after retry = (%v, %v), want empty", fib.pending, fib.pendingChanges)
		}
		if hasSyncFailureWarning(prefix.String()) {
			t.Fatal("retry did not clear fib-sync-failure warning")
		}
	})

	t.Run("flush removes both new and retained old table", func(t *testing.T) {
		report.ResetForTest()
		t.Cleanup(report.ResetForTest)

		prefix := netip.MustParsePrefix("198.18.203.0/24")
		backend := newTableMoveBackend()
		backend.failDelTable[100] = errors.New("operation not permitted")
		fib := newFIBKernel(backend)

		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action:   routeaction.Add,
			Prefix:   prefix,
			NextHop:  netip.MustParseAddr("192.0.2.1"),
			TableID:  100,
			Protocol: "bgp",
		}}))
		fib.processEvent(makeSysribPayload([]incomingChange{{
			Action:   routeaction.Update,
			Prefix:   prefix,
			NextHop:  netip.MustParseAddr("192.0.2.2"),
			TableID:  200,
			Protocol: "bgp",
		}}))

		delete(backend.failDelTable, 100)
		backend.ops = nil
		fib.flushRoutes()

		wantFlushOps := []tableMoveOp{
			{kind: "del", prefix: prefix.String(), table: 200},
			{kind: "del", prefix: prefix.String(), table: 100},
		}
		if !slices.Equal(backend.ops, wantFlushOps) {
			t.Fatalf("flush ops = %+v, want %+v", backend.ops, wantFlushOps)
		}
		if len(fib.supersededTables) != 0 {
			t.Fatalf("flush left supersededTables = %v, want empty", fib.supersededTables)
		}
	})
}

// VALIDATES: A plain (non-rich) routeBackend refuses nonzero TableID on
// addChange, replaceChange, and delChange with an error naming the prefix and
// table, while still accepting main-table (TableID 0) operations.
// PREVENTS: Silently programming or deleting a main-table route when a change
// targeted a named routing table on a plain backend.
func TestFIBKernelPlainBackendRefusesNonzeroTableOnAddReplaceDelete(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.204.0/24")
	backend := newMockBackend()
	fib := newFIBKernel(backend)

	// 1. Add in table 100 refused.
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		TableID:  100,
		Protocol: "bgp",
	}}))
	if len(backend.added) != 0 {
		t.Fatalf("plain backend added main-table routes %v for table 100", backend.added)
	}
	addErr := fib.addChange(&incomingChange{
		Action:  routeaction.Add,
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.1"),
		TableID: 100,
	}, prefix.String(), nil)
	if addErr == nil || !strings.Contains(addErr.Error(), "table 100") || !strings.Contains(addErr.Error(), prefix.String()) {
		t.Fatalf("addChange(table 100) err = %v, want error naming prefix and table 100", addErr)
	}

	// 2. Replace in table 100 refused even when slot is owned.
	fib.mu.Lock()
	fib.installed[prefix.String()] = "192.0.2.1"
	fib.forwarding[prefix.String()] = incomingChange{Prefix: prefix, TableID: 100}
	repErr := fib.replaceChange(&incomingChange{
		Action:  routeaction.Update,
		Prefix:  prefix,
		NextHop: netip.MustParseAddr("192.0.2.2"),
		TableID: 100,
	}, prefix.String(), nil)
	fib.mu.Unlock()
	if repErr == nil || !strings.Contains(repErr.Error(), "table 100") || !strings.Contains(repErr.Error(), prefix.String()) {
		t.Fatalf("replaceChange(table 100) err = %v, want error naming prefix and table 100", repErr)
	}
	if len(backend.replaced) != 0 {
		t.Fatalf("plain backend replaced main-table routes %v for table 100", backend.replaced)
	}

	// 3. Delete in table 100 refused.
	delErr := fib.delChange(&incomingChange{
		Action:  routeaction.Withdraw,
		Prefix:  prefix,
		TableID: 100,
	}, prefix.String(), nil)
	if delErr == nil || !strings.Contains(delErr.Error(), "table 100") || !strings.Contains(delErr.Error(), prefix.String()) {
		t.Fatalf("delChange(table 100) err = %v, want error naming prefix and table 100", delErr)
	}
	if len(backend.deleted) != 0 {
		t.Fatalf("plain backend deleted main-table routes %v for table 100", backend.deleted)
	}

	// Control: TableID 0 succeeds on add, replace, and delete.
	clear(fib.installed)
	clear(fib.forwarding)
	fib.processEvent(makeSysribPayload([]incomingChange{
		{Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("192.0.2.1"), Protocol: "bgp"},
		{Action: routeaction.Update, Prefix: prefix, NextHop: netip.MustParseAddr("192.0.2.2"), Protocol: "bgp"},
		{Action: routeaction.Withdraw, Prefix: prefix, Protocol: "bgp"},
	}))
	if backend.added[prefix.String()] != "192.0.2.1" || backend.replaced[prefix.String()] != "192.0.2.2" || len(backend.deleted) != 1 {
		t.Fatalf("main-table control failed: added=%v replaced=%v deleted=%v", backend.added, backend.replaced, backend.deleted)
	}
}

// retrySignalBackend coordinates deterministic retry-loop unit tests via
// channels instead of wall-clock sleeps.
type retrySignalBackend struct {
	*mockBackend
	mu        sync.Mutex
	failAdd   bool
	addCalls  int
	onAttempt chan int
	installed chan string
}

func newRetrySignalBackend(failAdd bool) *retrySignalBackend {
	return &retrySignalBackend{
		mockBackend: newMockBackend(),
		failAdd:     failAdd,
		onAttempt:   make(chan int, 32),
		installed:   make(chan string, 16),
	}
}

func (b *retrySignalBackend) setFailAdd(fail bool) {
	b.mu.Lock()
	b.failAdd = fail
	b.mu.Unlock()
}

func (b *retrySignalBackend) addRoute(prefix, nextHop string) error {
	b.mu.Lock()
	b.addCalls++
	calls := b.addCalls
	fail := b.failAdd
	b.mu.Unlock()
	select {
	case b.onAttempt <- calls:
	default:
	}
	if fail {
		return errors.New("network is unreachable")
	}
	if err := b.mockBackend.addRoute(prefix, nextHop); err != nil {
		return err
	}
	select {
	case b.installed <- prefix:
	default:
	}
	return nil
}

// VALIDATES: A failed FIB install raises fib-sync-failure as both error and
// warning (degrading the fib health probe), retries via runRetryLoop once the
// backend recovers, installs the route, and clears fib-sync-failure.
// PREVENTS: Routes permanently missing after transient ENETUNREACH or a stuck
// degraded fib health probe after recovery.
func TestFIBKernelPendingInstallRetriesAndClearsSyncFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.210.0/24")
	backend := newRetrySignalBackend(true)
	fib := newFIBKernel(backend)
	fib.retryBaseDelay = 5 * time.Millisecond
	fib.retryMaxDelay = 20 * time.Millisecond

	probe := report.HealthProbeDegraded(reportCodeFIBSyncFailure)

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Protocol: "bgp",
	}}))

	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("failed install did not raise fib-sync-failure warning")
	}
	if status, _ := probe(); status != health.StatusDegraded {
		t.Fatalf("health probe status = %v, want StatusDegraded while sync failure is active", status)
	}

	ctx, cancel := context.WithCancel(t.Context())
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		fib.runRetryLoop(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-loopDone
	})

	backend.setFailAdd(false)

	select {
	case got := <-backend.installed:
		if got != prefix.String() {
			t.Fatalf("retried install installed %q, want %s", got, prefix)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runRetryLoop did not retry and install the pending route")
	}

	cancel()
	<-loopDone

	fib.mu.RLock()
	installedNH := fib.installed[prefix.String()]
	pendingCount := len(fib.pending)
	pendingChangeCount := len(fib.pendingChanges)
	fib.mu.RUnlock()

	if installedNH != "192.0.2.1" {
		t.Fatalf("fib.installed[%s] = %q, want 192.0.2.1", prefix, installedNH)
	}
	if pendingCount != 0 || pendingChangeCount != 0 {
		t.Fatalf("pending state after retry = (%d, %d), want (0, 0)", pendingCount, pendingChangeCount)
	}
	if hasSyncFailureWarning(prefix.String()) {
		t.Fatal("fib-sync-failure warning not cleared after successful retry")
	}
	if status, _ := probe(); status != health.StatusHealthy {
		t.Fatalf("health probe status = %v, want StatusHealthy after retry cleared sync failure", status)
	}
}

// coverDependentBackend rejects dependentPrefix until coverPrefix is installed.
type coverDependentBackend struct {
	*mockBackend
	coverPrefix     string
	dependentPrefix string
}

func (b *coverDependentBackend) addRoute(prefix, nextHop string) error {
	b.mu.Lock()
	_, hasCover := b.added[b.coverPrefix]
	b.mu.Unlock()
	if prefix == b.dependentPrefix && !hasCover {
		return errors.New("network is unreachable")
	}
	return b.mockBackend.addRoute(prefix, nextHop)
}

// VALIDATES: When a dependent route failed earlier and its covering route
// arrives and installs cleanly in a subsequent batch, processEvent immediately
// retries the pending route in that same batch without waiting for the backoff
// timer, clearing pending state and fib-sync-failure.
// PREVENTS: Dependent routes sitting uninstalled for a backoff interval after
// their covering gateway route arrives.
func TestFIBKernelPendingInstallRetriedAfterCoveringRouteInSameBatch(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	cover := netip.MustParsePrefix("192.0.2.0/24")
	dependent := netip.MustParsePrefix("198.18.211.0/24")
	backend := &coverDependentBackend{
		mockBackend:     newMockBackend(),
		coverPrefix:     cover.String(),
		dependentPrefix: dependent.String(),
	}
	fib := newFIBKernel(backend)

	// Batch 1: dependent arrives before cover and fails.
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   dependent,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Protocol: "bgp",
	}}))
	if _, installed := fib.installed[dependent.String()]; installed {
		t.Fatal("control: dependent installed before its cover")
	}
	if !hasSyncFailureWarning(dependent.String()) {
		t.Fatal("control: failed dependent did not raise fib-sync-failure warning")
	}

	// Batch 2: cover installs cleanly; progress retry in the same processEvent
	// call installs the parked dependent immediately.
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   cover,
		NextHop:  netip.MustParseAddr("198.51.100.1"),
		Protocol: "ospf",
	}}))

	if got := fib.installed[cover.String()]; got != "198.51.100.1" {
		t.Fatalf("cover installed = %q, want 198.51.100.1", got)
	}
	if got := fib.installed[dependent.String()]; got != "192.0.2.1" {
		t.Fatalf("dependent not installed by progress retry: got %q, want 192.0.2.1", got)
	}
	if len(fib.pending) != 0 || len(fib.pendingChanges) != 0 {
		t.Fatalf("pending state not cleared: pending=%v pendingChanges=%v", fib.pending, fib.pendingChanges)
	}
	if hasSyncFailureWarning(dependent.String()) {
		t.Fatal("fib-sync-failure warning not cleared after progress retry installed dependent")
	}
}

// VALIDATES: Pending retries stop scheduling once retryMaxAttempts is reached,
// leaving pending and fib-sync-failure warning active without flooding the
// error ring with repeated RaiseError entries.
// PREVENTS: Unbounded retry loops and error-ring flooding on permanently
// unprogrammable routes.
func TestFIBKernelPendingRetryIsBoundedByMaxAttempts(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.212.0/24")
	backend := newRetrySignalBackend(true)
	fib := newFIBKernel(backend)
	fib.retryBaseDelay = 2 * time.Millisecond
	fib.retryMaxDelay = 4 * time.Millisecond
	fib.retryMaxAttempts = 3

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Protocol: "bgp",
	}}))
	// Consume the initial event-driven attempt notification.
	<-backend.onAttempt

	ctx, cancel := context.WithCancel(t.Context())
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		fib.runRetryLoop(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-loopDone
	})

	deadline := time.After(2 * time.Second)
	for retries := range fib.retryMaxAttempts {
		select {
		case <-backend.onAttempt:
		case <-deadline:
			t.Fatalf("timed out waiting for retry %d of %d", retries+1, fib.retryMaxAttempts)
		}
	}

	// retryDuePending holds f.mu across addRoute through writing the exhausted
	// entry back to f.pendingChanges, so once the final attempt has been
	// signaled, nextPendingRetry acquires f.mu.RLock only after exhaustion is
	// recorded.
	if _, hasNext := fib.nextPendingRetry(); hasNext {
		t.Fatal("nextPendingRetry still schedules a retry after retryMaxAttempts")
	}

	cancel()
	<-loopDone

	backend.mu.Lock()
	totalCalls := backend.addCalls
	backend.mu.Unlock()
	if totalCalls != 1+fib.retryMaxAttempts {
		t.Fatalf("total addRoute calls = %d, want %d (1 initial + %d retries)", totalCalls, 1+fib.retryMaxAttempts, fib.retryMaxAttempts)
	}
	if _, pending := fib.pending[prefix.String()]; !pending {
		t.Fatal("exhausted prefix was dropped from fib.pending")
	}
	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("exhausted prefix lost its fib-sync-failure warning")
	}
	if errs := countSyncFailureErrors(prefix.String()); errs != 1 {
		t.Fatalf("fib-sync-failure error count = %d, want 1 (timer retries must refresh warning only)", errs)
	}
}

// VALIDATES: Withdrawing a prefix that is parked in pendingChanges immediately
// cancels retry, clears pending state and fib-sync-failure warning, and never
// resurrects the withdrawn route on later retries or batch progress.
// PREVENTS: Zombie routes being installed by a pending retry after sysrib
// withdrew the prefix.
func TestFIBKernelWithdrawClearsPendingInstallAndSyncFailure(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	prefix := netip.MustParsePrefix("198.18.213.0/24")
	other := netip.MustParsePrefix("198.18.214.0/24")
	backend := &failingBackend{mockBackend: *newMockBackend(), failAdd: true}
	fib := newFIBKernel(backend)

	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Protocol: "bgp",
	}}))
	if _, parked := fib.pendingChanges[prefix.String()]; !parked {
		t.Fatal("control: failed add not parked in pendingChanges")
	}
	if !hasSyncFailureWarning(prefix.String()) {
		t.Fatal("control: failed add did not raise fib-sync-failure warning")
	}

	// Backend now accepts adds; withdraw arrives before retry runs.
	backend.failAdd = false
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Withdraw,
		Prefix:   prefix,
		Protocol: "bgp",
	}}))

	if len(fib.pending) != 0 || len(fib.pendingChanges) != 0 {
		t.Fatalf("withdraw left pending=%v pendingChanges=%v, want empty", fib.pending, fib.pendingChanges)
	}
	if hasSyncFailureWarning(prefix.String()) {
		t.Fatal("withdraw did not clear fib-sync-failure warning")
	}

	// Neither progress retry from another batch nor due retry may resurrect prefix.
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   other,
		NextHop:  netip.MustParseAddr("192.0.2.2"),
		Protocol: "bgp",
	}}))
	fib.retryDuePending(time.Now().Add(time.Hour))

	if _, resurrected := fib.installed[prefix.String()]; resurrected {
		t.Fatal("withdrawn prefix was resurrected into fib.installed")
	}
	if _, resurrected := backend.added[prefix.String()]; resurrected {
		t.Fatal("withdrawn prefix was resurrected in backend.added")
	}

	// Even when the backend refuses the delete on Withdraw, pendingChanges must
	// be cleared immediately so a stale Add is never retried.
	prefix2 := netip.MustParsePrefix("198.18.215.0/24")
	backend.failAdd = true
	backend.failDel = true
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Add,
		Prefix:   prefix2,
		NextHop:  netip.MustParseAddr("192.0.2.1"),
		Protocol: "bgp",
	}}))
	backend.failAdd = false
	fib.processEvent(makeSysribPayload([]incomingChange{{
		Action:   routeaction.Withdraw,
		Prefix:   prefix2,
		Protocol: "bgp",
	}}))
	if _, parked := fib.pendingChanges[prefix2.String()]; parked {
		t.Fatal("failed withdraw left stale Add parked in pendingChanges")
	}
}
