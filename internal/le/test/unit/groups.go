// Design: docs/architecture/core-design.md -- component-group unit actions.
// Detail: actions.go -- command dispatch and subprocess execution.
//
// Package testunit runs the Go unit tests three ways: six named component-group
// subsets, `pkg` over the packages a line names, and `all` over the whole
// checkout.
//
// A named group is the targeted subset a developer runs while working inside
// it. `pkg` is the scoped test of the packages being developed (pkg.go). The
// `all` verb runs every package of the checkout, then each group whose own
// build tags compile its test files out of that run.
package testunit

import (
	"runtime"
	"strings"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// Area is the root command that owns the six groups.
const Area = "test unit"

// Group is one component-group Go test. Pattern is its package population.
type Group struct {
	Verb    string
	Pattern string
	Summary string
	Why     string

	// Tags are build tags the group needs beyond the feature manifest's set.
	// A group that names any takes the bare core tag set as its base, because a
	// tag outside the manifest belongs to a personality rather than a feature.
	Tags []string

	// GOOS is the platform the group's files are written for. Empty means the
	// host, which is every group whose files carry no platform constraint.
	GOOS string

	// Race turns the detector on. Every host group runs under it; a
	// cross-targeted group cannot, because the detector needs cgo.
	Race bool
}

var groups = [...]Group{
	{
		Verb:    "bgp",
		Pattern: "./internal/component/bgp/...",
		Summary: "test the BGP component group",
		Why:     "the BGP component group: reactor, fsm, wire, message, attribute (~1:30)",
		Race:    true,
	},
	{
		Verb:    "core",
		Pattern: "./internal/core/...",
		Summary: "test the core leaf libraries",
		Why:     "the core leaf libraries every tier above depends on (~30s)",
		Race:    true,
	},
	{
		Verb:    "plugins",
		Pattern: "./internal/plugins/...",
		Summary: "test the system plugins",
		Why:     "the system plugins: DHCP, NTP, static, firewall, the CLI verb providers (~40s)",
		Race:    true,
	},
	{
		Verb:    "config",
		Pattern: "./internal/component/config/...",
		Summary: "test the config pipeline",
		Why:     "the YANG-modeled config pipeline: file, tree, resolve (~20s)",
		Race:    true,
	},
	{
		Verb:    "cli",
		Pattern: "./internal/component/cli/...",
		Summary: "test the CLI group",
		Why:     "the CLI: modes, completion, diff, commit, dashboard (~10s)",
		Race:    true,
	},
	{
		// The installer initrd is built with ze_installer, so every _test.go
		// guarded by that tag is invisible to every other group: `go test`
		// without it compiles the package with those files excluded and says
		// nothing. Five files sit in that state, the rescue console's
		// fatal-branch policy among them, which is the code that decides
		// whether a failed install opens a shell, opens nothing, or reboots.
		// They compile and pass; nothing was asking them to run.
		Verb:    "installer",
		Pattern: "./internal/install/...",
		Summary: "test the installer initrd logic",
		Why:     "the installer initrd's own logic behind the ze_installer tag: bootstrap, console, fault, rescue, initrd (~10s)",
		Tags:    []string{"ze_installer"},
		GOOS:    "linux",
	},
}

// Table returns the component groups in execution order.
func Table() []Group {
	rows := make([]Group, len(groups))
	copy(rows, groups[:])
	return rows
}

// tree is the population `all` runs first: every package of the checkout. It is
// not a row of the table, because each named group is a subset of it and a
// sweep carrying both would compile those packages twice. Its verb names the
// run in the report and in the `==>` banner rather than being a word a
// developer types, because a targeted subset is what the named verbs are for.
var tree = Group{
	Verb:    "tree",
	Pattern: "./...",
	Why:     "every package of the checkout, race-instrumented, under the feature tag set",
	Race:    true,
}

// allGroups returns the groups the `all` verb runs, in execution order: the
// whole checkout, then every group whose own build tags compile its test files
// out of that run.
//
// The second part is DERIVED from Tags rather than named. A group carrying a
// tag outside the feature manifest joins the sweep by declaring that tag, and a
// group the whole-checkout run already reads is never compiled a second time.
func allGroups() []Group {
	rows := make([]Group, 0, len(groups)+1)
	rows = append(rows, tree)
	// The range is over the array's address: ranging the array itself copies
	// every row before the loop starts.
	for _, group := range &groups {
		if len(group.Tags) == 0 {
			continue
		}
		rows = append(rows, group)
	}
	return rows
}

// placementKind is where a package value lies against the group table, which
// decides whether `pkg` may run it.
type placementKind uint8

const (
	placementUnspecified placementKind = iota
	// placementRuns: a package of no group, a package or sub-tree inside a
	// group that declares no tags, or a sub-tree holding exactly one such group.
	placementRuns
	// placementWholeTree: every package of the checkout.
	placementWholeTree
	// placementGroupPattern: exactly one group's whole pattern, which that
	// group's own action runs.
	placementGroupPattern
	// placementBehindGroupTags: a package under, or a sub-tree holding, a group
	// that declares its own tags. Under the feature tags its tagged tests
	// compile out while its untagged ones run, so a green covers part of it and
	// reads as all of it.
	placementBehindGroupTags
	// placementSpansGroups: a sub-tree holding two or more groups, which is a
	// near-whole-tree run.
	placementSpansGroups
)

// placement is a package value's placementKind and the groups that decided it:
// the one group for a pattern or a tag, every held group for a span.
type placement struct {
	kind   placementKind
	groups []Group
}

// place answers where value lies against rows. module is the checkout's module
// path, so an import-path spelling reduces to the same directory as a ./dir
// spelling.
//
// The value is reduced to a checkout-relative directory, and remembered as a
// sub-tree when it ends in /.... It is under a group when that directory is the
// group pattern's directory or lies below it. It equals the group's pattern when
// the directories are equal and the value is a sub-tree. It holds a group when
// it is a sub-tree and the group's directory lies below it. A value that is not
// a sub-tree names one directory and holds no group.
func place(value, module string, rows []Group) placement {
	dir, subtree := packageDir(value, module)
	if subtree && dir == "" {
		return placement{kind: placementWholeTree}
	}

	var held, pattern []Group
	for _, group := range rows {
		groupDir, _ := packageDir(group.Pattern, "")
		if underDir(dir, groupDir) && len(group.Tags) != 0 {
			return placement{kind: placementBehindGroupTags, groups: []Group{group}}
		}
		if subtree && dir == groupDir {
			pattern = append(pattern, group)
		}
		if subtree && strings.HasPrefix(groupDir, dir+"/") {
			held = append(held, group)
		}
	}
	if len(held) >= 2 {
		return placement{kind: placementSpansGroups, groups: held}
	}
	for _, group := range held {
		if len(group.Tags) != 0 {
			return placement{kind: placementBehindGroupTags, groups: []Group{group}}
		}
	}
	if len(pattern) != 0 {
		return placement{kind: placementGroupPattern, groups: pattern}
	}
	return placement{kind: placementRuns}
}

// packageDir reduces a package value to its checkout-relative directory, ""
// for the module root, and reports whether the value is a /... sub-tree. The
// module prefix of an import path, a leading ./, a trailing / and a trailing
// /... are removed.
func packageDir(value, module string) (string, bool) {
	dir := value
	if module != "" {
		rest, found := strings.CutPrefix(dir, module+"/")
		if found {
			dir = rest
		}
		if dir == module {
			dir = "."
		}
	}
	subtree := false
	if dir == "..." {
		dir, subtree = "", true
	}
	if trimmed, found := strings.CutSuffix(dir, "/..."); found {
		dir, subtree = trimmed, true
	}
	dir = strings.TrimSuffix(strings.TrimPrefix(dir, "./"), "/")
	if dir == "." {
		dir = ""
	}
	return dir, subtree
}

// underDir reports whether dir is groupDir or lies below it.
func underDir(dir, groupDir string) bool {
	if dir == groupDir {
		return true
	}
	return strings.HasPrefix(dir, groupDir+"/")
}

// options answers the tag selection this group's commands carry. A group naming
// its own tags takes the bare core set as the base, which is what the retired
// build spelled as -tags 'ze_core ze_installer'.
func (g Group) options() gotoolchain.TestOptions {
	return gotoolchain.TestOptions{Core: len(g.Tags) != 0, Race: g.Race, Tags: g.Tags}
}

// Argv returns the exact Go command for this group.
//
// A group targeting a platform this host is not runs `go vet` instead of
// `go test`, because `go test` cross-compiles the test binary and then tries to
// exec it, which on darwin fails with "fork/exec .../disk.test: exec format
// error". Vet type-checks the tag-guarded files without running them. The real
// execution happens on Linux, and in the Alpine VM through `le test qemu all`
// when the host is not Linux (ai/rules/platform-linux.md).
func (g Group) Argv(tc gotoolchain.Toolchain) []string {
	if g.crossTargeted() {
		return tc.GoVet(g.options(), g.Pattern)
	}
	return tc.GoTest(g.options(), g.Pattern)
}

// crossTargeted answers whether this group's platform is not the host's.
func (g Group) crossTargeted() bool {
	return g.GOOS != "" && g.GOOS != runtime.GOOS
}

// EnvOptions returns the toolchain environment this group's command needs. The
// race detector requires cgo, and GOMAXPROCS caps package and test
// concurrency. A group naming a platform pins GOOS, so its files are selected
// by that platform's build constraints rather than by the host's.
func (g Group) EnvOptions() gotoolchain.EnvOptions {
	return gotoolchain.EnvOptions{Test: true, CGO: g.Race, Procs: true, GOOS: g.GOOS}
}
