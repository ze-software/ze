// Design: docs/architecture/core-design.md — route server plugin
// Related: server.go — RunRouteServer registers this transaction
// Related: worker.go — setChanSize, the pool setting this transaction drives

package rs

import (
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// workerQueueSizeDefault is the per-source-peer worker channel capacity when
// neither the env var nor the YANG leaf names one. It matches the leaf's YANG
// default and the env entry's registered default.
const workerQueueSizeDefault = 4096

// newQueueSizeConfigTx returns bgp-rs's side of the config transaction for
// bgp { route-server { worker-queue-size } }, on the SDK's one transaction
// helper: Stage 2 installs the size at once, a reload's verify holds it, apply
// installs it and keeps the replaced size, and rollback puts that back.
//
// The size reaches the pool through setChanSize, which governs workers created
// afterwards. A worker that already exists keeps its channel until it idles
// out (poolConfig.idleTimeout) and is recreated.
//
// The ze.bgp.route-server.worker-queue-size env var wins over the leaf at boot
// and on every reload: with envOverride set, no install changes the size.
func newQueueSizeConfigTx(workers *workerPool, envOverride bool) *sdk.ConfigTx[int] {
	workers.mu.Lock()
	boot := workers.cfg.chanSize
	workers.mu.Unlock()
	return sdk.NewConfigTx(boot, parseQueueSizeSections, func(size int) error {
		if envOverride {
			logger().Info("worker-queue-size from env, config value ignored", "config-size", size)
			return nil
		}
		workers.setChanSize(size)
		logger().Info("worker-queue-size applied", "size", size, "scope", "workers created from now on")
		return nil
	})
}

// parseQueueSizeSections reads the size a delivery names. A delivery without
// the bgp root is not present, so a commit of another root changes nothing; a
// bgp tree without the leaf names the default, so removing the leaf restores
// it.
func parseQueueSizeSections(sections []sdk.ConfigSection) (int, bool, error) {
	for _, section := range sections {
		if section.Root == configRootBGP {
			return queueSizeOrDefault(parseWorkerQueueSize(section.Data)), true, nil
		}
	}
	return 0, false, nil
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
