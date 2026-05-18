// Design: plan/spec-diag-core.md -- non-Linux stub for /proc reading
// Overview: reader.go -- types and parsing helpers
//
//go:build !linux

package procfs

func ReadFileLines(_ string) ([]string, error) {
	return nil, ErrUnsupported
}
