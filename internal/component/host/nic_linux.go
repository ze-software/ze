// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DetectNICs walks /sys/class/net/ under the Detector's Root and
// returns entries for **physical** network interfaces. Virtual drivers
// (bridge, veth, tun, tap, dummy, bond, vlan, macvlan, ipvlan,
// wireguard, docker0, lo...) are filtered by the absence of a
// /sys/class/net/<n>/device/ subdirectory. This heuristic is uniform
// across present and future virtual drivers because the kernel only
// exposes `device/` when there's a real bus backing the netdev.
//
// Per-interface population:
//   - name, driver, pci-vendor/device, mac, speed, duplex, carrier,
//     rx-queues, tx-queues   — sysfs (fixture-testable)
//   - firmware-version, ring-rx, ring-tx   — ethtool ioctl on AF_INET
//     socket (best-effort; skipped when Root != "" because ioctls
//     target the running kernel's netdev namespace, not a testdata
//     tree)
func (d *Detector) DetectNICs() ([]NICInfo, error) {
	return d.detectNICs(true)
}

func (d *Detector) detectNICs(useEthtool bool) ([]NICInfo, error) {
	dir := d.sysfsPath("class/net")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrUnsupported
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	nics := make([]NICInfo, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		nicDir := filepath.Join(dir, name)
		if !hasPhysicalDeviceDir(nicDir) {
			continue
		}
		nic := d.readNICSysfs(name, nicDir)
		// ethtool ioctls target the running netdev namespace. Only
		// run them when the Detector is reading from the real root.
		if useEthtool && d.Root == "" {
			enrichNICEthtool(&nic)
		}
		nics = append(nics, nic)
	}
	sort.Slice(nics, func(i, j int) bool { return nics[i].Name < nics[j].Name })
	return nics, nil
}

// hasPhysicalDeviceDir returns true iff <nicDir>/device is a directory
// (real or symlink to a real dir). Virtual interfaces lack this.
func hasPhysicalDeviceDir(nicDir string) bool {
	info, err := os.Stat(filepath.Join(nicDir, "device"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// readNICSysfs fills the sysfs-sourced fields for one netdev.
func (d *Detector) readNICSysfs(name, nicDir string) NICInfo {
	nic := NICInfo{
		Name:      name,
		Transport: NICTransportUnknown,
	}
	nic.MAC = readFileString(filepath.Join(nicDir, "address"))
	nic.Carrier = readFileInt(filepath.Join(nicDir, "carrier")) == 1
	if nic.Carrier {
		nic.LinkSpeedMbps = readFileInt(filepath.Join(nicDir, "speed"))
		nic.Duplex = readFileString(filepath.Join(nicDir, "duplex"))
	}
	nic.Driver = parseDriverFromUevent(readFileString(filepath.Join(nicDir, "device/uevent")))
	nic.PCIVendor = strings.TrimPrefix(readFileString(filepath.Join(nicDir, "device/vendor")), "0x")
	nic.PCIDevice = strings.TrimPrefix(readFileString(filepath.Join(nicDir, "device/device")), "0x")
	if nic.PCIVendor != "" {
		nic.Transport = NICTransportPCI
	}
	nic.RxQueues = countQueues(filepath.Join(nicDir, "queues"), "rx-")
	nic.TxQueues = countQueues(filepath.Join(nicDir, "queues"), "tx-")
	return nic
}

// parseDriverFromUevent extracts the DRIVER=<name> line from a netdev's
// `device/uevent` content. Returns "" when the line is missing.
func parseDriverFromUevent(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "DRIVER=") {
			return strings.TrimPrefix(line, "DRIVER=")
		}
	}
	return ""
}

// countQueues counts subdirectories under base whose names start with
// prefix (e.g. "rx-" or "tx-"). Returns 0 when base is unreadable.
func countQueues(base, prefix string) int {
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			n++
		}
	}
	return n
}

// readFileString reads a newline-terminated string file and returns its
// trimmed content; returns "" on any error.
func readFileString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
