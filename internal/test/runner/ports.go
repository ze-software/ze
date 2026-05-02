// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// PortRange holds a range of ports for testing.
type PortRange struct {
	Start int
	Count int
}

// PortReservation holds advisory locks for a port range assigned to a
// running test suite. The locks coordinate concurrent ze-test processes;
// they do not bind the TCP ports, so child ze/ze-peer processes can use them.
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
	return fmt.Sprintf("%d-%d", p.Start, p.End()-1)
}

// Release drops the advisory port locks. It is safe to call more than once.
func (p *PortReservation) Release() {
	for _, f := range p.files {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	p.files = nil
}

// FindFreePortRange finds N consecutive free ports starting from base.
// Returns the starting port of the free range.
func FindFreePortRange(base, count int) (int, error) {
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

// isPortRangeFree checks if ports [start, start+count) are all available.
func isPortRangeFree(start, count int) bool {
	listeners := make([]net.Listener, 0, count)
	defer func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}()

	for port := start; port < start+count; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)) //nolint:noctx // port probing, no context needed
		if err != nil {
			return false // Port in use
		}
		listeners = append(listeners, ln)
	}

	return true
}

// AllocatePorts tries to allocate a port range, falling back if base is occupied.
// Returns the actual range and whether it was shifted from base.
func AllocatePorts(base, count int) (PortRange, bool, error) {
	reservation, shifted, err := ReservePorts(base, count)
	if err != nil {
		return PortRange{}, false, err
	}
	defer reservation.Release()
	return reservation.PortRange, shifted, nil
}

// ReservePorts allocates a free port range and keeps an advisory
// reservation until Release is called. This prevents concurrent ze-test
// processes from probing the same free range and racing each other at bind time.
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

// CheckPortAvailable checks if a single port is available.
func CheckPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)) //nolint:noctx // port probing, no context needed
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
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
	lockDir := filepath.Join(os.TempDir(), "ze-test-port-locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, false, fmt.Errorf("create port lock directory: %w", err)
	}

	reservation := &PortReservation{
		PortRange: PortRange{Start: start, Count: count},
		files:     make([]*os.File, 0, count),
	}
	for port := start; port < start+count; port++ {
		path := filepath.Join(lockDir, fmt.Sprintf("%d.lock", port))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // path is lockDir (TempDir constant) + integer port number
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

func isWouldBlock(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && (errno == syscall.EWOULDBLOCK || errno == syscall.EAGAIN)
}
