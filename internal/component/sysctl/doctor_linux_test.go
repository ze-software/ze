// Design: docs/features/ai-first.md -- real-kernel sysctl writability probe
// Related: doctor_linux.go -- probeProcSysAccess, under test

//go:build linux

package sysctl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProbeProcSysAccessRealKernel runs probeProcSysAccess against real paths
// on Linux to verify faccessat2(AT_EACCESS) classifies an absent path as
// unknown and /proc/sys/net/ipv4/ip_forward as writable or refused without
// erroring out as unknown.
func TestProbeProcSysAccessRealKernel(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	verdict, err := probeProcSysAccess(missing)
	if verdict != procSysUnknown || err == nil {
		t.Fatalf("missing path: verdict = %s, err = %v; want unknown with error", verdict, err)
	}

	const keyPath = "/proc/sys/net/ipv4/ip_forward"
	if _, statErr := os.Stat(keyPath); statErr != nil {
		t.Skipf("%s not present: %v", keyPath, statErr)
	}
	verdict, err = probeProcSysAccess(keyPath)
	if verdict != procSysWritable && verdict != procSysRefused && verdict != procSysReadOnlyFS {
		t.Fatalf("%s: verdict = %s, err = %v; want writable, refused, or read-only-fs", keyPath, verdict, err)
	}
}
