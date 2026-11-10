// Design: docs/architecture/plugin/plugin-system.md -- registration closes when main starts
package pluginregistrationsites

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// fixtureRegistry is a registry package with the seal guard, two registering
// functions that reach it and a reader that does not.
const fixtureRegistry = `package registry
type Registration struct { Name string }
func refuseSealedLocked(kind, name string) error { return nil }
func panicIfSealedLocked(fn, kind, argument string) { _ = refuseSealedLocked(kind, argument) }
func Register(reg Registration) error { return refuseSealedLocked("plugin", reg.Name) }
func AddConfigRead(name, root string) error { return refuseSealedLocked(name, root) }
func Lookup(name string) *Registration { return nil }
`

// registryLine is the import line of every fixture plugin.
const registryLine = `import "github.com/ze-software/ze/internal/component/plugin/registry"` + "\n"

// committedSites commits a minimal Ze tree holding files over the base
// population, reads it through repoinventory.ReadCommitted and judges it.
func committedSites(t *testing.T, files map[string]string) Report {
	t.Helper()
	root := t.TempDir()
	tree := map[string]string{
		"go.mod":                               "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt":                    "ze_example internal/plugins/example\n",
		"cmd/ze/main.go":                       "//go:build ze_le\npackage main\nimport _ \"github.com/ze-software/ze/internal/component/plugin/all\"\nfunc main() {}\n",
		"internal/component/plugin/all/all.go": "package all\nimport _ \"github.com/ze-software/ze/internal/plugins/example\"\n",
		"internal/component/plugin/registry/registry.go": fixtureRegistry,
	}
	maps.Copy(tree, files)
	for name, body := range tree {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "--initial-branch=main"},
		{"add", "."},
		{"-c", "user.name=Sites Fixture", "-c", "user.email=sites@example.invalid", "commit", "-m", "fixture"},
	} {
		command := exec.CommandContext(t.Context(), "git", args...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	snapshot, err := repoinventory.ReadCommitted(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Judge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

// lineOf answers the 1-based line of body holding marker.
func lineOf(t *testing.T, body, marker string) int {
	t.Helper()
	for index, line := range strings.Split(body, "\n") {
		if strings.Contains(line, marker) {
			return index + 1
		}
	}
	t.Fatalf("fixture has no line holding %q", marker)
	return 0
}

// TestRegistrationSitesCheckAcceptsTheCommittedTree runs the check over the
// committed HEAD of this checkout (AC-7): it passes, reads Ze packages, and
// accepts the registry.Register sites of every plugin.
func TestRegistrationSitesCheckAcceptsTheCommittedTree(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	report, err := Check(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Refused) != 0 {
		t.Fatalf("committed tree breaks the registration rule:\n%s", report.Text())
	}
	registers := 0
	for _, site := range report.Accepted {
		if site.Function == "registry.Register" {
			registers++
		}
	}
	// Every plugin the committed Linux le source declares registers once, so a
	// count that differs means the check stopped finding sites, or counted one
	// twice, rather than that they became correct. The declared set is the
	// inventory's own reading of the same tree, never a literal that drifts.
	declared, err := repoinventory.DeclaredPlugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) == 0 {
		t.Fatal("the inventory declared no plugin, so the count proves nothing")
	}
	if report.Packages < len(declared) || registers != len(declared) {
		t.Fatalf("check read %d packages and accepted %d registry.Register sites; the tree declares %d plugins", report.Packages, registers, len(declared))
	}
	if !slices.Contains(report.Registering, "Register") || slices.Contains(report.Registering, "Lookup") {
		t.Fatalf("registering functions derived as %v", report.Registering)
	}
}

// TestRegistrationSitesRefusesEachWrongSite gives the check one wrong site per
// fixture (AC-8) and requires a refusal at its file and line, naming the
// registering function and the rule it breaks, with the repair command. The
// aliased-import fixture is the one that passes.
func TestRegistrationSitesRefusesEachWrongSite(t *testing.T) {
	const plugin = "internal/plugins/example/"
	cases := []struct {
		name, file, body, rule, function string
	}{
		{
			name: "file not named register.go", file: plugin + "plugin.go",
			body:     "package example\n" + registryLine + "func init() { _ = registry.Register(registry.Registration{Name: \"x\"}) } // site\n",
			rule:     "outside a register*.go file",
			function: "registry.Register",
		},
		{
			name: "reached from a function the engine runs", file: plugin + "register.go",
			body: "package example\n" + registryLine +
				"func late() { _ = registry.Register(registry.Registration{Name: \"late\"}) } // site\n" +
				"func run() { late() }\nvar engine = run\n",
			rule:     "run is used as a value",
			function: "registry.Register",
		},
		{
			name: "inside a function literal", file: plugin + "register.go",
			body: "package example\n" + registryLine +
				"func init() {\n register := func() { _ = registry.Register(registry.Registration{Name: \"x\"}) } // site\n register()\n}\n",
			rule:     "inside a function literal",
			function: "registry.Register",
		},
		{
			name: "assigned as a value", file: plugin + "register.go",
			body:     "package example\n" + registryLine + "var register = registry.Register // site\n",
			rule:     "used as a value",
			function: "registry.Register",
		},
		{
			name: "passed as a value", file: plugin + "register.go",
			body: "package example\n" + registryLine +
				"func use(func(registry.Registration) error) {}\nfunc init() { use(registry.Register) } // site\n",
			rule:     "used as a value",
			function: "registry.Register",
		},
		{
			name: "returned as a value", file: plugin + "register.go",
			body: "package example\n" + registryLine +
				"func get() func(registry.Registration) error { return registry.Register } // site\nfunc init() { _ = get() }\n",
			rule:     "used as a value",
			function: "registry.Register",
		},
		{
			name: "inside a method", file: plugin + "register.go",
			body: "package example\n" + registryLine +
				"type holder struct{}\nfunc (holder) register() { _ = registry.Register(registry.Registration{Name: \"x\"}) } // site\nfunc init() { holder{}.register() }\n",
			rule:     "in method register",
			function: "registry.Register",
		},
		{
			name: "registry dot-imported", file: plugin + "register.go",
			body: "package example\n" +
				"import . \"github.com/ze-software/ze/internal/component/plugin/registry\" // site\n" +
				"func init() { _ = Register(Registration{Name: \"x\"}) }\n",
			rule:     "dot-imported",
			function: "registry (dot import)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := committedSites(t, map[string]string{tc.file: tc.body})
			line := lineOf(t, tc.body, "// site")
			index := slices.IndexFunc(report.Refused, func(refusal Refusal) bool {
				return refusal.File == tc.file && refusal.Line == line
			})
			if index < 0 {
				t.Fatalf("no refusal at %s:%d; report:\n%s", tc.file, line, report.Text())
			}
			refusal := report.Refused[index]
			if refusal.Function != tc.function || !strings.Contains(refusal.Rule, tc.rule) || !strings.Contains(refusal.Fix, ruleCommand) {
				t.Fatalf("refusal %+v, want function %q, rule holding %q, fix naming %q", refusal, tc.function, tc.rule, ruleCommand)
			}
		})
	}
	t.Run("aliased import used correctly", func(t *testing.T) {
		body := "package example\nimport reg \"github.com/ze-software/ze/internal/component/plugin/registry\"\n" +
			"func init() { _ = reg.Register(reg.Registration{Name: \"example\"}) } // site\n"
		report := committedSites(t, map[string]string{plugin + "register.go": body})
		want := Site{Package: "internal/plugins/example", File: plugin + "register.go", Line: lineOf(t, body, "// site"), Function: "registry.Register", Within: "init"}
		if len(report.Refused) != 0 || !slices.Equal(report.Accepted, []Site{want}) {
			t.Fatalf("aliased import: report\n%s\nwant one accepted site %+v", report.Text(), want)
		}
	})
}

// TestRegistrationSitesAcceptsInitializationChains proves the helper half of
// the rule (R-1): a helper only init() calls, a package-level initializer, and
// a chain through another package's exported function that only register
// files' init() call are accepted; the same chain refuses once the exported
// function is also called at run time.
func TestRegistrationSitesAcceptsInitializationChains(t *testing.T) {
	library := map[string]string{
		"internal/lib/lib.go":      "package lib\nfunc MustAdd(root string) { add(root) }\n",
		"internal/lib/register.go": "package lib\n" + registryLine + "func add(root string) { _ = registry.AddConfigRead(\"lib\", root) } // chain\n",
	}
	plugin := "package example\n" + registryLine + "import \"github.com/ze-software/ze/internal/lib\"\n" +
		"func init() { registerExample(); lib.MustAdd(\"root\") }\n" +
		"func registerExample() { _ = registry.Register(registry.Registration{Name: \"example\"}) }\n" +
		"var _ = registerBuiltin()\nfunc registerBuiltin() bool { _ = registry.Register(registry.Registration{Name: \"builtin\"}); return true }\n"
	files := maps.Clone(library)
	files["internal/plugins/example/register.go"] = plugin
	report := committedSites(t, files)
	if len(report.Refused) != 0 || len(report.Accepted) != 3 {
		t.Fatalf("initialization chains: want three accepted sites, got\n%s", report.Text())
	}

	files["internal/plugins/example/serve.go"] = "package example\nimport \"github.com/ze-software/ze/internal/lib\"\nfunc serve() { lib.MustAdd(\"late\") } // runtime\nvar engine = serve\n"
	report = committedSites(t, files)
	index := slices.IndexFunc(report.Refused, func(refusal Refusal) bool {
		return refusal.File == "internal/lib/register.go" && strings.Contains(refusal.Rule, "serve is used as a value")
	})
	if index < 0 {
		t.Fatalf("a chain also reached at run time must refuse at its registering call; got\n%s", report.Text())
	}
}

// TestRegistrationSitesDerivesTheMutatorSet adds a mutator to the fixture
// registry that reaches the seal guard (AC-9): with this check's source
// unchanged, its init() call is accepted, its run-time call refused, and a
// registry reader is never judged.
func TestRegistrationSitesDerivesTheMutatorSet(t *testing.T) {
	plugin := "package example\nimport reg \"github.com/ze-software/ze/internal/component/plugin/registry\"\n" +
		"func init() { reg.AddWidget(\"w\") } // accepted\n" +
		"func serve() { reg.AddWidget(\"late\"); _ = reg.Lookup(\"x\") } // refused\nvar engine = serve\n"
	report := committedSites(t, map[string]string{
		"internal/component/plugin/registry/widget.go": "package registry\nfunc AddWidget(name string) { panicIfSealedLocked(\"AddWidget\", \"widget\", name) }\n",
		"internal/plugins/example/register.go":         plugin,
	})
	if want := []string{"AddConfigRead", "AddWidget", "Register"}; !slices.Equal(report.Registering, want) {
		t.Fatalf("registering functions = %v, want %v", report.Registering, want)
	}
	file := "internal/plugins/example/register.go"
	accepted := Site{Package: "internal/plugins/example", File: file, Line: lineOf(t, plugin, "// accepted"), Function: "registry.AddWidget", Within: "init"}
	if !slices.Equal(report.Accepted, []Site{accepted}) {
		t.Fatalf("accepted = %+v, want %+v", report.Accepted, accepted)
	}
	if len(report.Refused) != 1 || report.Refused[0].Line != lineOf(t, plugin, "// refused") || report.Refused[0].Function != "registry.AddWidget" {
		t.Fatalf("want exactly the run-time AddWidget call refused, got\n%s", report.Text())
	}
}
