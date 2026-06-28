// Design: plan/learned/907-appliance-install-robust.md -- network fallback for initrd installer

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

// bringUpAllNICs is the network fallback (a package var so tests can stub it).
// It brings every NIC up and runs DHCP so the install NIC gets an address even
// when the kernel's ip=dhcp configured a different one.
var bringUpAllNICs = fallbackDHCP

// ensureNetwork makes the install server reachable. Reachability -- not the
// mere presence of a default route -- is the signal that matters: on a
// dual-homed target the kernel's ip=dhcp can configure the wrong NIC (e.g. a
// corporate LAN port) and install a default route that does not reach the
// install server. So probe the server first; only if it is unreachable do we
// bring up all NICs and DHCP, then probe again over the install NIC's
// directly-connected route.
func ensureNetwork(server, port string, maxProbe int) error {
	var tb textbuf.Buffer
	probeURL := tb.Str("http://").Str(server).Byte(':').Str(port).Byte('/').String()

	if maxProbe <= 0 {
		slog.Info("network: ze.wait=0, skipping server probe")
		return nil
	}

	if probeServer(probeURL) {
		slog.Info("network: install server reachable")
		return nil
	}

	slog.Info("network: install server unreachable, bringing up all NICs")
	if err := bringUpAllNICs(); err != nil {
		return fmt.Errorf("network setup: %w", err)
	}
	return waitForServer(probeURL, maxProbe)
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

	// DHCP every NIC with carrier, not just the first to answer: on a
	// dual-homed target the install network may be a different NIC than the one
	// the kernel already leased, and we need an address on it to reach the
	// install server over its directly-connected route.
	leased := false
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		if runCmd("udhcpc", "-i", name, "-t", "5", "-n", "-q") == nil {
			slog.Info("network: got DHCP lease", "interface", name)
			leased = true
		}
	}

	if !leased {
		return fmt.Errorf("no interface got a DHCP lease")
	}
	return nil
}

// probeServer reports whether the install server answers one HTTP request. It
// is a package var so tests can drive ensureNetwork without real networking.
var probeServer = func(url string) bool {
	client := &http.Client{Timeout: probeTimeout}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return false
	}
	resp, doErr := client.Do(req)
	if doErr != nil {
		return false
	}
	resp.Body.Close() //nolint:errcheck // probe only
	return true
}

func waitForServer(url string, maxAttempts int) error {
	slog.Info("network: probing server", "url", url)
	for attempt := range maxAttempts {
		if probeServer(url) {
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
