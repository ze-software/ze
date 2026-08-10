// Design: docs/architecture/system-architecture.md — temporary filesystem management

package tmpfs

// Default limits for Tmpfs parsing (overridable via ze.ci.* or ze.ci_* env vars).
const (
	DefaultMaxFileSize  int64 = 1 << 20 // 1 MB
	DefaultMaxTotalSize int64 = 1 << 20 // 1 MB
	DefaultMaxFiles           = 100
	DefaultMaxPathLen         = 256
	DefaultMaxPathDepth       = 10
)
