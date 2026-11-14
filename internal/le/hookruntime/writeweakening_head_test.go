// Related: writeedit.go -- writeWeakening, the edit hook these cases drive
package hookruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWeakeningJudgesOnlyWhatHEADHolds drives the pretool-writeedit weakening
// check (writeWeakening) over the refusal of 2026-11-13
// (plan/journal/guard-blocks-its-own-authors-repair.md): a new, untracked test
// file renamed a helper whose name a committed file of its directory also
// declared, and the hook refused the rename as a weakening.
// Method: a real Git fixture holding the committed file, then the author's
// rename Edit on the untracked file, and an Edit that comments out an
// assertion of the committed file, both through the hook's own entry point.
// VALIDATES: the hook admits an edit to units HEAD never held and still
// refuses the loss of a committed assertion.
// PREVENTS: a guard that blocks its author's repair of uncommitted test code.
func TestWeakeningJudgesOnlyWhatHEADHolds(t *testing.T) {
	root := t.TempDir()
	committed := filepath.Join(root, "pkg", "intent_test.go")
	if err := os.MkdirAll(filepath.Dir(committed), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "package pkg\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n\tintentRow(t)\n}\n" +
		"func intentRow(t *testing.T) {\n\trequire.Equal(t, 1, got)\n}\n"
	if err := os.WriteFile(committed, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	derivedDebtGit(t, root, "init", "-q")
	derivedDebtGit(t, root, "add", "--", "pkg/intent_test.go")
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline")
	untracked := filepath.Join(root, "pkg", "split_test.go")
	split := "package pkg\nfunc TestSplit(t *testing.T) {\n\tintentRow(t, 2)\n}\n" +
		"func intentRow(t *testing.T, want int) {\n\trequire.Equal(t, want, got)\n}\n"
	if err := os.WriteFile(untracked, []byte(split), 0o600); err != nil {
		t.Fatal(err)
	}

	rename := context{
		root: root, tool: "Edit", path: untracked,
		input:   map[string]any{"file_path": untracked, "old_string": "intentRow(", "new_string": "splitRow(", "replace_all": true},
		payload: Payload{SessionID: "parent-sid", AgentID: "agent-1"},
	}
	if verdict := writeWeakening(rename); verdict != nil {
		t.Fatalf("renaming a helper of an untracked test file = %#v, want admitted", verdict)
	}

	weaken := context{
		root: root, tool: "Edit", path: committed,
		input:   map[string]any{"file_path": committed, "old_string": "\trequire.NoError(t, err)\n", "new_string": "\t// require.NoError(t, err)\n"},
		payload: Payload{SessionID: "parent-sid", AgentID: "agent-1"},
	}
	if refused := writeWeakening(weaken); refused == nil || refused.code != 2 ||
		!strings.Contains(refused.message, "pkg.TestA") {
		t.Fatalf("a committed test losing an assertion = %#v, want pkg.TestA refused", refused)
	}
}

// TestWeakeningJudgesADraftSymlinkAsItsTarget proves the draft incubator's
// exemption follows the file an edit reaches: an Edit through a symlink under
// test/draft/ that names a committed test is judged as that test.
// Method: pkg/intent_test.go is committed; test/draft/pkg/link_test.go is a
// symlink to it; an Edit through the link comments out a committed assertion,
// through the hook's own entry point.
// VALIDATES: writeWeakening skips test/draft/ only when the resolved file lies
// there too, and testweakened.Proposed judges the target.
// PREVENTS: the incubator exemption carrying an edit of committed proof past
// the check (before the fix, the lexical path alone exempted it).
func TestWeakeningJudgesADraftSymlinkAsItsTarget(t *testing.T) {
	root := t.TempDir()
	committed := filepath.Join(root, "pkg", "intent_test.go")
	link := filepath.Join(root, "test", "draft", "pkg", "link_test.go")
	for _, directory := range []string{filepath.Dir(committed), filepath.Dir(link)} {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	body := "package pkg\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	if err := os.WriteFile(committed, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	derivedDebtGit(t, root, "init", "-q")
	derivedDebtGit(t, root, "add", "--", "pkg/intent_test.go")
	derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", "baseline")
	if err := os.Symlink(filepath.Join("..", "..", "..", "pkg", "intent_test.go"), link); err != nil {
		t.Fatal(err)
	}

	weaken := context{
		root: root, tool: "Edit", path: link,
		input:   map[string]any{"file_path": link, "old_string": "\trequire.NoError(t, err)\n", "new_string": "\t// require.NoError(t, err)\n"},
		payload: Payload{SessionID: "parent-sid", AgentID: "agent-1"},
	}
	if refused := writeWeakening(weaken); refused == nil || refused.code != 2 ||
		!strings.Contains(refused.message, "pkg.TestA") {
		t.Fatalf("an Edit through a draft symlink to a committed test = %#v, want pkg.TestA refused", refused)
	}
}

// TestWeakeningDraftExemptionNeedsAResolvedDraftFile proves the draft
// incubator's exemption is decided on the file a Write reaches, resolved
// strictly: a new regular file under test/draft/ is skipped, a symlink there
// to a committed test is judged as that test, and a dangling symlink there,
// whose committed target was removed from disk, is not skipped.
// Method: a real Git fixture per case, each Write through the hook's own
// entry point. (a) commits test/draft/pkg/old_test.go and deletes it, so a
// new draft file declaring a weaker copy of its TestA would be refused if
// judged; (c) and (d) commit pkg/intent_test.go and link to it from
// test/draft/pkg/link_test.go, (d) then removes the committed file.
// VALIDATES: writeWeakening skips only on a path resolveEvidencePath resolves
// under test/draft/; an unresolvable one reaches testweakened.Proposed,
// which refuses an alias it cannot resolve.
// PREVENTS: a Write through a dangling draft symlink recreating the committed
// file weaker and unjudged (before the fix, resolveExisting walked up past the
// dangling link to the draft directory and the hook skipped), and the fix
// losing the skip for a draft file that does not exist yet.
func TestWeakeningDraftExemptionNeedsAResolvedDraftFile(t *testing.T) {
	body := "package pkg\nfunc TestA(t *testing.T) {\n\trequire.NoError(t, err)\n}\n"
	weaker := "package pkg\nfunc TestA(t *testing.T) {\n\t// require.NoError(t, err)\n}\n"
	repository := func(t *testing.T, committed string) string {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(committed))
		for _, directory := range []string{filepath.Dir(path), filepath.Join(root, "test", "draft", "pkg")} {
			if err := os.MkdirAll(directory, 0o750); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		derivedDebtGit(t, root, "init", "-q")
		derivedDebtGit(t, root, "add", "--", committed)
		derivedDebtGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "-m", "baseline")
		return root
	}
	write := func(root, path string) *verdict {
		return writeWeakening(context{
			root: root, tool: toolWrite, path: path,
			input:   map[string]any{"file_path": path, "content": weaker},
			payload: Payload{SessionID: "parent-sid", AgentID: "agent-1"},
		})
	}

	t.Run("a new regular draft file is skipped", func(t *testing.T) {
		root := repository(t, "test/draft/pkg/old_test.go")
		if err := os.Remove(filepath.Join(root, "test", "draft", "pkg", "old_test.go")); err != nil {
			t.Fatal(err)
		}
		if verdict := write(root, filepath.Join(root, "test", "draft", "pkg", "new_test.go")); verdict != nil {
			t.Fatalf("a Write creating a new draft file = %#v, want skipped", verdict)
		}
	})

	t.Run("a draft symlink to a committed test is judged", func(t *testing.T) {
		root := repository(t, "pkg/intent_test.go")
		link := filepath.Join(root, "test", "draft", "pkg", "link_test.go")
		if err := os.Symlink(filepath.Join("..", "..", "..", "pkg", "intent_test.go"), link); err != nil {
			t.Fatal(err)
		}
		if refused := write(root, link); refused == nil || refused.code != 2 || !strings.Contains(refused.message, "pkg.TestA") {
			t.Fatalf("a weaker Write through a draft symlink to a committed test = %#v, want pkg.TestA refused", refused)
		}
	})

	t.Run("a dangling draft symlink is not skipped", func(t *testing.T) {
		root := repository(t, "pkg/intent_test.go")
		link := filepath.Join(root, "test", "draft", "pkg", "link_test.go")
		if err := os.Symlink(filepath.Join("..", "..", "..", "pkg", "intent_test.go"), link); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, "pkg", "intent_test.go")); err != nil {
			t.Fatal(err)
		}
		refused := write(root, link)
		if refused == nil || refused.code != 2 {
			t.Fatalf("a weaker Write through a dangling draft symlink to a removed committed test = %#v, want refused", refused)
		}
		// The refusal names the dangling link and its fix, not only the
		// failed lookup.
		for _, want := range []string{"link_test.go", "report the dangling link"} {
			if !strings.Contains(refused.message, want) {
				t.Fatalf("dangling-link refusal %q lacks %q", refused.message, want)
			}
		}
	})
}
