// Related: stored.go -- the precedence and the two read paths these tests drive
package irr

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// delegationFile answers the bytes of a one-range delegation table generated
// on the given date. The date is what the precedence compares and the token
// is how a test tells the two tables apart in an answer.
func delegationFile(date, token string) string {
	return "# Generated: " + date + "\n1 10 " + token + "\n"
}

// tableFrom parses a delegation table a test wrote, and fails the test when it
// does not parse: a malformed fixture would otherwise be read as the behavior
// under test.
func tableFrom(t *testing.T, body string) *rirTable {
	t.Helper()

	table, err := parseRIRTable(strings.NewReader(body))
	if err != nil {
		t.Fatalf("the test fixture does not parse: %v", err)
	}
	return table
}

// seedSource answers a seed accessor that hands back one table, standing in
// for the embedded seed.
func seedSource(table *rirTable) func() (*rirTable, error) {
	return func() (*rirTable, error) { return table, nil }
}

// storedSource answers a stored-copy accessor that hands back one blob.
func storedSource(blob string) func() ([]byte, bool) {
	return func() ([]byte, bool) { return []byte(blob), true }
}

// registryOf answers the registry the table holds AS5 for, which every fixture
// here delegates.
func registryOf(t *testing.T, table *rirTable) string {
	t.Helper()

	entry := table.rirForASN(5)
	if entry == nil {
		t.Fatal("AS5 is in no range of a table that holds AS1 to AS10")
		return "" // unreachable, satisfies staticcheck SA5011
	}
	return entry.RIR
}

// VALIDATES: a stored copy generated after the shipped seed answers the
// lookup (AC-7).
// PREVENTS: a refresh the operator ordered staying invisible, so `update
// resolve rir` changes nothing a later `show resolve rir` says.
func TestTheStoredCopyWinsWhenItIsNewer(t *testing.T) {
	seed := tableFrom(t, delegationFile("2026-08-16", "arin"))

	table, note := preferStoredDelegation(seedSource(seed), storedSource(delegationFile("2026-09-01", "ripencc")))
	if note != nil {
		t.Fatalf("a readable stored copy was reported as a problem: %v", note)
	}
	if got := registryOf(t, table); got != RIRRIPE {
		t.Errorf("the lookup answers %q, and the newer stored copy says %q", got, RIRRIPE)
	}
}

// VALIDATES: the shipped seed answers when the stored copy is not newer than
// it, whether the stored copy is older or carries the same date (AC-8).
// PREVENTS: an upgrade shipping fresher data being overruled by a stale copy a
// refresh stored a year earlier (R-5).
func TestTheSeedWinsWhenTheStoredCopyIsNotNewer(t *testing.T) {
	tests := []struct {
		name       string
		storedDate string
	}{
		{"older", "2026-01-01"},
		{"the same date", "2026-08-16"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seed := tableFrom(t, delegationFile("2026-08-16", "arin"))

			table, note := preferStoredDelegation(seedSource(seed), storedSource(delegationFile(tt.storedDate, "ripencc")))
			if note != nil {
				t.Fatalf("a readable stored copy was reported as a problem: %v", note)
			}
			if got := registryOf(t, table); got != RIRARIN {
				t.Errorf("the lookup answers %q, and the shipped seed says %q", got, RIRARIN)
			}
		})
	}
}

// VALIDATES: a stored copy that does not parse is reported and the shipped
// seed answers (AC-9).
// PREVENTS: a half-read stored copy answering, and a stored copy written by a
// newer binary silently taking every lookup down with it (R-3).
func TestAnUnreadableStoredCopyFallsBackToTheSeed(t *testing.T) {
	seed := tableFrom(t, delegationFile("2026-08-16", "arin"))

	table, note := preferStoredDelegation(seedSource(seed), storedSource("# Generated: 2026-09-01\n1 10 nosuchregistry\n"))
	if note == nil {
		t.Fatal("a stored copy that does not parse was passed over in silence")
	}
	if got := registryOf(t, table); got != RIRARIN {
		t.Errorf("the lookup answers %q, and the shipped seed says %q", got, RIRARIN)
	}
}

// VALIDATES: the shipped seed answers when no refresh has stored a copy.
// PREVENTS: a fresh install with an empty store answering that no registry
// holds any AS number.
func TestTheSeedAnswersWhenNothingIsStored(t *testing.T) {
	seed := tableFrom(t, delegationFile("2026-08-16", "arin"))

	table, note := preferStoredDelegation(seedSource(seed), func() ([]byte, bool) { return nil, false })
	if note != nil {
		t.Fatalf("an absent stored copy was reported as a problem: %v", note)
	}
	if got := registryOf(t, table); got != RIRARIN {
		t.Errorf("the lookup answers %q, and the shipped seed says %q", got, RIRARIN)
	}
}

// VALIDATES: an unreadable shipped seed answers no table at all, even when a
// stored copy is present.
// PREVENTS: the two failures merging, so a caller cannot tell "I read the
// table and nobody holds this AS number" from "I could not read the table"
// (ai/rules/principles.md).
func TestAnUnreadableSeedAnswersNoTable(t *testing.T) {
	seedErr := errors.New("embedded delegation seed: line 1")
	seed := func() (*rirTable, error) { return nil, seedErr }

	table, note := preferStoredDelegation(seed, storedSource(delegationFile("2026-09-01", "ripencc")))
	if table != nil {
		t.Fatal("an unreadable seed still answered a table")
	}
	if !errors.Is(note, seedErr) {
		t.Errorf("the failure reported is %v, and the seed said %v", note, seedErr)
	}
}

// VALIDATES: the host reads the managed store read-only while another handle
// holds the same file open (spec assumption A-2).
// PREVENTS: `ze resolve rir` on the host answering from the seed alone because
// the running daemon holds database.zefs.
func TestTheHostReadsTheStoreWhileItIsHeldOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	held, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	defer func() { _ = held.Close() }()

	body := delegationFile("2026-09-01", "ripencc")
	if err := held.WriteFile(zefs.KeyRIRDelegation.Pattern, []byte(body), 0); err != nil {
		t.Fatalf("write the delegation key: %v", err)
	}

	// held stays open, as the daemon's own handle does while the host CLI runs.
	blob, ok := storedDelegationFile(path)
	if !ok {
		t.Fatal("the host read nothing from a store that holds the delegation key")
	}
	if string(blob) != body {
		t.Errorf("the host read %q, and the store holds %q", blob, body)
	}
}

// VALIDATES: the whole host path, from RegistryForASN to a stored copy under
// the config directory, answers from that copy when it is newer than the
// shipped seed (AC-7 on the host, user story 3).
// PREVENTS: a refresh the daemon stored being invisible to `ze resolve rir`,
// which reads the same file with no daemon and no network.
func TestTheHostLookupAnswersFromANewerStoredCopy(t *testing.T) {
	dir := t.TempDir()
	previous := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", previous) })
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatalf("point the config directory at the test store: %v", err)
	}

	store, err := zefs.Create(filepath.Join(dir, storeFileName))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	// AS3333 is RIPE in the shipped seed, so an answer of LACNIC can only come
	// from the copy this test stored.
	body := "# Generated: 2099-01-01\n3333 3333 lacnic\n"
	if err := store.WriteFile(zefs.KeyRIRDelegation.Pattern, []byte(body), 0); err != nil {
		t.Fatalf("write the delegation key: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close the store: %v", err)
	}

	entry, err := RegistryForASN(3333)
	if err != nil {
		t.Fatalf("RegistryForASN: %v", err)
	}
	if entry.RIR != RIRLACNIC {
		t.Errorf("the host answers %q, and the newer stored copy says %q", entry.RIR, RIRLACNIC)
	}
}

// VALIDATES: a store file that holds no delegation key answers "nothing
// stored" rather than an empty table.
// PREVENTS: an appliance that never refreshed reporting every AS number as
// undelegated.
func TestAStoreWithNoDelegationKeyAnswersNothingStored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.zefs")
	bs, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	defer func() { _ = bs.Close() }()

	if _, ok := storedDelegationFile(path); ok {
		t.Error("a store holding no delegation key answered a stored copy")
	}
}

// VALIDATES: inside the daemon the stored copy comes from the registered state
// store, and the file is never opened a second time.
// PREVENTS: a transient zefs handle in the hub, which makes the config store's
// next flush drop every state key (internal/core/statestore package doc).
func TestTheDaemonReadsTheStoredCopyThroughTheStateStore(t *testing.T) {
	bs, err := zefs.Create(filepath.Join(t.TempDir(), "database.zefs"))
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		_ = bs.Close()
	})

	body := delegationFile("2026-09-01", "ripencc")
	if _, err := statestore.Put(zefs.KeyRIRDelegation.Pattern, []byte(body)); err != nil {
		t.Fatalf("statestore.Put: %v", err)
	}

	blob, ok := storedDelegation()
	if !ok {
		t.Fatal("the daemon read nothing from the registered state store")
	}
	if string(blob) != body {
		t.Errorf("the daemon read %q, and the store holds %q", blob, body)
	}
}
