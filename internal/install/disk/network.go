// Design: plan/spec-appliance-install-robust.md -- network fallback for initrd installer

package disk

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	carrierWaitMax = 10
	serverProbeMax = 30
	probeTimeout   = 2 * time.Second
)

// ensureNetwork checks for an existing default route. If absent (kernel
// ip=dhcp raced with NIC link-up), brings up all NICs, waits for carrier,
// and runs udhcpc. Then probes the install server until reachable.
func ensureNetwork(server, port string, maxProbe int) error {
	if hasDefaultRoute() {
		slog.Info("network: default route present")
	} else {
		slog.Info("network: no default route, trying userspace DHCP")
		if err := fallbackDHCP(); err != nil {
			return fmt.Errorf("network setup: %w", err)
		}
	}

	var tb textbuf.Buffer
	probeURL := tb.Str("http://").Str(server).Byte(':').Str(port).Byte('/').String()
	if maxProbe <= 0 {
		slog.Info("network: ze.wait=0, skipping server probe")
		return nil
	}
	return waitForServer(probeURL, maxProbe)
}

func hasDefaultRoute() bool {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Iface" {
			continue
		}
		if fields[1] == "00000000" {
			return true
		}
	}
	return false
}

func fallbackDHCP() error {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return fmt.Errorf("read /sys/class/net: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		_ = runCmd("ip", "link", "set", name, "up")
	}

	slog.Info("network: waiting for carrier")
	carrierFound := false
	for range carrierWaitMax {
		for _, entry := range entries {
			name := entry.Name()
			if name == "lo" {
				continue
			}
			var tb textbuf.Buffer
			carrierPath := tb.Str("/sys/class/net/").Str(name).Str("/carrier").String()
			data, readErr := os.ReadFile(carrierPath) //nolint:gosec // sysfs path
			if readErr == nil && strings.TrimSpace(string(data)) == "1" {
				carrierFound = true
				break
			}
		}
		if carrierFound {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !carrierFound {
		return fmt.Errorf("no NIC carrier detected after %ds", carrierWaitMax)
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		if runCmd("udhcpc", "-i", name, "-t", "5", "-n", "-q") == nil {
			slog.Info("network: got DHCP lease", "interface", name)
			return nil
		}
	}

	return fmt.Errorf("no interface got a DHCP lease")
}

func waitForServer(url string, maxAttempts int) error {
	slog.Info("network: probing server", "url", url)
	client := &http.Client{Timeout: probeTimeout}

	for attempt := range maxAttempts {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
		if err != nil {
			continue
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			resp.Body.Close() //nolint:errcheck // probe only
			slog.Info("network: server reachable", "attempt", attempt+1)
			return nil
		}

		if attempt == 0 || attempt%10 == 0 {
			slog.Info("network: waiting for server", "attempt", attempt+1, "max", maxAttempts)
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server %s not reachable after %d attempts", url, maxAttempts)
}

// parseWait parses the ze.wait cmdline value into an int.
func parseWait(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return serverProbeMax
	}
	return n
}
