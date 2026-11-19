// Design: docs/research/vpp-deployment-reference.md -- startup configuration transactions

package vpp

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Equal delivered settings must remain equal after feature selection, so an
// unrelated read-root edit cannot restart the forwarding process. The registry
// predicate belongs to selection, never to the installed configuration value.
func TestStartupFeatureSelectionsCompareEqual(t *testing.T) {
	feature, err := newStartupFeature("probe", StartupFeature{
		Plugin: "probe_plugin.so", Section: "probe",
		Enabled: func(*config.Tree) (bool, error) { return true, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	sections := []sdk.ConfigSection{{Root: "vpp", Data: `{"vpp":{"enabled":"true"}}`}}
	first, err := parseStartupSections(sections, []startupFeature{feature})
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseStartupSections(sections, []startupFeature{feature})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical selected startup requirements compare unequal")
	}
}
