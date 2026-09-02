// VALIDATES: AC-10/AC-11 (disk detection parity with shell)
// PREVENTS: installer writing to wrong disk or failing to find target

package disk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestFindTargetDiskRefusesSeveralCandidates drives findTargetDisk over a
// fixture /sys/block tree. The goal is the refusal: two fixed disks and no
// explicit target must produce an error naming both, in EVERY mode, because
// nothing in a directory listing says which disk the operator wants
// overwritten.
func TestFindTargetDiskRefusesSeveralCandidates(t *testing.T) {
	dir := t.TempDir()
	writeBlockDevice(t, dir, "sda", "0")
	writeBlockDevice(t, dir, "sdb", "0")

	got, err := findTargetDisk("", nil, dir)
	if err == nil {
		t.Fatalf("findTargetDisk over two fixed disks = %q, want an error", got)
	}
	if !strings.Contains(err.Error(), "sda") || !strings.Contains(err.Error(), "sdb") {
		t.Errorf("error %q names neither candidate, want both", err)
	}
}

// TestFindTargetDiskSingleCandidate proves the refusal above is not a blanket
// one: one fixed disk still installs without an explicit target.
func TestFindTargetDiskSingleCandidate(t *testing.T) {
	dir := t.TempDir()
	writeBlockDevice(t, dir, "sda", "0")
	writeBlockDevice(t, dir, "sr0", "1")
	writeBlockDevice(t, dir, "loop0", "0")

	got, err := findTargetDisk("", nil, dir)
	if err != nil {
		t.Fatalf("findTargetDisk over one fixed disk: %v", err)
	}
	if got != "/dev/sda" {
		t.Errorf("findTargetDisk = %q, want /dev/sda", got)
	}
}

// TestFindTargetDiskExcludesSourceMedia proves the ISO source media is not a
// candidate, so booting an ISO off one of two disks still resolves.
func TestFindTargetDiskExcludesSourceMedia(t *testing.T) {
	dir := t.TempDir()
	writeBlockDevice(t, dir, "sda", "0")
	writeBlockDevice(t, dir, "sdb", "0")

	got, err := findTargetDisk("", []string{"sdb"}, dir)
	if err != nil {
		t.Fatalf("findTargetDisk with sdb as source media: %v", err)
	}
	if got != "/dev/sda" {
		t.Errorf("findTargetDisk = %q, want /dev/sda", got)
	}
}

// TestFindTargetDiskExplicitTarget proves an explicit ze.target skips detection
// entirely, which is the operator's answer to the refusal above.
func TestFindTargetDiskExplicitTarget(t *testing.T) {
	dir := t.TempDir()
	writeBlockDevice(t, dir, "sda", "0")
	writeBlockDevice(t, dir, "sdb", "0")

	got, err := findTargetDisk("/dev/sdb", nil, dir)
	if err != nil {
		t.Fatalf("findTargetDisk with an explicit target: %v", err)
	}
	if got != "/dev/sdb" {
		t.Errorf("findTargetDisk = %q, want /dev/sdb", got)
	}
}

// writeBlockDevice creates one /sys/block entry: a directory named for the
// device, holding the removable flag the kernel exposes.
func writeBlockDevice(t *testing.T, root, name, removable string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "removable"), []byte(removable+"\n"), 0o644); err != nil {
		t.Fatalf("write removable for %s: %v", name, err)
	}
}
