// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// RFC: rfc/short/rfc6793.md -- AS path reconstruction on receive (Section 4.2.3)

package filter_path_asn

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// subjectFromWire renders the filter subject the reactor hands this plugin for
// one UPDATE, through the producer itself so this test and the running daemon
// cannot drift. asn4 is what the session negotiated.
func subjectFromWire(t *testing.T, asn4 bool, attrs ...[]byte) string {
	t.Helper()
	var packed []byte
	for _, a := range attrs {
		packed = append(packed, a...)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	require.NoError(t, err)
	return string(reactor.AppendUpdateForFilter(nil, attribute.NewAttributesWire(packed, ctxID), nil, nil))
}

// wireAttr wraps one attribute value in its wire header.
func wireAttr(flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) []byte {
	buf := make([]byte, 3+len(value))
	attribute.WriteHeaderTo(buf, 0, flags, code, uint16(len(value))) //nolint:gosec // test data, bounded
	copy(buf[3:], value)
	return buf
}

// wirePath2 encodes one AS_SEQUENCE with two-octet AS numbers, what an OLD
// speaker puts in AS_PATH.
func wirePath2(asns ...uint32) []byte {
	value := make([]byte, 2+len(asns)*2)
	value[0] = byte(attribute.ASSequence)
	value[1] = byte(len(asns))
	for i, asn := range asns {
		binary.BigEndian.PutUint16(value[2+i*2:], uint16(asn)) //nolint:gosec // test data, mappable by construction
	}
	return value
}

// wirePath4 encodes one AS_SEQUENCE with four-octet AS numbers, what AS4_PATH
// always carries.
func wirePath4(asns ...uint32) []byte {
	value := make([]byte, 2+len(asns)*4)
	value[0] = byte(attribute.ASSequence)
	value[1] = byte(len(asns))
	for i, asn := range asns {
		binary.BigEndian.PutUint32(value[2+i*4:], asn)
	}
	return value
}

// TestRejectsFourOctetASNBehindASTrans drives the plugin from the subject the
// reactor really builds, for a peer that did not negotiate four-octet AS.
//
// The route traversed AS199524, which is what the operator listed. On the wire
// the AS_PATH says 23456 because two octets cannot hold that number, and the
// real path travels in AS4_PATH (RFC 6793 Section 4.2.2). A filter reading the
// placeholder accepts the very leak the list exists to reject.
//
// VALIDATES: a reject-asn list naming a four-octet ASN rejects a route that
// reached it through an OLD-speaker session.
// PREVENTS: a fail-open that no test over the plugin alone can see, because
// the plugin is handed a subject that is already wrong.
func TestRejectsFourOctetASNBehindASTrans(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 199524 ]
        }`)

	subject := subjectFromWire(t, false,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath2(65001, attribute.ASTrans, 65002)),
		wireAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			wirePath4(65001, 199524, 65002)),
	)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: subject,
	})
	assert.Equal(t, sdk.FilterReject, out.Action,
		"the route reached 199524 through the peer, so indirect must reject it")
}

// TestRejectsFourOctetASNOnFourOctetSession is the paired positive: the same
// route on a session that negotiated four-octet AS, where AS_PATH carries the
// real number and no AS4_PATH is sent.
//
// VALIDATES: the reject that already worked keeps working.
// PREVENTS: a fix that trades the common path for the OLD-speaker one.
func TestRejectsFourOctetASNOnFourOctetSession(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 199524 ]
        }`)

	subject := subjectFromWire(t, true,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath4(65001, 199524, 65002)),
	)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: subject,
	})
	assert.Equal(t, sdk.FilterReject, out.Action)
}

// TestAcceptsUnlistedPathBehindASTrans is the negative arm: the same
// OLD-speaker encoding, a list naming an ASN the reconstructed path does not
// hold, and the route is accepted.
//
// VALIDATES: the merged subject is judged rather than trusted -- rejecting
// everything would pass the two tests above.
// PREVENTS: a merge that pulls unrelated AS numbers into the subject.
func TestAcceptsUnlistedPathBehindASTrans(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 199525 ]
        }`)

	subject := subjectFromWire(t, false,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath2(65001, attribute.ASTrans, 65002)),
		wireAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			wirePath4(65001, 199524, 65002)),
	)

	out := handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
		Update: subject,
	})
	assert.Equal(t, sdk.FilterAccept, out.Action,
		"199525 is not in the reconstructed path, so the route is accepted")
}
