// VALIDATES: a leafref in the bgp section naming nothing refuses `commit now`
// and `commit verify` through the editor, and naming a configured node lets
// the commit through.
// PREVENTS: the editor dropping a bgp reference failure with the bgp type
// failures its own YANG pass reports, so `commit` wrote a config the daemon
// then refused to load.

package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// bgpReferenceProbe augments the bgp section with a leafref, which no Ze
// module puts there today: probe-ref names a probe-name.
const bgpReferenceProbe = `module ze-probe-ref {
    namespace "urn:ze:probe-ref";
    prefix probe;
    import ze-bgp-conf { prefix bgp; }
    augment "/bgp:bgp" {
        leaf probe-name { type string; }
        leaf probe-ref { type leafref { path "../probe-name"; } }
    }
}`

// TestCommitRefusesBGPReferenceFailure drives `commit verify` and `commit now`
// over a bgp leafref naming nothing, then over one naming a configured node.
// The module is registered before the model is built, so the editor's schema
// and validator both carry it.
func TestCommitRefusesBGPReferenceFailure(t *testing.T) {
	t.Cleanup(yang.RegisterModuleForTest("ze-probe-ref.yang", bgpReferenceProbe))
	model, store, configPath := newCommitGrammarModel(t, "")

	_, err := model.dispatchCommand("set bgp probe-ref missing")
	require.NoError(t, err)

	result, err := model.dispatchCommand("commit verify")
	require.NoError(t, err)
	assert.Contains(t, result.statusMessage, "1 error(s)")
	assert.Contains(t, result.statusMessage, `no probe-name "missing" exists`)

	result, err = model.dispatchCommand("commit now")
	require.NoError(t, err, "a blocked commit is reported as status")
	assert.Contains(t, result.statusMessage, "commit blocked: 1 error(s)")
	assert.Contains(t, result.statusMessage, `no probe-name "missing" exists`)
	committed, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(committed), "probe-ref", "a refused commit applied the candidate")

	_, err = model.dispatchCommand("set bgp probe-name missing")
	require.NoError(t, err)
	validation := model.validator.ValidateTransition(model.editor.OriginalContent(), model.editor.WorkingContent())
	assert.Empty(t, validation.Errors, "a leafref naming a configured node is valid")
}
