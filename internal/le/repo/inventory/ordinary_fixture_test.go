// Design: docs/architecture/plugin/feature-gates.md -- authored committed inputs with an exact ordinary report oracle.
package repoinventory

import (
	"maps"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const ordinaryDir = "internal/plugins/ordinary"
const ordinarySchema = "module fixture { namespace urn:fixture; prefix f; rpc fixture-list { } }\n"
const ordinaryExtraSchema = "module extra { namespace urn:extra; prefix e; rpc extra-check { } }\n"

// ordinaryFixture is a committed fixture repository holding one ordinary
// plugin, with every Go source it wrote kept so the report oracle can count
// the authored population.
type ordinaryFixture struct {
	root      string
	goSources map[string]string
}

// newOrdinaryFixture establishes exact ordinary registration/report controls
// before a test changes its input. It never compiles its source.
func newOrdinaryFixture(t *testing.T) *ordinaryFixture {
	t.Helper()
	fixture := &ordinaryFixture{root: t.TempDir(), goSources: make(map[string]string)}
	write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n")
	write(t, fixture.root, "feature-gates.txt", "ze_fixture "+ordinaryDir+"\n")
	fixture.goFile(t, "cmd/ze/main.go", "//go:build ze_le && ze_fixture\npackage main\nimport _ \"github.com/ze-software/ze/"+ordinaryDir+"\"\nfunc main() {}\n")
	fixture.goFile(t, "internal/component/plugin/registry/registry.go", `package registry
type Registration struct { Name, Description, YANG, Features string; Families, Dependencies, OptionalDependencies, StartAfter, ConfigRoots, RFCs []string; CapabilityCodes []uint8; RunEngine, InProcessNLRIDecoder, InProcessNLRIEncoder, InProcessRouteEncoder, InProcessConfigNLRIBuilder func() }
func Register(Registration) error { return nil }
`)
	fixture.goFile(t, "internal/component/config/yang/register.go", "package yang\nfunc RegisterModule(name, content string) {}\n")
	fixture.goFile(t, ordinaryDir+"/register.go", `package ordinary
import (
 r "github.com/ze-software/ze/internal/component/plugin/registry"
 y "github.com/ze-software/ze/internal/component/config/yang"
)
func engine() {}
func init() {
 _ = r.Register(r.Registration{Name: "ordinary", Description: "ordinary committed control", Families: []string{"ipv4/unicast"}, CapabilityCodes: []uint8{64}, OptionalDependencies: []string{"optional"}, StartAfter: []string{"ordered"}, ConfigRoots: []string{"fixture"}, RFCs: []string{"fixture-rfc"}, Features: "fixture feature", RunEngine: engine, YANG: fixtureSchema, InProcessNLRIDecoder: engine, InProcessNLRIEncoder: engine, InProcessRouteEncoder: engine, InProcessConfigNLRIBuilder: engine})
 y.RegisterModule("extra.yang", extraSchema)
}
const fixtureSchema = `+strconv.Quote(ordinarySchema)+"\nconst extraSchema = "+strconv.Quote(ordinaryExtraSchema)+"\n")
	write(t, fixture.root, ordinaryDir+"/yang/fixture.yang", ordinarySchema)
	write(t, fixture.root, ordinaryDir+"/yang/extra.yang", ordinaryExtraSchema)
	write(t, fixture.root, "test/ui/fixture.ci", "cmd=foreground:exec=ze fixture list\n")
	write(t, fixture.root, "test/schema/extra.ci", "cmd=foreground:exec=ze extra check\n")
	committedGit(t, fixture.root, "init", "--initial-branch=main")
	commitInventoryFixture(t, fixture.root)
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("ordinary registered control: %v", err)
	}
	fixture.checkReport(t, inv)
	return fixture
}

func (fixture *ordinaryFixture) goFile(t *testing.T, name, body string) {
	t.Helper()
	fixture.goSources[name] = body
	write(t, fixture.root, name, body)
}

func (fixture *ordinaryFixture) checkPlugins(t *testing.T, plugins []Plugin) {
	t.Helper()
	want := []Plugin{{
		Name: "ordinary", Description: "ordinary committed control", Families: []string{"ipv4/unicast"},
		Capabilities: []uint8{64}, OptionalDependencies: []string{"optional"}, StartAfter: []string{"ordered"},
		ConfigRoots: []string{"fixture"}, RFCs: []string{"fixture-rfc"}, Features: "fixture feature",
		SourceDir: ordinaryDir, YANGFiles: []string{ordinaryDir + "/yang/extra.yang", ordinaryDir + "/yang/fixture.yang"},
		HasYANG: true, HasDecoder: true, HasEncoder: true,
	}}
	if !reflect.DeepEqual(plugins, want) {
		t.Fatalf("committed plugin metadata = %+v, want %+v", plugins, want)
	}
}

// checkReport derives code counts from the authored fixture corpus, not a host
// registry, live filesystem walk or a second invocation of the report producer.
func (fixture *ordinaryFixture) checkReport(t *testing.T, inv Inventory) {
	t.Helper()
	fixture.checkPlugins(t, inv.Plugins)
	if !reflect.DeepEqual(inv.Families, map[string]string{"ipv4/unicast": "ordinary"}) {
		t.Errorf("families = %v", inv.Families)
	}
	if !reflect.DeepEqual(inv.Capabilities, map[string]string{"64": "ordinary"}) {
		t.Errorf("capabilities = %v", inv.Capabilities)
	}
	wantSupport := []Family{{Family: "ipv4/unicast", Plugin: "ordinary", Decode: true, Encode: true, RouteEncode: true, ConfigNLRI: true}}
	if !reflect.DeepEqual(inv.FamilySupport, wantSupport) {
		t.Errorf("family support = %+v", inv.FamilySupport)
	}
	wantModules := []YANGModule{{Name: "extra.yang", Source: "plugin:ordinary"}, {Name: "fixture", Source: "plugin:ordinary"}}
	if !reflect.DeepEqual(inv.YANGModules, wantModules) {
		t.Errorf("registered schemas = %+v, want %+v", inv.YANGModules, wantModules)
	}
	wantRPC := []RPC{{Name: "extra-check", Module: "extra.yang", Covered: true}, {Name: "fixture-list", Module: "fixture.yang", Covered: true}}
	if !reflect.DeepEqual(inv.RPCList, wantRPC) || inv.TotalRPCs != 2 || !reflect.DeepEqual(inv.RPCsByModule, map[string]int{"extra.yang": 1, "fixture.yang": 1}) {
		t.Errorf("RPC report = %+v totals %d %v", inv.RPCList, inv.TotalRPCs, inv.RPCsByModule)
	}
	if inv.TotalTests != 2 || !reflect.DeepEqual(inv.TestCounts, map[string]int{"ui": 1, "schema": 1}) {
		t.Errorf("CI report = %d %v", inv.TotalTests, inv.TestCounts)
	}
	wantStats := make([]AreaStats, 0, 3)
	for _, area := range []string{"internal", "pkg", "cmd"} {
		stats := AreaStats{Area: area + "/"}
		packages := make(map[string]bool)
		for name, body := range fixture.goSources {
			if !strings.HasPrefix(name, area+"/") {
				continue
			}
			stats.Files++
			packages[path.Dir(name)] = true
			stats.Lines += strings.Count(body, "\n")
			if !strings.HasSuffix(body, "\n") {
				stats.Lines++
			}
		}
		stats.Packages = len(packages)
		wantStats = append(wantStats, stats)
	}
	if !reflect.DeepEqual(inv.PackageStats, wantStats) {
		t.Errorf("Go report population = %+v, want exact authored %+v", inv.PackageStats, wantStats)
	}
}

// checkSelectedPaths requires the snapshot to select exactly the authored Go
// paths, so a report-only file or a stray dependency cannot enter.
func checkSelectedPaths(t *testing.T, snapshot *Snapshot, want []string) {
	t.Helper()
	sources, _ := snapshot.Sources()
	got := make([]string, 0, len(sources))
	for _, source := range sources {
		got = append(got, source.Path)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("selected retained Go paths = %q, want exact authored %q", got, want)
	}
}

// checkOrdinaryPositive requires the report and the declared plugin list to
// match the ordinary oracle, and the snapshot to select exactly paths.
func checkOrdinaryPositive(t *testing.T, fixture *ordinaryFixture, paths []string) *Snapshot {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("positive ordinary prerequisite failed: %v", err)
	}
	fixture.checkReport(t, inv)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := snapshot.DeclaredPlugins()
	if err != nil {
		t.Fatalf("positive retained plugin prerequisite failed: %v", err)
	}
	fixture.checkPlugins(t, plugins)
	checkSelectedPaths(t, snapshot, paths)
	return snapshot
}

// frozenFixture freezes the authored expected report corpus before later
// dirty writes; a changed fixture map must not become the committed oracle.
func frozenFixture(fixture *ordinaryFixture) *ordinaryFixture {
	return &ordinaryFixture{root: fixture.root, goSources: maps.Clone(fixture.goSources)}
}

const neutralParcelDir = "pkg/neutralparcel"
const neutralParcelFile = neutralParcelDir + "/parcel.go"
const neutralParcelSource = "package neutralparcel\nvar Callback func()\nconst Label = \"ordinary committed control\"\n"

// importingFixture is callableOwnerFixture whose caller imports standard
// library packages and a neutral Ze package, and whose sinks carry the same
// imports as witnesses. Inventory never reads or executes the standard
// library's implementation. It first proves the ordinary report.
func importingFixture(t *testing.T, kind string) *ordinaryFixture {
	t.Helper()
	fixture := callableOwnerFixture(t, kind, callableOwnerCase{})
	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(callableOwnerCase{}, "_ = ownerProbe()"))
	fixture.goFile(t, neutralParcelFile, neutralParcelSource)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\")\n")
	}
	commitInventoryFixture(t, fixture.root)
	importingPositive(t, fixture)
	return fixture
}

// importingSource is the caller source of importingFixture: call runs in its
// init with the standard library aliases remote, conduit, order and clock and
// the neutral package store in scope.
func importingSource(declarations, call string) string {
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "remote \"os\"\n conduit \"io\"\n order \"sort\"\n clock \"time\"\n store \"github.com/ze-software/ze/" + neutralParcelDir + "\"",
		prefix:       "_ = remote.Stat; _ = conduit.Pipe; _ = order.Slice; _ = clock.AfterFunc; _ = store.Label\n",
	}, call)
}

// importingPositive requires the report and the declared plugin list to
// match the ordinary oracle.
func importingPositive(t *testing.T, fixture *ordinaryFixture) *Snapshot {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("importing control refused: %v", err)
	}
	fixture.checkReport(t, inv)
	plugins, err := DeclaredPlugins(fixture.root)
	if err != nil {
		t.Fatalf("importing control DeclaredPlugins refused: %v", err)
	}
	fixture.checkPlugins(t, plugins)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
