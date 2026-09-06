// Design: (none -- new package)

// Package version provides build and VCS version information for ze.
//
// The release version and build date are set via ldflags at build time.
// VCS revision, modification status, Go version, and build settings are
// read from debug.ReadBuildInfo() which Go 1.18+ populates automatically.
//
// Reference: https://michael.stapelberg.ch/posts/2026-04-05-stamp-it-all-programs-must-report-their-version/
package version

import (
	"encoding/json"
	"os"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	mu        sync.RWMutex
	release   = "dev"
	buildDate = "unknown"
)

// Stamp stores the release version and build date (called from main via ldflags).
func Stamp(v, d string) {
	mu.Lock()
	release = v
	buildDate = d
	mu.Unlock()
}

// info holds parsed VCS and build metadata from debug.ReadBuildInfo.
type info struct {
	commit   string // short git commit hash
	modified bool   // working tree was dirty at build time
	vcsTime  string // commit timestamp
	goVer    string // Go version
	cgo      string // CGO_ENABLED value
}

// readInfo extracts build info once.
var readInfo = sync.OnceValue(func() info {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info{goVer: runtime.Version()}
	}
	i := info{goVer: bi.GoVersion}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			i.commit = s.Value
			if len(i.commit) > 12 {
				i.commit = i.commit[:12]
			}
		case "vcs.modified":
			i.modified = s.Value == "true"
		case "vcs.time":
			i.vcsTime = s.Value
		case "CGO_ENABLED":
			i.cgo = s.Value
		}
	}
	return i
})

func Release() string {
	mu.RLock()
	defer mu.RUnlock()
	return release
}

func BuildDate() string {
	mu.RLock()
	defer mu.RUnlock()
	return buildDate
}

// Short returns the single-line version string: "ze 26.04.05 (built 2026-04-05T...)".
func Short() string {
	mu.RLock()
	v, d := release, buildDate
	mu.RUnlock()
	var tb textbuf.Buffer
	return tb.Str("ze ").Str(v).Str(" (built ").Str(d).Byte(')').String()
}

// Extended returns multi-line version details including VCS and build metadata.
func Extended() string {
	mu.RLock()
	v, d := release, buildDate
	mu.RUnlock()
	i := readInfo()

	var b textbuf.Buffer
	b.Str("ze ").Str(v).Str(" (built ").Str(d).Str(")\n")
	if i.commit != "" {
		b.Str("  commit:   ").Str(i.commit)
		if i.modified {
			b.Str(" (modified)")
		}
		b.Byte('\n')
	}
	if i.vcsTime != "" {
		b.Str("  vcs-time: ").Str(i.vcsTime).Byte('\n')
	}
	b.Str("  go:       ").Str(i.goVer).Byte('\n')
	b.Str("  os/arch:  ").Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH)
	if i.cgo != "" {
		cgoLabel := "disabled"
		if i.cgo == "1" {
			cgoLabel = "enabled"
		}
		b.Byte('\n').Str("  cgo:      ").Str(cgoLabel)
	}
	if img, ok := readImageManifest(); ok {
		b.Byte('\n').Str("  image:    ").Str(img.Image)
		if img.Timestamp != "" {
			b.Str(" (built ").Str(img.Timestamp).Byte(')')
		}
		if img.Appliance != "" {
			b.Byte('\n').Str("  appliance:").Byte(' ').Str(img.Appliance)
		}
		if img.SHA256 != "" {
			b.Byte('\n').Str("  img-sha:  ").Str(img.SHA256)
		}
	}
	return b.String()
}

// imageManifestPath is where `ze appliance build` bakes the build manifest into
// the image (/perm/ze/build.json). Overridable in tests.
var imageManifestPath = "/perm/ze/build.json"

// ImageInfo mirrors the appliance build manifest fields that ze reports at
// runtime. It is duplicated here (rather than importing internal/appliance) so
// this leaf package stays dependency-free; JSON tags MUST match
// appliance.BuildManifest.
type ImageInfo struct {
	Appliance string `json:"appliance"`
	Timestamp string `json:"timestamp"`
	ZeVersion string `json:"ze-version"`
	Arch      string `json:"arch"`
	Image     string `json:"image"`
	SHA256    string `json:"image-sha256"`
}

// readImageManifest loads the build manifest baked into the running image, if
// any. Returns ok=false on a dev box (no /perm) or a malformed/empty manifest.
func readImageManifest() (ImageInfo, bool) {
	return ReadManifestFile(imageManifestPath)
}

// ReadManifestFile loads an appliance build manifest from path. Returns
// ok=false when the file is missing, malformed, or carries no identity. Shared
// by the version command (baked /perm manifest) and the install server (the
// build.json next to the image it serves).
func ReadManifestFile(path string) (ImageInfo, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // caller-controlled manifest path
	if err != nil {
		return ImageInfo{}, false
	}
	var m ImageInfo
	if err := json.Unmarshal(data, &m); err != nil {
		return ImageInfo{}, false
	}
	if m.Image == "" && m.Timestamp == "" {
		return ImageInfo{}, false
	}
	return m, true
}

// CompareReleases compares two Ze releases (YY.MM.DD format).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Unparseable releases (e.g. "dev", "") sort as infinitely old (-1 against any valid release).
func CompareReleases(a, b string) int {
	aok := IsValidRelease(a)
	bok := IsValidRelease(b)

	if !aok && !bok {
		return 0
	}
	if !aok {
		return -1
	}
	if !bok {
		return 1
	}

	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// IsNewerRelease returns true if candidate release is newer than base.
// Returns false when candidate is unparseable (unknown is never "newer").
func IsNewerRelease(candidate, base string) bool {
	return CompareReleases(candidate, base) > 0
}

// IsValidRelease returns true if v has the YY.MM.DD format (8 chars, dots at positions 2 and 5).
func IsValidRelease(v string) bool {
	if len(v) != 8 {
		return false
	}
	return v[2] == '.' && v[5] == '.'
}

// HTTPHeader returns a compact version string for the X-Ze-Version HTTP header.
// Format: "ze/26.04.05 (ac8f5391; go1.26; darwin/arm64)".
func HTTPHeader() string {
	mu.RLock()
	v := release
	mu.RUnlock()
	i := readInfo()

	var tb textbuf.Buffer
	tb.Str("ze/").Str(v).Str(" (")
	if i.commit != "" {
		tb.Str(i.commit)
		if i.modified {
			tb.Byte('+')
		}
		tb.Str("; ")
	}
	tb.Str(i.goVer).Str("; ").Str(runtime.GOOS).Byte('/').Str(runtime.GOARCH).Byte(')')
	return tb.String()
}

// EnvKeyHideVersion is the Ze env key an operator writes as
// `environment { hide-version true; }` in the config file. The config loader
// plumbs the YANG leaf into this key (internal/component/config/apply_env.go,
// envPlumbingTable), and it names this constant rather than repeating the
// spelling, so the import that carries the name also carries the registration
// below.
//
// The registration lives here, beside the only reader, as
// internal/core/privilege registers ze.user beside its own reader. env.Get and
// env.Set both end the process on an unregistered key, so a package that reads
// a key and leaves its registration to another component fails in every build
// that links one and not the other.
const EnvKeyHideVersion = "ze.hide-version"

// Env var registration for the HTTP version banner.
var _ = env.MustRegister(env.EnvEntry{
	Key:         EnvKeyHideVersion,
	Type:        "bool",
	Default:     "false",
	Description: "Hide the X-Ze-Version banner on every HTTP response",
})

// HTTPHeaderHidden reports whether the operator asked Ze to keep the build
// banner off its HTTP responses. Every writer of X-Ze-Version MUST consult it
// before the write, so one setting covers the web interface and the looking
// glass together.
//
// HTTPHeader stays the banner producer and answers the same string either way:
// an empty banner would be a value a caller cannot tell from a build that
// carries no version.
//
// The default is false, so a configuration that says nothing keeps the header
// exactly as it is today and the hardening is opt-in.
//
// The value is read on each response rather than cached. Both callers are
// control-plane HTTP surfaces, where one registry lookup costs less than a
// second copy of the setting that can disagree with the registry.
func HTTPHeaderHidden() bool {
	return env.GetBool(EnvKeyHideVersion, false)
}
