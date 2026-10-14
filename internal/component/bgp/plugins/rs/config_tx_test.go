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

func newQueueSizeTestTx(t *testing.T) *queueSizeConfigTx {
	t.Helper()
	wp := newWorkerPool(func(_ workerKey, _ workItem) {}, poolConfig{
		chanSize:    workerQueueSizeDefault,
		idleTimeout: 5 * time.Second,
	})
	t.Cleanup(wp.Stop)
	return newQueueSizeConfigTx(wp, false)
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
	tx := newQueueSizeTestTx(t)
	if err := tx.configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.1"); got != 8192 {
		t.Fatalf("after boot: capacity %d, want 8192", got)
	}

	if err := tx.verify(queueSizeSection("16384")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.2"); got != 8192 {
		t.Fatalf("verify applied the candidate: capacity %d, want 8192 until apply", got)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.3"); got != 16384 {
		t.Fatalf("after apply: capacity %d, want 16384", got)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.1"); got != 8192 {
		t.Fatalf("existing worker resized: capacity %d, want 8192 until it is recreated", got)
	}

	if err := tx.rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.4"); got != 8192 {
		t.Fatalf("after rollback: capacity %d, want 8192", got)
	}

	if err := tx.verify(queueSizeSection("")); err != nil {
		t.Fatalf("verify without the leaf: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply without the leaf: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.5"); got != workerQueueSizeDefault {
		t.Fatalf("leaf removed: capacity %d, want the default %d", got, workerQueueSizeDefault)
	}
}

// VALIDATES: a verify that carries no bgp section is a verified transaction
// that leaves the size as it is.
// PREVENTS: a commit of another root resetting the size to the default.
func TestQueueSizeConfigTxKeepsSizeWithoutBGPSection(t *testing.T) {
	tx := newQueueSizeTestTx(t)
	if err := tx.configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.verify(nil); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.1"); got != 8192 {
		t.Fatalf("capacity %d, want 8192", got)
	}
}

// VALIDATES: the env var wins over the leaf on reload as it does at boot.
// PREVENTS: a reload overriding the operator's emergency override.
func TestQueueSizeConfigTxEnvOverrideIgnoresReload(t *testing.T) {
	tx := newQueueSizeTestTx(t)
	tx.envOverride = true
	if err := tx.configure(queueSizeSection("8192")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.verify(queueSizeSection("16384")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := newWorkerCapacity(t, tx.workers, "10.0.0.1"); got != workerQueueSizeDefault {
		t.Fatalf("capacity %d, want the env-chosen %d", got, workerQueueSizeDefault)
	}
}

// VALIDATES: an apply with no verified candidate is refused.
// PREVENTS: answering OK for a commit the plugin never verified.
func TestQueueSizeConfigTxRefusesApplyWithoutVerify(t *testing.T) {
	tx := newQueueSizeTestTx(t)
	if err := tx.apply(); !errors.Is(err, errQueueSizeApplyUnverified) {
		t.Fatalf("apply without verify: %v, want errQueueSizeApplyUnverified", err)
	}
}
