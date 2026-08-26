// Design: docs/architecture/testing/tracked-build-gate.md -- which flavors the commit owes
//
// matrix.go declares the build flavors this gate compiles, and nothing else.
// The table is the only part of the gate that is a judgement about which
// combinations matter, so it is stated apart from the machinery that runs it.

package trackedbuild

// Flavor is one build flavor of the module. Each is built over `./...`, not
// over its own main package alone: measured on 2026-08-04, `./...` costs about
// 2 seconds more than `./cmd/ze` and type-checks every package the tag set
// selects, including the ones no binary imports.
type Flavor struct {
	Name string `json:"name"`
	// Tags are the literal build tags, minus the feature tags. Features says
	// whether the expansion of the feature manifest is appended, as the
	// Makefile does.
	Tags     []string `json:"tags"`
	Features bool     `json:"features"`
	// GOOS pins the target operating system when the flavor's own code is
	// OS-gated. Empty means the host. This is load-bearing rather than
	// tidiness: `go build ./...` SKIPS a package whose build constraints
	// exclude every file, so a Linux-only flavor built on macOS compiles
	// nothing and reports success. See Anchor, which is what actually refuses
	// that outcome.
	GOOS string `json:"goos,omitempty"`
	// Anchor is the package this flavor exists to compile.
	Anchor string `json:"anchor"`
	// AnchorFiles are files of the Anchor package that this flavor's OWN tags
	// select. Every one MUST appear in the package's GoFiles, or the flavor
	// compiled the wrong thing and the run fails.
	//
	// The package alone is not enough, and that gap is what a review caught:
	// cmd/ze/main.go carries no build constraint, so `go list ./cmd/ze`
	// resolves under ANY tag set, `-tags ze_bogus` included. A mistyped or
	// dropped tag left five of six flavors green while compiling none of the
	// dispatch code their tags exist to select. Naming a tag-gated FILE is what
	// ties the result back to the tags.
	AnchorFiles []string `json:"anchor-files"`
	Why         string   `json:"why"`
}

// Matrix is the whole set of flavors one run compiles.
type Matrix []Flavor

// The names the matrix repeats: the two tags every product build carries, the
// package five of the six flavors anchor on, and the dispatch file three of
// them select.
const (
	coreTag      = "ze_core"
	distroTag    = "ze_distro"
	zeCommand    = "./cmd/ze"
	coreDispatch = "ze_core_dispatch.go"
)

// buildMatrix is a REPRESENTATIVE set, not the full matrix the Makefile can
// build, and the choice is a cost decision: about 45 seconds warm, against a
// 25-minute pre-commit gate.
//
// Included: every flavor whose failure stops a person or the test suite from
// working -- the daemon, the functional runner, the appliance image, the two
// setup binaries, and the installer initrd.
//
// Deliberately NOT included: `ze_chaos ze_bgp`, `ze_perf ze_bgp`, `ze_analyze`
// and `ze_core ze_ssh` (bin/ze-stripped). The first three are developer tools
// whose dispatch imports nothing the daemon flavor misses; the fourth differs
// from `distro` only through `!ze_*` negations. A row costs about 3 seconds --
// add one rather than widening an existing row, if this class appears there.
var buildMatrix = Matrix{
	{
		Name:        "distro",
		Tags:        []string{coreTag, distroTag},
		Features:    true,
		Anchor:      zeCommand,
		AnchorFiles: []string{coreDispatch, "setup_features_distro.go"},
		Why:         "bin/ze, the daemon `make ze-build` builds. All four 2026-08-04 breaks were here",
	},
	{
		Name:        "test-runner",
		Tags:        []string{"ze_test"},
		Features:    true,
		Anchor:      zeCommand,
		AnchorFiles: []string{"ze_test_register.go"},
		Why:         "bin/ze-test: a break here disables the whole functional suite",
	},
	{
		Name:        "appliance",
		Tags:        []string{coreTag, "ze_appliance"},
		Features:    true,
		Anchor:      zeCommand,
		AnchorFiles: []string{coreDispatch, "setup_features_appliance.go"},
		Why:         "the binary gokrazy packs into the appliance image",
	},
	{
		Name:        "setup",
		Tags:        []string{"ze_setup"},
		Anchor:      zeCommand,
		AnchorFiles: []string{"setup_dispatch.go", "setup_features_setup.go"},
		Why:         "bin/ze-setup, the Makefile's ze-setup-build target: a disjoint cmd/ze dispatch",
	},
	{
		Name:   "host",
		Tags:   []string{coreTag, "ze_setup"},
		Anchor: zeCommand,
		// setup_dispatch.go is `ze_setup && !ze_core`, so this flavor takes the
		// core dispatch instead. That difference is the reason the row exists.
		AnchorFiles: []string{coreDispatch, "setup_features_setup.go"},
		Why:         "ze-host, the `ze appliance ...` build driver (mk/build-gokrazy.mk)",
	},
	{
		Name:        "installer",
		Tags:        []string{"ze_installer"},
		GOOS:        "linux",
		Anchor:      "./cmd/ze-installer",
		AnchorFiles: []string{"main.go"},
		Why:         "cmd/ze-installer, the installer initrd's PID 1 (linux-only)",
	},
}

// BuildMatrix answers the flavors this gate compiles, in table order. The
// caller receives the table itself: it is read, never edited.
func BuildMatrix() Matrix { return buildMatrix }

// buildPackages is the pattern every flavor is built over.
const buildPackages = "./..."

// whyOf answers why a named flavor exists, which is what the failure page
// prints beside its compiler output.
func (m Matrix) whyOf(name string) string {
	for _, flavor := range m {
		if flavor.Name == name {
			return flavor.Why
		}
	}
	return ""
}
