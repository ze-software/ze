// Tests for the OPEN ze-peer builds (open.go).
//
// The goal each test shares: every fact ze-peer's OPEN asserts about ze-peer is
// resolved once, and no octet of that fact is inherited from ze's OPEN. The
// method is to build ze's OPEN by hand, hand it to buildOpen, and read the
// answer back through the same parser ze uses.

package peer

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// zeOpenBody builds an OPEN body carrying one Capabilities Optional Parameter
// with the given capability TLVs bundled inside it, which is the RFC 5492
// Section 4 shape ze itself emits.
func zeOpenBody(as uint16, routerID uint32, caps ...[]byte) []byte {
	var bundle []byte
	for _, c := range caps {
		bundle = append(bundle, c...)
	}
	body := make([]byte, openBodyFixedLen+1)
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:], as)
	binary.BigEndian.PutUint16(body[3:], 180)
	binary.BigEndian.PutUint32(body[5:], routerID)
	if len(bundle) == 0 {
		return body
	}
	body[openBodyFixedLen] = byte(2 + len(bundle))
	body = append(body, paramTypeCapability, byte(len(bundle)))
	return append(body, bundle...)
}

// capTLV wraps a capability value in its RFC 5492 Section 4 code and length.
func capTLV(code byte, value ...byte) []byte {
	return append([]byte{code, byte(len(value))}, value...)
}

// asn4TLV is the four-octet AS capability (RFC 6793 Section 3) carrying as.
func asn4TLV(as uint32) []byte {
	value := make([]byte, 4)
	binary.BigEndian.PutUint32(value, as)
	return capTLV(byte(capability.CodeASN4), value...)
}

// peerCaps reads back the capabilities of an OPEN ze-peer built, through the
// same parser ze's own receive path uses.
func peerCaps(t *testing.T, open []byte) []capability.Capability {
	t.Helper()
	body := open[HeaderLen:]
	require.GreaterOrEqual(t, len(body), openBodyFixedLen+1)

	params := body[openBodyFixedLen+1:]
	extended := false
	if body[openBodyFixedLen] == message.ExtendedParamMarker && body[openBodyFixedLen+1] == message.ExtendedParamMarker {
		extended = true
		params = body[openBodyFixedLen+4:]
	}
	caps, err := capability.ParseFromOptionalParams(params, extended)
	require.NoError(t, err)
	return caps
}

// peerASN4 is the AS ze-peer's OPEN carries in the four-octet AS capability, and
// whether it carries one at all.
func peerASN4(t *testing.T, open []byte) (uint32, bool) {
	t.Helper()
	for _, entry := range peerCaps(t, open) {
		if asn4, ok := entry.(*capability.ASN4); ok {
			return asn4.ASN, true
		}
	}
	return 0, false
}

// peerMyAS is the two-octet My Autonomous System field of ze-peer's OPEN.
func peerMyAS(open []byte) uint16 {
	return binary.BigEndian.Uint16(open[HeaderLen+1:])
}

// TestPeerOpenASReachesBothCarriers is AC-1.
//
// VALIDATES: option=asn:value=N puts N in the My Autonomous System field AND in
// the four-octet AS capability, and no other AS appears anywhere.
// PREVENTS: the two carriers disagreeing, which ze answers with NOTIFICATION 2/2
// Bad Peer AS (validateOpenPeerAS, internal/component/bgp/reactor/session_open_as.go).
func TestPeerOpenASReachesBothCarriers(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000))
	cfg := &Config{OpenAS: []OpenASBinding{{AS: 65010}}}

	open, err := buildOpen(ze, openIdentity{as: 65010, routerID: 0x01020305}, cfg)
	require.NoError(t, err)

	assert.Equal(t, uint16(65010), peerMyAS(open), "the My AS field carries the declared AS")
	as, present := peerASN4(t, open)
	require.True(t, present, "the mirrored capability 65 is still offered")
	assert.Equal(t, uint32(65010), as, "capability 65 carries the same declared AS")
}

// TestPeerOpenFourOctetASUsesASTrans is AC-3.
//
// VALIDATES: RFC 6793 Section 3, AS_TRANS in the My AS field and the real AS in
// the capability, for an AS that does not fit in two octets.
// PREVENTS: the old range guard, which skipped BOTH writes above 65535 and let
// the peer open with ze's own AS after a .ci asked for a four-octet one.
func TestPeerOpenFourOctetASUsesASTrans(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000))

	open, err := buildOpen(ze, openIdentity{as: 4200000000, routerID: 0x01020305}, &Config{})
	require.NoError(t, err)

	assert.Equal(t, uint16(asTrans), peerMyAS(open), "RFC 6793 Section 3 puts AS_TRANS in the My AS field")
	as, present := peerASN4(t, open)
	require.True(t, present)
	assert.Equal(t, uint32(4200000000), as, "the capability carries the real AS")
}

// TestPeerOpenFourOctetASAddsTheCapability proves the four-octet AS is carried
// even when ze offered no capability 65 to mirror.
//
// VALIDATES: the capability is added, because above 65535 it is the only carrier
// the AS has.
// PREVENTS: a four-octet request reaching the wire as AS_TRANS alone, which
// claims an AS the .ci never asked for.
func TestPeerOpenFourOctetASAddsTheCapability(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, capTLV(byte(capability.CodeRouteRefresh)))

	open, err := buildOpen(ze, openIdentity{as: 4200000000, routerID: 1}, &Config{})
	require.NoError(t, err)

	as, present := peerASN4(t, open)
	require.True(t, present, "a four-octet AS has no other carrier")
	assert.Equal(t, uint32(4200000000), as)
}

// TestPeerOpenDefaultASMatchesZesExpectation is AC-2.
//
// VALIDATES: with no option=asn the peer opens with ze's own AS, read through
// the RFC 6793 Section 4.1 precedence rule, and with the declared AS once the
// .ci states one.
// PREVENTS: a default that reads only the two-octet field, which is the field a
// conforming receiver never reads.
func TestPeerOpenDefaultASMatchesZesExpectation(t *testing.T) {
	ze := zeOpenBody(asTrans, 0x01020304, asn4TLV(4200000000))

	open, err := buildOpen(ze, openIdentity{as: zeAdvertisedAS(ze), routerID: 1}, &Config{})
	require.NoError(t, err)

	as, present := peerASN4(t, open)
	require.True(t, present)
	assert.Equal(t, uint32(4200000000), as, "the mirror reads the capability, not the AS_TRANS placeholder")
	assert.Equal(t, uint16(asTrans), peerMyAS(open))
}

// TestPeerOpenASBindingPicksTheConnectionsPeer is AC-2 for the tests where one
// ze-peer process serves several of ze's peers at once.
//
// VALIDATES: a keyed option=asn wins over an unkeyed one, on either endpoint.
// PREVENTS: one AS answering every connection of a conn_map batch, which is
// wrong for every peer but the first.
func TestPeerOpenASBindingPicksTheConnectionsPeer(t *testing.T) {
	cfg := &Config{OpenAS: []OpenASBinding{
		{AS: 65009},
		{Addr: netip.MustParseAddr("127.0.0.1"), AS: 65001},
		{Addr: netip.MustParseAddr("127.0.0.2"), AS: 65002},
	}}

	as, declared := cfg.resolveOpenAS(netip.MustParseAddr("127.0.0.2"), netip.Addr{})
	assert.True(t, declared)
	assert.Equal(t, uint32(65002), as, "the local endpoint carries the key when ze dialed us")

	as, declared = cfg.resolveOpenAS(netip.Addr{}, netip.MustParseAddr("127.0.0.1"))
	assert.True(t, declared)
	assert.Equal(t, uint32(65001), as, "the remote endpoint carries the key when we dialed ze")

	as, declared = cfg.resolveOpenAS(netip.MustParseAddr("10.0.0.1"), netip.Addr{})
	assert.True(t, declared)
	assert.Equal(t, uint32(65009), as, "an address no key names falls back to the unkeyed declaration")

	_, declared = (&Config{}).resolveOpenAS(netip.Addr{}, netip.Addr{})
	assert.False(t, declared, "no declaration is reported as none, never as AS 0")
}

// TestPeerOpenRoleIsComplementary is AC-5.
//
// VALIDATES: RFC 9234 Section 4.2 Table 2, the role that corresponds to ze's.
// PREVENTS: the mirrored role, which corresponds for Peer alone and makes ze
// answer NOTIFICATION 2/11 for the other four values.
func TestPeerOpenRoleIsComplementary(t *testing.T) {
	// RFC 9234 Section 4: Provider=0, RS=1, RS-Client=2, Customer=3, Peer=4.
	pairs := map[byte]byte{0: 3, 3: 0, 1: 2, 2: 1, 4: 4}
	for zeRole, want := range pairs {
		ze := zeOpenBody(65000, 0x01020304, capTLV(byte(capability.CodeRole), zeRole))

		open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, &Config{})
		require.NoError(t, err)

		role, present := peerRoleValue(t, open)
		require.True(t, present, "ze offered a Role capability, so the mirror keeps one")
		assert.Equal(t, want, role, "ze role %d must be answered with %d", zeRole, want)
	}
}

// peerRoleValue reads the one-octet Role capability value out of an OPEN.
func peerRoleValue(t *testing.T, open []byte) (byte, bool) {
	t.Helper()
	for _, entry := range peerCaps(t, open) {
		unknown, ok := entry.(*capability.Unknown)
		if !ok || unknown.Code() != capability.CodeRole || len(unknown.Data) != 1 {
			continue
		}
		return unknown.Data[0], true
	}
	return 0, false
}

// TestPeerOpenExplicitRoleWins is AC-6.
//
// VALIDATES: an add-capability naming a code the builder resolves replaces the
// builder's own, and the harness adds no second capability of that code.
// PREVENTS: the four Role .ci files, which drop code 9 and add it back, ending
// up with two Role capabilities -- which RFC 9234 Section 4.2 makes a Role
// Mismatch when their values differ.
func TestPeerOpenExplicitRoleWins(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, capTLV(byte(capability.CodeRole), 0))
	cfg := &Config{CapabilityOverrides: []CapabilityOverride{
		{Code: byte(capability.CodeRole), Add: false},
		{Code: byte(capability.CodeRole), Value: []byte{4}, Add: true},
	}}

	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, cfg)
	require.NoError(t, err)

	roles := 0
	for _, entry := range peerCaps(t, open) {
		if entry.Code() == capability.CodeRole {
			roles++
		}
	}
	assert.Equal(t, 1, roles, "the .ci stated the role, so exactly one is sent")
	role, present := peerRoleValue(t, open)
	require.True(t, present)
	assert.Equal(t, byte(4), role, "the role sent is the one the .ci stated")

	// An add with no drop beside it is the shape test/encode/cap-refuse-asn4.ci
	// uses for capability 65: the .ci states the AS capability itself, so the
	// builder must not emit a second one carrying its own resolution.
	stated := &Config{CapabilityOverrides: []CapabilityOverride{
		{Code: byte(capability.CodeASN4), Value: []byte{0, 0, 0xFD, 0xE9}, Add: true},
	}}
	peer := &Peer{config: stated}
	mirrored := zeOpenBody(65000, 0x01020304, asn4TLV(65000))
	open, err = buildOpen(mirrored, peer.openIdentity(mirrored, nil), stated)
	require.NoError(t, err)

	asn4s := 0
	for _, entry := range peerCaps(t, open) {
		if entry.Code() == capability.CodeASN4 {
			asn4s++
		}
	}
	assert.Equal(t, 1, asn4s, "the .ci stated capability 65, so the builder adds no second one")
	as, present := peerASN4(t, open)
	require.True(t, present)
	assert.Equal(t, uint32(65001), as, "the AS sent is the one the .ci stated")
	// The stated capability is the AS declaration, so the header field follows it.
	// Left to the resolved AS, the two carriers disagree and ze answers
	// NOTIFICATION 2/2 Bad Peer AS on the carrier RFC 6793 Section 4.1 makes it
	// read first.
	assert.Equal(t, uint16(65001), peerMyAS(open), "the My AS field follows the stated capability")
}

// TestPeerOpenStatedASWinsOverEveryOtherDeclaration pins the AS precedence.
//
// VALIDATES: an add-capability for code 65 outranks option=asn, and both carriers
// follow it; a stated capability 65 of any other length is sent as written and
// leaves the header field at the resolved AS.
// PREVENTS: one carrier taking the stated value while the other takes the
// resolved one, which is the disagreement the whole builder exists to remove.
func TestPeerOpenStatedASWinsOverEveryOtherDeclaration(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000))

	cfg := &Config{
		OpenAS: []OpenASBinding{{AS: 65010}},
		CapabilityOverrides: []CapabilityOverride{
			{Code: byte(capability.CodeASN4), Value: []byte{0xFA, 0x56, 0xEA, 0x00}, Add: true},
		},
	}
	peer := &Peer{config: cfg}
	open, err := buildOpen(ze, peer.openIdentity(ze, nil), cfg)
	require.NoError(t, err)

	as, present := peerASN4(t, open)
	require.True(t, present)
	assert.Equal(t, uint32(0xFA56EA00), as, "the stated capability carries its own octets")
	assert.Equal(t, uint16(asTrans), peerMyAS(open),
		"the header field narrows the STATED AS, not the one option=asn named")

	// A stated capability 65 that is not four octets is a malformed capability the
	// test is driving on purpose. It goes on the wire as written, and the header
	// field keeps the AS the rest of the configuration resolved.
	malformed := &Config{
		OpenAS: []OpenASBinding{{AS: 65010}},
		CapabilityOverrides: []CapabilityOverride{
			{Code: byte(capability.CodeASN4), Value: []byte{0, 1}, Add: true},
		},
	}
	peer = &Peer{config: malformed}
	open, err = buildOpen(ze, peer.openIdentity(ze, nil), malformed)
	require.NoError(t, err)
	assert.Equal(t, uint16(65010), peerMyAS(open),
		"a malformed stated capability declares no AS, so the header field keeps the resolved one")
}

// TestPeerOpenAddPathDirectionsInverted is AC-7.
//
// VALIDATES: RFC 7911 Section 4, Send becomes Receive and Receive becomes Send,
// so Negotiate intersects to a non-empty mode.
// PREVENTS: the mirrored direction, which intersects to AddPathNone and
// negotiates ADD-PATH off with no error anywhere.
func TestPeerOpenAddPathDirectionsInverted(t *testing.T) {
	value := []byte{
		0, 1, 1, byte(capability.AddPathSend),
		0, 2, 1, byte(capability.AddPathReceive),
		0, 1, 2, byte(capability.AddPathBoth),
	}
	ze := zeOpenBody(65000, 0x01020304, capTLV(byte(capability.CodeAddPath), value...))

	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, &Config{})
	require.NoError(t, err)

	var addPath *capability.AddPath
	for _, entry := range peerCaps(t, open) {
		if found, ok := entry.(*capability.AddPath); ok {
			addPath = found
		}
	}
	require.NotNil(t, addPath)
	require.Len(t, addPath.Families, 3)
	assert.Equal(t, capability.AddPathReceive, addPath.Families[0].Mode, "Send is answered with Receive")
	assert.Equal(t, capability.AddPathSend, addPath.Families[1].Mode, "Receive is answered with Send")
	assert.Equal(t, capability.AddPathBoth, addPath.Families[2].Mode, "Both is its own complement")
}

// TestPeerOpenFQDNIsTheHarnessName is AC-8.
//
// VALIDATES: the FQDN capability carries ze-peer's own name.
// PREVENTS: the mirror, which sends ze's hostname back as a claim about the test
// peer.
func TestPeerOpenFQDNIsTheHarnessName(t *testing.T) {
	fqdn := &capability.FQDN{Hostname: "ze-router", DomainName: "example.net"}
	tlv := make([]byte, fqdn.Len())
	fqdn.WriteTo(tlv, 0)
	ze := zeOpenBody(65000, 0x01020304, tlv)

	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, &Config{})
	require.NoError(t, err)

	var answered *capability.FQDN
	for _, entry := range peerCaps(t, open) {
		if found, ok := entry.(*capability.FQDN); ok {
			answered = found
		}
	}
	require.NotNil(t, answered)
	assert.Equal(t, peerHostname, answered.Hostname)
	assert.Empty(t, answered.DomainName, "the harness has no domain to claim")
}

// TestPeerOpenReadsExtendedParameterFraming is AC-9.
//
// VALIDATES: an RFC 9072 Section 2 framed OPEN is read whole, and drop-capability
// acts on it as it does on a one-octet one.
// PREVENTS: reading body octet 9 as the parameter length, which misframes such a
// message from its first parameter.
func TestPeerOpenReadsExtendedParameterFraming(t *testing.T) {
	bundle := append(asn4TLV(65000), capTLV(byte(capability.CodeRouteRefresh))...)
	body := make([]byte, openBodyFixedLen)
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:], 65000)
	binary.BigEndian.PutUint16(body[3:], 180)
	binary.BigEndian.PutUint32(body[5:], 0x01020304)
	params := append([]byte{paramTypeCapability, 0, byte(len(bundle))}, bundle...)
	body = append(body, message.ExtendedParamMarker, message.ExtendedParamMarker, 0, byte(len(params)))
	body = append(body, params...)

	cfg := &Config{CapabilityOverrides: []CapabilityOverride{{Code: byte(capability.CodeRouteRefresh)}}}
	open, err := buildOpen(body, openIdentity{as: 65001, routerID: 1}, cfg)
	require.NoError(t, err)

	caps := peerCaps(t, open)
	require.Len(t, caps, 1, "the dropped capability is gone and the other one survived")
	asn4, ok := caps[0].(*capability.ASN4)
	require.True(t, ok)
	assert.Equal(t, uint32(65001), asn4.ASN)
}

// TestPeerOpenEmitsExtendedFramingAbove255 is AC-10.
//
// VALIDATES: RFC 9072 Section 2 framing once the reconciled parameters pass 255
// octets, with no length field truncated.
// PREVENTS: the one-octet length wrapping, which sends a message whose first
// parameter no reader can frame.
func TestPeerOpenEmitsExtendedFramingAbove255(t *testing.T) {
	// ze's own parameters fit the one-octet length. The capability the .ci adds
	// is what carries the block past it, which is the case that used to truncate.
	bulk := capTLV(200, make([]byte, 240)...)
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000), bulk)
	cfg := &Config{CapabilityOverrides: []CapabilityOverride{
		{Code: 201, Value: make([]byte, 10), Add: true},
	}}

	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, cfg)
	require.NoError(t, err)

	body := open[HeaderLen:]
	require.Equal(t, byte(message.ExtendedParamMarker), body[openBodyFixedLen], "Non-Ext OP Len is the RFC 9072 marker")
	require.Equal(t, byte(message.ExtendedParamMarker), body[openBodyFixedLen+1], "Non-Ext OP Type is the RFC 9072 marker")
	assert.Equal(t, len(body)-openBodyFixedLen-4, int(binary.BigEndian.Uint16(body[openBodyFixedLen+2:])),
		"the two-octet length states the whole block")

	caps := peerCaps(t, open)
	assert.Len(t, caps, 3, "every capability survives the wider framing")
}

// TestPeerOpenPassThroughIsByteIdentical is R-3.
//
// VALIDATES: with nothing to reconcile and no override, ze-peer's parameters are
// ze's own octets, in the order they were read.
// PREVENTS: the re-encode reordering or rewriting a capability ze-peer has no
// opinion about, which would change what every .ci asserting ze's reply sees.
func TestPeerOpenPassThroughIsByteIdentical(t *testing.T) {
	caps := [][]byte{
		capTLV(byte(capability.CodeMultiprotocol), 0, 1, 0, 1),
		capTLV(byte(capability.CodeRouteRefresh)),
		capTLV(byte(capability.CodeExtendedMessage)),
		capTLV(200, 1, 2, 3),
	}
	ze := zeOpenBody(65000, 0x01020304, caps...)

	open, err := buildOpen(ze, openIdentity{as: 65000, routerID: 0x01020305}, &Config{})
	require.NoError(t, err)

	assert.Equal(t, ze[openBodyFixedLen:], open[HeaderLen+openBodyFixedLen:],
		"the whole Optional Parameters block comes back unchanged")
}

// TestPeerOpenCapabilityOverridesUnchanged is AC-11.
//
// VALIDATES: drop-capability and add-capability produce the capability set the
// mirroring harness produced, for the codes test/ uses today.
// PREVENTS: a .ci that tests require/refuse enforcement negotiating a different
// set after the builder replaced the mirror.
func TestPeerOpenCapabilityOverridesUnchanged(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304,
		capTLV(byte(capability.CodeMultiprotocol), 0, 1, 0, 1),
		asn4TLV(65000),
		capTLV(byte(capability.CodeRouteRefresh)),
	)
	cfg := &Config{CapabilityOverrides: []CapabilityOverride{
		{Code: byte(capability.CodeASN4)},
		{Code: byte(capability.CodeExtendedMessage), Add: true},
	}}

	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, cfg)
	require.NoError(t, err)

	var codes []capability.Code
	for _, entry := range peerCaps(t, open) {
		codes = append(codes, entry.Code())
	}
	assert.Equal(t, []capability.Code{
		capability.CodeMultiprotocol,
		capability.CodeRouteRefresh,
		capability.CodeExtendedMessage,
	}, codes, "the dropped code is gone, the added one is last, and the order is ze's")
}

// TestPeerOpenSendUnknownCapability pins the behavior a .ci reaches through
// option=open:value=send-unknown-capability.
//
// VALIDATES: capability 66 with the value "loremipsum" still reaches the wire.
// PREVENTS: the option becoming a no-op when the mirroring writer was deleted.
func TestPeerOpenSendUnknownCapability(t *testing.T) {
	ze := zeOpenBody(65000, 0x01020304, asn4TLV(65000))

	open, err := buildOpen(ze, openIdentity{as: 65000, routerID: 1}, &Config{SendUnknownCapability: true})
	require.NoError(t, err)

	var found *capability.Unknown
	for _, entry := range peerCaps(t, open) {
		if unknown, ok := entry.(*capability.Unknown); ok && unknown.Code() == unknownCapabilityCode {
			found = unknown
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, unknownCapabilityValue, string(found.Data))
}

// TestPeerOpenRouterIDDerivesFromZes pins the identifier default and its override.
//
// VALIDATES: the default is ze's identifier with the last octet incremented, and
// option=open:value=router-id replaces it outright.
// PREVENTS: an RFC 6286 test losing the identifier it asked for, and every .ci
// that asserts a specific identifier changing verdict.
func TestPeerOpenRouterIDDerivesFromZes(t *testing.T) {
	assert.Equal(t, uint32(0x0102030A), peerRouterID(zeOpenBody(65000, 0x01020309)))
	assert.Equal(t, uint32(0x01020300), peerRouterID(zeOpenBody(65000, 0x010203FF)),
		"the increment stays inside the last octet, so it wraps rather than carrying")
}

// zeExtendedOpenBody builds an OPEN body under the RFC 9072 Section 2 framing,
// whose Optional Parameters Length is two octets and whose parameters inside
// carry two-octet lengths.
func zeExtendedOpenBody(as uint16, routerID uint32, caps ...[]byte) []byte {
	var bundle []byte
	for _, c := range caps {
		bundle = append(bundle, c...)
	}
	body := make([]byte, openBodyFixedLen)
	body[0] = 4
	binary.BigEndian.PutUint16(body[1:], as)
	binary.BigEndian.PutUint16(body[3:], 180)
	binary.BigEndian.PutUint32(body[5:], routerID)

	params := []byte{paramTypeCapability, byte(len(bundle) >> 8), byte(len(bundle))}
	params = append(params, bundle...)
	body = append(body, message.ExtendedParamMarker, message.ExtendedParamMarker,
		byte(len(params)>>8), byte(len(params)))
	return append(body, params...)
}

// TestPeerOpenRefusesAMessageAboveTheRFC4271Ceiling is the message-length bound.
//
// VALIDATES: RFC 4271 Section 4.1, "The value of the Length field MUST always be
// at least 19 and no greater than 4096". RFC 8654 Section 6 raises that number
// to 65535 "for all messages except for OPEN and KEEPALIVE messages", so the
// bound holds whatever is negotiated.
// PREVENTS: the header Length field wrapping. Bounding the PARAMETERS at 65535
// rather than the message let 65504 to 65535 octets of parameters pass the check
// and write a Length of 31 for a 65567-octet frame, which no reader can cut.
func TestPeerOpenRefusesAMessageAboveTheRFC4271Ceiling(t *testing.T) {
	// Sixteen capabilities of 255 octets each is 4112 octets of TLVs, which is
	// past the ceiling once the header, the fixed fields and the envelope are
	// counted, and inside what the RFC 9072 framing can state.
	caps := make([][]byte, 0, 16)
	for range 16 {
		caps = append(caps, capTLV(200, make([]byte, 255)...))
	}
	ze := zeExtendedOpenBody(65000, 0x01020304, caps...)

	_, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, &Config{})
	require.Error(t, err, "an OPEN past the RFC 4271 ceiling must be refused, not truncated")
	assert.Contains(t, err.Error(), "4096", "the error names the ceiling")

	// One octet under it still builds, and its Length field states the truth.
	caps = caps[:15]
	ze = zeExtendedOpenBody(65000, 0x01020304, caps...)
	open, err := buildOpen(ze, openIdentity{as: 65001, routerID: 1}, &Config{})
	require.NoError(t, err)
	assert.Equal(t, len(open), int(binary.BigEndian.Uint16(open[16:])),
		"the header Length field states the whole message")
}
