// Design: (none -- predates documentation)
// Overview: store.go -- no-op flock fallback for non-unix platforms
// Related: flock_unix.go -- real flock on unix

//go:build !unix

package zefs

import "os"

func openLockFd(_ string) (*os.File, error) { return nil, nil }
func closeLockFd(_ *os.File) error          { return nil }
func flockExclusive(_ *os.File) error       { return nil }
func flockUnlock(_ *os.File) error          { return nil }
