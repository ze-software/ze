// Design: docs/architecture/config/yang-config-design.md -- separate configuration and runtime commands.
// Related: ../session_transaction.go -- the hidden subscriber transaction declaration.

//go:build ze_l2tp

package yang_test

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/plugins/cos"
)

// TestCoSSessionCommandStaysOutsideConfiguration loads the real configuration
// schema and plugin declaration. Profiles remain configurable, while the
// subscriber transaction retains its argument and hidden status outside the
// configuration navigation tree.
func TestCoSSessionCommandStaysOutsideConfiguration(t *testing.T) {
	resolved, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	schema, err := config.SchemaOf(resolved)
	if err != nil {
		t.Fatal(err)
	}
	profiles, ok := schema.Get("class-of-service").(*config.ContainerNode)
	if !ok {
		t.Fatal("registered model lost the class-of-service configuration")
	}
	if profiles.Get("ieee-802.1p") == nil {
		t.Error("persistent class-of-service configuration lost its profile list")
	}
	if schema.Get("request") != nil {
		t.Error("runtime request operation leaked into the configuration schema")
	}
	registered := registry.Lookup(cos.Name)
	if registered == nil {
		t.Fatal("registered model lost the real class-of-service plugin declaration")
	}
	found := false
	for _, declaration := range registered.Commands {
		if declaration.Name != "request class-of-service session" {
			continue
		}
		if found {
			t.Fatal("subscriber class-of-service transaction is declared more than once")
		}
		found = true
		if !slices.Equal(declaration.Args, []string{"transaction"}) {
			t.Errorf("session command arguments = %v, want [transaction]", declaration.Args)
		}
		if !declaration.Hidden {
			t.Error("internal subscriber transaction command is no longer hidden")
		}
		listed, err := command.ListedOnDaemonSurfaces(command.CommandRegistration{
			Path: declaration.Name, Kind: command.KindPlugin, Hidden: declaration.Hidden,
		})
		if err != nil {
			t.Fatal(err)
		}
		if listed {
			t.Error("internal subscriber transaction command is offered on a daemon surface")
		}
	}
	if !found {
		t.Fatal("plugin declaration lost request class-of-service session")
	}
}
