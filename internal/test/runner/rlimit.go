// Design: docs/architecture/testing/ci-format.md -- test runner framework
// Related: rlimit_freebsd.go -- the same limits through FreeBSD's signed Rlimit

//go:build !freebsd

package runner

import "syscall"

// rlimitNofile answers the soft and hard RLIMIT_NOFILE limits.
func rlimitNofile() (soft, hard uint64, err error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, 0, err
	}
	return limit.Cur, limit.Max, nil
}

// setRlimitNofile sets the soft and hard RLIMIT_NOFILE limits.
func setRlimitNofile(soft, hard uint64) error {
	limit := syscall.Rlimit{Cur: soft, Max: hard}
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
}
