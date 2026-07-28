// VALIDATES: test/draft/ is invisible to suite discovery and to every repo-wide
//            .ci gate, and that SuiteDir routes --draft at the incubator.
// PREVENTS:  a test under development reddening `make ze-verify` -- for its
//            author, and worse, for another session sharing the checkout who then
//            has to decide whether the red is theirs. The whole point of the
//            directory is that answer is always "no".

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuiteDirRoutesDraftToTheIncubator(t *testing.T) {
	require.Equal(t,
		filepath.Join("base", "test", "plugin"),
		SuiteDir("base", "plugin", false),
		"without --draft a suite must discover the REAL tests")
	require.Equal(t,
		filepath.Join("base", "test", "draft", "plugin"),
		SuiteDir("base", "plugin", true),
		"--draft must discover the incubator, never the real tests")
}

func TestIsDraftPath(t *testing.T) {
	root := filepath.Join("repo", "test")
	for _, tc := range []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "draft"), true},
		{filepath.Join(root, "draft", "plugin"), true},
		{filepath.Join(root, "draft", "plugin", "wip.ci"), true},
		{filepath.Join(root, "plugin"), false},
		{filepath.Join(root, "plugin", "eor.ci"), false},
		// A suite whose name merely STARTS with the word must not be pruned:
		// pruning test/drafting/ would silently delete a real suite from every
		// gate, which is the failure mode this whole mechanism exists to avoid.
		{filepath.Join(root, "drafting", "x.ci"), false},
		{filepath.Join("repo", "docs", "draft", "x.ci"), false},
	} {
		require.Equalf(t, tc.want, IsDraftPath(root, tc.path), "IsDraftPath(%q)", tc.path)
	}
}

// TestDiscoverIgnoresSubdirectories pins the property the whole design rests on:
// suite discovery is a NON-recursive glob, so test/draft/ needs no exclusion
// there. If Discover ever learns to recurse, this fails and whoever changed it
// finds out here rather than through a stranger's red verify.
func TestDiscoverIgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.ci"), []byte("# real\n"), 0o600))
	sub := filepath.Join(dir, DraftDirName, "plugin")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "wip.ci"), []byte("# draft\n"), 0o600))

	et := NewEncodingTests(dir)
	require.NoError(t, et.Discover(dir))

	names := make([]string, 0, et.Count())
	for _, r := range et.Registered() {
		names = append(names, r.Name)
	}
	require.Equal(t, []string{"real"}, names,
		"Discover must glob one level only; a .ci under a subdirectory is not a test")
}

// TestDraftDirIsInvisibleToRepoGates is the ratchet.
//
// Every gate below walks test/ RECURSIVELY, so each needs an explicit skip. This
// test asserts the skip is still spelled in each producer. It is a source-text
// assertion on purpose: the alternative is materializing a fake repo and running
// six checks against it, which is slower, and two of the six are Python.
//
// Adding a new repo-wide .ci scanner? Skip test/draft/ in it and add a row here.
func TestDraftDirIsInvisibleToRepoGates(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	for _, g := range []struct {
		file   string
		needle string
		gate   string
	}{
		{"internal/test/runner/accept_only.go", "IsDraftPath(testDir, p)", "accept-only lint"},
		{"internal/test/runner/ci_fixture_test.go", "IsDraftPath(root, path)", "BGP frame-length fixtures"},
		{"scripts/checks/ci_dispatch_commands.go", "draftTestDir", "dispatch-command check"},
		{"scripts/dev/verify_wiring_docs.py", "real_ci_files(root)", "ci-sleep ratchet"},
		{"scripts/dev/ci_observer_recover_check.py", `!= ("draft",)`, "observer-recover check"},
	} {
		raw, err := os.ReadFile(filepath.Join(repo, g.file)) //nolint:gosec // repo-relative fixed path
		require.NoErrorf(t, err, "gate source %s unreadable", g.file)
		require.Containsf(t, string(raw), g.needle,
			"%s (%s) no longer skips test/draft/: a test under development will redden it for every session in this checkout",
			g.file, g.gate)
	}
}

// TestDraftDirIsGitignored pins the guarantee that does not depend on any gate:
// CI checks out git, so an ignored tree does not exist there at all.
//
// It pins the CONTENTS form (`test/draft/*`) on purpose. Excluding the directory
// itself (`test/draft/`) also works for drafts, but git never descends into an
// excluded directory, so the negation below it is never evaluated and the README
// is ignored along with everything else -- which is how the convention becomes
// invisible from a fresh clone. `git check-ignore -v test/draft/README.md`
// reported exactly that before this was corrected.
func TestDraftDirIsGitignored(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".gitignore")) //nolint:gosec // repo-relative fixed path
	require.NoError(t, err)
	body := string(raw)
	require.Contains(t, body, "\ntest/draft/*\n",
		".gitignore must ignore test/draft/* -- otherwise a draft reaches CI, where none of the local skips apply")
	require.NotContains(t, body, "\ntest/draft/\n",
		"excluding the DIRECTORY stops git descending into it, so !test/draft/README.md never applies")
	require.Contains(t, body, "!test/draft/README.md",
		"the README must stay tracked, or the convention is undiscoverable from a fresh clone")
}

// TestDraftReadmeNamesEveryGate keeps the README's table honest: it is the first
// thing the next author reads, and a stale list there is how a gate gets missed.
func TestDraftReadmeNamesEveryGate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "draft", "README.md")) //nolint:gosec // repo-relative fixed path
	require.NoError(t, err)
	body := string(raw)
	for _, producer := range []string{
		"accept_only.go",
		"ci_fixture_test.go",
		"verify_wiring_docs.py",
		"ci_observer_recover_check.py",
		"ci_dispatch_commands.go",
		"inert_tests.go",
	} {
		require.Containsf(t, body, producer,
			"test/draft/README.md no longer names %s in its gate table", producer)
	}
	require.True(t, strings.Contains(body, "TestDraftDirIsInvisibleToRepoGates"),
		"the README must point at the ratchet so a new scanner's author knows where to register it")
}
