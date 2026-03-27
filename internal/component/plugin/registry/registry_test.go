package registry

import (
	"bytes"
	"errors"
	"net"
	"testing"
)

// dummyEngine is a stub RunEngine handler for testing.
func dummyEngine(_ net.Conn) int { return 0 }

// dummyCLI is a stub CLIHandler for testing.
func dummyCLI(_ []string) int { return 0 }

// dummyDecoder is a stub InProcessDecoder for testing.
func dummyDecoder(_, _ *bytes.Buffer) int { return 0 }

// validReg returns a minimal valid Registration for testing.
func validReg(name string) Registration {
	return Registration{
		Name:        name,
		Description: "test plugin " + name,
		RunEngine:   dummyEngine,
		CLIHandler:  dummyCLI,
	}
}

// TestRegister verifies that a valid registration succeeds.
//
// VALIDATES: Register stores a plugin that can be retrieved via Lookup.
// PREVENTS: Registration silently failing or losing data.
func TestRegister(t *testing.T) {
	t.Cleanup(func() { Reset() })

	err := Register(validReg("alpha"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reg := Lookup("alpha")
	if reg == nil {
		t.Fatal("expected registration, got nil")
		return
	}
	if reg.Name != "alpha" {
		t.Errorf("expected name %q, got %q", "alpha", reg.Name)
	}
	if reg.Description != "test plugin alpha" {
		t.Errorf("expected description %q, got %q", "test plugin alpha", reg.Description)
	}
}

// TestRegisterDuplicate verifies that registering the same name twice returns an error.
//
// VALIDATES: Duplicate names are rejected with ErrDuplicateName.
// PREVENTS: Silent overwrites of existing plugins.
func TestRegisterDuplicate(t *testing.T) {
	t.Cleanup(func() { Reset() })

	if err := Register(validReg("dup")); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := Register(validReg("dup"))
	if !errors.Is(err, ErrDuplicateName) {
		t.Errorf("expected ErrDuplicateName, got %v", err)
	}
}

// TestRegisterEmptyName verifies that an empty name is rejected.
//
// VALIDATES: Empty name returns ErrEmptyName.
// PREVENTS: Anonymous plugins that cannot be looked up.
func TestRegisterEmptyName(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("")
	err := Register(reg)
	if !errors.Is(err, ErrEmptyName) {
		t.Errorf("expected ErrEmptyName, got %v", err)
	}
}

// TestRegisterNilRunEngine verifies that nil RunEngine is rejected.
//
// VALIDATES: Nil RunEngine returns ErrNilRunEngine.
// PREVENTS: Plugins that crash when engine mode is invoked.
func TestRegisterNilRunEngine(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("noengine")
	reg.RunEngine = nil
	err := Register(reg)
	if !errors.Is(err, ErrNilRunEngine) {
		t.Errorf("expected ErrNilRunEngine, got %v", err)
	}
}

// TestRegisterNilCLIHandler verifies that nil CLIHandler is rejected.
//
// VALIDATES: Nil CLIHandler returns ErrNilCLIHandler.
// PREVENTS: Plugins that crash when CLI mode is invoked.
func TestRegisterNilCLIHandler(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("nocli")
	reg.CLIHandler = nil
	err := Register(reg)
	if !errors.Is(err, ErrNilCLIHandler) {
		t.Errorf("expected ErrNilCLIHandler, got %v", err)
	}
}

// TestRegisterInvalidFamily verifies that malformed family strings are rejected.
//
// VALIDATES: Family strings without "/" return ErrInvalidFamily.
// PREVENTS: Broken family-to-plugin mapping from typos.
func TestRegisterInvalidFamily(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("badfam")
	reg.Families = []string{"ipv4-flow"} // missing /
	err := Register(reg)
	if !errors.Is(err, ErrInvalidFamily) {
		t.Errorf("expected ErrInvalidFamily, got %v", err)
	}
}

// TestRegisterValidFamily verifies that well-formed family strings are accepted.
//
// VALIDATES: Family strings with "/" are accepted.
// PREVENTS: False rejection of valid family strings.
func TestRegisterValidFamily(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("goodfam")
	reg.Families = []string{"ipv4/flow", "ipv6/flow"}
	err := Register(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestLookupUnknown verifies that looking up an unregistered name returns nil.
//
// VALIDATES: Lookup returns nil for unknown plugins.
// PREVENTS: Nil pointer dereference from bad lookups.
func TestLookupUnknown(t *testing.T) {
	t.Cleanup(func() { Reset() })

	if reg := Lookup("nonexistent"); reg != nil {
		t.Errorf("expected nil, got %v", reg)
	}
}

// TestHas verifies the Has function.
//
// VALIDATES: Has returns true for registered, false for unknown.
// PREVENTS: Incorrect plugin existence checks.
func TestHas(t *testing.T) {
	t.Cleanup(func() { Reset() })

	if err := Register(validReg("exists")); err != nil {
		t.Fatal(err)
	}

	if !Has("exists") {
		t.Error("expected Has(exists)=true")
	}
	if Has("missing") {
		t.Error("expected Has(missing)=false")
	}
}

// TestAll verifies that All returns sorted registrations.
//
// VALIDATES: All returns all plugins sorted alphabetically.
// PREVENTS: Unstable ordering in help text and plugin listing.
func TestAll(t *testing.T) {
	t.Cleanup(func() { Reset() })

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		if err := Register(validReg(name)); err != nil {
			t.Fatal(err)
		}
	}

	all := All()
	if len(all) != 3 {
		t.Fatalf("expected 3 registrations, got %d", len(all))
	}
	expected := []string{"alpha", "bravo", "charlie"}
	for i, want := range expected {
		if all[i].Name != want {
			t.Errorf("All()[%d].Name = %q, want %q", i, all[i].Name, want)
		}
	}
}

// TestNames verifies that Names returns sorted plugin names.
//
// VALIDATES: Names returns alphabetically sorted name list.
// PREVENTS: Unstable ordering in iteration.
func TestNames(t *testing.T) {
	t.Cleanup(func() { Reset() })

	for _, name := range []string{"zulu", "alpha"} {
		if err := Register(validReg(name)); err != nil {
			t.Fatal(err)
		}
	}

	names := Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "zulu" {
		t.Errorf("expected [alpha zulu], got %v", names)
	}
}

// TestFamilyMap verifies that FamilyMap aggregates families from all plugins.
//
// VALIDATES: FamilyMap returns family->plugin mapping from all registrations.
// PREVENTS: Missing families when multiple plugins register different families.
func TestFamilyMap(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("flowspec")
	reg1.Families = []string{"ipv4/flow", "ipv6/flow"}
	reg2 := validReg("evpn")
	reg2.Families = []string{"l2vpn/evpn"}

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	fm := FamilyMap()
	if fm["ipv4/flow"] != "flowspec" {
		t.Errorf("ipv4/flow -> %q, want flowspec", fm["ipv4/flow"])
	}
	if fm["ipv6/flow"] != "flowspec" {
		t.Errorf("ipv6/flow -> %q, want flowspec", fm["ipv6/flow"])
	}
	if fm["l2vpn/evpn"] != "evpn" {
		t.Errorf("l2vpn/evpn -> %q, want evpn", fm["l2vpn/evpn"])
	}
	if len(fm) != 3 {
		t.Errorf("expected 3 entries, got %d", len(fm))
	}
}

// TestCapabilityMap verifies that CapabilityMap aggregates capability codes.
//
// VALIDATES: CapabilityMap returns code->plugin mapping.
// PREVENTS: Missing capability decode routing.
func TestCapabilityMap(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("hostname")
	reg1.CapabilityCodes = []uint8{73}
	reg2 := validReg("llnh")
	reg2.CapabilityCodes = []uint8{77}

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	cm := CapabilityMap()
	if cm[73] != "hostname" {
		t.Errorf("code 73 -> %q, want hostname", cm[73])
	}
	if cm[77] != "llnh" {
		t.Errorf("code 77 -> %q, want llnh", cm[77])
	}
}

// TestInProcessDecoders verifies that only plugins with decoders are included.
//
// VALIDATES: InProcessDecoders filters to plugins with non-nil InProcessDecoder.
// PREVENTS: Nil decoder invocation for plugins without decode support.
func TestInProcessDecoders(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("withDecoder")
	reg1.InProcessDecoder = dummyDecoder
	reg2 := validReg("withoutDecoder")

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	decoders := InProcessDecoders()
	if len(decoders) != 1 {
		t.Fatalf("expected 1 decoder, got %d", len(decoders))
	}
	if decoders["withDecoder"] == nil {
		t.Error("expected decoder for withDecoder")
	}
}

// TestYANGSchemas verifies that only plugins with YANG are included.
//
// VALIDATES: YANGSchemas filters to plugins with non-empty YANG.
// PREVENTS: Empty schema entries in config validation.
func TestYANGSchemas(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("withYANG")
	reg1.YANG = "module test { }"
	reg2 := validReg("withoutYANG")

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	schemas := YANGSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas["withYANG"] != "module test { }" {
		t.Errorf("unexpected YANG: %q", schemas["withYANG"])
	}
}

// TestConfigRootsMap verifies that only plugins with config roots are included.
//
// VALIDATES: ConfigRootsMap filters to plugins that declared config roots.
// PREVENTS: Spurious config delivery to plugins that don't want it.
func TestConfigRootsMap(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("wantsConfig")
	reg1.ConfigRoots = []string{"bgp"}
	reg2 := validReg("noConfig")

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	crm := ConfigRootsMap()
	if len(crm) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(crm))
	}
	if len(crm["wantsConfig"]) != 1 || crm["wantsConfig"][0] != "bgp" {
		t.Errorf("unexpected config roots: %v", crm["wantsConfig"])
	}
}

// TestPluginForFamily verifies family lookup returns correct plugin.
//
// VALIDATES: PluginForFamily returns the plugin handling a family.
// PREVENTS: Incorrect family routing.
func TestPluginForFamily(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("vpn")
	reg.Families = []string{"ipv4/vpn", "ipv6/vpn"}
	if err := Register(reg); err != nil {
		t.Fatal(err)
	}

	if got := PluginForFamily("ipv4/vpn"); got != "vpn" {
		t.Errorf("ipv4/vpn -> %q, want vpn", got)
	}
	if got := PluginForFamily("unknown/fam"); got != "" {
		t.Errorf("unknown/fam -> %q, want empty", got)
	}
}

// TestRequiredPlugins verifies deduplication of required plugins.
//
// VALIDATES: RequiredPlugins returns unique plugin names for given families.
// PREVENTS: Duplicate plugin starts for multi-family plugins.
func TestRequiredPlugins(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("flowspec")
	reg.Families = []string{"ipv4/flow", "ipv6/flow"}
	if err := Register(reg); err != nil {
		t.Fatal(err)
	}

	got := RequiredPlugins([]string{"ipv4/flow", "ipv6/flow", "ipv4/unicast"})
	if len(got) != 1 || got[0] != "flowspec" {
		t.Errorf("expected [flowspec], got %v", got)
	}
}

// TestWriteUsage verifies help text generation.
//
// VALIDATES: WriteUsage produces formatted, sorted output.
// PREVENTS: Broken help text layout.
func TestWriteUsage(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg1 := validReg("gr")
	reg1.Description = "Graceful Restart"
	reg1.RFCs = []string{"4724"}
	reg2 := validReg("flowspec")
	reg2.Description = "FlowSpec NLRI"
	reg2.RFCs = []string{"8955", "8956"}

	if err := Register(reg1); err != nil {
		t.Fatal(err)
	}
	if err := Register(reg2); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := WriteUsage(&buf); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	// flowspec comes before gr alphabetically.
	if !bytes.Contains([]byte(out), []byte("flowspec")) {
		t.Error("missing flowspec in output")
	}
	if !bytes.Contains([]byte(out), []byte("gr")) {
		t.Error("missing gr in output")
	}
	if !bytes.Contains([]byte(out), []byte("RFC 4724")) {
		t.Error("missing RFC 4724 in output")
	}
	if !bytes.Contains([]byte(out), []byte("RFC 8955, 8956")) {
		t.Error("missing RFC 8955, 8956 in output")
	}
}

// TestWriteUsageEmpty verifies no output when registry is empty.
//
// VALIDATES: WriteUsage returns nil with no output for empty registry.
// PREVENTS: Spurious output or errors when no plugins registered.
func TestWriteUsageEmpty(t *testing.T) {
	t.Cleanup(func() { Reset() })

	var buf bytes.Buffer
	if err := WriteUsage(&buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

// TestReset verifies that Reset clears all registrations.
//
// VALIDATES: Reset removes all plugins from the registry.
// PREVENTS: Test pollution between test cases.
func TestReset(t *testing.T) {
	if err := Register(validReg("temp")); err != nil {
		t.Fatal(err)
	}
	Reset()

	if Has("temp") {
		t.Error("expected empty registry after Reset")
	}
	if len(All()) != 0 {
		t.Errorf("expected 0 registrations, got %d", len(All()))
	}
}

// --- Dependencies tests ---

// TestRegisterWithDependencies verifies that Dependencies are stored and retrievable.
//
// VALIDATES: Dependencies field stored on registration, retrievable via Lookup.
// PREVENTS: Dependencies silently dropped during registration.
func TestRegisterWithDependencies(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("plugin-a")
	reg.Dependencies = []string{"plugin-b"}
	if err := Register(reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := Lookup("plugin-a")
	if got == nil {
		t.Fatal("expected registration, got nil")
		return
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != "plugin-b" {
		t.Errorf("expected Dependencies=[plugin-b], got %v", got.Dependencies)
	}
}

// TestRegisterSelfDependency verifies that a plugin cannot depend on itself.
//
// VALIDATES: Self-dependency rejected with ErrSelfDependency.
// PREVENTS: Infinite loops in dependency resolution.
func TestRegisterSelfDependency(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("self-dep")
	reg.Dependencies = []string{"self-dep"}
	err := Register(reg)
	if !errors.Is(err, ErrSelfDependency) {
		t.Errorf("expected ErrSelfDependency, got %v", err)
	}
}

// TestRegisterEmptyDependencyName verifies that empty dependency names are rejected.
//
// VALIDATES: Empty string in Dependencies rejected with ErrEmptyDependency.
// PREVENTS: Invisible dependencies that can never be resolved.
func TestRegisterEmptyDependencyName(t *testing.T) {
	t.Cleanup(func() { Reset() })

	reg := validReg("has-empty-dep")
	reg.Dependencies = []string{""}
	err := Register(reg)
	if !errors.Is(err, ErrEmptyDependency) {
		t.Errorf("expected ErrEmptyDependency, got %v", err)
	}
}

// TestResolveDependencies_NoDeps verifies that plugins without deps return unchanged.
//
// VALIDATES: ResolveDependencies returns same list when no deps declared.
// PREVENTS: Spurious plugin additions from empty resolution.
func TestResolveDependencies_NoDeps(t *testing.T) {
	t.Cleanup(func() { Reset() })

	if err := Register(validReg("standalone")); err != nil {
		t.Fatal(err)
	}

	result, err := ResolveDependencies([]string{"standalone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0] != "standalone" {
		t.Errorf("expected [standalone], got %v", result)
	}
}

// TestResolveDependencies_DirectDep verifies that a direct dependency is added.
//
// VALIDATES: A depends on B → both in result.
// PREVENTS: Missing direct dependencies in expanded list.
func TestResolveDependencies_DirectDep(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")

	if err := Register(regA); err != nil {
		t.Fatal(err)
	}
	if err := Register(regB); err != nil {
		t.Fatal(err)
	}

	result, err := ResolveDependencies([]string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	has := make(map[string]bool)
	for _, n := range result {
		has[n] = true
	}
	if !has["a"] || !has["b"] {
		t.Errorf("expected [a, b], got %v", result)
	}
}

// TestResolveDependencies_TransitiveDep verifies transitive deps are resolved.
//
// VALIDATES: A→B→C: all three in result.
// PREVENTS: Transitive dependencies not followed.
func TestResolveDependencies_TransitiveDep(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")
	regB.Dependencies = []string{"c"}
	regC := validReg("c")

	for _, r := range []Registration{regA, regB, regC} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ResolveDependencies([]string{"a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	has := make(map[string]bool)
	for _, n := range result {
		has[n] = true
	}
	if !has["a"] || !has["b"] || !has["c"] {
		t.Errorf("expected [a, b, c], got %v", result)
	}
}

// TestResolveDependencies_AlreadyPresent verifies no duplicates when dep already requested.
//
// VALIDATES: Both A and B requested, A depends on B → B appears once.
// PREVENTS: Duplicate plugin entries in expanded list.
func TestResolveDependencies_AlreadyPresent(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")

	if err := Register(regA); err != nil {
		t.Fatal(err)
	}
	if err := Register(regB); err != nil {
		t.Fatal(err)
	}

	result, err := ResolveDependencies([]string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for _, n := range result {
		if n == "b" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected b once, found %d times in %v", count, result)
	}
}

// TestResolveDependencies_CircularDep verifies circular deps are detected.
//
// VALIDATES: A→B→A returns ErrCircularDependency.
// PREVENTS: Infinite loops in dependency resolution.
func TestResolveDependencies_CircularDep(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")
	regB.Dependencies = []string{"a"}

	if err := Register(regA); err != nil {
		t.Fatal(err)
	}
	if err := Register(regB); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveDependencies([]string{"a"})
	if !errors.Is(err, ErrCircularDependency) {
		t.Errorf("expected ErrCircularDependency, got %v", err)
	}
}

// TestResolveDependencies_MissingDep verifies unknown deps are detected.
//
// VALIDATES: A depends on unknown → returns ErrMissingDependency.
// PREVENTS: Silent skip of unregistered dependencies.
func TestResolveDependencies_MissingDep(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"unknown"}

	if err := Register(regA); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveDependencies([]string{"a"})
	if !errors.Is(err, ErrMissingDependency) {
		t.Errorf("expected ErrMissingDependency, got %v", err)
	}
}

// --- TopologicalTiers tests ---

// TestTopologicalTiers verifies correct tier assignment for a direct dependency.
//
// VALIDATES: bgp-adj-rib-in (no deps) = tier 0, bgp-rs (depends on bgp-adj-rib-in) = tier 1.
// PREVENTS: Dependent plugins starting in same tier as their dependencies.
func TestTopologicalTiers(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regRIB := validReg("bgp-adj-rib-in")
	regRS := validReg("bgp-rs")
	regRS.Dependencies = []string{"bgp-adj-rib-in"}

	for _, r := range []Registration{regRIB, regRS} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	tiers, err := TopologicalTiers([]string{"bgp-adj-rib-in", "bgp-rs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 1 || tiers[0][0] != "bgp-adj-rib-in" {
		t.Errorf("tier 0: expected [bgp-adj-rib-in], got %v", tiers[0])
	}
	if len(tiers[1]) != 1 || tiers[1][0] != "bgp-rs" {
		t.Errorf("tier 1: expected [bgp-rs], got %v", tiers[1])
	}
}

// TestTopologicalTiersCycle verifies cycle detection returns an error.
//
// VALIDATES: A→B→A returns ErrCircularDependency.
// PREVENTS: Infinite loops or deadlocks from cyclic plugin dependencies.
func TestTopologicalTiersCycle(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")
	regB.Dependencies = []string{"a"}

	for _, r := range []Registration{regA, regB} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	_, err := TopologicalTiers([]string{"a", "b"})
	if !errors.Is(err, ErrCircularDependency) {
		t.Errorf("expected ErrCircularDependency, got %v", err)
	}
}

// TestTopologicalTiersNoDeps verifies all plugins land in tier 0 when none have deps.
//
// VALIDATES: Independent plugins all in tier 0.
// PREVENTS: Spurious tier creation for plugins without dependencies.
func TestTopologicalTiersNoDeps(t *testing.T) {
	t.Cleanup(func() { Reset() })

	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if err := Register(validReg(name)); err != nil {
			t.Fatal(err)
		}
	}

	tiers, err := TopologicalTiers([]string{"alpha", "bravo", "charlie"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 3 {
		t.Errorf("tier 0: expected 3 plugins, got %d: %v", len(tiers[0]), tiers[0])
	}
}

// TestTopologicalTiersTransitive verifies transitive chain A→B→C produces 3 tiers.
//
// VALIDATES: A→B→C produces [[C], [B], [A]] (C=tier0, B=tier1, A=tier2).
// PREVENTS: Transitive dependencies collapsing into fewer tiers.
func TestTopologicalTiersTransitive(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"b"}
	regB := validReg("b")
	regB.Dependencies = []string{"c"}
	regC := validReg("c")

	for _, r := range []Registration{regA, regB, regC} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	tiers, err := TopologicalTiers([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 1 || tiers[0][0] != "c" {
		t.Errorf("tier 0: expected [c], got %v", tiers[0])
	}
	if len(tiers[1]) != 1 || tiers[1][0] != "b" {
		t.Errorf("tier 1: expected [b], got %v", tiers[1])
	}
	if len(tiers[2]) != 1 || tiers[2][0] != "a" {
		t.Errorf("tier 2: expected [a], got %v", tiers[2])
	}
}

// TestTopologicalTiersMultipleSameTier verifies siblings share a tier.
//
// VALIDATES: B→A, C→A produces [[A], [B,C]].
// PREVENTS: Siblings unnecessarily serialized into separate tiers.
func TestTopologicalTiersMultipleSameTier(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regB := validReg("b")
	regB.Dependencies = []string{"a"}
	regC := validReg("c")
	regC.Dependencies = []string{"a"}

	for _, r := range []Registration{regA, regB, regC} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	tiers, err := TopologicalTiers([]string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 1 || tiers[0][0] != "a" {
		t.Errorf("tier 0: expected [a], got %v", tiers[0])
	}
	if len(tiers[1]) != 2 {
		t.Errorf("tier 1: expected 2 plugins, got %d: %v", len(tiers[1]), tiers[1])
	}
	// Check both b and c are in tier 1 (order within tier is sorted)
	tier1 := make(map[string]bool)
	for _, name := range tiers[1] {
		tier1[name] = true
	}
	if !tier1["b"] || !tier1["c"] {
		t.Errorf("tier 1: expected {b, c}, got %v", tiers[1])
	}
}

// TestTopologicalTiersUnknownPlugin verifies external plugins go to tier 0.
//
// VALIDATES: Plugin not in registry is placed in tier 0.
// PREVENTS: External plugins being excluded or causing errors.
func TestTopologicalTiersUnknownPlugin(t *testing.T) {
	t.Cleanup(func() { Reset() })

	// Only register "known", not "external"
	if err := Register(validReg("known")); err != nil {
		t.Fatal(err)
	}

	tiers, err := TopologicalTiers([]string{"external", "known"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 2 {
		t.Errorf("tier 0: expected 2 plugins, got %d: %v", len(tiers[0]), tiers[0])
	}
}

// TestPluginForEventType verifies event type lookup returns correct plugin.
//
// VALIDATES: PluginForEventType returns the plugin that declares a given event type.
// PREVENTS: Incorrect event type routing or missing lookups for registered/unregistered types.
func TestPluginForEventType(t *testing.T) {
	t.Cleanup(func() { Reset() })

	tests := []struct {
		name      string
		setup     func()
		eventType string
		want      string
	}{
		{
			name: "returns plugin name when event type matches",
			setup: func() {
				reg := validReg("rpki-decorator")
				reg.EventTypes = []string{"update-rpki", "rpki"}
				if err := Register(reg); err != nil {
					t.Fatal(err)
				}
			},
			eventType: "update-rpki",
			want:      "rpki-decorator",
		},
		{
			name: "returns empty when no plugin has that event type",
			setup: func() {
				reg := validReg("some-plugin")
				reg.EventTypes = []string{"update-rpki"}
				if err := Register(reg); err != nil {
					t.Fatal(err)
				}
			},
			eventType: "nonexistent-event",
			want:      "",
		},
		{
			name:      "returns empty when registry has no plugins with event types",
			setup:     func() {},
			eventType: "update-rpki",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reset()
			tt.setup()
			got := PluginForEventType(tt.eventType)
			if got != tt.want {
				t.Errorf("PluginForEventType(%q) = %q, want %q", tt.eventType, got, tt.want)
			}
		})
	}
}

// TestResolveDependencies_Diamond verifies diamond deps produce no duplicates.
//
// VALIDATES: A→C, B→C: C appears once in result.
// PREVENTS: Duplicate entries from diamond dependency graphs.
func TestResolveDependencies_Diamond(t *testing.T) {
	t.Cleanup(func() { Reset() })

	regA := validReg("a")
	regA.Dependencies = []string{"c"}
	regB := validReg("b")
	regB.Dependencies = []string{"c"}
	regC := validReg("c")

	for _, r := range []Registration{regA, regB, regC} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ResolveDependencies([]string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for _, n := range result {
		if n == "c" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected c once, found %d times in %v", count, result)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 plugins, got %d: %v", len(result), result)
	}
}

// --- ModAccumulator tests ---

// VALIDATES: AC-5 — ModAccumulator.Len() returns 0 when empty, no allocation.
// PREVENTS: Accidental allocation on the zero-mod path.
func TestModAccumulator_LazyAlloc(t *testing.T) {
	var mods ModAccumulator
	if mods.Len() != 0 {
		t.Fatalf("empty ModAccumulator.Len() = %d, want 0", mods.Len())
	}
	if ops := mods.Ops(); len(ops) != 0 {
		t.Fatalf("empty ModAccumulator.Ops() returned %d ops", len(ops))
	}
	// Op triggers allocation.
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0x00, 0x64})
	if mods.Len() != 1 {
		t.Fatalf("after Op, Len() = %d, want 1", mods.Len())
	}
}

// VALIDATES: AC-6 — Multiple Op calls accumulated, all retrievable.
// PREVENTS: Overwrite or loss of mods from different filters.
func TestModAccumulator_MultipleOps(t *testing.T) {
	var mods ModAccumulator
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0x00, 0x00})
	mods.Op(8, AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x01})
	mods.Op(8, AttrModRemove, []byte{0xFF, 0xFF, 0xFF, 0x03})

	if mods.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", mods.Len())
	}

	ops := mods.Ops()
	if len(ops) != 3 {
		t.Fatalf("Ops() len = %d, want 3", len(ops))
	}
	if ops[0].Code != 35 || ops[0].Action != AttrModSet {
		t.Fatalf("ops[0] = {%d, %d}, want {35, AttrModSet}", ops[0].Code, ops[0].Action)
	}
	if ops[1].Code != 8 || ops[1].Action != AttrModAdd {
		t.Fatalf("ops[1] = {%d, %d}, want {8, AttrModAdd}", ops[1].Code, ops[1].Action)
	}
	if ops[2].Code != 8 || ops[2].Action != AttrModRemove {
		t.Fatalf("ops[2] = {%d, %d}, want {8, AttrModRemove}", ops[2].Code, ops[2].Action)
	}

	// Reset clears.
	mods.Reset()
	if mods.Len() != 0 {
		t.Fatalf("after Reset, Len() = %d, want 0", mods.Len())
	}
}

// --- AttrOp / AttrModHandler tests (v2 progressive build) ---

// VALIDATES: AC-11 — AttrOp holds code, action, buf fields.
// PREVENTS: Wrong structure for mod accumulation.
func TestAttrOpStructure(t *testing.T) {
	op := AttrOp{
		Code:   35,
		Action: AttrModSet,
		Buf:    []byte{0x00, 0x00, 0xFD, 0xE8}, // ASN 65000
	}
	if op.Code != 35 {
		t.Fatalf("Code = %d, want 35", op.Code)
	}
	if op.Action != AttrModSet {
		t.Fatalf("Action = %d, want AttrModSet", op.Action)
	}
	if len(op.Buf) != 4 {
		t.Fatalf("Buf len = %d, want 4", len(op.Buf))
	}
}

// VALIDATES: AC-11 — ModAccumulator.Op() stores AttrOp entries, Len() reflects count.
// PREVENTS: Op() not accumulating, or Len() wrong.
func TestModAccumulatorOp(t *testing.T) {
	var mods ModAccumulator
	if mods.Len() != 0 {
		t.Fatalf("empty Len() = %d, want 0", mods.Len())
	}

	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0xFD, 0xE8})
	if mods.Len() != 1 {
		t.Fatalf("after Op, Len() = %d, want 1", mods.Len())
	}

	// Multiple ops on same code accumulate separately.
	mods.Op(8, AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x01})
	mods.Op(8, AttrModRemove, []byte{0xFF, 0xFF, 0xFF, 0x03})
	if mods.Len() != 3 {
		t.Fatalf("after 3 ops, Len() = %d, want 3", mods.Len())
	}

	ops := mods.Ops()
	if len(ops) != 3 {
		t.Fatalf("Ops() len = %d, want 3", len(ops))
	}
	if ops[0].Code != 35 || ops[0].Action != AttrModSet {
		t.Fatalf("ops[0] = {%d, %d}, want {35, AttrModSet}", ops[0].Code, ops[0].Action)
	}
	if ops[1].Code != 8 || ops[1].Action != AttrModAdd {
		t.Fatalf("ops[1] = {%d, %d}, want {8, AttrModAdd}", ops[1].Code, ops[1].Action)
	}
	if ops[2].Code != 8 || ops[2].Action != AttrModRemove {
		t.Fatalf("ops[2] = {%d, %d}, want {8, AttrModRemove}", ops[2].Code, ops[2].Action)
	}
}

// VALIDATES: AC-11 — ModAccumulator.Reset() clears ops for reuse.
// PREVENTS: Stale ops leaking between peers.
func TestModAccumulatorOpReset(t *testing.T) {
	var mods ModAccumulator
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0xFD, 0xE8})
	mods.Reset()
	if mods.Len() != 0 {
		t.Fatalf("after Reset, Len() = %d, want 0", mods.Len())
	}
	if ops := mods.Ops(); len(ops) != 0 {
		t.Fatalf("after Reset, Ops() len = %d, want 0", len(ops))
	}
}

// VALIDATES: AC-12 — AttrModHandler registered by attr code, retrievable.
// PREVENTS: Handler registration lost or wrong code mapping.
func TestAttrModHandlerRegistration(t *testing.T) {
	called := false
	handler := AttrModHandler(func(src []byte, ops []AttrOp, buf []byte, off int) int {
		called = true
		return off
	})

	RegisterAttrModHandler(35, handler)
	t.Cleanup(func() { UnregisterAttrModHandler(35) })

	got := AttrModHandlerFor(35)
	if got == nil {
		t.Fatal("AttrModHandlerFor returned nil for registered code")
	}

	buf := make([]byte, 64)
	got(nil, nil, buf, 0)
	if !called {
		t.Fatal("handler was not called")
	}
}

// VALIDATES: AC-18 — Unknown attr code returns nil handler.
// PREVENTS: Panic on unregistered code lookup.
func TestAttrModHandlerNotFound(t *testing.T) {
	got := AttrModHandlerFor(99)
	if got != nil {
		t.Fatal("AttrModHandlerFor returned non-nil for unregistered code")
	}
}

// VALIDATES: AC-12 — AttrModHandlers returns snapshot for reactor startup.
// PREVENTS: Reactor sharing mutable reference with registry.
func TestAttrModHandlersSnapshot(t *testing.T) {
	h := AttrModHandler(func(src []byte, ops []AttrOp, buf []byte, off int) int { return off })

	RegisterAttrModHandler(200, h)
	RegisterAttrModHandler(201, h)
	t.Cleanup(func() {
		UnregisterAttrModHandler(200)
		UnregisterAttrModHandler(201)
	})

	snap := AttrModHandlers()
	if snap[200] == nil || snap[201] == nil {
		t.Fatal("snapshot missing registered handlers")
	}

	// Mutating snapshot must not affect registry.
	delete(snap, 200)
	if AttrModHandlerFor(200) == nil {
		t.Fatal("deleting from snapshot affected the registry")
	}
}

// VALIDATES: RegisterAttrModHandler ignores nil handler.
// PREVENTS: Nil handler registered leading to panic in progressive build.
func TestRegisterAttrModHandlerNil(t *testing.T) {
	RegisterAttrModHandler(250, nil)
	t.Cleanup(func() { UnregisterAttrModHandler(250) })

	got := AttrModHandlerFor(250)
	if got != nil {
		t.Fatal("nil handler should not be registered")
	}
}

// TestFilterPriorityOrdering verifies that IngressFilters and EgressFilters return
// filters sorted by FilterStage first, then FilterPriority, then by plugin name.
//
// VALIDATES: AC-12 -- filters execute in stage+priority order.
// PREVENTS: Non-deterministic filter ordering from map iteration.
func TestFilterPriorityOrdering(t *testing.T) {
	t.Cleanup(func() { Reset() })

	// Register four plugins across stages and priorities.
	// "community" stage Policy priority 0, "loop" stage Protocol priority 0,
	// "otc" stage Annotation priority 0, "prefix" stage Policy priority 10.
	// Expected order: loop (Protocol/0), community (Policy/0), prefix (Policy/10), otc (Annotation/0).
	noop := func(_ PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) { return true, nil }

	regCommunity := validReg("community")
	regCommunity.FilterStage = FilterStagePolicy
	regCommunity.FilterPriority = 0
	regCommunity.IngressFilter = noop
	if err := Register(regCommunity); err != nil {
		t.Fatal(err)
	}

	regLoop := validReg("loop")
	regLoop.FilterStage = FilterStageProtocol
	regLoop.FilterPriority = 0
	regLoop.IngressFilter = noop
	if err := Register(regLoop); err != nil {
		t.Fatal(err)
	}

	regOTC := validReg("otc")
	regOTC.FilterStage = FilterStageAnnotation
	regOTC.FilterPriority = 0
	regOTC.IngressFilter = noop
	if err := Register(regOTC); err != nil {
		t.Fatal(err)
	}

	regPrefix := validReg("prefix")
	regPrefix.FilterStage = FilterStagePolicy
	regPrefix.FilterPriority = 10
	regPrefix.IngressFilter = noop
	if err := Register(regPrefix); err != nil {
		t.Fatal(err)
	}

	names := IngressFilterNames()
	if len(names) != 4 {
		t.Fatalf("IngressFilterNames() len = %d, want 4", len(names))
	}
	want := []string{"loop", "community", "prefix", "otc"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q (full order: %v)", i, names[i], w, names)
		}
	}
}

// TestFilterSameStageNameBreaksTie verifies that name is the tiebreaker
// when both stage and priority are equal.
//
// VALIDATES: Deterministic ordering within identical stage+priority.
// PREVENTS: Random ordering from map iteration when priorities match.
func TestFilterSameStageNameBreaksTie(t *testing.T) {
	t.Cleanup(func() { Reset() })

	noop := func(_ PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) { return true, nil }

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		reg := validReg(name)
		reg.FilterStage = FilterStagePolicy
		reg.FilterPriority = 0
		reg.IngressFilter = noop
		if err := Register(reg); err != nil {
			t.Fatal(err)
		}
	}

	names := IngressFilterNames()
	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

// TestPeerFilterInfoFields verifies that PeerFilterInfo has Name and GroupName fields.
//
// VALIDATES: AC-20 -- PeerFilterInfo includes peer identity fields.
// PREVENTS: Filters building their own address-to-name lookup maps.
func TestPeerFilterInfoFields(t *testing.T) {
	info := PeerFilterInfo{
		Name:      "upstream-1",
		GroupName: "transit",
	}
	if info.Name != "upstream-1" {
		t.Errorf("Name = %q, want %q", info.Name, "upstream-1")
	}
	if info.GroupName != "transit" {
		t.Errorf("GroupName = %q, want %q", info.GroupName, "transit")
	}
}
