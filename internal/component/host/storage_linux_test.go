//go:build linux

package host

import (
	"testing"
)

// VALIDATES: AC-8 — `show host storage` lists block devices with
// name, size-bytes, model, serial, transport, rotational, and NVMe
// firmware where applicable. Partitions are excluded.
func TestDetectStorage_BlockDevices(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	s, err := d.DetectStorage()
	if err != nil {
		t.Fatalf("DetectStorage: %v", err)
	}
	if got, want := len(s.Devices), 2; got != want {
		t.Fatalf("len(Devices) = %d, want %d (partitions should be excluded)", got, want)
	}
	var nvme, sda *StorageDevice
	for i := range s.Devices {
		switch s.Devices[i].Name {
		case "nvme0n1":
			nvme = &s.Devices[i]
		case "sda":
			sda = &s.Devices[i]
		}
	}
	if nvme == nil {
		t.Fatal("nvme0n1 not found")
	}
	if nvme.Transport != "nvme" {
		t.Errorf("nvme Transport = %q, want nvme", nvme.Transport)
	}
	if nvme.Rotational {
		t.Error("nvme Rotational = true, want false")
	}
	if nvme.Model != "BKHD NVME-128G" {
		t.Errorf("nvme Model = %q", nvme.Model)
	}
	// 250069680 sectors * 512 = 128035676160 bytes (~128 GB).
	if got, want := nvme.SizeBytes, uint64(250069680)*512; got != want {
		t.Errorf("nvme SizeBytes = %d, want %d", got, want)
	}
	if nvme.NVMeFirmware != "1A3B5C7D" {
		t.Errorf("nvme firmware = %q, want 1A3B5C7D", nvme.NVMeFirmware)
	}
	if sda == nil {
		t.Fatal("sda not found")
	}
	if sda.Transport != "sata" {
		t.Errorf("sda Transport = %q, want sata", sda.Transport)
	}
	if !sda.Rotational {
		t.Error("sda Rotational = false, want true")
	}
	if got, want := sda.SizeBytes, uint64(1953525168)*512; got != want {
		t.Errorf("sda SizeBytes = %d, want %d", got, want)
	}
	if sda.NVMeFirmware != "" {
		t.Errorf("sda NVMeFirmware = %q, want empty (not NVMe)", sda.NVMeFirmware)
	}
}

// VALIDATES: transport classification covers common prefixes and
// defaults to "unknown" for unrecognized names.
func TestClassifyTransport(t *testing.T) {
	cases := []struct{ name, want string }{
		{"nvme0n1", "nvme"},
		{"nvme1n1", "nvme"},
		{"sda", "sata"},
		{"sdb", "sata"},
		{"mmcblk0", "mmc"},
		{"vda", "virtio"},
		{"loop0", "unknown"},
		{"", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyTransport(tc.name); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
