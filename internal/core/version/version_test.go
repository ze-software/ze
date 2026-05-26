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

func TestCompareReleases(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"26.05.26", "26.05.26", 0},
		{"26.05.25", "26.05.26", -1},
		{"26.05.27", "26.05.26", 1},
		{"26.04.30", "26.05.01", -1},
		{"25.12.31", "26.01.01", -1},
		{"27.01.01", "26.12.31", 1},
	}
	for _, tc := range cases {
		if got := CompareReleases(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareReleases(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareReleasesUnparseable(t *testing.T) {
	if got := CompareReleases("dev", "26.05.26"); got != -1 {
		t.Errorf("unparseable a is infinitely old, want -1, got %d", got)
	}
	if got := CompareReleases("26.05.26", "dev"); got != 1 {
		t.Errorf("unparseable b is infinitely old, want 1, got %d", got)
	}
	if got := CompareReleases("dev", "dev"); got != 0 {
		t.Errorf("both unparseable should be equal, got %d", got)
	}
}

func TestIsValidRelease(t *testing.T) {
	if !IsValidRelease("26.05.26") {
		t.Error("26.05.26 should be valid")
	}
	if IsValidRelease("dev") {
		t.Error("dev should be invalid")
	}
	if IsValidRelease("") {
		t.Error("empty should be invalid")
	}
	if IsValidRelease("126.05.26") {
		t.Error("9-char should be invalid")
	}
}
