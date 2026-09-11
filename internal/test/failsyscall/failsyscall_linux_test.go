// VALIDATES: arming installs a filter that fails the selected read length and
// leaves every other read alone, and that a filter which cannot select what the
// caller named is refused rather than installed.
//
// PREVENTS: the two silent failures this launcher exists to rule out. A filter
// that installs and matches nothing leaves the daemon healthy while the test
// reports an armed window; a filter whose jump offsets are wrong fails EVERY
// read, so the daemon under measurement has no working socket at all. One wrong
// jf byte produced the second of those during the mechanism's first build.

//go:build linux

package failsyscall

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/crashlog"
)

// armChildEnv marks the child process that is allowed to install a filter. A
// seccomp filter cannot be removed once it is installed, so arming inside the
// test process would change every test that ran after it in the same binary.
const armChildEnv = "ZE_FAILSYSCALL_ARM_CHILD"

// execChildEnv marks the child that runs the whole launcher, execve included.
const execChildEnv = "ZE_FAILSYSCALL_EXEC_CHILD"

func TestArmFailsOnlyTheSelectedReadLength(t *testing.T) {
	if os.Getenv(armChildEnv) != "" {
		armAndProveInChild(t)
		return
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestArmFailsOnlyTheSelectedReadLength", "-test.v")
	child.Env = append(os.Environ(), armChildEnv+"=1")
	output, err := child.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", output)
}

// armAndProveInChild runs in the child. It arms the filter, then asks a socket
// the launcher's own probe never touched, so the assertion does not rest on
// the probe that arm already made.
func armAndProveInChild(t *testing.T) {
	require.NoError(t, arm(options{
		SyscallName: "recvfrom",
		ErrnoName:   "ENETDOWN",
		ReadLength:  1500,
		Command:     []string{"true"},
	}))

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	defer unix.Close(fd) //nolint:errcheck // the probe socket carries no data

	_, _, err = unix.Recvfrom(fd, make([]byte, 1500), unix.MSG_DONTWAIT)
	require.ErrorIs(t, err, unix.ENETDOWN, "the selected read length must fail")

	_, _, err = unix.Recvfrom(fd, make([]byte, 256), unix.MSG_DONTWAIT)
	require.ErrorIs(t, err, unix.EAGAIN, "an unselected read length must reach the kernel unchanged")
}

func TestArmRefusesWhatItCannotSelect(t *testing.T) {
	cases := []struct {
		name    string
		request options
		want    string
	}{
		{
			name:    "unknown syscall",
			request: options{SyscallName: "readv", ErrnoName: "ENETDOWN", ReadLength: lengthUnset, Command: []string{"true"}},
			want:    "syscall readv is not one this can fail",
		},
		{
			name:    "length on a syscall whose third argument is not a length",
			request: options{SyscallName: "recvmsg", ErrnoName: "ENETDOWN", ReadLength: 1500, Command: []string{"true"}},
			want:    "carries its flags in the third argument",
		},
		{
			name:    "unknown errno",
			request: options{SyscallName: "recvfrom", ErrnoName: "ENOTANERRNO", ReadLength: lengthUnset, Command: []string{"true"}},
			want:    "is not an errno name",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			// Each request is refused before anything is installed, so these
			// run safely in the test process itself.
			err := arm(one.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), one.want)
		})
	}
}

func TestBuildFilterJumpsLandOnTheAllow(t *testing.T) {
	cases := []struct {
		name       string
		readLength int
		want       int
	}{
		{name: "every read", readLength: lengthUnset, want: 6},
		{name: "one read length", readLength: 1500, want: 8},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			filter := buildFilter(unix.AUDIT_ARCH_AARCH64, unix.SYS_RECVFROM, uint32(unix.ENETDOWN), one.readLength)
			require.Len(t, filter, one.want)
			allow := len(filter) - 1
			assert.Equal(t, uint32(unix.SECCOMP_RET_ALLOW), filter[allow].K)
			assert.Equal(t, uint32(unix.SECCOMP_RET_ERRNO)|uint32(unix.ENETDOWN), filter[allow-1].K)
			for index, instruction := range filter {
				if instruction.Code != bpfJumpIfEqual {
					continue
				}
				assert.Zero(t, instruction.Jt, "a match at %d must fall through", index)
				assert.Equal(t, allow, index+1+int(instruction.Jf),
					"a mismatch at %d must jump to the allow", index)
			}
		})
	}
}

// TestRunRestoresStderrBeforeExec drives the whole launcher, crashlog and
// execve included, because that pair is where the descriptor is lost. The
// child calls crashlog.Init first, exactly as every cmd/ze binary does before
// a root handler runs, so a launcher that forgot to restore fd 2 would hand
// the launched command a pipe nobody reads and this test would see nothing.
func TestRunRestoresStderrBeforeExec(t *testing.T) {
	if os.Getenv(execChildEnv) != "" {
		crashlog.Init()
		Run([]string{
			"syscall", "recvfrom", "errno", "ENETDOWN", "length", "1500",
			"--", "sh", "-c", "echo LAUNCHED-ON-STDERR 1>&2",
		})
		return
	}
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestRunRestoresStderrBeforeExec")
	// The crash directory is named so the child's crashlog.Init cannot pick a
	// shared one and leave a directory behind in the developer's config dir.
	child.Env = append(os.Environ(), execChildEnv+"=1", "ze.crash.dir="+t.TempDir())
	var stderr strings.Builder
	child.Stderr = &stderr
	require.NoError(t, child.Run(), "stderr:\n%s", stderr.String())
	assert.Contains(t, stderr.String(), "LAUNCHED-ON-STDERR",
		"the launched command's stderr must reach the caller, not the crashlog pipe")
}
