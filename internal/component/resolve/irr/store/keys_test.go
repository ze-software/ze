// Design: docs/architecture/irr-owner.md -- the state keys the owner forms from an AS-SET name
package store

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// storageStateReader answers StateReader over a daemon store handle, the way
// firewall-irr's doctor reads it: a key the store does not hold is not found,
// never an error.
type storageStateReader struct{ keys storage.Storage }

func (r storageStateReader) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	value, err := r.keys.ReadKey(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// VALIDATES: A-5 -- an AS-SET name holding ':' (hierarchical IRR names such as
// RIPE::AS-FOO) forms a dataset key and a pointer key the daemon's store
// accepts, and the store opened again after they were written reads every set
// back whole through the consumer reader.
// PREVENTS: a hierarchical AS-SET whose stored copy cannot be written, or whose
// key makes the shared store file unreadable at the next start.
// METHOD: write each set's dataset, then its pointer, as the owner does
// (internal/plugins/irr writeDataset), into a real store; close it, open the
// folder again, and ReadSet every set.
func TestStoredKeysHoldColonNames(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sets := []struct {
		name       string
		prefixes   []netip.Prefix
		generation uint32
	}{
		{"AS-FOO:AS-BAR", []netip.Prefix{netip.MustParsePrefix("203.0.113.0/32")}, 1},
		{"RIPE::AS-FOO", []netip.Prefix{netip.MustParsePrefix("203.0.113.1/32")}, 2},
		{"AS-OK", []netip.Prefix{netip.MustParsePrefix("203.0.113.2/32")}, 3},
	}

	first, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range sets {
		dataset := mustDataset(t, DatasetInput{
			ASSet: set.name, Server: "whois.radb.net", IPv4: set.prefixes,
			RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_000_600, 0),
		})
		value := encodeDataset(t, dataset)
		if writeErr := first.WriteKey(dataset.Key(), value); writeErr != nil {
			t.Fatalf("write dataset %s: %v", dataset.Key(), writeErr)
		}
		digest := DatasetDigest(value)
		pointer, encodeErr := EncodePointer(Pointer{Server: dataset.Server(), Generation: set.generation, Digest: hex.EncodeToString(digest[:])})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if writeErr := first.WriteKey(PointerKey(set.name), pointer); writeErr != nil {
			t.Fatalf("write pointer %s: %v", PointerKey(set.name), writeErr)
		}
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("open the store again after writing ':' keys: %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, set := range sets {
		got, readErr := ReadSet(ctx, storageStateReader{keys: reopened}, set.name)
		if readErr != nil {
			t.Fatalf("ReadSet(%q) after reopen: %v", set.name, readErr)
		}
		if got.Dataset.ASSet() != set.name || !slices.Equal(got.Dataset.IPv4(), set.prefixes) {
			t.Fatalf("ReadSet(%q) after reopen = %s %v, want %s %v", set.name, got.Dataset.ASSet(), got.Dataset.IPv4(), set.name, set.prefixes)
		}
		if got.Pointer.Generation != set.generation {
			t.Fatalf("ReadSet(%q) pointer generation %d, want %d", set.name, got.Pointer.Generation, set.generation)
		}
	}
}

// VALIDATES: a name that is not a valid zefs path segment ("", ".", "..",
// "a..b") forms no key: ASSetName, DatasetKey and NewDataset refuse it, and
// PrefixStore.Seed refuses to hold it, so no write the owner makes can reach
// the store with it. A good name passes every one, so each refusal is tied to
// the name.
// PREVENTS: a "meta/irr-set/." or "meta/irr-data/<server>/.." key, which fails
// fs.ValidPath at decode and makes the whole shared store file unreadable, or
// panics KeyEntry.Key on "..".
// METHOD: pass each bad name, then "AS-OK", to every constructor the owner forms
// a key or a seeded copy with.
func TestBadNamesFormNoKey(t *testing.T) {
	s := New(nil, nil) // Seed reaches no client
	prefixes := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}
	for _, bad := range []string{"", ".", "..", "a..b"} {
		if got, err := ASSetName(bad); err == nil {
			t.Errorf("ASSetName(%q) = %q, want a refusal", bad, got)
		}
		if got, err := DatasetKey("whois.radb.net", bad); err == nil {
			t.Errorf("DatasetKey(%q) = %q, want a refusal", bad, got)
		}
		if _, err := NewDataset(DatasetInput{ASSet: bad, Server: "whois.radb.net", IPv4: prefixes}); err == nil {
			t.Errorf("NewDataset(%q) accepted the name", bad)
		}
		if err := s.Seed(CachedEntry{Name: bad, ASSet: bad, IPv4: prefixes}); err == nil {
			t.Errorf("Seed(%q) accepted the name", bad)
		}
		if got := s.Get(bad); got != nil {
			t.Errorf("Get(%q) = %+v after a refused Seed", bad, got)
		}
	}

	if _, err := ASSetName("AS-OK"); err != nil {
		t.Errorf("ASSetName(AS-OK): %v", err)
	}
	if _, err := DatasetKey("whois.radb.net", "AS-OK"); err != nil {
		t.Errorf("DatasetKey(AS-OK): %v", err)
	}
	if _, err := NewDataset(DatasetInput{ASSet: "AS-OK", Server: "whois.radb.net", IPv4: prefixes}); err != nil {
		t.Errorf("NewDataset(AS-OK): %v", err)
	}
	if err := s.Seed(CachedEntry{Name: "AS-OK", ASSet: "AS-OK", IPv4: prefixes}); err != nil {
		t.Errorf("Seed(AS-OK): %v", err)
	}
}
