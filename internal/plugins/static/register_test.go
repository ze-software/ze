package static

import (
	"slices"
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

// TestStaticDeclaresOptionalInterfaceDependency
// VALIDATES: AC-8 / A-1c -- static declares "interface" as an OPTIONAL
// dependency, so TopologicalTiers orders static after the iface component when
// both are present (loading the iface backend before static resolves a next-hop
// interface), while leaving static unconstrained when no interface stanza
// exists. Asserted on the registration because the startup-tier race itself is
// not reliably observable by re-running.
// PREVENTS: a regression to the tier race where static's resolve raced
// iface.LoadBackend and failed with "no backend loaded" nondeterministically.
func TestStaticDeclaresOptionalInterfaceDependency(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	if !slices.Contains(reg.OptionalDependencies, "interface") {
		t.Errorf("optional dependencies = %v, want to include \"interface\"", reg.OptionalDependencies)
	}
	// "interface" must be OPTIONAL, not a hard dependency: a static config with
	// no interface stanza must still load.
	if slices.Contains(reg.Dependencies, "interface") {
		t.Error("\"interface\" is a hard dependency; it must be optional so no-interface configs still load")
	}
}

// TestStaticDeclaresInterfaceNexthopDoctorCheck
// VALIDATES: D-2 (b) -- the static plugin owns a doctor readiness check for the
// interface-only next-hop's runtime dependency on a loaded iface backend.
func TestStaticDeclaresInterfaceNexthopDoctorCheck(t *testing.T) {
	reg := registry.Lookup("static")
	if reg == nil {
		t.Fatal("static plugin not registered")
	}
	found := false
	for _, dc := range reg.DoctorChecks {
		for _, code := range dc.Codes {
			if code == "doctor-static-interface-nexthop-no-backend" {
				found = true
			}
		}
	}
	if !found {
		t.Error("static plugin declares no doctor check emitting doctor-static-interface-nexthop-no-backend")
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
