// Design: docs/architecture/testing/ci-format.md -- test runner framework
// Related: rlimit.go -- the same limits through the unsigned Rlimit elsewhere

package runner

import (
	"fmt"
	"math"
	"syscall"
)

// rlimitNofile answers the soft and hard RLIMIT_NOFILE limits. FreeBSD's
// Rlimit fields are int64 (RLIM_INFINITY is math.MaxInt64), so a negative value
// is a kernel answer this code does not understand and is refused.
func rlimitNofile() (soft, hard uint64, err error) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		return 0, 0, err
	}
	if limit.Cur < 0 {
		return 0, 0, fmt.Errorf("getrlimit: negative soft limit %d", limit.Cur)
	}
	if limit.Max < 0 {
		return 0, 0, fmt.Errorf("getrlimit: negative hard limit %d", limit.Max)
	}
	return uint64(limit.Cur), uint64(limit.Max), nil
}

// setRlimitNofile sets the soft and hard RLIMIT_NOFILE limits, refusing a value
// FreeBSD's signed Rlimit cannot hold.
func setRlimitNofile(soft, hard uint64) error {
	if soft > math.MaxInt64 {
		return fmt.Errorf("setrlimit: soft limit %d exceeds the FreeBSD maximum", soft)
	}
	if hard > math.MaxInt64 {
		return fmt.Errorf("setrlimit: hard limit %d exceeds the FreeBSD maximum", hard)
	}
	limit := syscall.Rlimit{Cur: int64(soft), Max: int64(hard)}
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
}
