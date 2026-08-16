// Design: docs/functional-tests.md -- test/draft/, the incubator for functional tests under development
// Related: record_parse.go -- Discover, the non-recursive glob that already ignores subdirectories

package runner

import (
	"path/filepath"
	"strings"
)

// DraftDirName is the ONE directory under test/ that holds functional tests
// under development: test/draft/<suite>/<name>.ci.
//
// A .ci in there is invisible to every suite and every repo-wide gate, so a
// half-written test cannot redden `make ze-precommit-verify` for the author or for any
// other session sharing the checkout. It is gitignored as well, which is what
// makes the guarantee independent of every gate remembering to skip it: CI
// checks out git, so the tree does not exist there at all.
//
// Suite discovery needs no help -- Discover globs `<dir>/*.ci` and never
// recurses. This constant exists for the gates that DO walk test/ recursively;
// each one skips the directory and TestDraftDirIsInvisibleToRepoGates pins that
// they all still do.
const DraftDirName = "draft"

// SuiteDir resolves the directory a suite's .ci files are discovered from,
// swapping in the draft tree when the caller passed --draft.
//
//	SuiteDir(base, "plugin", false) -> <base>/test/plugin
//	SuiteDir(base, "plugin", true)  -> <base>/test/draft/plugin
//
// One helper rather than a conditional at each of the twelve call sites that
// used to build this path by hand: a suite that forgets the draft branch is a
// suite where --draft silently runs the REAL tests, which is the one outcome
// this whole mechanism exists to prevent.
func SuiteDir(baseDir, suite string, draft bool) string {
	if draft {
		return filepath.Join(baseDir, "test", DraftDirName, suite)
	}
	return filepath.Join(baseDir, "test", suite)
}

// IsDraftPath reports whether path is the draft tree, or anything inside it,
// relative to a walk rooted at testRoot (normally <repo>/test).
//
// Callers walking test/ recursively use it to prune:
//
//	if d.IsDir() && runner.IsDraftPath(testRoot, p) {
//	    return filepath.SkipDir
//	}
//
// Pruning at the DIRECTORY is deliberate: a per-file check re-reads and re-tests
// every draft file for nothing, and misses a draft's non-.ci companions.
func IsDraftPath(testRoot, path string) bool {
	rel, err := filepath.Rel(testRoot, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return false // outside the root entirely; not ours to judge
	}
	first, _, _ := strings.Cut(rel, "/")
	return first == DraftDirName
}
