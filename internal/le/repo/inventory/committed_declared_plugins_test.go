// Design: docs/architecture/plugin/feature-gates.md -- the declared list beside the proved one.
package repoinventory

import (
	"reflect"
	"testing"
)

// TestDeclaredPluginsMatchesTheProvedPopulation proves the list-only reader
// publishes exactly what the proof-gated reader does wherever the proof
// accepts the tree. Method: the external-effects fixture, whose standard
// library calls make the proof run and accept, read through both entries.
//
// MUTATION: making the callDeclared walk skip a registry call (registryCall)
// leaves DeclaredPlugins with no population, so it refuses and this goes red.
func TestDeclaredPluginsMatchesTheProvedPopulation(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			proved, err := Plugins(fixture.root)
			if err != nil {
				t.Fatalf("the proof refused its own positive fixture: %v", err)
			}
			declared, err := DeclaredPlugins(fixture.root)
			if err != nil {
				t.Fatalf("the declared reader refused a tree the proof accepts: %v", err)
			}
			fixture.checkPlugins(t, declared)
			if !reflect.DeepEqual(declared, proved) {
				t.Fatalf("declared population %+v differs from the proved one %+v", declared, proved)
			}
		})
	}
}

// TestDeclaredPluginsRefusesAnUnreadablePopulation keeps the list-only reader
// fail-closed: a root with no committed source answers an error and no list.
func TestDeclaredPluginsRefusesAnUnreadablePopulation(t *testing.T) {
	plugins, err := DeclaredPlugins(t.TempDir())
	if err == nil {
		t.Fatalf("an unreadable root answered %+v", plugins)
	}
	if plugins != nil {
		t.Fatalf("a refusal leaked a partial list: %+v", plugins)
	}
}
