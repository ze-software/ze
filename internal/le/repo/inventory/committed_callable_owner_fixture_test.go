// Design: docs/architecture/plugin/feature-gates.md -- executable owners cannot disappear behind call syntax.
package repoinventory

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const callableOwnerDir = "internal/callableowner"
const callableOwnerFile = callableOwnerDir + "/owner.go"
const callableOwnerCaller = ordinaryDir + "/callable_owner.go"
const callableOwnerSchema = "module owner { namespace urn:owner; prefix o; }\n"

type callableOwnerCase struct {
	name           string
	declarations   string
	call           string
	extraImport    string
	prefix         string
	witnessImports []string
}

func callableOwnerPaths() []string {
	paths := []string{
		"cmd/ze/main.go",
		"internal/component/config/yang/register.go",
		"internal/component/plugin/registry/registry.go",
		ordinaryDir + "/register.go",
		callableOwnerCaller,
		callableOwnerFile,
	}
	slices.Sort(paths)
	return paths
}

func callableOwnerSource(scenario callableOwnerCase, call string) string {
	return "package ordinary\nimport (\n helper \"github.com/ze-software/ze/" + callableOwnerDir + "\"\n" + scenario.extraImport + "\n)\n" +
		"func ownerProbe() uint32 { return 0 }\n" +
		"func install(_ uint32) { helper.Install(0) }\n" +
		scenario.declarations + "\nfunc init() {\n" + scenario.prefix + call + "\n}\n"
}

func callableOwnerHelper(kind string) string {
	if kind == "yang" {
		return "package ownerhelper\nimport y \"github.com/ze-software/ze/internal/component/config/yang\"\n" +
			"func Install(_ uint32) { y.RegisterModule(\"owner.yang\", " + strconv.Quote(callableOwnerSchema) + ") }\n" +
			"func ArgumentInstall() uint32 { Install(0); return 0 }\n"
	}
	return "package ownerhelper\nimport r \"github.com/ze-software/ze/internal/component/plugin/registry\"\n" +
		"func engine() {}\n" +
		"func Install(_ uint32) { _ = r.Register(r.Registration{Name: \"owner-plugin\", Description: \"installed through executable owner\", RunEngine: engine}) }\n" +
		"func ArgumentInstall() uint32 { Install(0); return 0 }\n"
}

// callableOwnerFixture uses only ordinary Go bodies. Every candidate has an
// exact ordinary report/path control and a successful direct installer control
// before its call syntax changes. No assembly/header admission can mask it.
func callableOwnerFixture(t *testing.T, kind string, scenario callableOwnerCase) *ordinaryFixture {
	t.Helper()
	fixture := newOrdinaryFixture(t)
	for _, origin := range scenario.witnessImports {
		for _, sink := range []struct{ file, name string }{
			{"internal/component/plugin/registry/registry.go", "registry"},
			{"internal/component/config/yang/register.go", "yang"},
		} {
			declaration := "package " + sink.name + "\n"
			body := strings.Replace(fixture.goSources[sink.file], declaration, declaration+"import _ "+strconv.Quote(origin)+"\n", 1)
			fixture.goFile(t, sink.file, body)
		}
	}
	fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind))
	fixture.goFile(t, "internal/callableownerreportonly/report.go", "package reportonly\nconst Unselected = true\n")
	if kind == "yang" {
		write(t, fixture.root, callableOwnerDir+"/yang/owner.yang", callableOwnerSchema)
	}
	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "_ = ownerProbe()"))
	commitInventoryFixture(t, fixture.root)
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("ordinary Go owner prerequisite: %v", err)
	}
	fixture.checkReport(t, inv)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := snapshot.DeclaredPlugins()
	if err != nil {
		t.Fatalf("ordinary retained owner prerequisite: %v", err)
	}
	fixture.checkPlugins(t, plugins)
	checkSelectedPaths(t, snapshot, callableOwnerPaths())

	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "install(ownerProbe())"))
	commitInventoryFixture(t, fixture.root)
	callableOwnerOutcome(t, fixture, kind, false)
	return fixture
}

// callableOwnerInstalled validates the complete useful report, not only names.
// Only the one explicitly expected registration is removed before reusing the
// frozen exact ordinary report oracle; unexpected rows are never filtered out.
func callableOwnerInstalled(t *testing.T, fixture *ordinaryFixture, kind string, inv Inventory) {
	t.Helper()
	ordinary := inv
	if kind == "yang" {
		want := []YANGModule{
			{Name: "extra.yang", Source: "plugin:ordinary"},
			{Name: "fixture", Source: "plugin:ordinary"},
			{Name: "owner.yang", Source: "infrastructure"},
		}
		if !reflect.DeepEqual(inv.YANGModules, want) {
			t.Fatalf("executed callable YANG owner disappeared: got %+v, want %+v", inv.YANGModules, want)
		}
		ordinary.YANGModules = inv.YANGModules[:2]
	} else {
		if len(inv.Plugins) != 2 || inv.Plugins[0].Name != "ordinary" {
			t.Fatalf("executed callable plugin owner disappeared: %+v", inv.Plugins)
		}
		want := Plugin{Name: "owner-plugin", Description: "installed through executable owner", SourceDir: callableOwnerDir}
		if !reflect.DeepEqual(inv.Plugins[1], want) {
			t.Fatalf("executed callable plugin metadata = %+v, want %+v", inv.Plugins[1], want)
		}
		ordinary.Plugins = inv.Plugins[:1]
	}
	fixture.checkReport(t, ordinary)
}

func callableOwnerCause(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected explicit executable-owner refusal")
	}
	text := err.Error()
	if !strings.Contains(text, "ownership") || (!strings.Contains(text, "call") && !strings.Contains(text, "execution")) {
		t.Fatalf("callable owner refusal masked by an unrelated prerequisite: %v", err)
	}
	if !strings.Contains(text, callableOwnerCaller+":") && !strings.Contains(text, callableOwnerFile+":") {
		t.Fatalf("callable owner refusal has no authored call/declaration attribution: %v", err)
	}
	t.Logf("explicit executable-owner refusal: %v", err)
}

// callableOwnerOutcome requires both public consumers to agree. A supported
// call must retain exact membership and metadata; an unsupported call must name
// execution ownership and return no partial plugins from either consumer.
func callableOwnerOutcome(t *testing.T, fixture *ordinaryFixture, kind string, allowRefusal bool) (*Snapshot, []Plugin, error) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatalf("source capture failed before callable-owner admission: %v", err)
	}
	checkSelectedPaths(t, snapshot, callableOwnerPaths())
	plugins, pluginErr := snapshot.DeclaredPlugins()
	if reportErr != nil {
		if !allowRefusal {
			t.Fatalf("direct Go installer prerequisite failed: %v", reportErr)
		}
		if inv.Plugins != nil || plugins != nil {
			t.Fatalf("callable-owner refusal leaked partial plugins: report %+v, retained %+v", inv.Plugins, plugins)
		}
		callableOwnerCause(t, reportErr)
		callableOwnerCause(t, pluginErr)
		if reportErr.Error() != pluginErr.Error() {
			t.Fatalf("public consumers disagree about callable refusal: %v / %v", reportErr, pluginErr)
		}
		return snapshot, plugins, pluginErr
	}
	if pluginErr != nil {
		t.Fatalf("report accepted owner but retained Plugins refused: %v", pluginErr)
	}
	callableOwnerInstalled(t, fixture, kind, inv)
	if !reflect.DeepEqual(plugins, inv.Plugins) {
		t.Fatalf("public callable membership differs: report %+v, retained %+v", inv.Plugins, plugins)
	}
	return snapshot, plugins, nil
}

func callableOwnerRetained(t *testing.T, snapshot *Snapshot, plugins []Plugin, previous error) {
	t.Helper()
	got, err := snapshot.DeclaredPlugins()
	if previous != nil {
		if got != nil {
			t.Fatalf("captured owner refusal became partial success: %+v", got)
		}
		callableOwnerCause(t, err)
		if err.Error() != previous.Error() {
			t.Fatalf("captured ownership cause changed: %v / %v", err, previous)
		}
	} else if err != nil || !reflect.DeepEqual(got, plugins) {
		t.Fatalf("captured executable owners changed: %+v %v, want %+v", got, err, plugins)
	}
	checkSelectedPaths(t, snapshot, callableOwnerPaths())
}
