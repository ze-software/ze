package vfs

import (
	"codeberg.org/thomas-mangin/zebgp/internal/env"
)

// Default limits for VFS parsing (overridable via zebgp.ci.* or zebgp_ci_* env vars).
const (
	DefaultMaxFileSize  int64 = 1 << 20 // 1 MB
	DefaultMaxTotalSize int64 = 1 << 20 // 1 MB
	DefaultMaxFiles           = 100
	DefaultMaxPathLen         = 256
	DefaultMaxPathDepth       = 10
)

// LimitsFromEnv reads limits from environment, falling back to defaults.
//
// Environment variables:
//   - zebgp.ci.max_file_size / zebgp_ci_max_file_size
//   - zebgp.ci.max_total_size / zebgp_ci_max_total_size
//   - zebgp.ci.max_files / zebgp_ci_max_files
//   - zebgp.ci.max_path_length / zebgp_ci_max_path_length
//   - zebgp.ci.max_path_depth / zebgp_ci_max_path_depth
func LimitsFromEnv() Limits {
	return Limits{
		MaxFileSize:  env.GetInt64("ci", "max_file_size", DefaultMaxFileSize),
		MaxTotalSize: env.GetInt64("ci", "max_total_size", DefaultMaxTotalSize),
		MaxFiles:     env.GetInt("ci", "max_files", DefaultMaxFiles),
		MaxPathLen:   env.GetInt("ci", "max_path_length", DefaultMaxPathLen),
		MaxPathDepth: env.GetInt("ci", "max_path_depth", DefaultMaxPathDepth),
	}
}
