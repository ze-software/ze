package version

import (
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
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

// TestHTTPHeaderHidden checks the toggle every HTTP surface consults before it
// writes X-Ze-Version. The default answer is false, so an operator who says
// nothing keeps the banner.
//
// VALIDATES: the ze.hide-version key is registered in this package and reads
// as a boolean; the default preserves today's behavior.
// PREVENTS: a key typo that leaves the toggle unreachable, and a default that
// hides the banner nobody asked to hide.
func TestHTTPHeaderHidden(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset keeps the banner", value: "", want: false},
		{name: "false keeps the banner", value: "false", want: false},
		{name: "true hides the banner", value: "true", want: true},
		{name: "enabled hides the banner", value: "enabled", want: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(EnvKeyHideVersion, c.value)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			if got := HTTPHeaderHidden(); got != c.want {
				t.Errorf("HTTPHeaderHidden() with %q = %v, want %v", c.value, got, c.want)
			}
		})
	}
}
