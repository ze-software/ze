// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PortRange holds a range of ports for testing.
type PortRange struct {
	Start int
	Count int
}

// PortReservation holds advisory locks for a port range assigned to one running
// test (LeaseTestPorts) or to one running web case (ReservePorts). The locks
// coordinate same-user le test processes sharing /tmp, independently of TMPDIR,
// checkout, or session. They do not bind TCP ports, so child ze/ze-peer processes
// can use them; nonparticipating binders are not excluded.
//
// flock is per open file description, so a second reservation over a port this
// process already holds is refused exactly as another process's would be. A
// suite MUST NOT hold a range that its own tests then lease out of.
type PortReservation struct {
	PortRange
	files []*os.File
}

// End returns the last port in the range (exclusive).
func (p PortRange) End() int {
	return p.Start + p.Count
}

// String returns a human-readable representation.
func (p PortRange) String() string {
	var b textbuf.Buffer
	return b.Reset().Int(int64(p.Start)).Byte('-').Int(int64(p.End() - 1)).String()
}

// Release drops the advisory port locks. It is safe to call more than once.
// The owner MUST call Release after its use of the leased ports ends.
// Lock files MUST NOT be unlinked: replacing an inode splits lock ownership.
func (p *PortReservation) Release() {
	for _, f := range p.files {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	p.files = nil
}

// findFreePortRange finds N consecutive free ports starting from base.
// Returns the starting port of the free range.
func findFreePortRange(base, count int) (int, error) {
	const maxPort = 65000
	const step = 100 // Jump in larger steps for efficiency

	for startPort := base; startPort < maxPort; startPort += step {
		if isPortRangeFree(startPort, count) {
			return startPort, nil
		}
	}

	// If stepping didn't work, try smaller increments
	for startPort := base; startPort < maxPort; startPort++ {
		if isPortRangeFree(startPort, count) {
			return startPort, nil
		}
	}

	return 0, fmt.Errorf("no free port range of %d ports found starting from %d", count, base)
}

// isPortRangeFree probes both loopback families for ports [start, start+count).
// All successful probes MUST remain open until the complete range is checked,
// then close before returning so the lease owner's children can bind.
func isPortRangeFree(start, count int) bool {
	listeners := make([]net.Listener, 0, 2*count)
	defer func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	for port := start; port < start+count; port++ {
		ln, err := net.Listen("tcp4", textbuf.StrInt("127.0.0.1:", int64(port))) //nolint:noctx // Port probing, no context needed.
		if err != nil {
			return false
		}
		listeners = append(listeners, ln)

		ln, err = net.Listen("tcp6", textbuf.StrInt("[::1]:", int64(port))) //nolint:noctx // Port probing, no context needed.
		if err != nil {
			if isIPv6LoopbackUnavailable(err) {
				continue
			}
			return false
		}
		listeners = append(listeners, ln)
	}

	return true
}

// isIPv6LoopbackUnavailable distinguishes an unsupported loopback family from
// an occupied port or an arbitrary probe failure, both of which reject a range.
func isIPv6LoopbackUnavailable(err error) bool {
	return errors.Is(err, syscall.EAFNOSUPPORT) ||
		errors.Is(err, syscall.EPROTONOSUPPORT) ||
		errors.Is(err, syscall.EADDRNOTAVAIL)
}

// allocatePorts tries to allocate a port range, falling back if base is occupied.
// Returns the actual range and whether it was shifted from base.
func allocatePorts(base, count int) (PortRange, bool, error) {
	reservation, shifted, err := ReservePorts(base, count)
	if err != nil {
		return PortRange{}, false, err
	}
	defer reservation.Release()
	return reservation.PortRange, shifted, nil
}

// ReservePorts allocates a free port range and keeps an advisory reservation.
// The owner MUST call Release after its use of the leased ports ends.
// This prevents participating same-user le test processes sharing /tmp from
// probing the same free range and racing each other at bind time.
func ReservePorts(base, count int) (*PortReservation, bool, error) {
	if reservation, ok, err := tryReservePortRange(base, count); err != nil {
		return nil, false, err
	} else if ok {
		return reservation, false, nil
	}

	start, reservation, err := findReservedFreePortRange(base+count, count)
	if err != nil {
		return nil, false, err
	}
	reservation.Start = start
	return reservation, true, nil
}

// TestPortSpan is the number of consecutive ports one .ci test owns: Record.Port
// is $PORT and Record.Port+1 is $PORT2 (runner_exec.go substitutes both).
const TestPortSpan = 2

// The band LeaseTestPorts falls back to when a test's preferred pair is taken.
// It sits above every base a suite prefers (1790 for the .ci runners, 10200 for
// web, 21790 for vpp) and below the lowest ephemeral range of a supported host
// (32768 on Linux, 49152 on Darwin), so a lease is never handed a port the
// kernel can also give to an outbound connection.
const (
	leasePortBase  = 25000
	leasePortLimit = 32760
)

var errNoLeasablePortPair = errors.New("no free test port pair in the lease band")

// LeaseTestPorts leases the port span one .ci test owns and holds advisory locks.
// The owner MUST call Release after its use of the leased ports ends, so the
// lease covers exactly the test that binds the ports.
//
// preferred is the port the suite would like this test to have. It is honored
// when the pair is both unlocked and bindable AT LEASE TIME; otherwise the test
// gets a pair from the lease band instead. This is the whole mechanism against
// parallel copies colliding on a deterministic port: every .ci suite numbers its
// tests from the same base (EncodingTests.parseAndAdd), so the Nth test of every
// suite prefers the same port, and two le test processes running different
// suites at once both reach it. The lock decides who keeps it, and the loser
// moves rather than failing at bind.
func LeaseTestPorts(preferred int) (*PortReservation, error) {
	if reservation, ok, err := tryReservePortRange(preferred, TestPortSpan); err != nil {
		return nil, err
	} else if ok {
		return reservation, nil
	}

	for start := leasePortBase; start+TestPortSpan <= leasePortLimit; start += TestPortSpan {
		reservation, ok, err := tryReservePortRange(start, TestPortSpan)
		if err != nil {
			return nil, err
		}
		if ok {
			return reservation, nil
		}
	}

	return nil, fmt.Errorf("%w: %d-%d", errNoLeasablePortPair, leasePortBase, leasePortLimit-1)
}

// checkPortAvailable checks if a single port is available.
func checkPortAvailable(port int) bool {
	return isPortRangeFree(port, 1)
}

func findReservedFreePortRange(base, count int) (int, *PortReservation, error) {
	const maxPort = 65000
	const step = 100

	for startPort := base; startPort < maxPort; startPort += step {
		reservation, ok, err := tryReservePortRange(startPort, count)
		if err != nil {
			return 0, nil, err
		}
		if ok {
			return startPort, reservation, nil
		}
	}

	for startPort := base; startPort < maxPort; startPort++ {
		reservation, ok, err := tryReservePortRange(startPort, count)
		if err != nil {
			return 0, nil, err
		}
		if ok {
			return startPort, reservation, nil
		}
	}

	return 0, nil, fmt.Errorf("no free port range of %d ports found starting from %d", count, base)
}

func tryReservePortRange(start, count int) (*PortReservation, bool, error) {
	reservation, ok, err := reservePortLocks(start, count)
	if err != nil || !ok {
		return nil, ok, err
	}
	if !isPortRangeFree(start, count) {
		reservation.Release()
		return nil, false, nil
	}
	return reservation, true, nil
}

func reservePortLocks(start, count int) (*PortReservation, bool, error) {
	// A flat per-user directory cannot be blocked by another user's private
	// legacy directory, and stays shared across TMPDIR overrides.
	return reservePortLocksIn(textbuf.StrInt("/tmp/le-port-locks-", int64(os.Geteuid())), start, count)
}

// reservePortLocksIn takes the per-port locks for [start, start+count) in
// lockDir, creating the directory when it is absent.
func reservePortLocksIn(lockDir string, start, count int) (*PortReservation, bool, error) {
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, false, fmt.Errorf("create port lock directory: %w", err)
	}
	if err := checkPortLockDir(lockDir); err != nil {
		return nil, false, err
	}

	reservation := &PortReservation{
		Start: start,
		Count: count,
		files: make([]*os.File, 0, count),
	}
	for port := start; port < start+count; port++ {
		path := filepath.Join(lockDir, textbuf.IntStr(int64(port), ".lock"))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // Path is a private effective-UID lock directory plus an integer port.
		if err != nil {
			reservation.Release()
			return nil, false, fmt.Errorf("open port lock %d: %w", port, err)
		}
		reservation.files = append(reservation.files, f)
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			reservation.Release()
			if isWouldBlock(err) {
				return nil, false, nil
			}
			return nil, false, fmt.Errorf("flock port %d: %w", port, err)
		}
	}
	return reservation, true, nil
}

// checkPortLockDir refuses a lock directory this runner cannot trust, naming
// its owner and mode so the operator can see what to remove.
//
// MkdirAll accepts whatever already sits at the path, so the directory is
// judged as itself (Lstat, never through a link) and must be a directory the
// effective user owns, can use, and nobody else can write. A directory others
// can write lets them unlink a held lock file, and a replacement inode then
// admits a second holder.
func checkPortLockDir(lockDir string) error {
	info, err := os.Lstat(lockDir)
	if err != nil {
		return fmt.Errorf("port lock directory %s: %w", lockDir, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("port lock directory %s: no owner information on this platform", lockDir)
	}
	euid := os.Geteuid()
	usable := info.IsDir() && int(stat.Uid) == euid &&
		info.Mode().Perm()&0o700 == 0o700 && info.Mode().Perm()&0o022 == 0
	if usable {
		return nil
	}
	return fmt.Errorf("port lock directory %s is owned by uid %d with mode %s; "+
		"it must be a directory owned by uid %d with mode drwx------ (0700): remove it and rerun",
		lockDir, stat.Uid, info.Mode(), euid)
}

func isWouldBlock(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == syscall.EWOULDBLOCK || errno == syscall.EAGAIN)
}
