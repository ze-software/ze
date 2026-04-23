// Design: docs/architecture/core-design.md — Netdata-compatible OS metric collection

//go:build linux

package collector

import (
	"bufio"
	"os"
	"strings"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

type diskSpaceCollector struct {
	gauge metrics.GaugeVec
}

func newDiskSpaceCollector() *diskSpaceCollector {
	return &diskSpaceCollector{}
}

func (c *diskSpaceCollector) Name() string { return "diskspace" }

func (c *diskSpaceCollector) Init(reg metrics.Registry, prefix string) {
	c.gauge = reg.GaugeVec(
		prefix+"_disk_space_GiB_average",
		"Disk Space Usage",
		[]string{"chart", "dimension", "family"},
	)
}

func (c *diskSpaceCollector) Collect() error {
	mounts, err := readMountPoints()
	if err != nil {
		return err
	}

	for _, mp := range mounts {
		var st syscall.Statfs_t
		if err := syscall.Statfs(mp.mountpoint, &st); err != nil {
			continue
		}
		if st.Blocks == 0 {
			continue
		}

		bsize := uint64(st.Bsize) //nolint:unconvert // Bsize type varies by platform
		totalBytes := st.Blocks * bsize
		freeBytes := st.Bfree * bsize
		availBytes := st.Bavail * bsize
		usedBytes := totalBytes - freeBytes
		reservedBytes := freeBytes - availBytes

		const gib = 1024 * 1024 * 1024
		chart := "disk_space." + sanitizeMountpoint(mp.mountpoint)
		family := mp.mountpoint

		c.gauge.With(chart, "avail", family).Set(float64(availBytes) / gib)
		c.gauge.With(chart, "used", family).Set(float64(usedBytes) / gib)
		c.gauge.With(chart, "reserved_for_root", family).Set(float64(reservedBytes) / gib)
	}

	return nil
}

type mountPoint struct {
	device     string
	mountpoint string
	fstype     string
}

func readMountPoints() ([]mountPoint, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var mounts []mountPoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		fstype := fields[2]
		if !isRealFS(fstype) {
			continue
		}
		mounts = append(mounts, mountPoint{
			device:     fields[0],
			mountpoint: fields[1],
			fstype:     fstype,
		})
	}
	return mounts, scanner.Err()
}

func isRealFS(fstype string) bool {
	switch fstype {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "zfs", "vfat", "ntfs", "f2fs", "bcachefs":
		return true
	}
	return false
}

func sanitizeMountpoint(mp string) string {
	if mp == "/" {
		return "_"
	}
	r := strings.NewReplacer("/", "_", " ", "_")
	return r.Replace(mp)
}
