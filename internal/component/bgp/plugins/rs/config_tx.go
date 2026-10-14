// Design: docs/architecture/core-design.md — route server plugin
// Related: server.go — RunRouteServer registers the four config callbacks
// Related: worker.go — setChanSize, the pool setting this transaction drives

package rs

import (
	"errors"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// workerQueueSizeDefault is the per-source-peer worker channel capacity when
// neither the env var nor the YANG leaf names one. It matches the leaf's YANG
// default and the env entry's registered default.
const workerQueueSizeDefault = 4096

// errQueueSizeApplyUnverified refuses a config-apply that no accepted
// config-verify preceded.
var errQueueSizeApplyUnverified = errors.New("bgp-rs: config apply without a verified candidate")

// queueSizeConfigTx is bgp-rs's side of the config transaction for
// bgp { route-server { worker-queue-size } }: the size the pool runs with,
// the verified candidate, and what the last apply replaced, so a rollback can
// put it back.
//
// The size reaches the pool through setChanSize, which governs workers created
// afterwards. A worker that already exists keeps its channel until it idles
// out (poolConfig.idleTimeout) and is recreated.
//
// The ze.bgp.route-server.worker-queue-size env var wins over the leaf at boot
// and on every reload: with envOverride set, no delivery changes the size.
//
// Not safe for concurrent use. The SDK serializes the configure, verify, apply
// and rollback callbacks that call it.
type queueSizeConfigTx struct {
	workers     *workerPool
	envOverride bool

	current  int
	pending  int
	previous int
	// hasPending is set by a verify whose sections carried the bgp root. A
	// verify of other roots only is still verified and changes nothing here.
	hasPending bool
	// hasPrevious is set by an apply that changed the size, so the rollback
	// of that transaction knows there is a size to put back.
	hasPrevious bool
	// verified is set by a verify that accepted the candidate and cleared by
	// the apply or rollback that consumes it.
	verified bool
}

// newQueueSizeConfigTx returns the transaction for workers. envOverride is
// whether the env var names the size, which then outranks every delivery.
func newQueueSizeConfigTx(workers *workerPool, envOverride bool) *queueSizeConfigTx {
	workers.mu.Lock()
	current := workers.cfg.chanSize
	workers.mu.Unlock()
	return &queueSizeConfigTx{workers: workers, envOverride: envOverride, current: current}
}

// configure is the boot delivery (Stage 2). A leaf it carries sets the size at
// once; a bgp tree without the leaf leaves the boot size as it is.
func (tx *queueSizeConfigTx) configure(sections []sdk.ConfigSection) error {
	if tx.envOverride {
		return nil
	}
	data, ok := bgpSectionData(sections)
	if !ok {
		return nil
	}
	size := parseWorkerQueueSize(data)
	if size == 0 {
		return nil
	}
	tx.workers.setChanSize(size)
	tx.current = size
	logger().Info("worker-queue-size from config", "size", size)
	return nil
}

// verify holds a reload candidate for apply. A bgp tree without the leaf names
// the default, so removing the leaf restores it.
func (tx *queueSizeConfigTx) verify(sections []sdk.ConfigSection) error {
	tx.pending, tx.hasPending, tx.hasPrevious, tx.verified = 0, false, false, false
	data, ok := bgpSectionData(sections)
	if ok {
		tx.pending, tx.hasPending = queueSizeOrDefault(parseWorkerQueueSize(data)), true
	}
	tx.verified = true
	return nil
}

// apply makes the verified candidate the pool's size for new workers. An apply
// with no verified candidate is refused: answering OK would report a commit
// the plugin never took.
func (tx *queueSizeConfigTx) apply() error {
	if !tx.verified {
		return errQueueSizeApplyUnverified
	}
	next, hasNext := tx.pending, tx.hasPending
	tx.pending, tx.hasPending, tx.verified = 0, false, false
	if !hasNext {
		return nil
	}
	if tx.envOverride {
		logger().Info("worker-queue-size from env, config value ignored", "config-size", next)
		return nil
	}
	tx.workers.setChanSize(next)
	tx.previous, tx.hasPrevious, tx.current = tx.current, true, next
	logger().Info("worker-queue-size applied", "size", next, "scope", "workers created from now on")
	return nil
}

// rollback puts back the size the last apply replaced, when this transaction
// applied one.
func (tx *queueSizeConfigTx) rollback() error {
	tx.pending, tx.hasPending, tx.verified = 0, false, false
	if !tx.hasPrevious {
		return nil
	}
	tx.workers.setChanSize(tx.previous)
	tx.current, tx.hasPrevious = tx.previous, false
	logger().Info("worker-queue-size rolled back", "size", tx.current)
	return nil
}

// bgpSectionData answers the JSON of the bgp section, and whether the
// delivery carried one.
func bgpSectionData(sections []sdk.ConfigSection) (string, bool) {
	for _, section := range sections {
		if section.Root == configRootBGP {
			return section.Data, true
		}
	}
	return "", false
}

// queueSizeOrDefault maps parseWorkerQueueSize's 0 to the default. The engine
// enforces the leaf's YANG range (1..1000000) before delivery, so 0 here means
// the leaf is absent from the tree.
func queueSizeOrDefault(size int) int {
	if size == 0 {
		return workerQueueSizeDefault
	}
	return size
}
