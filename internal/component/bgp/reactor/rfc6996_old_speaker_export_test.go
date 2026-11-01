// Design: docs/architecture/edge-cases/as4.md -- the RFC 6793 Section 4.2.2 send procedure
// RFC: rfc/short/rfc6996.md -- Private Use ASN removal before advertisement
// RFC: rfc/short/rfc6793.md -- AS_PATH and AS4_PATH toward an OLD speaker
// Related: as4_ingest_collapse_test.go -- collapseForwardIngest, the forward harness
// Related: rfc6996_remove_private_test.go -- rfc6996ExportPolicy, the remove-private export member
// Related: reactor_api_forward.go -- forwardUpdateCore, the export chain on the collapsed route
// Related: forward_body.go -- the transcode for a two-octet destination

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC requirement: RFC6996-4-1 positive -- toward a two-octet EBGP destination,
// the AS path the export chain stripped is what both path attributes carry: a
// route received from an OLD speaker with AS_PATH [65100 64496 64512 23456
// 23456 64497] and AS4_PATH [65100 64496 64512 4200000000 65536 64497],
// forwarded through ForwardUpdate with an export member answering as-path
// remove "private", reaches the destination with AS_PATH value exactly 0204
// fde8 fbf0 5ba0 fbf1 and AS4_PATH value exactly 0204 0000fde8 0000fbf0
// 00010000 0000fbf1: 65100, 64512 and 4200000000 are gone from both, and the
// local AS 65000 is prepended after the strip.
//
// RFC requirement: RFC6793-4.2.2-1 positive -- the same delivered AS_PATH is
// encoded with two-octet AS numbers, the non-mappable 65536 written as
// AS_TRANS 23456, as the exact AS_PATH octets above show.
//
// 65536 is a documentation ASN (RFC 5398) outside both Private Use ranges, so
// the strip keeps it and the transcode must carry it in AS4_PATH.
//
// RFC requirement: RFC6793-4.2.2-2 positive -- the same delivered UPDATE carries
// an AS4_PATH whose four-octet AS numbers are exactly [65000 64496 65536
// 64497], the path the AS_PATH carries with the real AS in place of AS_TRANS,
// as the exact AS4_PATH octets above show.
//
// VALIDATES: the AS4_PATH ze sends to an OLD speaker is rebuilt from the
// AS_PATH the export chain already rewrote, so RFC 6996 Section 4 "(including
// AS4_PATH if utilizing a four-octet AS number space)" holds on the wire.
// PREVENTS: the transcode deriving AS4_PATH from the received path and
// re-advertising the Private Use ASNs the strip removed, or narrowing without
// AS_TRANS.
func TestRFC6996OldSpeakerReceivesStrippedASPathAndAS4Path(t *testing.T) {
	attrs := collapseAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrASPath),
		collapseASPathValue(2, collapseSourceAS, 64496, 64512, collapseASTrans, collapseASTrans, 64497))...)
	attrs = append(attrs, collapseAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})...)
	attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
		collapseASPathValue(4, collapseSourceAS, 64496, 64512, 4200000000, 65536, 64497))...)

	const destAddr = "10.9.0.3"
	env := collapseForwardIngest(t, attrs, []collapseDest{{name: "two-octet eBGP destination", addr: destAddr, asn2: true}})

	transport := &externalTransport{reads: []string{"as-path"}, sets: []string{"as-path"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify,
			Sets: []rpc.FilterSet{intentSet("as-path", rpc.FilterSetActionRemove, `"private"`)}}}
	env.reactor.filterTransportSeam = transport
	env.reactor.orderedEgressSteps = []orderedEgressStep{{name: policyChainStepName, policyChain: true}}
	var dest *Peer
	for _, peer := range env.reactor.peers {
		if peer.Settings().Address.String() == destAddr {
			dest = peer
		}
	}
	require.NotNil(t, dest, "the destination peer must be in the reactor")
	dest.settings.ExportPolicy = rfc6996ExportPolicy(t)
	dest.refreshForwardFacts()

	sel, err := selector.Parse("*")
	require.NoError(t, err)
	require.NoError(t, env.api.ForwardUpdate(sel, env.updateID, "test-plugin", plugin.OperatorSender()))

	frame := env.await(t, 1)[destAddr]
	require.NotEmpty(t, transport.sent, "the export member must have been asked")
	assert.Equal(t, nlriHex(t, "0204 fde8 fbf0 5ba0 fbf1"), frame.asPathValue,
		"AS_PATH toward the OLD speaker: two-octet, stripped, AS_TRANS for 65536")
	assert.Equal(t, nlriHex(t, "0204 0000fde8 0000fbf0 00010000 0000fbf1"), frame.as4PathValue,
		"AS4_PATH toward the OLD speaker: four-octet, the same stripped path")
}
