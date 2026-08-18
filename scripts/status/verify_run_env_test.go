package main

// The verify runner launches every stage as a child process, and a stage
// inherits this process's environment. That inheritance is load-bearing and
// nothing in the runner says so: it is a property of ONE expression in
// execStage, and deleting it breaks a caller in another repository directory
// with no error message anywhere.
//
// What depends on it: scripts/dev/ze-run.sh admits one heavy job at a time and
// exports ZE_RUN_JOB naming its registry entry, so a wrapped stage of a wrapped
// job runs INSIDE its parent's slot. `make ze-precommit-verify` holds a slot and
// runs `make ze-lint`, which is wrapped too. A stage launched with a replaced
// environment cannot see ZE_RUN_JOB, so it queues for the slot its own parent is
// holding, and nothing ever releases it. The symptom is a verify that hangs for
// as long as anyone lets it, not a red test.
//
// This is the shape plan/journal/invariant-enforced-by-an-absent-call-site.md
// collects: the code that would break is the code nobody wrote. The precedent
// for the fix is TestEveryExecSiteUsesChildEnv in internal/test/runner, which
// parses the package rather than trusting a comment.
//
// The assertion is on the PRODUCING expression, never on the string
// "ZE_RUN_JOB": the wrapper may rename its variable, and export more of them,
// without this test noticing or needing to.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"
)

// osImportNames returns the qualifiers under which a file reaches package os.
// A dot import contributes the empty string, so `Environ()` is recognized too.
func osImportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range file.Imports {
		if imp.Path == nil || imp.Path.Value != `"os"` {
			continue
		}
		switch {
		case imp.Name == nil:
			names["os"] = true
		case imp.Name.Name == ".":
			names[""] = true
		case imp.Name.Name == "_":
		default:
			names[imp.Name.Name] = true
		}
	}
	return names
}

// unparen strips the parentheses `((os.Environ))()` hides a call behind.
func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// containsEnvironCall reports whether the expression calls os.Environ anywhere
// inside it, under any of the qualifiers the file imports os as.
func containsEnvironCall(expr ast.Expr, osNames map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := unparen(call.Fun).(type) {
		case *ast.SelectorExpr:
			pkg, ok := unparen(fun.X).(*ast.Ident)
			if ok && fun.Sel.Name == "Environ" && osNames[pkg.Name] {
				found = true
			}
		case *ast.Ident:
			if fun.Name == "Environ" && osNames[""] {
				found = true
			}
		}
		return !found
	})
	return found
}

// extendsSameField reports whether the right-hand side is `append(x.Env, ...)`
// for the same x.Env being assigned. That extends a value which already
// inherits; it does not replace it.
func extendsSameField(rhs ast.Expr, lhs *ast.SelectorExpr) bool {
	call, ok := unparen(rhs).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	fun, ok := unparen(call.Fun).(*ast.Ident)
	if !ok || fun.Name != "append" {
		return false
	}
	return types.ExprString(unparen(call.Args[0])) == types.ExprString(lhs)
}

// envAssignment is one `something.Env = ...` statement.
type envAssignment struct {
	Pos      token.Position
	Inherits bool
}

// envAssignments returns every assignment to an `.Env` field in the file, each
// marked with whether it preserves the inherited environment.
//
// Fails closed twice. An assignment whose values cannot be matched one-to-one
// with its targets (`cmd.Env, err = f()`) counts as NOT inheriting, because
// this parser cannot see what f returns; and an `.Env` assignment in any shape
// not recognized below counts the same way. A test that cannot judge an
// expression must not pass it.
func envAssignments(fset *token.FileSet, file *ast.File) []envAssignment {
	osNames := osImportNames(file)
	var out []envAssignment
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, target := range as.Lhs {
			sel, ok := unparen(target).(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Env" {
				continue
			}
			found := envAssignment{Pos: fset.Position(as.Pos())}
			if len(as.Rhs) == len(as.Lhs) {
				rhs := as.Rhs[i]
				found.Inherits = containsEnvironCall(rhs, osNames) || extendsSameField(rhs, sel)
			}
			out = append(out, found)
		}
		return true
	})
	return out
}

// TestEveryStageInheritsTheParentEnvironment is the gate.
//
// VALIDATES: no source file in this package hands a child process an
// environment built from scratch.
// PREVENTS: a verify that hangs forever. See the file comment: a stage that
// cannot read ZE_RUN_JOB queues for the slot its own parent holds.
func TestEveryStageInheritsTheParentEnvironment(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	scanned, assignments, inheriting := 0, 0, 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++
		for _, a := range envAssignments(fset, file) {
			assignments++
			if a.Inherits {
				inheriting++
				continue
			}
			t.Errorf("%s:%d replaces a child process's environment instead of extending os.Environ(). "+
				"A stage launched this way cannot read ZE_RUN_JOB, so a wrapped stage "+
				"(`make ze-lint` under `make ze-precommit-verify`) queues for the slot its own "+
				"parent is holding and the run hangs with no failure. Build the environment as "+
				"`append(os.Environ(), ...)`, or leave Env nil and let os/exec inherit it",
				name, a.Pos.Line)
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no source files; this test cannot be gating anything")
	}
	if assignments == 0 {
		t.Fatal("no `.Env` assignment found in this package: the stage launcher changed shape " +
			"and this test would pass whatever it now does")
	}
	if inheriting == 0 {
		t.Fatal("no `.Env` assignment inherits os.Environ(): the runner no longer passes its " +
			"environment to any stage")
	}
}

// TestEnvAssignmentsJudgeEveryShape proves the checker above discriminates.
// Each case is source the parser must classify, including the two shapes the
// real launcher uses and the three ways to lose the environment.
func TestEnvAssignmentsJudgeEveryShape(t *testing.T) {
	for _, tc := range []struct {
		name         string
		src          string
		want         int
		wantInherits int
	}{
		{
			name:         "append to os.Environ, as execStage builds it",
			src:          "package main\nimport (\"os\"; \"os/exec\")\nfunc f(cmd *exec.Cmd) { cmd.Env = append(os.Environ(), \"A=B\") }\n",
			want:         1,
			wantInherits: 1,
		},
		{
			name:         "append to the field itself, as execStage extends it",
			src:          "package main\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd, extra []string) { cmd.Env = append(cmd.Env, extra...) }\n",
			want:         1,
			wantInherits: 1,
		},
		{
			name: "a literal environment replaces the inherited one",
			src:  "package main\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd) { cmd.Env = []string{\"ZE_VERIFY_MODE=1\"} }\n",
			want: 1,
		},
		{
			name: "the stage's own env replaces the inherited one",
			src:  "package main\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd, st struct{ Env []string }) { cmd.Env = st.Env }\n",
			want: 1,
		},
		{
			name: "append to somebody else's field is not an extension",
			src:  "package main\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd, other *exec.Cmd) { cmd.Env = append(other.Env, \"A=B\") }\n",
			want: 1,
		},
		{
			name:         "aliased os import still counts as inheritance",
			src:          "package main\nimport stdos \"os\"\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd) { cmd.Env = append(stdos.Environ(), \"A=B\") }\n",
			want:         1,
			wantInherits: 1,
		},
		{
			name:         "dot-imported os still counts as inheritance",
			src:          "package main\nimport . \"os\"\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd) { cmd.Env = append(Environ(), \"A=B\") }\n",
			want:         1,
			wantInherits: 1,
		},
		{
			name:         "a parenthesized call is still the call",
			src:          "package main\nimport \"os\"\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd) { cmd.Env = append(((os.Environ))(), \"A=B\") }\n",
			want:         1,
			wantInherits: 1,
		},
		{
			name: "a multi-value assignment cannot be judged, so it fails closed",
			src:  "package main\nimport \"os/exec\"\nfunc g() ([]string, error) { return nil, nil }\nfunc f(cmd *exec.Cmd) { var err error; cmd.Env, err = g(); _ = err }\n",
			want: 1,
		},
		{
			name: "a field that is not Env is not this test's business",
			src:  "package main\nimport \"os/exec\"\nfunc f(cmd *exec.Cmd) { cmd.Dir = \"/tmp\" }\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "injected.go", tc.src, 0)
			if err != nil {
				t.Fatalf("parse injected source: %v", err)
			}
			got := envAssignments(fset, file)
			if len(got) != tc.want {
				t.Fatalf("`.Env` assignments = %d, want %d", len(got), tc.want)
			}
			inherits := 0
			for _, a := range got {
				if a.Inherits {
					inherits++
				}
			}
			if inherits != tc.wantInherits {
				t.Fatalf("inheriting assignments = %d, want %d", inherits, tc.wantInherits)
			}
		})
	}
}
