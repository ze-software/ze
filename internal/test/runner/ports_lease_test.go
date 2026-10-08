// Design: docs/architecture/testing/ci-format.md — advisory port lease scope.

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestPortLeaseAcrossTempDirs proves cross-process exclusion without TCP listeners.
// Two pipe-synchronized subprocesses request the same pair with different TMPDIRs;
// both keep their advisory leases until the parent releases their stdin barriers.
func TestPortLeaseAcrossTempDirs(t *testing.T) {
	const marker = "port-lease-subprocess"
	if len(os.Args) >= 3 && os.Args[len(os.Args)-2] == marker {
		preferred, err := strconv.Atoi(os.Args[len(os.Args)-1])
		if err != nil {
			t.Fatal(err)
		}
		lease, err := LeaseTestPorts(preferred)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Release()
		if err := json.NewEncoder(os.Stdout).Encode(lease.PortRange); err != nil {
			t.Fatal(err)
		}
		var release [1]byte
		if _, err := io.ReadFull(os.Stdin, release[:]); err != nil {
			t.Fatalf("lease release barrier: %v", err)
		}
		return
	}

	first := startPortLeaseSubprocess(t, marker, 53600, t.TempDir())
	// A bind while the child holds its lease proves exclusion cannot be supplied
	// accidentally by a socket the child kept open after probing.
	assertPortLeaseBindable(t, first)
	second := startPortLeaseSubprocess(t, marker, first.Start, t.TempDir())
	if overlaps(first, second) {
		t.Fatalf("different TMPDIRs reused a held advisory lease: first %s, second %s", first, second)
	}
	if first.Count != TestPortSpan {
		t.Fatalf("first lease count = %d, want %d", first.Count, TestPortSpan)
	}
	if second.Count != TestPortSpan {
		t.Fatalf("second lease count = %d, want %d", second.Count, TestPortSpan)
	}
	assertPortLeaseBindable(t, second)
}

// startPortLeaseSubprocess MUST register cleanup that releases and reaps the child.
// The returned range is published only after the child owns the lease.
func startPortLeaseSubprocess(t *testing.T, marker string, preferred int, tempDir string) PortRange {
	t.Helper()
	// t.Context is canceled before cleanup, but cleanup MUST release and reap
	// the children normally before canceling their deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPortLeaseAcrossTempDirs$", "--", marker, strconv.Itoa(preferred))
	cmd.Env = append(os.Environ(), "TMPDIR="+tempDir)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Cleanup MUST release the stdin barrier before waiting for child exit.
	t.Cleanup(func() {
		if _, err := stdin.Write([]byte{'\n'}); err != nil {
			t.Errorf("release child lease: %v", err)
		}
		if err := stdin.Close(); err != nil {
			t.Errorf("close child stdin: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Errorf("port lease subprocess: %v", err)
		}
	})
	var ports PortRange
	if err := json.NewDecoder(stdout).Decode(&ports); err != nil {
		t.Fatalf("read child lease: %v", err)
	}
	return ports
}

// TestPortLeaseLoopbackOccupants rejects real IPv4 and IPv6 occupants on either
// port through both reservation APIs and both availability predicates.
func TestPortLeaseLoopbackOccupants(t *testing.T) {
	for _, network := range []string{"tcp4", "tcp6"} {
		for _, offset := range []int{0, 1} {
			for _, api := range []string{"LeaseTestPorts", "ReservePorts"} {
				t.Run(fmt.Sprintf("%s/port%d/%s", network, offset+1, api), func(t *testing.T) {
					control, _, err := ReservePorts(54000, TestPortSpan)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(control.Release)
					preferred := control.PortRange
					address := "127.0.0.1"
					if network == "tcp6" {
						address = "::1"
					}
					occupant, err := net.Listen(network, net.JoinHostPort(address, strconv.Itoa(preferred.Start+offset))) //nolint:noctx // Local test listener has explicit cleanup.
					if err != nil {
						if network == "tcp6" && portLeaseTestIPv6Unavailable(err) {
							t.Skipf("IPv6 loopback unavailable: %v", err)
						}
						t.Fatalf("bind occupant: %v", err)
					}
					t.Cleanup(func() {
						if err := occupant.Close(); err != nil {
							t.Error(err)
						}
					})
					// Hold the advisory lease through binding, then release it so
					// only the live socket can force the allocation off this pair.
					control.Release()
					if isPortRangeFree(preferred.Start, preferred.Count) {
						t.Error("range probe accepted an occupied loopback port")
					}
					if checkPortAvailable(preferred.Start + offset) {
						t.Error("single-port probe accepted an occupied loopback port")
					}
					var lease *PortReservation
					if api == "LeaseTestPorts" {
						lease, err = LeaseTestPorts(preferred.Start)
					} else {
						var shifted bool
						lease, shifted, err = ReservePorts(preferred.Start, preferred.Count)
						if !shifted {
							t.Error("occupied preferred range did not report shifted=true")
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					defer lease.Release()
					if overlaps(preferred, lease.PortRange) {
						t.Errorf("lease %s overlaps occupied preferred pair %s", lease.PortRange, preferred)
					} else {
						assertPortLeaseBindable(t, lease.PortRange)
					}
				})
			}
		}
	}
}

// assertPortLeaseBindable holds both families on the entire pair simultaneously,
// proving every successful probe listener was closed before returning the lease.
func assertPortLeaseBindable(t *testing.T, ports PortRange) {
	t.Helper()
	for port := ports.Start; port < ports.End(); port++ {
		for _, network := range []string{"tcp4", "tcp6"} {
			address := "127.0.0.1"
			if network == "tcp6" {
				address = "::1"
			}
			listener, err := net.Listen(network, net.JoinHostPort(address, strconv.Itoa(port))) //nolint:noctx // Local test listener is closed before the helper returns.
			if err != nil {
				if network == "tcp6" && portLeaseTestIPv6Unavailable(err) {
					t.Logf("IPv6 bindability prerequisite unavailable: %v", err)
					continue
				}
				t.Fatalf("leased %s port %d is not bindable: %v", network, port, err)
			}
			defer func() {
				if err := listener.Close(); err != nil {
					t.Error(err)
				}
			}()
		}
	}
}

func portLeaseTestIPv6Unavailable(err error) bool {
	return errors.Is(err, syscall.EAFNOSUPPORT) ||
		errors.Is(err, syscall.EPROTONOSUPPORT) ||
		errors.Is(err, syscall.EADDRNOTAVAIL)
}

// TestPortLeaseIPv6UnavailableErrors pins the narrow unsupported-family policy;
// an occupied port, permissions failure, or unrelated error MUST reject a range.
func TestPortLeaseIPv6UnavailableErrors(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		unavailable bool
	}{
		{name: "family", err: syscall.EAFNOSUPPORT, unavailable: true},
		{name: "protocol", err: syscall.EPROTONOSUPPORT, unavailable: true},
		{name: "loopback", err: syscall.EADDRNOTAVAIL, unavailable: true},
		{name: "occupied", err: syscall.EADDRINUSE},
		{name: "permission", err: syscall.EACCES},
		{name: "invalid", err: syscall.EINVAL},
		{name: "other", err: errors.New("unrelated probe failure")},
		{name: "success"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isIPv6LoopbackUnavailable(tc.err); got != tc.unavailable {
				t.Errorf("unavailable(%v) = %v, want %v", tc.err, got, tc.unavailable)
			}
			if tc.err != nil {
				wrapped := &net.OpError{Op: "listen", Net: "tcp6", Err: os.NewSyscallError("bind", tc.err)}
				if got := isIPv6LoopbackUnavailable(wrapped); got != tc.unavailable {
					t.Errorf("unavailable(wrapped %v) = %v, want %v", tc.err, got, tc.unavailable)
				}
			}
		})
	}
}
