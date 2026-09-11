// Design: docs/architecture/testing/qemu-integration.md -- failing one socket in the guest
// Related: failsyscall_linux.go -- the seccomp filter these options describe
// Related: test/l2tp/subscriber-reader-failing-socket.ci -- the first caller

// Package failsyscall fails one named syscall for one launched process, through
// a seccomp filter rather than a tracer.
package failsyscall

import (
	"errors"
	"fmt"
	"strconv"
)

// lengthUnset is the ReadLength of a run that names no length, so the filter
// fails the syscall on every descriptor the command holds. It is negative
// rather than zero because a read of zero bytes is a legitimate selector.
const lengthUnset = -1

// readLengthMax bounds the `length` selector. Two things need the bound. The
// filter compares a 32-bit word, so a larger value would be truncated and arm a
// filter for a length the caller never named; and proveArmed allocates a buffer
// of exactly this many bytes to make its probe, so an unbounded keyword is an
// unbounded allocation driven by one .ci line. One mebibyte is far above every
// receive buffer a ze loop names: the l2tp listener's rxBufLen
// (internal/component/l2tp/listener.go) is 1500.
const readLengthMax = 1 << 20

// commandSeparator ends the options and begins the command to launch. The
// command carries its own flags and keywords, so it needs a marker the parser
// cannot confuse with one of its own keywords.
const commandSeparator = "--"

// options is one run of the launcher: which syscall fails, what it answers,
// which reads it selects, and what to launch under it.
type options struct {
	// SyscallName is the syscall to fail, by name. Its number differs between
	// architectures, so the name is what a .ci file can carry.
	SyscallName string
	// ErrnoName is the errno the syscall answers, by name.
	ErrnoName string
	// ReadLength selects ONE socket by the byte count its reader asks for,
	// which is a per-loop constant in a daemon. lengthUnset selects every read.
	ReadLength int
	// Command is the argv to launch once the filter is installed.
	Command []string
}

var (
	errNoSyscall = errors.New("fail-syscall: `syscall <name>` is required")
	errNoErrno   = errors.New("fail-syscall: `errno <NAME>` is required")
	errNoCommand = errors.New("fail-syscall: `-- <command> [args...]` is required")
)

// parseOptions reads the keyword-value options that precede `--`.
func parseOptions(args []string) (options, error) {
	separator := -1
	for index, arg := range args {
		if arg == commandSeparator {
			separator = index
			break
		}
	}
	if separator < 0 {
		return options{}, errNoCommand
	}
	parsed := options{ReadLength: lengthUnset, Command: args[separator+1:]}
	if len(parsed.Command) == 0 {
		return options{}, errNoCommand
	}

	keywords := args[:separator]
	for index := 0; index < len(keywords); index += 2 {
		if index+1 >= len(keywords) {
			return options{}, fmt.Errorf("fail-syscall: keyword %s has no value", keywords[index])
		}
		if err := parsed.set(keywords[index], keywords[index+1]); err != nil {
			return options{}, err
		}
	}

	if parsed.SyscallName == "" {
		return options{}, errNoSyscall
	}
	if parsed.ErrnoName == "" {
		return options{}, errNoErrno
	}
	return parsed, nil
}

// set applies one keyword-value pair. An unknown keyword is an error rather
// than a skipped word: a .ci file that misspells `length` would otherwise arm a
// filter selecting every socket and still report that it armed.
func (o *options) set(keyword, value string) error {
	switch keyword {
	case "syscall":
		o.SyscallName = value
		return nil
	case "errno":
		o.ErrnoName = value
		return nil
	case "length":
		length, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("fail-syscall: length %s is not a number", value)
		}
		if length < 0 {
			return fmt.Errorf("fail-syscall: length %s is negative; omit the keyword to select every read", value)
		}
		if length > readLengthMax {
			return fmt.Errorf("fail-syscall: length %s is above %d, which no receive loop asks for and the filter's 32-bit comparison cannot carry", value, readLengthMax)
		}
		o.ReadLength = length
		return nil
	default:
		return fmt.Errorf("fail-syscall: unknown keyword %s; it takes syscall, errno and length", keyword)
	}
}
