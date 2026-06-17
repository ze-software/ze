// VALIDATES: AC-10/AC-11 (disk detection parity with shell)
// PREVENTS: installer writing to wrong disk or failing to find target

package disk

import "testing"

func TestDiskNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/dev/sda", "sda"},
		{"/dev/sda1", "sda"},
		{"/dev/nvme0n1", "nvme0n1"},
		{"/dev/nvme0n1p4", "nvme0n1"},
		{"/dev/mmcblk0", "mmcblk0"},
		{"/dev/mmcblk0p1", "mmcblk0"},
		{"/dev/vda", "vda"},
		{"/dev/vda3", "vda"},
		{"/dev/xvda", "xvda"},
	}
	for _, tt := range tests {
		got := diskNameFromPath(tt.path)
		if got != tt.want {
			t.Errorf("diskNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestIsSkippedDiskName(t *testing.T) {
	skipped := []string{"loop0", "ram0", "dm-0", "sr0", "fd0", "md0", "zram0", "mtdblock0"}
	for _, name := range skipped {
		if !isSkippedDisk(name) {
			t.Errorf("isSkippedDisk(%q) = false, want true", name)
		}
	}

	kept := []string{"sda", "vda", "nvme0n1", "mmcblk0"}
	for _, name := range kept {
		if isSkippedDisk(name) {
			t.Errorf("isSkippedDisk(%q) = true, want false", name)
		}
	}
}

func TestPartitionPath(t *testing.T) {
	tests := []struct {
		disk string
		num  int
		want string
	}{
		{"/dev/sda", 4, "/dev/sda4"},
		{"/dev/nvme0n1", 4, "/dev/nvme0n1p4"},
		{"/dev/mmcblk0", 4, "/dev/mmcblk0p4"},
		{"/dev/vda", 1, "/dev/vda1"},
	}
	for _, tt := range tests {
		got := partitionPath(tt.disk, tt.num)
		if got != tt.want {
			t.Errorf("partitionPath(%q, %d) = %q, want %q", tt.disk, tt.num, got, tt.want)
		}
	}
}
