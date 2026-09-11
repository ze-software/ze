// Design: docs/architecture/testing/qemu-integration.md -- failing one socket in the guest
// Related: options.go -- the keyword grammar this reads
// Related: internal/test/cli/register.go -- registers the `fail-syscall` verb
// Related: test/l2tp/subscriber-reader-failing-socket.ci -- the first caller
//
// A daemon that meets a socket it cannot read is meant to pace its retries. To
// test that, a test has to hold one socket in a persistently failing state from
// OUTSIDE the daemon, and no ordinary network condition does: an interface that
// goes down delivers silence, and a device-bound AF_PACKET socket is told
// ENETDOWN once before it reverts to blocking.
//
// ptrace-based injection (`strace -e inject=`) does sustain the error and
// destroys the measurement with it. Go's runtime emits a continuous SIGURG
// stream for goroutine preemption, ptrace traps every delivery, and time spent
// ptrace-stopped counts as wall clock rather than as the tracee's CPU. The paced
// and unpaced builds then read the same.
//
// A classic seccomp filter answering SECCOMP_RET_ERRNO has neither problem. The
// kernel refuses the call at its entry and returns immediately, with no tracer
// in the path, so the failing call is charged to the daemon's own CPU time,
// which is the signal a process-level assertion reads.

//go:build linux

package failsyscall

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// The byte offsets of struct seccomp_data, fixed by the kernel ABI
// (<linux/seccomp.h>): the syscall number, the architecture, then the six
// arguments as 64-bit words from offset 16.
const (
	seccompOffsetNr   = 0
	seccompOffsetArch = 4
	seccompOffsetArg2 = 32
)

// The three classic BPF opcodes the filter uses: load a 32-bit word at a fixed
// offset, compare the accumulator with a constant, and return a constant.
const (
	bpfLoadWord    = unix.BPF_LD | unix.BPF_W | unix.BPF_ABS
	bpfJumpIfEqual = unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K
	bpfReturnConst = unix.BPF_RET | unix.BPF_K
)

// errnoMax is the largest value the kernel accepts as an injected errno
// (MAX_ERRNO in <linux/err.h>), and so bounds the walk that resolves a name.
const errnoMax = 4095

// syscallSelector is one syscall this can fail on the architecture the binary
// was compiled for.
type syscallSelector struct {
	// number is the syscall number. It differs between architectures, so it is
	// taken from the compile-time constant rather than written down.
	number uint32
	// arg2IsReadLength says whether the third argument of the call is the read
	// length. recvfrom(fd, buf, len, flags, ...) carries the length there, so a
	// length selects one socket. recvmsg(fd, msghdr, flags) carries the FLAGS
	// there, where a length selector would compare against a flag word.
	arg2IsReadLength bool
	// probe makes the call once, so arming can be proven before the exec.
	probe func(fd int, buffer []byte) error
}

// syscallsByName is what the launcher knows how to fail. It is short on
// purpose: each entry states what its third argument means, and that fact has
// to be checked against the kernel ABI before a name is added.
var syscallsByName = map[string]syscallSelector{
	"recvfrom": {
		number:           unix.SYS_RECVFROM,
		arg2IsReadLength: true,
		probe: func(fd int, buffer []byte) error {
			_, _, err := unix.Recvfrom(fd, buffer, unix.MSG_DONTWAIT)
			return err
		},
	},
	"recvmsg": {
		number:           unix.SYS_RECVMSG,
		arg2IsReadLength: false,
		probe: func(fd int, buffer []byte) error {
			_, _, _, _, err := unix.Recvmsg(fd, buffer, nil, unix.MSG_DONTWAIT)
			return err
		},
	},
}

// usageLine is printed when the options do not parse. It lives beside the only
// verb that prints it rather than beside the grammar it describes (options.go),
// because options.go carries no build tag and the launcher is Linux-only.
const usageLine = "usage: ze-test fail-syscall syscall <name> errno <NAME> [length <bytes>] -- <command> [args...]"

// Run installs the filter and replaces this process with the command after
// `--`. It returns only when something refused, because a successful execve
// does not return.
func Run(args []string) int {
	parsed, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck // diagnostic on the way out
		fmt.Fprintln(os.Stderr, usageLine)
		return 2
	}
	if err := arm(parsed); err != nil {
		fmt.Fprintln(os.Stderr, err) //nolint:errcheck // diagnostic on the way out
		return 1
	}
	path, err := exec.LookPath(parsed.Command[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "fail-syscall: %v\n", err) //nolint:errcheck // diagnostic on the way out
		return 1
	}
	// Put the real stderr back on descriptor 2 before the image is replaced.
	// Every cmd/ze binary calls crashlog.Init at startup, which dup2s a PIPE
	// onto fd 2 and reads it from a goroutine. The execve below destroys that
	// goroutine and leaves the launched daemon writing into a pipe nobody
	// drains: its log vanishes, and once 64 KiB have accumulated every further
	// write BLOCKS FOREVER. Flush restores the saved descriptor and closes the
	// pipe. Measured in the QEMU guest on 2026-09-11: without this call the
	// daemon served metrics and counted read errors while emitting not one log
	// line, which is what `expect=stderr:contains=l2tp: listener read error`
	// in test/l2tp/subscriber-reader-failing-socket.ci caught.
	crashlog.Flush()
	err = unix.Exec(path, parsed.Command, os.Environ())
	fmt.Fprintf(os.Stderr, "fail-syscall: exec %s: %v\n", path, err) //nolint:errcheck // diagnostic on the way out
	return 1
}

// arm installs the filter on every thread of this process and then proves that
// it selects what it was asked to select.
func arm(parsed options) error {
	selector, known := syscallsByName[parsed.SyscallName]
	if !known {
		return fmt.Errorf("fail-syscall: syscall %s is not one this can fail; it knows %s",
			parsed.SyscallName, knownSyscallNames())
	}
	if parsed.ReadLength != lengthUnset && !selector.arg2IsReadLength {
		return fmt.Errorf("fail-syscall: %s carries its flags in the third argument rather than a read length, so `length` cannot select one socket on it",
			parsed.SyscallName)
	}
	errno, err := errnoByName(parsed.ErrnoName)
	if err != nil {
		return err
	}
	arch, err := auditArch()
	if err != nil {
		return err
	}

	filter := buildFilter(arch, selector.number, uint32(errno), parsed.ReadLength)
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("fail-syscall: prctl(PR_SET_NO_NEW_PRIVS): %w", err)
	}
	// TSYNC puts the filter on every thread this process already has. Without
	// it the filter would sit on whichever thread the Go runtime was running
	// this goroutine on, and a goroutine that migrated before the execve below
	// would launch the command unfiltered.
	thread, _, installErrno := unix.RawSyscall(unix.SYS_SECCOMP,
		uintptr(unix.SECCOMP_SET_MODE_FILTER),
		uintptr(unix.SECCOMP_FILTER_FLAG_TSYNC),
		uintptr(unsafe.Pointer(&program))) //nolint:gosec // G103: the seccomp ABI takes sock_fprog by address, and x/sys publishes no typed wrapper for SECCOMP_SET_MODE_FILTER
	runtime.KeepAlive(filter)
	if installErrno != 0 {
		return fmt.Errorf("fail-syscall: seccomp(SECCOMP_SET_MODE_FILTER): %w", installErrno)
	}
	if thread != 0 {
		return fmt.Errorf("fail-syscall: seccomp could not put the filter on thread %d", thread)
	}
	return proveArmed(selector, errno, parsed.ReadLength)
}

// buildFilter answers the classic BPF program seccomp runs at each syscall
// entry. The shape is fixed: compare the architecture, compare the syscall
// number, compare the read length when one selects the socket, then answer the
// errno. Every comparison falls THROUGH on a match and jumps to the final allow
// on a mismatch, so each jf is the distance from the instruction after the
// comparison to that allow.
//
// The architecture is compared first because a syscall number means nothing
// without it: the same number names a different call under another ABI, so a
// filter that skipped the check would fail an unrelated syscall on a process
// running in compatibility mode.
func buildFilter(auditArchValue, syscallNumber, errno uint32, readLength int) []unix.SockFilter {
	selectsLength := readLength != lengthUnset
	// Two instructions for each comparison, plus the two returns.
	count := 6
	if selectsLength {
		count = 8
	}
	allow := count - 1
	filter := make([]unix.SockFilter, 0, count)
	appendComparison := func(offset, want uint32) {
		// len(filter) is the index the load will take, so the comparison after
		// it sits one further on, and the jump counts from the one after that.
		jumpToAllow := uint8(allow - len(filter) - 2)
		filter = append(filter,
			unix.SockFilter{Code: bpfLoadWord, K: offset},
			unix.SockFilter{Code: bpfJumpIfEqual, K: want, Jf: jumpToAllow},
		)
	}

	appendComparison(seccompOffsetArch, auditArchValue)
	appendComparison(seccompOffsetNr, syscallNumber)
	if selectsLength {
		appendComparison(seccompOffsetArg2, uint32(readLength))
	}
	return append(filter,
		unix.SockFilter{Code: bpfReturnConst, K: unix.SECCOMP_RET_ERRNO | errno},
		unix.SockFilter{Code: bpfReturnConst, K: unix.SECCOMP_RET_ALLOW},
	)
}

// proveArmed makes the selected call once, here, before the exec. A filter that
// installed and selects nothing is otherwise indistinguishable from a daemon
// that never reached the call, and the test would pass while nothing was ever
// injected (`ai/rules/principles.md`).
func proveArmed(selector syscallSelector, errno syscall.Errno, readLength int) error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("fail-syscall: open the probe socket: %w", err)
	}
	defer unix.Close(fd) //nolint:errcheck // the probe socket carries no data

	probeLength := readLength
	if probeLength == lengthUnset {
		probeLength = 1
	}
	if err := selector.probe(fd, make([]byte, probeLength)); !errors.Is(err, errno) {
		//nolint:errorlint // %w cannot render the case that matters most: a probe the filter never
		// touched answers a nil error, and this line is a diagnostic rather than a chain to unwrap.
		return fmt.Errorf("fail-syscall: the filter did not arm: a read of %d bytes answered %v, want %v",
			probeLength, err, errno)
	}
	if readLength == lengthUnset {
		return nil
	}
	// The negative control. A filter whose jump offsets are wrong fails every
	// read rather than the selected one, and a daemon with no working socket at
	// all is not what the caller asked to measure.
	if err := selector.probe(fd, make([]byte, readLength+1)); errors.Is(err, errno) {
		return fmt.Errorf("fail-syscall: the filter fires on a read of %d bytes, which `length %d` does not select",
			readLength+1, readLength)
	}
	return nil
}

// errnoByName answers the errno a name stands for, by walking the table
// golang.org/x/sys/unix already publishes rather than copying it. A second
// hand-written list would be a second declaration of the same fact.
func errnoByName(name string) (syscall.Errno, error) {
	for value := 1; value <= errnoMax; value++ {
		if unix.ErrnoName(syscall.Errno(value)) == name {
			return syscall.Errno(value), nil
		}
	}
	return 0, fmt.Errorf("fail-syscall: %s is not an errno name on this kernel's ABI", name)
}

// auditArch answers the AUDIT_ARCH value the filter compares against. Only the
// architectures ze cross-compiles for are listed: an unlisted one is an error
// rather than a guess, because the wrong value makes the filter match nothing
// and the run would report an arming failure with no reason on it.
func auditArch() (uint32, error) {
	switch runtime.GOARCH {
	case "arm64":
		return unix.AUDIT_ARCH_AARCH64, nil
	case "amd64":
		return unix.AUDIT_ARCH_X86_64, nil
	default:
		return 0, fmt.Errorf("fail-syscall: no AUDIT_ARCH value is recorded for GOARCH %s", runtime.GOARCH)
	}
}

// knownSyscallNames lists what the launcher can fail, for the refusal message.
func knownSyscallNames() string {
	names := make([]string, 0, len(syscallsByName))
	for name := range syscallsByName {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}
