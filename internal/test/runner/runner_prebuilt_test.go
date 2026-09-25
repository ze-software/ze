package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/env"
)

// setBuildEnv sets a build-related env var and resets the env cache so
// env.Get/env.IsEnabled see the change (and the restore at cleanup).
func setBuildEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// clearBinOverrides drops the binary-path overrides so a test that asserts on
// the binary-is-ABSENT branch controls where the lookup points.
//
// ZE_BIN redirects the lookup to a path the test does not own. The QEMU unit
// phase runs with it exported -- it selects the FUNCTIONAL phase's
// cross-compiled ze -- so the lookup found a real binary, Build succeeded, and
// the assertion failed on an error that could not occur. It stayed invisible
// until that phase was repaired and ran for the first time.
//
// Deliberately NOT folded into setBuildEnv: TestBuildNoBuildWithEnvOverride sets
// ZE_BIN THROUGH that helper, and clearing on every call made its second
// call erase the value its first call had just established.
func clearBinOverrides(t *testing.T) {
	t.Helper()
	t.Setenv("ZE_BIN", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// TestBuildNoBuildSkip verifies the LE_TEST_NO_BUILD path: Build skips the
// in-process `go build` and uses a pre-built ze, erroring only when it is
// absent. This is what lets a slow QEMU VM run binaries cross-compiled on a
// fast host instead of compiling the whole tree over a slow 9p mount.
//
// VALIDATES: LE_TEST_NO_BUILD=1 makes Build a no-op when bin/ze exists, and a
// clear, named error when it does not. No harness binary is needed: every
// harness command is the runner's own le.
// PREVENTS: silent fallthrough to a real compile, or a confusing failure when
// the prebuilt binaries are missing.
func TestBuildNoBuildSkip(t *testing.T) {
	clearBinOverrides(t)
	setBuildEnv(t, "LE_TEST_NO_BUILD", "1")
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "bin"), 0o755))

	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	// Binaries absent: Build must fail with an actionable, named error rather
	// than silently compiling.
	err = r.Build(context.Background())
	require.Error(t, err, "Build must fail when LE_TEST_NO_BUILD is set but binaries are missing")
	require.Contains(t, err.Error(), "LE_TEST_NO_BUILD")

	// Binaries present: Build must skip compilation and succeed.
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "bin", "ze"), []byte("prebuilt"), 0o755))
	require.NoError(t, r.Build(context.Background()), "Build must skip and succeed when prebuilt binaries exist")
}

func TestBuildNoBuildWithEnvOverride(t *testing.T) {
	setBuildEnv(t, EnvNoBuild, "1")
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "bin"), 0o755))

	zeBin := filepath.Join(baseDir, "bin", "ze-linux-arm64")
	setBuildEnv(t, "ZE_BIN", zeBin)

	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	// Arch-suffixed binaries absent: must fail pointing at the overridden path.
	err = r.Build(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ze-linux-arm64")

	// Arch-suffixed binaries present: must succeed.
	require.NoError(t, os.WriteFile(zeBin, []byte("prebuilt"), 0o755))
	require.NoError(t, r.Build(context.Background()))
}
