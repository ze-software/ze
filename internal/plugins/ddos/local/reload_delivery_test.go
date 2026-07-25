package local

import (
	"encoding/json"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"

	// The ddos-local YANG module imports ze-ddos-detect-conf, and the loader
	// resolves the whole module set, so both schemas must be registered for
	// LoadConfig to parse a `ddos {}` block. In the daemon the generated
	// composition root blank-imports them; here the test does it directly.
	_ "github.com/ze-software/ze/internal/plugins/ddos/detect/yang"
	_ "github.com/ze-software/ze/internal/plugins/ddos/local/yang"
	_ "github.com/ze-software/ze/internal/plugins/ddos/observe/yang"
)

// deliverSection reproduces the daemon's config-delivery chain for this plugin,
// end to end and with no stubs:
//
//	zeconfig.LoadConfig      (cmd/ze/hub/main.go:546 -- the reload loader)
//	  -> Tree.ToMap          (main.go:553)
//	  -> ExtractConfigSubtree(newTree, "ddos/local")
//	                         (plugin/server/reload.go:230, via startup.go:814)
//	  -> json.Marshal        (reload.go:237)
//	  -> ParseConfig         (ddos/local/register.go:69)
//
// Driving the real loader rather than a hand-built map is the point: the bug this
// file exists to pin lives in the SHAPE the loader produces for a boolean leaf,
// which a hand-built map would paper over.
func deliverSection(t *testing.T, text string) *Config {
	t.Helper()
	result, err := zeconfig.LoadConfig(text, "test.conf", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToMap(), configRoot)
	if subtree == nil {
		t.Fatalf("ExtractConfigSubtree(%q) returned nil; the plugin would be handed {}", configRoot)
	}
	data, err := json.Marshal(subtree)
	if err != nil {
		t.Fatalf("marshal subtree: %v", err)
	}
	t.Logf("delivered section for %s: %s", configRoot, data)
	cfg, err := ParseConfig(string(data))
	if err != nil {
		t.Fatalf("ParseConfig(%s): %v", data, err)
	}
	return cfg
}

const forwardMitigationOn = `ddos {
	local {
		response-level enforce
		forward-mitigation true
	}
}
`

const forwardMitigationOff = `ddos {
	local {
		response-level enforce
		forward-mitigation false
	}
}
`

// TestForwardMitigationSurvivesDelivery pins that BOTH polarities of the
// forward-mitigation leaf survive the loader -> extract -> marshal -> ParseConfig
// chain the daemon uses for the initial configure AND for every SIGHUP reload.
//
// VALIDATES: `forward-mitigation true` parses to true and `forward-mitigation
// false` parses to false, as delivered by the real config loader.
// PREVENTS: the QEMU failure in test/plugin/ddos-transit-forward-drop.ci Phase B
// -- after a SIGHUP that flips the leaf to false the responder still installed a
// FORWARD drop. A coercion that cannot read the delivered spelling leaves the
// struct field at its zero value for BOTH spellings, so the leaf silently does
// nothing and no reload can ever turn the feature off.
func TestForwardMitigationSurvivesDelivery(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"true", forwardMitigationOn, true},
		{"false", forwardMitigationOff, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := deliverSection(t, tc.text)
			if cfg.ForwardMitigation != tc.want {
				t.Errorf("ForwardMitigation = %v, want %v (the leaf did not survive delivery)", cfg.ForwardMitigation, tc.want)
			}
			if cfg.ResponseLevel != responseEnforce {
				t.Errorf("ResponseLevel = %q, want %q -- the section itself did not arrive", cfg.ResponseLevel, responseEnforce)
			}
		})
	}
}

// TestForwardMitigationReloadSelectsPlugin pins the SELECTION half of a SIGHUP
// reload: flipping the leaf must produce a diff that names this plugin's nested
// config root, so plugin/server/reload.go includes it in the affected set and
// hands it the new bytes.
//
// VALIDATES: DiffMaps over the two real trees reports a change under
// "ddos/local", and the subtree extracted for that root carries the NEW value.
// PREVENTS: a silent no-op reload -- reloadConfig returns early with "config
// reload: no changes" (reload.go:178-181) when the diff is empty, and that line
// logs at INFO on the plugin.server subsystem, so a .ci raising only
// ze.log.ddos.* sees a reload that reports success and changed nothing.
func TestForwardMitigationReloadSelectsPlugin(t *testing.T) {
	load := func(text string) map[string]any {
		t.Helper()
		result, err := zeconfig.LoadConfig(text, "test.conf", nil)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		return result.Tree.ToMap()
	}
	running := load(forwardMitigationOn)
	next := load(forwardMitigationOff)

	diff := zeconfig.DiffMaps(running, next)
	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		t.Fatal("flipping forward-mitigation produced an EMPTY diff: reloadConfig would return early and no plugin would ever be told")
	}

	// Mirrors rootHasChanges(diff, configRoot) (plugin/server/reload.go:297-319):
	// the key is either the root itself or sits under "<root>/".
	prefix := configRoot + zeconfig.PathSep
	selected := false
	for _, keys := range []map[string]bool{keySet(diff.Added), keySet(diff.Removed), keySet(diff.Changed)} {
		for k := range keys {
			if k == configRoot || len(k) > len(prefix) && k[:len(prefix)] == prefix {
				selected = true
				t.Logf("diff key selecting %s: %s", configRoot, k)
			}
		}
	}
	if !selected {
		t.Fatalf("no diff key matches root %q; the plugin is left out of the affected set and keeps its stale config. diff keys: added=%v removed=%v changed=%v",
			configRoot, keySet(diff.Added), keySet(diff.Removed), keySet(diff.Changed))
	}

	// The bytes the affected plugin would actually receive (reload.go:230-242).
	subtree := zeconfig.ExtractConfigSubtree(next, configRoot)
	if subtree == nil {
		t.Fatalf("ExtractConfigSubtree(%q) on the new tree returned nil; the plugin would be handed {} and fall back to defaults", configRoot)
	}
	data, err := json.Marshal(subtree)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg, err := ParseConfig(string(data))
	if err != nil {
		t.Fatalf("ParseConfig(%s): %v", data, err)
	}
	if cfg.ForwardMitigation {
		t.Errorf("the reload section still parses ForwardMitigation=true; delivered bytes: %s", data)
	}
}

func keySet[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}
