// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_two_ze_pppoe_link.go -- the declaration this checks.
package topology

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkPPPoELink proves a layer-2 link: the client holds an IPCP address
// from the server's pool on a ppp device, which the kernel assigns only once
// IPCP is Opened; the server's kernel PPPoE table holds the session on the
// link device; and observer, on eth0 alone, received no untagged discovery
// frame. Every assertion reads state.
func checkPPPoELink(ctx context.Context, check *interoplab.CheckContext) error {
	access, err := check.Member("ze-server", "access")
	if err != nil {
		return err
	}
	addresses, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second,
		Description: "ze-client IPCP address from ze-server's pool",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "ze-client", []string{"ip", "-o", "-4", "address", "show"}, nil)
	}, pppAddressFromPool)
	if err != nil {
		return fmt.Errorf("ze-client never held an address from 10.20.0.0/24 on a ppp device (last answer %q): %w", addresses, err)
	}
	sessions, err := check.Lab.Query(ctx, "ze-server", []string{"cat", "/proc/net/pppoe"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-server's PPPoE session table: %w", err)
	}
	if !sessionOnDevice(sessions, access.Device) {
		return fmt.Errorf("ze-server's /proc/net/pppoe holds no session on %s, the access link:\n%s", access.Device, sessions)
	}
	frames, err := check.Lab.Exec(ctx, "observer", []string{"tcpdump", "-n", "-r", observerCapture}, nil)
	if err != nil {
		return fmt.Errorf("read observer's capture: %w", err)
	}
	if frames.ExitCode != 0 {
		return fmt.Errorf("observer's capture is unreadable (exit %d): %s", frames.ExitCode, frames.Stderr)
	}
	if strings.TrimSpace(frames.Stdout) != "" {
		return fmt.Errorf("observer, on eth0 alone, received untagged PPPoE discovery, so the access link leaks:\n%s", frames.Stdout)
	}
	return nil
}

// pppAddressFromPool reports whether `ip -o -4 address show` lists an
// address from the server's pool on a ppp device.
func pppAddressFromPool(answer string) bool {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if !strings.HasPrefix(fields[1], "ppp") {
			continue
		}
		if strings.HasPrefix(fields[3], "10.20.0.") {
			return true
		}
	}
	return false
}

// sessionOnDevice reports whether a /proc/net/pppoe table lists a session on
// device: its rows are "Id Address Device".
func sessionOnDevice(table, device string) bool {
	for _, line := range strings.Split(table, "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[2] == device {
			return true
		}
	}
	return false
}
