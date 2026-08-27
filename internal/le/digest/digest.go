// Design: docs/contributing/documentation-testing.md -- the digest anchor check
//
// Package digest validates the `file:line` anchors in ai/digests/*.md against
// the tree those digests describe.
//
// The subsystem flow digests are hand-maintained rather than generated, so
// their anchors rot silently when code moves. Every backtick token shaped like
// `path.go:123` (or `path.go:123-140`, or a bare `path.go`) must resolve to a
// real file, and a line number must be inside that file.
//
// Anchors are written subsystem-relative, so each digest declares its
// subtree(s) in a machine-readable header:
//
//	<!-- digest-base: internal/component/bgp internal/core/bgp -->
//
// A partial anchor is resolved by searching those subtrees for a file whose
// path ends with it, and a unique match is required. A repo-relative anchor
// (one whose first segment is a top-level directory) resolves against the tree
// root instead. An ambiguous anchor fails closed.
//
// Detail: report.go -- what the check answers
package digest

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/pyfmt"
)

// DigestDir is where the hand-maintained subsystem digests live, relative to
// the tree.
var DigestDir = [...]string{"ai", "digests"}

// SkipFiles are files in the digest directory that are not digests.
var SkipFiles = map[string]bool{"README.md": true}

// SkipWalk are directory names the base index never descends into. A dot-prefixed
// directory is skipped too, which is why .git appears here only for symmetry
// with the script's own list.
var SkipWalk = map[string]bool{
	"vendor":       true,
	"tmp":          true,
	"testdata":     true,
	"node_modules": true,
	".git":         true,
}

// TopDirs are the tree's top-level directories. An anchor whose first segment
// is one of them is already repo-relative and resolves against the tree root
// rather than against a digest base.
//
// It is exported for the migration's constant comparison. That comparison
// tests it by value against the script's TOP_DIRS. An output comparison cannot
// detect a missing entry because only an unwritten anchor changes.
var TopDirs = map[string]bool{
	"internal": true,
	"pkg":      true,
	"cmd":      true,
	"scripts":  true,
	"ai":       true,
	"plan":     true,
	"docs":     true,
	"rfc":      true,
	"mk":       true,
	"test":     true,
	"hooks":    true,
	"yang":     true,
	".claude":  true,
	".codex":   true,
	".agents":  true,
}

// The three patterns match the script character for character. Thus, the
// migration compares each pattern with the Python module's compiled pattern.
//
// AnchorPattern's \w is ASCII here and Unicode in Python. The patterns differ
// only for a backtick path token with a non-ASCII character. The corpus has no
// such path token. Its 7 non-ASCII backtick tokens contain a space, arrow, or
// parenthesis, so both patterns reject them
// (TestNoDigestTokenNeedsAUnicodeWordCharacter).
const (
	// BasePattern reads a digest's declared subtrees out of its header.
	BasePattern = `<!--\s*digest-base:\s*(.+?)\s*-->`
	// BacktickPattern is every backtick token, which is the population the
	// anchor pattern then filters.
	BacktickPattern = "`([^`]+)`"
	// AnchorPattern is a code anchor with a code-like extension. Its optional
	// line specification is one line, a range, or a comma-separated list.
	AnchorPattern = `^(?P<path>[A-Za-z0-9_][\w./-]*\.(?:go|py|md|yang|sh|mk|ya?ml|json|txt|proto|c|h|tmpl|html))(?::(?P<lines>\d+(?:-\d+)?(?:,\d+(?:-\d+)?)*))?$`
)

var (
	baseRe     = regexp.MustCompile(BasePattern)
	backtickRe = regexp.MustCompile(BacktickPattern)
	anchorRe   = regexp.MustCompile(AnchorPattern)
	// baseSepRe splits a header's subtree list the way the script's re.split
	// did: on any run of commas and whitespace.
	baseSepRe = regexp.MustCompile(`[,\s]+`)
)

// ErrNoDigests says the tree holds no digest to check.
//
// The script answers "checked 0 anchors across 0 digests, all resolve" and
// exits 0. Thus, a renamed or removed ai/digests looks like a tree whose every
// anchor resolves. The port refuses this state. The script keeps the defect
// until the swap. TestScriptDigestStillPassesATreeWithNoDigests pins the defect
// so this line reddens when somebody repairs the Python.
var ErrNoDigests = errors.New("no digest found under ai/digests, so nothing was checked")

// Anchor is one anchor a digest wrote, parsed.
type anchor struct {
	// path is the anchor's path token, as written.
	path string
	// start is the first line the anchor names, and hasStart says whether it
	// named one at all.
	start    int
	hasStart bool
	// end is the last line of a range, and hasEnd says whether the anchor is a
	// range rather than a single line.
	end    int
	hasEnd bool
}

// text renders the anchor the way the report names it.
//
// The script builds this string from TRUTHINESS but checks the range with
// "is not None". Thus, `file.go:0` renders as the bare path, and `file.go:5-0`
// renders without its end. This port reproduces both forms. The rules disagree
// only for zero, and the check below still sees that zero
// (TestAZeroLineIsNamedTheWayTheScriptNamesIt).
func (a anchor) text() string {
	if !a.hasStart || a.start == 0 {
		return a.path
	}
	var tb textbuf.Buffer
	tb.Str(a.path).Byte(':').Int(int64(a.start))
	if a.hasEnd && a.end != 0 {
		tb.Byte('-').Int(int64(a.end))
	}
	return tb.String()
}

// DigestFiles answers every digest under the tree, in name order.
func DigestFiles(tree string) ([]string, error) {
	dir := filepath.Join(tree, filepath.Join(DigestDir[:]...))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || SkipFiles[entry.Name()] {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

// ParseBases answers the subtrees a digest declares, deduplicated and in
// declaration order.
func ParseBases(text string) []string {
	var bases []string
	seen := make(map[string]bool)
	for _, match := range baseRe.FindAllStringSubmatch(text, -1) {
		for _, token := range baseSepRe.Split(strings.TrimSpace(match[1]), -1) {
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			bases = append(bases, token)
		}
	}
	return bases
}

// AnchorsIn answers every backtick token of text that looks like a code
// anchor. A comma list expands to one anchor per element, so every cited line
// is checked instead of the whole token being dropped.
func AnchorsIn(text string) []anchor {
	var out []anchor
	for _, match := range backtickRe.FindAllStringSubmatch(text, -1) {
		fields := anchorRe.FindStringSubmatch(strings.TrimSpace(match[1]))
		if fields == nil {
			continue
		}
		anchorPath, lines := fields[1], fields[2]
		if lines == "" {
			out = append(out, anchor{path: anchorPath})
			continue
		}
		for piece := range strings.SplitSeq(lines, ",") {
			first, last, isRange := strings.Cut(piece, "-")
			// The pattern already proved every piece is digits, optionally
			// split by one dash, so a parse that fails here cannot come from
			// the corpus. Dropping the piece rather than guessing a number
			// keeps the anchor out of the resolved count instead of inventing
			// a line for it.
			start, err := strconv.Atoi(first)
			if err != nil {
				continue
			}
			if !isRange {
				out = append(out, anchor{path: anchorPath, start: start, hasStart: true})
				continue
			}
			end, err := strconv.Atoi(last)
			if err != nil {
				continue
			}
			out = append(out, anchor{path: anchorPath, start: start, hasStart: true, end: end, hasEnd: true})
		}
	}
	return out
}

// IsRepoRelative reports whether an anchor path starts at the tree root.
func IsRepoRelative(anchorPath string) bool {
	head, _, _ := strings.Cut(anchorPath, "/")
	return TopDirs[head]
}

// baseIndex answers every file under one digest base, tree-relative and in
// lexical order.
//
// The script walks with os.walk and keeps the filesystem's own order, so its
// ambiguity list is ordered by whatever the directory hands back. This one is
// ordered, which is the same set and a message a reader can compare between two
// runs.
func baseIndex(tree, base string) ([]string, error) {
	dir := filepath.Join(tree, base)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil //nolint:nilerr // a base that is not a directory is reported by the caller as its own finding
	}

	var paths []string
	err = filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name == dir {
				return nil
			}
			if SkipWalk[entry.Name()] || strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(tree, name)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// matchesInBase answers which files under one base an anchor path can name.
// A base-plus-path exact hit is unambiguous. Otherwise, an anchor with a slash
// matches by suffix, and an anchor without one matches by base name.
func matchesInBase(index []string, base, anchorPath string) []string {
	direct := path.Join(base, anchorPath)
	if slices.Contains(index, direct) {
		return []string{direct}
	}

	var tb textbuf.Buffer
	tail := tb.Byte('/').Str(anchorPath).String()
	hasSlash := strings.Contains(anchorPath, "/")
	basename := anchorPath
	if hasSlash {
		basename = anchorPath[strings.LastIndex(anchorPath, "/")+1:]
	}

	var hits []string
	for _, rel := range index {
		if hasSlash {
			if strings.HasSuffix(rel, tail) {
				hits = append(hits, rel)
			}
			continue
		}
		if rel[strings.LastIndex(rel, "/")+1:] == basename {
			hits = append(hits, rel)
		}
	}
	return hits
}

// resolver holds the per-base index for one run, so a tree is walked once per
// base however many anchors name it.
type resolver struct {
	tree  string
	index map[string][]string
}

func newResolver(tree string) *resolver {
	return &resolver{tree: tree, index: make(map[string][]string)}
}

func (r *resolver) forBase(base string) ([]string, error) {
	if index, ok := r.index[base]; ok {
		return index, nil
	}
	index, err := baseIndex(r.tree, base)
	if err != nil {
		return nil, err
	}
	r.index[base] = index
	return index, nil
}

// resolve answers which tree-relative files an anchor path can name.
//
// A repo-relative anchor short-circuits. Otherwise, the match must be unique
// across every declared base. Deduplication lets overlapping bases reach one
// file. A bare name under two bases is ambiguous. The check fails closed
// instead of selecting the first base and checking the wrong same-named file.
func (r *resolver) resolve(bases []string, anchorPath string) ([]string, error) {
	if IsRepoRelative(anchorPath) {
		return []string{anchorPath}, nil
	}

	var hits []string
	seen := make(map[string]bool)
	for _, base := range bases {
		index, err := r.forBase(base)
		if err != nil {
			return nil, err
		}
		for _, hit := range matchesInBase(index, base, anchorPath) {
			if seen[hit] {
				continue
			}
			seen[hit] = true
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

// lineCount answers how many lines a file holds. As in the script, a trailing
// chunk without a newline is a line, and an empty file has no lines.
func lineCount(tree, rel string) int {
	raw, err := os.ReadFile(filepath.Join(tree, rel)) //nolint:gosec // a file of the tree the caller named
	if err != nil {
		// -1 rather than 0: an unreadable file is not an empty file. Otherwise,
		// every anchor becomes out of range, and the read failure looks like a
		// rotted anchor.
		return -1
	}
	if len(raw) == 0 {
		return 0
	}
	count := strings.Count(string(raw), "\n")
	if raw[len(raw)-1] != '\n' {
		count++
	}
	return count
}

// isFile reports whether a tree-relative path names a regular file.
func isFile(tree, rel string) bool {
	info, err := os.Stat(filepath.Join(tree, rel))
	return err == nil && info.Mode().IsRegular()
}

// checkDigest answers the problems and the resolutions of one digest.
func checkDigest(tree, name string, res *resolver) ([]Problem, []Resolution, error) {
	relDigest := path.Join(path.Join(DigestDir[:]...), name)
	raw, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(relDigest))) //nolint:gosec // a digest of the tree the caller named
	if err != nil {
		return nil, nil, err
	}
	text := string(raw)

	bases := ParseBases(text)
	anchors := AnchorsIn(text)
	var problems []Problem
	var resolved []Resolution

	// A base header is required only for the strict case: a subsystem-relative
	// anchor carrying a line number. A bare mention with no line is informal.
	needsBase := false
	for _, item := range anchors {
		if item.hasStart && !IsRepoRelative(item.path) {
			needsBase = true
			break
		}
	}
	if needsBase && len(bases) == 0 {
		problems = append(problems, Problem{
			Digest: relDigest,
			Anchor: "(header)",
			Detail: "no `<!-- digest-base: <subtree> -->` header, so subsystem-relative `file:line` anchors cannot be resolved",
		})
		return problems, resolved, nil
	}

	for _, base := range bases {
		info, statErr := os.Stat(filepath.Join(tree, base))
		if statErr == nil && info.IsDir() {
			continue
		}
		var tb textbuf.Buffer
		named := tb.Str("digest-base: ").Str(base).String()
		tb.Reset()
		detail := tb.Str("declared base subtree `").Str(base).Str("` does not exist").String()
		problems = append(problems, Problem{Digest: relDigest, Anchor: named, Detail: detail})
	}

	for _, item := range anchors {
		named := item.text()
		hits, resolveErr := res.resolve(bases, item.path)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}

		if !item.hasStart {
			problem, resolution := checkBareAnchor(tree, relDigest, named, item, hits)
			if problem != nil {
				problems = append(problems, *problem)
			}
			if resolution != nil {
				resolved = append(resolved, *resolution)
			}
			continue
		}

		problem, resolution := checkLineAnchor(tree, relDigest, named, item, bases, hits)
		if problem != nil {
			problems = append(problems, *problem)
			continue
		}
		resolved = append(resolved, *resolution)
	}
	return problems, resolved, nil
}

// checkBareAnchor judges an anchor with no line number.
//
// A repo-relative path is a cross-reference and must exist. A bare name is
// informal shorthand. The check records a unique resolution but never reports
// it as a finding. Thus, prose can mention `register.go`, and the gate does not
// fail.
func checkBareAnchor(tree, relDigest, named string, item anchor, hits []string) (*Problem, *Resolution) {
	if IsRepoRelative(item.path) {
		if len(hits) > 0 && isFile(tree, hits[0]) {
			return nil, &Resolution{Digest: relDigest, Anchor: named, File: hits[0]}
		}
		return &Problem{Digest: relDigest, Anchor: named, Detail: "linked file does not exist"}, nil
	}
	if len(hits) == 1 && isFile(tree, hits[0]) {
		return nil, &Resolution{Digest: relDigest, Anchor: named, File: hits[0]}
	}
	return nil, nil
}

// checkLineAnchor judges an anchor that names a line, which is the strict case.
func checkLineAnchor(tree, relDigest, named string, item anchor, bases, hits []string) (*Problem, *Resolution) {
	var tb textbuf.Buffer
	switch {
	case len(hits) == 0:
		where := bases
		if len(where) == 0 {
			where = []string{"repo root"}
		}
		return &Problem{
			Digest: relDigest, Anchor: named,
			Detail: tb.Str("file not found under ").Str(pyfmt.List(where)).String(),
		}, nil
	case len(hits) > 1:
		return &Problem{
			Digest: relDigest, Anchor: named,
			Detail: tb.Str("ambiguous -- matches ").Join(hits, ", ").Str("; qualify the path").String(),
		}, nil
	}

	rel := hits[0]
	if !isFile(tree, rel) {
		return &Problem{
			Digest: relDigest, Anchor: named,
			Detail: tb.Str("resolved to `").Str(rel).Str("` which does not exist").String(),
		}, nil
	}

	count := lineCount(tree, rel)
	highest := item.start
	if item.hasEnd {
		highest = item.end
		if item.end < item.start {
			return &Problem{
				Digest: relDigest, Anchor: named,
				Detail: tb.Str("reversed line range ").Int(int64(item.start)).Byte('-').Int(int64(item.end)).String(),
			}, nil
		}
	}
	if item.start < 1 || highest > count {
		return &Problem{
			Digest: relDigest, Anchor: named,
			Detail: tb.Str("line ").Int(int64(highest)).Str(" out of range (`").Str(rel).
				Str("` has ").Int(int64(count)).Str(" lines)").String(),
		}, nil
	}
	return nil, &Resolution{Digest: relDigest, Anchor: named, File: rel}
}

// Check validates every digest of the tree.
func Check(tree string) (Report, error) {
	names, err := DigestFiles(tree)
	if err != nil {
		return Report{}, err
	}
	if len(names) == 0 {
		return Report{}, ErrNoDigests
	}

	res := newResolver(tree)
	report := Report{Digests: len(names), Errors: []Problem{}, Resolved: []Resolution{}}
	for _, name := range names {
		problems, resolved, checkErr := checkDigest(tree, name, res)
		if checkErr != nil {
			return Report{}, checkErr
		}
		report.Errors = append(report.Errors, problems...)
		report.Resolved = append(report.Resolved, resolved...)
	}
	report.Anchors = len(report.Errors) + len(report.Resolved)
	return report, nil
}
