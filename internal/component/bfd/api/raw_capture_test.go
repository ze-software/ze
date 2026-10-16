package api

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

type fakeRawCapture struct{ enabled bool }

func (f *fakeRawCapture) EnableRawCapture()  { f.enabled = true }
func (f *fakeRawCapture) DisableRawCapture() { f.enabled = false }
func (f *fakeRawCapture) BFDRawCaptureSnapshot(int) []plugin.BGPRawCaptureEntry {
	return nil
}

// TestRawCaptureProviderPublication verifies the provider the bfd plugin
// publishes is the one a reader gets, and that clearing it with nil makes the
// reader see none.
//
// VALIDATES: SetRawCaptureProvider / RawCaptureProvider round trip and the
// nil clear the bfd plugin's shutdown path relies on.
// PREVENTS: capture-raw reaching a provider whose engine has torn down.
func TestRawCaptureProviderPublication(t *testing.T) {
	t.Cleanup(func() { SetRawCaptureProvider(nil) })

	if got := RawCaptureProvider(); got != nil {
		t.Fatalf("provider before publication = %v, want nil", got)
	}
	fake := &fakeRawCapture{}
	SetRawCaptureProvider(fake)
	got := RawCaptureProvider()
	if got == nil {
		t.Fatal("published provider not returned")
	}
	got.EnableRawCapture()
	if !fake.enabled {
		t.Fatal("returned provider is not the published one")
	}
	SetRawCaptureProvider(nil)
	if got := RawCaptureProvider(); got != nil {
		t.Fatalf("provider after clear = %v, want nil", got)
	}
}
