package fleetui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/test/golden"
)

// The golden capture of the fleet package: every templ component it declares
// is rendered with fixed data and compared, byte for byte, with a fixture
// under testdata/golden. The walk over the .templ files is the authority on
// which components exist, so a component with no entry in the plan fails the
// test, and so does a fixture no entry produces.
//
// Recapture a deliberate markup change with the capture action of
// docs/contributing/running-commands.md, over this package, with
// `-run TestFleetGoldenOutput -update-golden`.

// fleetGoldenParts lists the capture plan of each family of templates. Each
// entry is a function its own test file declares, and it answers the plan of
// the .templ files that family owns.
//
// THIS IS WHERE A FAMILY ADDS ITS PLAN: declare goldenSpec<Family> in a test
// file of your own, beside your templates, and add its name here on a line
// of its own. Never edit another family's function. Two families naming the
// same file fail the test.
var fleetGoldenParts = []func(t *testing.T) golden.Spec{
	goldenSpecShell, // shell.templ, tree.templ, search.templ: golden_shell_test.go
}

// fleetGoldenSpecVar is what a coverage failure tells the reader to edit.
const fleetGoldenSpecVar = "fleetGoldenParts"

// fleetGoldenSpec joins the plans of every family into the plan of the
// package.
func fleetGoldenSpec(t *testing.T) golden.Spec {
	t.Helper()

	merged := golden.Spec{}
	for _, part := range fleetGoldenParts {
		for file, units := range part(t) {
			if _, twice := merged[file]; twice {
				t.Fatalf("two entries of %s plan the capture of %s; a file belongs to one family", fleetGoldenSpecVar, file)
			}
			merged[file] = units
		}
	}

	return merged
}

// TestFleetGoldenOutput renders every component of the package and compares
// each with its fixture.
//
// VALIDATES: the rendered bytes of every fleet component are the ones a
// reviewer accepted, and every component and every fixture has a case.
// PREVENTS: markup that moved with no fixture behind it, and a component
// nobody renders in a test.
func TestFleetGoldenOutput(t *testing.T) {
	root := filepath.Join("testdata", "golden")
	if !golden.Updating() {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("fixture directory %s is missing; capture it with -update-golden: %v", root, err)
		}
	}

	set := golden.Set{
		FS:      os.DirFS("."),
		Dir:     ".",
		Ext:     ".templ",
		Spec:    fleetGoldenSpec(t),
		SpecVar: fleetGoldenSpecVar,
	}

	files := set.Files(t)
	set.AssertCoversFS(t, files)

	written := make([]string, 0, len(files))

	for _, file := range files {
		for _, unit := range set.Spec[file] {
			content := false

			for _, variant := range unit.Variants {
				name := unit.FixtureName(variant)
				path := set.FixturePath(root, file, name)
				written = append(written, path)

				t.Run(name, func(t *testing.T) {
					component, ok := variant.Data.(templ.Component)
					if !ok {
						t.Fatalf("variant of %q in %s carries %T, not a templ.Component", unit.Name, file, variant.Data)
					}

					var rendered bytes.Buffer
					if err := component.Render(context.Background(), &rendered); err != nil {
						t.Fatalf("render %q from %s: %v", unit.Name, file, err)
					}

					if strings.TrimSpace(rendered.String()) != "" {
						content = true
					}

					golden.Compare(t, path, rendered.Bytes())
				})
			}

			if !content && !golden.Updating() {
				t.Errorf("component %q from %s rendered only whitespace in every variant; its fixture data does not reach the markup",
					unit.Name, file)
			}
		}
	}

	// A component deleted from the tree takes its plan entry with it. Its
	// fixture would stay on disk, counted as coverage no case produces.
	golden.AssertCoversDir(t, root, fleetGoldenSpecVar, written)
}
