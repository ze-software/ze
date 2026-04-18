//go:build linux

package host

import (
	"testing"
)

// VALIDATES: AC-7 — thermal section lists hwmon sensors with name,
// label (device), temp-mc and alarm; and per-CPU throttle counters.
func TestDetectThermal_HwmonScan(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	th, err := d.DetectThermal()
	if err != nil {
		t.Fatalf("DetectThermal: %v", err)
	}
	// Fixture: 1 acpitz + 1 nvme + 4 coretemp = 6 sensors.
	if got, want := len(th.Sensors), 6; got != want {
		t.Fatalf("len(Sensors) = %d, want %d", got, want)
	}
	// Names are sorted: acpitz, coretemp*4, nvme.
	if th.Sensors[0].Name != "acpitz" {
		t.Errorf("Sensors[0].Name = %q, want acpitz", th.Sensors[0].Name)
	}
	if th.Sensors[0].TempMC != 35000 {
		t.Errorf("Sensors[0].TempMC = %d, want 35000", th.Sensors[0].TempMC)
	}
	// Coretemp entries carry the label (Core N) in Device.
	for _, s := range th.Sensors {
		if s.Name == "coretemp" && s.Device == "" {
			t.Errorf("coretemp entry missing Device label")
		}
	}
	// NVMe label should be "Composite".
	for _, s := range th.Sensors {
		if s.Name == "nvme" && s.Device != "Composite" {
			t.Errorf("nvme label = %q, want Composite", s.Device)
		}
	}
}

// VALIDATES: AC-7 throttle — per-CPU entries read from
// /sys/devices/system/cpu/cpu*/thermal_throttle/.
func TestDetectThermal_ThrottleCounters(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	th, err := d.DetectThermal()
	if err != nil {
		t.Fatalf("DetectThermal: %v", err)
	}
	if got, want := len(th.Throttle), 4; got != want {
		t.Fatalf("len(Throttle) = %d, want %d", got, want)
	}
	for i, e := range th.Throttle {
		if e.CPU != i {
			t.Errorf("Throttle[%d].CPU = %d, want %d", i, e.CPU, i)
		}
		if e.CoreThrottleCount != 0 {
			t.Errorf("cpu%d CoreThrottleCount = %d, want 0", e.CPU, e.CoreThrottleCount)
		}
	}
}

// VALIDATES: parseCPUIndex extracts N from cpuN and rejects cpufreq,
// cpuidle, and other non-numeric suffixes.
func TestParseCPUIndex(t *testing.T) {
	cases := []struct {
		in    string
		n     int
		okExp bool
	}{
		{"cpu0", 0, true},
		{"cpu7", 7, true},
		{"cpu42", 42, true},
		{"cpufreq", 0, false},
		{"cpuidle", 0, false},
		{"cpu", 0, false},
		{"cpuN", 0, false},
		{"other", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			n, ok := parseCPUIndex(tc.in)
			if ok != tc.okExp {
				t.Fatalf("ok = %v, want %v", ok, tc.okExp)
			}
			if ok && n != tc.n {
				t.Errorf("n = %d, want %d", n, tc.n)
			}
		})
	}
}
