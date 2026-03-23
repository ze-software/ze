// Design: docs/architecture/config/syntax.md — system identity config extraction

package system

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// SystemConfig holds system-wide identity configuration.
// Extracted from the system {} block in config.
type SystemConfig struct {
	Host   string
	Domain string

	// PeeringDB API settings for prefix data lookups.
	PeeringDBURL    string
	PeeringDBMargin uint8
}

// ExpandEnvValue resolves $ENV_VAR references in config values.
// If the value starts with $, the remainder is treated as an OS environment
// variable name. If the env var is empty or unset, the literal string is returned.
// Non-$ values are returned as-is.
func ExpandEnvValue(s string) string {
	if s == "" || s[0] != '$' {
		return s
	}

	envName := s[1:]
	if v := os.Getenv(envName); v != "" {
		return v
	}

	return s
}

// ExtractSystemConfig extracts system identity config from a parsed Tree.
// Reads system.host and system.domain, applying $ENV expansion.
// Returns defaults (host="unknown", domain="") if the system block is absent.
func ExtractSystemConfig(tree *config.Tree) SystemConfig {
	sc := SystemConfig{
		Host:            "unknown",
		PeeringDBURL:    "https://www.peeringdb.com",
		PeeringDBMargin: 10,
	}

	sys := tree.GetContainer("system")
	if sys == nil {
		return sc
	}

	if host, ok := sys.Get("host"); ok {
		sc.Host = ExpandEnvValue(host)
	}

	if domain, ok := sys.Get("domain"); ok {
		sc.Domain = ExpandEnvValue(domain)
	}

	pdb := sys.GetContainer("peeringdb")
	if pdb == nil {
		return sc
	}

	if url, ok := pdb.Get("url"); ok {
		sc.PeeringDBURL = url
	}

	if margin, ok := pdb.Get("margin"); ok {
		var v int
		if _, err := fmt.Sscanf(margin, "%d", &v); err == nil && v >= 0 && v <= 100 {
			sc.PeeringDBMargin = uint8(v) //nolint:gosec // Bounded by range check above
		}
	}

	return sc
}
