// Design: docs/architecture/config/syntax.md — system identity config extraction

package system

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// SystemConfig holds system-wide identity configuration.
// Extracted from the system {} block in config.
type SystemConfig struct {
	Host   string
	Domain string

	// Static DNS name servers (from system { name-server [...] }).
	NameServers []string

	// DNS resolver tuning (from system { dns {} }).
	DNSTimeout     uint16
	DNSCacheSize   uint32
	DNSCacheTTL    uint32
	ResolvConfPath string

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
		DNSTimeout:      5,
		DNSCacheSize:    10000,
		DNSCacheTTL:     86400,
		ResolvConfPath:  "/tmp/resolv.conf",
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

	if servers := sys.GetSlice("name-server"); len(servers) > 0 {
		sc.NameServers = servers
	}

	if dns := sys.GetContainer("dns"); dns != nil {
		if v, ok := dns.Get("resolv-conf-path"); ok {
			sc.ResolvConfPath = sanitizeResolvConfPath(v)
		}
		if v, ok := dns.Get("timeout"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 1 && n <= 60 {
				sc.DNSTimeout = uint16(n) //nolint:gosec // Bounded by range check above
			}
		}
		if v, ok := dns.Get("cache-size"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n <= 1000000 {
				sc.DNSCacheSize = uint32(n) //nolint:gosec // Bounded by range check above
			}
		}
		if v, ok := dns.Get("cache-ttl"); ok {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n >= 0 && n <= 604800 {
				sc.DNSCacheTTL = uint32(n) //nolint:gosec // Bounded by range check above
			}
		}
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

// sanitizeResolvConfPath validates and cleans a resolv-conf-path value.
// Rejects relative paths and path traversal; returns empty string (disabling
// resolv.conf writing) for invalid input.
func sanitizeResolvConfPath(v string) string {
	if v == "" {
		return ""
	}
	if !filepath.IsAbs(v) {
		return ""
	}
	if filepath.Clean(v) != v {
		return ""
	}
	return v
}
