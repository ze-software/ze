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

// TestBuildNoBuildSkip verifies the ZE_TEST_NO_BUILD path: Build skips the
// in-process `go build` and uses pre-built binaries, erroring only when they
// are absent. This is what lets a slow QEMU VM run binaries cross-compiled on a
// fast host instead of compiling the whole tree over a slow 9p mount.
//
// VALIDATES: ZE_TEST_NO_BUILD=1 makes Build a no-op when bin/ze and bin/ze-test
// exist, and a clear, named error when they do not.
// PREVENTS: silent fallthrough to a real compile, or a confusing failure when
// the prebuilt binaries are missing.
func TestBuildNoBuildSkip(t *testing.T) {
	setBuildEnv(t, "ZE_TEST_NO_BUILD", "1")
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "bin"), 0o755))

	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	// Binaries absent: Build must fail with an actionable, named error rather
	// than silently compiling.
	err = r.Build(context.Background())
	require.Error(t, err, "Build must fail when ZE_TEST_NO_BUILD is set but binaries are missing")
	require.Contains(t, err.Error(), "ZE_TEST_NO_BUILD")

	// Binaries present: Build must skip compilation and succeed.
	for _, name := range []string{"ze", "ze-test"} {
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, "bin", name), []byte("prebuilt"), 0o755))
	}
	require.NoError(t, r.Build(context.Background()), "Build must skip and succeed when prebuilt binaries exist")
}

func TestBuildNoBuildWithEnvOverride(t *testing.T) {
	setBuildEnv(t, "ZE_TEST_NO_BUILD", "1")
	baseDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "bin"), 0o755))

	zeBin := filepath.Join(baseDir, "bin", "ze-linux-arm64")
	testBin := filepath.Join(baseDir, "bin", "ze-test-linux-arm64")
	setBuildEnv(t, "ZE_BIN", zeBin)
	setBuildEnv(t, "ZE_TEST_BIN", testBin)

	r, err := NewRunner(NewEncodingTests(baseDir), baseDir)
	require.NoError(t, err)
	defer r.Cleanup()

	// Arch-suffixed binaries absent: must fail pointing at the overridden path.
	err = r.Build(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ze-linux-arm64")

	// Arch-suffixed binaries present: must succeed.
	for _, p := range []string{zeBin, testBin} {
		require.NoError(t, os.WriteFile(p, []byte("prebuilt"), 0o755))
	}
	require.NoError(t, r.Build(context.Background()))
}
