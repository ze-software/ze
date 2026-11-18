// Design: docs/architecture/config/change-file-structural-ops.md -- pending kinds retain intent.
package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// TestEditorManagerComparePendingKinds sends each public editing operation
// through its real producer. Both review surfaces must retain its typed
// intent, including quoted entry addresses and value-less entry creation.
func TestEditorManagerComparePendingKinds(t *testing.T) {
	const module = `module ze-web-pending-kinds-conf {
  yang-version 1.1;
  namespace "urn:ze:web-pending-kinds";
  prefix probe;
  import ze-extensions { prefix ze; }
  container pending-kinds {
    description "Legal quoted entry addresses for pending operation review.";
    ze:help "Pending operation review fixture";
    list entry {
      key "name";
      description "Entries retain their quoted string keys through structural edits.";
      ze:help "Entry with a free-form string key";
      leaf name {
        type string;
        description "Spaces are valid in this entry's key and require address quoting.";
        ze:help "Free-form entry name";
      }
      leaf value {
        type string;
        description "Payload retained when the entry is copied or renamed.";
        ze:help "Entry payload";
      }
    }
  }
}`
	t.Cleanup(configyang.RegisterModuleForTest("ze-web-pending-kinds-conf.yang", module))
	for _, tc := range []struct {
		name, prefix, command, row string
		kind                       contract.PendingChangeKind
	}{
		{"set", "", "set bgp router-id 192.0.2.10", "+ bgp router-id 192.0.2.10", contract.PendingChangeSet},
		{"delete", "", "delete bgp router-id", "- bgp router-id 1.2.3.4", contract.PendingChangeDelete},
		{"rename", "", `rename pending-kinds entry "alpha space" to gamma`, `~ rename pending-kinds entry "alpha space" to pending-kinds entry gamma`, contract.PendingChangeRename},
		{"copy", "", `copy pending-kinds entry "alpha space" to gamma`, `+ copy pending-kinds entry "alpha space" to pending-kinds entry gamma`, contract.PendingChangeCopy},
		{"deactivate", "", "deactivate bgp router-id", "~ deactivate bgp router-id", contract.PendingChangeDeactivate},
		{"activate", "inactive: ", "activate bgp router-id", "~ activate bgp router-id", contract.PendingChangeActivate},
		{"create-entry", "", "set pending-kinds entry name gamma", "+ pending-kinds entry gamma", contract.PendingChangeSet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seed := "bgp {\n" + tc.prefix + `router-id 1.2.3.4;
    session { asn { local 65000; } }
}
pending-kinds {
    entry "alpha space" { value retained; }
}`
			path := filepath.Join(t.TempDir(), "test.conf")
			require.NoError(t, os.WriteFile(path, []byte(seed), 0o600))
			schema, err := config.YANGSchema()
			require.NoError(t, err)
			mgr := NewEditorManager(testConfigStore(t, path), path, schema, testEditorFactory(), testEditSessionFactory())
			t.Cleanup(func() { require.NoError(t, mgr.Close()) })
			_, err = mgr.ConfigCommand("alice", contract.ConfigCommandRequest{Line: tc.command})
			require.NoError(t, err)
			us, err := mgr.GetOrCreate("alice")
			require.NoError(t, err)
			pending := us.editor.PendingChanges(us.editor.SessionID())
			require.NotEmpty(t, pending, "the mutation must produce real pending intent")
			var kinds []contract.PendingChangeKind
			for _, change := range pending {
				kinds = append(kinds, change.Kind)
			}
			require.Contains(t, kinds, tc.kind)

			review, err := mgr.Diff("alice")
			require.NoError(t, err)
			assert.Contains(t, review, tc.row)
			answer, err := mgr.ConfigCommand("alice", contract.ConfigCommandRequest{Line: "show | compare"})
			require.NoError(t, err)
			assert.Equal(t, strings.TrimSpace(review), strings.TrimSpace(answer.Output))
			assert.Equal(t, pending, us.editor.PendingChanges(us.editor.SessionID()), "review must preserve the operation program")
		})
	}
}
