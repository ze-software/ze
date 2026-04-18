//go:build linux

package host

import (
	"testing"
)

// VALIDATES: AC-1 — `show host cpu` on linux returns vendor, model-name,
// family, model, stepping, logical-cpus, physical-cores, threads-per-core,
// scaling-driver, hwp-available, base-freq-mhz, max-freq-mhz, microcode.
// PREVENTS: regressions where /proc/cpuinfo parser drops fields silently.
func TestDetectCPU_ParsesCPUInfo(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU: %v", err)
	}
	if got, want := cpu.Vendor, CPUVendorIntel; got != want {
		t.Errorf("Vendor = %v, want %v", got, want)
	}
	if got, want := cpu.ModelName, "Intel(R) N100"; got != want {
		t.Errorf("ModelName = %q, want %q", got, want)
	}
	if got, want := cpu.Family, 6; got != want {
		t.Errorf("Family = %d, want %d", got, want)
	}
	if got, want := cpu.Model, 190; got != want {
		t.Errorf("Model = %d, want %d", got, want)
	}
	if got, want := cpu.LogicalCPUs, 4; got != want {
		t.Errorf("LogicalCPUs = %d, want %d", got, want)
	}
	if got, want := cpu.PhysicalCores, 4; got != want {
		t.Errorf("PhysicalCores = %d, want %d", got, want)
	}
	if got, want := cpu.ThreadsPerCore, 1; got != want {
		t.Errorf("ThreadsPerCore = %d, want %d", got, want)
	}
	if got, want := cpu.ScalingDriver, ScalingDriverIntelPState; got != want {
		t.Errorf("ScalingDriver = %v, want %v", got, want)
	}
	if !cpu.HWPAvailable {
		t.Error("HWPAvailable = false, want true (hwp flag is set in fixture)")
	}
	if got, want := cpu.Microcode, "0x1c"; got != want {
		t.Errorf("Microcode = %q, want %q", got, want)
	}
	if len(cpu.Cores) != 4 {
		t.Fatalf("Cores len = %d, want 4", len(cpu.Cores))
	}
}

// VALIDATES: AC-17 — non-hybrid CPU (N100 all-E) reports hybrid=false and
// every core role=uniform.
func TestDetectCPU_Uniform_N100(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU: %v", err)
	}
	if cpu.Hybrid {
		t.Error("Hybrid = true, want false")
	}
	for _, c := range cpu.Cores {
		if c.Role != CoreRoleUniform {
			t.Errorf("cpu%d: Role = %v, want uniform", c.CPU, c.Role)
		}
		if c.Capacity != 1024 {
			t.Errorf("cpu%d: Capacity = %d, want 1024", c.CPU, c.Capacity)
		}
	}
}

// VALIDATES: AC-16 — hybrid detection on Alder Lake+ Intel sets
// hybrid=true and splits cores into performance (cpu_capacity=1024) and
// efficient (cpu_capacity=768).
func TestDetectCPU_Hybrid_AlderLake(t *testing.T) {
	d := &Detector{Root: "testdata/alder-lake-hybrid"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU: %v", err)
	}
	if !cpu.Hybrid {
		t.Fatal("Hybrid = false, want true")
	}
	var p, e int
	for _, c := range cpu.Cores {
		switch c.Role {
		case CoreRolePerformance:
			p++
			if c.Capacity != 1024 {
				t.Errorf("P-core cpu%d: Capacity = %d, want 1024", c.CPU, c.Capacity)
			}
		case CoreRoleEfficient:
			e++
			if c.Capacity != 768 {
				t.Errorf("E-core cpu%d: Capacity = %d, want 768", c.CPU, c.Capacity)
			}
		case CoreRoleUniform, CoreRoleUnknown:
			t.Errorf("cpu%d: unexpected role %v on hybrid CPU", c.CPU, c.Role)
		}
	}
	if p != 4 || e != 4 {
		t.Errorf("P/E split = %d/%d, want 4/4", p, e)
	}
}

// VALIDATES: AC-1 freq fields — base-freq-mhz, max-freq-mhz, per-core
// current-freq-mhz populated from sysfs.
func TestDetectCPU_Frequencies(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU: %v", err)
	}
	if got, want := cpu.BaseFreqMHz, 800; got != want {
		t.Errorf("BaseFreqMHz = %d, want %d", got, want)
	}
	if got, want := cpu.MaxFreqMHz, 3400; got != want {
		t.Errorf("MaxFreqMHz = %d, want %d", got, want)
	}
	// Fixture sets 1700/1750/1800/1850 MHz for cpus 0-3; average = 1775.
	if got, want := cpu.CurrentFreqAvgMHz, 1775; got != want {
		t.Errorf("CurrentFreqAvgMHz = %d, want %d", got, want)
	}
	// Each core has a distinct current freq.
	expected := []int{1700, 1750, 1800, 1850}
	for i, c := range cpu.Cores {
		if c.CurrentFreqMHz != expected[i] {
			t.Errorf("cpu%d CurrentFreqMHz = %d, want %d", c.CPU, c.CurrentFreqMHz, expected[i])
		}
	}
}

// VALIDATES: AC-1 throttle fields — per-core throttle counts read from
// thermal_throttle/ in sysfs, zero when fixture reports no throttle
// events.
func TestDetectCPU_ThrottleCounts(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU: %v", err)
	}
	for _, c := range cpu.Cores {
		if c.CoreThrottleCount != 0 {
			t.Errorf("cpu%d CoreThrottleCount = %d, want 0", c.CPU, c.CoreThrottleCount)
		}
		if c.PackageThrottleCount != 0 {
			t.Errorf("cpu%d PackageThrottleCount = %d, want 0", c.CPU, c.PackageThrottleCount)
		}
	}
}

// VALIDATES: AC-22 — malformed /proc/cpuinfo lines (no colon, empty
// key) are skipped and parsing continues; known-parseable fields in
// the same block remain populated. Uses a fixture with two deliberate
// junk lines and verifies the CPU/model/flags are still recovered.
// PREVENTS: a brittle parser that aborts on the first unexpected line.
func TestDetectCPU_SkipsMalformedLine(t *testing.T) {
	d := &Detector{Root: "testdata/malformed-cpuinfo"}
	cpu, err := d.DetectCPU()
	if err != nil {
		t.Fatalf("DetectCPU on fixture with junk lines: %v", err)
	}
	if got, want := cpu.LogicalCPUs, 2; got != want {
		t.Errorf("LogicalCPUs = %d, want %d (both processor blocks must be recovered)", got, want)
	}
	if got, want := cpu.ModelName, "Dummy Parser Test CPU"; got != want {
		t.Errorf("ModelName = %q, want %q (must be recovered despite nearby junk line)", got, want)
	}
	if cpu.Vendor != CPUVendorIntel {
		t.Errorf("Vendor = %v, want intel", cpu.Vendor)
	}
	if !cpu.HWPAvailable {
		t.Error("HWPAvailable = false, want true (flags with hwp must parse)")
	}
}

// VALIDATES: DetectCPU specifically is safe for concurrent use —
// narrow complement to TestDetect_Concurrent in inventory_test.go
// which exercises Detect() across every section. This test lives
// in cpu_linux_test.go because it is scoped to the CPU detector
// and its sysfs reads.
// PREVENTS: a future refactor of cpu_linux.go introducing a package-
// level cache or shared buffer racing across CPU-specific callers.
func TestDetectCPU_Concurrent(t *testing.T) {
	d := &Detector{Root: "testdata/n100-4x-igc"}
	const n = 32
	done := make(chan *CPUInfo, n)
	errCh := make(chan error, n)
	for range n {
		go func() {
			cpu, err := d.DetectCPU()
			done <- cpu
			errCh <- err
		}()
	}
	seen := make(map[*CPUInfo]struct{})
	for range n {
		cpu := <-done
		err := <-errCh
		if err != nil {
			t.Errorf("concurrent DetectCPU: %v", err)
		}
		if cpu == nil {
			t.Error("DetectCPU returned nil under concurrency")
			continue
		}
		if _, dup := seen[cpu]; dup {
			t.Error("DetectCPU returned the same *CPUInfo pointer twice — callers must get independent values")
		}
		seen[cpu] = struct{}{}
	}
}
