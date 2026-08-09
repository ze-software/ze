//go:build !linux

// Design: docs/architecture/storage/smart-health.md -- SMART disk health ioctl library
// Related: smart.go — Info type, ParseNVMeBuf, NvmeNamespace

package smart

// Detect is a no-op on non-Linux platforms.
func Detect(_, _ string) *Info { return nil }

// Enable is a no-op on non-Linux platforms.
func Enable(_ string) error { return nil }

// StartSelfTest is a no-op on non-Linux platforms.
func StartSelfTest(_ string, _ SelfTestType) error { return nil }

// IsSelfTestInProgress is a no-op on non-Linux platforms.
func IsSelfTestInProgress(_ string) bool { return false }
