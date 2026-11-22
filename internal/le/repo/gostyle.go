// Design: docs/contributing/ze-go-style.md -- commit-time Go formatting and newly added style checks
//
// gostyle.go registers a diff-scoped change-set check into `./le commit create`
// so Ze's Go formatting and AST-level style rules hold across every agent
// harness (Claude, Codex, Antigravity, and shell writes), not only Claude
// Write/Edit hooks, without tripping pre-existing legacy debt.

package repo

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

const goStyleCheckName = "repo go-style check"

// goStyleHit is one formatting or newly added Go style defect in a commit.
type goStyleHit struct {
	Path string
	Line int
	Err  *opererr.Error
}

// ze point: go-standards/directives/never-write-these-forbidden-go-patterns
//
// CheckAddedGoStyle refuses commits carrying un-gofmt'd Go files or newly added
// non-test Go lines that introduce forbidden patterns (fmt.Sprintf/Printf,
// strconv.FormatInt/FormatUint, switch args[0], anonymous go func, init()
// outside register/all/init files, or //nolint without a reason).
func CheckAddedGoStyle(change changecheck.Change) (string, error) {
	var hits []goStyleHit
	for _, rel := range change.Paths {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		if strings.Contains(rel, "/testdata/") {
			continue
		}
		after, present, err := change.After(rel)
		if err != nil {
			return "", err
		}
		if !present || isGeneratedGoFile(after) {
			continue
		}
		if fmtHit := checkGoFmt(rel, after); fmtHit != nil {
			hits = append(hits, *fmtHit)
		}
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		var before string
		if change.Before != nil {
			prev, existed, beforeErr := change.Before(rel)
			if beforeErr != nil {
				return "", beforeErr
			}
			if existed {
				before = prev
			}
		}
		hits = append(hits, checkAddedGoAST(rel, before, after)...)
	}
	return formatGoStyleHits(hits), nil
}

func isGeneratedGoFile(source string) bool {
	head := source
	if len(head) > 512 {
		head = head[:512]
	}
	return strings.Contains(head, "// Code generated ") && strings.Contains(head, "DO NOT EDIT")
}

func checkGoFmt(rel, source string) *goStyleHit {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		// Syntax errors are reported by the compiler/typecheck, not as a gofmt issue.
		return nil
	}
	if bytes.Equal(formatted, []byte(source)) {
		return nil
	}
	return &goStyleHit{
		Path: rel,
		Line: 1,
		Err: opererr.New(
			rel+" is not gofmt-formatted",
			"an unformatted Go file breaks scoped lint for every later session touching this package (plan/journal/lint-contract-not-applied.md)",
			"gofmt -w "+rel,
		),
	}
}

func checkAddedGoAST(rel, before, after string) []goStyleHit {
	added := addedLineNumbers(before, after)
	if len(added) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, after, parser.ParseComments)
	if err != nil {
		return nil
	}
	var hits []goStyleHit
	base := path.Base(rel)
	allowInit := base == "init.go" || strings.HasPrefix(base, "register") || strings.HasPrefix(base, "all")
	allowFprintf := strings.HasPrefix(rel, "cmd/") || strings.HasPrefix(rel, "internal/le/hookruntime/")

	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := fset.Position(comment.Pos()).Line
			if !added[line] {
				continue
			}
			if nolintMissingReason(comment.Text) {
				hits = append(hits, newStyleHit(rel, line,
					"newly added //nolint directive carries no reason",
					"a bare //nolint hides a lint check without justifying why it cannot be fixed",
					"fix the lint finding or append // <reason> to the //nolint directive in "+rel,
				))
			}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			sel, ok := typed.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch pkgIdent.Name {
			case "fmt":
				switch sel.Sel.Name {
				case "Sprintf", "Printf":
					if !strings.HasPrefix(rel, "cmd/") {
						hits = append(hits, newStyleHit(rel, line,
							"newly added fmt."+sel.Sel.Name+" call",
							"Ze uses internal/core/textbuf.Buffer for string building and slogutil for logging",
							"replace fmt."+sel.Sel.Name+" with textbuf.Buffer or slogutil in "+rel,
						))
					}
				case "Fprintf":
					if !allowFprintf {
						hits = append(hits, newStyleHit(rel, line,
							"newly added fmt.Fprintf call",
							"Ze writes structured text with internal/core/textbuf.Buffer instead of fmt.Fprintf",
							"replace fmt.Fprintf with textbuf.Buffer in "+rel,
						))
					}
				}
			case "strconv":
				if sel.Sel.Name == "FormatInt" || sel.Sel.Name == "FormatUint" {
					hits = append(hits, newStyleHit(rel, line,
						"newly added strconv."+sel.Sel.Name+" call",
						"Ze formats integers into internal/core/textbuf.Buffer (.Int / .Uint) to avoid heap allocations",
						"replace strconv."+sel.Sel.Name+" with textbuf.Buffer in "+rel,
					))
				}
			}
		case *ast.GoStmt:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if _, isLit := typed.Call.Fun.(*ast.FuncLit); isLit {
				hits = append(hits, newStyleHit(rel, line,
					"newly added anonymous goroutine (go func)",
					"anonymous goroutines hide ownership and shutdown lifecycle (ai/rules/goroutine-lifecycle.md)",
					"extract a named worker/helper method with explicit shutdown in "+rel,
				))
			}
		case *ast.SwitchStmt:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if isSwitchArgsZero(typed.Tag) {
				hits = append(hits, newStyleHit(rel, line,
					"newly added switch args[0] dispatch",
					"Ze dispatches commands through registration tables, never manual switch args[0]",
					"register the command in the package's command/action table instead of switching on args[0] in "+rel,
				))
			}
		case *ast.FuncDecl:
			line := fset.Position(typed.Pos()).Line
			if !added[line] {
				return true
			}
			if typed.Recv == nil && typed.Name.Name == "init" && !allowInit {
				hits = append(hits, newStyleHit(rel, line,
					"newly added init() outside register*.go",
					"Ze keeps package registration in register*.go so startup side effects stay discoverable",
					"move init() into register.go in "+path.Dir(rel),
				))
			}
		}
		return true
	})

	return hits
}

func newStyleHit(rel string, line int, problem, why, fix string) goStyleHit {
	var tb textbuf.Buffer
	fullProblem := tb.Str(rel).Byte(':').Int(int64(line)).Str(": ").Str(problem).String()
	return goStyleHit{
		Path: rel,
		Line: line,
		Err:  opererr.New(fullProblem, why, fix),
	}
}

// addedLineNumbers answers the 1-based line numbers in after whose trimmed text
// appears more times in after than in before, so newly added lines are judged
// without flagging pre-existing legacy lines.
func addedLineNumbers(before, after string) map[int]bool {
	oldCounts := make(map[string]int)
	if before != "" {
		for line := range strings.SplitSeq(before, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				oldCounts[trimmed]++
			}
		}
	}
	added := make(map[int]bool)
	for idx, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if oldCounts[trimmed] > 0 {
			oldCounts[trimmed]--
			continue
		}
		added[idx+1] = true
	}
	return added
}

func nolintMissingReason(comment string) bool {
	trimmed := strings.TrimSpace(comment)
	rest, ok := strings.CutPrefix(trimmed, "//nolint")
	if !ok {
		rest, ok = strings.CutPrefix(trimmed, "// nolint")
		if !ok {
			return false
		}
	}
	// Require a second `//` introducing a non-empty explanation after the linter list.
	idx := strings.Index(rest, "//")
	if idx < 0 {
		return true
	}
	return strings.TrimSpace(rest[idx+2:]) == ""
}

func isSwitchArgsZero(tag ast.Expr) bool {
	idx, ok := tag.(*ast.IndexExpr)
	if !ok {
		return false
	}
	ident, ok := idx.X.(*ast.Ident)
	if !ok || ident.Name != "args" {
		return false
	}
	lit, ok := idx.Index.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}

func formatGoStyleHits(hits []goStyleHit) string {
	if len(hits) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Str("Go style/formatting check refused what this commit adds:\n")
	for _, hit := range hits {
		tb.Str("  ").Str(hit.Err.Error()).Byte('\n')
	}
	tb.Str("See docs/contributing/ze-go-style.md and plan/journal/lint-contract-not-applied.md.\n")
	return tb.String()
}
