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
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
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

// Short returns the single-line version string: "ze 26.04.05 (built 2026-04-05T...)".
func Short() string {
	mu.RLock()
	v, d := release, buildDate
	mu.RUnlock()
	return fmt.Sprintf("ze %s (built %s)", v, d)
}

// Extended returns multi-line version details including VCS and build metadata.
func Extended() string {
	mu.RLock()
	v, d := release, buildDate
	mu.RUnlock()
	i := readInfo()

	var b strings.Builder
	fmt.Fprintf(&b, "ze %s (built %s)\n", v, d)
	if i.commit != "" {
		commitLine := i.commit
		if i.modified {
			commitLine += " (modified)"
		}
		fmt.Fprintf(&b, "  commit:   %s\n", commitLine)
	}
	if i.vcsTime != "" {
		fmt.Fprintf(&b, "  vcs-time: %s\n", i.vcsTime)
	}
	fmt.Fprintf(&b, "  go:       %s\n", i.goVer)
	fmt.Fprintf(&b, "  os/arch:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if i.cgo != "" {
		cgoLabel := "disabled"
		if i.cgo == "1" {
			cgoLabel = "enabled"
		}
		fmt.Fprintf(&b, "  cgo:      %s\n", cgoLabel)
	}
	return strings.TrimRight(b.String(), "\n")
}

// HTTPHeader returns a compact version string for the X-Ze-Version HTTP header.
// Format: "ze/26.04.05 (ac8f5391; go1.25; darwin/arm64)".
func HTTPHeader() string {
	mu.RLock()
	v := release
	mu.RUnlock()
	i := readInfo()

	parts := []string{i.goVer, runtime.GOOS + "/" + runtime.GOARCH}
	if i.commit != "" {
		commit := i.commit
		if i.modified {
			commit += "+"
		}
		parts = append([]string{commit}, parts...)
	}
	return fmt.Sprintf("ze/%s (%s)", v, strings.Join(parts, "; "))
}
