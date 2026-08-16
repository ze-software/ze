package envcatalog

import (
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	env.MustRegister(env.EnvEntry{Key: "ze.test.catalog.public", Type: "string", Default: "val", Description: "test public var"})
	env.MustRegister(env.EnvEntry{Key: "ze.test.catalog.private", Type: "string", Description: "test private var", Private: true})
	env.MustRegister(env.EnvEntry{Key: "ze.test.catalog.secret", Type: "string", Description: "test secret var", Secret: true})

	_ = slogutil.LazyLogger("test.catalog.sub1")
}

// VALIDATES: AC-7 — shared env catalog hides Private entries.
// PREVENTS: private env vars leaking into completion output.
// Private was removed from CatalogEntry (it was always false), and env.Entries()
// pre-filters, so there is no per-entry field left to check here.
func TestVisibleEntriesExcludePrivate(t *testing.T) {
	entries := VisibleEntries()

	found := false
	for _, e := range entries {
		if e.Key == "ze.test.catalog.public" {
			found = true
		}
		if e.Key == "ze.test.catalog.private" {
			t.Error("VisibleEntries() should not include private entry ze.test.catalog.private")
		}
	}
	if !found {
		t.Error("VisibleEntries() should include public test entry ze.test.catalog.public")
	}
}

// VALIDATES: AC-6 — catalog expands ze.log.<subsystem> from slogutil.Subsystems().
// PREVENTS: concrete log subsystem keys missing from completion suggestions.
func TestVisibleEntriesIncludeConcreteLogSubsystems(t *testing.T) {
	entries := VisibleEntries()

	found := false
	for _, e := range entries {
		if e.Key == "ze.log.test.catalog.sub1" {
			found = true
			if e.Description == "" {
				t.Error("concrete log subsystem entry should have a description")
			}
			break
		}
	}
	if !found {
		t.Error("VisibleEntries() should include concrete ze.log.test.catalog.sub1 from slogutil.Subsystems()")
	}
}

// VALIDATES: AC-7 — catalog entries are sorted by key.
// PREVENTS: completion output appearing in random order.
func TestVisibleEntriesAreSorted(t *testing.T) {
	entries := VisibleEntries()
	for i := 1; i < len(entries); i++ {
		if entries[i].Key < entries[i-1].Key {
			t.Errorf("VisibleEntries() not sorted: %q before %q", entries[i-1].Key, entries[i].Key)
			break
		}
	}
}

// VALIDATES: AC-7 — catalog never includes entries with angle-bracket keys.
// PREVENTS: template-style "ze.log.<subsystem>" appearing in completion.
func TestVisibleEntriesExcludeAngleBracketKeys(t *testing.T) {
	entries := VisibleEntries()
	for _, e := range entries {
		for _, c := range e.Key {
			if c == '<' || c == '>' {
				t.Errorf("VisibleEntries() should not contain angle-bracket key %q", e.Key)
				break
			}
		}
	}
}

// VALIDATES: AC-6 — LookupLogSubsystem resolves concrete log subsystem keys.
// PREVENTS: completed log key failing inspection because showOne doesn't know it.
func TestLookupLogSubsystem(t *testing.T) {
	info, ok := LookupLogSubsystem("ze.log.test.catalog.sub1")
	if !ok {
		t.Fatal("LookupLogSubsystem should find ze.log.test.catalog.sub1")
	}
	if info.Name != "test.catalog.sub1" {
		t.Errorf("LookupLogSubsystem name = %q, want %q", info.Name, "test.catalog.sub1")
	}

	_, ok = LookupLogSubsystem("ze.log.nonexistent")
	if ok {
		t.Error("LookupLogSubsystem should return false for unknown subsystem")
	}

	_, ok = LookupLogSubsystem("ze.pprof")
	if ok {
		t.Error("LookupLogSubsystem should return false for non-log keys")
	}
}
