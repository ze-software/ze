package bridgeplugin

import (
	"strings"
	"testing"
)

// VALIDATES: the internal exabgp-bridge parses every `process` block an ExaBGP
// config declares, and refuses one it cannot run.
// PREVENTS: a config naming two scripts running one of them in silence.

// TestParseConfigNested verifies the runner parses the nested
// `exabgp { bridge { ... } }` config root (spec-followup-subsystem AC-1, user
// directive 2026-07-09: nested shape, registry name stays exabgp-bridge).
func TestParseConfigNested(t *testing.T) {
	data := `{"exabgp":{"bridge":{"process":{"main":{"run":"./plugin.py arg"}},"family":["ipv4/unicast","ipv6/unicast"],"route-refresh":"true","add-path":"receive"}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Present {
		t.Fatalf("expected Present=true")
	}
	if len(cfg.Scripts) != 1 {
		t.Fatalf("Scripts = %+v, want one", cfg.Scripts)
	}
	if got := cfg.Scripts[0]; got.Name != "main" || len(got.Argv) != 2 || got.Argv[0] != "./plugin.py" || got.Argv[1] != "arg" {
		t.Errorf("Scripts[0] = %+v, want main [./plugin.py arg]", got)
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

// TestParseConfigDefaults verifies an exabgp.bridge with only one process gets
// the default family, add-path=none, and ExaBGP's respawn default of true.
func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./plugin.py"}}}}}`)
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
	if len(cfg.Scripts) != 1 || !cfg.Scripts[0].Respawn {
		t.Errorf("Scripts = %+v, want one with Respawn=true", cfg.Scripts)
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
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"family":["bogus/family"]}}}`)
	if err == nil {
		t.Fatalf("expected error for invalid family")
	}
}

// TestParseConfigInvalidAddPath rejects an out-of-range add-path mode.
func TestParseConfigInvalidAddPath(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./p.py"}},"add-path":"sideways"}}}`)
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

// TestParseConfigTwoProcesses is the defect this list exists to remove: an
// ExaBGP config declaring two processes reaches the runner as two scripts,
// sorted by name, with the respawn setting each block asked for.
func TestParseConfigTwoProcesses(t *testing.T) {
	data := `{"exabgp":{"bridge":{"process":{` +
		`"one-shot":{"run":"./run/api-no-respawn-1.run","respawn":"false"},` +
		`"respawn":{"run":"./run/api-no-respawn-2.run"}}}}}`
	cfg, err := parseConfig(data)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if len(cfg.Scripts) != 2 {
		t.Fatalf("Scripts = %+v, want two", cfg.Scripts)
	}
	if cfg.Scripts[0].Name != "one-shot" || cfg.Scripts[1].Name != "respawn" {
		t.Fatalf("Scripts = %+v, want [one-shot respawn] sorted by name", cfg.Scripts)
	}
	if cfg.Scripts[0].Respawn {
		t.Errorf("one-shot Respawn = true, want false (the block says respawn false)")
	}
	if !cfg.Scripts[1].Respawn {
		t.Errorf("respawn Respawn = false, want true (no leaf means ExaBGP's default)")
	}
	if got := cfg.Scripts[1].Argv; len(got) != 1 || got[0] != "./run/api-no-respawn-2.run" {
		t.Errorf("respawn Argv = %v", got)
	}
}

// TestParseConfigProcessWithoutRun REFUSES a process block the bridge cannot
// run, and names it. Starting the other blocks and saying nothing about this
// one is the silently-wrong answer ai/rules/principles.md bans.
func TestParseConfigProcessWithoutRun(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"good":{"run":"./a.py"},"bad":{}}}}}`)
	if err == nil {
		t.Fatalf("expected a refusal for a process with no run command")
	}
	if !strings.Contains(err.Error(), `"bad"`) {
		t.Errorf("error = %v, want it to name the process bad", err)
	}
}

// TestParseConfigRespawnInvalid fails closed on a respawn value it cannot read,
// rather than reading it as ExaBGP's default of true.
func TestParseConfigRespawnInvalid(t *testing.T) {
	_, err := parseConfig(`{"exabgp":{"bridge":{"process":{"main":{"run":"./a.py","respawn":"sometimes"}}}}}`)
	if err == nil {
		t.Fatalf("expected a refusal for respawn=sometimes")
	}
}
