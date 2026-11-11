// Design: docs/architecture/appliance/iso-installer.md -- optical media discovery.

package disk

import "testing"

func TestISOMediaSearchIncludesOpticalWithoutAllowingOpticalTargets(t *testing.T) {
	for _, name := range []string{"sr0", "sr1", "sda", "vda", "nvme0n1"} {
		if !isISOMediaDisk(name) {
			t.Errorf("installer media discovery excluded %q", name)
		}
	}
	for _, name := range []string{"loop0", "ram0", "dm-0", "fd0", "md0", "zram0", "mtdblock0"} {
		if isISOMediaDisk(name) {
			t.Errorf("installer media discovery included virtual device %q", name)
		}
	}
	if !isSkippedDisk("sr0") {
		t.Error("optical media became eligible as an installation target")
	}
}
