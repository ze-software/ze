// Design: docs/architecture/api/commands.md -- hidden commands in the catalog.
package help_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/plugins/help"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCommandCatalogOmitsSchemaHiddenPaths calls the public catalog producer
// over a real registered module and plugin declaration, with visible controls.
func TestCommandCatalogOmitsSchemaHiddenPaths(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("catalog-hidden-projection-cmd.yang", `
module catalog-hidden-projection-cmd {
 namespace "urn:test:catalog-hidden-projection"; prefix ch;
 import ze-extensions { prefix ze; }
 container show { config false;
  container zz-hidden-projection { config false;
   container public { config false; ze:command "fixture:public";
    ze:help "Public answer"; description "The public answer.";
   }
   container secret { config false; ze:hidden true; ze:command "fixture:secret";
    ze:help "Hidden answer"; description "The hidden answer.";
   }
   container concealed { config false; ze:hidden true;
    container child { config false; ze:command "fixture:child";
     ze:help "Concealed child"; description "The concealed child.";
    }
   }
   container plugin { config false; ze:hidden true;
    ze:help "Hidden plugin"; description "The hidden plugin.";
   }
  }
 }
}`))
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: "catalog-hidden-projection", Description: "Catalog visibility fixture",
		RunEngine: func(_ net.Conn) int { return 0 }, CLIHandler: func(_ []string) int { return 0 },
		Commands: []rpc.CommandDecl{
			{Name: "show zz-hidden-projection plugin", ShortHelp: "Schema-hidden plugin"},
			{Name: "show zz-hidden-projection dynamic", ShortHelp: "Visible plugin"},
		},
	}))
	entries, err := help.CollectCommands()
	require.NoError(t, err)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	require.Contains(t, paths, "show zz-hidden-projection public")
	require.Contains(t, paths, "show zz-hidden-projection dynamic")
	for _, path := range []string{"secret", "concealed child", "plugin"} {
		require.NotContains(t, paths, "show zz-hidden-projection "+path)
	}
}
