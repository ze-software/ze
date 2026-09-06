// Design: docs/architecture/testing/verify-freshness-scope.md -- failure attribution
// Related: ../engine/artifacts.go -- DeclaredGroups, which reads what Declare writes
// Related: ../../commit/verification.go -- structuralGateReds, which charges the red
//
// Package failuregroup lets a verify stage say WHICH files its red is about.
//
// A stage that declares no usable paths is UNATTRIBUTABLE, and an unattributable
// structural red is charged to every commit in the checkout rather than to the
// one that caused it. So a stage whose output already names files owes those
// names to the gate, and two stages needed the same scanner to give them.
//
// The declaration goes into what the action RETURNS. The engine reads a stage's
// groups back out of the stage log, and that log holds the action's answer
// (../dispatch/dispatch.go, dispatch, which hands leroot.Run a capturing
// writer). A line written to the process's own stderr reaches the operator's
// terminal and never the log.
package failuregroup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// diagnosticRE matches the position prefix a Go toolchain and golangci-lint both
// print, `path:line:col:` or `path:line:`, at the start of a line. The path is
// relative to the checkout, which is what a group's related paths must be.
var diagnosticRE = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_][\w./-]*\.go):\d+:(?:\d+:)?`)

// MaxPaths bounds one run's answer. A tree-wide failure can name thousands of
// findings across a few hundred files, and a group line that carries all of them
// serves nobody.
const MaxPaths = 500

// Paths answers the distinct Go files a run's output named, sorted, so two runs
// over one tree produce one artifact.
func Paths(text string) []string {
	seen := make(map[string]struct{})
	for _, match := range diagnosticRE.FindAllStringSubmatch(text, -1) {
		if len(seen) >= MaxPaths {
			break
		}
		seen[match[1]] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	slices.Sort(out)

	return out
}

// Merge folds another run's paths into a set already collected, keeping the
// answer sorted, distinct, and bounded.
func Merge(into, more []string) []string {
	for _, path := range more {
		if len(into) >= MaxPaths {
			break
		}
		if !slices.Contains(into, path) {
			into = append(into, path)
		}
	}
	slices.Sort(into)

	return into
}

// Declare writes the group the verify engine reads back, followed by the count
// that closes the list. The engine accepts a declaration only when the count and
// the group total agree.
//
// A run that named no file still declares a group, with no paths and a kind the
// commit side does not read paths from. That is the honest answer for a failure
// the scanner could not place, and it leaves the red charged to everyone, which
// is the behavior that already stood rather than a new one.
func Declare(w io.Writer, id, kind, summary, rerun string, paths []string) error {
	related := paths
	if len(related) == 0 {
		kind, related = "generic", []string{}
	}
	line, err := json.Marshal(map[string]any{
		"group-id": id, "kind": kind, "related": related,
		"summary": summary, "rerun": rerun,
	})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "VERIFY FAILURE GROUP: %s\n", line); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "VERIFY FAILURE GROUPS COMPLETE: 1\n")

	return err
}

// pathKinds names the group kinds whose Related list holds checkout paths. A
// kind outside this set relates its failure to something else -- a rule id, a
// subcheck name -- so reading a path out of it would attribute the red to a
// file the stage never mentioned.
var pathKinds = map[string]bool{"files": true, "lint": true, "package": true}

// CarriesPaths reports whether a group of this kind names checkout paths in its
// Related list.
func CarriesPaths(kind string) bool { return pathKinds[kind] }

// CleanPath answers the checkout-relative path one Related value names, and
// whether it names one at all. A value that escapes the checkout, or that names
// nothing present under root, is refused rather than normalized into a
// neighboring path.
//
// The two affixes it removes are what a producer writes: `./` prefixes a Go
// package path and `/...` closes a package pattern.
func CleanPath(root, related string) (string, bool) {
	related = strings.TrimSpace(related)
	related = strings.TrimSuffix(related, "/...")
	related = strings.TrimPrefix(related, "./")
	if related == "" || related == "." {
		return "", false
	}
	if strings.HasPrefix(related, "/") {
		return "", false
	}
	for component := range strings.SplitSeq(related, "/") {
		if component == "" || component == "." || component == ".." {
			return "", false
		}
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(related))); err != nil {
		return "", false
	}

	return related, true
}

// Covers reports whether related, a path one failure group named, falls inside
// the paths an asker is responsible for.
//
// The containment runs both ways on purpose. A red about a file inside a
// directory the asker named is the asker's, and so is a red about a directory
// that holds a file the asker named: in each case the asker's change is inside
// the subject the stage judged.
func Covers(related string, asked []string) bool {
	for _, path := range asked {
		if related == path {
			return true
		}
		if strings.HasPrefix(related, path+"/") {
			return true
		}
		if strings.HasPrefix(path, related+"/") {
			return true
		}
	}

	return false
}
