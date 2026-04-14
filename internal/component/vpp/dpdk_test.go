package vpp

import "testing"

func TestNewDPDKBinder(t *testing.T) {
	// VALIDATES: DPDKBinder construction
	// PREVENTS: nil map panic
	b := NewDPDKBinder()
	if b == nil {
		t.Fatal("NewDPDKBinder returned nil")
	}
	if b.savedDrivers == nil {
		t.Fatal("savedDrivers map not initialized")
	}
}

func TestDPDKBinderValidatesPCI(t *testing.T) {
	// VALIDATES: AC-10 -- invalid PCI address rejected during bind
	// PREVENTS: sysfs path traversal
	b := NewDPDKBinder()
	err := b.BindAll([]DPDKInterface{
		{PCIAddress: "invalid-addr", Name: "xe0"},
	})
	if err == nil {
		t.Error("expected error for invalid PCI address")
	}
}
