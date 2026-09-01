// VALIDATES: test/draft/ is invisible to suite discovery and every recursive
// .ci check, and SuiteDir routes --draft at the incubator.
// PREVENTS: a test under development reddening another session's native verify.

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
		require.Equalf(t, tc.want, isDraftPath(root, tc.path), "isDraftPath(%q)", tc.path)
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

// TestDraftDirIsInvisibleToRepoChecks is the ratchet for recursive .ci readers.
func TestDraftDirIsInvisibleToRepoChecks(t *testing.T) {
	repo := filepath.Join("..", "..", "..")
	for _, check := range []struct {
		file, needle, name string
	}{
		{"internal/test/runner/accept_only.go", "isDraftPath(testDir, p)", "accept-only lint"},
		{"internal/test/runner/ci_fixture_test.go", "isDraftPath(root, path)", "BGP frame-length fixtures"},
		{"internal/le/doc/wiring/checks.go", "entry.Name() == DraftDir", "documentation wiring"},
		{"internal/le/rfc/carriers.go", "strings.HasPrefix(rel, draftPrefix)", "RFC evidence"},
	} {
		raw, err := os.ReadFile(filepath.Join(repo, check.file)) //nolint:gosec // fixed repository path
		require.NoErrorf(t, err, "check source %s unreadable", check.file)
		require.Containsf(t, string(raw), check.needle,
			"%s (%s) no longer skips test/draft/", check.file, check.name)
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

// TestDraftReadmeNamesEveryCheck keeps the README's current producer table honest.
func TestDraftReadmeNamesEveryCheck(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "draft", "README.md")) //nolint:gosec // fixed repository path
	require.NoError(t, err)
	body := string(raw)
	for _, producer := range []string{
		"accept_only.go",
		"ci_fixture_test.go",
		"doc/wiring/checks.go",
		"rfc/carriers.go",
	} {
		require.Containsf(t, body, producer,
			"test/draft/README.md no longer names %s in its check table", producer)
	}
	require.True(t, strings.Contains(body, "TestDraftDirIsInvisibleToRepoChecks"),
		"the README must point at the ratchet so a new scanner's author knows where to register it")
}
