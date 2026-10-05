// Design: docs/contributing/rfc-conformance-gates.md -- compile admissibility.

package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckCompileRejectsChangedTaggedPackage runs the public gate against a
// tiny module, then adds a type error behind its feature gate and repairs it.
// Each invocation must judge current source with the real compiler; a previous
// green answer and a gated-out broken file must never admit unexecutable tags.
// Invalid inherited Go settings make this fail if the child loses its derived
// toolchain environment, without a fake compiler or environment echo program.
func TestCheckCompileRejectsChangedTaggedPackage(t *testing.T) {
	root := checkFixtureTree(t, gapFixtureFiles(true, gapFixtureTest(selftestRIDSend)))
	t.Setenv("GOTOOLCHAIN", "invalid-toolchain")
	t.Setenv("GOCACHE", filepath.Join(root, "go.mod"))
	t.Setenv("CGO_ENABLED", "invalid")
	for _, one := range []struct {
		name   string
		source string
		code   int
	}{
		{name: "compiles", source: "var compileWitness int = 1\n", code: 0},
		{name: "broken", source: "var compileWitness int = \"not-an-integer\"\n", code: 2},
		{name: "repaired", source: "var compileWitness int = 2\n", code: 0},
	} {
		t.Run(one.name, func(t *testing.T) {
			if err := writeSelftestFiles(root, map[string]string{
				"cmd/widget/compile.go": "//go:build ze_widget\n\npackage widget\n\n" + one.source,
			}); err != nil {
				t.Fatal(err)
			}
			report, code := Check(root, nil)
			if report.CannotRun != "" {
				t.Fatalf("compiler fixture cannot run: %s", report.CannotRun)
			}
			if code != one.code {
				t.Fatalf("compile fixture returned %d, want %d:\n%s", code, one.code, report.Text())
			}
			if one.code == 0 {
				if len(report.Violations) != 0 {
					t.Fatalf("valid source has violations: %s", report.Text())
				}
				return
			}
			if violationWith(&report, "cmd/widget", "`go vet` cannot type-check", selftestRIDSend, "not-an-integer") == "" {
				t.Fatalf("compile refusal omits package, tagged requirement or compiler error:\n%s", report.Text())
			}
		})
	}
}

// TestCheckCompileRequiresCompleteToolchain drives the public check after
// removing or corrupting its build metadata. It must refuse the observation,
// not admit tags checked under a smaller product or an ambient toolchain.
func TestCheckCompileRequiresCompleteToolchain(t *testing.T) {
	for _, one := range []struct {
		name string
		path string
		body string
	}{
		{name: "missing manifest", path: "feature-gates.txt"},
		{name: "missing module", path: "go.mod"},
		{name: "empty gates", path: "feature-gates.txt", body: "# no feature gates\n"},
	} {
		t.Run(one.name, func(t *testing.T) {
			root := checkFixtureTree(t, gapFixtureFiles(true, gapFixtureTest(selftestRIDSend)))
			if one.body == "" {
				if err := os.Remove(filepath.Join(root, one.path)); err != nil {
					t.Fatal(err)
				}
			} else if err := writeSelftestFiles(root, map[string]string{one.path: one.body}); err != nil {
				t.Fatal(err)
			}
			report, code := Check(root, nil)
			if code != 2 {
				t.Fatalf("incomplete toolchain answered %d, want 2:\n%s", code, report.Text())
			}
			if !strings.Contains(report.CannotRun, "derive native Go test toolchain") {
				t.Fatalf("incomplete toolchain was not refused as unreadable:\n%s", report.Text())
			}
		})
	}
}
