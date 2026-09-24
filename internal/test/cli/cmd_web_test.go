package cli

import (
	"os"
	"strconv"
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

// TestZeTestBrowserSessionIsPerRun proves a test's browser session is owned by
// its run. Method: the name carries this process id and the test nick, so two
// concurrent ze-test web runs never drive one browser for the same test number.
//
// VALIDATES: zeTestBrowserSession embeds the pid and the nick.
// PREVENTS: sessions keyed by nick alone, where one run's test N navigated and
// closed the browser another run's test N was reading.
func TestZeTestBrowserSessionIsPerRun(t *testing.T) {
	got := zeTestBrowserSession("12")
	want := "ze-test-web-" + strconv.Itoa(os.Getpid()) + "-12"
	if got != want {
		t.Fatalf("zeTestBrowserSession(12) = %q, want %q", got, want)
	}
	if zeTestBrowserSession("1") == zeTestBrowserSession("12") {
		t.Fatal("two nicks share one session name")
	}
}
