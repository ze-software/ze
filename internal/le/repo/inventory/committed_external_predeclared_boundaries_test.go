// Design: docs/architecture/plugin/feature-gates.md -- lexical metadata and reference authority.
package repoinventory

import (
	"strings"
	"testing"
)

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
