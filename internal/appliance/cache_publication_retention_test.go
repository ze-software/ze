// Design: docs/architecture/appliance/build-artifacts.md -- shared kernel publication retention

package appliance

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// TestInstallerKernelPublicationPreservesSharedEntries builds through the real
// resolver with controlled output and reopens older shared artifacts by path.
// An open artifact, a useful comparison and another run's staging directory
// remain usable even when a successful build exceeds the old keep-two bound.
func TestInstallerKernelPublicationPreservesSharedEntries(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(kernelTestOutputDirEnv, "")
	writeInstallerKernelRegistry(t)
	entries := sharedKernelPublicationEntries(t, kernelCacheDir, kernelTargetInstaller, kernelFileName)
	setTestHTTP(t, func(string) (*http.Response, error) { return nil, errors.New("fixture has no download") })
	setTestKernelBuild(t, fakeInstallerBuild("new installer kernel"))

	path, err := resolveKernel(root, "7.1.1", kernelbuilder.HostArch(), defaultKernelProfile, "", kernelTargetInstaller, "")
	if err != nil {
		t.Fatalf("publish installer kernel: %v", err)
	}
	assertSharedPublicationBytes(t, path, "new installer kernel")
	for path, want := range entries {
		assertSharedPublicationBytes(t, path, want)
	}
}

// TestRuntimeKernelPublicationPreservesSharedEntries publishes a controlled
// runtime tree through resolveKernel and checks both its usable new output and
// every older shared input, including a held-open artifact and staging tree.
func TestRuntimeKernelPublicationPreservesSharedEntries(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(kernelTestOutputDirEnv, "")
	writeRuntimeKernelRegistry(t)
	entries := sharedKernelPublicationEntries(t, runtimeKernelCacheDir, kernelTargetRuntime, "vmlinuz")
	setTestKernelBuild(t, fakeRuntimeBuild)

	tree, err := resolveKernel(root, "7.1.1", kernelbuilder.HostArch(), runtimeKernelProfile, "", kernelTargetRuntime, "")
	if err != nil {
		t.Fatalf("publish runtime kernel: %v", err)
	}
	assertSharedPublicationBytes(t, filepath.Join(tree, "vmlinuz"), "runtime-vmlinuz")
	if _, err := os.Stat(filepath.Join(tree, "lib", "modules", "7.1.1-ze", "modules.dep")); err != nil {
		t.Errorf("published runtime modules cannot be reopened: %v", err)
	}
	for path, want := range entries {
		assertSharedPublicationBytes(t, path, want)
	}
}

// sharedKernelPublicationEntries creates only private fixture artifacts. Their
// ages establish that neither a grace period nor a recency rank proves disuse.
func sharedKernelPublicationEntries(t *testing.T, namespace, target, artifact string) map[string]string {
	t.Helper()
	namespace = filepath.Join(resolveCacheDir(), namespace)
	entries := make(map[string]string)
	for index, suffix := range []string{"open", "comparison", "previous", "newer"} {
		name := "7.0.1-" + target + "-" + kernelbuilder.HostArch() + "-runtime-" + suffix
		path := filepath.Join(namespace, name, artifact)
		want := "retained shared kernel " + suffix
		writeSharedPublicationEntry(t, path, want, time.Now().Add(time.Duration(index-72)*time.Hour))
		entries[path] = want
		if suffix == "open" {
			open, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := open.Close(); err != nil {
					t.Errorf("close held shared artifact: %v", err)
				}
			})
		}
	}
	staging := filepath.Join(namespace, ".copytree-other-run", artifact)
	writeSharedPublicationEntry(t, staging, "other run's staging output", time.Now().Add(-72*time.Hour))
	entries[staging] = "other run's staging output"
	return entries
}

func writeSharedPublicationEntry(t *testing.T, path, content string, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Dir(path), modified, modified); err != nil {
		t.Fatal(err)
	}
}

func assertSharedPublicationBytes(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("published or retained artifact cannot be reopened at %s: %v", path, err)
		return
	}
	if string(body) != want {
		t.Errorf("artifact at %s = %q, want %q", path, body, want)
	}
}
