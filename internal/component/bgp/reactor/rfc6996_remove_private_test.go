package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc6996AS4Path encodes one AS4_PATH value: an AS_SEQUENCE of the given ASNs.
func rfc6996AS4Path(asns ...uint32) []byte {
	path := &attribute.AS4Path{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}}
	buf := make([]byte, path.Len())
	path.WriteTo(buf, 0)
	return buf
}

// rfc6996Strip runs the remove-private strip intent over an UPDATE whose
// only attribute is the given AS4_PATH, and returns the ops the reactor applies
// before the route is advertised. removePrivateASOps is the producer the
// library chain's remove-private set reaches (filter_library.go
// libraryASPathOps).
func rfc6996Strip(t *testing.T, as4Path []byte) *filterapi.ModAccumulator {
	t.Helper()
	attrsWire := attribute.NewAttributesWire(makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Path), 0)
	var mods filterapi.ModAccumulator
	removePrivateASOps(newTestScratch(t), attrsWire, true, policychain.RemovePrivateStrip, 65001, &mods)
	return &mods
}

// RFC requirement: RFC6996-4-1 positive -- Private Use ASNs are removed from
// AS4_PATH before the route is advertised: an AS4_PATH of [64496 4200000000
// 64512 4294967294 64497] is rewritten to [64496 64497], both ends of the
// four-octet range and the two-octet range removed.
//
// VALIDATES: RFC 6996 Section 4 "(including AS4_PATH if utilizing a four-octet AS number space)".
// PREVENTS: the reactor rewriting AS_PATH alone and leaving Private Use ASNs in AS4_PATH.
func TestRFC6996StripRemovesPrivateUseASNsFromAS4Path(t *testing.T) {
	mods := rfc6996Strip(t, rfc6996AS4Path(64496, 4200000000, 64512, 4294967294, 64497))
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	assert.Equal(t, byte(attribute.AttrAS4Path), op.Code)
	assert.Equal(t, filterapi.AttrModSet, op.Action)
	assert.Equal(t, rfc6996AS4Path(64496, 64497), op.Buf)
}

// RFC requirement: RFC6996-4-1 negative -- an AS4_PATH that holds only ASNs
// outside the Private Use ranges (64511 65535 4199999999 4294967295) draws no
// rewrite: the reactor emits no op and the attribute is advertised unchanged.
//
// VALIDATES: the ASNs adjacent to each Private Use range survive the strip.
// PREVENTS: a widened range removing ASNs RFC 6996 does not reserve.
func TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges(t *testing.T) {
	mods := rfc6996Strip(t, rfc6996AS4Path(64511, 65535, 4199999999, 4294967295))
	assert.Equal(t, 0, mods.Len())
}

// rfc6996ExportPolicy compiles the one external member probe:f at the intent
// test export attachment, standing for the remove-private-as plugin: it reads
// the AS path and answers the as-path remove intent.
func rfc6996ExportPolicy(t *testing.T) filterapi.Policy {
	t.Helper()
	reg, err := routepolicy.NewRegistry()
	require.NoError(t, err)
	members := []policychain.Member{{Name: "f", Plugin: "probe"}}
	chain, err := policychain.Compile(libraryIntentExport, reg, members, routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
	require.NoError(t, err)
	return mustPolicy(t, []routepolicy.Ref{{Name: "probe:f"}}, routepolicy.FinalAccept).WithLibrary(chain)
}

// RFC requirement: RFC6996-4-1 positive -- Private Use ASNs are removed from
// AS_PATH before the route is advertised: an external filter answering the
// as-path remove "private" intent on the export step toward an EBGP peer turns
// a four-octet AS_PATH of [64511 64512 65534 65535 4199999999 4200000000
// 4294967294 4294967295] into exactly [64511 65535 4199999999 4294967295],
// both ends of both Private Use ranges removed and their neighbors kept.
//
// VALIDATES: RFC 6996 Section 4 "Private Use ASNs MUST be removed from AS path attributes".
// PREVENTS: the reactor dropping the remove-private intent, or rewriting AS_PATH wrongly at a range edge.
func TestRFC6996ExportRemovesPrivateUseASNsFromASPath(t *testing.T) {
	received := nlriHex(t, "0000 0030 40010100 400222 0208"+
		" 0000fbff 0000fc00 0000fffe 0000ffff fa56e9ff fa56ea00 fffffffe ffffffff"+
		" 40030401010101 180a0001")
	want := nlriHex(t, "0000 0020 40010100 400212 0204"+
		" 0000fbff 0000ffff fa56e9ff ffffffff"+
		" 40030401010101 180a0001")
	transport := &externalTransport{reads: []string{"as-path"}, sets: []string{"as-path"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterModify,
			Sets: []rpc.FilterSet{intentSet("as-path", rpc.FilterSetActionRemove, `"private"`)}}}
	// The filter is sent the AS path as JSON, which needs the receive
	// context's four-octet AS number width.
	ctxID, err := bgpctx.Registry.Register(bgpctx.NewEncodingContext(
		&capability.PeerIdentity{LocalASN: 65000, PeerASN: 65001},
		&capability.EncodingCaps{ASN4: true}, bgpctx.DirectionRecv))
	require.NoError(t, err)
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	out := r.runEgressPolicyChainASN4(rfc6996ExportPolicy(t), "10.0.0.5", 65002, 65000, false, policyview.Session{EBGP: true}, wireu.NewWireUpdate(received, ctxID), true)
	require.True(t, out.accept, "export refused the route")
	require.NotNil(t, out.wireOverride, "export left the AS_PATH unchanged")
	assert.Equal(t, want, out.wireOverride.Payload())
}
