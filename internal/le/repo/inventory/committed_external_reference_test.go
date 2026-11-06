// Design: docs/architecture/plugin/feature-gates.md -- external reference capabilities.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalStaticConstructorReturn exposes a private returned owner
// through a possibly imported neutral package. No retained caller calls New;
// analyzing its body without exposing its returned capabilities is insufficient.
func TestCommittedExternalStaticConstructorReturn(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			neutral := "package neutralparcel\nconst Label = \"ordinary committed control\"\n" +
				"var saved func()\ntype parcel struct { callback func() }\n" +
				"func Store(callback func()) { saved = callback }\n" +
				"func New() parcel { return parcel{callback: saved} }\n" +
				"func (p parcel) Fire() { p.callback() }\n"
			fixture.goFile(t, externalNeutralFile, neutral)
			call := "store.Store(func() { BODY }); parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", strings.Replace(call, "BODY", "", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", strings.Replace(call, "BODY", "install(0)", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalReceiverCallback keeps method targets separate from the
// callback stored in their receiver. The callback only escapes inside Send;
// treating it as a certain direct callee loses the external uncertainty.
func TestCommittedExternalReceiverCallback(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			declarations := "type parcel struct { callback func() }\n" +
				"func (p parcel) Send() { _ = clock.AfterFunc(0, p.callback) }\n"
			call := "p := parcel{callback: func() { BODY }}; p.Send()"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, strings.Replace(call, "BODY", "", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, strings.Replace(call, "BODY", "install(0)", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalSliceMetadataAlias transfers a slice's writable backing
// storage without taking its address. Equal contents in a separate allocation
// are inert; copying the slice descriptor still exposes registration metadata.
func TestCommittedExternalSliceMetadataAlias(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	neutral := "package neutralparcel\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\")\n" +
		"const Label = \"ordinary committed control\"\nvar Labels = []string{\"optional\"}\n"
	fixture.goFile(t, externalNeutralFile, neutral)
	registration := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: store.Labels", 1)
	fixture.goFile(t, registration, source)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Slice([]string{\"optional\"}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "alias := store.Labels; order.Slice(alias, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}

// TestCommittedExternalChannelTransferRefuses requires an explicit unsupported
// channel proof rather than silently dropping a sent callback. Neither inert
// nor effectful channels are asserted to be supported by this bounded model.
func TestCommittedExternalChannelTransferRefuses(t *testing.T) {
	for _, body := range []string{"", "install(0)"} {
		t.Run("callback-"+body, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			call := "queue := make(chan func(), 1); queue <- func() { " + body + " }; _ = clock.AfterFunc(0, func() { callback := <-queue; callback() })"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "channel")
		})
	}
}
