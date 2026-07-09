package bridgeplugin

import (
	"testing"
)

// TestParseConfigNested verifies the runner parses the nested
// `exabgp { bridge { ... } }` config root (spec-followup-subsystem AC-1, user
// directive 2026-07-09: nested shape, registry name stays exabgp-bridge).
func TestParseConfigNested(t *testing.T) {
	data := `{"exabgp":{"bridge":{"run":"./plugin.py arg","family":["ipv4/unicast","ipv6/unicast"],"route-refresh":"true","add-path":"receive"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Present {
		t.Fatalf("expected Present=true")
	}
	if cfg.Run != "./plugin.py arg" {
		t.Errorf("Run = %q, want %q", cfg.Run, "./plugin.py arg")
	}
	if len(cfg.Families) != 2 || cfg.Families[0] != "ipv4/unicast" || cfg.Families[1] != "ipv6/unicast" {
		t.Errorf("Families = %v", cfg.Families)
	}
	if !cfg.RouteRefresh {
		t.Errorf("RouteRefresh = false, want true")
	}
	if cfg.AddPath != "receive" {
		t.Errorf("AddPath = %q, want receive", cfg.AddPath)
	}
}

// TestParseConfigDefaults verifies an exabgp.bridge with only `run` gets the
// default family and add-path=none.
func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(`{"exabgp":{"bridge":{"run":"./plugin.py"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Present {
		t.Fatalf("expected Present=true")
	}
	if len(cfg.Families) != 1 || cfg.Families[0] != defaultFamily {
		t.Errorf("Families = %v, want [%s]", cfg.Families, defaultFamily)
	}
	if cfg.AddPath != addPathNone {
		t.Errorf("AddPath = %q, want none", cfg.AddPath)
	}
	if cfg.RouteRefresh {
		t.Errorf("RouteRefresh = true, want false")
	}
}

// TestParseConfigAbsent verifies a section without the exabgp.bridge container
// yields Present=false with no error.
func TestParseConfigAbsent(t *testing.T) {
	cfg, err := parseConfig(`{"exabgp":{}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Present {
		t.Errorf("expected Present=false for empty exabgp root")
	}

	cfg, err = parseConfig(`{"other":{"x":"y"}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Present {
		t.Errorf("expected Present=false for unrelated root")
	}
}

// TestParseConfigInvalidFamily rejects an unregistered address family.
func TestParseConfigInvalidFamily(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"run":"./p.py","family":["bogus/family"]}}}`)
	if err == nil {
		t.Fatalf("expected error for invalid family")
	}
}

// TestParseConfigInvalidAddPath rejects an out-of-range add-path mode.
func TestParseConfigInvalidAddPath(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"run":"./p.py","add-path":"sideways"}}}`)
	if err == nil {
		t.Fatalf("expected error for invalid add-path")
	}
}

// TestCapabilityDecls maps route-refresh and add-path config to BGP capability
// declarations (RFC 2918 code 2, RFC 7911 code 69).
func TestCapabilityDecls(t *testing.T) {
	caps := capabilityDecls(bridgeConfig{RouteRefresh: true, AddPath: addPathNone, Families: []string{defaultFamily}})
	if len(caps) != 1 || caps[0].Code != 2 {
		t.Fatalf("route-refresh caps = %+v, want one code=2", caps)
	}

	caps = capabilityDecls(bridgeConfig{AddPath: addPathReceive, Families: []string{defaultFamily}})
	if len(caps) != 1 || caps[0].Code != 69 {
		t.Fatalf("add-path caps = %+v, want one code=69", caps)
	}

	caps = capabilityDecls(bridgeConfig{AddPath: addPathNone, Families: []string{defaultFamily}})
	if len(caps) != 0 {
		t.Fatalf("no-cap config = %+v, want empty", caps)
	}
}

// TestFamilyDeclsDefault verifies familyDecls falls back to the default family
// when given none, and resolves configured families.
func TestFamilyDeclsDefault(t *testing.T) {
	decls := familyDecls(nil)
	if len(decls) != 1 || decls[0].Name != defaultFamily {
		t.Fatalf("default familyDecls = %+v, want [%s]", decls, defaultFamily)
	}

	decls = familyDecls([]string{"ipv4/unicast"})
	if len(decls) != 1 || decls[0].Name != "ipv4/unicast" {
		t.Fatalf("familyDecls = %+v", decls)
	}
}

// TestSplitCommand verifies whitespace argv splitting.
func TestSplitCommand(t *testing.T) {
	got := splitCommand("  python3   /opt/plugin.py  --flag ")
	want := []string{"python3", "/opt/plugin.py", "--flag"}
	if len(got) != len(want) {
		t.Fatalf("splitCommand = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCommand[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(splitCommand("   ")) != 0 {
		t.Errorf("splitCommand(blank) should be empty")
	}
}
