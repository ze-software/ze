// Package env provides ZeBGP environment variable handling with dot/underscore support.
//
// Environment variables follow the naming pattern:
//   - zebgp.section.key (dot notation, higher priority)
//   - zebgp_section_key (underscore notation, shell-compatible)
//
// Example:
//
//	zebgp.ci.max_files=100  → Get("ci", "max_files") returns "100"
//	zebgp_ci_max_files=100  → Get("ci", "max_files") returns "100"
package env

import (
	"os"
	"strconv"
	"strings"
)

// Get returns the environment variable value with ZeBGP naming.
// Checks both dot notation (zebgp.section.key) and underscore (zebgp_section_key).
// Dot notation takes precedence.
func Get(section, key string) string {
	// Dot notation first (higher priority)
	dotKey := "zebgp." + section + "." + key
	if v := os.Getenv(dotKey); v != "" {
		return v
	}

	// Underscore notation (shell-compatible)
	underKey := strings.ReplaceAll(dotKey, ".", "_")
	return os.Getenv(underKey)
}

// GetInt returns int value, or default if not set/invalid.
func GetInt(section, key string, defaultVal int) int {
	s := Get(section, key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// GetInt64 returns int64 value, or default if not set/invalid.
func GetInt64(section, key string, defaultVal int64) int64 {
	s := Get(section, key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultVal
	}
	return v
}
