// Design: docs/architecture/config/yang-config-design.md -- separate config and command schemas.
// Related: ze-l2tp-shaper-conf.yang -- persistent shaping settings.
// Related: ../register.go -- the hidden runtime transaction declaration.

package yang_test

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	l2tpshaper "github.com/ze-software/ze/internal/component/l2tp/plugins/shaper"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestShaperRateCommandStaysOutsideConfiguration loads the registered models
// through the real configuration builder and checks the real plugin declaration.
// Persistent shaping remains configurable, while the runtime operation retains
// its transaction argument and hidden status outside configuration navigation.
func TestShaperRateCommandStaysOutsideConfiguration(t *testing.T) {
	resolved, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := config.SchemaOf(resolved)
	if err != nil {
		t.Fatal(err)
	}
	l2tp, ok := schema.Get("l2tp").(*config.ContainerNode)
	if !ok {
		t.Fatal("registered model lost the L2TP configuration container")
	}
	shaper, ok := l2tp.Get("shaper").(*config.ContainerNode)
	if !ok {
		t.Fatal("registered model lost the persistent shaper configuration")
	}
	if shaper.Get("default-rate") == nil {
		t.Error("persistent shaper configuration lost default-rate")
	}
	if schema.Get("request") != nil {
		t.Error("runtime request operation leaked into the configuration schema")
	}
	registered := registry.Lookup(l2tpshaper.Name)
	if registered == nil {
		t.Fatal("registered model lost the real shaper plugin declaration")
	}
	found := false
	for _, declaration := range registered.Commands {
		if declaration.Name != "request l2tp shaper rate" {
			continue
		}
		if found {
			t.Fatal("rate transaction command is declared more than once")
		}
		found = true
		if !slices.Equal(declaration.Args, []string{"transaction"}) {
			t.Errorf("rate command arguments = %v, want [transaction]", declaration.Args)
		}
		if !declaration.Hidden {
			t.Error("internal rate transaction command is no longer hidden")
		}
		listed, err := command.ListedOnDaemonSurfaces(command.CommandRegistration{
			Path: declaration.Name, Kind: command.KindPlugin, Hidden: declaration.Hidden,
		})
		if err != nil {
			t.Fatal(err)
		}
		if listed {
			t.Error("internal rate transaction command is offered on a daemon surface")
		}
	}
	if !found {
		t.Fatal("plugin declaration lost request l2tp shaper rate")
	}
}
