//go:build !zetest

package dataplane

import (
	"strings"
	"testing"
)

// VALIDATES: a build without the zetest tag, which is every shipped ze, has no
// recording backend, so ze.test.ike.dataplane=recording cannot select one.
// METHOD: load the backend by name, as runEngine does, and expect the unknown-backend
// refusal.
// PREVENTS: a test-only backend that accepts every SPD write and programs no kernel
// reaching a shipped daemon, where it would report a catch-all no packet ever meets.
func TestRecordingBackendAbsentFromShippedBuild(t *testing.T) {
	err := Load("recording")
	if err == nil {
		t.Fatal("a shipped build loaded the recording dataplane")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Load(recording) = %v, want the unknown-backend refusal", err)
	}
}

// VALIDATES: a shipped build has no recording-spd-refused backend, so
// ze.test.ike.dataplane cannot make a shipped daemon refuse an operator SPD entry for
// a test's reason.
// METHOD: load the backend by name, as runEngine does, and expect the unknown-backend
// refusal.
// PREVENTS: a test-only refusal reaching an operator, whose entry at order 4242 would
// then never install.
func TestRecordingSPDRefusedBackendAbsentFromShippedBuild(t *testing.T) {
	err := Load("recording-spd-refused")
	if err == nil {
		t.Fatal("a shipped build loaded the recording-spd-refused dataplane")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Load(recording-spd-refused) = %v, want the unknown-backend refusal", err)
	}
}

// VALIDATES: a shipped build has no recording-esp-form-unavailable backend either, so
// ze.test.ike.dataplane cannot make a shipped daemon report a fast path it never tried
// to load.
// METHOD: load the backend by name, as runEngine does, and expect the unknown-backend
// refusal.
// PREVENTS: a test-only backend whose doctor finding names a test cause reaching an
// operator, beside an SPD no kernel holds.
func TestRecordingESPFormUnavailableBackendAbsentFromShippedBuild(t *testing.T) {
	err := Load("recording-esp-form-unavailable")
	if err == nil {
		t.Fatal("a shipped build loaded the recording-esp-form-unavailable dataplane")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Load(recording-esp-form-unavailable) = %v, want the unknown-backend refusal", err)
	}
}
