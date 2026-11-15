// VALIDATES: seccomp selects exact write lengths across descriptors and exec,
// including the high word of the syscall's byte count.
// PREVENTS: an injection proof on a different kernel from the daemon under test.
// These controls run in QEMU against Ze's runtime kernel.
// Design: docs/architecture/testing/qemu-integration.md -- syscall failure injection in the guest

//go:build integration && linux

package failsyscall

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// writeExecChildEnv distinguishes the launcher and probe images of the same
// child. The second image proves that execve preserves the installed filter.
const writeExecChildEnv = "ZE_FAILSYSCALL_WRITE_EXEC_CHILD"

// TestArmFailsOnlyTheSelectedWriteLength checks the kernel result on fresh
// descriptors: selected writes fail, and a different length delivers its bytes.
// A second socket proves that the selector is process-wide, not socket-specific.
func TestArmFailsOnlyTheSelectedWriteLength(t *testing.T) {
	if os.Getenv(armChildEnv) != "" {
		require.NoError(t, arm(options{
			SyscallName: "write",
			ErrnoName:   "EIO",
			ByteLength:  21,
			Command:     []string{"true"},
		}))
		proveWriteSelectionInChild(t)
		return
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestArmFailsOnlyTheSelectedWriteLength$")
	child.Env = append(os.Environ(), armChildEnv+"=1")
	output, err := child.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", output)
}

// TestArmDoesNotAliasHighWriteCount distinguishes the full 64-bit syscall count
// from its low word. /dev/null accepts the unselected count without reading the
// nil buffer, so a matching low word must not inject EIO into that call.
func TestArmDoesNotAliasHighWriteCount(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("the high-word selector test needs a 64-bit Linux syscall ABI")
	}
	if os.Getenv(armChildEnv) != "" {
		require.NoError(t, arm(options{
			SyscallName: "write",
			ErrnoName:   "EIO",
			ByteLength:  21,
			Command:     []string{"true"},
		}))
		// This child MUST close its test-owned sink on every return.
		fd, err := unix.Open("/dev/null", unix.O_WRONLY|unix.O_CLOEXEC, 0)
		require.NoError(t, err)
		// The deferred close MUST also run after a failed assertion.
		defer unix.Close(fd) //nolint:errcheck // the test owns this write sink

		count := uint64(1)<<32 + 21
		written, _, errno := unix.Syscall(unix.SYS_WRITE, uintptr(fd), 0, uintptr(count))
		require.Zero(t, errno, "a count with a nonzero high word is not length 21")
		require.Positive(t, written, "the unselected write must reach /dev/null")
		return
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestArmDoesNotAliasHighWriteCount$")
	child.Env = append(os.Environ(), armChildEnv+"=1")
	output, err := child.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", output)
}

// proveWriteSelectionInChild checks both sides of the length boundary on two
// connected sockets, so an unselected write must deliver data rather than fail
// for an unrelated reason such as an unconnected datagram socket.
func proveWriteSelectionInChild(t *testing.T) {
	for range 2 {
		// The test MUST retain both pairs until its assertions finish.
		pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
		require.NoError(t, err)
		// Keep both pairs open through every control so the second pair cannot
		// reuse the first pair's descriptors. Cleanup also runs after Fatal.
		// It MUST close both ends even when an assertion aborts the test.
		t.Cleanup(func() {
			unix.Close(pair[0]) //nolint:errcheck // test-owned probe cleanup cannot change the assertion result
			unix.Close(pair[1]) //nolint:errcheck // test-owned probe cleanup cannot change the assertion result
		})

		_, err = unix.Write(pair[0], make([]byte, 21))
		require.ErrorIs(t, err, unix.EIO, "every descriptor with the selected byte count must fail")

		payload := bytes.Repeat([]byte{'x'}, 22)
		written, err := unix.Write(pair[0], payload)
		require.NoError(t, err, "an adjacent write length must reach the kernel unchanged")
		require.Equal(t, len(payload), written)
		received := make([]byte, len(payload))
		read, err := unix.Read(pair[1], received)
		require.NoError(t, err)
		require.Equal(t, payload, received[:read], "an unselected write must deliver its bytes")
	}
}

// TestRunWriteFilterSurvivesExec drives Run through its command boundary and
// checks real socket writes in the replacement image, not only before execve.
func TestRunWriteFilterSurvivesExec(t *testing.T) {
	switch os.Getenv(writeExecChildEnv) {
	case "launch":
		require.NoError(t, os.Setenv(writeExecChildEnv, "probe"))
		status := Run([]string{
			"syscall", "write", "errno", "EIO", "length", "21", "--",
			os.Args[0], "-test.run=^TestRunWriteFilterSurvivesExec$",
		})
		t.Fatalf("Run returned %d instead of replacing the launcher", status)
	case "probe":
		proveWriteSelectionInChild(t)
	default:
		child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRunWriteFilterSurvivesExec$")
		child.Env = append(os.Environ(), writeExecChildEnv+"=launch")
		output, err := child.CombinedOutput()
		require.NoError(t, err, "child output:\n%s", output)
	}
}
