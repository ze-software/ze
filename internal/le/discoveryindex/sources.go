// Design: docs/architecture/core-design.md -- which written file drifts the package map
// Overview: discoveryindex.go -- the generator these rules describe
//
// sources.go answers whether writing a path can make ai/PACKAGE-MAP.md
// outdated. The invalidation hook asks it after every Write and Edit, and
// removes the map when the answer is yes.
//
// It answered a COMMITTED population until 2026-09-11, for a changed-file
// router and a commit-time gate that compared the map against its tracked copy.
// Both are deleted: the map is derived on demand, so there is no tracked copy
// for a comparison to judge, and the only question left is which write drifts
// it. `IsSource` and `HeaderText` served those two callers and went with them.

package discoveryindex

import (
	"slices"
	"strings"
)

// outputs names every generated index these rules cover.
//
// ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md are absent by design because they
// are no longer tracked. This file answers whether a commit must refresh a
// COMMITTED index. Git cannot supply an untracked file in a materialized commit
// view. The working tree still generates both files on demand.
var outputs = [...]string{OutputRel}

// inPopulation reports whether the walk Build performs can reach path.
//
// It reads the same roots and skipDirs that walk uses, so the trigger predicate
// and the generator answer one population. Deriving it a second time is what
// let a `go mod vendor` result refuse to commit: every vendored Go file
// carrying a package header read as a source of a map that skips vendor
// outright, and each dependency bump had to spend stale-index-ok on a premise
// the generator contradicts.
//
// A path is a slash-separated repository path. Its last segment is the file, so
// only the segments before it are judged as directories, the way scanDir judges
// the entries it descends into.
func inPopulation(path string) bool {
	segments := strings.Split(path, "/")
	if len(segments) < 2 || !slices.Contains(roots[:], segments[0]) {
		return false
	}
	for _, name := range segments[1 : len(segments)-1] {
		if skipDirs[name] || strings.HasPrefix(name, ".") {
			return false
		}
	}
	return true
}

// IsSourcePath reports whether writing path can change ai/PACKAGE-MAP.md.
//
// The header is NOT consulted, and that is the decision this function makes.
// It is asked AFTER the write, so the edit that most obviously drifts the map,
// deleting a `// Package` comment, leaves a file with no header to read: a
// predicate that required one would answer false exactly then, and the map
// would keep a summary the tree no longer supports while recordPackage now
// renders the register.go description or TODO.
//
// The two failure modes are not symmetric, which is what decides the trade. An
// over-wide answer costs ONE rebuild, on a read that was going to happen
// anyway. An under-wide answer is a stale index nothing announces. Precision is
// an optimization here; correctness is not.
func IsSourcePath(path string) bool {
	// A derived output feeds itself: a hand edit of it is not evidence about
	// the tree, so it is discarded rather than kept.
	if slices.Contains(outputs[:], path) {
		return true
	}
	if !inPopulation(path) {
		return false
	}
	// The generator, internal/le/discoveryindex/discoveryindex.go, needs no
	// clause of its own: it is a non-test `.go` file inside the population, so
	// the line below already answers for it.
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
