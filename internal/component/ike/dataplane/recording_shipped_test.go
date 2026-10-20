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
