package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/locatororigin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These tests drive whole UPDATE bodies through enforceRFC7606, the receive path, with
// the UPDATE-level check the mup plugin's init() registers (mup_test.go links the plugin
// into this binary). Every UPDATE is ORIGIN IGP, an empty AS_PATH and an MP_REACH_NLRI
// for AFI 1 / SAFI 85 with next hop 192.0.2.1; draft-ietf-bess-mup-safi Section 3.1
// frames each NLRI as [architecture-type:1][route-type:2][length:1][value], RD 65001:100.
const (
	// dsdISDHex is an Interwork Segment Discovery route for 10.0.0.0/24 (Section 3.1.1).
	dsdISDHex = "0100010c0000fde90000006418" + "0a0000"
	// dsdDSDHex is a Direct Segment Discovery route for 192.0.2.1 (Section 3.1.2).
	dsdDSDHex = "0100020c0000fde900000064" + "c0000201"
	// dsdT2STHex is a Type 2 Session Transformed route, endpoint 192.0.2.1, TEID 1
	// (Section 3.1.4).
	dsdT2STHex = "010004110000fde90000006440" + "c0000201" + "00000001"
	// dsdPrefixSIDHex is a Prefix-SID attribute (type 40) holding an SRv6 L3 Service TLV
	// (RFC 9252 Section 2) with one SRv6 SID Information Sub-TLV: SID 2001:db8::1,
	// behavior End.DT4.
	dsdPrefixSIDHex = "c0281c" + "05001900" + "01001500" +
		"20010db8000000000000000000000001" + "00001300"
)

// dsdUpdateBody decodes one UPDATE body: no withdrawn routes, the attribute section
// attrsHex, no IPv4 NLRI.
func dsdUpdateBody(t *testing.T, attrsHex string) []byte {
	t.Helper()
	attrs, err := hex.DecodeString(attrsHex)
	require.NoError(t, err)
	return makeUpdateBody(nil, attrs, nil)
}

// dsdUpdateAttrsHex builds ORIGIN, AS_PATH and an MP_REACH_NLRI holding nlriHex,
// followed by tail (another attribute, or nothing).
func dsdUpdateAttrsHex(t *testing.T, nlriHex, tail string) string {
	t.Helper()
	nlri, err := hex.DecodeString(nlriHex)
	require.NoError(t, err)
	mpReach := hex.EncodeToString([]byte{0x80, 0x0e, byte(9 + len(nlri))}) +
		"00015504c000020100" + nlriHex
	return "40010100" + "400200" + mpReach + tail
}

// mupCountMPUnreachAttrs counts occurrences of MP_UNREACH_NLRI inside body's path attribute section.
func mupCountMPUnreachAttrs(t *testing.T, body []byte) int {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	attrs := sections.Attrs(body)
	count := 0
	pos := 0
	for pos+2 <= len(attrs) {
		flags := attribute.AttributeFlags(attrs[pos])
		cur := attribute.AttributeCode(attrs[pos+1])
		pos += 2
		var attrLen int
		if flags&attribute.FlagExtLength == 0 {
			require.LessOrEqual(t, pos+1, len(attrs))
			attrLen = int(attrs[pos])
			pos++
		} else {
			require.LessOrEqual(t, pos+2, len(attrs))
			attrLen = int(attrs[pos])<<8 | int(attrs[pos+1])
			pos += 2
		}
		require.LessOrEqual(t, pos+attrLen, len(attrs))
		if cur == attribute.AttrMPUnreachNLRI {
			count++
		}
		pos += attrLen
	}
	return count
}

// TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive drives the Section 3.3.6 sentence through
// the receive path: "When a BGP speaker receives a MP_REACH_NLRI attribute update message
// with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be
// treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw
// procedure of [RFC7606] is applied.  A BGP speaker MUST skip such NLRIs and continue
// processing of rest of the Update message."
//
// VALIDATES: a DSD route with no Prefix-SID leaves MP_REACH_NLRI and is withdrawn in an
// MP_UNREACH_NLRI while its Type 2 ST sibling stays; the same holds when the Prefix-SID is
// present but the RFC 8669 Section 4 EBGP rule discards it; no session reset.
// PREVENTS: a DSD route that carries no SID being installed and relayed.
func TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	locatororigin.ResetForTest()

	t2st, err := hex.DecodeString(dsdT2STHex)
	require.NoError(t, err)
	dsd, err := hex.DecodeString(dsdDSDHex)
	require.NoError(t, err)

	cases := []struct {
		name   string
		accept bool
		tail   string
	}{
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 positive -- a DSD route in an MP_REACH_NLRI with no Prefix-SID attribute leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, its Type 2 ST sibling is kept, and the session is not reset
		{"no Prefix-SID attribute", true, ""},
		{"Prefix-SID discarded from an EBGP peer", false, dsdPrefixSIDHex},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := rfc8669EBGPSession(tc.accept)
			body := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdT2STHex+dsdDSDHex, tc.tail))
			wu, _, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err, "a DSD route without a Prefix-SID is never a session reset")

			kept, found := mpReachNLRIOf(t, wu.Payload())
			require.True(t, found)
			assert.Equal(t, t2st, kept, "only the DSD route leaves the announcement")
			withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
			require.True(t, found, "the DSD route is withdrawn, not dropped")
			assert.Equal(t, dsd, withdrawn)
		})
	}
}

// TestMUPDSDWithPrefixSIDKeptOnReceive is the other polarity, and also verifies
// Section 3.3.3 on an UPDATE carrying ISD and T2ST without a Prefix-SID.
//
// VALIDATES: a DSD route whose UPDATE carries a Prefix-SID is returned unchanged,
// Type 2 ST routes with no Prefix-SID are returned unchanged, and ISD routes
// without a Prefix-SID are withdrawn while their Type 2 ST sibling stays kept.
// PREVENTS: the check withdrawing a DSD route that carries its SID, or reaching a route
// type neither Section 3.3.3 nor Section 3.3.6 names.
func TestMUPDSDWithPrefixSIDKeptOnReceive(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	locatororigin.ResetForTest()

	cases := []struct {
		name  string
		attrs string
	}{
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 negative -- a DSD route in an MP_REACH_NLRI whose UPDATE carries a Prefix-SID attribute is kept, and Type 2 ST routes with no Prefix-SID are kept
		{"DSD with a Prefix-SID", dsdUpdateAttrsHex(t, dsdISDHex+dsdDSDHex, dsdPrefixSIDHex)},
		{"T2ST without a Prefix-SID", dsdUpdateAttrsHex(t, dsdT2STHex, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := rfc8669EBGPSession(true)
			body := dsdUpdateBody(t, tc.attrs)
			wu, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
			require.NoError(t, err)
			assert.Equal(t, message.RFC7606ActionNone, action)
			assert.Equal(t, body, wu.Payload(), "the UPDATE is returned byte for byte")
		})
	}

	t.Run("ISD without a Prefix-SID is withdrawn while T2ST stays kept", func(t *testing.T) {
		isd, err := hex.DecodeString(dsdISDHex)
		require.NoError(t, err)
		t2st, err := hex.DecodeString(dsdT2STHex)
		require.NoError(t, err)

		s := rfc8669EBGPSession(true)
		body := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdISDHex+dsdT2STHex, ""))
		wu, _, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
		require.NoError(t, err)

		kept, found := mpReachNLRIOf(t, wu.Payload())
		require.True(t, found)
		assert.Equal(t, t2st, kept, "T2ST sibling without Prefix-SID stays kept")
		withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
		require.True(t, found, "ISD without Prefix-SID is withdrawn")
		assert.Equal(t, isd, withdrawn)
	})
}

// TestRejectedMUPWithdrawDispatch drives DSD and ISD rejections (both missing
// Prefix-SID and proven locator mismatch) through applyRFC7606 and
// processValidatedMessage on (a) an UPDATE that already holds another family's
// MP_UNREACH_NLRI and (b) an UPDATE whose routes are all rejected.
//
// VALIDATES: existing MP_UNREACH_NLRI stays byte-identical, each UPDATE body
// carries at most one MP_UNREACH_NLRI, validation.typedWithdrawBodies carries
// the MUP withdrawal when MP_UNREACH_NLRI is already occupied, and every
// rejected route dispatches a withdrawal even when all routes in the UPDATE
// are rejected.
// PREVENTS: duplicate MP_UNREACH_NLRI causing RFC 7606 Section 3(g) session
// resets, dropped withdrawals when all routes in an UPDATE are rejected, or a
// forged End-of-RIB marker.
func TestRejectedMUPWithdrawDispatch(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	locatororigin.ResetForTest()

	dsdBytes, err := hex.DecodeString(dsdDSDHex)
	require.NoError(t, err)
	isdBytes, err := hex.DecodeString(dsdISDHex)
	require.NoError(t, err)
	t2stBytes, err := hex.DecodeString(dsdT2STHex)
	require.NoError(t, err)
	otherUnreach := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0, 0x09, 0, 0}
	otherUnreachAttr := mpUnreachAttrs(family.IPv6Unicast, otherUnreach)

	cases := []struct {
		name             string
		rejectedHex      string
		rejectedBytes    []byte
		tail             string
		seedKnownLocator bool
	}{
		{
			name:          "DSD missing Prefix-SID",
			rejectedHex:   dsdDSDHex,
			rejectedBytes: dsdBytes,
		},
		{
			name:          "ISD missing Prefix-SID",
			rejectedHex:   dsdISDHex,
			rejectedBytes: isdBytes,
		},
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 positive -- a DSD route rejected on a proven locator-originator mismatch is stripped from MP_REACH_NLRI and dispatched as an MP_UNREACH_NLRI withdrawal whether or not the UPDATE already holds another family's MP_UNREACH_NLRI or all routes are rejected
		{
			name:             "DSD proven locator mismatch",
			rejectedHex:      dsdDSDHex,
			rejectedBytes:    dsdBytes,
			tail:             dsdPrefixSIDHex,
			seedKnownLocator: true,
		},
		// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4 positive -- an ISD route rejected on a proven locator-originator Address mismatch is stripped from MP_REACH_NLRI and dispatched as an MP_UNREACH_NLRI withdrawal whether or not the UPDATE already holds another family's MP_UNREACH_NLRI or all routes are rejected
		{
			name:             "ISD proven locator mismatch",
			rejectedHex:      dsdISDHex,
			rejectedBytes:    isdBytes,
			tail:             dsdPrefixSIDHex,
			seedKnownLocator: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			locatororigin.ResetForTest()
			if tc.seedKnownLocator {
				locatororigin.ApplySnapshot(&locatororigin.Snapshot{
					Source:     "isis",
					Generation: 1,
					Active:     true,
					Entries: []locatororigin.LocatorEntry{
						{
							Prefix:     netip.MustParsePrefix("2001:db8::/48"),
							Known:      true,
							Originator: "0100.0000.0001",
							Addresses:  []netip.Addr{netip.MustParseAddr("2001:db8:ffff::1")},
						},
					},
				})
			}

			t.Run("another family already carries MP_UNREACH_NLRI", func(t *testing.T) {
				attrsHex := dsdUpdateAttrsHex(t, dsdT2STHex+tc.rejectedHex, tc.tail)
				rawAttrs, decodeErr := hex.DecodeString(attrsHex)
				require.NoError(t, decodeErr)
				rawAttrs = append(rawAttrs, otherUnreachAttr...)
				body := makeUpdateBody(nil, rawAttrs, nil)

				// Verify applyRFC7606 output directly.
				s := rfc8669EBGPSession(true)
				wu, action, validation := applyRFC7606Body(t, s, body)
				assert.Equal(t, message.RFC7606ActionNone, action)
				kept, found := mpReachNLRIOf(t, wu.Payload())
				require.True(t, found)
				assert.Equal(t, t2stBytes, kept)
				unreach, found := mpUnreachNLRIOf(t, wu.Payload())
				require.True(t, found)
				assert.Equal(t, otherUnreach, unreach, "existing MP_UNREACH_NLRI must remain byte-identical")
				assert.Equal(t, 1, mupCountMPUnreachAttrs(t, wu.Payload()))
				require.Len(t, validation.typedWithdrawBodies, 1)
				wantWithdraw := message.BuildMPUnreachBody(uint16(mupFam.AFI), uint8(mupFam.SAFI), tc.rejectedBytes)
				assert.Equal(t, wantWithdraw, validation.typedWithdrawBodies[0])
				assert.Equal(t, 1, mupCountMPUnreachAttrs(t, validation.typedWithdrawBodies[0]))

				// Verify full processValidatedMessage dispatch delivers both the
				// MUP withdrawal and the primary UPDATE with untouched IPv6 unreach.
				ps := prefixReceiveSession(t, mupFam, PrefixCountInstalled, 100, false, family.IPv6Unicast)
				var delivered [][]byte
				ps.onMessageReceived = func(_ *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte,
					dwu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
					_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
					delivered = append(delivered, bytes.Clone(dwu.Payload()))
					return false
				}
				recvProcess(t, ps, body)
				require.Len(t, delivered, 2)
				assert.Equal(t, wantWithdraw, delivered[0])
				assert.Equal(t, 1, mupCountMPUnreachAttrs(t, delivered[0]))
				primaryUnreach, found := mpUnreachNLRIOf(t, delivered[1])
				require.True(t, found)
				assert.Equal(t, otherUnreach, primaryUnreach)
				assert.Equal(t, 1, mupCountMPUnreachAttrs(t, delivered[1]))
			})

			t.Run("all routes rejected without existing MP_UNREACH_NLRI", func(t *testing.T) {
				body := dsdUpdateBody(t, dsdUpdateAttrsHex(t, tc.rejectedHex, tc.tail))
				s := rfc8669EBGPSession(true)
				wu, action, validation := applyRFC7606Body(t, s, body)
				assert.Equal(t, message.RFC7606ActionNone, action)
				assert.Nil(t, validation.typedWithdrawBodies)
				_, foundReach := mpReachNLRIOf(t, wu.Payload())
				assert.False(t, foundReach)
				withdrawn, foundUnreach := mpUnreachNLRIOf(t, wu.Payload())
				require.True(t, foundUnreach)
				assert.Equal(t, tc.rejectedBytes, withdrawn)
				assert.False(t, updateCarriesNoRoutes(wu.Payload()), "never an End-of-RIB")
				assert.Equal(t, 1, mupCountMPUnreachAttrs(t, wu.Payload()))

				ps := prefixReceiveSession(t, mupFam, PrefixCountInstalled, 100, false)
				var delivered [][]byte
				ps.onMessageReceived = func(_ *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte,
					dwu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
					_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
					delivered = append(delivered, bytes.Clone(dwu.Payload()))
					return false
				}
				recvProcess(t, ps, body)
				require.Len(t, delivered, 1)
				dispatchedUnreach, found := mpUnreachNLRIOf(t, delivered[0])
				require.True(t, found)
				assert.Equal(t, tc.rejectedBytes, dispatchedUnreach)
			})

			t.Run("all MUP routes rejected beside existing MP_UNREACH_NLRI", func(t *testing.T) {
				attrsHex := dsdUpdateAttrsHex(t, tc.rejectedHex, tc.tail)
				rawAttrs, decodeErr := hex.DecodeString(attrsHex)
				require.NoError(t, decodeErr)
				rawAttrs = append(rawAttrs, otherUnreachAttr...)
				body := makeUpdateBody(nil, rawAttrs, nil)

				ps := prefixReceiveSession(t, mupFam, PrefixCountInstalled, 100, false, family.IPv6Unicast)
				var delivered [][]byte
				ps.onMessageReceived = func(_ *Session, _ netip.Addr, _ msgtype.MessageType, _ []byte,
					dwu *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection,
					_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
					delivered = append(delivered, bytes.Clone(dwu.Payload()))
					return false
				}
				recvProcess(t, ps, body)
				require.Len(t, delivered, 2)
				wantWithdraw := message.BuildMPUnreachBody(uint16(mupFam.AFI), uint8(mupFam.SAFI), tc.rejectedBytes)
				assert.Equal(t, wantWithdraw, delivered[0])
				_, reachFound := mpReachNLRIOf(t, delivered[1])
				assert.False(t, reachFound)
				primaryUnreach, found := mpUnreachNLRIOf(t, delivered[1])
				require.True(t, found)
				assert.Equal(t, otherUnreach, primaryUnreach)
			})
		})
	}
}

// TestMUPISDWithoutPrefixSIDOnReceive drives whole UPDATE bodies through applyRFC7606
// to prove draft-ietf-bess-mup-safi Section 3.3.3 (row 14) on the receive path: an ISD route
// arriving without a Prefix-SID attribute (or with Prefix-SID discarded by RFC 8669 Section 4
// from an EBGP peer) is stripped from MP_REACH_NLRI and withdrawn in MP_UNREACH_NLRI while its
// Type 2 ST sibling stays kept and the session is not reset, whereas an ISD route carrying a
// surviving Prefix-SID is kept byte-for-byte unchanged.
//
// VALIDATES: full applyRFC7606 treat-as-withdraw synthesis and EBGP Prefix-SID discard
// wiring for ISD routes under draft-ietf-bess-mup-safi Section 3.3.3.
// PREVENTS: missing-Prefix-SID ISD routes leaking past receive validation or resetting sessions.
func TestMUPISDWithoutPrefixSIDOnReceive(t *testing.T) {
	t.Cleanup(locatororigin.ResetForTest)
	locatororigin.ResetForTest()

	isd, err := hex.DecodeString(dsdISDHex)
	require.NoError(t, err)
	t2st, err := hex.DecodeString(dsdT2STHex)
	require.NoError(t, err)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 positive -- an ISD route in an MP_REACH_NLRI with no Prefix-SID attribute (or with Prefix-SID discarded from an EBGP peer) leaves MP_REACH_NLRI and is withdrawn in MP_UNREACH_NLRI, its Type 2 ST sibling is kept, and the session is not reset
	t.Run("ISD without surviving Prefix-SID is treated as withdrawn", func(t *testing.T) {
		cases := []struct {
			name       string
			s          *Session
			tail       string
			wantAction message.RFC7606Action
		}{
			{"no Prefix-SID attribute", rfc8669EBGPSession(true), "", message.RFC7606ActionNone},
			{"Prefix-SID discarded from an EBGP peer", rfc8669EBGPSession(false), dsdPrefixSIDHex, message.RFC7606ActionAttributeDiscard},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				body := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdISDHex+dsdT2STHex, tc.tail))
				wu, action, validation := applyRFC7606Body(t, tc.s, body)
				assert.Equal(t, tc.wantAction, action, "treat-as-withdraw and attribute-discard never reset the session")
				assert.Nil(t, validation.typedWithdrawBodies)
				kept, foundReach := mpReachNLRIOf(t, wu.Payload())
				require.True(t, foundReach)
				assert.Equal(t, t2st, kept, "sibling T2ST route must survive unchanged")
				withdrawn, foundUnreach := mpUnreachNLRIOf(t, wu.Payload())
				require.True(t, foundUnreach)
				assert.Equal(t, isd, withdrawn, "ISD route must be synthesized into MP_UNREACH_NLRI")
			})
		}
	})

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3 negative -- an ISD route in an MP_REACH_NLRI whose UPDATE carries a surviving Prefix-SID attribute is kept unchanged, and a Type 2 ST route without a Prefix-SID is kept
	t.Run("ISD with surviving Prefix-SID and T2ST without Prefix-SID are kept", func(t *testing.T) {
		bodyWithSID := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdISDHex+dsdT2STHex, dsdPrefixSIDHex))
		wu, action, validation := applyRFC7606Body(t, rfc8669EBGPSession(true), bodyWithSID)
		assert.Equal(t, message.RFC7606ActionNone, action)
		assert.Nil(t, validation.typedWithdrawBodies)
		assert.Equal(t, bodyWithSID, wu.Payload(), "conforming ISD + T2ST UPDATE must stay byte-identical")
		_, foundUnreach := mpUnreachNLRIOf(t, wu.Payload())
		assert.False(t, foundUnreach)

		bodyT2STOnly := dsdUpdateBody(t, dsdUpdateAttrsHex(t, dsdT2STHex, ""))
		wuT2ST, actionT2ST, valT2ST := applyRFC7606Body(t, rfc8669EBGPSession(true), bodyT2STOnly)
		assert.Equal(t, message.RFC7606ActionNone, actionT2ST)
		assert.Nil(t, valT2ST.typedWithdrawBodies)
		assert.Equal(t, bodyT2STOnly, wuT2ST.Payload())
		_, foundUnreach = mpUnreachNLRIOf(t, wuT2ST.Payload())
		assert.False(t, foundUnreach)
	})
}
