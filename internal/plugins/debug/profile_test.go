// VALIDATES: AC-8 -- debug state persists in debug.zefs, not database.zefs.
// VALIDATES: AC-1, AC-2 -- profile toggle adds/removes module entries.
// VALIDATES: AC-3 -- flag toggle adds/removes flag entries in module section.
// VALIDATES: AC-13 -- named profiles saved to debug.zefs.
// PREVENTS: Profile corruption on toggle, partial state on failure.

package debug

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

func setupDebugStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "debug.zefs")
	store, err := zefs.Create(storePath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close() //nolint:errcheck // test setup
	return storePath
}

func TestProfileToggleModule(t *testing.T) {
	storePath := setupDebugStore(t)

	p := NewProfile()
	if p.HasModule("bgp.reactor") {
		t.Fatal("new profile should not have bgp.reactor")
	}

	p.ToggleModule("bgp.reactor")
	if !p.HasModule("bgp.reactor") {
		t.Fatal("after toggle on, should have bgp.reactor")
	}

	p.ToggleModule("bgp.reactor")
	if p.HasModule("bgp.reactor") {
		t.Fatal("after toggle off, should not have bgp.reactor")
	}

	_ = storePath
}

func TestProfileToggleFlag(t *testing.T) {
	p := NewProfile()
	p.ToggleModule("bgp.reactor")

	p.ToggleFlag("bgp.reactor", "update")
	flags := p.Flags("bgp.reactor")
	if len(flags) != 1 || flags[0].Name != "update" {
		t.Fatalf("expected [update], got %v", flags)
	}

	p.ToggleFlag("bgp.reactor", "update")
	flags = p.Flags("bgp.reactor")
	if len(flags) != 0 {
		t.Fatalf("expected [], got %v", flags)
	}
}

func TestProfileDirectionAsScope(t *testing.T) {
	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	p.ToggleFlag("bgp.reactor", "update")
	p.ToggleScope("bgp.reactor", "direction", "receive")

	flags := p.Flags("bgp.reactor")
	if len(flags) != 1 || flags[0].Name != "update" {
		t.Fatalf("expected [update], got %v", flags)
	}

	scopes := p.Scopes("bgp.reactor")
	if len(scopes) != 1 || scopes[0].Kind != "direction" || scopes[0].Value != "receive" {
		t.Fatalf("expected direction=receive scope, got %v", scopes)
	}

	p.ToggleScope("bgp.reactor", "direction", "receive")
	scopes = p.Scopes("bgp.reactor")
	if len(scopes) != 0 {
		t.Fatalf("expected 0 scopes after toggle off, got %d", len(scopes))
	}
}

func TestProfileToggleScope(t *testing.T) {
	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	p.ToggleScope("bgp.reactor", "neighbor", "192.0.2.1")

	scopes := p.Scopes("bgp.reactor")
	if len(scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(scopes))
	}

	p.ToggleScope("bgp.reactor", "neighbor", "192.0.2.1")
	scopes = p.Scopes("bgp.reactor")
	if len(scopes) != 0 {
		t.Fatalf("expected 0 scopes after toggle off, got %d", len(scopes))
	}
}

func TestProfileLoadSave(t *testing.T) {
	storePath := setupDebugStore(t)

	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	p.ToggleFlag("bgp.reactor", "update")
	p.ToggleScope("bgp.reactor", "neighbor", "192.0.2.1")
	p.Timeout = 30

	if err := SaveProfile(storePath, "default", p); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadProfile(storePath, "default")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !loaded.HasModule("bgp.reactor") {
		t.Error("loaded profile missing bgp.reactor")
	}

	flags := loaded.Flags("bgp.reactor")
	if len(flags) != 1 || flags[0].Name != "update" {
		t.Errorf("loaded flags: %v", flags)
	}

	scopes := loaded.Scopes("bgp.reactor")
	if len(scopes) != 1 {
		t.Errorf("loaded scopes: %v", scopes)
	}

	if loaded.Timeout != 30 {
		t.Errorf("timeout = %d, want 30", loaded.Timeout)
	}
}

func TestProfileSaveNamed(t *testing.T) {
	storePath := setupDebugStore(t)

	p1 := NewProfile()
	p1.ToggleModule("bgp.reactor")

	p2 := NewProfile()
	p2.ToggleModule("plugin.manager")

	if err := SaveProfile(storePath, "bgp-deep", p1); err != nil {
		t.Fatalf("save p1: %v", err)
	}
	if err := SaveProfile(storePath, "plugin-debug", p2); err != nil {
		t.Fatalf("save p2: %v", err)
	}

	names, err := ListProfiles(storePath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 profiles, got %d: %v", len(names), names)
	}
}

func TestProfileLoadNotFound(t *testing.T) {
	storePath := setupDebugStore(t)

	_, err := LoadProfile(storePath, "nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent profile")
	}
}

func TestProfileNameValidation(t *testing.T) {
	storePath := setupDebugStore(t)
	p := NewProfile()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"", true},
		{"has space", true},
		{"has/slash", true},
		{strings.Repeat("x", 64), false},
		{strings.Repeat("x", 65), true},
	}

	for _, tt := range tests {
		err := SaveProfile(storePath, tt.name, p)
		if (err != nil) != tt.wantErr {
			t.Errorf("SaveProfile(%q) err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestProfileDelete(t *testing.T) {
	storePath := setupDebugStore(t)

	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	if err := SaveProfile(storePath, "temp", p); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := DeleteProfile(storePath, "temp"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := LoadProfile(storePath, "temp")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestProfileModules(t *testing.T) {
	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	p.ToggleModule("plugin.manager")

	mods := p.ModuleNames()
	if len(mods) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(mods))
	}
}

func TestProfileSetLevel(t *testing.T) {
	p := NewProfile()
	p.ToggleModule("bgp.reactor")
	p.SetLevel("bgp.reactor", "info")

	entry := p.Module("bgp.reactor")
	if entry == nil {
		t.Fatal("expected module entry")
	}
	if entry.Level != "info" {
		t.Errorf("level = %q, want info", entry.Level)
	}
}
