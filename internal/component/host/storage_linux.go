// Design: plan/spec-host-0-inventory.md — hardware inventory detection

//go:build linux

package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const storageTransportNVMe = "nvme"

// DetectStorage walks /sys/class/block/ collecting top-level block
// devices (partitions are skipped by testing for the presence of a
// /partition file).
func (d *Detector) DetectStorage() (*StorageInfo, error) {
	base := d.sysfsPath("class/block")
	entries, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrUnsupported
		}
		return nil, err
	}
	var devs []StorageDevice
	for _, e := range entries {
		name := e.Name()
		dev := filepath.Join(base, name)
		if isPartition(dev) {
			continue
		}
		devs = append(devs, d.readBlockDevice(name, dev))
	}
	sort.Slice(devs, func(i, j int) bool { return devs[i].Name < devs[j].Name })
	return &StorageInfo{Devices: devs}, nil
}

// isPartition returns true when <dev>/partition exists — the kernel
// marker for a partition entry (e.g. nvme0n1p2).
func isPartition(dev string) bool {
	_, err := os.Stat(filepath.Join(dev, "partition"))
	return err == nil
}

// readBlockDevice fills one StorageDevice entry from sysfs. Size is
// reported in 512-byte sectors by the kernel and converted to bytes.
func (d *Detector) readBlockDevice(name, dev string) StorageDevice {
	dev512 := readFileInt(filepath.Join(dev, "size")) // sectors
	s := StorageDevice{
		Name:       name,
		SizeBytes:  uint64(dev512) * 512, //nolint:gosec // sector count is non-negative
		Model:      readFileString(filepath.Join(dev, "device", "model")),
		Serial:     readFileString(filepath.Join(dev, "device", "serial")),
		Rotational: readFileInt(filepath.Join(dev, "queue", "rotational")) == 1,
		Transport:  classifyTransport(name),
	}
	if strings.HasPrefix(name, storageTransportNVMe) {
		s.NVMeFirmware = readFileString(filepath.Join(dev, "device", "firmware_rev"))
	}
	return s
}

// classifyTransport picks a transport label from the device name. The
// kernel exposes the real transport in `/sys/class/block/<n>/device/`
// attributes but they vary by bus — name-based classification is
// accurate and portable enough for inventory purposes.
func classifyTransport(name string) string {
	if name == "" {
		return "unknown"
	}
	if strings.HasPrefix(name, storageTransportNVMe) {
		return storageTransportNVMe
	}
	if strings.HasPrefix(name, "sd") {
		return "sata"
	}
	if strings.HasPrefix(name, "mmcblk") {
		return "mmc"
	}
	if strings.HasPrefix(name, "vd") {
		return "virtio"
	}
	return "unknown"
}
