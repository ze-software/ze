package cli

import (
	"strings"
	"testing"
)

// VALIDATES: the web suite fails hard (not silently skips) when agent-browser
// is absent during a verify-gate run, and still skips for casual local runs.
// PREVENTS: a green `./le verify current mode full` or CI pass that silently
// excluded all .wb web tests because agent-browser was not installed.
func TestWebBrowserMissingFailsInVerifyMode(t *testing.T) {
	err := webBrowserMissing(true)
	if err == nil {
		t.Fatal("verify mode must fail hard when agent-browser is missing, got nil")
	}
	for _, want := range []string{"agent-browser", "ZE_SKIP_SUITES=web"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err.Error(), want)
		}
	}
}

func TestWebBrowserMissingSkipsOutsideVerifyMode(t *testing.T) {
	if err := webBrowserMissing(false); err != nil {
		t.Fatalf("local runs without agent-browser must skip, got error: %v", err)
	}
}
