package rsvpte

import (
	"testing"
)

func TestRSVPAdmissionReserveAndRelease(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)

	if err := ac.Reserve("eth0", 3e9); err != nil {
		t.Fatalf("Reserve 3Gbps: %v", err)
	}

	ib, ok := ac.GetInterface("eth0")
	if !ok {
		t.Fatal("GetInterface failed")
	}
	if ib.ReservedBandwidth != 3e9 {
		t.Errorf("Reserved = %g, want %g", ib.ReservedBandwidth, float32(3e9))
	}
	if ib.Available() != 5e9 {
		t.Errorf("Available = %g, want %g", ib.Available(), float32(5e9))
	}

	ac.Release("eth0", 1e9)
	ib, _ = ac.GetInterface("eth0")
	if ib.ReservedBandwidth != 2e9 {
		t.Errorf("Reserved after release = %g, want %g", ib.ReservedBandwidth, float32(2e9))
	}
}

func TestRSVPAdmissionDenied(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)

	if err := ac.Reserve("eth0", 7e9); err != nil {
		t.Fatalf("Reserve 7Gbps: %v", err)
	}

	err := ac.Reserve("eth0", 2e9)
	if err == nil {
		t.Fatal("expected admission denied for oversubscription")
	}
}

func TestRSVPAdmissionUnknownInterface(t *testing.T) {
	ac := newAdmissionController()

	if err := ac.Reserve("eth99", 1e9); err != nil {
		t.Fatalf("Reserve on unknown interface should succeed (no limit): %v", err)
	}
}

func TestRSVPAdmissionReleaseFloor(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)

	ac.Release("eth0", 5e9)
	ib, _ := ac.GetInterface("eth0")
	if ib.ReservedBandwidth != 0 {
		t.Errorf("Reserved after over-release = %g, want 0", ib.ReservedBandwidth)
	}
}

func TestRSVPAdmissionAllInterfaces(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)
	ac.setInterface("eth1", 1e9, 800e6)

	all := ac.allInterfaces()
	if len(all) != 2 {
		t.Fatalf("AllInterfaces returned %d, want 2", len(all))
	}
	if _, ok := all["eth0"]; !ok {
		t.Error("missing eth0")
	}
	if _, ok := all["eth1"]; !ok {
		t.Error("missing eth1")
	}
}

func TestRSVPAdmissionExactLimit(t *testing.T) {
	ac := newAdmissionController()
	ac.setInterface("eth0", 10e9, 8e9)

	if err := ac.Reserve("eth0", 8e9); err != nil {
		t.Fatalf("Reserve exactly at limit should succeed: %v", err)
	}

	err := ac.Reserve("eth0", 1)
	if err == nil {
		t.Fatal("expected denial when at exact capacity")
	}
}
