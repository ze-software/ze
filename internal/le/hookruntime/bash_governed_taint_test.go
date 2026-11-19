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

// TestGovernedPathStartsAtTheTree drives the pretool-bash hook with paths that
// merely contain plan/ or ai/rules/ and with paths into the real trees.
//
// It exists because every route matched the tree name anywhere in an argument,
// so an in-place edit of internal/le/ai/rules/digest_test.go, and a commit whose
// quoted body described one, were refused as writes to the rule tree on
// 2026-11-19. Only a literal relative path that starts with an ordinary
// directory name is freed. The blocked rows are the spellings a first, wider
// narrowing admitted (found by independent review): a root from an expansion
// or a variable, a path built by a string join, a pattern-filtered file list,
// a hidden-directory worktree, another checkout's absolute path, a brace and a
// non-normalised path.
func TestGovernedPathStartsAtTheTree(t *testing.T) {
	assertGovernedRefusals(t, []governedCase{
		{"sed-nested-ai-rules", `sed -i '' -e 's/a/b/' internal/le/ai/rules/digest_test.go`, false},
		{"perl-nested-plan", `perl -pi -e 's/a/b/' internal/le/plan/x.go`, false},
		{"quoted-nested-path", `sed -i 's/a/b/' "internal/le/ai/rules/x.go"`, false},
		{"python-nested-path", `python3 -c "open('internal/le/ai/rules/x.go','w').write('x')"`, false},
		{"commit-body-prose", `./le commit create subject x body "an in-place sed -i of internal/le/ai/rules/x.go" file y`, false},
		{"assigned-nested", `f=internal/le/plan/x.go; sed -i 's/a/b/' "$f"`, false},
		{"dot-dot-plan", `sed -i 's/a/b/' ../plan/x.md`, true},
		{"dot-slash-ai-rules", `sed -i 's/a/b/' ./ai/rules/x.md`, true},
		{"python-tree-path", `python3 -c "open('plan/x.md','w').write('x')"`, true},
		{"project-dir-expansion", `sed -i 's/a/b/' "$CLAUDE_PROJECT_DIR/plan/x.md"`, true},
		{"pwd-expansion", `sed -i 's/a/b/' $PWD/ai/rules/commands.md`, true},
		{"toplevel-substitution", `perl -pi -e 's/a/b/' "$(git rev-parse --show-toplevel)/plan/x.md"`, true},
		{"tilde-path", `sed -i 's/a/b/' ~/Code/ze/main/plan/x.md`, true},
		{"assigned-root", `R=/srv/ze/main; sed -i 's/a/b/' "$R/plan/x.md"`, true},
		{"python-fstring", `python3 -c "root='.'; open(f'{root}/plan/x.md','w').write('x')"`, true},
		{"python-concat", `python3 -c "import os; open(os.getcwd()+'/plan/x.md','w').write('x')"`, true},
		{"node-template", "node -e \"require('fs').writeFileSync(`${process.cwd()}/plan/x.md`,'x')\"", true},
		{"grep-anchored-feed", `git ls-files | grep '^plan/' | xargs sed -i 's/a/b/'`, true},
		{"find-path-glob", `find . -path '*/plan/*.md' -exec sed -i 's/a/b/' {} +`, true},
		{"worktree-plan", `sed -i 's/a/b/' .claude/worktrees/x/plan/y.md`, true},
		{"other-checkout-absolute", `sed -i 's/a/b/' /srv/other/plan/x.md`, true},
		{"brace-expansion", `sed -i 's/a/b/' {plan/a.md,plan/b.md}`, true},
		{"dot-dot-through-dir", `sed -i 's/a/b/' tmp/../ai/rules/x.md`, true},
		{"dot-dot-after-nested-tree", `sed -i 's/a/b/' internal/le/ai/rules/../../../plan/x.md`, true},
		{"second-path-in-word", `sed -i 's/a/b/' internal/le/plan/a.go,plan/b.md`, true},
		{"dot-slash-nested", `sed -i 's/a/b/' ./internal/le/ai/rules/x.go`, false},
		{"tree-with-nested-name-redirect", `echo x > plan/learned/plan/x.md`, true},
		{"tree-with-nested-name-copy", `cp new.md plan/ai/rules/x.md`, true},
		{"rules-with-nested-plan", `sed -i 's/a/b/' ai/rules/points/plan/x.md`, true},
		{"climb-back-four", `perl -pi -e 's/a/b/' internal/le/ai/rules/../../../../ai/rules/commands.md`, true},
		{"climb-back-into-plan", `sed -i 's/a/b/' internal/plan/../../plan/x.md`, true},
		{"trailing-in-place-fan-out", `find plan -name '*.md' | xargs perl -pe 's/a/b/' -i`, true},
	})
	root := t.TempDir()
	for _, command := range []string{
		`sed -i 's/a/b/' ` + root + `/plan/x.md`,
		`sed -i 's/a/b/' "` + root + `/ai/rules/x.md"`,
		`echo x > ` + root + `/plan/x.md`,
		`cp new.md ` + root + `/ai/rules/x.md`,
	} {
		code, _, message := runHook(t, root, "pretool-bash", map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": command},
		})
		if code != 2 {
			t.Errorf("a write through the checkout root %q was admitted (code %d): %s", command, code, message)
		}
	}
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
