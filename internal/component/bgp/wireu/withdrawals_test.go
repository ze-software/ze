package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// woPayload builds an UPDATE body from its four components. Each is raw wire:
// withdrawn is IPv4 prefixes, attrs is a complete attribute section, nlri is IPv4
// prefixes.
func woPayload(withdrawn, attrs, nlri []byte) []byte {
	body := binary.BigEndian.AppendUint16(nil, uint16(len(withdrawn)))
	body = append(body, withdrawn...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(attrs)))
	body = append(body, attrs...)
	return append(body, nlri...)
}

// woAnnounceAttrs is ORIGIN + AS_PATH + NEXT_HOP + COMMUNITIES(NO_EXPORT): the
// attribute section of a route the RFC 1997 gate refuses to an external peer.
func woAnnounceAttrs() []byte {
	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN igp
	attrs = append(attrs, 0x40, 0x02, 0x04, 0x02, 0x01)
	attrs = binary.BigEndian.AppendUint16(attrs, 65001) // AS_PATH [65001]
	attrs = append(attrs,
		0x40, 0x03, 0x04, 1, 1, 1, 1, // NEXT_HOP 1.1.1.1
		0xC0, 0x08, 0x04) // COMMUNITIES, one value
	return binary.BigEndian.AppendUint32(attrs, uint32(attribute.CommunityNoExport))
}

// woMPUnreach is an MP_UNREACH_NLRI (type 15) for one IPv6 prefix, 2001:db8::/32.
func woMPUnreach() []byte {
	return []byte{
		0x90, 0x0F, 0x00, 0x08, // optional, extended length, type 15, len 8
		0x00, 0x02, 0x01, // AFI 2, SAFI 1
		0x20, 0x20, 0x01, 0x0D, 0xB8, // 2001:db8::/32
	}
}

// VALIDATES: a mixed UPDATE yields a part carrying the withdrawals and nothing else.
// PREVENTS: a destination that an egress prohibition refuses losing that UPDATE's
// withdrawals. The peer would keep a prefix ze can no longer take back until the session
// resets, which is worse than the leak the prohibition prevents.
func TestWithdrawalsOnlyKeepsTheWithdrawnRoutes(t *testing.T) {
	t.Parallel()
	withdrawn := []byte{24, 198, 51, 100} // 198.51.100.0/24
	nlri := []byte{24, 10, 0, 0}          // 10.0.0.0/24
	src := NewWireUpdate(woPayload(withdrawn, woAnnounceAttrs(), nlri), 7)

	part := WithdrawalsOnly(src)
	require.NotNil(t, part)

	got, err := part.Withdrawn()
	require.NoError(t, err)
	assert.Equal(t, withdrawn, got, "the withdrawn routes must survive whole")

	announced, err := part.NLRI()
	require.NoError(t, err)
	assert.Empty(t, announced, "the announcement must not survive")

	attrs, err := part.Attrs()
	require.NoError(t, err)
	assert.Nil(t, attrs, "a withdrawal needs no path attribute, and the split messages of the same "+
		"UPDATE carry none either (split.go, buildCombinedUpdates)")

	w, ok := ScanWellKnown(part.Payload())
	require.True(t, ok)
	assert.Equal(t, WellKnown(0), w, "the part carries no community for a later gate to read")
	assert.Equal(t, bgpCtxID(src), bgpCtxID(part), "the source encoding context travels with the part")
}

// VALIDATES: MP_UNREACH_NLRI is withdrawal information too.
// PREVENTS: an IPv6 (or any non-IPv4-unicast) withdrawal being dropped because it rides in
// an attribute rather than in the Withdrawn Routes field (RFC 4760 Section 3).
func TestWithdrawalsOnlyKeepsMPUnreach(t *testing.T) {
	t.Parallel()
	attrs := append(woAnnounceAttrs(), woMPUnreach()...)
	src := NewWireUpdate(woPayload(nil, attrs, []byte{24, 10, 0, 0}), 7)

	part := WithdrawalsOnly(src)
	require.NotNil(t, part)

	unreach, err := part.MPUnreach()
	require.NoError(t, err)
	require.NotNil(t, unreach)
	assert.Equal(t, woMPUnreach()[4:], []byte(unreach), "the MP_UNREACH value must survive whole")

	announced, err := part.NLRI()
	require.NoError(t, err)
	assert.Empty(t, announced)
}

// VALIDATES: an UPDATE that announces only answers nil, and one that withdraws only is
// returned unchanged.
// PREVENTS: two costs. A nil answer is what tells the caller there is nothing to send, so
// an empty UPDATE is never put on the wire; and returning the input pointer keeps the
// zero-copy forward a pure withdrawal already had.
func TestWithdrawalsOnlyEdges(t *testing.T) {
	t.Parallel()
	announce := NewWireUpdate(woPayload(nil, woAnnounceAttrs(), []byte{24, 10, 0, 0}), 7)
	assert.Nil(t, WithdrawalsOnly(announce), "an announcement withdraws nothing")

	pure := NewWireUpdate(woPayload([]byte{24, 198, 51, 100}, nil, nil), 7)
	assert.Same(t, pure, WithdrawalsOnly(pure), "a pure withdrawal must keep its zero-copy identity")

	// RFC 4724 Section 2: an MP_UNREACH whose value is AFI/SAFI only is an End-of-RIB
	// marker, not a withdrawal. Emitting one would end a restarting peer's deferral early.
	eor := []byte{0x90, 0x0F, 0x00, 0x03, 0x00, 0x02, 0x01}
	marker := NewWireUpdate(woPayload(nil, append(woAnnounceAttrs(), eor...), []byte{24, 10, 0, 0}), 7)
	assert.Nil(t, WithdrawalsOnly(marker), "an End-of-RIB marker withdraws nothing")

	assert.Nil(t, WithdrawalsOnly(NewWireUpdate(nil, 7)))
	assert.Nil(t, WithdrawalsOnly(NewWireUpdate([]byte{0x00, 0x08, 0x0A}, 7)),
		"a payload whose withdrawn length overruns is not readable")
}

// bgpCtxID reads the source encoding context of a wire update, so the assertion above names
// what it compares rather than repeating the accessor twice.
func bgpCtxID(u *WireUpdate) uint32 { return uint32(u.SourceCtxID()) }
