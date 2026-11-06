// Design: docs/architecture/plugin/feature-gates.md -- lexical metadata and reference authority.
package repoinventory

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCommittedLocalNilMetadataShadow(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: nil", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, nil, true, false)
	fixture.goFile(t, name, strings.Replace(source, "func init() {", "func init() {\n nil := []string{\"shadow dependency\"}\n", 1))
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, []string{"shadow dependency"}, true, true)
}

func TestCommittedLocalNilFunctionShadow(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "InProcessNLRIDecoder: engine", "InProcessNLRIDecoder: nil", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, []string{"optional"}, false, false)
	fixture.goFile(t, name, strings.Replace(source, "func init() {", "func init() {\n nil := engine\n", 1))
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, []string{"optional"}, true, true)
}

func TestCommittedPackageAppendMetadataShadow(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: append([]string{}, \"optional\")", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, []string{"optional"}, true, false)
	fixture.goFile(t, asmOrdinaryDir+"/shadow_append.go", "package ordinary\nfunc append(_ []string, _ string) []string { return []string{\"actual dependency\"} }\n")
	commitInventoryFixture(t, fixture.root)
	externalBoundaryMetadata(t, fixture, []string{"actual dependency"}, true, true)
}

func TestCommittedExternalStructSliceFieldAuthority(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "func init() {", "var metadata = struct { Names []string; Label string }{Names: []string{\"optional\"}, Label: \"ordinary committed control\"}\nfunc init() {", 1)
	source = strings.Replace(source, "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: metadata.Names", 1)
	fixture.goFile(t, name, source)
	// Copying only the scalar field must not expose the holder's slice.
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Strings([]string{metadata.Label})"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Strings(metadata.Names)"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}

func TestCommittedExternalPackagePredeclaredCallbacks(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, name := range []string{"nil", "true", "false"} {
			t.Run(kind+"/"+name, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				file := asmOrdinaryDir + "/shadow_callback.go"
				fixture.goFile(t, file, "package ordinary\nfunc "+name+"() {}\n")
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "_ = clock.AfterFunc(0, "+name+")"))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, file, "package ordinary\nfunc "+name+"() { install(0) }\n")
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// externalBoundaryMetadata checks both public projections before normalizing
// only the explicitly varied fields for the complete existing report oracle.
func externalBoundaryMetadata(t *testing.T, fixture *asmInventoryFixture, dependencies []string, decoder, allowRefusal bool) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr != nil {
		if !allowRefusal {
			t.Fatalf("metadata control refused: %v", reportErr)
		}
		if pluginErr == nil {
			t.Fatalf("metadata refusal disagrees with Plugins: %+v", plugins)
		}
		if !reflect.DeepEqual(inv, Inventory{}) {
			t.Fatalf("metadata refusal leaked report: %+v", inv)
		}
		if plugins != nil {
			t.Fatalf("metadata refusal leaked plugins: %+v", plugins)
		}
		for _, err := range []error{reportErr, pluginErr} {
			if !strings.Contains(err.Error(), asmOrdinaryDir+"/") {
				t.Fatalf("metadata refusal lost source ownership: %v", err)
			}
		}
		return
	}
	if pluginErr != nil {
		t.Fatalf("Plugins refused successful report: %v", pluginErr)
	}
	if !reflect.DeepEqual(plugins, inv.Plugins) {
		t.Fatalf("public metadata projections disagree: %+v / %+v", inv.Plugins, plugins)
	}
	if len(inv.Plugins) != 1 {
		t.Fatalf("metadata changed membership: %+v", inv.Plugins)
	}
	if !slices.Equal(inv.Plugins[0].OptionalDependencies, dependencies) {
		t.Fatalf("lexical dependencies = %v, want %v", inv.Plugins[0].OptionalDependencies, dependencies)
	}
	if inv.Plugins[0].HasDecoder != decoder {
		t.Fatalf("lexical decoder = %v, want %v", inv.Plugins[0].HasDecoder, decoder)
	}
	if len(inv.FamilySupport) != 1 {
		t.Fatalf("metadata changed family membership: %+v", inv.FamilySupport)
	}
	if inv.FamilySupport[0].Decode != decoder {
		t.Fatalf("family decoder = %v, want %v", inv.FamilySupport[0].Decode, decoder)
	}
	inv.Plugins = slices.Clone(inv.Plugins)
	inv.Plugins[0].OptionalDependencies = []string{"optional"}
	inv.Plugins[0].HasDecoder = true
	inv.FamilySupport = slices.Clone(inv.FamilySupport)
	inv.FamilySupport[0].Decode = true
	fixture.checkReport(t, inv)
}
