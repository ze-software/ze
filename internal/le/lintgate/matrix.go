// Design: docs/contributing/testing.md -- every Go build flavor lint must cover
// Detail: lintgate.go -- scope derivation and direct golangci-lint execution
//
// Package lintgate owns the native full-tree lint stage. The matrix below is the
// Go form of the builds formerly selected by scripts/dev/lint_flavors.py.
package lintgate

const (
	area        = "verify-lint"
	actionRun   = "run"
	packageRoot = "./..."
)

// Flavor identifies one Go build whose selected files contribute to lint
// coverage. Tags are added to the current .golangci.yml build tags. Without
// removes tags from that configuration through a derived tagless copy.
type Flavor struct {
	Name    string   `json:"name"`
	GOOS    string   `json:"goos,omitempty"`
	GOARCH  string   `json:"goarch,omitempty"`
	Tags    []string `json:"tags-add,omitempty"`
	Without []string `json:"tags-drop,omitempty"`
	Why     string   `json:"why"`
}

func basePasses() []Flavor {
	return []Flavor{
		{
			Name: "host",
			Why:  "the host build selected by the current golangci configuration",
		},
		{
			Name: "linux-integration",
			GOOS: "linux",
			Tags: []string{"integration"},
			Why:  "Linux-only and integration-tagged files outside the host build",
		},
	}
}

// flavorMatrix returns the ordered additional builds. featureTags is every
// configured tag except ze_core, and compile-out drops all of them at once.
func flavorMatrix(featureTags []string) []Flavor {
	return []Flavor{
		{
			Name: "darwin", GOOS: "darwin",
			Why: "every !linux and darwin file that both Linux base passes miss",
		},
		{
			Name: "freebsd", GOOS: "freebsd",
			Why: "the FreeBSD TCP-MD5 socket option and non-Linux fallback files",
		},
		{
			Name: "openbsd", GOOS: "openbsd",
			Why: "the generic non-Linux TCP-MD5 fallback selected on OpenBSD",
		},
		{
			Name: "dragonfly", GOOS: "dragonfly",
			Why: "the generic Unix fallback outside the explicitly supported BSD targets",
		},
		{
			Name: "wasip1", GOOS: "wasip1", GOARCH: "wasm",
			Why: "the !unix fallbacks through a target whose whole import graph type-checks",
		},
		{
			Name: "linux-arm64", GOOS: "linux", GOARCH: "arm64",
			Why: "the arm64 filename-selected netlink implementations shipped by the appliance",
		},
		{
			Name: "linux-other-arch", GOOS: "linux", GOARCH: "riscv64",
			Why: "the linux && !amd64 && !arm64 netlink fallback",
		},
		{
			Name: "capability", GOOS: "linux",
			Tags: []string{
				"debug", "race", "live", "stress", "maprib", "fleetperf", "zetest",
				"gokrazy", "ze_test", "ze_perf", "ze_analyze", "ze_chaos", "ze_le",
				"integration",
			},
			Why: "every additive capability tag that is not a mutually exclusive personality",
		},
		{
			Name: "distro", GOOS: "linux", Tags: []string{"ze_distro"},
			Why: "the distro daemon build",
		},
		{
			Name: "appliance", GOOS: "linux", Tags: []string{"ze_appliance"},
			Why: "the daemon packed into the appliance image",
		},
		{
			Name: "setup", GOOS: "linux", Tags: []string{"ze_setup"},
			Why: "the appliance setup build driver",
		},
		{
			Name: "personalities", GOOS: "linux",
			Tags: []string{"ze_distro", "ze_appliance", "ze_setup"},
			Why:  "files that assert the behavior of combined personality tags",
		},
		{
			Name: "installer", GOOS: "linux",
			Tags: []string{"ze_installer", "ze_installer_fault"},
			Why:  "the installer initrd and its fault-injection files",
		},
		{
			Name: "installer-nofault", GOOS: "linux", Tags: []string{"ze_installer"},
			Why: "the installer files selected when fault injection is off",
		},
		{
			Name: "tinygo", GOOS: "linux", Tags: []string{"tinygo"},
			Why: "the TinyGo pprof stub",
		},
		{
			Name: "setup-standalone", GOOS: "linux", Tags: []string{"ze_setup"},
			Without: []string{"ze_core"},
			Why:     "the standalone ze_setup && !ze_core program",
		},
		{
			Name: "compile-out", GOOS: "linux", Without: append([]string(nil), featureTags...),
			Why: "every !ze_<feature> stub selected with ze_core and no feature gate",
		},
	}
}
