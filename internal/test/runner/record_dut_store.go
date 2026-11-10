// Design: docs/functional-tests.md -- the DUT's store
// Related: record_parse_vocabulary.go -- recordOptionArms, where option=storage is declared
// Related: runner_exec.go -- runTest and runOrchestrated, which hand the choice to ze

package runner

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

// dutStore is the config store a ze DUT creates when its work directory holds
// none. The zero value is the default a .ci gets by writing nothing, and it is
// named so the switch in env says what the default is.
type dutStore uint8

const (
	// dutStoreUnspecified is a .ci without option=storage: the DUT runs on
	// the in-memory store, the same as dutStoreMemory.
	dutStoreUnspecified dutStore = iota
	// dutStoreMemory is option=storage:value=memory, the default made explicit.
	dutStoreMemory
	// dutStoreDisk is option=storage:value=disk: the DUT creates the live
	// store on disk, for a test that restarts ze in the same work directory or
	// reads the store after the DUT.
	dutStoreDisk
)

// DUTStoreKey and DUTStoreMemory are the selector the DUT build reads
// (cmd/ze/register_store_zetest.go, testStorageKey) to create an absent store
// in process memory. A store the test seeded on disk is opened, never
// replaced, whatever this says. They are exported for the compiled fixtures,
// which refuse to read a store a daemon was told to keep in memory
// (internal/test/fixture/daemon_store.go).
const (
	DUTStoreKey    = "ze.test.storage"
	DUTStoreMemory = "memory"
)

// dutStoreEnvMemory is the environment entry that selects the memory store.
const dutStoreEnvMemory = DUTStoreKey + "=" + DUTStoreMemory

// errOptionStorageValue names the value a .ci wrote and the two that work.
var errOptionStorageValue = errors.New("option=storage takes value=disk (the live store on disk, for a test that restarts ze or reads its store) or value=memory (the default)")

// parseOptionStorage reads option=storage.
func parseOptionStorage(r *Record, _ string, kv map[string]string) error {
	switch kv[keyValue] {
	case "disk":
		r.DUTStore = dutStoreDisk
	case "memory":
		r.DUTStore = dutStoreMemory
	default:
		return errOptionStorageValue
	}
	return nil
}

// env is the environment entry that selects the store, empty for the disk.
// The in-memory store syncs nothing, so a DUT's start never waits on another
// writer's disk traffic (plan/journal/startup-wait-expires-under-load.md).
func (s dutStore) env() string {
	switch s {
	case dutStoreUnspecified, dutStoreMemory:
		return dutStoreEnvMemory
	case dutStoreDisk:
		return ""
	}
	panic("BUG: dutStore holds a value no constant names")
}

// dutConfigRootKey is the config root a ze child seeds and starts under
// (internal/core/paths, DefaultConfigDir).
const dutConfigRootKey = "ze.config.dir"

// dutEnv answers environ with a DUT's store choices applied, in order: the
// config root, the store selector, then the .ci's own option=env entries, so
// an explicit option=env still decides. runTest and runOrchestrated both call
// it, so every child of a test gets the same answer.
//
// Each entry replaces every earlier spelling of its key. env.Get reads a key
// case-insensitively with a dot and an underscore as one separator, and
// prefers a dotted spelling whatever the order, so an appended entry did not
// decide: an inherited ze.config.dir or the runner's own dotted root beat a
// .ci's `option=env:var=ZE_CONFIG_DIR`, while the same declaration spelled
// with dots won. Seed (`ze init`, DefaultConfigDir) and start then resolved a
// different root for one declaration, and a non-Go reader of ZE_CONFIG_DIR
// saw the operator's store.
func (r *Record) dutEnv(environ []string, root string) []string {
	environ = withEntry(environ, dutConfigRootKey+"="+root)
	if storeEnv := r.DUTStore.env(); storeEnv != "" {
		environ = withEntry(environ, storeEnv)
	}
	for _, kv := range r.EnvVars {
		environ = withEntry(environ, ExpandPorts(kv, r.Port))
	}
	return environ
}

// withEntry answers environ with entry as the only spelling of its key.
// It may reuse environ's backing array, as append does.
func withEntry(environ []string, entry string) []string {
	name, _, _ := strings.Cut(entry, "=")
	return append(env.Without(environ, name), entry)
}
