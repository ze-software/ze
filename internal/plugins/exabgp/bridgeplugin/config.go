// Design: docs/architecture/exabgp-bridge.md -- internal exabgp bridge config
//
// Parses the `exabgp { bridge { ... } }` config root delivered to the internal
// plugin via the SDK OnConfigure (Stage 2) callback. The script command reaches
// the in-process runner here rather than through the process-manager `run` line
// (which startInternal consumes only for runner-name resolution). User
// directive 2026-07-09: nest under a top-level `exabgp` root so the root stays
// available for future exabgp-related config; the registry plugin name stays
// `exabgp-bridge`.

package bridgeplugin

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/exabgp/bridge"
)

// configRoot is the top-level YANG container the plugin subscribes to.
// bridgeContainer is the child container holding the bridge settings.
const (
	configRoot      = "exabgp"
	bridgeContainer = "bridge"
)

const (
	addPathNone    = "none"
	addPathReceive = "receive"
	addPathSend    = "send"
	addPathBoth    = "both"
)

var validAddPath = map[string]bool{
	addPathNone:    true,
	addPathReceive: true,
	addPathSend:    true,
	addPathBoth:    true,
}

// defaultFamily is the address family the bridge negotiates when the operator
// configures none. Mirrors the `ze exabgp plugin` CLI default (main.go's
// effectiveFamilies fallback).
const defaultFamily = "ipv4/unicast"

// bridgeConfig is the parsed, validated `exabgp.bridge` configuration.
type bridgeConfig struct {
	// Present reports whether the exabgp.bridge container appeared in the
	// committed config at all (vs an empty/other-root section).
	Present      bool
	Run          string
	Families     []string
	RouteRefresh bool
	AddPath      string
}

// parseConfig extracts and validates the bridge configuration from a single
// config section's JSON data (shape: {"exabgp": {"bridge": {...}}}). An absent
// container yields Present=false with no error (nothing to run yet).
func parseConfig(data string) (bridgeConfig, error) {
	cfg := bridgeConfig{AddPath: addPathNone}

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("exabgp bridge config: unmarshal: %w", err)
	}
	exa, ok := asMap(root, configRoot)
	if !ok {
		return cfg, nil
	}
	blk, ok := asMap(exa, bridgeContainer)
	if !ok {
		return cfg, nil
	}
	cfg.Present = true

	if v, ok := asString(blk, "run"); ok {
		cfg.Run = v
	}

	for _, fam := range asStringList(blk, "family") {
		if err := bridge.ValidateFamily(fam); err != nil {
			return cfg, fmt.Errorf("exabgp bridge: %w", err)
		}
		cfg.Families = append(cfg.Families, fam)
	}

	if v, ok := asString(blk, "route-refresh"); ok {
		cfg.RouteRefresh = v == "true"
	}

	if v, ok := asString(blk, "add-path"); ok {
		if !validAddPath[v] {
			return cfg, fmt.Errorf("exabgp bridge: add-path %q invalid (none|receive|send|both)", v)
		}
		cfg.AddPath = v
	}

	if len(cfg.Families) == 0 {
		cfg.Families = []string{defaultFamily}
	}
	return cfg, nil
}

func asMap(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key].(map[string]any)
	return v, ok
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key].(string)
	return v, ok
}

func asStringList(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}
