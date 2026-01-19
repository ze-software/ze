package runner

import (
	"fmt"
	"net"
)

// PortRange holds a range of ports for testing.
type PortRange struct {
	Start int
	Count int
}

// End returns the last port in the range (exclusive).
func (p PortRange) End() int {
	return p.Start + p.Count
}

// String returns a human-readable representation.
func (p PortRange) String() string {
	return fmt.Sprintf("%d-%d", p.Start, p.End()-1)
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
	// First, check if the base range is available
	if isPortRangeFree(base, count) {
		return PortRange{Start: base, Count: count}, false, nil
	}

	// Base is occupied, find alternative
	start, err := FindFreePortRange(base+count, count)
	if err != nil {
		return PortRange{}, false, err
	}

	return PortRange{Start: start, Count: count}, true, nil
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
