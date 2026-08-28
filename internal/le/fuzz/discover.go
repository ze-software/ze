// Design: docs/architecture/core-design.md -- Go fuzzing, discovered at run time
//
// Package fuzz discovers every `func Fuzz` under internal/ when the fuzzers
// run, so there is no enumeration to commit and nothing to go stale.
//
// Discovery reads the tree at run time. Adding a `func Fuzz` therefore adds the
// target without a generated fragment or a second stale-data check.
//
// Two constraints on the emitted command are Go fuzz REQUIREMENTS rather than
// preferences, and both are why the generator existed at all:
//
//   - The package path is an exact single directory, never a wildcard. A tree
//     with sibling packages (isis/{packet,yang}) makes a wildcard fail with
//     "matches more than one package".
//   - The name is anchored. Without it a target whose name is a prefix of
//     another (FuzzParseVPN against FuzzParseVPNAddPath) fails with "matches
//     more than one fuzz target".
//
// The native job action owns admission on shared machines. Fuzz discovery
// itself stays a pure inventory operation and does not re-enter another command
// dispatcher.

package fuzz

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// The budget kept from the historic hand-list: a short stage suitable for
// scheduled CI, with a hard per-target timeout above it.
const (
	// DefaultFuzzTime is what each target of a sweep gets.
	DefaultFuzzTime = "10s"
	// NamedFuzzTime is what a single named target gets, which is longer
	// because a caller who named one is looking at it.
	NamedFuzzTime = "30s"
	// DefaultTimeout is the hard per-target ceiling above the fuzz budget.
	DefaultTimeout = "60s"
)

// DefaultPackage is where a named run looks when the caller names a target and
// no package. Go resolves it directly.
const DefaultPackage = "./internal/..."

// DefaultName is what a named run fuzzes when the caller named a package and no
// target. It is Go's own unanchored default, and it matches every target in
// that package.
const DefaultName = "Fuzz"

// SkipDirs names the directories that never hold fuzz targets of ours. A
// vendored target is not ours to run, and a testdata one is a fixture.
var SkipDirs = map[string]bool{
	"vendor":       true,
	"tmp":          true,
	"testdata":     true,
	"node_modules": true,
	".git":         true,
}

// funcFuzz implements Go's naming rule for fuzz targets.
// It accepts `func Fuzz` and `func FuzzXxx` when Xxx starts with an uppercase letter.
// `func Fuzzy` is an ordinary function, but the deleted generator incorrectly matched it.
// No such function exists in the tree, so this port corrects a latent fault.
//
// The explicit whitespace class excludes newlines.
// A `func Fuzz` followed by a line break is not a declaration that this walk accepts.
var funcFuzz = regexp.MustCompile(`(?m)^func (Fuzz(?:[A-Z][A-Za-z0-9_]*)?)[\t ]*\(`)

// Target is one fuzz entry point, and the exact package directory holding it.
type Target struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}

// Command answers the `go test` invocation for this target: anchored, and over
// one package directory.
func (t Target) Command(chain gotoolchain.Toolchain, fuzztime, timeout string) []string {
	var tb textbuf.Buffer
	name := tb.Str("-fuzz=^").Str(t.Name).Byte('$').String()
	tb.Reset()
	budget := tb.Str("-fuzztime=").Str(fuzztime).String()
	tb.Reset()
	ceiling := tb.Str("-timeout=").Str(timeout).String()

	return chain.GoTest(gotoolchain.TestOptions{}, name, budget, ceiling, t.Package)
}

// Discover answers every `func Fuzz` under internal/, sorted by package then
// name.
//
// Sorting makes runs reproducible and keeps targets from one package together.
// An interrupted run is therefore easier to read.
// A tree without internal/ returns no targets, and the caller converts that result into a verdict.
//
// An unreadable path returns an error instead of omitting a target.
// The Python skipped unreadable directories but crashed on unreadable files.
// Neither behavior let the caller distinguish a partial list from a complete list.
func Discover(root string) ([]Target, error) {
	base := filepath.Join(root, "internal")
	// A tree without internal/ returns no targets.
	// The reason for its absence does not change that answer, so the stat error is discarded.
	// The caller converts an empty list into a more specific verdict.
	if !isDirectory(base) {
		return nil, nil
	}

	var found []Target
	seen := make(map[Target]bool)

	walkErr := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if SkipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		raw, readErr := os.ReadFile(path) //nolint:gosec // a build tool reads the checkout it was pointed at
		if readErr != nil {
			return readErr
		}
		pkg := packagePath(root, filepath.Dir(path))
		for _, match := range funcFuzz.FindAllStringSubmatch(string(raw), -1) {
			target := Target{Name: match[1], Package: pkg}
			if seen[target] {
				continue
			}
			seen[target] = true
			found = append(found, target)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].Package != found[j].Package {
			return found[i].Package < found[j].Package
		}
		return found[i].Name < found[j].Name
	})
	return found, nil
}

// isDirectory reports whether path is a directory that can be read.
func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// packagePath renders one directory as the `./internal/...`-style path go test
// takes, relative to the checkout.
func packagePath(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return dir
	}
	var tb textbuf.Buffer
	return tb.Str("./").Str(filepath.ToSlash(rel)).String()
}
