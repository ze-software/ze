package version

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// TestBuildIdentityFields checks BuildIdentity returns structured provenance
// including stamped release metadata, Go runtime fields, and optional appliance
// image manifest data.
//
// VALIDATES: AC-4 -- BuildIdentity() exposes release, build-date, go, os, arch,
// and optional image manifest fields as a structured Identity value.
// PREVENTS: show system triage falling back to parsing multi-line version text
// or omitting stamped build and appliance provenance.
func TestBuildIdentityFields(t *testing.T) {
	prevRelease, prevDate := Release(), BuildDate()
	t.Cleanup(func() { Stamp(prevRelease, prevDate) })

	prevManifest := imageManifestPath
	t.Cleanup(func() { imageManifestPath = prevManifest })

	const (
		wantRelease   = "26.11.27"
		wantBuildDate = "2026-11-27T05:00:00Z"
	)
	Stamp(wantRelease, wantBuildDate)

	manifestDir := t.TempDir()
	manifestPath := filepath.Join(manifestDir, "build.json")
	manifestJSON := `{"appliance":"ze-router","timestamp":"2026-11-27T04:55:00Z","ze-version":"26.11.27","arch":"amd64","image":"ze-26.11.27.img","image-sha256":"deadbeef"}`
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	t.Run("with image manifest", func(t *testing.T) {
		imageManifestPath = manifestPath

		got := BuildIdentity()
		if got.Release != wantRelease {
			t.Errorf("Release = %q, want %q", got.Release, wantRelease)
		}
		if got.BuildDate != wantBuildDate {
			t.Errorf("BuildDate = %q, want %q", got.BuildDate, wantBuildDate)
		}
		if !strings.HasPrefix(got.Go, "go") {
			t.Errorf("Go = %q, want prefix %q", got.Go, "go")
		}
		if got.OS != runtime.GOOS {
			t.Errorf("OS = %q, want %q", got.OS, runtime.GOOS)
		}
		if got.Arch != runtime.GOARCH {
			t.Errorf("Arch = %q, want %q", got.Arch, runtime.GOARCH)
		}
		if got.CGO != "" && got.CGO != "enabled" && got.CGO != "disabled" {
			t.Errorf("CGO = %q, want empty, enabled, or disabled", got.CGO)
		}
		if got.Image == nil {
			t.Fatalf("Image = nil, want manifest from %s", manifestPath)
		}
		if got.Image.Image != "ze-26.11.27.img" {
			t.Errorf("Image.Image = %q, want %q", got.Image.Image, "ze-26.11.27.img")
		}
		if got.Image.Appliance != "ze-router" {
			t.Errorf("Image.Appliance = %q, want %q", got.Image.Appliance, "ze-router")
		}
		if got.Image.Timestamp != "2026-11-27T04:55:00Z" {
			t.Errorf("Image.Timestamp = %q, want %q", got.Image.Timestamp, "2026-11-27T04:55:00Z")
		}
		if got.Image.ZeVersion != wantRelease {
			t.Errorf("Image.ZeVersion = %q, want %q", got.Image.ZeVersion, wantRelease)
		}
		if got.Image.Arch != "amd64" {
			t.Errorf("Image.Arch = %q, want %q", got.Image.Arch, "amd64")
		}
		if got.Image.SHA256 != "deadbeef" {
			t.Errorf("Image.SHA256 = %q, want %q", got.Image.SHA256, "deadbeef")
		}

		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal Identity: %v", err)
		}
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("unmarshal Identity: %v", err)
		}
		if wire["build-date"] != wantBuildDate {
			t.Errorf("JSON build-date = %v, want %q", wire["build-date"], wantBuildDate)
		}
		if _, ok := wire["image"]; !ok {
			t.Errorf("JSON missing image field when manifest present: %s", raw)
		}
	})

	t.Run("without image manifest", func(t *testing.T) {
		imageManifestPath = filepath.Join(manifestDir, "missing.json")

		withoutImage := BuildIdentity()
		if withoutImage.Release != wantRelease {
			t.Errorf("Release = %q, want %q", withoutImage.Release, wantRelease)
		}
		if withoutImage.Image != nil {
			t.Errorf("Image = %+v when manifest missing, want nil", withoutImage.Image)
		}

		raw, err := json.Marshal(withoutImage)
		if err != nil {
			t.Fatalf("marshal Identity without image: %v", err)
		}
		var wire map[string]any
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatalf("unmarshal Identity without image: %v", err)
		}
		if _, ok := wire["image"]; ok {
			t.Errorf("JSON included image field when manifest absent: %s", raw)
		}
	})
}
