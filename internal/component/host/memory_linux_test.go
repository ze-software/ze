//go:build linux

package host

import (
	"testing"
)

// VALIDATES: AC-6 — `show host memory` returns total/free/available,
// buffers/cached, and swap totals converted from kB (kernel) to bytes.
func TestDetectMemory_FromMeminfo(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	m, err := d.DetectMemory()
	if err != nil {
		t.Fatalf("DetectMemory: %v", err)
	}
	// Fixture: MemTotal 8053028 kB -> 8246300672 bytes
	if got, want := m.TotalBytes, uint64(8053028*1024); got != want {
		t.Errorf("TotalBytes = %d, want %d", got, want)
	}
	if got, want := m.FreeBytes, uint64(1204568*1024); got != want {
		t.Errorf("FreeBytes = %d, want %d", got, want)
	}
	if got, want := m.AvailableBytes, uint64(5612364*1024); got != want {
		t.Errorf("AvailableBytes = %d, want %d", got, want)
	}
	if got, want := m.SwapTotalBytes, uint64(2097148*1024); got != want {
		t.Errorf("SwapTotalBytes = %d, want %d", got, want)
	}
	if got, want := m.SwapFreeBytes, uint64(2097148*1024); got != want {
		t.Errorf("SwapFreeBytes = %d, want %d", got, want)
	}
	if got, want := m.BuffersBytes, uint64(208124*1024); got != want {
		t.Errorf("BuffersBytes = %d, want %d", got, want)
	}
	if got, want := m.CachedBytes, uint64(3908496*1024); got != want {
		t.Errorf("CachedBytes = %d, want %d", got, want)
	}
}

// VALIDATES: AC-6 absent edac — ECC counters stay zero and ECCPresent
// is false when /sys/devices/system/edac/mc is missing.
func TestDetectMemory_NoECC(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	m, err := d.DetectMemory()
	if err != nil {
		t.Fatalf("DetectMemory: %v", err)
	}
	if m.ECCPresent {
		t.Error("ECCPresent = true, want false (no edac in fixture)")
	}
	if m.ECCCorrectableErrors != 0 || m.ECCUncorrectableErrors != 0 {
		t.Errorf("ECC counters = (%d,%d), want (0,0)",
			m.ECCCorrectableErrors, m.ECCUncorrectableErrors)
	}
}

// VALIDATES: parseMeminfoLine handles the canonical "Key: N kB" format
// and returns ok=false for unit-less or malformed lines.
func TestParseMeminfoLine(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		key   string
		kb    uint64
		okExp bool
	}{
		{"canonical", "MemTotal:        8053028 kB", "MemTotal", 8053028, true},
		{"zero", "SwapFree: 0 kB", "SwapFree", 0, true},
		{"no colon", "MemTotal 8053028 kB", "", 0, false},
		{"no number", "MemTotal: junk kB", "", 0, false},
		{"empty", "", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, kb, ok := parseMeminfoLine(tc.line)
			if ok != tc.okExp {
				t.Fatalf("ok = %v, want %v", ok, tc.okExp)
			}
			if !ok {
				return
			}
			if k != tc.key {
				t.Errorf("key = %q, want %q", k, tc.key)
			}
			if kb != tc.kb {
				t.Errorf("kb = %d, want %d", kb, tc.kb)
			}
		})
	}
}
