package cmd

import (
	"testing"

	bfdapi "github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/plugin"
)

type fakeBFDRawCapture struct{ enabled bool }

func (f *fakeBFDRawCapture) EnableRawCapture()  { f.enabled = true }
func (f *fakeBFDRawCapture) DisableRawCapture() { f.enabled = false }
func (f *fakeBFDRawCapture) BFDRawCaptureSnapshot(int) []plugin.BGPRawCaptureEntry {
	return nil
}

// TestCaptureRawBFDReadsProviderFromBFDAPI verifies capture-raw reaches the
// BFD capture through the provider the bfd plugin publishes in bfd/api, and
// answers today's not-loaded error when none is published.
//
// VALIDATES: AC-13: start and stop drive the bfdapi provider; dump with no
// provider reports "BFD plugin not loaded".
// PREVENTS: the diag command silently skipping BFD after the seam moved out
// of this package. Not parallel: it publishes a process-wide provider.
func TestCaptureRawBFDReadsProviderFromBFDAPI(t *testing.T) {
	t.Cleanup(func() { bfdapi.SetRawCaptureProvider(nil) })

	bfdapi.SetRawCaptureProvider(nil)
	resp, err := captureRawDump(nil, capBFD, "json", 0)
	if err != nil {
		t.Fatalf("dump without provider: %v", err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("dump data = %T, want plugin.Map", resp.Data)
	}
	if data["bfd"] != "BFD plugin not loaded" {
		t.Fatalf("dump without provider: bfd = %v, want %q", data["bfd"], "BFD plugin not loaded")
	}

	fake := &fakeBFDRawCapture{}
	bfdapi.SetRawCaptureProvider(fake)
	if _, err := captureRawStart(nil, capBFD); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !fake.enabled {
		t.Fatal("start did not enable the provider published in bfd/api")
	}
	if _, err := captureRawStop(nil, capBFD); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if fake.enabled {
		t.Fatal("stop did not disable the provider published in bfd/api")
	}
}
