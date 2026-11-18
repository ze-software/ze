// Design: docs/architecture/config/yang-config-design.md -- command visibility from YANG.
package yang

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestCommandHiddenProjection builds the actual schema tree and asks its public
// completion/help producers, keeping an executable hidden path as a control.
func TestCommandHiddenProjection(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("visibility-cmd.yang", `
module visibility-cmd {
 namespace "urn:test:visibility"; prefix v;
 import ze-extensions { prefix ze; }
 container show { config false;
  container visibility { config false;
   container public { config false; ze:hidden false; ze:command "fixture:public";
    ze:help "Public answer"; description "The public answer.";
   }
   container secret { config false; ze:hidden true; ze:command "fixture:secret";
    ze:help "Private answer"; description "The private answer.";
   }
   container concealed { config false; ze:hidden true;
    container child { config false; ze:command "fixture:child";
     ze:help "Concealed child"; description "The concealed child.";
    }
   }
  }
 }
}`))
	schema, err := loader.Resolve()
	require.NoError(t, err)
	root := BuildCommandTree(schema)
	completer := command.NewTreeCompleter(root)
	t.Run("completion", func(t *testing.T) {
		for _, prefix := range []string{"show visibility ", "show visibility s", "show visibility c"} {
			for _, suggestion := range completer.Complete(prefix) {
				require.NotEqual(t, "secret", suggestion.Text)
				require.NotEqual(t, "concealed", suggestion.Text)
			}
		}
		require.Empty(t, completer.Complete("show visibility concealed "))
		public := completer.Complete("show visibility p")
		require.Len(t, public, 1)
		require.Equal(t, "public", public[0].Text)
	})
	t.Run("help", func(t *testing.T) {
		require.Equal(t, []string{"public"}, command.SubcommandNames(command.FindNode(root, []string{"show", "visibility"})))
		require.Empty(t, command.HelpEntries(root, []string{"show", "visibility", "concealed"}))
		_, found := completer.Explain("show visibility secret")
		require.False(t, found)
		text, found := completer.Explain("show visibility public")
		require.True(t, found)
		require.Equal(t, "The public answer.", text)
	})
	t.Run("dispatch-paths", func(t *testing.T) {
		require.Equal(t, "fixture:secret", command.FindNode(root, []string{"show", "visibility", "secret"}).WireMethod)
		require.Equal(t, []string{"show visibility concealed child"}, WireMethodToPaths(schema)["fixture:child"])
	})
}
