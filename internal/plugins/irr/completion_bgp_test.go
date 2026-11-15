//go:build ze_bgp

package irr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"

	// bgp-filter-irr's `irr as-set` leaf carries `ze:validate "irr-as-set"`;
	// linking its YANG and the BGP schema it augments puts the leaf in the
	// schema the editor completes over. Both sit behind ze_bgp.
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_irr/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
)

// TestEditorOffersStoredASSetsOnTheBGPLeaf proves the BGP consumer's AS-SET
// leaf completes from the same store as the firewall leaves.
//
// VALIDATES: spec-fetched-data-survives-restart B-5, the config-leaf half on
// `bgp peer <name> session irr as-set`, through Completer.Complete.
// PREVENTS: the ze:validate line dropped from bgp-filter-irr's leaf, which the
// firewall tests in completion_test.go would not see.
func TestEditorOffersStoredASSetsOnTheBGPLeaf(t *testing.T) {
	seedASSetState(t, useDaemonStore(t))
	completer, err := cli.NewCompleter()
	require.NoError(t, err)

	offered := make(map[string]string)
	for _, completion := range completer.Complete(nil, "set bgp peer peer1 session irr as-set ", nil) {
		offered[completion.Text] = completion.ShortHelp
	}
	want := make(map[string]string, len(wantKnown))
	for _, set := range wantKnown {
		want[set.name] = set.help()
	}
	assert.Equal(t, want, offered)
}
