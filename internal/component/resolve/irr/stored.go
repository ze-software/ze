// Design: docs/architecture/resolve.md -- the stored delegation table
//
// Two sources answer a lookup: the seed the binary ships (seed.go) and the
// copy `update resolve rir` stores under the meta/rir/delegation key. Both
// carry the same format, so one parser reads both, and the newer of the two
// answers.
//
// Related: rir.go -- the table, the parser and the lookup
// Related: seed.go -- the embedded seed and its lazily parsed accessor
package irr

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// logger writes the two reports this file owes an operator: a stored copy
// passed over, and a managed store that exists and cannot be read.
var logger = slogutil.LazyLogger("resolve.irr")

// storeFileName is the managed store's file name inside the config directory.
// It is the same file filter_irr's cacheStorePath names.
const storeFileName = "database.zefs"

// delegationTable answers the table every lookup searches, and is what
// RegistryForASN reads.
//
// Nothing is cached here. The seed is parsed once, because embedded bytes
// never change, and the stored copy is read and parsed on every lookup. What
// that guarantees is that the answer after `update resolve rir` comes from the
// copy that refresh stored, with no restart to remember and no invalidation to
// get wrong. What it costs is one store read for each lookup, which an
// operator command can afford and a wire path could not.
func delegationTable() (*rirTable, error) {
	table, note := preferStoredDelegation(seedTable, storedDelegation)
	if table == nil {
		return nil, note
	}
	if note != nil {
		// The seed answers, so this is a report rather than a failure, and it
		// is owed: a stored copy is never half-used and never passed over in
		// silence (AC-9).
		logger().Warn("resolve/irr: the stored delegation table was passed over", "error", note)
	}
	return table, nil
}

// preferStoredDelegation answers the table a lookup reads: the stored copy
// when its generation date is strictly after the seed's, and the seed
// otherwise. An upgrade that ships fresher data than the last refresh stored
// therefore takes over on its own.
//
// It answers a table and a note together, and a caller MUST read both:
//   - a table and no note: the answer, whichever source produced it.
//   - a table and a note: the seed answers, and the note says why the stored
//     copy was passed over. A stored copy that does not parse is reported and
//     never half-used (AC-9).
//   - no table and a note: the shipped seed itself could not be read, and
//     nothing at all is known.
//
// Both sources are parameters so a test drives every branch without a zefs
// store and without a rebuilt binary.
func preferStoredDelegation(seed func() (*rirTable, error), stored func() ([]byte, bool)) (*rirTable, error) {
	table, err := seed()
	if err != nil {
		return nil, err
	}

	blob, held := stored()
	if !held {
		return table, nil
	}

	refreshed, err := parseRIRTable(bytes.NewReader(blob))
	if err != nil {
		return table, fmt.Errorf("the stored delegation table cannot be read: %w", err)
	}
	if refreshed.generated.After(table.generated) {
		return refreshed, nil
	}
	return table, nil
}

// storedDelegation answers the delegation table a refresh stored, and whether
// one is stored at all.
//
// Two processes read it and each reads it its own way. Inside the daemon a
// state store is registered, and statestore.Get reads the config system's OWN
// handle: a second zefs handle in that process would make the config store's
// next flush re-encode from a stale tree and DROP every state key
// (internal/core/statestore package doc). Where no store is registered, which
// is the host CLI and any plugin process, database.zefs is opened READ-ONLY
// if it exists.
//
// Neither path ever writes that file. The refresh writes, in the daemon, and
// through statestore.Put alone.
func storedDelegation() ([]byte, bool) {
	if statestore.Store() != nil {
		return statestore.Get(zefs.KeyRIRDelegation.Pattern)
	}
	return storedDelegationFile(storePath())
}

// storePath answers the managed store's file, or empty when no config
// directory is known.
func storePath() string {
	dir := paths.DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, storeFileName)
}

// storedDelegationFile reads the delegation key out of the managed store on
// disk, for a process that has none registered.
//
// The read is transient and read-only: the store is opened, the one key is
// read, and the handle is closed. A file that is absent answers "nothing
// stored", and the shipped seed then answers the lookup. A file that exists
// and cannot be read answers the same, and says so, because a corrupt or
// half-written store is not evidence that nobody refreshed (R-4).
func storedDelegationFile(path string) ([]byte, bool) {
	if path == "" {
		return nil, false
	}
	if _, err := os.Stat(path); err != nil {
		return nil, false
	}

	store, err := zefs.Open(path)
	if err != nil {
		logger().Warn("resolve/irr: the managed store cannot be opened", "path", path, "error", err)
		return nil, false
	}
	defer func() { _ = store.Close() }()

	if !store.Has(zefs.KeyRIRDelegation.Pattern) {
		return nil, false
	}

	blob, err := store.ReadFile(zefs.KeyRIRDelegation.Pattern)
	if err != nil {
		logger().Warn("resolve/irr: the stored delegation table cannot be read", "path", path, "error", err)
		return nil, false
	}
	return blob, true
}
