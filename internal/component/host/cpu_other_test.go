//go:build !linux

package host

import (
	"errors"
	"testing"
)

// VALIDATES: AC-2 — `show host cpu` on darwin returns ErrUnsupported so
// the caller can omit the `hardware` enrichment on `show system cpu` on
// non-Linux platforms.
func TestDetectCPU_Darwin(t *testing.T) {
	d := &Detector{}
	cpu, err := d.DetectCPU()
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if cpu != nil {
		t.Errorf("cpu = %+v, want nil", cpu)
	}
}

// VALIDATES: Inventory.Detect on a non-Linux platform returns an empty
// Inventory (no sections, no Errors entry for ErrUnsupported).
func TestDetect_DarwinEmptyInventory(t *testing.T) {
	d := &Detector{}
	inv, err := d.Detect()
	if err != nil {
		t.Fatalf("Detect returned err = %v, want nil", err)
	}
	if inv == nil {
		t.Fatal("Inventory = nil, want non-nil")
	}
	if inv.CPU != nil {
		t.Errorf("CPU = %+v, want nil on darwin", inv.CPU)
	}
	if len(inv.Errors) != 0 {
		t.Errorf("Errors = %v, want empty (ErrUnsupported is not an error)", inv.Errors)
	}
}
