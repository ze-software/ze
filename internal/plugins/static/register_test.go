package static

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestStaticRouteRegistration(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	if reg.Name != "static" {
		t.Errorf("name = %q, want %q", reg.Name, "static")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "static" {
		t.Errorf("config roots = %v, want [static]", reg.ConfigRoots)
	}
	if reg.YANG == "" {
		t.Error("YANG schema is empty")
	}
}

// TestStaticRegistersRedistributeSource
// VALIDATES: the static plugin registers "static" as a redistribute source at init(),
// so `redistribute { destination <proto> { import static } }` resolves and is visible to
// `ze config validate` (which imports plugins but does not run their engines).
// PREVENTS: regression of the bug where static emitted route events but was not a
// registered config source, so `import static` was rejected at runtime ("unknown source").
func TestStaticRegistersRedistributeSource(t *testing.T) {
	src, ok := redistribute.LookupSource("static")
	if !ok {
		t.Fatal("redistribute source \"static\" not registered (import static would be rejected)")
	}
	if src.Name != "static" {
		t.Errorf("source name = %q, want %q", src.Name, "static")
	}
	if src.Protocol != "static" {
		t.Errorf("source protocol = %q, want %q", src.Protocol, "static")
	}
}
