// Design: docs/contributing/testing.md -- every Go build flavor lint must cover
// Detail: verifylint.go -- scope derivation and direct golangci-lint execution
//
// Package verifylint owns the native full-tree lint stage. The matrix below is the
// Go form of the builds formerly selected by internal/le/verify/lint/matrix.go.
package verifylint

import "sort"

const (
	area        = "verify lint"
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

// The names that repeat across the matrix rows and the runner. They are declared
// once because a typo in a flavor name is silent: the pass runs, selects a
// different file set, and reports zero issues over it.
const (
	passHost             = "host"
	passLinuxIntegration = "linux-integration"
	goosLinux            = "linux"
	// goosHost is what a pass with no explicit GOOS prints as its target. It
	// holds the same text as passHost and means something else: one names a
	// pass, the other names a build target, so they are two constants.
	goosHost   = "host"
	tagZeCore  = "ze_core"
	tagZeSetup = "ze_setup"
)

func basePasses() []Flavor {
	return []Flavor{
		{
			Name: passHost,
			Why:  "the host build selected by the current golangci configuration",
		},
		{
			Name: passLinuxIntegration,
			GOOS: goosLinux,
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
			Name: "dragonfly", GOOS: "dragonfly", GOARCH: "amd64",
			Why: "the generic Unix fallback outside the explicitly supported BSD targets; amd64 is pinned because it is the only architecture DragonFly builds for, so an arm64 host would select no files and lint nothing",
		},
		{
			Name: "wasip1", GOOS: "wasip1", GOARCH: "wasm",
			Why: "the !unix fallbacks through a target whose whole import graph type-checks",
		},
		{
			Name: "linux-amd64", GOOS: goosLinux, GOARCH: "amd64",
			Why: "the amd64 filename-selected netlink implementations, which the base Linux pass covers only on an amd64 host",
		},
		{
			Name: "linux-arm64", GOOS: goosLinux, GOARCH: "arm64",
			Why: "the arm64 filename-selected netlink implementations shipped by the appliance",
		},
		{
			Name: "linux-other-arch", GOOS: goosLinux, GOARCH: "riscv64",
			Why: "the linux && !amd64 && !arm64 netlink fallback",
		},
		{
			Name: "capability", GOOS: goosLinux,
			Tags: []string{
				"debug", "race", "live", "stress", "maprib", "fleetperf", "zetest",
				"gokrazy", "ze_test", "ze_perf", "ze_analyze", "ze_chaos", "ze_le",
				"integration", "ze_docvalid_fixture",
			},
			Why: "every additive capability tag that is not a mutually exclusive personality",
		},
		{
			Name: "distro", GOOS: goosLinux, Tags: []string{"ze_distro"},
			Why: "the distro daemon build",
		},
		{
			Name: "appliance", GOOS: goosLinux, Tags: []string{"ze_appliance"},
			Why: "the daemon packed into the appliance image",
		},
		{
			Name: "setup", GOOS: goosLinux, Tags: []string{tagZeSetup},
			Why: "the appliance setup build driver",
		},
		{
			Name: "personalities", GOOS: goosLinux,
			Tags: []string{"ze_distro", "ze_appliance", tagZeSetup},
			Why:  "files that assert the behavior of combined personality tags",
		},
		{
			Name: "installer", GOOS: goosLinux,
			Tags: []string{"ze_installer", "ze_installer_fault"},
			Why:  "the installer initrd and its fault-injection files",
		},
		{
			Name: "installer-nofault", GOOS: goosLinux, Tags: []string{"ze_installer"},
			Why: "the installer files selected when fault injection is off",
		},
		{
			Name: "tinygo", GOOS: goosLinux, Tags: []string{"tinygo"},
			Why: "the TinyGo pprof stub",
		},
		{
			Name: "setup-standalone", GOOS: goosLinux, Tags: []string{tagZeSetup},
			Without: []string{tagZeCore},
			Why:     "the standalone ze_setup && !ze_core program",
		},
		{
			Name: "compile-out", GOOS: goosLinux, Without: append([]string(nil), featureTags...),
			Why: "every !ze_<feature> stub selected with ze_core and no feature gate",
		},
	}
}

// ReachableProjectTags returns every project tag selected by the native lint
// action population. Lint analyzes test files, so this is the build-tag
// universe the test-sensitivity check uses.
func ReachableProjectTags(featureTags []string) []string {
	seen := map[string]bool{tagZeCore: true}
	for _, tag := range featureTags {
		seen[tag] = true
	}
	for _, flavor := range flavorMatrix(featureTags) {
		for _, tag := range flavor.Tags {
			if len(tag) > 3 && tag[:3] == "ze_" {
				seen[tag] = true
			}
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}
