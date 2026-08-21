package runner

import (
	"fmt"
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
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)) //nolint:noctx // test code
		if err != nil {
			t.Fatalf("allocated port %d is not bindable: %v", port, err)
		}
		_ = ln.Close()
	}
}

// TestReservePortsExcludesHeldRange verifies suite-lifetime reservations.
//
// VALIDATES: Concurrent ze-test processes using ReservePorts do not receive
// the same range while the first reservation is still held.
// PREVENTS: Full-suite flakes where one category probes a free range, releases
// it, and another category chooses the same ports before the first has finished.
func TestReservePortsExcludesHeldRange(t *testing.T) {
	base, err := FindFreePortRange(53000, 4)
	if err != nil {
		t.Fatalf("FindFreePortRange: %v", err)
	}

	first, _, err := ReservePorts(base, 2)
	if err != nil {
		t.Fatalf("first ReservePorts: %v", err)
	}
	defer first.Release()

	second, shifted, err := ReservePorts(first.Start, 2)
	if err != nil {
		t.Fatalf("second ReservePorts: %v", err)
	}
	defer second.Release()

	if !shifted {
		t.Fatal("second reservation did not report shifted=true")
	}
	if second.Start == first.Start {
		t.Fatalf("second reservation reused held start port %d", first.Start)
	}
}

// TestReservePortsLeavesTCPPortsBindable verifies reservations are advisory.
//
// VALIDATES: Holding a reservation does not bind the TCP port, so ze and
// ze-peer child processes can still listen on the assigned port.
// PREVENTS: Replacing the old probe with a listener-held reservation that would
// make the selected port unusable by the test itself.
func TestReservePortsLeavesTCPPortsBindable(t *testing.T) {
	reservation, _, err := ReservePorts(54000, 1)
	if err != nil {
		t.Fatalf("ReservePorts: %v", err)
	}
	defer reservation.Release()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", reservation.Start)) //nolint:noctx // test code
	if err != nil {
		t.Fatalf("reserved port %d is not bindable: %v", reservation.Start, err)
	}
	_ = ln.Close()
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

// TestLeaseTestPortsRefusesToHandTwoTestsOnePort is the discrimination this
// class needs: it fails whenever two callers that PREFER the same port are both
// given it.
//
// VALIDATES: Two tests that prefer the same port (the Nth test of two suites,
// which EncodingTests.parseAndAdd numbers identically) get disjoint pairs while
// the first lease is held.
// PREVENTS: The recorded failure class. reload-remove-bgp is the 30th test of
// test/reload, so it preferred 1790+2*29=1848, and so did the 30th test of every
// other suite; a second ze-test process on the same box handed 1848 out again
// and one of the two died with "bind: address already in use".
func TestLeaseTestPortsRefusesToHandTwoTestsOnePort(t *testing.T) {
	preferred, err := FindFreePortRange(53200, TestPortSpan)
	if err != nil {
		t.Fatalf("FindFreePortRange: %v", err)
	}

	first, err := LeaseTestPorts(preferred)
	if err != nil {
		t.Fatalf("first LeaseTestPorts: %v", err)
	}
	defer first.Release()

	second, err := LeaseTestPorts(preferred)
	if err != nil {
		t.Fatalf("second LeaseTestPorts: %v", err)
	}
	defer second.Release()

	if first.Start != preferred {
		t.Fatalf("first lease did not honor the free preferred port: want %d, got %d", preferred, first.Start)
	}
	if overlaps(first.PortRange, second.PortRange) {
		t.Fatalf("both tests were handed the same ports: first %s, second %s", first.String(), second.String())
	}
	for _, port := range []int{second.Start, second.Start + 1} {
		ln, lnErr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)) //nolint:noctx // test code
		if lnErr != nil {
			t.Fatalf("leased port %d is not bindable: %v", port, lnErr)
		}
		_ = ln.Close()
	}
}

// TestLeaseTestPortsMovesOffAPortSomethingIsListeningOn covers the half no lock
// can see: a process that never took a port lock, such as a daemon leaked by an
// earlier suite in the same run.
//
// VALIDATES: A preferred port with a live listener on it is not leased out.
// PREVENTS: Reporting a port as this test's and then failing at bind, which is
// how the class reads in a suite log ("peer did not start listening within 5s").
func TestLeaseTestPortsMovesOffAPortSomethingIsListeningOn(t *testing.T) {
	preferred, err := FindFreePortRange(53400, TestPortSpan)
	if err != nil {
		t.Fatalf("FindFreePortRange: %v", err)
	}

	squatter, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred+1)) //nolint:noctx // test code
	if err != nil {
		t.Fatalf("bind squatter on %d: %v", preferred+1, err)
	}
	defer func() { _ = squatter.Close() }()

	lease, err := LeaseTestPorts(preferred)
	if err != nil {
		t.Fatalf("LeaseTestPorts: %v", err)
	}
	defer lease.Release()

	if overlaps(lease.PortRange, PortRange{Start: preferred, Count: TestPortSpan}) {
		t.Fatalf("lease %s covers the occupied port %d", lease.String(), preferred+1)
	}
}

func overlaps(a, b PortRange) bool {
	return a.Start < b.End() && b.Start < a.End()
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
