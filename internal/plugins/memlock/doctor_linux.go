// Design: docs/architecture/doctor-and-health-checks.md -- memlock pre-flight check
// Overview: memlock.go -- why the executable is locked
// Related: memlock_linux.go -- the init() that takes the lock and records the outcome

//go:build linux

package memlock

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctor_linux.go answers a question the setup record cannot: can THIS HOST
// lock the ze executable at all?
//
// The two are one topic in two tiers, and neither derives from the other.
// `show plugins` replays what a ze process's own init() ACHIEVED, so it needs a
// host where ze already started and it answers for that one run. This check
// reads the ENVIRONMENT, before ze runs and with no lock taken, so an operator
// preparing a host learns the limit is too small without booting the daemon
// and reading a soft failure back.

// codeMemlockRlimitLow is raised when RLIMIT_MEMLOCK is smaller than the ze
// executable and no capability lifts the limit.
const codeMemlockRlimitLow = "doctor-memlock-rlimit-low"

// codeMemlockRlimitUnknown is raised when the host could not be read, so the
// check has no verdict. It is a separate code because the operator acts
// differently: one says raise the limit, the other says the probe failed.
const codeMemlockRlimitUnknown = "doctor-memlock-rlimit-unknown"

// errNoCapEff says /proc/self/status carried no CapEff line, so the capability
// set could not be read. The check reports that rather than assuming the
// capability is absent, which would warn on a host that locks memory fine.
var errNoCapEff = errors.New("no CapEff line in /proc/self/status")

// errNegativeExecutableSize says the executable reported a size below zero,
// which no file has. It exists so the octet count is never converted from a
// value the comparison would read as an enormous limit.
var errNegativeExecutableSize = errors.New("/proc/self/exe reports a negative size")

// capIPCLock is the Linux capability bit for CAP_IPC_LOCK.
//
// mlock(2): "a privileged process (CAP_IPC_LOCK) can lock as much memory as it
// likes". RLIMIT_MEMLOCK does not apply to such a process, so a warning about
// the limit would be false on every appliance, where ze runs as root, and a
// false warning on every appliance is worse than no check.
const capIPCLock = 14

// memlockEnvironment is what the check reads about the host. It is a value so
// a test can supply one, rather than changing the rlimit of the process that
// runs the test.
type memlockEnvironment struct {
	// LimitOctets is the soft RLIMIT_MEMLOCK. RLIM_INFINITY arrives as the
	// largest uint64, which no executable size reaches, so an unlimited host
	// needs no case of its own.
	LimitOctets uint64
	// ExecutableOctets is the size of /proc/self/exe on disk. It is a FLOOR
	// for the mapped size rather than the mapped size: the loader maps at
	// least the whole file, and it also maps bss and page padding the file
	// does not carry. So a limit below this number cannot possibly hold the
	// executable, a limit above it still might not, and this check claims only
	// the first.
	ExecutableOctets uint64
	// PrivilegedLock reports CAP_IPC_LOCK in this process's EFFECTIVE set,
	// which is what the kernel tests. Reading the bit rather than the user id
	// gets the container right: root under Docker without --cap-add IPC_LOCK
	// does not hold it, and the limit applies to it.
	PrivilegedLock bool
}

// checkMemlockLimit is the registered doctor check. It reads the real host.
func checkMemlockLimit(_ registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	return memlockLimitDiagnostics(readMemlockEnvironment)
}

// memlockLimitDiagnostics is the check's whole verdict, over an environment a
// caller supplies. It stays silent when the limit does not decide the outcome:
// a process holding CAP_IPC_LOCK locks what it likes, and a limit at or above
// the executable's size leaves nothing this check can prove.
func memlockLimitDiagnostics(read func() (memlockEnvironment, error)) []rpc.DoctorCheckDiagnostic {
	host, err := read()
	if err != nil {
		var message textbuf.Buffer
		message.Str("the RLIMIT_MEMLOCK pre-flight check could not read this host, so whether ze can lock its executable is unknown: ").Err(err)
		return []rpc.DoctorCheckDiagnostic{{
			Code:     codeMemlockRlimitUnknown,
			Severity: "warning",
			Message:  message.String(),
		}}
	}

	if host.PrivilegedLock {
		return nil
	}
	if host.LimitOctets >= host.ExecutableOctets {
		return nil
	}

	var message textbuf.Buffer
	message.Str("RLIMIT_MEMLOCK is ").Uint(host.LimitOctets).
		Str(" octets and the ze executable is at least ").Uint(host.ExecutableOctets).
		Str(" octets, so the kernel refuses to lock it and its pages can be evicted under memory pressure; raise the limit, which the generated ze.service unit does with LimitMEMLOCK=infinity, or under Docker use --ulimit memlock=-1 or --cap-add IPC_LOCK")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     codeMemlockRlimitLow,
		Severity: "warning",
		Message:  message.String(),
	}}
}

// readMemlockEnvironment reads the three facts the verdict rests on. It
// returns an error rather than a zero environment when it cannot read one: a
// zero limit beside a zero executable size compares equal and would read as a
// host that passes.
func readMemlockEnvironment() (memlockEnvironment, error) {
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &limit); err != nil {
		return memlockEnvironment{}, err
	}

	// The size of the file behind /proc/self/exe, which is the running ze
	// binary. Stat follows the link, so this is the executable rather than the
	// link itself.
	info, err := os.Stat("/proc/self/exe")
	if err != nil {
		return memlockEnvironment{}, err
	}

	privileged, err := holdsIPCLock()
	if err != nil {
		return memlockEnvironment{}, err
	}

	if info.Size() < 0 {
		return memlockEnvironment{}, errNegativeExecutableSize
	}

	return memlockEnvironment{
		LimitOctets:      limit.Cur,
		ExecutableOctets: uint64(info.Size()),
		PrivilegedLock:   privileged,
	}, nil
}

// holdsIPCLock reports whether CAP_IPC_LOCK is in this process's effective
// capability set, read from /proc/self/status the way
// internal/core/privilege/check_linux.go reads it.
func holdsIPCLock() (bool, error) {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(string(status), "\n") {
		hex, found := strings.CutPrefix(line, "CapEff:\t")
		if !found {
			continue
		}
		effective, err := strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
		if err != nil {
			return false, err
		}
		return effective&(1<<capIPCLock) != 0, nil
	}
	return false, errNoCapEff
}
