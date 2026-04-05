package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestShort(t *testing.T) {
	Stamp("1.2.3", "2026-01-01")
	defer Stamp("dev", "unknown")

	got := Short()
	if got != "ze 1.2.3 (built 2026-01-01)" {
		t.Errorf("Short() = %q, want %q", got, "ze 1.2.3 (built 2026-01-01)")
	}
}

func TestExtended(t *testing.T) {
	Stamp("1.2.3", "2026-01-01")
	defer Stamp("dev", "unknown")

	got := Extended()
	if !strings.HasPrefix(got, "ze 1.2.3 (built 2026-01-01)") {
		t.Errorf("Extended() should start with short version, got:\n%s", got)
	}
	if !strings.Contains(got, "go:") {
		t.Errorf("Extended() should contain go version, got:\n%s", got)
	}
	if !strings.Contains(got, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("Extended() should contain os/arch, got:\n%s", got)
	}
}

func TestHTTPHeader(t *testing.T) {
	Stamp("1.2.3", "2026-01-01")
	defer Stamp("dev", "unknown")

	got := HTTPHeader()
	if !strings.HasPrefix(got, "ze/1.2.3 (") {
		t.Errorf("HTTPHeader() should start with ze/1.2.3, got %q", got)
	}
	if !strings.Contains(got, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Errorf("HTTPHeader() should contain os/arch, got %q", got)
	}
}
