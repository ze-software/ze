package redistribute

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
)

// TestImportFiltersReferenceChecked proves the policy-filter reference source
// knows a redistribute import chain: completion offers the match instances
// the chain compiles, and the reference check `ze config validate` and commit
// run refuses a name the policy root does not define.
//
// VALIDATES: redistribute destination * import * filters resolves to the
// redistribute import attachment (MustRegisterRouteChainPath).
// PREVENTS: a misspelled filter name that validates offline and fails only
// when the daemon compiles the chain.
func TestImportFiltersReferenceChecked(t *testing.T) {
	resolved, err := yang.DefaultLoader()
	require.NoError(t, err)
	v := yang.NewValidator(resolved)

	tree, err := config.ParseTreeWithYANG(`
policy { match TAGGED { tag [ 42 ]; } }
redistribute {
	destination ospf {
		import static { filters [ TAGGED ]; final accept; }
		import connected { filters [ MISSING ]; final accept; }
	}
}
`, nil)
	require.NoError(t, err)
	root := tree.ToPluginMap()

	offered := v.ReferenceInstances(root, []string{"redistribute", "destination", "ospf", "import", "static", "filters"})
	assert.True(t, slices.Contains(offered, "TAGGED"), "completion offers the match instance, got %v", offered)

	var refused []string
	for _, ve := range v.ValidateReferences(root) {
		refused = append(refused, ve.Path+": "+ve.Message)
	}
	require.Len(t, refused, 1, "only the undefined name is refused: %v", refused)
	assert.Contains(t, strings.Join(refused, "\n"), "MISSING")
}
