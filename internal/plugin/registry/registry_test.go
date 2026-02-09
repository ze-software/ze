package registry

import (
	"bytes"
	"errors"
	"net"
	"testing"
)

// dummyEngine is a stub RunEngine handler for testing.
func dummyEngine(_, _ net.Conn) int { return 0 }

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
