// Design: docs/architecture/system-architecture.md — temporary filesystem management

package tmpfs

import (
	"codeberg.org/thomas-mangin/ze/internal/config/env"
)

// Default limits for Tmpfs parsing (overridable via ze.ci.* or ze.ci_* env vars).
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
//   - ze.ci.max_file_size / ze.ci_max_file_size
//   - ze.ci.max_total_size / ze.ci_max_total_size
//   - ze.ci.max_files / ze.ci_max_files
//   - ze.ci.max_path_length / ze.ci_max_path_length
//   - ze.ci.max_path_depth / ze.ci_max_path_depth
func LimitsFromEnv() Limits {
	return Limits{
		MaxFileSize:  env.GetInt64("ci", "max_file_size", DefaultMaxFileSize),
		MaxTotalSize: env.GetInt64("ci", "max_total_size", DefaultMaxTotalSize),
		MaxFiles:     env.GetInt("ci", "max_files", DefaultMaxFiles),
		MaxPathLen:   env.GetInt("ci", "max_path_length", DefaultMaxPathLen),
		MaxPathDepth: env.GetInt("ci", "max_path_depth", DefaultMaxPathDepth),
	}
}
