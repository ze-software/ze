// Design: docs/architecture/wire/attributes.md -- AS_PATH toward an internal peer
// Related: filter_ordered.go -- runEgressPolicyChainASN4, the export chain every egress rail runs

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportFilterTo runs the export chain with an operator filter whose answer
// is the given text delta, toward a destination of the given AS with local AS
// 65000, and answers the AS_PATH that leaves (the body's own path when the
// chain produced no override).
func exportFilterTo(t *testing.T, delta string, body []byte, destPeerAS uint32) []uint32 {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)

	r := &Reactor{
		api: &pluginserver.Server{},
		policyFilterSeam: func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
			return PolicyResponse{Action: PolicyModify, Delta: delta}
		},
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	res := r.runEgressPolicyChainASN4([]filterapi.FilterRef{{Name: "aspath-edit"}},
		"10.0.0.30", destPeerAS, 65000, wireu.NewWireUpdate(body, ctxID), true)
	require.True(t, res.accept, "the export chain accepts the route")

	payload := body
	if res.wireOverride != nil {
		payload = res.wireOverride.Payload()
	}
	value, found := attrValueFromPayload(t, payload, attribute.AttrASPath)
	require.True(t, found, "the route leaves with an AS_PATH")
	path, err := attribute.ParseASPath(value, true)
	require.NoError(t, err)
	return flatASNs(path)
}

// privateASPathBody answers an UPDATE body whose four-octet AS_PATH is one
// AS_SEQUENCE [64512 64496]: a private AS (RFC 6996) ahead of a public one.
func privateASPathBody() []byte {
	_, payload := prependFixture([]byte{byte(attribute.ASSequence), 2, 0, 0, 0xFC, 0x00, 0, 0, 0xFB, 0xF0}, nil)
	return payload
}

// TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath drives the two
// operator inputs that ask Ze to modify the AS_PATH of a route it advertises:
// an export filter answering "as-path-prepend 2", and one answering
// "remove-private strip".
//
// VALIDATES: toward an internal peer (peer AS = local AS) the route leaves with
// its AS_PATH unchanged, [64496] and [64512 64496]; the same filters toward an
// external peer do modify it, [65000 65000 64496] and [64496], so each filter
// really fired.
// PREVENTS: an export policy breaking the IBGP AS_PATH invariant, which every
// internal peer relies on for loop detection and the path-length step.
//
// RFC requirement: RFC4271-5.1.2-2 negative -- an export as-path-prepend or
// remove-private toward an internal peer is not applied:
// runEgressPolicyChainASN4 leaves the AS_PATH unchanged, while the
// external-peer controls are prepended and stripped.
func TestRFC4271ExportPrependNeverModifiesAnInternalPeersASPath(t *testing.T) {
	assert.Equal(t, []uint32{64496}, exportFilterTo(t, "as-path-prepend 2", asPathBodyAtWidth(true), 65000),
		"internal peer: the prepend leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{65000, 65000, 64496}, exportFilterTo(t, "as-path-prepend 2", asPathBodyAtWidth(true), 65001),
		"external peer: the same filter prepends the local AS twice")
	assert.Equal(t, []uint32{64512, 64496}, exportFilterTo(t, "remove-private strip", privateASPathBody(), 65000),
		"internal peer: remove-private leaves the AS_PATH unmodified")
	assert.Equal(t, []uint32{64496}, exportFilterTo(t, "remove-private strip", privateASPathBody(), 65001),
		"external peer: the same filter strips the private AS")
}
