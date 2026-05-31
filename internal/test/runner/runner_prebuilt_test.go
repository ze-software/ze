package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	t.Setenv("ZE_TEST_NO_BUILD", "1")
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
