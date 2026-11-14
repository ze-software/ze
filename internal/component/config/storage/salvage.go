// Design: docs/architecture/storage-backends.md -- a live salvage walk under the configuration guard.
// Related: backup.go -- Backup holds the same guard for its walk, and refuseStoreName judges both outputs.
// Related: store.go -- acquire and holdRuntime take the two locks every writer waits on.

package storage

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/pkg/zefs"
)

// HoldWrites runs walk while no writer can change the live tree behind source.
// One guard holds the configuration lock and the runtime-state lock from the
// start of walk to its return, so a key the daemon writes or removes meanwhile
// waits for the walk, and the walk reads one moment of the store, as a live
// backup does. walk receives the absolute path of the live tree (database/), so
// a walk over the frame files (zefs.CheckPath, zefs.RepairPath) reads the tree
// the daemon commits to. An unguarded read takes neither lock, so readers are
// not held.
//
// walk MUST NOT write through source: every write acquires a lock this guard
// holds, so it would wait for itself. A store that is not a live tree (a blob
// artifact) is refused: it has no frame files to walk.
func HoldWrites(source Storage, walk func(treePath string) error) (err error) {
	owned, ok := ownedStore(source)
	if !ok {
		return fmt.Errorf("hold writes: unsupported storage %T: %w", source, errors.ErrUnsupported)
	}
	if owned.tree == nil {
		return fmt.Errorf("hold writes: the store is a blob artifact, not a live tree: %w", errors.ErrUnsupported)
	}
	g, err := owned.acquire(false)
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(g, err) }()

	g.holdRuntime()
	return walk(owned.tree.root.Name())
}

// Salvage writes a salvage copy of the live tree behind source to path: a new
// tree holding every frame that reads, the salvage `ze data repair` writes
// (zefs.RepairPath), taken while the daemon keeps running. The walk opens one
// directory entry after another, so a key the daemon removed between the
// listing and the open failed it; under HoldWrites that removal waits for the
// walk instead. path is refused when it names something the store owns in its
// folder or lies inside the live tree, and zefs.RepairPath refuses a path that
// exists. The live store is not changed.
func Salvage(source Storage, path string) (*zefs.RepairReport, error) {
	owned, ok := ownedStore(source)
	if !ok {
		return nil, fmt.Errorf("repair: unsupported storage %T: %w", source, errors.ErrUnsupported)
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("repair %s: %w", path, err)
	}
	if err := refuseStoreName(owned, target, "repair"); err != nil {
		return nil, err
	}
	var report *zefs.RepairReport
	err = HoldWrites(source, func(tree string) error {
		var walkErr error
		report, walkErr = zefs.RepairPath(tree, target)
		return walkErr
	})
	if err != nil {
		return nil, err
	}
	return report, nil
}
