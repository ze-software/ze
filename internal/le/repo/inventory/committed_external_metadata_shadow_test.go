// Design: docs/architecture/plugin/feature-gates.md -- metadata keeps lexical ownership.
package repoinventory

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestCommittedExternalMetadataKeepsLocalShadow makes an imported constant and
// a local receiver agree first. Changing only the local field must change the
// projected description or explicitly refuse its ownership; the import alias
// must never supply a plausible but incorrect value for the local selector.
func TestCommittedExternalMetadataKeepsLocalShadow(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Label", 1)
	// The first read uses the import legitimately. The declaration then
	// shadows it only within this initializer, as ordinary Go scope requires.
	source = strings.Replace(source, "func init() {", "func init() {\n _ = store.Label\n store := struct { Label string }{Label: \"LOCAL_DESCRIPTION\"}\n", 1)
	fixture.goFile(t, name, strings.Replace(source, "LOCAL_DESCRIPTION", "ordinary committed control", 1))
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	const local = "local shadow description"
	fixture.goFile(t, name, strings.Replace(source, "LOCAL_DESCRIPTION", local, 1))
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr != nil {
		if pluginErr == nil {
			t.Fatalf("report refused shadow ownership but Plugins published %+v", plugins)
		}
		if !reflect.DeepEqual(inv, Inventory{}) {
			t.Fatalf("shadow refusal leaked a partial report: %+v", inv)
		}
		if plugins != nil {
			t.Fatalf("shadow refusal leaked partial plugins: %+v", plugins)
		}
		for _, err := range []error{reportErr, pluginErr} {
			if !strings.Contains(err.Error(), name+":") {
				t.Fatalf("shadow refusal lost its registration-source attribution: %v", err)
			}
			if !strings.Contains(err.Error(), "ownership") {
				t.Fatalf("shadow refusal did not name unresolved ownership: %v", err)
			}
		}
		return
	}
	if pluginErr != nil {
		t.Fatalf("report projected shadow but Plugins refused: %v", pluginErr)
	}
	if len(inv.Plugins) != 1 {
		t.Fatalf("local metadata changed plugin membership: %+v", inv.Plugins)
	}
	if inv.Plugins[0].Description != local {
		t.Fatalf("local selector borrowed imported metadata: got %q, want %q", inv.Plugins[0].Description, local)
	}
	if !reflect.DeepEqual(plugins, inv.Plugins) {
		t.Fatalf("public shadow projections disagree: report %+v, plugins %+v", inv.Plugins, plugins)
	}
	// Normalize only the explicitly changed description before applying the
	// existing exact report oracle; every other row and field remains checked.
	ordinary := inv
	ordinary.Plugins = slices.Clone(inv.Plugins)
	ordinary.Plugins[0].Description = "ordinary committed control"
	fixture.checkReport(t, ordinary)
}
