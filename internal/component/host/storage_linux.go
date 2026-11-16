// Design: docs/architecture/host/inventory.md -- hardware inventory detection
// Related: smart_linux.go — SMART health detection per device

//go:build linux

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const storageTransportNVMe = "nvme"

// persistentMountPoint is where an appliance mounts its persistent partition.
const persistentMountPoint = "/perm"

// The fields of one line of /proc/self/mountinfo that name a mount, as
// proc(5) lays them out: the mount point is the fifth field, a field holding
// one hyphen ends the optional fields, and the mount source is the second
// field after it.
const (
	mountinfoMountPointField = 4
	mountinfoSeparator       = "-"
	mountinfoSourceOffset    = 2
)

// errPersistentSizesNotImplemented is what DetectPersistentStorage answers for
// a persistent partition it found mounted, until the sizes are read.
var errPersistentSizesNotImplemented = errors.New("reading the sizes of the persistent storage is not implemented yet")

// errPersistentMountSource is what DetectPersistentStorage answers when the
// mount table holds a line for the persistent mount point that does not name
// its source: a line without the separator field, or cut short after it.
var errPersistentMountSource = errors.New("the mount table line for " + persistentMountPoint + " names no mount source")

// DetectPersistentStorage reads the persistent storage of an appliance. It
// answers ErrNoPersistentStorage when nothing is mounted on the persistent
// mount point, and any other error when the state could not be read: a caller
// MUST NOT read that error as storage that is absent or grown.
//
// It is a wiring stub past the mount table. For a persistent partition that
// is mounted it reads no size and answers an error that names the device. The
// device-surface phase of
// plan/post-release/features/spec-appliance-perm-grows-at-install.md reads the
// disk, partition and filesystem sizes and fills the value.
func (d *Detector) DetectPersistentStorage() (PersistentStorage, error) {
	device, err := d.persistentDevice()
	if err != nil {
		return PersistentStorage{}, err
	}
	return PersistentStorage{}, fmt.Errorf("persistent storage %s: %w", device, errPersistentSizesNotImplemented)
}

// persistentDevice answers the source of the mount on the persistent mount
// point, from the mount table of the calling process. A later line for the
// same mount point wins, because it is the mount that covers the earlier one.
// The loop is bounded by the mount table the kernel printed.
func (d *Detector) persistentDevice() (string, error) {
	data, err := os.ReadFile(d.procPath("self", "mountinfo")) //nolint:gosec // path built from root
	if err != nil {
		return "", fmt.Errorf("read the mount table: %w", err)
	}
	device := ""
	mounted := false
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) <= mountinfoMountPointField {
			continue
		}
		if fields[mountinfoMountPointField] != persistentMountPoint {
			continue
		}
		separator := slices.Index(fields, mountinfoSeparator)
		if separator < 0 {
			return "", errPersistentMountSource
		}
		if separator+mountinfoSourceOffset >= len(fields) {
			return "", errPersistentMountSource
		}
		device = fields[separator+mountinfoSourceOffset]
		mounted = true
	}
	if !mounted {
		return "", ErrNoPersistentStorage
	}
	return device, nil
}

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
	s.Smart = d.detectSMART(name)
	return s
}

// classifyTransport picks a transport label from the device name. The
// kernel exposes the real transport in `/sys/class/block/<n>/device/`
// attributes but they vary by bus — name-based classification is
// accurate and portable enough for inventory purposes.
func classifyTransport(name string) string {
	if name == "" {
		return strUnknown
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
	return strUnknown
}
