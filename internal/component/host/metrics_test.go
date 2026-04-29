package host

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// VALIDATES: RegisterMetrics + CollectOnce do not panic with a
// NopRegistry and a nil Inventory (non-Linux platforms where Detect
// returns ErrUnsupported).
// PREVENTS: nil-pointer dereference when metrics are registered but
// host detection is unsupported.
func TestRegisterMetrics_NoPanic(t *testing.T) {
	var reg metrics.NopRegistry
	m := RegisterMetrics(reg)
	if m == nil {
		t.Fatal("RegisterMetrics returned nil")
	}
	// CollectOnce calls Detect() which returns ErrUnsupported on
	// non-Linux; the function should silently return without panic.
	m.CollectOnce()
}

// VALIDATES: collectFrom correctly populates gauges from a synthetic
// Inventory without requiring sysfs. Uses NopRegistry so Set/With
// calls succeed silently.
// PREVENTS: nil-section panic when only some inventory sections are
// populated (e.g. Memory present but CPU absent).
func TestCollectFrom_PartialInventory(t *testing.T) {
	var reg metrics.NopRegistry
	m := RegisterMetrics(reg)

	// Only Memory populated; all other sections nil.
	inv := &Inventory{
		Memory: &MemoryInfo{
			TotalBytes:     16_000_000_000,
			AvailableBytes: 8_000_000_000,
		},
	}
	m.collectFrom(inv) // must not panic
}

// VALIDATES: collectFrom handles a fully-populated Inventory with
// NICs, Storage, and Thermal sections.
// PREVENTS: index-out-of-range or nil dereference on per-device
// gauge-vec With() calls.
func TestCollectFrom_FullInventory(t *testing.T) {
	var reg metrics.NopRegistry
	m := RegisterMetrics(reg)

	inv := &Inventory{
		CPU: &CPUInfo{
			LogicalCPUs:   8,
			PhysicalCores: 4,
		},
		Memory: &MemoryInfo{
			TotalBytes:             32_000_000_000,
			AvailableBytes:         16_000_000_000,
			ECCCorrectableErrors:   3,
			ECCUncorrectableErrors: 0,
		},
		Host: &HostInfo{
			UptimeSeconds: 86400,
		},
		NICs: []NICInfo{
			{Name: "eth0", LinkSpeedMbps: 1000, Carrier: true},
			{Name: "eth1", LinkSpeedMbps: 10000, Carrier: false},
		},
		Storage: &StorageInfo{
			Devices: []StorageDevice{
				{Name: "sda", SizeBytes: 500_000_000_000},
				{Name: "nvme0n1", SizeBytes: 1_000_000_000_000},
			},
		},
		Thermal: &ThermalInfo{
			Sensors: []SensorReading{
				{Name: "coretemp", Device: "hwmon0", TempMC: 45000},
				{Name: "pch_cannonlake", Device: "hwmon1", TempMC: 52000},
			},
		},
	}
	m.collectFrom(inv) // must not panic
}

// VALIDATES: collectFrom with nil Inventory is a no-op.
// PREVENTS: nil-pointer dereference when Detect returns nil.
func TestCollectFrom_NilInventory(t *testing.T) {
	var reg metrics.NopRegistry
	m := RegisterMetrics(reg)
	m.collectFrom(nil) // must not panic
}
