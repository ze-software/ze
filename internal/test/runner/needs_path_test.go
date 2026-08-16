package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newNeedsPathFixture writes a .ci carrying the given option line into a fake
// repo root (a dir holding a go.mod), and returns the parsed record. The go.mod
// is what repoRootFrom walks up to find, so the fixture must have one.
func newNeedsPathFixture(t *testing.T, optionLine string) (*Record, error) {
	t.Helper()
	ResetNickCounter()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600))
	testDir := filepath.Join(root, "test", "install")
	require.NoError(t, os.MkdirAll(testDir, 0o750))

	ciFile := filepath.Join(testDir, "test.ci")
	confFile := filepath.Join(testDir, "test.conf")
	require.NoError(t, os.WriteFile(ciFile, []byte("option=file:path=test.conf\n"+optionLine+"\n"), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

	et := NewEncodingTests(testDir)
	if _, err := et.parseAndAdd(ciFile); err != nil {
		return nil, err
	}
	return et.GetByNick("1"), nil
}

// VALIDATES: option=needs-path skips (with a reason naming the path and the
// hint) when the declared repo-relative path is absent, and is inert when it is
// present.
// PREVENTS: test/install/ze-kernel-overlay.ci failing on every CI run with
// "shasum: gokrazy/modcache/.../vmlinuz: No such file or directory". The pinned
// rtr7/kernel module is gitignored (gokrazy/modcache/.gitignore ignores all but
// the vendored gokrazy init source), so it exists only after
// `make ze-gokrazy-deps-download` and a fresh checkout has no way to satisfy the test.
func TestParseCIOptionNeedsPath(t *testing.T) {
	t.Run("absent-skips-with-reason-and-hint", func(t *testing.T) {
		rec, err := newNeedsPathFixture(t, "option=needs-path:value=gokrazy/modcache/github.com/rtr7:hint=make ze-gokrazy-deps-download")
		require.NoError(t, err)
		require.NotNil(t, rec)

		require.NotEmpty(t, rec.SkipReason, "an absent needs-path must skip, not run")
		assert.Contains(t, rec.SkipReason, "gokrazy/modcache/github.com/rtr7",
			"the reason must name the missing path")
		assert.Contains(t, rec.SkipReason, "make ze-gokrazy-deps-download",
			"the reason must name the command that materializes it")
	})

	t.Run("present-runs", func(t *testing.T) {
		ResetNickCounter()

		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600))
		require.NoError(t, os.MkdirAll(filepath.Join(root, "artifact", "sub"), 0o750))
		testDir := filepath.Join(root, "test", "install")
		require.NoError(t, os.MkdirAll(testDir, 0o750))

		ciFile := filepath.Join(testDir, "test.ci")
		confFile := filepath.Join(testDir, "test.conf")
		require.NoError(t, os.WriteFile(ciFile,
			[]byte("option=file:path=test.conf\noption=needs-path:value=artifact/sub:hint=make something\n"), 0o600))
		require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

		et := NewEncodingTests(testDir)
		_, err := et.parseAndAdd(ciFile)
		require.NoError(t, err)

		rec := et.GetByNick("1")
		require.NotNil(t, rec)
		assert.Empty(t, rec.SkipReason, "a present needs-path must leave the test running")
	})

	// VALIDATES: a glob matches a version-pinned artifact, and matches NOTHING
	// when only a sibling directory exists.
	// PREVENTS: the fail-open shape the first version shipped -- declaring the
	// parent dir `gokrazy/modcache/github.com/rtr7` was satisfied by the
	// unrelated `rtr7/dhcp4@...` entries beside the kernel module, so a checkout
	// with no kernel passed the gate and died on the missing vmlinuz anyway.
	t.Run("glob-matches-versioned-artifact", func(t *testing.T) {
		ResetNickCounter()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0o600))
		testDir := filepath.Join(root, "test", "install")
		require.NoError(t, os.MkdirAll(testDir, 0o750))

		// A sibling that must NOT satisfy a glob aimed at kernel@*.
		require.NoError(t, os.MkdirAll(filepath.Join(root, "mc", "rtr7", "dhcp4@v1"), 0o750))

		ciFile := filepath.Join(testDir, "test.ci")
		confFile := filepath.Join(testDir, "test.conf")
		require.NoError(t, os.WriteFile(ciFile,
			[]byte("option=file:path=test.conf\noption=needs-path:value=mc/rtr7/kernel@*/vmlinuz:hint=make deps\n"), 0o600))
		require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))

		et := NewEncodingTests(testDir)
		_, err := et.parseAndAdd(ciFile)
		require.NoError(t, err)
		rec := et.GetByNick("1")
		require.NotNil(t, rec)
		assert.NotEmpty(t, rec.SkipReason,
			"a sibling module must not satisfy a glob aimed at kernel@*/vmlinuz")

		// Now materialize the real artifact under a pinned version.
		require.NoError(t, os.MkdirAll(filepath.Join(root, "mc", "rtr7", "kernel@v0.0.0-abc"), 0o750))
		require.NoError(t, os.WriteFile(
			filepath.Join(root, "mc", "rtr7", "kernel@v0.0.0-abc", "vmlinuz"), []byte("k"), 0o600))

		ResetNickCounter()
		et2 := NewEncodingTests(testDir)
		_, err = et2.parseAndAdd(ciFile)
		require.NoError(t, err)
		rec2 := et2.GetByNick("1")
		require.NotNil(t, rec2)
		assert.Empty(t, rec2.SkipReason, "the pinned artifact must satisfy the glob")
	})

	t.Run("hint-is-optional", func(t *testing.T) {
		rec, err := newNeedsPathFixture(t, "option=needs-path:value=nope/missing")
		require.NoError(t, err)
		require.NotNil(t, rec)
		assert.Contains(t, rec.SkipReason, "nope/missing")
	})
}

// VALIDATES: a malformed option=needs-path is a parse error on every host.
// PREVENTS: a typo silently disabling the gate -- an absolute path or one
// containing ".." would resolve outside the repo, and a missing value= would
// make the option a no-op that reads as a satisfied prerequisite.
func TestParseCIOptionNeedsPathRejectsMalformed(t *testing.T) {
	for name, line := range map[string]string{
		"missing-value": "option=needs-path",
		"empty-value":   "option=needs-path:value=",
		"absolute":      "option=needs-path:value=/etc/passwd",
		"escaping":      "option=needs-path:value=../outside",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := newNeedsPathFixture(t, line)
			require.Error(t, err, "malformed needs-path must fail at parse time")
			assert.Contains(t, err.Error(), "needs-path")
		})
	}
}
