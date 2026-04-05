// Design: docs/architecture/config/yang-config-design.md — config path separator

package config

import "strings"

// PathSep is the separator used in YANG/config paths (e.g., "bgp/peer/timer").
// Environment variable paths (ze.bgp.X.Y) use dots as env var convention and
// are unrelated to this separator.
const PathSep = "/"

// JoinPath joins path segments with the config path separator.
func JoinPath(parts ...string) string {
	return strings.Join(parts, PathSep)
}

// AppendPath appends a segment to a path prefix.
// If prefix is empty, returns name alone (no leading separator).
func AppendPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + PathSep + name
}

// SplitPath splits a config path into its segments.
func SplitPath(path string) []string {
	return strings.Split(path, PathSep)
}
