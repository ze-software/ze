// Design: docs/contributing/committing.md -- change-set checks `./le commit create` runs
//
// refusal_advice.go refuses guard and gate refusal messages that advise banned
// commands, retired scripts, or retired in-file escape markers
// (plan/journal/guard-message-teaches-the-violation.md).

package commit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

const refusalAdviceCheckName = "commit refusal-advice check"

// refusalAdviceScopeDirs are the guard and gate packages whose non-test Go
// sources emit agent- or operator-facing refusal guidance.
var refusalAdviceScopeDirs = [...]string{
	"internal/le/hookruntime/",
	"internal/le/commit/",
	"internal/le/rfc/",
}

type bannedAdvicePattern struct {
	needle string
	reason string
	fix    string
}

// bannedRefusalAdvice lists remediation strings recorded in
// plan/journal/guard-message-teaches-the-violation.md that must never reappear
// in guard or gate messages. Each entry is split across a prefix and suffix so
// this table does not trigger itself if scanned as raw text.
var bannedRefusalAdvice = [...]bannedAdvicePattern{
	{
		needle: "Use git " + "merge",
		reason: "worktree agents are banned from git merge (2026-11-12 row)",
		fix:    "advise `./le commit create` on the worktree branch and the owner's `git rebase`",
	},
	{
		needle: "or cherry-" + "pick",
		reason: "worktree agents are banned from git cherry-pick (2026-11-12 row)",
		fix:    "advise `./le commit create` on the worktree branch",
	},
	{
		needle: "tmp/" + "out-",
		reason: "loose files at the tmp/ root are banned (2026-08-10 row)",
		fix:    "advise `./le session scratch ensure`",
	},
	{
		needle: "tmp/" + "<subfolder>",
		reason: "hand-named tmp/ subfolders are banned (2026-08-10 row)",
		fix:    "advise `./le session scratch ensure`",
	},
	{
		needle: "scripts/" + "dev/",
		reason: "scripts/dev/ helpers are retired (2026-08-19 row)",
		fix:    "advise the native `./le` command instead",
	},
	{
		needle: "rfc-test-change-" + "approved:",
		reason: "in-file RFC test-approval markers are retired (2026-08-19 row)",
		fix:    "advise `./le rfc approve unit ... reason ...`",
	},
	{
		needle: "test-" + "relax:",
		reason: "in-file test-relax markers are retired",
		fix:    "advise `./le test weakened approve ...`",
	},
}

// RefusalAdviceHit is one string literal in a guard/gate source that advises a
// banned command, retired script, or retired marker.
type RefusalAdviceHit struct {
	Path   string
	Line   int
	Needle string
	Err    *opererr.Error
}

func init() {
	changecheck.Register(changecheck.Check{
		Name:  refusalAdviceCheckName,
		Judge: CheckRefusalAdvice,
	})
}

// ze point: user-facing-errors/directives/an-error-names-the-cause-and-the-fix
//
// CheckRefusalAdvice is the commit-time change-set check that refuses added or
// modified non-test Go sources under internal/le/{hookruntime,commit,rfc} when
// a string literal advises a banned command, retired script, or retired marker.
func CheckRefusalAdvice(change changecheck.Change) (string, error) {
	var hits []RefusalAdviceHit
	for _, rel := range change.Paths {
		if !inRefusalAdviceScope(rel) {
			continue
		}
		text, present, err := change.After(rel)
		if err != nil {
			return "", err
		}
		if !present {
			continue
		}
		fileHits, err := scanRefusalAdviceSource(rel, text)
		if err != nil {
			// A file that does not parse is a red the build reports, and a red
			// never refuses a commit (ai/META-DECISIONS.md, "A red prevents a
			// release, never a commit"): skip it, and say so.
			writeRefusalAdviceSkip(rel, err)
			continue
		}
		hits = append(hits, fileHits...)
	}
	return formatRefusalAdviceHits(hits), nil
}

// refusalAdviceNotes receives the note naming each carried file the check
// skipped because it does not parse. A test replaces it to read the note.
var refusalAdviceNotes io.Writer = os.Stderr

// writeRefusalAdviceSkip writes the note for a carried file the check could
// not read as Go: which file went unchecked, why, and the command that shows
// the syntax error.
func writeRefusalAdviceSkip(rel string, parseErr error) {
	var problem, fix textbuf.Buffer
	problem.Str(rel).Str(": not checked for refusal advice that names a banned command: it does not parse (").
		Err(parseErr).Byte(')')
	fix.Str("gopls check ").Str(rel)
	note := opererr.New(problem.String(),
		"the commit goes ahead, since a file that does not parse is a red for the build to report, never a reason to refuse a commit",
		fix.String())
	_, _ = io.WriteString(refusalAdviceNotes, note.Error()+"\n") //nolint:errcheck // a note on stderr; the commit does not depend on it
}

// CheckRefusalAdviceTree scans every non-test Go file under the guard and gate
// scope directories in root and returns all banned-advice hits.
func CheckRefusalAdviceTree(root string) ([]RefusalAdviceHit, error) {
	var hits []RefusalAdviceHit
	for _, prefix := range refusalAdviceScopeDirs {
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
		err := filepath.WalkDir(dir, func(at string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if at != dir && (entry.Name() == "testdata" || strings.HasPrefix(entry.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, at)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !inRefusalAdviceScope(rel) {
				return nil
			}
			raw, err := os.ReadFile(at) //nolint:gosec // walking repository guard packages under checkout root
			if err != nil {
				return err
			}
			fileHits, err := scanRefusalAdviceSource(rel, string(raw))
			if err != nil {
				return err
			}
			hits = append(hits, fileHits...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return hits, nil
}

func inRefusalAdviceScope(rel string) bool {
	if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
		return false
	}
	if rel == "internal/le/commit/refusal_advice.go" {
		return false
	}
	for _, prefix := range refusalAdviceScopeDirs {
		if strings.HasPrefix(rel, prefix) {
			return true
		}
	}
	return false
}

func scanRefusalAdviceSource(rel, source string) ([]RefusalAdviceHit, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, source, 0)
	if err != nil {
		return nil, err
	}
	matched := matchedLiterals(file)
	var hits []RefusalAdviceHit
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if matched[lit] {
			return true
		}
		value, unquoteErr := strconv.Unquote(lit.Value)
		if unquoteErr != nil {
			return true
		}
		line := fset.Position(lit.Pos()).Line
		for _, rule := range bannedRefusalAdvice {
			if strings.Contains(value, rule.needle) {
				var tb textbuf.Buffer
				problem := tb.Str(rel).Byte(':').Int(int64(line)).
					Str(": guard refusal literal contains \"").Str(rule.needle).Byte('"').String()
				hits = append(hits, RefusalAdviceHit{
					Path:   rel,
					Line:   line,
					Needle: rule.needle,
					Err:    opererr.New(problem, rule.reason, rule.fix),
				})
			}
		}
		return true
	})
	return hits, nil
}

// textMatchers are the standard library functions whose string arguments are
// patterns compared against text, never text shown to a reader.
var textMatchers = map[string]map[string]bool{
	"strings": {"Contains": true, "HasPrefix": true, "HasSuffix": true, "Index": true, "LastIndex": true, "Cut": true, "CutPrefix": true, "CutSuffix": true, "TrimPrefix": true, "TrimSuffix": true, "EqualFold": true, "Count": true},
	"bytes":   {"Contains": true, "HasPrefix": true, "HasSuffix": true, "Index": true, "LastIndex": true, "Cut": true, "CutPrefix": true, "CutSuffix": true, "TrimPrefix": true, "TrimSuffix": true, "EqualFold": true, "Count": true},
	"regexp":  {"MustCompile": true, "Compile": true, "MatchString": true, "QuoteMeta": true},
}

// matchedLiterals answers the string literals file uses only to recognize
// text: an argument of a textMatchers call, an operand of == or !=, and a
// switch case value. Code that detects a retired marker in order to refuse it
// names the marker in such a position, and no reader is ever shown it from
// there; a literal anywhere else may reach a message and is judged.
func matchedLiterals(file *ast.File) map[*ast.BasicLit]bool {
	matched := make(map[*ast.BasicLit]bool)
	mark := func(expr ast.Expr) {
		if lit, ok := expr.(*ast.BasicLit); ok {
			matched[lit] = true
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			selector, ok := typed.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if !textMatchers[pkg.Name][selector.Sel.Name] {
				return true
			}
			for _, arg := range typed.Args {
				mark(arg)
			}
		case *ast.BinaryExpr:
			// Only the two comparisons recognize text; every other operator
			// leaves its operands to be judged.
			//exhaustive:ignore
			switch typed.Op {
			case token.EQL, token.NEQ:
				mark(typed.X)
				mark(typed.Y)
			}
		case *ast.CaseClause:
			for _, value := range typed.List {
				mark(value)
			}
		}
		return true
	})
	return matched
}

func formatRefusalAdviceHits(hits []RefusalAdviceHit) string {
	if len(hits) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("guard refusal message advises a banned command or retired path:\n")
	for _, hit := range hits {
		tb.Str("  ").Str(hit.Err.Error()).Byte('\n')
	}
	tb.Str("See plan/journal/guard-message-teaches-the-violation.md.\n")
	return tb.String()
}
