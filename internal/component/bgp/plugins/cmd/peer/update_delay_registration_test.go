// VALIDATES: the YANG command tree builds the path `show bgp update-delay`, and
//            binds it to the wire method `ze-bgp:update-delay`. It reads the
//            TREE and nothing else: the RPC registry is not consulted, so this
//            says nothing about whether a handler is registered for that method.
//            The method string here is therefore a second copy of the literal in
//            peer.go's RegisterRPCs, unchecked against it.
//            That binding is proven by test/ui/bgp-update-delay-command.ci,
//            which types the command at a running daemon and reads a field out
//            of the answer: nothing but a registered handler can produce one.
// PREVENTS:  the defect this file was written for. The command node was first
//            declared in ze-bgp-cmd-peer-api.yang, and BuildCommandTree walks
//            modules whose name ends -cmd alone (cmdModuleSuffix), so no path
//            existed. Both dispatchers took their "no path" branch and
//            CONTINUED IN SILENCE, three documentation pages told an operator to
//            run a command nothing could answer, and the handler test could not
//            see it: calling the handler directly exercises the half that
//            worked.
//
// It is the mirror of TestShowBgpSummaryIsNotRegistered
// (internal/component/plugin/server/command_test.go), which proves a RETIRED
// path produces no command. DO NOT MOVE THIS BESIDE IT. It was written there
// first and passed while asserting nothing: the module is registered by this
// plugin's own yang package, internal/component/plugin/server does not import
// it, so `DefaultLoader` there returns a tree holding no BGP peer command at
// all. A NotContains assertion is safe in that package and a Contains assertion
// is vacuous in it, which is exactly the asymmetry that makes the two tests live
// apart.
//
// The assertion is on the PATH an operator types, not on the module the node
// happens to sit in, so moving the node again is fine and losing it is not.

package peer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

func TestShowBgpUpdateDelayIsRegistered(t *testing.T) {
	loader, err := yang.DefaultLoader()
	require.NoError(t, err, "load YANG")

	wireToPaths := yang.WireMethodToPaths(loader)
	require.Contains(t, wireToPaths, "ze-bgp:update-delay",
		"the wire method must appear in the YANG command tree, or no CLI path reaches the handler")
	assert.Contains(t, wireToPaths["ze-bgp:update-delay"], "show bgp update-delay",
		"the method must produce the path an operator types")

	tree := yang.BuildCommandTree(loader)
	require.NotNil(t, tree, "the command tree must build")
}
