//go:build !linux

package userns

import (
	"os"
	"testing"
)

// TestEnterRunsInPlaceOffLinux proves that off Linux Enter hands the test body
// to the calling process: it answers true, and no child runs it, so no child
// marker is set. Run on the owner's Mac; compiled on Linux with GOOS=darwin.
func TestEnterRunsInPlaceOffLinux(t *testing.T) {
	if !Enter(t) {
		t.Fatal("Enter answered false off Linux, so the test body never ran")
	}
	if os.Getenv("ZE_TEST_USERNS_CHILD") != "" {
		t.Fatal("Enter off Linux ran the test in a re-executed child, not in place")
	}
}
