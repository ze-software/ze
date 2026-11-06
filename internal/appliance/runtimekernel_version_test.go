// Design: docs/architecture/appliance/build-artifacts.md -- the runtime kernel a selected root owns

package appliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// VALIDATES: a selected root pinning another Linux release refuses, naming both
// releases, before its config reaches a build of this binary's release; a root
// pinning this binary's release goes on to the next stage.
// PREVENTS: a kernel of one release built from another release's config, cached
// under a name that hides the mix.
func TestRuntimeKernelTreeRefusesAnotherRootsRelease(t *testing.T) {
	root := t.TempDir()
	versionFile := filepath.Join(root, filepath.FromSlash(kernelVersionRel))
	if err := os.MkdirAll(filepath.Dir(versionFile), 0o750); err != nil {
		t.Fatal(err)
	}
	foreign := defaultKernelVersion + ".99"
	if err := os.WriteFile(versionFile, []byte(foreign+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := RuntimeKernelTree(root, runtime.GOARCH)
	if err == nil {
		t.Fatal("a root pinning another release was accepted")
	}
	for _, want := range []string{foreign, defaultKernelVersion, "Fix:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q lacks %q", err, want)
		}
	}

	if err := os.WriteFile(versionFile, []byte(defaultKernelVersion+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = RuntimeKernelTree(root, runtime.GOARCH)
	if err == nil {
		t.Fatal("a root with no kernel profile was accepted")
	}
	if strings.Contains(err.Error(), foreign) || strings.Contains(err.Error(), "Fix: rebuild this tool") {
		t.Errorf("a matching release met the version refusal: %v", err)
	}
}
