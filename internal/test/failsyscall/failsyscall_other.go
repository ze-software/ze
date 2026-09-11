// Design: docs/architecture/testing/qemu-integration.md -- failing one socket in the guest
// Related: failsyscall_linux.go -- the implementation

//go:build !linux

package failsyscall

import (
	"fmt"
	"os"
)

// Run refuses on any platform without seccomp. The verb stays registered
// everywhere so `ze-test` has one command list on every host, and a caller that
// reaches it off Linux is told why rather than finding no such command.
func Run(_ []string) int {
	fmt.Fprintln(os.Stderr, "fail-syscall: seccomp filters are Linux-only; run this inside the QEMU guest") //nolint:errcheck // diagnostic on the way out
	return 2
}
