// Design: docs/functional-tests.md -- the DUT's store
// Related: ../runner/record_dut_store.go -- option=storage and the DUTStoreKey selector
// Related: fixture.go -- childEnvironment, which replaces every spelling of a key

package fixture

import (
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// daemonStore is where a ze daemon a fixture starts keeps a store its root
// does not already hold. The zero value names no choice and is refused, so a
// fixture that builds a daemon environment always says which one it needs.
type daemonStore uint8

const (
	// daemonStoreUnspecified is no choice; daemonEnvironment refuses it.
	daemonStoreUnspecified daemonStore = iota
	// daemonStoreMemory keeps an absent store in the daemon's memory: for a
	// fixture that never reads what the daemon persisted, so its start does
	// not wait on the disk.
	daemonStoreMemory
	// daemonStoreDisk keeps the store on disk under the root: for a fixture
	// that seeds a store the daemon starts on, or reads one it persisted.
	daemonStoreDisk
)

// daemonEnvironment answers base for a ze child whose config root is root.
// Every spelling of ze.config.dir is replaced by root, so the seed and the
// start resolve one root whichever spelling base inherited: env reads a
// dotted spelling first, so an appended `ZE_CONFIG_DIR=` lost to the runner's
// dotted root. The store selector is set for store under every spelling too.
func daemonEnvironment(base []string, root string, store daemonStore) []string {
	switch store {
	case daemonStoreMemory:
		return childEnvironment(base, map[string]string{
			envConfigDirDotted: root,
			runner.DUTStoreKey: runner.DUTStoreMemory,
		})
	case daemonStoreDisk:
		return childEnvironment(withoutEnv(base, runner.DUTStoreKey), map[string]string{envConfigDirDotted: root})
	case daemonStoreUnspecified:
	}
	panic("BUG: daemonEnvironment needs daemonStoreMemory or daemonStoreDisk")
}

// daemonRootEnvironment answers base with root as the only spelling of
// ze.config.dir and the store selector left as base carries it: for a fixture
// whose daemon or CLI must resolve root, and which reads no store itself, so
// the .ci's option=storage still decides where an absent store lives.
func daemonRootEnvironment(base []string, root string) []string {
	return childEnvironment(base, map[string]string{envConfigDirDotted: root})
}

// daemonOverrides is daemonEnvironment(…, root, daemonStoreDisk) as overrides
// for a builder that takes a map (extra1Environment, miscEnvironment): the
// root under one spelling, and the selector emptied under every spelling, so
// the daemon's env.Get reads no memory selector and keeps its store on disk.
func daemonOverrides(root string) map[string]string {
	return map[string]string{envConfigDirDotted: root, runner.DUTStoreKey: ""}
}

// requireDaemonRoot checks that dir is the store a ze daemon started with
// environment keeps on disk.
//
// It refuses three ways, each a store the fixture would not share with the
// daemon: the environment selects the memory store (any spelling), so what the
// daemon creates dies with it; the environment names no root, so the daemon
// would resolve the binary-relative operator store (paths.DefaultConfigDir);
// or the root it names is another directory than dir.
func requireDaemonRoot(environment []string, dir string) error {
	if selected := env.LookupIn(environment, runner.DUTStoreKey); selected == runner.DUTStoreMemory {
		return fmt.Errorf("this fixture reads the ze store under %s, but the daemon's environment sets %s=%s, so a store the daemon creates lives in its memory and never reaches the disk: add option=storage:value=disk to the .ci", dir, runner.DUTStoreKey, runner.DUTStoreMemory)
	}
	root := env.LookupIn(environment, envConfigDirDotted)
	if root == "" {
		return fmt.Errorf("this fixture reads the ze store under %s, but the daemon's environment sets no %s, so the daemon would open the operator's store beside its binary: run the fixture under ./le test, or build its daemon environment with daemonEnvironment", dir, envConfigDirDotted)
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve the daemon root %s: %w", root, err)
	}
	dirPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve the fixture store %s: %w", dir, err)
	}
	if rootPath != dirPath {
		return fmt.Errorf("this fixture reads the ze store under %s, but the daemon's environment roots it at %s (%s), so the two would open different stores: open the daemon's root, or start the daemon with daemonEnvironment rooted here", dirPath, rootPath, envConfigDirDotted)
	}
	return nil
}

// openDaemonStore is storage.Open on the store a daemon started with
// environment keeps under dir. Caller MUST Close the result.
func openDaemonStore(environment []string, dir string) (storage.Storage, error) {
	if err := requireDaemonRoot(environment, dir); err != nil {
		return nil, err
	}
	return storage.Open(dir)
}

// openDaemonStoreReadOnly is storage.OpenReadOnly on that store.
// Caller MUST Close the result.
func openDaemonStoreReadOnly(environment []string, dir string) (storage.Storage, error) {
	if err := requireDaemonRoot(environment, dir); err != nil {
		return nil, err
	}
	return storage.OpenReadOnly(dir, dir)
}

// openDaemonTree is storage.OpenTree on that store's live tree.
// Caller MUST Close the result.
func openDaemonTree(environment []string, dir string, writable bool) (storage.Storage, error) {
	if err := requireDaemonRoot(environment, dir); err != nil {
		return nil, err
	}
	return storage.OpenTree(filepath.Join(dir, storageTreeName), writable)
}

// createDaemonStore is storage.Create of the store a daemon started with
// environment will open under dir. Caller MUST Close the result.
func createDaemonStore(environment []string, dir string) (storage.Storage, error) {
	if err := requireDaemonRoot(environment, dir); err != nil {
		return nil, err
	}
	return storage.Create(dir)
}

// daemonStoreCreator answers createDaemonStore bound to environment, for a
// helper that takes the creator of the tree it writes.
func daemonStoreCreator(environment []string) func(string) (storage.Storage, error) {
	return func(dir string) (storage.Storage, error) {
		return createDaemonStore(environment, dir)
	}
}

// createDaemonStorePopulated is storage.CreatePopulated of that store.
// Caller MUST Close the result.
func createDaemonStorePopulated(environment []string, dir string, populate func(storage.Storage) error) (storage.Storage, error) {
	if err := requireDaemonRoot(environment, dir); err != nil {
		return nil, err
	}
	return storage.CreatePopulated(dir, populate)
}
