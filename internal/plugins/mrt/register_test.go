// VALIDATES: MRT config parsing (ParseConfig) across every dump mode, the
// direction/peer-filter/flag fields, and the plugin's registry wiring.
// PREVENTS: a malformed or mode-specific MRT config silently parsing to the
// wrong Config, and the mrt plugin losing its registry entry / config root.
package mrt

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func mustParse(t *testing.T, raw string) *Config {
	t.Helper()
	cfg, err := ParseConfig(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("ParseConfig(%s): %v", raw, err)
	}
	return cfg
}

func TestParseConfigUpdatesMode(t *testing.T) {
	cfg := mustParse(t, `{"updates":{"file":"/var/mrt/updates","interval":60}}`)
	if cfg.UpdatesPath != "/var/mrt/updates" {
		t.Errorf("UpdatesPath = %q, want /var/mrt/updates", cfg.UpdatesPath)
	}
	if cfg.UpdatesInterval != 60*time.Second {
		t.Errorf("UpdatesInterval = %v, want 60s", cfg.UpdatesInterval)
	}
	if !cfg.hasUpdates() || cfg.hasAll() || cfg.hasRoutes() {
		t.Errorf("mode flags wrong: updates=%v all=%v routes=%v", cfg.hasUpdates(), cfg.hasAll(), cfg.hasRoutes())
	}
}

func TestParseConfigAllMode(t *testing.T) {
	cfg := mustParse(t, `{"all":{"file":"/var/mrt/all","interval":30}}`)
	if cfg.AllPath != "/var/mrt/all" || cfg.AllInterval != 30*time.Second {
		t.Errorf("all: got path=%q interval=%v", cfg.AllPath, cfg.AllInterval)
	}
	if !cfg.hasAll() {
		t.Error("HasAll false for all-mode config")
	}
}

func TestParseConfigRoutesMode(t *testing.T) {
	cfg := mustParse(t, `{"routes":{"file":"/var/mrt/rib","interval":900}}`)
	if cfg.RoutesPath != "/var/mrt/rib" || cfg.RoutesInterval != 900*time.Second {
		t.Errorf("routes: got path=%q interval=%v", cfg.RoutesPath, cfg.RoutesInterval)
	}
	// HasRoutes requires BOTH a path and a positive interval.
	if !cfg.hasRoutes() {
		t.Error("HasRoutes false for routes-mode config")
	}
}

func TestParseConfigRoutesNoIntervalNotActive(t *testing.T) {
	// A routes path with interval 0 must not enable RIB dumps (HasRoutes gates on interval).
	cfg := mustParse(t, `{"routes":{"file":"/var/mrt/rib","interval":0}}`)
	if cfg.hasRoutes() {
		t.Error("HasRoutes true despite interval=0")
	}
}

func TestParseConfigPeerFilterAndFlags(t *testing.T) {
	cfg := mustParse(t, `{"updates":{"file":"/x","interval":1},"peer-filter":["10.0.0.1","2001:db8::1"],"extended-timestamp":true,"add-path":true}`)
	if len(cfg.PeerFilter) != 2 || cfg.PeerFilter[0] != "10.0.0.1" {
		t.Errorf("PeerFilter = %v, want [10.0.0.1 2001:db8::1]", cfg.PeerFilter)
	}
	if !cfg.ExtendedTimestamp || !cfg.AddPath {
		t.Errorf("flags: extTS=%v addPath=%v, want both true", cfg.ExtendedTimestamp, cfg.AddPath)
	}
}

func TestParseConfigDirection(t *testing.T) {
	cases := map[string]string{
		`{"updates":{"file":"/x","interval":1},"direction":"received"}`: "received",
		`{"updates":{"file":"/x","interval":1},"direction":"sent"}`:     "sent",
		`{"updates":{"file":"/x","interval":1},"direction":"both"}`:     "", // "both" normalizes to empty
		`{"updates":{"file":"/x","interval":1}}`:                        "", // absent
	}
	for raw, want := range cases {
		if got := mustParse(t, raw).Direction; got != want {
			t.Errorf("ParseConfig(%s).Direction = %q, want %q", raw, got, want)
		}
	}
}

// TestParseConfigWrappedRoot guards the production delivery format: the plugin
// server hands mrt its section wrapped in the config root, e.g.
// {"mrt":{"updates":{...}}}. ParseConfig must unwrap "mrt" or the plugin sees an
// empty config and stays idle (the never-worked bug this test pins down).
func TestParseConfigWrappedRoot(t *testing.T) {
	cfg := mustParse(t, `{"mrt":{"updates":{"file":"/var/mrt/updates","interval":60}}}`)
	if cfg.UpdatesPath != "/var/mrt/updates" || cfg.UpdatesInterval != 60*time.Second {
		t.Fatalf("wrapped-root config not unwrapped: path=%q interval=%v", cfg.UpdatesPath, cfg.UpdatesInterval)
	}
	if cfg.IsEmpty() {
		t.Fatal("wrapped-root config parsed as empty (plugin would go idle)")
	}
}

func TestParseConfigEmpty(t *testing.T) {
	cfg := mustParse(t, `{}`)
	if !cfg.IsEmpty() {
		t.Errorf("empty config not IsEmpty: %+v", cfg)
	}
}

func TestParseConfigInvalidJSON(t *testing.T) {
	if _, err := ParseConfig(json.RawMessage(`{bad`)); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestMRTPluginRegistered(t *testing.T) {
	reg := registry.Lookup("mrt")
	if reg == nil {
		t.Fatal("mrt plugin not registered")
	}
	if reg.RunEngine == nil {
		t.Error("mrt registration has no RunEngine")
	}
	if reg.YANG == "" {
		t.Error("mrt registration carries no YANG schema")
	}
	found := false
	for _, r := range reg.ConfigRoots {
		if r == configRoot {
			found = true
		}
	}
	if !found {
		t.Errorf("mrt ConfigRoots = %v, want to include %q", reg.ConfigRoots, configRoot)
	}
}
