// VALIDATES: le exec sets the four cache variables, keeps Ze's patched GOROOT
// only inside a checkout, and refuses its own shims and a missing program
// (spec-le-one-go-cache-per-host AC-7, AC-8, AC-9).
// PREVENTS: a shim loop (R-1) and Ze's GOROOT leaking into another project (R-8).

package leexec

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// fakeCheckout makes a directory carrying both checkout markers, so lepath
// counts it as a Ze checkout, and answers a subdirectory of it.
func fakeCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, marker := range []string{"go.mod", "feature-gates.txt"} {
		if err := os.WriteFile(filepath.Join(root, marker), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	work := filepath.Join(root, "internal", "work")
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	return work
}

// program writes an executable file at path with body, creating its directory.
func program(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil { //nolint:gosec // G306: a test program has to be executable
		t.Fatal(err)
	}
	return path
}

// values answers the last value of each variable environ assigns, and whether
// it assigns it at all.
func values(environ []string) map[string]string {
	assigned := make(map[string]string, len(environ))
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		assigned[name] = value
	}
	return assigned
}

// TestExecSetsCacheEnvironment proves AC-7 short of the execve: inside a
// checkout the program gets the four cache variables under the per-user target,
// each directory exists, GOROOT is left as activatePatchedGOROOT set it, and
// argv is the caller's words untouched.
func TestExecSetsCacheEnvironment(t *testing.T) {
	work := fakeCheckout(t)
	cacheHome := t.TempDir()
	tool := program(t, filepath.Join(t.TempDir(), "tool"), "#!/bin/sh\nexit 0\n")
	patched := "/patched/tree/go1.99"
	environ := []string{
		"XDG_CACHE_HOME=" + cacheHome,
		"GOROOT=" + patched,
		"GOCACHE=/somewhere/else",
		"PATH=/usr/bin",
	}
	args := []string{tool, "-remote=auto", "--", "|", "a b"}

	var stderr bytes.Buffer
	launched, err := prepare(work, args, environ, &stderr)
	if err != nil {
		t.Fatalf("prepare refused: %v\n%s", err, stderr.String())
	}
	if launched.program != tool {
		t.Errorf("program = %q, want %q", launched.program, tool)
	}
	if !slices.Equal(launched.argv, args) {
		t.Errorf("argv = %q, want the caller's words untouched %q", launched.argv, args)
	}
	got := values(launched.environ)
	for _, variable := range cacheVariables {
		want := filepath.Join(cacheHome, "ze", variable.directory)
		if got[variable.name] != want {
			t.Errorf("%s = %q, want %q", variable.name, got[variable.name], want)
		}
		info, err := os.Stat(want)
		if err != nil || !info.IsDir() {
			t.Errorf("%s directory %s was not created: %v", variable.name, want, err)
		}
	}
	if count := strings.Count(strings.Join(launched.environ, "\n"), "GOCACHE="); count != 1 {
		t.Errorf("GOCACHE is assigned %d times, want once", count)
	}
	if got["GOROOT"] != patched {
		t.Errorf("inside a checkout GOROOT = %q, want the patched tree %q kept", got["GOROOT"], patched)
	}
}

// TestExecOutsideCheckoutKeepsCallerGOROOT proves AC-8 and R-8: outside every
// checkout the cache variables are still set, a caller's real GOROOT is kept,
// and a GOROOT or PATH entry that is a patched tree is removed, so no program
// on another repository runs on Ze's toolchain. It reads the patches of the
// checkout this test runs in, the one lepath.Root answers, and builds a tree
// look-alike: a directory holding a file one of the patches adds.
func TestExecOutsideCheckoutKeepsCallerGOROOT(t *testing.T) {
	outside := t.TempDir()
	if _, inside := lepath.CheckoutContaining(outside); inside {
		t.Skipf("the temporary directory %s lies inside a checkout", outside)
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("this test runs in a checkout: %v", err)
	}
	patches, err := gen.Patches(root)
	if err != nil {
		t.Fatal(err)
	}
	added := gen.AddedFiles(patches[0])
	if len(added) == 0 {
		t.Fatalf("patch %s adds no file, so no tree look-alike can be built", patches[0].Path)
	}
	tree := filepath.Join(t.TempDir(), "tree")
	program(t, filepath.Join(tree, filepath.FromSlash(added[0])), "")
	caller := t.TempDir()
	tool := program(t, filepath.Join(t.TempDir(), "tool"), "#!/bin/sh\nexit 0\n")
	cacheHome := t.TempDir()
	searchPath := filepath.Join(tree, "bin") + string(filepath.ListSeparator) + "/usr/bin"

	for _, test := range []struct {
		name       string
		goroot     []string
		wantGOROOT string
		wantSet    bool
	}{
		{name: "caller GOROOT kept", goroot: []string{"GOROOT=" + caller}, wantGOROOT: caller, wantSet: true},
		{name: "patched GOROOT removed", goroot: []string{"GOROOT=" + tree}, wantSet: false},
		{name: "no GOROOT stays unset", goroot: nil, wantSet: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			environ := append([]string{"XDG_CACHE_HOME=" + cacheHome, "PATH=" + searchPath}, test.goroot...)
			var stderr bytes.Buffer
			launched, err := prepare(outside, []string{tool}, environ, &stderr)
			if err != nil {
				t.Fatalf("prepare refused: %v\n%s", err, stderr.String())
			}
			got := values(launched.environ)
			goroot, set := got["GOROOT"]
			if set != test.wantSet || goroot != test.wantGOROOT {
				t.Errorf("GOROOT = %q (set %v), want %q (set %v)", goroot, set, test.wantGOROOT, test.wantSet)
			}
			if got["PATH"] != "/usr/bin" {
				t.Errorf("PATH = %q, want the tree's bin removed: %q", got["PATH"], "/usr/bin")
			}
			for _, variable := range cacheVariables {
				want := filepath.Join(cacheHome, "ze", variable.directory)
				if got[variable.name] != want {
					t.Errorf("%s = %q, want %q", variable.name, got[variable.name], want)
				}
			}
		})
	}
}

// TestExecRefusesShimAndMissingPath proves AC-9 and R-1: a path holding the
// shim marker and a path that does not exist are refused with exit 1 and a
// message naming the path, why, and ./le setup, and a bare name skips a shim on
// PATH to reach the real tool behind it.
func TestExecRefusesShimAndMissingPath(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	shims := t.TempDir()
	shim := program(t, filepath.Join(shims, "gopls"), "#!/bin/sh\n"+ShimMarker+"\nexec le exec /real/gopls \"$@\"\n")
	missing := filepath.Join(t.TempDir(), "gopls")

	for _, test := range []struct {
		name string
		path string
		why  string
	}{
		{name: "shim", path: shim, why: "is a shim ./le setup wrote"},
		{name: "missing", path: missing, why: "no such file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{test.path, "version"}, &stdout, &stderr)
			if code != 1 {
				t.Errorf("exit = %d, want 1", code)
			}
			message := stderr.String()
			for _, want := range []string{test.path, test.why, "./le setup"} {
				if !strings.Contains(message, want) {
					t.Errorf("refusal does not name %q:\n%s", want, message)
				}
			}
			if stdout.Len() != 0 {
				t.Errorf("a refusal wrote to stdout: %q", stdout.String())
			}
		})
	}

	t.Run("bare name skips the shim", func(t *testing.T) {
		real := program(t, filepath.Join(t.TempDir(), "gopls"), "#!/bin/sh\nexit 0\n")
		searchPath := shims + string(filepath.ListSeparator) + filepath.Dir(real)
		var stderr bytes.Buffer
		found, err := resolve("gopls", t.TempDir(), searchPath, &stderr)
		if err != nil {
			t.Fatalf("resolve refused: %v\n%s", err, stderr.String())
		}
		if found != real {
			t.Errorf("resolved %q, want the real tool %q behind the shim", found, real)
		}
	})

	t.Run("bare name with only a shim is refused", func(t *testing.T) {
		var stderr bytes.Buffer
		_, err := resolve("gopls", t.TempDir(), shims, &stderr)
		if !errors.Is(err, errRefused) {
			t.Fatalf("err = %v, want errRefused", err)
		}
		if !strings.Contains(stderr.String(), "./le setup") {
			t.Errorf("refusal does not name ./le setup:\n%s", stderr.String())
		}
	})

	t.Run("usage", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		if code := run(nil, &stdout, &stderr); code != 2 {
			t.Errorf("bare le exec exit = %d, want 2", code)
		}
		if code := run([]string{"-x"}, &stdout, &stderr); code != 2 {
			t.Errorf("le exec -x exit = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "-x is not an option") {
			t.Errorf("le exec -x does not name the option:\n%s", stderr.String())
		}
	})
}
