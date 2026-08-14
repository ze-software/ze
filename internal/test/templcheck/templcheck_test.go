package templcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// typedFixture is the shape a ported package must have. Four parameter forms
// are accepted: a named struct, a slice of named structs, a scalar, and
// templ.Component.
const typedFixture = `package typed

import (
	"html/template"

	"github.com/a-h/templ"
)

type pageView struct {
	Title string
}

type row struct {
	Name string
}

type layoutView struct {
	pageView
	Rows    []row
	Content template.HTML
}

func page(v pageView) templ.Component { return nil }

func rows(rs []row, title string) templ.Component { return nil }

func layout(v layoutView, content templ.Component) templ.Component { return nil }
`

// escapeFixture holds one component per way of putting an unchecked value
// inside a templ component.
const escapeFixture = `package escapes

import (
	"github.com/a-h/templ"

	"example.com/other"
)

type viewData map[string]any

type rowList []viewData

type wrappedMap struct {
	Title string
	Data  map[string]any
}

type inner struct {
	Data map[string]any
}

type nestedMap struct {
	Title string
	Inner inner
}

type embeddedMap struct {
	inner
	Title string
}

type sliceOfStructs struct {
	Rows []inner
}

func rawMap(v map[string]any) templ.Component { return nil }

func wrappedInAStruct(v wrappedMap) templ.Component { return nil }

func nestedInAStruct(v nestedMap) templ.Component { return nil }

func embeddedInAStruct(v embeddedMap) templ.Component { return nil }

func sliceOfStructsWithAMap(v sliceOfStructs) templ.Component { return nil }

func anonymousStructWrapper(v struct{ Data map[string]any }) templ.Component { return nil }

func namedMap(v viewData) templ.Component { return nil }

func sliceOfNamedMaps(v rowList) templ.Component { return nil }

func pointerToMap(v *viewData) templ.Component { return nil }

func bareAny(v any) templ.Component { return nil }

func emptyInterface(v interface{}) templ.Component { return nil }

func foreignType(v other.View) templ.Component { return nil }

func undeclaredType(v missingType) templ.Component { return nil }
`

// writeFixture writes one generated-component file into a new directory and
// returns the directory. The fixture is built at run time on purpose. A
// committed one is a second copy of the rules under test, free to drift.
func writeFixture(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page_templ.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}

	return dir
}

// TestReportPassesTypedComponents pins the shape a ported package must have.
//
// VALIDATES: a component taking a named struct, a slice of named structs, a
// scalar or templ.Component is accepted.
// PREVENTS: a guard so strict that no package can satisfy it, which gets the
// guard deleted rather than obeyed.
func TestReportPassesTypedComponents(t *testing.T) {
	lines, err := Report(writeFixture(t, typedFixture), 3)
	if err != nil {
		t.Fatalf("read the typed fixture: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("the typed fixture was refused:\n  %s", strings.Join(lines, "\n  "))
	}
}

// TestReportRefusesEachEscape is the discrimination proof.
//
// VALIDATES: every spelling that puts an unchecked value inside a templ
// component is reported by the component's name.
// PREVENTS: the guard this replaces. It walked MapType, ArrayType and StarExpr
// only, so a component taking viewData over `type viewData map[string]any`
// passed, and so did a bare any. It also returned early on every struct, so a
// one-field wrapper around the map passed as well. Each is the failure AC-8
// exists to stop, and the wrapper is the cheapest port of a package whose
// handlers build map[string]any today.
func TestReportRefusesEachEscape(t *testing.T) {
	want := map[string]string{
		"rawMap":                 "a map",
		"namedMap":               "a map",
		"sliceOfNamedMaps":       "a map",
		"pointerToMap":           "a map",
		"wrappedInAStruct":       "field Data is a map",
		"nestedInAStruct":        "field Inner is a struct whose field Data is a map",
		"embeddedInAStruct":      "field inner is a struct whose field Data is a map",
		"sliceOfStructsWithAMap": "field Rows is a struct whose field Data is a map",
		"anonymousStructWrapper": "field Data is a map",
		"bareAny":                "an empty interface",
		"emptyInterface":         "an empty interface",
		"foreignType":            "another package",
		"undeclaredType":         "does not declare",
	}

	lines, err := Report(writeFixture(t, escapeFixture), len(want))
	if err != nil {
		t.Fatalf("read the escapes fixture: %v", err)
	}

	for component, reason := range want {
		found := false

		for _, line := range lines {
			if strings.Contains(line, component) && strings.Contains(line, reason) {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("%s was not refused for %q; the report held:\n  %s",
				component, reason, strings.Join(lines, "\n  "))
		}
	}
}

// TestReportRefusesAVacuousWalk pins the count.
//
// VALIDATES: a walk that reads a different number of components than the caller
// declared is reported, an empty walk included.
// PREVENTS: the check passing over zero files after a rename, a moved package,
// or a generator that did not run.
func TestReportRefusesAVacuousWalk(t *testing.T) {
	t.Run("no generated file", func(t *testing.T) {
		lines, err := Report(t.TempDir(), 1)
		if err != nil {
			t.Fatalf("read the empty fixture: %v", err)
		}

		if len(lines) != 1 || !strings.Contains(lines[0], "inspected 0 components") {
			t.Errorf("an empty walk was not reported: %v", lines)
		}
	})

	t.Run("wrong count", func(t *testing.T) {
		lines, err := Report(writeFixture(t, typedFixture), 99)
		if err != nil {
			t.Fatalf("read the typed fixture: %v", err)
		}

		if len(lines) != 1 || !strings.Contains(lines[0], "expected 99") {
			t.Errorf("a wrong count was not reported: %v", lines)
		}
	})
}

// TestReportFailsOnAMissingDirectory keeps the guard fail-closed.
//
// VALIDATES: a directory that cannot be read is an error, not an empty report.
// PREVENTS: a typo in the caller's path reading as a clean pass.
func TestReportFailsOnAMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "there-is-no-such-directory")

	if _, err := Report(missing, 1); err == nil {
		t.Error("a missing directory returned no error, so a typo would pass")
	}
}
