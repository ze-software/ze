// Design: docs/architecture/testing/runner-architecture.md -- host load detection for contended-run classification

//go:build !linux && !darwin

package hostload

// readLoadAvg1 has no portable source on platforms other than Linux and macOS.
// Returning 0 makes Contended() report false there (best-effort, consistent with
// the "returns 0 if sampling fails" contract), so the package still builds and
// runs everywhere the dev/verify tooling might.
func readLoadAvg1() float64 {
	return 0
}
