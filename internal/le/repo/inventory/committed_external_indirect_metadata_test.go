// Design: docs/architecture/plugin/feature-gates.md -- retained writes invalidate metadata.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalIndirectHelperMetadataWrite keeps the address inside
// retained code. An unrelated opaque call activates the effect certificate;
// it receives no metadata pointer and cannot stand in for the helper's write.
func TestCommittedExternalIndirectHelperMetadataWrite(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "func init() {", "func change(p *string) { HELPER_BODY }\nfunc init() {\n label := \"ordinary committed control\"\n change(&label)\n", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: label", 1)
	fixture.goFile(t, name, strings.Replace(source, "HELPER_BODY", "_ = p", 1))
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	// Only the reached helper body changes. Reporting the original literal
	// would now contradict the write performed before registration.
	fixture.goFile(t, name, strings.Replace(source, "HELPER_BODY", "*p = \"changed description\"", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}
