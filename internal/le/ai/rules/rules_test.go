// VALIDATES: spec-le-is-a-ze-binary AC-5 and AC-11. Functions expose the shared
// corpus spellings and match the Python answers.
// PREVENTS: messages that look equivalent but differ by byte. Python repr()
// selects quotes from content, and Python slices count runes. Go slices count
// bytes. Thus, a quoted rule line with an em dash can silently diverge.

package airules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestPyReprPicksTheQuotePythonPicks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"it's", `"it's"`},
		{`say "hi"`, `'say "hi"'`},
		{`both ' and "`, `'both \' and "'`},
		{"back\\slash", `'back\\slash'`},
		{"tab\there", `'tab\there'`},
		{"line\nbreak", `'line\nbreak'`},
		{"", "''"},
		{"em—dash", "'em—dash'"},
		{"\x00", `'\x00'`},
		{"\x7f", `'\x7f'`},
		// A zero-width space and a non-breaking space are both unprintable to
		// Python, and each escapes at its own width. Both are reachable: a rule
		// line pasted from a rendered document carries them.
		{"\u200b", `'\u200b'`},
		{"\u00a0", `'\xa0'`},
	}
	for _, tc := range cases {
		if got := pyRepr(tc.in); got != tc.want {
			t.Errorf("pyRepr(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestPyListReprRendersAPythonList(t *testing.T) {
	if got := pyListRepr(nil); got != "[]" {
		t.Errorf("pyListRepr(nil) = %s, want []", got)
	}
	if got := pyListRepr([]string{"advisory", "blocking"}); got != "['advisory', 'blocking']" {
		t.Errorf("pyListRepr = %s", got)
	}
}

func TestIsUpperStemNeedsACasedCharacter(t *testing.T) {
	cases := map[string]bool{
		"RETIRED":     true,
		"TRIGGERS":    true,
		"CORE":        true,
		"rule-format": false,
		"Planning":    false,
		"123":         false,
		"":            false,
		"ABC-1":       true,
	}
	for stem, want := range cases {
		if got := isUpperStem(stem); got != want {
			t.Errorf("isUpperStem(%q) = %v, want %v", stem, got, want)
		}
	}
}

func TestFirstRunesCountsRunesNotBytes(t *testing.T) {
	// The corpus is full of em dashes, and each one is three bytes. A byte
	// slice would cut one in half and produce invalid UTF-8 in a message the
	// script spells with Python's rune-counting slice.
	line := "a—b—c"
	if got := firstRunes(line, 3); got != "a—b" {
		t.Errorf("firstRunes = %q, want %q", got, "a—b")
	}
	if got := firstRunes(line, 99); got != line {
		t.Errorf("firstRunes past the end = %q, want the whole string", got)
	}
	if got := firstRunes(line, 0); got != "" {
		t.Errorf("firstRunes(_, 0) = %q, want the empty string", got)
	}
}

func TestLastRuneAnswersARuneNotAByte(t *testing.T) {
	if got := lastRune("ends—"); got != "—" {
		t.Errorf("lastRune = %q, want an em dash", got)
	}
	if got := lastRune("ends,"); got != "," {
		t.Errorf("lastRune = %q, want a comma", got)
	}
	if got := lastRune(""); got != "" {
		t.Errorf("lastRune of the empty string = %q", got)
	}
}

func TestRuleFilesDropsTheGeneratedArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"planning.md", "INDEX.md", "CONDENSED.md", "TRIGGERS.md", "CORE.md",
		"architecture.md", "notes.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "points"), 0o750); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	found, err := ruleFiles(dir)
	if err != nil {
		t.Fatalf("RuleFiles: %v", err)
	}
	want := []string{
		filepath.Join(dir, "architecture.md"),
		filepath.Join(dir, "planning.md"),
	}
	if len(found) != len(want) {
		t.Fatalf("RuleFiles found %v, want %v", found, want)
	}
	for i := range want {
		if found[i] != want[i] {
			t.Errorf("RuleFiles[%d] = %s, want %s", i, found[i], want[i])
		}
	}
}

func TestRelToNamesAPathAgainstTheCheckout(t *testing.T) {
	tree := string(filepath.Separator) + "checkout"
	if got := relTo(tree, filepath.Join(tree, "ai", "rules", "planning.md")); got != "ai/rules/planning.md" {
		t.Errorf("relTo = %s", got)
	}
}

func TestNativeArtifactCommandsPreserveRoutingAndCheckWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, update, check, artifact string
		kept                          []string
		dropped                       string
	}{
		{
			name: "trigger rows", update: "update condensed", check: "check condensed", artifact: "TRIGGERS.md",
			kept: []string{
				"| `ai/rules/completion.md` | blocking | before claiming any gokrazy qdisc work done |",
				"| `ai/rules/never-destroy-work.md` | blocking, always-on | before deleting a file holding uncommitted work |",
				"| `ai/rules/rfc-compliance.md` | blocking, always-on | writing protocol code for any RFC ze implements |",
				"| `ai/rules/rule-precedence.md` | blocking, always-on | when two rules disagree |",
			},
		},
		{
			name: "core directives", update: "update condensed", check: "check condensed", artifact: "CORE.md",
			kept: []string{
				"## Never Destroy Work\n`ai/rules/never-destroy-work.md`",
				"- MUST ask first.",
				"## RFC Compliance\n`ai/rules/rfc-compliance.md`",
				"- MUST conform.",
			},
			dropped: "- MUST finish it.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := digestCorpus()
			files["plan/post-release/engineering/spec-thing.md"] = "# Spec\n\n## Task\n\nRewrite the gokrazy qdisc encoder.\n"
			root := digestTree(t, files)
			t.Setenv("ZE_REPO_ROOT", root)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			if answer, code := Answer(strings.Fields(tc.update)); code != 0 {
				t.Fatalf("%s = %d, %+v", tc.update, code, answer)
			}
			path := filepath.Join(root, "ai", "rules", tc.artifact)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.kept {
				if strings.Count(string(body), want) != 1 {
					t.Errorf("%s must carry exactly one %q: %s", tc.artifact, want, body)
				}
			}
			if tc.dropped != "" && strings.Contains(string(body), tc.dropped) {
				t.Errorf("%s loaded a routed directive: %s", tc.artifact, body)
			}
			if answer, code := Answer(strings.Fields(tc.check)); code != 0 {
				t.Fatalf("fresh %s = %d, %+v", tc.check, code, answer)
			}
			// Both words have eight bytes. A size-only freshness check is not
			// enough, and a check must never repair the artifact it rejects.
			stale := strings.Replace(string(body), "blocking", "advisory", 1)
			if stale == string(body) || len(stale) != len(body) {
				t.Fatal("fixture must introduce same-size drift")
			}
			if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
				t.Fatal(err)
			}
			if answer, code := Answer(strings.Fields(tc.check)); code != 1 {
				t.Errorf("stale %s = %d, %+v; want 1", tc.check, code, answer)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != stale {
				t.Errorf("%s rewrote the rejected artifact", tc.check)
			}
		})
	}
}

func TestNativeRenderCommandPreservesPointBodiesAndCheckWithoutWriting(t *testing.T) {
	const want = "# T\n\n**When:** when x\n**Severity:** blocking\n\n## S\n\n- **MUST x.**\n"
	root := digestTree(t, map[string]string{
		"ai/rules/points/sample/manifest.md": "---\ntitle: T\nwhen: when x\nseverity: blocking\n---\ns ## S\n  p\n",
		"ai/rules/points/sample/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST x.**\n",
	})
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if answer, code := Answer([]string{"update", "render"}); code != 0 {
		t.Fatalf("update render = %d, %+v", code, answer)
	}
	path := filepath.Join(root, "ai", "rules", "sample.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("rendered rule = %q, want %q", body, want)
	}
	if answer, code := Answer([]string{"check", "render"}); code != 0 {
		t.Fatalf("fresh check render = %d, %+v", code, answer)
	}
	stale := strings.Replace(want, "MUST x", "MUST y", 1)
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	if answer, code := Answer([]string{"check", "render"}); code != 1 {
		t.Errorf("stale check render = %d, %+v; want 1", code, answer)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != stale {
		t.Errorf("check render rewrote the rejected rule: %q", after)
	}
}

func TestNativeRenderCommandRefusesToOverwriteDigestArtifacts(t *testing.T) {
	for _, name := range []string{"CORE", "TRIGGERS", "INDEX"} {
		t.Run(name, func(t *testing.T) {
			const saved = "another generator owns these bytes\n"
			root := digestTree(t, map[string]string{
				"ai/rules/" + name + ".md":                 saved,
				"ai/rules/points/" + name + "/manifest.md": "---\ntitle: T\nwhen: when x\nseverity: blocking\n---\ns ## S\n  p\n",
				"ai/rules/points/" + name + "/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST x.**\n",
			})
			t.Setenv("ZE_REPO_ROOT", root)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			answer, code := Answer([]string{"update", "render"})
			report, ok := answer.(RenderReport)
			if code != 1 || !ok || len(report.Failures) != 1 ||
				!strings.Contains(report.Failures[0], "a point directory may not be named for a generated artifact") {
				t.Fatalf("update render = %d, %+v; want artifact ownership refusal", code, answer)
			}
			body, err := os.ReadFile(filepath.Join(root, "ai", "rules", name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != saved {
				t.Errorf("update render overwrote %s.md: %q", name, body)
			}
		})
	}
}
