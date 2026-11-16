// Design: docs/architecture/container/runtime.md -- the system-call filter
// Related: filter.go -- the program this file loads

package seccomp

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/net/bpf"
	"golang.org/x/sys/unix"
)

// loadFilter loads program into the calling process, on every one of its
// threads: a Go program runs on several, and a thread the filter missed
// would make the calls the profile refuses. It MUST be called with
// no-new-privileges set, or with CAP_SYS_ADMIN: the library's init does
// both before it calls the seam. The load is permanent.
func loadFilter(program []bpf.RawInstruction) error {
	if len(program) == 0 {
		return errors.New("container system-call filter: an empty program")
	}
	if len(program) > instructionsMax {
		return fmt.Errorf("container system-call filter: the program holds %d instructions, above the kernel's %d", len(program), instructionsMax)
	}
	filter := make([]unix.SockFilter, len(program))
	for i, instruction := range program {
		filter[i] = unix.SockFilter{Code: instruction.Op, Jt: instruction.Jt, Jf: instruction.Jf, K: instruction.K}
	}
	fprog := unix.SockFprog{
		Len:    uint16(len(filter)), //nolint:gosec // at most instructionsMax, checked above
		Filter: &filter[0],
	}
	thread, _, errno := unix.Syscall(unix.SYS_SECCOMP,
		unix.SECCOMP_SET_MODE_FILTER,
		unix.SECCOMP_FILTER_FLAG_TSYNC,
		uintptr(unsafe.Pointer(&fprog)), //nolint:gosec // the kernel reads the program through this pointer during the call
	)
	runtime.KeepAlive(filter)
	if errno != 0 {
		return fmt.Errorf("container system-call filter: the kernel refused the filter: %w", errno)
	}
	// With the thread-sync flag, a positive answer is the thread that could
	// not take the filter, and no thread took it.
	if thread != 0 {
		return fmt.Errorf("container system-call filter: thread %d could not take the filter, so no thread took it", thread)
	}
	return nil
}
