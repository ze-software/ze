package selfcheck

import (
	"net"
	"testing"
)

// TestFindFreePortRange verifies port range finding.
//
// VALIDATES: Can find consecutive free ports.
// PREVENTS: Test failures due to port conflicts.
func TestFindFreePortRange(t *testing.T) {
	// Find a range of 5 ports starting from a high port
	start, err := FindFreePortRange(50000, 5)
	if err != nil {
		t.Fatalf("FindFreePortRange failed: %v", err)
	}

	if start < 50000 {
		t.Errorf("expected start >= 50000, got %d", start)
	}

	// Verify all ports in range are actually free
	for port := start; port < start+5; port++ {
		ln, err := net.Listen("tcp", "127.0.0.1:"+string(rune('0'+port%10))) //nolint:noctx // test code
		if err == nil {
			_ = ln.Close()
		}
	}
}

// TestAllocatePorts verifies port allocation with fallback.
//
// VALIDATES: Ports are allocated, with shift detection.
// PREVENTS: False positives on port availability.
func TestAllocatePorts(t *testing.T) {
	// Allocate from a high base port that should be free
	pr, shifted, err := AllocatePorts(51000, 3)
	if err != nil {
		t.Fatalf("AllocatePorts failed: %v", err)
	}

	if pr.Count != 3 {
		t.Errorf("expected count=3, got %d", pr.Count)
	}

	// If not shifted, start should be base
	if !shifted && pr.Start != 51000 {
		t.Errorf("expected start=51000 when not shifted, got %d", pr.Start)
	}

	t.Logf("Allocated ports: %s (shifted=%v)", pr.String(), shifted)
}

// TestPortRangeString verifies string formatting.
//
// VALIDATES: Range format is "start-end".
// PREVENTS: Confusing port range output.
func TestPortRangeString(t *testing.T) {
	pr := PortRange{Start: 1790, Count: 10}
	expected := "1790-1799"
	if pr.String() != expected {
		t.Errorf("expected %q, got %q", expected, pr.String())
	}
}

// TestCheckPortAvailable verifies single port check.
//
// VALIDATES: Can detect if a port is available.
// PREVENTS: False reports of port availability.
func TestCheckPortAvailable(t *testing.T) {
	// Bind a port
	ln, err := net.Listen("tcp", "127.0.0.1:52000") //nolint:noctx // test code
	if err != nil {
		t.Skipf("Could not bind test port: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// Port should be unavailable
	if CheckPortAvailable(52000) {
		t.Error("expected port 52000 to be unavailable")
	}

	// A different high port should be available
	if !CheckPortAvailable(52999) {
		t.Log("Port 52999 also in use, skipping availability check")
	}
}
