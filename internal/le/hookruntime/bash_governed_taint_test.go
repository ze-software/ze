package hookruntime

import (
	"regexp"
	"strings"
	"testing"
)

// TestGovernedTaintCatchesVariablePaths drives the pretool-bash hook with a
// write whose governed path reaches the writer through a variable, a pipe or a
// cd. The method is the hook's own entry point, so the taint sources, the sinks
// and the escape are judged together.
//
// It exists because the guard saw only a literal plan/ beside the write, and a
// `while read` loop over `find plan` rewrote 44 spec files on 2026-09-08. The
// allowed rows prove a read into scratch, a read-only editor and a read after a
// cd stay free.
func TestGovernedTaintCatchesVariablePaths(t *testing.T) {
	tests := []governedCase{
		{"assigned-variable", `f=plan/x.md; sed -i 's/a/b/' "$f"`, true},
		{"assigned-bare-tree", `d="ai/rules"; sed -i 's/a/b/' "$d/commands.md"`, true},
		{"for-loop", `for f in plan/*.md; do sed -i 's/a/b/' "$f"; done`, true},
		{"for-loop-braced", `for f in plan/*.md; do perl -pi -e 's/a/b/' "${f}"; done`, true},
		{"for-loop-quoted-semicolon", `for f in plan/*.md; do sed -i 's/a/b/;s/c/d/' "$f"; done`, true},
		{"for-loop-redirect", `for f in plan/*.md; do sed 's/a/b/' x > "$f"; done`, true},
		{"for-loop-tee", `for f in plan/*.md; do echo x | tee -a "$f"; done`, true},
		{"for-loop-copy", `for f in plan/*.md; do cp new.md "$f"; done`, true},
		{"while-read-pipe", `find plan -name '*.md' | while read -r f; do sed -i 's/a/b/' "$f"; done`, true},
		{"while-read-git-ls-files", `git ls-files ai/rules | while IFS= read -r f; do sed -i 's/a/b/' "$f"; done`, true},
		{"while-read-process-substitution", `while read -r f; do sed -i 's/a/b/' "$f"; done < <(find plan -name x)`, true},
		{"xargs", `find plan/ -name x | xargs sed -i 's/a/b/'`, true},
		{"find-exec", `find plan -name '*.md' -exec sed -i 's/a/b/' {} +`, true},
		{"command-substitution", `sed -i 's/a/b/' $(find plan -name '*.md')`, true},
		{"cd-in-place", `cd plan && sed -i 's/a/b/' x.md`, true},
		{"cd-redirect", `cd ai/rules && echo x >> commands.md`, true},
		{"for-loop-over-tmp", `for f in tmp/*.md; do sed -i 's/a/b/' "$f"; done`, false},
		{"read-into-scratch", `S=tmp/session/x/scratch; grep x plan/a.md > "$S/out"`, false},
		{"read-only-editor", `f=plan/x.md; sed -n p "$f"`, false},
		{"while-read-reads", `find plan -name '*.md' | while read -r f; do grep -c x "$f"; done`, false},
		{"cd-read", `cd plan && grep x a.md`, false},
		{"cd-read-discarded", `cd plan && grep x a.md > /dev/null 2>&1`, false},
		{"xargs-over-tmp", `find tmp/planning -name x | xargs sed -i 's/a/b/'`, false},
		{"admitted", `ZE_ADMIT_GOVERNED_WRITE="bulk rename reviewed" for f in plan/*.md; do sed -i 's/a/b/' "$f"; done`, false},
	}
	assertGovernedRefusals(t, tests)
}

// TestGovernedLiteralRouteStepsOverQuotedSemicolon drives the pretool-bash hook
// with a literal governed path behind a quoted word that holds a semicolon.
//
// It exists because the literal routes' argument class stopped at every `;`,
// so `sed -i 's/a/b/;s/c/d/' plan/x.md` ended its segment inside the script
// and wrote the spec unrefused. The quoted-path rows prove the widened class
// still reaches a path inside quotes.
func TestGovernedLiteralRouteStepsOverQuotedSemicolon(t *testing.T) {
	assertGovernedRefusals(t, []governedCase{
		{"sed-quoted-semicolon", `sed -i 's/a/b/;s/c/d/' plan/x.md`, true},
		{"perl-quoted-semicolon", `perl -pi -e 's/a/b/;s/c/d/' "ai/rules/commands.md"`, true},
		{"sed-dot-slash-quoted-path", `sed -i 's/a/b/' "./plan/x.md"`, true},
		{"copy-quoted-semicolon", `cp 'a;b.md' plan/x.md`, true},
		{"sed-then-read", `sed -i 's/a/b/' tmp/x; grep y plan/x.md`, false},
		{"sed-read-only-quoted-semicolon", `sed -n 's/a/b/;p' plan/x.md`, false},
	})
}

// TestGovernedSinksDeriveFromLiteralRoutes asserts that every literal route the
// taint sinks derive from ends in the governed tree. governedSink returns a
// route that does not unchanged, which keeps the hook narrow but silently
// blinds that sink, so the drift fails here instead.
func TestGovernedSinksDeriveFromLiteralRoutes(t *testing.T) {
	routes := map[string]*regexp.Regexp{
		"governedSed":      governedSed,
		"governedRedirect": governedRedirect,
		"governedTee":      governedTee,
		"governedCopy":     governedCopy,
	}
	for name, route := range routes {
		if !strings.HasSuffix(route.String(), governedPath.String()) {
			t.Errorf("%s = %q does not end in governedPath %q, so its taint sink matches nothing",
				name, route.String(), governedPath.String())
		}
	}
}

// governedCase is one command and whether the governed-write guard refuses it.
type governedCase struct {
	name    string
	command string
	blocked bool
}

// assertGovernedRefusals runs each case through the pretool-bash entry point
// and checks the refusal and its exit code.
func assertGovernedRefusals(t *testing.T, tests []governedCase) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			code, _, message := runHook(t, root, "pretool-bash", map[string]any{
				"tool_name":  "Bash",
				"tool_input": map[string]any{"command": test.command},
			})
			refused := strings.Contains(message, "shell write to plan/ or ai/rules/")
			if refused != test.blocked {
				t.Fatalf("command %q: refused = %v (code %d), want %v: %s",
					test.command, refused, code, test.blocked, message)
			}
			if test.blocked && code != 2 {
				t.Fatalf("command %q: code = %d, want 2", test.command, code)
			}
		})
	}
}
