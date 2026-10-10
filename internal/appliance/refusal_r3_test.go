package appliance

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGokrazyForeignArchNamesTheGokrazyCommand proves `./le build gokrazy`
// with a GOARCH this host cannot build, and a cold cache, is refused with a fix
// for that command: building on a host of that arch, or GOARCH set to this
// host's arch. A kernel build for the foreign arch would fill another host's
// cache, so it is never offered.
//
// VALIDATES: round-3 ISSUE 3.
func TestGokrazyForeignArchNamesTheGokrazyCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	writeRuntimeKernelRegistry(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	foreign := archARM64
	if runtime.GOARCH == archARM64 {
		foreign = archAMD64
	}
	err := GokrazyRuntimeKernelReachable(foreign, []string{"--parent_dir", "my dir", "overwrite"})
	if err == nil {
		t.Fatalf("a cold %s cache on a %s host was reachable", foreign, runtime.GOARCH)
	}
	lines := strings.Split(err.Error(), "\n")
	if want := "GOARCH=" + runtime.GOARCH + " ./le build gokrazy --parent_dir 'my dir' overwrite"; lines[len(lines)-1] != want {
		t.Errorf("last line = %q, want %q:\n%v", lines[len(lines)-1], want, err)
	}
	if !strings.Contains(err.Error(), "build it on an "+foreign+" host") || strings.Contains(err.Error(), "--arch "+foreign) {
		t.Errorf("refusal does not offer the two moves that work:\n%v", err)
	}
	if err := GokrazyRuntimeKernelReachable(runtime.GOARCH, nil); err != nil {
		t.Errorf("the host's own arch was refused: %v", err)
	}
}

// TestForInstanceQuotesTheDirectory proves the image-build fix line survives a
// directory with a space when pasted into a shell.
//
// VALIDATES: round-3 NOTE 6.
func TestForInstanceQuotesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my lab")
	refusal := (&foreignArchKernelError{version: "7.2.9", arch: archARM64, cachedDir: "/cache"}).forInstance(dir, "lab").Error()
	lines := strings.Split(refusal, "\n")
	if want := "ze appliance --dir '" + dir + "' build lab"; lines[len(lines)-1] != want {
		t.Errorf("last line = %q, want %q", lines[len(lines)-1], want)
	}
}

// TestCheckGoToolchainRefusesAnotherModule proves a go.mod that belongs to
// another module is refused rather than read for Ze's go version.
//
// VALIDATES: round-3 NOTE 8.
func TestCheckGoToolchainRefusesAnotherModule(t *testing.T) {
	goMod := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(goMod, []byte("module example.com/other\n\ngo 1.20\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := CheckGoToolchain(goMod)
	if err == nil || !strings.Contains(err.Error(), "holds module example.com/other, not module github.com/ze-software/ze") {
		t.Fatalf("CheckGoToolchain(another module) = %v, want the refusal", err)
	}
}
