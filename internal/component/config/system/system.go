// Design: docs/architecture/config/syntax.md — system identity config extraction

package system

import (
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// SystemConfig holds system-wide identity configuration.
// Extracted from the system {} block in config.
type SystemConfig struct {
	Host   string
	Domain string
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
		Host: "unknown",
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

	return sc
}
