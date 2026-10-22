//go:build docker

package labtest

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// proofImage is the server image; the orphan test uses the same one.
const proofImage = "alpine:3.24.3"

// proofGreeting is what the proof server writes to each connection.
const proofGreeting = "labtest-ready"

// greetingReady answers nil once the server at address writes proofGreeting:
// a protocol-level answer, which Docker's userland proxy cannot give for a
// server that does not listen yet.
func greetingReady(ctx context.Context, address netip.AddrPort) error {
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address.String())
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // a probe connection; the read decides
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != proofGreeting {
		return &net.AddrError{Err: "unexpected greeting " + strconv.Quote(line), Addr: address.String()}
	}
	return nil
}

// VALIDATES: AC-18 against the real daemon: Start runs one labeled container,
// waits until it serves, answers a loopback address that serves, and the
// test's cleanup removes the container, so nothing named for this run
// remains afterwards.
// PREVENTS: a helper that answers before the server serves, or leaves its
// container behind on a passing test.
func TestLabtestStartsServerAndCleansUp(t *testing.T) {
	name := "proof-" + strconv.Itoa(os.Getpid())
	t.Run("start", func(t *testing.T) {
		address := Start(t, Spec{
			Name: name, Image: proofImage, Port: interoplab.TCPPort(8080),
			Command: []string{"sh", "-c", "while true; do echo " + proofGreeting + " | nc -l -p 8080; done"},
			Ready:   greetingReady, ReadyTimeout: 60 * time.Second, LogLines: 50,
		})
		if !address.Addr().IsLoopback() {
			t.Errorf("address %v is not on the loopback", address)
		}
		// The proof server restarts nc after each connection, so one probe can
		// land in that gap; a few attempts prove the address serves.
		var served error
		for range 20 {
			if served = greetingReady(t.Context(), address); served == nil {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		if served != nil {
			t.Fatalf("the server Start answered does not serve: %v", served)
		}
		labels := containersNamed(t, containerPrefix+name+"-", "label=ze.lab.run")
		if len(labels) != 1 {
			t.Fatalf("labeled containers for this run = %v, want exactly one", labels)
		}
	})
	if left := containersNamed(t, containerPrefix+name+"-", ""); len(left) != 0 {
		t.Errorf("containers left after the test's cleanup: %v", left)
	}
}

// containersNamed lists every container, running or stopped, whose name
// starts with prefix and that matches filter when it is not empty.
func containersNamed(t *testing.T, prefix, filter string) []string {
	t.Helper()
	argv := []string{"ps", "-a", "--format", "{{.Names}}", "--filter", "name=" + prefix}
	if filter != "" {
		argv = append(argv, "--filter", filter)
	}
	out, err := exec.CommandContext(context.Background(), "docker", argv...).Output()
	if err != nil {
		t.Fatalf("docker ps: %v", err)
	}
	return strings.Fields(string(out))
}
