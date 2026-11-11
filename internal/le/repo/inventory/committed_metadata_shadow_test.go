// Design: docs/architecture/plugin/feature-gates.md -- registration metadata keeps lexical ownership.
package repoinventory

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestCommittedLocalNilMetadataShadow requires a registration field that
// names nil to read a local variable named nil when one shadows it, and the
// predeclared nil otherwise. Method: OptionalDependencies is set to nil, then
// a local nil holding a slice is declared in the same init; the report and the
// declared list carry no dependency, then exactly the shadow's, or the shadow
// refuses with its source named.
func TestCommittedLocalNilMetadataShadow(t *testing.T) {
	fixture := importingFixture(t, "plugin")
	name := ordinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: nil", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, nil, true, false)
	fixture.goFile(t, name, strings.Replace(source, "func init() {", "func init() {\n nil := []string{\"shadow dependency\"}\n", 1))
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, []string{"shadow dependency"}, true, true)
}

// TestCommittedLocalNilFunctionShadow requires a function-valued
// registration field that names nil to follow a local nil holding a function.
// Method: InProcessNLRIDecoder is set to nil, so the plugin has no decoder,
// then a local nil := engine is declared; the decoder appears, or the shadow
// refuses with its source named.
func TestCommittedLocalNilFunctionShadow(t *testing.T) {
	fixture := importingFixture(t, "plugin")
	name := ordinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "InProcessNLRIDecoder: engine", "InProcessNLRIDecoder: nil", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, []string{"optional"}, false, false)
	fixture.goFile(t, name, strings.Replace(source, "func init() {", "func init() {\n nil := engine\n", 1))
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, []string{"optional"}, true, true)
}

// TestCommittedPackageAppendMetadataShadow requires a package function named
// append to replace the builtin in registration metadata. Method:
// OptionalDependencies is built with append, then the package declares its
// own append returning another slice; the declared dependencies follow the
// package function, or the shadow refuses with its source named.
func TestCommittedPackageAppendMetadataShadow(t *testing.T) {
	fixture := importingFixture(t, "plugin")
	name := ordinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: append([]string{}, \"optional\")", 1)
	fixture.goFile(t, name, source)
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, []string{"optional"}, true, false)
	fixture.goFile(t, ordinaryDir+"/shadow_append.go", "package ordinary\nfunc append(_ []string, _ string) []string { return []string{\"actual dependency\"} }\n")
	commitInventoryFixture(t, fixture.root)
	checkLexicalMetadata(t, fixture, []string{"actual dependency"}, true, true)
}

// TestCommittedMetadataKeepsLocalShadow makes an imported constant and
// a local receiver agree first. Changing only the local field must change the
// projected description or explicitly refuse its ownership; the import alias
// must never supply a plausible but incorrect value for the local selector.
func TestCommittedMetadataKeepsLocalShadow(t *testing.T) {
	fixture := importingFixture(t, "plugin")
	name := ordinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "import (", "import (\n store \"github.com/ze-software/ze/"+neutralParcelDir+"\"", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Label", 1)
	// The first read uses the import legitimately. The declaration then
	// shadows it only within this initializer, as ordinary Go scope requires.
	source = strings.Replace(source, "func init() {", "func init() {\n _ = store.Label\n store := struct { Label string }{Label: \"LOCAL_DESCRIPTION\"}\n", 1)
	fixture.goFile(t, name, strings.Replace(source, "LOCAL_DESCRIPTION", "ordinary committed control", 1))
	fixture.goFile(t, callableOwnerCaller, importingSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	importingPositive(t, fixture)

	const local = "local shadow description"
	fixture.goFile(t, name, strings.Replace(source, "LOCAL_DESCRIPTION", local, 1))
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := DeclaredPlugins(fixture.root)
	if reportErr != nil {
		if pluginErr == nil {
			t.Fatalf("report refused shadow ownership but DeclaredPlugins published %+v", plugins)
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
		t.Fatalf("report projected shadow but DeclaredPlugins refused: %v", pluginErr)
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

// checkLexicalMetadata requires the report and the declared plugin list to
// agree, then checks the varied dependencies and decoder before normalizing
// only those fields for the complete existing report oracle.
func checkLexicalMetadata(t *testing.T, fixture *ordinaryFixture, dependencies []string, decoder, allowRefusal bool) {
	t.Helper()
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := DeclaredPlugins(fixture.root)
	if reportErr != nil {
		if !allowRefusal {
			t.Fatalf("metadata control refused: %v", reportErr)
		}
		if pluginErr == nil {
			t.Fatalf("metadata refusal disagrees with DeclaredPlugins: %+v", plugins)
		}
		if !reflect.DeepEqual(inv, Inventory{}) {
			t.Fatalf("metadata refusal leaked report: %+v", inv)
		}
		if plugins != nil {
			t.Fatalf("metadata refusal leaked plugins: %+v", plugins)
		}
		for _, err := range []error{reportErr, pluginErr} {
			if !strings.Contains(err.Error(), ordinaryDir+"/") {
				t.Fatalf("metadata refusal lost source ownership: %v", err)
			}
		}
		return
	}
	if pluginErr != nil {
		t.Fatalf("DeclaredPlugins refused successful report: %v", pluginErr)
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
