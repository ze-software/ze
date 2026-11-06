// Design: docs/architecture/plugin/feature-gates.md -- certified calls still evaluate their receivers.
package repoinventory

import (
	"reflect"
	"testing"
)

// TestCommittedExternalReceiverFactoryKeepsCertainRegistration compares a
// statement-separated receiver with the same retained factory evaluated inside
// an opaque method call. The factory installs synchronously before returning
// an external-owned receiver; the outer method's certificate cannot erase that
// certain registration or turn it into uncertain external callback execution.
func TestCommittedExternalReceiverFactoryKeepsCertainRegistration(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			const factory = "func installedParcel() remote.FileInfo { install(0); parcel, fault := remote.Stat(\".\"); _ = fault; return parcel }\n"
			for _, call := range []string{
				"parcel := installedParcel(); _ = parcel.IsDir()",
				"_ = installedParcel().IsDir()",
			} {
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(factory, call))
				commitInventoryFixture(t, fixture.root)
				inv, err := Collect(fixture.root)
				if err != nil {
					t.Fatalf("receiver factory %q refused certain registration: %v", call, err)
				}
				callableOwnerInstalled(t, fixture, kind, inv)
				plugins, err := Plugins(fixture.root)
				if err != nil {
					t.Fatalf("receiver factory %q Plugins refused: %v", call, err)
				}
				if !reflect.DeepEqual(plugins, inv.Plugins) {
					t.Fatalf("receiver factory %q public outputs disagree: report %+v, plugins %+v", call, inv.Plugins, plugins)
				}
			}
		})
	}
}
