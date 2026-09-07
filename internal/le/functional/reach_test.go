// Related: reach.go -- the reduction these tests drive
//
// VALIDATES: spec-verify-scope-5-suite-coverage-map, the Key Design Decision
// that a package is REACHED only when the suite covered a block outside
// register.go and outside every func init() body.
// PREVENTS: the phase 1 answer coming back. Counting every covered block made
// the three-suite intersection 443 packages of 646, because Ze runs each
// package's init() on any process start, and a map built on that number selects
// every suite for every change.

package functional

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// module writes one Go file at its import path under a throwaway checkout root,
// so the reduction can find the file a profile row names.
func module(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// profile writes one text coverage profile and answers its path.
func profile(t *testing.T, root string, rows ...string) string {
	t.Helper()
	path := filepath.Join(root, "profile.txt")
	body := "mode: set\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the profile: %v", err)
	}
	return path
}

// TestOnlyABlockOutsideRegistrationCountsAsReached walks every reason a covered
// block says nothing about what a suite did, over one profile that holds all of
// them at once.
func TestOnlyABlockOutsideRegistrationCountsAsReached(t *testing.T) {
	root := t.TempDir()

	// Registration alone: what every ze process runs on any start.
	module(t, root, "internal/component/alpha/register.go", "package alpha\n\nfunc init() {\n\t_ = 1\n}\n")
	// An init() body outside register.go does the same job under another name.
	module(t, root, "internal/component/beta/beta.go",
		"package beta\n\nfunc init() {\n\t_ = 1\n}\n\nfunc Work() int {\n\treturn 2\n}\n")
	// A package the binary links and the suite never ran.
	module(t, root, "internal/component/gamma/gamma.go", "package gamma\n\nfunc Work() int {\n\treturn 3\n}\n")
	// A package the suite really reached, through work of its own.
	module(t, root, "internal/component/delta/delta.go", "package delta\n\nfunc Work() int {\n\treturn 4\n}\n")
	// Registration AND work, in two files of one package: the work decides.
	module(t, root, "internal/component/epsilon/register.go", "package epsilon\n\nfunc init() {\n\t_ = 5\n}\n")
	module(t, root, "internal/component/epsilon/epsilon.go", "package epsilon\n\nfunc Work() int {\n\treturn 5\n}\n")

	reached, err := packagesInProfile(root, profile(t, root,
		"github.com/ze-software/ze/internal/component/alpha/register.go:3.12,5.2 1 1",
		"github.com/ze-software/ze/internal/component/beta/beta.go:3.12,5.2 1 1",
		"github.com/ze-software/ze/internal/component/beta/beta.go:7.19,9.2 1 0",
		"github.com/ze-software/ze/internal/component/gamma/gamma.go:3.19,5.2 1 0",
		"github.com/ze-software/ze/internal/component/delta/delta.go:3.19,5.2 1 7",
		"github.com/ze-software/ze/internal/component/epsilon/register.go:3.12,5.2 1 1",
		"github.com/ze-software/ze/internal/component/epsilon/epsilon.go:3.19,5.2 1 1",
		// A dependency outside this module names no package a change set can
		// hold, so it can neither narrow nor widen anything.
		"github.com/some/vendored/thing.go:3.19,5.2 1 1",
	))
	if err != nil {
		t.Fatalf("reduce the profile: %v", err)
	}

	want := []string{"./internal/component/delta", "./internal/component/epsilon"}
	if !slices.Equal(reached, want) {
		t.Errorf("the suite reached %v, want %v", reached, want)
	}
}

// TestAFileThatWillNotParseWidens holds the direction every uncertainty in the
// reduction takes. A file go/ast cannot read might carry its covered block
// inside an init(), and the reduction cannot tell: calling the package reached
// runs the suite on a change to it, and calling it unreached would skip the
// suite for ever.
func TestAFileThatWillNotParseWidens(t *testing.T) {
	root := t.TempDir()
	module(t, root, "internal/component/broken/broken.go", "package broken\n\nfunc init() { this is not Go\n")

	reached, err := packagesInProfile(root, profile(t, root,
		"github.com/ze-software/ze/internal/component/broken/broken.go:3.14,3.20 1 1"))
	if err != nil {
		t.Fatalf("reduce the profile: %v", err)
	}

	if want := []string{"./internal/component/broken"}; !slices.Equal(reached, want) {
		t.Errorf("the suite reached %v, want %v", reached, want)
	}
}

// TestAProfileRowThisCannotReadIsARefusal keeps a shape change in the go tool
// from shrinking every recorded set in silence. A skipped row would drop
// packages, and a map that records fewer packages narrows more.
func TestAProfileRowThisCannotReadIsARefusal(t *testing.T) {
	for _, row := range []string{
		"github.com/ze-software/ze/internal/component/delta/delta.go:3.19,5.2 1",
		"github.com/ze-software/ze/internal/component/delta/delta.go:3.19,5.2 1 many",
		"github.com/ze-software/ze/internal/component/delta/delta.go 1 1",
		"github.com/ze-software/ze/internal/component/delta/delta.go:3.19 1 1",
		"github.com/ze-software/ze/internal/component/delta/delta.go:start,5.2 1 1",
	} {
		t.Run(row, func(t *testing.T) {
			root := t.TempDir()

			if _, err := packagesInProfile(root, profile(t, root, row)); err == nil {
				t.Error("a profile row this cannot read was accepted")
			}
		})
	}
}

// TestASuiteThatRecordedNothingReachesNoPackage is the state editor, web,
// runner and policy are in on every run: an empty profile, no error, and an
// empty set the caller omits from the map rather than writing.
func TestASuiteThatRecordedNothingReachesNoPackage(t *testing.T) {
	root := t.TempDir()

	reached, err := packagesInProfile(root, profile(t, root))

	if err != nil {
		t.Fatalf("reduce an empty profile: %v", err)
	}
	if len(reached) != 0 {
		t.Errorf("an empty profile reached %v", reached)
	}
}
