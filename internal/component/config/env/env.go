// Design: docs/architecture/config/environment.md — environment variable handling
//
// Package env provides Ze BGP environment variable handling with dot/underscore support.
// Delegates to internal/core/env for the actual lookup logic.
//
// Environment variables follow the naming pattern:
//   - ze.bgp.section.key (dot notation, higher priority)
//   - ze_bgp_section_key (underscore notation, shell-compatible)
//   - ZE_BGP_SECTION_KEY (uppercase underscore, shell convention)
//
// Example:
//
//	ze.bgp.ci.max_files=100  -> Get("ci", "max_files") returns "100"
//	ze_bgp_ci_max_files=100  -> Get("ci", "max_files") returns "100"
//	ZE_BGP_CI_MAX_FILES=100  -> Get("ci", "max_files") returns "100"
package env

import (
	coreenv "codeberg.org/thomas-mangin/ze/internal/core/env"
)

// Get returns the environment variable value with Ze BGP naming.
// Checks dot notation (ze.bgp.section.key), lowercase underscore (ze_bgp_section_key),
// and uppercase underscore (ZE_BGP_SECTION_KEY). Dot notation takes precedence.
func Get(section, key string) string {
	return coreenv.Get("ze.bgp." + section + "." + key)
}

// GetInt returns int value, or default if not set/invalid.
func GetInt(section, key string, defaultVal int) int {
	return coreenv.GetInt("ze.bgp."+section+"."+key, defaultVal)
}

// GetInt64 returns int64 value, or default if not set/invalid.
func GetInt64(section, key string, defaultVal int64) int64 {
	return coreenv.GetInt64("ze.bgp."+section+"."+key, defaultVal)
}
