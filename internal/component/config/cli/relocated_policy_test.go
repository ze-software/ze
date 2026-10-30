//go:build ze_bgp

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// oldBGPPolicyConfig is a file written before the policy block left bgp: a
// prefix-list under `bgp { policy { ... } }`.
const oldBGPPolicyConfig = `bgp {
	policy {
		prefix-list CUSTOMERS {
			entry 10.0.0.0/8 {
				ge 16
				le 24
				action accept
			}
		}
	}
}
`

// assertRelocationRefusal checks that err is the refusal of the old position:
// it names the new top-level policy root and the exact migrate command.
func assertRelocationRefusal(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "policy is no longer inside bgp")
	assert.Contains(t, msg, "top-level policy { ... } block")
	assert.Contains(t, msg, "fix: ze config migrate -o <new-file> <file>")
	assert.NotContains(t, msg, "did you mean", "the relocation hint replaces the closest-key guess")
}

// TestOldBGPPolicyRefusedNamingNewRoot: a config that still nests policy in
// bgp is refused at load, by `ze config validate`, and in the set form, each
// naming the top-level policy root and the migrate command (AC-2).
// Method: parse the file with the load schema in both forms, and validate it.
func TestOldBGPPolicyRefusedNamingNewRoot(t *testing.T) {
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	_, err = config.NewParser(schema).Parse(oldBGPPolicyConfig)
	assertRelocationRefusal(t, err)

	_, err = config.NewSetParser(schema).Parse("set bgp policy prefix-list CUSTOMERS entry 10.0.0.0/8 action accept\n")
	assertRelocationRefusal(t, err)

	assertRelocationRefusal(t, ValidateContent(oldBGPPolicyConfig, "old.conf"))
}

// TestMigrateMovesBGPPolicyToRoot: `ze config migrate` rewrites
// `bgp { policy { ... } }` as `policy { ... }` and the result loads (AC-2).
// Method: migrate the old file in both output forms, check the policy block
// left bgp with its entry intact, and parse the output with the load schema.
func TestMigrateMovesBGPPolicyToRoot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.conf")
	require.NoError(t, os.WriteFile(path, []byte(oldBGPPolicyConfig), 0o600))

	schema, err := config.YANGSchema()
	require.NoError(t, err)

	for _, form := range []string{formSet, formHierarchical} {
		output, result, _, err := configMigrateWithWarnings(path, "", form)
		require.NoError(t, err, form)
		assert.Contains(t, result.Applied, "relocated-blocks", form)

		var tree *config.Tree
		if form == formSet {
			assert.Contains(t, output, "set policy prefix-list CUSTOMERS entry 10.0.0.0/8 action accept", form)
			assert.NotContains(t, output, "set bgp policy", form)
			tree, err = config.NewSetParser(schema).Parse(output)
		} else {
			tree, err = config.NewParser(schema).Parse(output)
		}
		require.NoError(t, err, "the migrated %s output must load: %s", form, output)

		policy := tree.GetContainer("policy")
		require.NotNil(t, policy, form)
		entry := policy.GetList("prefix-list")["CUSTOMERS"]
		require.NotNil(t, entry, form)
		if bgp := tree.GetContainer("bgp"); bgp != nil {
			assert.Nil(t, bgp.GetContainer("policy"), form)
		}
	}
}

// TestMigrateRefusesPolicyEntryAtBothPositions: an entry configured under
// both the old and the new position is refused rather than one silently
// overwriting the other.
// Method: migrate a file carrying prefix-list CUSTOMERS in bgp policy and in
// the top-level policy root.
func TestMigrateRefusesPolicyEntryAtBothPositions(t *testing.T) {
	content := oldBGPPolicyConfig + "policy {\n\tprefix-list CUSTOMERS {\n\t\tentry 192.0.2.0/24 {\n\t\t\taction accept\n\t\t}\n\t}\n}\n"
	path := filepath.Join(t.TempDir(), "both.conf")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, _, _, err := configMigrateWithWarnings(path, "", formSet)
	require.Error(t, err, content)
	assert.Contains(t, err.Error(), "bgp policy prefix-list CUSTOMERS and policy prefix-list CUSTOMERS")
}
