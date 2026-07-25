package cli

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

// TestBuildZeNoBuild verifies that buildZe honors ZE_TEST_NO_BUILD the same way
// runner.Runner.Build does: with the flag set it reuses a pre-built bin/ze rather
// than compiling, and errors clearly when the binary is absent.
//
// VALIDATES: the bgp suites (encode/plugin/decode/parse/reload), which build ze
// via buildZe rather than runner.Runner.Build, also skip the compile under
// ZE_TEST_NO_BUILD.
// PREVENTS: those suites recompiling ze on a slow target (e.g. a QEMU VM over
// 9p) when a host cross-compiled binary already exists, defeating the
// host-compile architecture used by `make ze-qemu-all-test`.
func TestBuildZeNoBuild(t *testing.T) {
	setBuildEnv(t, "ZE_TEST_NO_BUILD", "1")
	base := t.TempDir()

	// Binary absent: must fail with a named, actionable error rather than
	// silently compiling.
	_, err := buildZe(context.Background(), base)
	require.Error(t, err, "buildZe must fail when ZE_TEST_NO_BUILD is set but bin/ze is missing")
	require.Contains(t, err.Error(), "ZE_TEST_NO_BUILD")

	// Binary present: must return its path without compiling.
	zePath := filepath.Join(base, "bin", "ze")
	require.NoError(t, os.MkdirAll(filepath.Dir(zePath), 0o755))
	require.NoError(t, os.WriteFile(zePath, []byte("prebuilt"), 0o755))

	got, err := buildZe(context.Background(), base)
	require.NoError(t, err, "buildZe must reuse a pre-built bin/ze under ZE_TEST_NO_BUILD")
	require.Equal(t, zePath, got)
}

func TestBuildZeNoBuildEnvOverride(t *testing.T) {
	setBuildEnv(t, "ZE_TEST_NO_BUILD", "1")
	base := t.TempDir()

	override := filepath.Join(base, "bin", "ze-linux-arm64")
	setBuildEnv(t, "ZE_BIN", override)

	// Override path absent: must fail pointing at the overridden path.
	_, err := buildZe(context.Background(), base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ze-linux-arm64")

	// Override path present: must return it.
	require.NoError(t, os.MkdirAll(filepath.Dir(override), 0o755))
	require.NoError(t, os.WriteFile(override, []byte("prebuilt"), 0o755))

	got, err := buildZe(context.Background(), base)
	require.NoError(t, err)
	require.Equal(t, override, got)
}

func TestBuildZeNoBuildRelativeOverride(t *testing.T) {
	setBuildEnv(t, "ZE_TEST_NO_BUILD", "1")
	base := t.TempDir()

	// Relative path gets joined with baseDir.
	setBuildEnv(t, "ZE_BIN", "bin/ze-linux-arm64")

	override := filepath.Join(base, "bin", "ze-linux-arm64")
	require.NoError(t, os.MkdirAll(filepath.Dir(override), 0o755))
	require.NoError(t, os.WriteFile(override, []byte("prebuilt"), 0o755))

	got, err := buildZe(context.Background(), base)
	require.NoError(t, err)
	require.Equal(t, override, got)
}
