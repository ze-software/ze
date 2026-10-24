//go:build linux

package host

import (
	"os"
	"path/filepath"
	"testing"
)

// VALIDATES: AC-7 — thermal section lists hwmon sensors with name,
// label (device), temp-mc and alarm; and per-CPU throttle counters.
func TestDetectThermal_HwmonScan(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	th, err := d.detectThermal()
	if err != nil {
		t.Fatalf("detectThermal: %v", err)
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
	th, err := d.detectThermal()
	if err != nil {
		t.Fatalf("detectThermal: %v", err)
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

// TestDetectThermal_ZeroReadingIsAReading proves that a 0 degC hwmon reading
// is reported as a reading and that a channel whose value cannot be read is
// reported as a read failure, never as a reading and never dropped in silence.
// Method: a fixture hwmon node with temp1_input "0" and temp2_input holding no
// number; the first must come back with TempMC 0, the second in Errors.
func TestDetectThermal_ZeroReadingIsAReading(t *testing.T) {
	root := t.TempDir()
	node := filepath.Join(root, "sys", "class", "hwmon", "hwmon0")
	if err := os.MkdirAll(node, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"name":        "acpitz\n",
		"temp1_input": "0\n",
		"temp2_input": "not-a-number\n",
	}
	for leaf, body := range files {
		if err := os.WriteFile(filepath.Join(node, leaf), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	th, err := (&Detector{Root: root}).detectThermal()
	if err != nil {
		t.Fatalf("detectThermal: %v", err)
	}
	if got, want := len(th.Sensors), 1; got != want {
		t.Fatalf("len(Sensors) = %d, want %d (the 0 degC channel only): %+v", got, want, th.Sensors)
	}
	if th.Sensors[0].TempMC != 0 {
		t.Errorf("Sensors[0].TempMC = %d, want 0", th.Sensors[0].TempMC)
	}
	unreadable := filepath.Join(node, "temp2_input")
	if len(th.Errors) != 1 || th.Errors[0].Path != unreadable {
		t.Fatalf("Errors = %+v, want one entry for %s", th.Errors, unreadable)
	}
}

// TestDetectThermal_UnlistableNodeIsAReadFailure proves that an hwmon node
// whose directory cannot be listed is reported in Errors, not dropped as a
// node with no sensor. Method: a node whose name file is readable but whose
// directory has no read permission, so os.ReadDir fails with EACCES; the node
// must appear in Errors. Root reads any directory, so the test skips as root.
func TestDetectThermal_UnlistableNodeIsAReadFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists a directory with no read permission")
	}
	root := t.TempDir()
	node := filepath.Join(root, "sys", "class", "hwmon", "hwmon0")
	if err := os.MkdirAll(node, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(node, "name"), []byte("acpitz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(node, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(node, 0o755) })

	th, err := (&Detector{Root: root}).detectThermal()
	if err != nil {
		t.Fatalf("detectThermal: %v", err)
	}
	if len(th.Errors) != 1 || th.Errors[0].Path != node {
		t.Fatalf("Errors = %+v, want one entry for %s", th.Errors, node)
	}
}
