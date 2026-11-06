// Design: docs/architecture/plugin/feature-gates.md -- bounded external registration effects.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"
)

const externalNeutralDir = "pkg/neutralparcel"
const externalNeutralFile = externalNeutralDir + "/parcel.go"
const externalNeutralSource = "package neutralparcel\nvar Callback func()\nconst Label = \"ordinary committed control\"\n"

// externalEffectsFixture first proves the ordinary report and direct installer.
// Its external imports name real APIs, but inventory never reads or executes
// their implementation. Both sinks carry the authored import-cycle witnesses.
func externalEffectsFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := callableOwnerFixture(t, kind, callableOwnerCase{})
	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(callableOwnerCase{}, "_ = ownerProbe()"))
	fixture.goFile(t, externalNeutralFile, externalNeutralSource)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\")\n")
	}
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	return fixture
}

func externalEffectsSource(declarations, call string) string {
	return callableOwnerSource(callableOwnerCase{
		declarations: declarations,
		extraImport:  "remote \"os\"\n conduit \"io\"\n order \"sort\"\n clock \"time\"\n store \"github.com/ze-software/ze/" + externalNeutralDir + "\"",
		prefix:       "_ = remote.Stat; _ = conduit.Pipe; _ = order.Slice; _ = clock.AfterFunc; _ = store.Label\n",
	}, call)
}

func externalEffectsPositive(t *testing.T, fixture *asmInventoryFixture) *Snapshot {
	t.Helper()
	inv, err := Collect(fixture.root)
	if err != nil {
		t.Fatalf("external effect-free control refused: %v", err)
	}
	fixture.checkReport(t, inv)
	plugins, err := Plugins(fixture.root)
	if err != nil {
		t.Fatalf("external effect-free Plugins refused: %v", err)
	}
	fixture.checkPlugins(t, plugins)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func externalEffectsRefusal(t *testing.T, fixture *asmInventoryFixture, reason string) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("external uncertainty published a report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("external uncertainty published plugins: %+v", plugins)
	}
	if !reflect.DeepEqual(inv, Inventory{}) {
		t.Fatalf("external refusal leaked a partial report: %+v", inv)
	}
	if plugins != nil {
		t.Fatalf("external refusal leaked partial plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), "external") {
			t.Fatalf("external effect masked by unrelated refusal: %v", err)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("external %s cause absent: %v", reason, err)
		}
	}
}
