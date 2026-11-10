// Design: docs/functional-tests.md -- native tooling fixtures over authored inputs.
// Related: ui_fixture_le_discovery_answers.go -- the existing real Git population proof.
package fixture

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/test/runner"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestPersistenceFixturesUseOneEffectiveStore proves that a fixture which
// seeds or reads the store a ze daemon persists shares that one store with
// the daemon, and that a memory-only daemon environment keeps its isolation.
// Method: requireDaemonRoot and the openers built on it are judged over
// environments written in every spelling internal/core/env reads. The root
// they accept is compared with paths.DefaultConfigDir run over the same
// environment, which is how the daemon resolves its store, and
// daemonEnvironment is checked to build what the openers accept for the disk
// and to keep the memory selector for a memory-only daemon.
func TestPersistenceFixturesUseOneEffectiveStore(t *testing.T) {
	dir := t.TempDir()
	operator := t.TempDir()

	// MUTATION: match the selector by its exact dotted name, or strip only
	// the uppercase alias, and the other spellings below are accepted and a
	// store is created that the memory daemon never sees.
	for _, selector := range []string{"ze.test.storage", "ZE_TEST_STORAGE", "ze_test_storage", "Ze.Test-Storage"} {
		environment := []string{envConfigDirDotted + "=" + dir, selector + "=" + runner.DUTStoreMemory}
		err := requireDaemonRoot(environment, dir)
		if err == nil || !strings.Contains(err.Error(), "option=storage:value=disk") {
			t.Errorf("%s=memory: err = %v, want a refusal naming option=storage:value=disk", selector, err)
		}
		if store, err := createDaemonStore(environment, dir); err == nil {
			store.Close() //nolint:errcheck // the store should not exist
			t.Errorf("%s=memory: createDaemonStore created a store, want a refusal", selector)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("a refused open touched %s: entries %v, err %v", dir, entries, err)
	}

	// MUTATION: fall back to paths.DefaultConfigDir's binary-relative root
	// when the environment names none, and this row opens the operator store.
	if err := requireDaemonRoot([]string{}, dir); err == nil || !strings.Contains(err.Error(), envConfigDirDotted) {
		t.Errorf("no root: err = %v, want a refusal naming %s", err, envConfigDirDotted)
	}

	// MUTATION: read only one spelling of ze.config.dir, or let the last entry
	// win whatever its dots, and a row opens a root the daemon does not use.
	rows := []struct {
		name        string
		environment []string
	}{
		{"dotted", []string{"ze.config.dir=" + dir}},
		{"uppercase", []string{"ZE_CONFIG_DIR=" + dir}},
		{"uppercase after dotted", []string{"ze.config.dir=" + dir, "ZE_CONFIG_DIR=" + operator}},
		{"dotted after uppercase", []string{"ZE_CONFIG_DIR=" + operator, "ze.config.dir=" + dir}},
	}
	for _, row := range rows {
		if daemonRoot := daemonConfigDirUnder(t, row.environment); daemonRoot != dir {
			t.Fatalf("%s: the daemon resolves %q, want %q: the rows no longer describe the daemon", row.name, daemonRoot, dir)
		}
		if err := requireDaemonRoot(row.environment, dir); err != nil {
			t.Errorf("%s: the daemon's root %s refused: %v", row.name, dir, err)
		}
		if err := requireDaemonRoot(row.environment, operator); err == nil {
			t.Errorf("%s: %s accepted, but the daemon roots its store at %s", row.name, operator, dir)
		}
	}

	// The runner hands every child its root and the memory selector dotted,
	// and an option=env may add the uppercase spellings beside them.
	inherited := []string{
		"ZE_CONFIG_DIR=" + operator, "ze.config.dir=" + operator,
		"ZE_TEST_STORAGE=memory", "ze.test.storage=memory",
	}

	// MUTATION: drop withoutEnv from the disk arm of daemonEnvironment, and
	// the inherited selector survives: the seed below is refused.
	disk := daemonEnvironment(inherited, dir, daemonStoreDisk)
	if got := env.LookupIn(disk, runner.DUTStoreKey); got != "" {
		t.Errorf("disk environment selects store %q, want none", got)
	}
	if daemonRoot := daemonConfigDirUnder(t, disk); daemonRoot != dir {
		t.Errorf("disk environment roots the daemon at %q, want %q", daemonRoot, dir)
	}
	seed := []byte("seeded by the fixture\n")
	store, err := createDaemonStore(disk, dir)
	if err != nil {
		t.Fatalf("seed the daemon store: %v", err)
	}
	if err := store.WriteFile(zefs.KeyWebCert.Pattern, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := openDaemonStoreReadOnly(disk, dir)
	if err != nil {
		t.Fatalf("read the daemon store back: %v", err)
	}
	got, err := reader.ReadFile(zefs.KeyWebCert.Pattern)
	reader.Close() //nolint:errcheck // read-only handle
	if err != nil || !bytes.Equal(got, seed) {
		t.Errorf("read back %q, %v; want the seed %q", got, err, seed)
	}
	if entries, err := os.ReadDir(operator); err != nil || len(entries) != 0 {
		t.Errorf("the inherited root %s was written: entries %v, err %v", operator, entries, err)
	}

	// The memory-only control: a fixture whose daemon persists nothing keeps
	// the selector, and no opener will read that daemon's store.
	// MUTATION: build the memory arm like the disk arm, and the selector is gone.
	memory := daemonEnvironment(inherited, dir, daemonStoreMemory)
	if got := env.LookupIn(memory, runner.DUTStoreKey); got != runner.DUTStoreMemory {
		t.Errorf("memory environment selects store %q, want %q", got, runner.DUTStoreMemory)
	}
	if daemonRoot := daemonConfigDirUnder(t, memory); daemonRoot != dir {
		t.Errorf("memory environment roots the daemon at %q, want %q", daemonRoot, dir)
	}
	if err := requireDaemonRoot(memory, dir); err == nil {
		t.Error("a memory daemon's store was accepted for reading")
	}
}

// daemonConfigDirUnder answers what paths.DefaultConfigDir, the daemon's own
// resolution, reads in a process whose environment carries the ze entries of
// environment. It replaces every spelling of the root in this process first,
// and restores them when the test ends.
func daemonConfigDirUnder(t *testing.T, environment []string) string {
	t.Helper()
	for _, name := range []string{"ze.config.dir", "ZE_CONFIG_DIR", "ze_config_dir"} {
		t.Setenv(name, "")
		os.Unsetenv(name) //nolint:errcheck // t.Setenv restores it
	}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		if environmentKeyReading(name) == environmentKeyReading(envConfigDirDotted) {
			t.Setenv(name, value)
		}
	}
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	return paths.DefaultConfigDir()
}

// TestLGServedCertificateIsTheSeededOne proves that the lg-tls-default-on
// fixture passes only when the looking glass serves the pair the fixture
// seeded into the store its daemon starts on.
// Method: seed with lgTLSSeed09 under the daemon environment the fixture
// builds and read it back with lgTLSPersisted09; serve the status endpoint
// over TLS with the seeded certificate and with another one, and check
// lgTLSStatusBody09 accepts only the seeded one; and check a memory daemon
// environment refuses the seed before any store exists.
func TestLGServedCertificateIsTheSeededOne(t *testing.T) {
	dir := t.TempDir()
	seededCert, seededKey, err := selfcert.GenerateWebCertWithAddr("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	otherCert, otherKey, err := selfcert.GenerateWebCertWithAddr("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	// MUTATION: seed through storage.Open with no root check, and the memory
	// row is accepted: the daemon would generate its own certificate in memory.
	if err := lgTLSSeed09(daemonEnvironment(nil, dir, daemonStoreMemory), dir, seededCert, seededKey); err == nil {
		t.Error("lgTLSSeed09 accepted a memory daemon environment")
	}
	environment := daemonEnvironment([]string{"ze.test.storage=memory", "ze.config.dir=" + t.TempDir()}, dir, daemonStoreDisk)
	// The fixture runs `ze init` under this environment before it seeds, so
	// the store exists on disk under dir; create it the same way here.
	initial, err := createDaemonStore(environment, dir)
	if err != nil {
		t.Fatalf("create the store ze init would: %v", err)
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lgTLSSeed09(environment, dir, seededCert, seededKey); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := lgTLSPersisted09(environment, dir, seededCert, seededKey); err != nil {
		t.Errorf("the seeded pair did not read back: %v", err)
	}
	// MUTATION: check only that a certificate is stored, and another pair passes.
	if err := lgTLSPersisted09(environment, dir, otherCert, otherKey); err == nil {
		t.Error("lgTLSPersisted09 accepted a pair that was never seeded")
	}

	seeded := lgTestPair(t, seededCert, seededKey)
	fingerprint := sha256.Sum256(seeded.Certificate[0])
	// MUTATION: drop the served-versus-seeded comparison in lgTLSStatusBody09,
	// and the substituted certificate row passes.
	for _, row := range []struct {
		name   string
		served tls.Certificate
		accept bool
	}{
		{"seeded certificate", seeded, true},
		{"substituted certificate", lgTestPair(t, otherCert, otherKey), false},
	} {
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"router_id":"192.0.2.1"}`)) //nolint:errcheck // test handler
		}))
		server.TLS = &tls.Config{Certificates: []tls.Certificate{row.served}, MinVersion: tls.VersionTLS12}
		server.StartTLS()
		address, ok := server.Listener.Addr().(*net.TCPAddr)
		if !ok {
			server.Close()
			t.Fatalf("listener address %v is not TCP", server.Listener.Addr())
		}
		_, err := lgTLSStatusBody09(t.Context(), address.Port, fingerprint)
		server.Close()
		if row.accept && err != nil {
			t.Errorf("%s: refused: %v", row.name, err)
		}
		if !row.accept && (err == nil || !strings.Contains(err.Error(), "want seeded")) {
			t.Errorf("%s: err = %v, want a refusal naming the seeded fingerprint", row.name, err)
		}
	}
}

// lgTestPair parses a PEM pair the way the looking glass loads it.
func lgTestPair(t *testing.T, cert, key []byte) tls.Certificate {
	t.Helper()
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

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
		"ZE_REPO_ROOT":       root,
		"CLAUDE_PROJECT_DIR": root,
		"GOPROXY":            "off",
		"GOSUMDB":            "off",
		"GOTOOLCHAIN":        "local",
		"GOWORK":             "off",
		"GOFLAGS":            "",
		"GOENV":              "off",
		"CGO_ENABLED":        "0",
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
		"internal/component/ike/plugins/added/register.go":        "package added\nimport \"example.test/native-generators/internal/observed\"\nfunc init() { observed.Plugins[\"added\"] = true }\n",
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
		"go.mod":                     "module example.test/native-generators\n\ngo 1.26\n",
		"feature-gates.txt":          "",
		"internal/observed/state.go": "package observed\nvar Plugins = map[string]bool{}\nvar Modules = map[string]string{}\n",
		"internal/component/config/yang/register.go":  "package yang\nimport \"example.test/native-generators/internal/observed\"\nfunc RegisterModule(name, body string) { observed.Modules[name] = body }\n",
		"internal/plugins/sample/register.go":         "package sample\nimport \"example.test/native-generators/internal/observed\"\nfunc init() { observed.Plugins[\"sample\"] = true }\n",
		"internal/plugins/sample/yang/ze-sample.yang": "module ze-sample { namespace \"urn:sample\"; prefix sample; import ze-base { prefix base; } }\n",
		"internal/component/base/yang/ze-base.yang":   "module ze-base { namespace \"urn:base\"; prefix base; }\n",
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
				"go.mod":                          "module example.com/fixture\n",
				"feature-gates.txt":               contentCoreFeatureGate,
				"ai/rules/aaa.md":                 oldRule,
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
