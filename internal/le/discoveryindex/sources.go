// Design: docs/architecture/core-design.md -- which changed file drifts which index
// Overview: discoveryindex.go -- the generator these rules describe
//
// sources.go answers whether a committed path can make a generated discovery
// index outdated. The changed-file router uses this answer to select the
// freshness gate. The commit gate uses it to require an updated committed
// index.

package discoveryindex

import (
	"slices"
	"strings"
)

// generator is the source file that produces OutputRel. A commit touching it
// can change every byte of the index, so it feeds the index it writes.
const generator = "internal/le/discoveryindex/discoveryindex.go"

// HeaderText answers the first HeaderLines lines of content, which is the
// window packageDoc reads a package header from. A caller that asks IsSource
// about a `.go` file bounds its text with this, so the trigger and the
// generator judge the same bytes.
//
// The bound belongs to the caller rather than to feeds, because a caller can
// supply the working tree, HEAD, or both joined, and only a caller knows which
// trees it read. Handing feeds a whole FILE is what made any Go file that
// merely SPELLS a package header read as a source of the map, this one included
// (plan/journal/gate-fires-outside-its-population.md).
func HeaderText(content string) string {
	end, lines := 0, 0
	for end < len(content) {
		idx := strings.IndexByte(content[end:], '\n')
		if idx < 0 {
			break
		}
		end += idx + 1
		lines++
		if lines == HeaderLines {
			return content[:end]
		}
	}
	return content
}

// hasPackageHeader reports whether text carries a line packageDoc would read a
// package summary from.
//
// It asks packageLine, which is the generator's own test, so a file the map
// derives no text from cannot read as a source of that text. A substring search
// for the marker answered yes to a mention of it in prose, to a constant
// holding it, and to a comment quoting it.
func hasPackageHeader(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if _, ok := packageLine(strings.TrimSpace(line)); ok {
			return true
		}
	}
	return false
}

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

// feeds answers the indexes that committing path can drift, sorted, or nothing
// when path feeds none.
//
// headerText is used only for a non-test `.go` file, and it is the file's
// header rather than its body: the caller bounds it with HeaderText. It is
// searched for the package header line from which the index derives text. The
// caller supplies the working tree, HEAD, or both, because a change can add or
// remove that line and only the caller knows which trees are available.
func feeds(path, headerText string) []string {
	// A committed index feeds only itself: committing it is how its own
	// freshness is satisfied, and it never obliges any OTHER index to ride
	// along.
	if slices.Contains(outputs[:], path) {
		return []string{path}
	}

	// A generator feeds exactly the output it writes. It is judged before the
	// population, because a generator is a source of the map without being one
	// of the files the map describes.
	if path == generator {
		return []string{OutputRel}
	}

	// Every rule below reads a file the walk visits, so a path the walk cannot
	// reach drifts nothing whatever it is named or whatever header it carries.
	if !inPopulation(path) {
		return nil
	}

	// A register.go Description feeds the map, whether or not the file carries
	// a package header.
	if strings.HasSuffix(path, "register.go") {
		return []string{OutputRel}
	}

	if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") &&
		hasPackageHeader(headerText) {
		return []string{OutputRel}
	}
	return nil
}

// IsSource reports whether committing path can change a generated discovery
// index. It is defined in terms of feeds so the "is it a source" and "which
// index" answers can never disagree.
func IsSource(path, headerText string) bool {
	return len(feeds(path, headerText)) > 0
}
