// Design: docs/architecture/plugin/feature-gates.md -- bounded external registration effects.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"
)

// The names below forward to ordinary_fixture_test.go for the proof-named
// tests that still use them; they go with this file.
const externalNeutralDir = neutralParcelDir
const externalNeutralFile = neutralParcelFile
const externalNeutralSource = neutralParcelSource

func externalEffectsFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	return importingFixture(t, kind)
}

func externalEffectsSource(declarations, call string) string {
	return importingSource(declarations, call)
}

func externalEffectsPositive(t *testing.T, fixture *asmInventoryFixture) *Snapshot {
	t.Helper()
	return importingPositive(t, fixture)
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
