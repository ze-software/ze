package rs

import (
	"errors"
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// queueSizeSection builds a bgp config delivery. An empty size leaves the
// route-server container out, the shape a config without the leaf delivers.
func queueSizeSection(size string) []sdk.ConfigSection {
	data := `{"bgp":{"peer":{}}}`
	if size != "" {
		data = `{"bgp":{"route-server":{"worker-queue-size":"` + size + `"}}}`
	}
	return []sdk.ConfigSection{{Root: configRootBGP, Data: data}}
}

// newQueueSizeTestTx builds a worker pool at the default size and the
// transaction that drives it.
func newQueueSizeTestTx(t *testing.T, envOverride bool) (*workerPool, *sdk.ConfigTx[int]) {
	t.Helper()
	wp := newWorkerPool(func(_ workerKey, _ workItem) {}, poolConfig{
		chanSize:    workerQueueSizeDefault,
		idleTimeout: 5 * time.Second,
	})
	t.Cleanup(wp.Stop)
	return wp, newQueueSizeConfigTx(wp, envOverride)
}

// newWorkerCapacity creates a worker for a fresh source peer and answers the
// capacity of its channel: the size a worker built now receives.
func newWorkerCapacity(t *testing.T, wp *workerPool, source string) int {
	t.Helper()
	key := workerKey{sourcePeer: source}
	if !wp.Dispatch(key, workItem{sourcePeer: source}) {
		t.Fatalf("dispatch to %s refused", source)
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()
	w := wp.workers[key]
	if w == nil {
		t.Fatalf("no worker for %s", source)
	}
	return cap(w.ch)
}

// VALIDATES: a reload of worker-queue-size reaches the worker pool. Verify holds
// the candidate without applying it, apply gives workers built afterwards the
// new channel capacity, rollback puts the replaced size back, and a verify of a
// bgp tree without the leaf restores the default.
// PREVENTS: the acked-and-dropped reload: with OnConfigure alone the SDK
// answered config-apply OK and the workers kept the boot size until a restart.
func TestQueueSizeConfigTxAppliesAReloadedSize(t *testing.T) {
	wp, tx := newQueueSizeTestTx(t, false)
	if err := tx.Configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.1"); got != 8192 {
		t.Fatalf("after boot: capacity %d, want 8192", got)
	}

	if err := tx.Verify(queueSizeSection("16384")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.2"); got != 8192 {
		t.Fatalf("verify applied the candidate: capacity %d, want 8192 until apply", got)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.3"); got != 16384 {
		t.Fatalf("after apply: capacity %d, want 16384", got)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.1"); got != 8192 {
		t.Fatalf("existing worker resized: capacity %d, want 8192 until it is recreated", got)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.4"); got != 8192 {
		t.Fatalf("after rollback: capacity %d, want 8192", got)
	}

	if err := tx.Verify(queueSizeSection("")); err != nil {
		t.Fatalf("verify without the leaf: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply without the leaf: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.5"); got != workerQueueSizeDefault {
		t.Fatalf("leaf removed: capacity %d, want the default %d", got, workerQueueSizeDefault)
	}
}

// VALIDATES: a verify that carries no bgp section is a verified transaction
// that leaves the size as it is.
// PREVENTS: a commit of another root resetting the size to the default.
func TestQueueSizeConfigTxKeepsSizeWithoutBGPSection(t *testing.T) {
	wp, tx := newQueueSizeTestTx(t, false)
	if err := tx.Configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.1"); got != 8192 {
		t.Fatalf("capacity %d, want 8192", got)
	}
}

// VALIDATES: AC-5. A bgp reload that changes only another subtree (a peer
// description) installs nothing: the pool size an operator changed by hand
// after the last install survives verify, apply and rollback.
// PREVENTS: an unrelated bgp edit reinstalling the size, which would undo
// state the transaction does not own and restart nothing it should not.
// METHOD: the pool's size is moved out of band after boot, so any install of
// the unchanged 8192 is visible as the size snapping back.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	wp, tx := newQueueSizeTestTx(t, false)
	running := `{"bgp":{"route-server":{"worker-queue-size":"8192"},"peer":{"a":{"description":"one"}}}}`
	candidate := `{"bgp":{"route-server":{"worker-queue-size":"8192"},"peer":{"a":{"description":"two"}}}}`
	if err := tx.Configure([]sdk.ConfigSection{{Root: configRootBGP, Data: running}}); err != nil {
		t.Fatalf("configure: %v", err)
	}
	wp.setChanSize(1234)

	if err := tx.Verify([]sdk.ConfigSection{{Root: configRootBGP, Data: candidate}}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.1"); got != 1234 {
		t.Fatalf("after apply: capacity %d, want 1234 (an unchanged subtree installed the size)", got)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.2"); got != 1234 {
		t.Fatalf("after rollback: capacity %d, want 1234", got)
	}
}

// VALIDATES: the env var wins over the leaf on reload as it does at boot.
// PREVENTS: a reload overriding the operator's emergency override.
func TestQueueSizeConfigTxEnvOverrideIgnoresReload(t *testing.T) {
	wp, tx := newQueueSizeTestTx(t, true)
	if err := tx.Configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(queueSizeSection("16384")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, wp, "10.0.0.1"); got != workerQueueSizeDefault {
		t.Fatalf("capacity %d, want the env-chosen %d", got, workerQueueSizeDefault)
	}
}

// VALIDATES: an apply with no verified candidate is refused.
// PREVENTS: answering OK for a commit the plugin never verified.
func TestQueueSizeConfigTxRefusesApplyWithoutVerify(t *testing.T) {
	_, tx := newQueueSizeTestTx(t, false)
	if err := tx.Apply(); !errors.Is(err, sdk.ErrConfigApplyUnverified) {
		t.Fatalf("apply without verify: %v, want sdk.ErrConfigApplyUnverified", err)
	}
}
