package slogutil

import (
	"io/fs"
	"strings"
	"testing"
)

// fakeDebugStore implements DebugStore for testing.
type fakeDebugStore struct {
	files map[string]string
}

func newFakeStore() *fakeDebugStore {
	return &fakeDebugStore{files: make(map[string]string)}
}

func (s *fakeDebugStore) ReadFile(name string) ([]byte, error) {
	v, ok := s.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(v), nil
}

func (s *fakeDebugStore) Has(name string) bool {
	_, ok := s.files[name]
	return ok
}

func (s *fakeDebugStore) List(prefix string) []string {
	var result []string
	for k := range s.files {
		if strings.HasPrefix(k, prefix) {
			result = append(result, k)
		}
	}
	return result
}

func (s *fakeDebugStore) set(key, value string) {
	s.files[key] = value
}

func TestResolveDebugStateDefault(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.alpha")
	_ = Logger("test.beta")

	store := newFakeStore()
	states := ResolveDebugStates(store)

	for _, s := range states {
		if s.Enabled {
			t.Errorf("subsystem %s: expected disabled by default, got enabled", s.Name)
		}
		if s.Source != DebugSourceDefault {
			t.Errorf("subsystem %s: expected source default, got %s", s.Name, s.Source)
		}
	}
}

func TestResolveDebugStateExplicit(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.alpha")
	_ = Logger("test.beta")

	store := newFakeStore()
	store.set("state/debug/test.alpha", "on")

	states := ResolveDebugStates(store)

	for _, s := range states {
		switch s.Name {
		case "test.alpha":
			if !s.Enabled {
				t.Error("test.alpha: expected enabled")
			}
			if s.Source != DebugSourceExplicit {
				t.Errorf("test.alpha: expected source explicit, got %s", s.Source)
			}
		case "test.beta":
			if s.Enabled {
				t.Error("test.beta: expected disabled")
			}
			if s.Source != DebugSourceDefault {
				t.Errorf("test.beta: expected source default, got %s", s.Source)
			}
		}
	}
}

func TestResolveDebugStateGlobalOverride(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.alpha")
	_ = Logger("test.beta")

	store := newFakeStore()
	store.set("state/debug/all", "on")
	store.set("state/debug/test.alpha", "off")

	states := ResolveDebugStates(store)

	for _, s := range states {
		if !s.Enabled {
			t.Errorf("subsystem %s: expected enabled (global override), got disabled", s.Name)
		}
		if s.Source != DebugSourceGlobal {
			t.Errorf("subsystem %s: expected source global, got %s", s.Name, s.Source)
		}
	}
}

func TestResolveDebugStateHierarchical(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.parent.child1")
	_ = Logger("test.parent.child2")
	_ = Logger("test.other")

	store := newFakeStore()
	store.set("state/debug/test.parent", "on")

	states := ResolveDebugStates(store)

	for _, s := range states {
		switch {
		case strings.HasPrefix(s.Name, "test.parent."):
			if !s.Enabled {
				t.Errorf("%s: expected enabled via parent key, got disabled", s.Name)
			}
			if s.Source != DebugSourceExplicit {
				t.Errorf("%s: expected source explicit, got %s", s.Name, s.Source)
			}
		case s.Name == "test.other":
			if s.Enabled {
				t.Errorf("test.other: expected disabled, got enabled")
			}
		}
	}
}

func TestResolveDebugStateExplicitOffWithGlobalOff(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.alpha")

	store := newFakeStore()
	store.set("state/debug/test.alpha", "on")
	store.set("state/debug/all", "off")

	states := ResolveDebugStates(store)

	for _, s := range states {
		if s.Name == "test.alpha" {
			if !s.Enabled {
				t.Error("test.alpha: expected enabled (per-subsystem on, global off)")
			}
			if s.Source != DebugSourceExplicit {
				t.Errorf("test.alpha: expected source explicit, got %s", s.Source)
			}
		}
	}
}

func TestApplyDebugFlagsFromStore(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.apply")

	store := newFakeStore()
	store.set("state/debug/test.apply", "on")

	ApplyDebugFlags(store)

	levels := ListLevels()
	if levels["test.apply"] != "debug" {
		t.Errorf("test.apply: expected debug level after ApplyDebugFlags, got %s", levels["test.apply"])
	}
}

func TestValidateSubsystem(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.valid")
	_ = Logger("test.valid.child")

	tests := []struct {
		name  string
		valid bool
	}{
		{"all", true},
		{"test.valid", true},
		{"test.valid.child", true},
		{"test", true},
		{"nonexistent", false},
		{"", false},
		{"foo/bar", false},
		{"test\x00valid", false},
	}

	for _, tt := range tests {
		got := ValidateSubsystem(tt.name)
		if got != tt.valid {
			t.Errorf("ValidateSubsystem(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestSubsystemsMatching(t *testing.T) {
	ResetLevelRegistry()
	_ = Logger("test.match.a")
	_ = Logger("test.match.b")
	_ = Logger("test.other")

	matches := SubsystemsMatching("test.match")
	if len(matches) != 2 {
		t.Errorf("SubsystemsMatching(test.match): got %d matches, want 2: %v", len(matches), matches)
	}

	matches = SubsystemsMatching("test")
	if len(matches) < 3 {
		t.Errorf("SubsystemsMatching(test): got %d matches, want >=3: %v", len(matches), matches)
	}
}

func TestReadFlagValues(t *testing.T) {
	store := newFakeStore()
	store.set("state/debug/a", "on")
	store.set("state/debug/b", "off")
	store.set("state/debug/c", " on ")

	if !readFlag(store, "state/debug/a") {
		t.Error("expected 'on' to be true")
	}
	if readFlag(store, "state/debug/b") {
		t.Error("expected 'off' to be false")
	}
	if !readFlag(store, "state/debug/c") {
		t.Error("expected ' on ' (trimmed) to be true")
	}
	if readFlag(store, "state/debug/missing") {
		t.Error("expected missing key to be false")
	}
}
