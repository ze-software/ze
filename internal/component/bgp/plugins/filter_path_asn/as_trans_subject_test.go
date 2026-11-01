// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// RFC: rfc/short/rfc6793.md -- AS path reconstruction on receive (Section 4.2.3)

package filter_path_asn

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// judgeReceived answers the reject-asn list's verdict on one UPDATE as a
// session receives it: attrs (with one IPv4 prefix) on a session whose
// negotiated four-octet AS is asn4, from peerAS. The payload goes through the
// receive reconciliation the session read path runs
// (wireu.CollapseAS4Family, RFC 6793 Section 4.2.3), and the result is what
// the policy view reads, so the merge under test is the daemon's own. The
// condition (conditionCheck) then judges it on a live run.
func judgeReceived(t *testing.T, filter string, asn4 bool, peerAS uint32, attrs ...[]byte) sdk.FilterAction {
	t.Helper()
	held := installedRejectASN.Load()
	require.NotNil(t, held, "no reject-asn config installed")
	list, ok := held.lists[filter]
	require.True(t, ok, "reject-asn list %q is not installed", filter)

	var packed []byte
	for _, a := range attrs {
		packed = append(packed, a...)
	}
	payload := binary.BigEndian.AppendUint16(nil, 0)
	payload = binary.BigEndian.AppendUint16(payload, uint16(len(packed))) //nolint:gosec // G115: a test attribute block
	payload = append(payload, packed...)
	payload = append(payload, 24, 10, 1, 0)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	require.NoError(t, err)
	update := wireu.NewWireUpdate(payload, ctxID)

	dst := make([]byte, wireu.CollapseAS4FamilySize(payload))
	n, _, err := wireu.CollapseAS4Family(dst, payload, asn4)
	require.NoError(t, err)
	if n > 0 {
		collapsedID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
		require.NoError(t, err)
		update = wireu.NewWireUpdate(dst[:n:n], collapsedID)
	}

	var view policyview.View
	view.Reset(update, policyview.Session{EBGP: true, Sender: peerAS, SenderKnown: true})
	defer view.Clear()
	passed, err := conditionCheck(list)(&policychain.MatchRoute{View: &view, Reports: true})
	require.NoError(t, err)
	if passed {
		return sdk.FilterAccept
	}
	return sdk.FilterReject
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

// TestRejectsFourOctetASNBehindASTrans drives the condition from the UPDATE
// the session read path really produces, for a peer that did not negotiate
// four-octet AS: the received AS_PATH and AS4_PATH are merged by the receive
// reconciliation before the view reads the path.
//
// The route traversed AS199524, which is what the operator listed. On the wire
// the AS_PATH says 23456 because two octets cannot hold that number, and the
// real path travels in AS4_PATH (RFC 6793 Section 4.2.2). A filter reading the
// placeholder accepts the very leak the list exists to reject.
//
// VALIDATES: a reject-asn list naming a four-octet ASN rejects a route that
// reached it through an OLD-speaker session.
// PREVENTS: a fail-open that no test over the condition alone can see,
// because the condition is handed a path that is already wrong.
//
// MUTATION: PrepareAS4Family (wireu/aspath_collapse.go) reconciling without
// the received AS4_PATH (AS4Path: nil, so the placeholder 23456 stays in the
// path) turns this test red: the route is accepted.
func TestRejectsFourOctetASNBehindASTrans(t *testing.T) {
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 199524 ]
        }`)

	got := judgeReceived(t, "NO-TRANSIT", false, 65001,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath2(65001, attribute.ASTrans, 65002)),
		wireAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			wirePath4(65001, 199524, 65002)),
	)
	assert.Equal(t, sdk.FilterReject, got,
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

	got := judgeReceived(t, "NO-TRANSIT", true, 65001,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath4(65001, 199524, 65002)),
	)
	assert.Equal(t, sdk.FilterReject, got)
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

	got := judgeReceived(t, "NO-TRANSIT", false, 65001,
		wireAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1}),
		wireAttr(attribute.FlagTransitive, attribute.AttrASPath,
			wirePath2(65001, attribute.ASTrans, 65002)),
		wireAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			wirePath4(65001, 199524, 65002)),
	)
	assert.Equal(t, sdk.FilterAccept, got,
		"199525 is not in the reconstructed path, so the route is accepted")
}
