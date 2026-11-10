// VALIDATES: spec-rea-practices-2-error-inventory AC-8. A change-set check
// that `./le commit create` runs is discovered from its changecheck.Register
// call, and every `// ze point:` binding in its package joins the gate map
// under the check's registered name.
// PREVENTS: a change-set check's binding being dead text that no gate reads,
// and a hand list of the packages that register one.

package airules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// demoRegister registers a change-set check the way the arch tools do: the
// name is a package constant joined to a literal, and the import is renamed
// so the discovery cannot rely on the default package name.
const demoRegister = `package archdemo

import seam "github.com/ze-software/ze/internal/le/commit/changecheck"

func init() {
	seam.Register(seam.Check{Name: area + " check", Judge: judgeChange})
}
`

// demoActions holds the constant in a file that neither imports the seam nor
// carries a binding, so the name is resolved across the package.
const demoActions = `package archdemo

const area = "arch demo"
`

// demoReport carries the binding above a method, where the hook join would
// find no top-level function.
const demoReport = `package archdemo

type report struct{}

// ze point: alpha/section/third
//
// Text renders the refusal.
func (report) Text() string { return "" }
`

// writeTree writes root-relative slash paths into a fresh root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	return root
}

const pointBody = "---\nkind: directive\nlevel: MUST\nstage:\n---\n- **A thing MUST hold.**\n"

// demoTree is a checkout holding the native hook fixture, one change-set
// check package, the points both name, and a Go file under internal/le that
// imports the seam to run the checks and registers none.
func demoTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		hookRuntimeRel + "/runtime.go":         nativeSource,
		"internal/le/arch/demo/register.go":    demoRegister,
		"internal/le/arch/demo/actions.go":     demoActions,
		"internal/le/arch/demo/report.go":      demoReport,
		"internal/le/arch/demo/report_test.go": "package archdemo\n// ze point: alpha/section/missing\n",
		"internal/le/commit/run.go":            "package commit\n\nimport \"github.com/ze-software/ze/internal/le/commit/changecheck\"\n\nvar _ = changecheck.Registered\n",
		pointsRel + "/alpha/" + manifestName:   "# alpha\n",
		pointsRel + "/alpha/section/first.md":  pointBody,
		pointsRel + "/alpha/section/second.md": pointBody,
		pointsRel + "/alpha/section/third.md":  pointBody,
	})
}

// TestAChangeSetCheckBindingIsListedUnderItsCheckName builds the gate map over
// a tree with one change-set check and reads the GATED line of the page.
func TestAChangeSetCheckBindingIsListedUnderItsCheckName(t *testing.T) {
	root := demoTree(t)
	hooks, problems := nativeHookSources(root)
	if len(problems) != 0 {
		t.Fatalf("native hook roster: %v", problems)
	}
	sources, problems := changeSetSources(root)
	if len(problems) != 0 {
		t.Fatalf("change-set sources: %v", problems)
	}
	roster, problems := readChangeSetRoster(sources)
	if len(problems) != 0 {
		t.Fatalf("change-set roster: %v", problems)
	}
	gm, err := buildGateMap(hooks, roster.bindings, filepath.Join(root, filepath.FromSlash(pointsRel)), root)
	if err != nil {
		t.Fatalf("buildGateMap: %v", err)
	}
	if len(gm.Dangling) != 0 {
		t.Fatalf("dangling: %+v", gm.Dangling)
	}
	report := fillCoverage(gm, false)
	page := report.Text()
	if !strings.Contains(page, "alpha/section/third  <- arch demo check\n") {
		t.Fatalf("the change-set check is not listed under its name:\n%s", page)
	}
	if !strings.Contains(page, "gate map: 3 points, 5 bindings, 4 checks\n") {
		t.Fatalf("the counts line does not count the change-set check:\n%s", page)
	}
	for _, binding := range gm.Bindings {
		if strings.HasSuffix(binding.File, "_test.go") {
			t.Errorf("a test file's binding joined the map: %+v", binding)
		}
	}

	// The published hook tables owe no row for a change-set check: its
	// binding sits in no hookruntime source.
	doc := "## Runtime (`runtime.go`)\n\n" + tableHead + " Triggers on |\n" +
		"|---|---|---|\n" +
		"| `firstCheck` | `alpha.md` | always |\n" +
		"| `secondCheck` | `alpha.md` | always |\n" +
		"| `unboundCheck` | no rule | always |\n"
	if problems := hookTableProblems(gm, doc, hooks); len(problems) != 0 {
		t.Fatalf("a change-set binding was judged against the hook tables: %v", problems)
	}
}

// TestChangeSetDiscoveryFailsClosed names each shape that would make the
// attribution a guess, and requires a refusal for it.
func TestChangeSetDiscoveryFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		problem string
	}{
		{
			"registered check with no binding",
			map[string]string{
				"internal/le/arch/demo/register.go": demoRegister,
				"internal/le/arch/demo/actions.go":  demoActions,
			},
			"arch demo check has no `// ze point:` binding",
		},
		{
			"name not a constant",
			map[string]string{
				"internal/le/arch/demo/register.go": strings.Replace(demoRegister, "area + \" check\"", "name()", 1),
				"internal/le/arch/demo/report.go":   demoReport,
			},
			"check name is not a constant string",
		},
		{
			"two checks in one package",
			map[string]string{
				"internal/le/arch/demo/register.go": strings.Replace(demoRegister,
					"func init() {\n", "func init() {\n\tseam.Register(seam.Check{Name: \"other\", Judge: judgeChange})\n", 1),
				"internal/le/arch/demo/actions.go": demoActions,
				"internal/le/arch/demo/report.go":  demoReport,
			},
			"registers 2 change-set checks",
		},
		{
			"one name in two packages",
			map[string]string{
				"internal/le/arch/demo/register.go":  demoRegister,
				"internal/le/arch/demo/actions.go":   demoActions,
				"internal/le/arch/demo/report.go":    demoReport,
				"internal/le/arch/again/register.go": strings.Replace(demoRegister, "archdemo", "archagain", 1),
				"internal/le/arch/again/actions.go":  strings.Replace(demoActions, "archdemo", "archagain", 1),
				"internal/le/arch/again/report.go":   strings.Replace(demoReport, "archdemo", "archagain", 1),
			},
			"is also registered in",
		},
		{
			"Go cannot be parsed",
			map[string]string{
				"internal/le/arch/demo/register.go": demoRegister + "\nfunc broken(\n",
			},
			"cannot be parsed",
		},
	}
	for _, tc := range cases {
		root := writeTree(t, tc.files)
		sources, problems := changeSetSources(root)
		if len(problems) == 0 {
			var roster changeSetRoster
			roster, problems = readChangeSetRoster(sources)
			problems = append(problems, roster.unbound()...)
		}
		if !containsString(problems, tc.problem) {
			t.Errorf("%s: problems %v do not mention %q", tc.name, problems, tc.problem)
		}
	}
}

// TestABindingInAPackageThatRegistersNoCheckDangles proves a `// ze point:`
// comment under internal/le that no registered check owns is reported rather
// than read as gating anything.
func TestABindingInAPackageThatRegistersNoCheckDangles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"internal/le/tool/tool.go": "package tool\n\n// ze point: alpha/section/first\nfunc Run() {}\n",
	})
	sources, problems := changeSetSources(root)
	if len(problems) != 0 {
		t.Fatalf("change-set sources: %v", problems)
	}
	roster, problems := readChangeSetRoster(sources)
	if len(problems) != 0 {
		t.Fatalf("change-set roster: %v", problems)
	}
	checks := roster.bindings
	if len(checks) != 1 || checks[0].Check != noCheck || checks[0].File != "internal/le/tool/tool.go" {
		t.Fatalf("the orphan binding is %+v, want one %s binding in internal/le/tool/tool.go", checks, noCheck)
	}
}

// TestTheErrorInventoryCheckIsListedInTheGateMap reads the real checkout:
// the three change-set checks bind the cause-and-fix directive, and the gate
// map lists each under the name `./le commit create` refuses with.
func TestTheErrorInventoryCheckIsListedInTheGateMap(t *testing.T) {
	root := filepath.Clean("../../../..")
	sources, problems := changeSetSources(root)
	if len(problems) != 0 {
		t.Fatalf("change-set sources: %v", problems)
	}
	roster, problems := readChangeSetRoster(sources)
	if len(problems) != 0 {
		t.Fatalf("change-set roster: %v", problems)
	}
	if problems := roster.unbound(); len(problems) != 0 {
		t.Fatalf("change-set checks with no binding: %v", problems)
	}
	checks := roster.bindings
	const ref = "user-facing-errors/directives/an-error-names-the-cause-and-the-fix"
	named := map[string]bool{}
	for _, binding := range checks {
		if binding.Ref == ref {
			named[binding.Check] = true
		}
	}
	for _, check := range []string{"arch error-inventory check", "arch operator-error check", "arch doctor-coverage check"} {
		if !named[check] {
			t.Errorf("%s is not bound to %s: %+v", check, ref, checks)
		}
	}
}
