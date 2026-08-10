// VALIDATES: AC-6 -- show debug lists all active debug config.
// VALIDATES: AC-7 -- show debug <module> filters by subtree.
// PREVENTS: Show debug returning empty or unstructured output.

package debug

import (
	"testing"
)

func TestShowDebugEntries(t *testing.T) {
	p := newProfile()
	p.toggleModule("bgp.reactor")
	p.toggleFlag("bgp.reactor", "update")
	p.toggleModule("plugin.manager")

	entries := showEntries(p, "")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestShowDebugSubtree(t *testing.T) {
	p := newProfile()
	p.toggleModule("bgp.reactor")
	p.toggleModule("bgp.server")
	p.toggleModule("plugin.manager")

	entries := showEntries(p, "bgp")
	if len(entries) != 2 {
		t.Fatalf("expected 2 bgp entries, got %d: %v", len(entries), entries)
	}
}

func TestShowDebugEmpty(t *testing.T) {
	p := newProfile()
	entries := showEntries(p, "")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestShowDebugEntryFields(t *testing.T) {
	p := newProfile()
	p.toggleModule("bgp.reactor")
	p.toggleFlag("bgp.reactor", "update")
	p.toggleFlag("bgp.reactor", "open")
	p.toggleScope("bgp.reactor", "neighbor", "192.0.2.1")
	p.toggleScope("bgp.reactor", "direction", "receive")

	entries := showEntries(p, "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Module != "bgp.reactor" {
		t.Errorf("module = %q", e.Module)
	}
	if e.Level != "debug" {
		t.Errorf("level = %q", e.Level)
	}
	if e.Flags != "update, open" {
		t.Errorf("flags = %q", e.Flags)
	}
	if e.Scopes != "neighbor=192.0.2.1, direction=receive" {
		t.Errorf("scopes = %q", e.Scopes)
	}
}
