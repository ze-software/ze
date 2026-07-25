// Design: ai/rules/zefs-persistence.md -- runtime state persists in the managed
// zefs store; this is the CLI/filesystem-path opener for that store.
// Overview: main.go -- daemon startup, which registers the returned store with
// internal/core/statestore before any plugin configures.

package hub

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/pkg/zefs"
)

// errNoConfigDirForStateStore reports that no config directory was pinned, so a
// state-only store has no home. Callers treat this as "persistence stays a
// best-effort no-op", not as a startup failure.
var errNoConfigDirForStateStore = errors.New("no config directory pinned (ze.config.dir unset)")

// stateStoreName is the zefs file that holds runtime state on the CLI/filesystem
// path. It matches the appliance's config store name so an operator who later
// switches to blob storage keeps one file, not two.
const stateStoreName = "database.zefs"

// openStateOnlyStore opens (or creates) {configDir}/database.zefs purely as a
// runtime-STATE store for internal/core/statestore, on the CLI/filesystem path
// where config is NOT in a blob.
//
// It deliberately does NOT go through internal/core/resolve.Storage or
// storage.NewBlob:
//
//   - resolve.Storage answers "which backend holds my CONFIG" and returns a
//     filesystem backend whenever ze.storage.blob=false. Routing state through it
//     made a config-backend toggle silently discard every runtime-state key (BFD
//     auth sequence, DDoS baselines, NTP last-sync, tc original-qdisc snapshots).
//   - storage.NewBlob migrates on-disk config files into the blob it creates. A
//     state store must never do that: it would shadow the config file on disk and
//     a SIGHUP reload would then re-read the stale migrated copy instead.
//
// configDir must be non-empty; the caller gates on an explicit ze.config.dir so a
// one-off `ze -` never creates or contends on a shared binary-relative store.
// The returned store is owned by the process for its lifetime.
func openStateOnlyStore(configDir string) (*zefs.BlobStore, error) {
	if configDir == "" {
		return nil, errNoConfigDirForStateStore
	}
	path := filepath.Join(configDir, stateStoreName)
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(configDir, 0o700); err != nil {
			return nil, fmt.Errorf("create state store directory %s: %w", configDir, err)
		}
		bs, createErr := zefs.Create(path)
		if createErr != nil {
			return nil, fmt.Errorf("create state store %s: %w", path, createErr)
		}
		return bs, nil
	}
	bs, err := zefs.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open state store %s: %w", path, err)
	}
	return bs, nil
}
