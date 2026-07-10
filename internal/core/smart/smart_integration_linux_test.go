// VALIDATES: Detect runs the real ATA/NVMe ioctl path against the block devices
// present on the host and returns a well-formed *Info (or nil when the device
// cannot be opened) without panicking. Auto-enrolled in the QEMU integration run
// via the derived `integration && linux` package list.
// PREVENTS: an ioctl-layout or errno-handling regression in detectATA/detectNVMe
// surfacing only on a live appliance disk scrape.

//go:build integration && linux

package smart

import (
	"os"
	"strings"
	"testing"
)

func TestDetectOnHostBlockDevices(t *testing.T) {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		t.Skipf("/sys/block unreadable: %v", err)
	}

	var probed int
	for _, e := range entries {
		name := e.Name()
		// Only probe real disks; skip loop/ram/dm virtual devices.
		if !strings.HasPrefix(name, "sd") &&
			!strings.HasPrefix(name, "nvme") &&
			!strings.HasPrefix(name, "vd") {
			continue
		}
		probed++
		// Detect must not panic; nil (unopenable) or an Info are both valid.
		if info := Detect(name, ""); info != nil && info.Unavailable && info.UnavailableNote == "" {
			t.Errorf("%s: Unavailable Info with empty note", name)
		}
	}
	if probed == 0 {
		t.Skip("no ATA/NVMe/virtio block devices present to probe")
	}
}
