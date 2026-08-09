// Design: docs/architecture/appliance/on-device-installer.md -- network fallback for initrd installer

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

	"github.com/ze-software/ze/internal/core/textbuf"
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
func ensureNetwork(server, port, mac string, maxProbe int) error {
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

	if mac != "" {
		pinIF, err := ifaceForMAC(mac, "")
		if err != nil {
			slog.Warn("network: ze.mac matches no interface, scanning all", "mac", mac)
		} else {
			slog.Info("network: pinning to boot NIC", "iface", pinIF, "mac", mac)
			if upErr := linkUp(pinIF); upErr != nil {
				slog.Warn("network: linkUp failed on pinned NIC", "iface", pinIF, "error", upErr)
			}
			slog.Info("network: waiting for carrier on pinned NIC", "iface", pinIF)
			waitForCarrier(pinIF)
			slog.Info("network: running DHCP on pinned NIC", "iface", pinIF)
			dhcpErr := dhcpAcquireApply(pinIF)
			if dhcpErr != nil {
				slog.Warn("network: DHCP failed on pinned NIC", "iface", pinIF, "error", dhcpErr)
			}
			if dhcpErr == nil && probeServer(probeURL) {
				slog.Info("network: server reachable on pinned NIC", "iface", pinIF)
				return nil
			}
			slog.Info("network: pinned NIC cannot reach server, flushing", "iface", pinIF)
			_ = flushIface(pinIF)
		}
	}

	slog.Info("network: bringing up all NICs")
	if err := bringUpAllNICs(); err != nil {
		return fmt.Errorf("network setup: %w", err)
	}
	return waitForServer(probeURL, maxProbe)
}

func waitForCarrier(ifName string) {
	var tb textbuf.Buffer
	carrierPath := tb.Str("/sys/class/net/").Str(ifName).Str("/carrier").String()
	for range carrierWaitMax {
		data, err := os.ReadFile(carrierPath) //nolint:gosec // sysfs path
		if err == nil && strings.TrimSpace(string(data)) == "1" {
			return
		}
		time.Sleep(1 * time.Second)
	}
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
		if upErr := linkUp(name); upErr != nil {
			slog.Debug("network: linkUp failed", "iface", name, "error", upErr)
		}
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

	leased := false
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		if dhcpAcquireApply(name) == nil {
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

// ifaceForMAC returns the kernel interface name whose MAC matches mac.
// lo is skipped. The match is case-insensitive. sysnetDir overrides
// /sys/class/net for testing.
func ifaceForMAC(mac, sysnetDir string) (string, error) {
	if mac == "" {
		return "", fmt.Errorf("empty MAC")
	}
	want := strings.ToLower(mac)
	if sysnetDir == "" {
		sysnetDir = "/sys/class/net"
	}

	entries, err := os.ReadDir(sysnetDir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", sysnetDir, err)
	}

	var tb textbuf.Buffer
	for _, entry := range entries {
		name := entry.Name()
		if name == "lo" {
			continue
		}
		addrPath := tb.Reset().Str(sysnetDir).Byte('/').Str(name).Str("/address").String()
		data, readErr := os.ReadFile(addrPath) //nolint:gosec // sysfs path
		if readErr != nil {
			continue
		}
		addr := strings.ToLower(strings.TrimSpace(string(data)))
		if addr == want {
			return name, nil
		}
	}
	return "", fmt.Errorf("no interface with MAC %s", mac)
}

// parseWait parses the ze.wait cmdline value into an int.
func parseWait(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return serverProbeMax
	}
	return n
}
