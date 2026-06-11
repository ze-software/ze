// VALIDATES: F17 (spec-web-ui-integrity) -- Host Hardware page shows populated
// data on Linux, not "No hardware information detected".
// PREVENTS: regression where _linux.go detectors silently return nil on a real
// kernel, causing the web UI to display the empty-inventory fallback.

//go:build integration && linux

package host

import (
	"testing"
)

func TestIntegration_DetectPopulatesInventory(t *testing.T) {
	d := &Detector{}
	inv, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if inv.CPU == nil {
		t.Fatal("CPU section is nil on a real Linux host")
	}
	if inv.CPU.LogicalCPUs < 1 {
		t.Errorf("CPU.LogicalCPUs = %d, want >= 1", inv.CPU.LogicalCPUs)
	}
	if inv.CPU.ModelName == "" {
		t.Error("CPU.ModelName is empty")
	}

	if inv.Memory == nil {
		t.Fatal("Memory section is nil on a real Linux host")
	}
	if inv.Memory.TotalBytes == 0 {
		t.Error("Memory.TotalBytes = 0, want > 0")
	}
	if inv.Memory.AvailableBytes == 0 {
		t.Error("Memory.AvailableBytes = 0, want > 0")
	}

	if inv.Kernel == nil {
		t.Fatal("Kernel section is nil on a real Linux host")
	}
	if inv.Kernel.Release == "" {
		t.Error("Kernel.Release is empty")
	}
	if inv.Kernel.Architecture == "" {
		t.Error("Kernel.Architecture is empty")
	}

	if inv.Host == nil {
		t.Fatal("Host section is nil on a real Linux host")
	}
	if inv.Host.UptimeSeconds == 0 {
		t.Error("Host.UptimeSeconds = 0, want > 0")
	}

	if inv.Platform == nil {
		t.Fatal("Platform section is nil on a real Linux host")
	}
	if inv.Platform.Type == PlatformUnknown {
		t.Error("Platform.Type = unknown, want a detected platform")
	}

	for _, e := range inv.Errors {
		t.Logf("detection error: path=%s err=%s", e.Path, e.Err)
	}
}

func TestIntegration_DetectSectionAll(t *testing.T) {
	result, err := DetectSection("all")
	if err != nil {
		t.Fatalf("DetectSection(all): %v", err)
	}
	inv, ok := result.(*Inventory)
	if !ok {
		t.Fatalf("DetectSection(all) returned %T, want *Inventory", result)
	}
	if inv.CPU == nil && inv.Memory == nil && inv.Kernel == nil {
		t.Error("DetectSection(all) returned an inventory with no populated sections")
	}
}

func TestIntegration_IndividualSections(t *testing.T) {
	sections := []string{"cpu", "memory", "kernel", "platform"}
	for _, name := range sections {
		t.Run(name, func(t *testing.T) {
			result, err := DetectSection(name)
			if err != nil {
				t.Fatalf("DetectSection(%s): %v", name, err)
			}
			if result == nil {
				t.Errorf("DetectSection(%s) returned nil", name)
			}
		})
	}
}
