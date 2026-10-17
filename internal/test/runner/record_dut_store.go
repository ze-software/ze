// Design: docs/functional-tests.md -- the DUT's store
// Related: record_parse_vocabulary.go -- recordOptionArms, where option=storage is declared
// Related: runner_exec.go -- runTest and runOrchestrated, which hand the choice to ze

package runner

import "errors"

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

// dutStoreEnvMemory is what the DUT build reads (cmd/ze/register_store_zetest.go,
// ze.test.storage) to create an absent store in process memory. A store the
// test seeded on disk is opened, never replaced, whatever this says.
const dutStoreEnvMemory = "ze.test.storage=memory"

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
