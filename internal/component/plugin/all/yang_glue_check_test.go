package all

import (
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	yangglue "github.com/ze-software/ze/internal/le/yang/glue"
)

// TestYANGGlueCurrent checks every generated yang/*/register.go and embed.go
// against the .yang files beside it.
//
// It calls yangglue.Check, which is the producer `./le yang glue check` answers
// from (internal/le/yang/glue/actions.go), so the check is the generator that
// writes the file rather than a reimplementation that can drift. The uncached
// backstop is `./le repository generated-check`, which runs that same action
// (generationChecks, internal/le/repo/generate.go).
//
// It mirrors TestGeneratedPluginImportsCurrent in all_test.go deliberately:
// same package, same call into the generator's own library.
//
// A .yang file is not a build input of this package, but Check opens it, and
// `go help test` says "Tests that open files within the package's module ...
// only match future runs in which the files ... are unchanged." So editing one
// invalidates the cached PASS. That was not true while the generator ran as a
// `go run` subprocess, whose opens the test binary never saw.
//
// VALIDATES: every yang/*/register.go is current with respect to its .yang
// sources, checked by the generator itself (internal/le/yang/glue/yangglue.go).
// PREVENTS: a YANG module silently never being wired because register.go was
// not regenerated -- config for that module would parse as unknown, with no
// build or test failure to point at the cause.
func TestYANGGlueCurrent(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout root: %v", err)
	}
	report, err := yangglue.Check(root)
	if err != nil {
		t.Fatalf("read the tree: %v", err)
	}
	if len(report.Stale) > 0 {
		t.Fatalf("yang glue generated files are stale: %v\nRun `./le yang glue write` and commit the result.", report.Stale)
	}

	// Non-vacuity. Check answers an empty report with no error when derive
	// matches nothing, so a layout change or a broken walk turns this test (and
	// the `./le repository generated-check` step) green while guarding zero
	// files. Assert it actually read a plausible number of directories, the
	// same way TestPythonUnitTests fails on an empty glob.
	const minYangDirs = 100 // 149 at the time of writing; a floor, not a count
	if report.Dirs == 0 {
		// Name the likely cause rather than a generic "it found nothing":
		// derive's own walk is what this assertion exists to catch.
		t.Fatal("yang glue check read NO yang/ directories, so it guarded nothing: derive no longer matches the tree layout")
	}
	if report.Dirs < minYangDirs {
		t.Fatalf("yang glue check read only %d yang/ directories (floor %d): discovery is broken and this check is guarding almost nothing", report.Dirs, minYangDirs)
	}
}
