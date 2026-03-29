// NOTE: these tests must not use t.Parallel() because they mutate package globals.
package zefs

import (
	"testing"
)

func TestMustRegister(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	e := MustRegister(KeyEntry{Pattern: "test/fixed", Description: "test fixed key"})
	if e.Pattern != "test/fixed" {
		t.Fatalf("expected pattern test/fixed, got %s", e.Pattern)
	}
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered, got %d", len(registered))
	}
	if registered[0].Pattern != "test/fixed" {
		t.Fatalf("expected registered[0].Pattern test/fixed, got %s", registered[0].Pattern)
	}
}

func TestMustRegisterEmptyPatternPanics(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty Pattern")
		}
	}()
	MustRegister(KeyEntry{Pattern: "", Description: "bad"})
}

func TestMustRegisterDuplicatePanics(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	MustRegister(KeyEntry{Pattern: "test/dup", Description: "first"})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate pattern")
		}
	}()
	MustRegister(KeyEntry{Pattern: "test/dup", Description: "second"})
}

func TestMustRegisterPrivate(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	MustRegister(KeyEntry{Pattern: "test/public", Description: "public"})
	MustRegister(KeyEntry{Pattern: "test/secret", Description: "secret", Private: true})

	entries := Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 public entry, got %d", len(entries))
	}
	if entries[0].Pattern != "test/public" {
		t.Fatalf("expected test/public, got %s", entries[0].Pattern)
	}

	all := AllEntries()
	if len(all) != 2 {
		t.Fatalf("expected 2 total entries, got %d", len(all))
	}
}

func TestEntriesSorted(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	MustRegister(KeyEntry{Pattern: "z/last"})
	MustRegister(KeyEntry{Pattern: "a/first"})
	MustRegister(KeyEntry{Pattern: "m/middle"})

	entries := Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Pattern != "a/first" || entries[1].Pattern != "m/middle" || entries[2].Pattern != "z/last" {
		t.Fatalf("expected sorted order, got %v", entries)
	}
}

func TestKeyEntryKey(t *testing.T) {
	e := KeyEntry{Pattern: "meta/history/{username}/{mode}"}
	got := e.Key("alice", "edit")
	if got != "meta/history/alice/edit" {
		t.Fatalf("expected meta/history/alice/edit, got %s", got)
	}
}

func TestKeyEntryKeySinglePlaceholder(t *testing.T) {
	e := KeyEntry{Pattern: "file/active/{basename}"}
	got := e.Key("router.conf")
	if got != "file/active/router.conf" {
		t.Fatalf("expected file/active/router.conf, got %s", got)
	}
}

func TestKeyEntryKeyPanicMismatch(t *testing.T) {
	e := KeyEntry{Pattern: "meta/history/{username}/{mode}"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on param count mismatch")
		}
	}()
	_ = e.Key("only-one")
}

func TestKeyEntryKeyPanicFixedWithParams(t *testing.T) {
	e := KeyEntry{Pattern: "meta/ssh/username"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when fixed key called with params")
		}
	}()
	_ = e.Key("unexpected")
}

func TestKeyEntryKeyPanicEmptyParam(t *testing.T) {
	e := KeyEntry{Pattern: "file/active/{basename}"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty param")
		}
	}()
	_ = e.Key("")
}

func TestKeyEntryKeyPanicDotDotParam(t *testing.T) {
	e := KeyEntry{Pattern: "file/active/{basename}"}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on '..' in param")
		}
	}()
	_ = e.Key("../../etc/passwd")
}

func TestKeyEntryKeyFixed(t *testing.T) {
	e := KeyEntry{Pattern: "meta/ssh/username"}
	got := e.Key()
	if got != "meta/ssh/username" {
		t.Fatalf("expected meta/ssh/username, got %s", got)
	}
}

func TestIsRegistered(t *testing.T) {
	origRegistered := registered
	origPrefixes := keyPrefixes
	t.Cleanup(func() {
		registered = origRegistered
		keyPrefixes = origPrefixes
	})
	registered = nil
	keyPrefixes = nil

	MustRegister(KeyEntry{Pattern: "meta/ssh/username"})
	MustRegister(KeyEntry{Pattern: "meta/history/{username}/{mode}"})

	tests := []struct {
		key  string
		want bool
	}{
		{"meta/ssh/username", true},             // exact match
		{"meta/history/alice/edit", true},       // prefix match for template
		{"meta/unknown/key", false},             // unknown
		{"", false},                             // empty
		{"meta/ssh/user", false},                // partial fixed key (not a prefix match)
		{"meta/historyX/foo", false},            // similar prefix but wrong
		{"meta/history/alice/edit/extra", true}, // extra segments still match prefix
	}
	for _, tt := range tests {
		got := IsRegistered(tt.key)
		if got != tt.want {
			t.Errorf("IsRegistered(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestPrefix(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"meta/ssh/username", "meta/ssh/username/"},
		{"meta/history/{username}/{mode}", "meta/history/"},
		{"file/active/{basename}", "file/active/"},
	}
	for _, tt := range tests {
		e := KeyEntry{Pattern: tt.pattern}
		got := e.Prefix()
		if got != tt.want {
			t.Errorf("Prefix(%q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

func TestDir(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"meta/ssh/username", "meta/ssh/username"},
		{"meta/history/{username}/{mode}", "meta/history"},
		{"file/active/{basename}", "file/active"},
	}
	for _, tt := range tests {
		e := KeyEntry{Pattern: tt.pattern}
		got := e.Dir()
		if got != tt.want {
			t.Errorf("Dir(%q) = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

func TestAllProductionKeysRegistered(t *testing.T) {
	// Verify all 13 keys from keys.go are registered with correct patterns.
	expected := map[string]bool{
		"meta/ssh/username":              true,
		"meta/ssh/password":              true,
		"meta/ssh/host":                  true,
		"meta/ssh/port":                  true,
		"meta/instance/name":             true,
		"meta/instance/managed":          true,
		"meta/web/cert":                  true,
		"meta/web/key":                   true,
		"meta/bgp/gr-marker":             true,
		"meta/history/max":               true,
		"meta/history/{username}/{mode}": true,
		"file/active/{basename}":         true,
		"file/draft/{basename}":          true,
	}

	all := AllEntries()
	found := make(map[string]bool)
	for _, e := range all {
		found[e.Pattern] = true
	}

	for pattern := range expected {
		if !found[pattern] {
			t.Errorf("expected key %q to be registered", pattern)
		}
	}
}

func TestPrivateKeysMarked(t *testing.T) {
	privatePatterns := map[string]bool{
		"meta/ssh/password": true,
		"meta/web/cert":     true,
		"meta/web/key":      true,
	}

	for _, e := range AllEntries() {
		if privatePatterns[e.Pattern] && !e.Private {
			t.Errorf("expected %q to be marked Private", e.Pattern)
		}
		if !privatePatterns[e.Pattern] && e.Private {
			t.Errorf("expected %q to NOT be marked Private", e.Pattern)
		}
	}
}
