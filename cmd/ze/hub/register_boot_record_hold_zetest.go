//go:build zetest

// Design: docs/architecture/storage-backends.md -- the boot record
// Related: config_source.go -- recordBootConfig, which calls the hold

package hub

import (
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// testBootRecordHoldKey names how long a test DUT's boot record holds the
// store's write lock before it writes. A slow disk is what it stands for: there
// each of the record's syncs waits behind another writer, and the record holds
// the store's write lock across all of them, so every store write waits as
// long, and so does a read of a blob store. A read of the tree does not.
const testBootRecordHoldKey = "ze.test.boot.record.hold"

func init() {
	_ = env.MustRegister(env.EnvEntry{
		Key:         testBootRecordHoldKey,
		Type:        "duration",
		Description: "Test DUT builds only: how long the boot record holds the store's write lock before it writes, standing for a slow disk (test infrastructure)",
		Private:     true,
	})
	bootRecordHold = holdBootRecord
}

// holdBootRecord takes the store-wide write guard (an empty name is no
// configuration path, so the record's own gate does not apply) and keeps it for
// the configured duration. A guard it cannot take is logged and the hold is
// skipped, so the record then runs as it would in a shipped build.
func holdBootRecord(store storage.Storage) {
	hold := env.GetDuration(testBootRecordHoldKey, 0)
	if hold <= 0 {
		return
	}
	guard, err := store.AcquireLock("")
	if err != nil {
		slogutil.Logger("hub.config").Error("test boot record hold: store lock not taken", "error", err)
		return
	}
	time.Sleep(hold)
	if err := guard.Release(); err != nil {
		slogutil.Logger("hub.config").Error("test boot record hold: store lock release failed", "error", err)
	}
}
