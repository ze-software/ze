// Design: docs/functional-tests.md -- native tooling fixtures over authored inputs.
// Related: ui_fixture_le_discovery_answers.go -- the existing real Git population proof.
package fixture

import (
	"os"
	"path/filepath"
	"testing"
)

// TestToolFixturesReachTheirProducer drives the registered generators through
// the supplied native le, then executes their generated composition root. The
// authored module has no external dependencies: no download or repository build
// can substitute for the registration and embedded-module behavior being tested.
func TestToolFixturesReachTheirProducer(t *testing.T) {
	binary, err := nativeLEBinary()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := leDiscoveryWriteFiles(root, toolGeneratorSources()); err != nil {
		t.Fatal(err)
	}
	if err := leFixtureStdpatch(root); err != nil {
		t.Fatal(err)
	}
	productEnv := childEnvironment(os.Environ(), map[string]string{
		"ZE_REPO_ROOT":      root,
		"CLAUDE_PROJECT_DIR": root,
		"GOPROXY":           "off",
		"GOSUMDB":           "off",
		"GOTOOLCHAIN":       "local",
		"GOWORK":            "off",
		"GOFLAGS":           "",
		"GOENV":             "off",
		"CGO_ENABLED":       "0",
	})
	run := func(args ...string) uiLeCodegenAnswersCommandResult {
		return uiLeCodegenAnswersRunCommand(t.Context(), root, productEnv, binary, args...)
	}
	for _, area := range []string{"yang glue", "plugin imports"} {
		before := run(area, "check")
		if before.exitCode() != 1 {
			t.Fatalf("%s check before generation exited %d, want stale verdict 1: %s", area, before.exitCode(), before.output())
		}
		updated := run(area, "update")
		if updated.exitCode() != 0 {
			t.Fatalf("%s update exited %d: %s", area, updated.exitCode(), updated.output())
		}
		checked := run(area, "check")
		if checked.exitCode() != 0 {
			t.Fatalf("%s check after generation exited %d: %s", area, checked.exitCode(), checked.output())
		}
	}
	toolGeneratorRunProgram(t, root, productEnv)

	// A newly authored nested plugin and schema must participate without an
	// edit to either the composition root or the fixture's expected count.
	if err := leDiscoveryWriteFiles(root, map[string]string{
		"internal/component/ike/plugins/added/register.go": "package added\nimport \"example.test/native-generators/internal/observed\"\nfunc init() { observed.Plugins[\"added\"] = true }\n",
		"internal/component/ike/plugins/added/yang/ze-added.yang": "module ze-added { namespace \"urn:added\"; prefix added; import ze-base { prefix base; } }\n",
		"cmd/probe/added.go": "package main\nfunc init() { wantPlugins[\"added\"] = true; wantModules[\"ze-added.yang\"] = \"module ze-added { namespace \\\"urn:added\\\"; prefix added; import ze-base { prefix base; } }\\n\" }\n",
	}); err != nil {
		t.Fatal(err)
	}
	for _, area := range []string{"yang glue", "plugin imports"} {
		updated := run(area, "update")
		if updated.exitCode() != 0 {
			t.Fatalf("%s update after adding a member exited %d: %s", area, updated.exitCode(), updated.output())
		}
		checked := run(area, "check")
		if checked.exitCode() != 0 {
			t.Fatalf("%s check after adding a member exited %d: %s", area, checked.exitCode(), checked.output())
		}
	}
	toolGeneratorRunProgram(t, root, productEnv)
}

func toolGeneratorRunProgram(t *testing.T, root string, environment []string) {
	t.Helper()
	result := uiLeCodegenAnswersRunCommand(t.Context(), root, environment, "go", "run", "-mod=readonly", "./cmd/probe")
	if result.exitCode() != 0 {
		t.Fatalf("generated module did not register its authored plugins and exact embedded YANG: %s", result.output())
	}
	if result.stdout != "registered authored plugins and exact YANG modules\n" {
		t.Fatalf("generated program returned %q", result.stdout)
	}
}

func toolGeneratorSources() map[string]string {
	return map[string]string{
		"go.mod": "module example.test/native-generators\n\ngo 1.26\n",
		"feature-gates.txt": "",
		"internal/observed/state.go": "package observed\nvar Plugins = map[string]bool{}\nvar Modules = map[string]string{}\n",
		"internal/component/config/yang/register.go": "package yang\nimport \"example.test/native-generators/internal/observed\"\nfunc RegisterModule(name, body string) { observed.Modules[name] = body }\n",
		"internal/plugins/sample/register.go": "package sample\nimport \"example.test/native-generators/internal/observed\"\nfunc init() { observed.Plugins[\"sample\"] = true }\n",
		"internal/plugins/sample/yang/ze-sample.yang": "module ze-sample { namespace \"urn:sample\"; prefix sample; import ze-base { prefix base; } }\n",
		"internal/component/base/yang/ze-base.yang": "module ze-base { namespace \"urn:base\"; prefix base; }\n",
		"cmd/probe/main.go": `package main
import (
    "fmt"
    "os"
    "reflect"
    "example.test/native-generators/internal/observed"
    _ "example.test/native-generators/internal/component/plugin/all"
)
var wantPlugins = map[string]bool{"sample": true}
var wantModules = map[string]string{
    "ze-base.yang": "module ze-base { namespace \"urn:base\"; prefix base; }\n",
    "ze-sample.yang": "module ze-sample { namespace \"urn:sample\"; prefix sample; import ze-base { prefix base; } }\n",
}
func main() {
    if !reflect.DeepEqual(observed.Plugins, wantPlugins) {
        fmt.Fprintf(os.Stderr, "registered plugins = %v, want %v\n", observed.Plugins, wantPlugins)
        os.Exit(1)
    }
    if !reflect.DeepEqual(observed.Modules, wantModules) {
        fmt.Fprintf(os.Stderr, "registered modules = %q, want %q\n", observed.Modules, wantModules)
        os.Exit(1)
    }
    fmt.Println("registered authored plugins and exact YANG modules")
}
`,
		"internal/component/plugin/all/.keep": "",
	}
}

// TestRulesFixtureRootAliasesReachOnlyOwnedTree drives the registered renderer
// with the rules fixture's child environment. Every inherited spelling of its
// root must yield to the selected fixture; a successful page cannot substitute
// for the authored rule bytes or authorize a write in the foreign tree.
func TestRulesFixtureRootAliasesReachOnlyOwnedTree(t *testing.T) {
	binary, err := nativeLEBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"ze.repo.root", "Ze_RePo_RoOt", "ZE.REPO.ROOT"} {
		t.Run(alias, func(t *testing.T) {
			const oldRule = "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n## S\n\n- **MUST do the OLD thing.**\n"
			const newRule = "# A\n\n**When:** when it happens\n**Severity:** blocking\n\n## S\n\n- **MUST do the NEW thing.**\n"
			files := map[string]string{
				"go.mod":                         "module example.com/fixture\n",
				"feature-gates.txt":              contentCoreFeatureGate,
				"ai/rules/aaa.md":                oldRule,
				"ai/rules/points/aaa/manifest.md": "---\ntitle: A\nwhen: when it happens\nseverity: blocking\n---\ns ## S\n  p\n",
				"ai/rules/points/aaa/s/p.md":      "---\nkind: directive\nlevel: MUST\n---\n- **MUST do the NEW thing.**\n",
			}
			owned, foreign := t.TempDir(), t.TempDir()
			for _, root := range []string{owned, foreign} {
				if err := writeFixture(root, files); err != nil {
					t.Fatal(err)
				}
			}
			inherited := append(os.Environ(), alias+"="+foreign)
			environment := uiLeRulesAnswersSetEnv(inherited, "ZE_REPO_ROOT", owned)
			result, err := uiLeRulesAnswersExecute(t.Context(), owned, environment, binary,
				areaAIRules, actionUpdate, rulesTargetRender)
			if err != nil {
				t.Fatal(err)
			}
			if err := requireFile(filepath.Join(foreign, "ai", "rules", "aaa.md"), oldRule,
				"rules renderer wrote through an inherited root alias"); err != nil {
				t.Error(err)
			}
			if result.code != 0 {
				t.Fatalf("registered rules renderer exited %d: %s%s", result.code, result.stdout, result.stderr)
			}
			if err := requireFile(filepath.Join(owned, "ai", "rules", "aaa.md"), newRule,
				"rules renderer did not update the selected fixture"); err != nil {
				t.Error(err)
			}
		})
	}
}
