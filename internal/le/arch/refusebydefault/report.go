// Design: docs/contributing/ze-go-style.md -- the refuse-by-default check's answer
// Overview: refusebydefault.go -- the shapes each finding names
//
// report.go judges the hits of each file against the change set and the
// annotations, and holds what `le arch refuse-by-default` ANSWERS.

package archrefusebydefault

import (
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// Finding is one shape the check refuses, or one it accepts under an
// annotation, and it is one ROW of the check's answer.
type Finding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Fn     string `json:"fn"`
	Rule   Rule   `json:"rule"`
	Detail string `json:"detail"`
	// Guard is the guard an accepted shape's annotation names.
	Guard string `json:"guard,omitempty"`
}

// Findings is a row set.
type Findings []Finding

// CheckReport is the whole answer of one run: what was judged, the shapes it
// refuses, and the annotated shapes it accepts. The accepted ones are listed
// so a reviewer reads every guard a change names (spec R-3: an annotation must
// not become a reflex).
type CheckReport struct {
	// Scope says what was judged, in words.
	Scope string `json:"scope"`
	// Base is the commit the change set was diffed against, empty for a
	// report over the tree.
	Base      repochanged.LineBase `json:"base"`
	Files     int                  `json:"files"`
	Findings  Findings             `json:"findings"`
	Annotated Findings             `json:"annotated"`
}

// exitCode is 1 on a finding and 0 on none. An annotated shape is accepted,
// so it does not count.
func (r CheckReport) exitCode() int {
	if len(r.Findings) > 0 {
		return 1
	}
	return 0
}

// judgedFile is one file's verdicts: the shapes refused and those accepted.
type judgedFile struct {
	refused  Findings
	accepted Findings
}

// judgeFile scans one file and sorts its hits into refused and accepted.
// keep decides which lines count: every line for a report, the changed lines
// for a check. A malformed annotation on a counted line is refused on its own,
// because an escape that names no guard is the reflex the annotation exists
// to prevent.
func judgeFile(fset *token.FileSet, tree, path string, keep func(Hit) bool) (judgedFile, error) {
	hits, found, err := ScanFile(fset, filepath.Join(tree, filepath.FromSlash(path)))
	if err != nil {
		return judgedFile{}, err
	}
	byLine := make(map[int]Annotation, len(found))
	answer := judgedFile{}
	for _, annotation := range found {
		byLine[annotation.Line] = annotation
		if annotation.Named() {
			continue
		}
		hit := Hit{Rule: RuleAnnotationNoGuard, Line: annotation.Line, Anchors: []span{{first: annotation.Line, last: annotation.Line}}}
		if !keep(hit) {
			continue
		}
		answer.refused = append(answer.refused, Finding{File: path, Line: annotation.Line, Fn: "-", Rule: RuleAnnotationNoGuard, Detail: annotation.Problem})
	}
	for _, hit := range hits {
		if !keep(hit) {
			continue
		}
		finding := Finding{File: path, Line: hit.Line, Fn: hit.Fn, Rule: hit.Rule, Detail: hit.Detail}
		annotation, annotated := annotationFor(hit, byLine)
		if annotated {
			if annotation.Named() {
				finding.Guard = annotation.Guard
				answer.accepted = append(answer.accepted, finding)
				continue
			}
		}
		answer.refused = append(answer.refused, finding)
	}
	return answer, nil
}

// Check answers the shapes whose anchor lines the change set touches, over
// every judged Go file it names under tree.
//
// A shape is due when the change touches any line that makes it: a default
// arm's header or its return, a branch's condition or its return, the
// comparison itself. An edit elsewhere in an old function does not make the
// function's old shapes due, so the class stops growing without making the
// tree's existing instances any session's debt (`report` lists those).
func Check(tree string, changed repochanged.ChangedLines) (CheckReport, error) {
	paths := make([]string, 0, len(changed))
	for path := range changed {
		if !judged(path) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	set := token.NewFileSet()
	report := CheckReport{Scope: "changed lines", Files: len(paths), Findings: Findings{}, Annotated: Findings{}}
	for _, path := range paths {
		touches := func(hit Hit) bool {
			for _, anchor := range hit.Anchors {
				if changed.Touches(path, anchor.first, anchor.last) {
					return true
				}
			}
			return false
		}
		verdict, err := judgeFile(set, tree, path, touches)
		if err != nil {
			return CheckReport{}, err
		}
		report.Findings = append(report.Findings, verdict.refused...)
		report.Annotated = append(report.Annotated, verdict.accepted...)
	}
	return report, nil
}

// Report answers every shape in every judged Go file under tree's source
// roots, refused and accepted alike.
func Report(tree string) (CheckReport, error) {
	var paths []string
	walked := 0
	for _, root := range sourceRoots {
		rootPath := filepath.Join(tree, root)
		if _, err := os.Stat(rootPath); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		walked++
		err := filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(tree, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if skippedDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if judged(rel) {
				paths = append(paths, rel)
			}
			return nil
		})
		if err != nil {
			return CheckReport{}, err
		}
	}
	if walked == 0 {
		// A tree with none of the roots is not a tree with no shape: the
		// walk read nothing, and a report of nothing would read as clean.
		return CheckReport{}, fmt.Errorf("%w: %s holds none of %s", ErrNoSourceRoot, tree, strings.Join(sourceRoots, ", "))
	}
	slices.Sort(paths)

	set := token.NewFileSet()
	everyLine := func(Hit) bool { return true }
	report := CheckReport{Scope: "every judged Go file under " + strings.Join(sourceRoots, ", "), Files: len(paths), Findings: Findings{}, Annotated: Findings{}}
	for _, path := range paths {
		verdict, err := judgeFile(set, tree, path, everyLine)
		if err != nil {
			return CheckReport{}, err
		}
		report.Findings = append(report.Findings, verdict.refused...)
		report.Annotated = append(report.Annotated, verdict.accepted...)
	}
	return report, nil
}

// ErrNoSourceRoot names a tree the report could not walk, because it holds
// none of the source roots. The fix is to run the report from a Ze checkout.
var ErrNoSourceRoot = errors.New("no source root to report on")

// sourceRoots are the directories holding Ze's own Go. Everything else in a
// checkout (vendor, gokrazy's module cache, tmp) is somebody else's code or
// nobody's.
var sourceRoots = []string{"cmd", "internal", "pkg"}

// judged reports whether a changed path is shipped Go source under a source
// root. Tests, vendored modules, fixtures and dot directories are not: a test
// table answers what its case says, and the other three are not Ze's code.
func judged(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	if strings.HasSuffix(path, "_test.go") {
		return false
	}
	underRoot := false
	for _, root := range sourceRoots {
		if strings.HasPrefix(path, root+"/") {
			underRoot = true
		}
	}
	if !underRoot {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if skippedDirectory(segment) {
			return false
		}
	}
	return true
}

// skippedDirectory reports whether a directory name holds no judged source.
func skippedDirectory(name string) bool {
	if name == "vendor" {
		return true
	}
	if name == "testdata" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

// Text renders the scope line, one line per refused shape, the accepted
// shapes with their guards, and the fix. It ends in a newline.
func (r CheckReport) Text() string {
	var tb textbuf.Buffer
	tb.Str("refuse-by-default scope: ").Str(r.Scope)
	if r.Base.Commit != "" {
		tb.Str(", the working tree against ").Str(r.Base.Commit)
		if r.Base.Reason != "" {
			tb.Str(" (").Str(r.Base.Reason).Byte(')')
		}
	}
	tb.Str(", ").Int(int64(len(r.Findings))).Str(" refused, ").Int(int64(len(r.Annotated))).Str(" annotated, in ").Int(int64(r.Files)).Str(" Go file(s)\n")
	if len(r.Annotated) > 0 {
		tb.Str("refuse-by-default: ").Int(int64(len(r.Annotated))).Str(" shape(s) accepted under an annotation, each naming its guard:\n")
		for _, finding := range r.Annotated {
			writeFinding(&tb, finding)
			tb.Str("      guard: ").Str(finding.Guard).Byte('\n')
		}
	}
	if len(r.Findings) == 0 {
		return tb.Str("refuse-by-default: OK\n").String()
	}
	tb.Str("refuse-by-default: ").Int(int64(len(r.Findings))).Str(" shape(s) answer a plausible value where the code has no answer:\n")
	for _, finding := range r.Findings {
		writeFinding(&tb, finding)
	}
	tb.Byte('\n')
	tb.Str("A caller cannot tell that value from a real one. Return an error, or the protocol's discard,\n")
	tb.Str("naming what could not be found, read or recognized. ADD-PATH presence follows the negotiated\n")
	tb.Str("capability the route carries, never the identifier's value. A health answer reads the state.\n")
	tb.Str("Where the zero IS the answer another branch relies on, name that guard on the line above:\n")
	tb.Str("  // refuse-by-default: guards <Symbol> -- <why>\n")
	tb.Str("See docs/contributing/ze-go-style.md, \"A zero value is never an answer\".\n")
	return tb.String()
}

// writeFinding renders one finding's line.
func writeFinding(tb *textbuf.Buffer, finding Finding) {
	tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).
		Str(" (").Str(finding.Fn).Str(") ").Str(string(finding.Rule)).Str(": ").Str(finding.Detail).Byte('\n')
}
