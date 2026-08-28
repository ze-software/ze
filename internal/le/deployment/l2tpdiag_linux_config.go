//go:build linux && !zetest

// Design: ai/rules/platform-linux.md -- production diagnostics use real Linux operations
// Related: l2tpdiag_linux_record.go -- the zetest-only built-binary seam
package deployment

func l2tpDiagnosticRecordingEnabled() bool { return false }

func configuredL2TPDiagnosticLinuxOps() l2tpDiagnosticLinuxOps {
	return newSystemL2TPDiagnosticOps()
}
