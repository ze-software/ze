//go:build linux

package host

import (
	"slices"
	"testing"
)

// VALIDATES: AC-9 — `show host kernel` returns release, version,
// cmdline, microcode-revision, boot-time.
func TestDetectKernel_Basics(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	k, err := d.DetectKernel()
	if err != nil {
		t.Fatalf("DetectKernel: %v", err)
	}
	if k.Release != "6.8.0-110-generic" {
		t.Errorf("Release = %q, want 6.8.0-110-generic", k.Release)
	}
	if k.Cmdline == "" {
		t.Error("Cmdline empty")
	}
	if k.MicrocodeRevision != "0x1c" {
		t.Errorf("MicrocodeRevision = %q, want 0x1c", k.MicrocodeRevision)
	}
	if k.BootTimeUnix != 1744300000 {
		t.Errorf("BootTimeUnix = %d, want 1744300000", k.BootTimeUnix)
	}
	if k.BootTime == "" {
		t.Error("BootTime empty (should be RFC3339)")
	}
}

// VALIDATES: AC-9 arch-flags — security-relevant flags are extracted
// from /proc/cpuinfo's flags list and sorted.
func TestDetectKernel_ArchFlags(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	k, err := d.DetectKernel()
	if err != nil {
		t.Fatalf("DetectKernel: %v", err)
	}
	// Fixture cpuinfo flags include: smep smap ibt user_shstk ibrs
	for _, want := range []string{"smep", "smap", "ibt", "user_shstk", "ibrs"} {
		if !slices.Contains(k.ArchFlags, want) {
			t.Errorf("ArchFlags missing %q; got %v", want, k.ArchFlags)
		}
	}
	// Non-security flags (e.g. fpu, sse) must NOT leak through the filter.
	for _, banned := range []string{"fpu", "sse", "sse2", "aes", "avx"} {
		if slices.Contains(k.ArchFlags, banned) {
			t.Errorf("ArchFlags leaked %q (should be filtered)", banned)
		}
	}
}

// VALIDATES: host section — hostname (OS), uptime (/proc/uptime first field).
func TestDetectHost_Uptime(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	h, err := d.DetectHost()
	if err != nil {
		t.Fatalf("DetectHost: %v", err)
	}
	// /proc/uptime fixture: "3600.42 14400.18" → 3600 seconds.
	if got, want := h.UptimeSeconds, uint64(3600); got != want {
		t.Errorf("UptimeSeconds = %d, want %d", got, want)
	}
	if h.Hostname == "" {
		t.Error("Hostname empty (os.Hostname should succeed)")
	}
}

// VALIDATES: parseInt64 accepts plain digits and rejects anything else.
func TestParseInt64(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		bad  bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"1744300000", 1744300000, false},
		{"abc", 0, true},
		{"12a", 0, true},
		{"", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			v, bad := parseInt64(tc.in)
			if bad != tc.bad {
				t.Fatalf("bad = %v, want %v", bad, tc.bad)
			}
			if !bad && v != tc.want {
				t.Errorf("v = %d, want %d", v, tc.want)
			}
		})
	}
}
