// Design: docs/architecture/wire/attributes.md -- LARGE_COMMUNITY repeats on send
// Overview: session_egress_normalize.go -- compactLargeCommunities

package reactor

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
)

// largeCommunityValue is the 12-octet value 65000:n:n.
func largeCommunityValue(n int) []byte {
	v := make([]byte, largeCommunityOctets)
	binary.BigEndian.PutUint32(v[0:], 65000)
	binary.BigEndian.PutUint32(v[4:], uint32(n)) //nolint:gosec // test values are small.
	binary.BigEndian.PutUint32(v[8:], uint32(n)) //nolint:gosec // test values are small.
	return v
}

// largeCommunityValues is count distinct values 1..count; with repeat the last
// one is replaced by a copy of the first, so count values hold count-1 distinct.
func largeCommunityValues(count int, repeat bool) []byte {
	var out []byte
	for i := 1; i <= count; i++ {
		n := i
		if repeat && i == count {
			n = 1
		}
		out = append(out, largeCommunityValue(n)...)
	}
	return out
}

// largeCommunityAttr frames value as a LARGE_COMMUNITY attribute, in Extended
// Length form when it does not fit one octet.
func largeCommunityAttr(value []byte) []byte {
	if len(value) > 255 {
		return append([]byte{0xD0, 32, byte(len(value) >> 8), byte(len(value))}, value...)
	}
	return append([]byte{0xC0, 32, byte(len(value))}, value...)
}

// largeCommunityBody is ORIGIN, AS_PATH, NEXT_HOP, LARGE_COMMUNITY value, then
// MED 7 after it (so a shrink must move a later attribute), NLRI egressNLRI.
func largeCommunityBody(value []byte) []byte {
	attrs := []byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9,
		0x40, 3, 4, 192, 0, 2, 99,
	}
	attrs = append(attrs, largeCommunityAttr(value)...)
	attrs = append(attrs, 0x80, 4, 4, 0, 0, 0, 7)
	return egressBody(attrs, egressNLRI)
}

// sentLargeCommunities reads the LARGE_COMMUNITY value and MED off a sent body
// and checks the section lengths agree with the body.
func sentLargeCommunities(t *testing.T, out []byte) []byte {
	t.Helper()
	sections, err := wire.ParseUpdateSections(out)
	require.NoError(t, err)
	attrs := sections.Attrs(out)
	require.Len(t, out, 4+len(attrs)+len(egressNLRI), "total path attribute length disagrees with the body")
	require.Equal(t, egressNLRI, sections.NLRI(out))
	_, _, med, found := attribute.AttrFind(attrs, attribute.AttrMED)
	require.True(t, found, "MED lost")
	require.Equal(t, []byte{0, 0, 0, 7}, med)
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrLargeCommunity)
	require.True(t, found, "LARGE_COMMUNITY lost")
	return value
}

// TestRFC8092WriteRemovesRepeatedLargeCommunity hands writeUpdateBody bodies
// whose LARGE_COMMUNITY holds a repeat, as any producer could build them.
//
// VALIDATES: AC-8 repeat case: [X, X, Y] is sent as [X, Y]; 20 values holding a
// repeat (the stack filter path) and 22 in Extended Length form are sent with
// the repeat gone, first-occurrence order, every length rewritten; the compaction
// allocates nothing.
// PREVENTS: a policy add, an ingress tag or a plugin raw body sending a repeat.
// RFC requirement: RFC8092-3-1 positive -- a LARGE_COMMUNITY handed to the raw, parsed, filter-override and relay writers with a repeated value is sent with each value once, first occurrence kept, on both the pair-scan and the filter path.
func TestRFC8092WriteRemovesRepeatedLargeCommunity(t *testing.T) {
	x, y := largeCommunityValue(1), largeCommunityValue(2)
	cases := []struct {
		name     string
		in, want []byte
	}{
		{"x-x-y", bytes.Join([][]byte{x, x, y}, nil), bytes.Join([][]byte{x, y}, nil)},
		{"20-one-repeat", largeCommunityValues(20, true), largeCommunityValues(19, false)},
		{"22-extended-one-repeat", largeCommunityValues(22, true), largeCommunityValues(21, false)},
	}
	for _, tc := range cases {
		for _, rail := range egressRails {
			t.Run(tc.name+"/"+rail, func(t *testing.T) {
				require.Equal(t, tc.want, sentLargeCommunities(t, egressWrite(t, rail, largeCommunityBody(tc.in))))
			})
		}
		t.Run(tc.name+"/allocs", func(t *testing.T) {
			in := largeCommunityBody(tc.in)
			scratch := make([]byte, len(in))
			allocs := testing.AllocsPerRun(50, func() {
				copy(scratch, in)
				normalizeTransmittedUpdate(scratch)
			})
			require.Zero(t, allocs)
		})
	}
}

// TestRFC8092WriteKeepsDistinctLargeCommunities hands writeUpdateBody bodies
// whose LARGE_COMMUNITY holds no repeat.
//
// VALIDATES: AC-8 distinct case: 3, 16, 17 and 20 distinct values leave
// byte-for-byte as given; the walk allocates nothing.
// PREVENTS: the removal dropping or reordering a distinct value.
// RFC requirement: RFC8092-3-1 negative -- a LARGE_COMMUNITY whose values are all distinct is sent unchanged, byte-for-byte, on every writer and on both scan paths.
func TestRFC8092WriteKeepsDistinctLargeCommunities(t *testing.T) {
	for _, count := range []int{3, 16, 17, 20} {
		in := largeCommunityBody(largeCommunityValues(count, false))
		for _, rail := range egressRails {
			t.Run(rail, func(t *testing.T) {
				require.Equal(t, in, egressWrite(t, rail, in))
			})
		}
		scratch := make([]byte, len(in))
		allocs := testing.AllocsPerRun(50, func() {
			copy(scratch, in)
			normalizeTransmittedUpdate(scratch)
		})
		require.Zero(t, allocs)
	}
}

// TestRFC8092PolicyAddOfPresentValueSentOnce relays a route carrying [X, Z]
// whose export policy on the destination adds X, then Y, then Y: the policy
// handler appends every Add with no membership check, so the body it hands the
// writer holds [X, Z, X, Y, Y].
//
// VALIDATES: AC-2a: the destination receives [X, Z, Y], attribute length 36,
// total path attribute and message lengths consistent.
// PREVENTS: an export policy add putting a repeat on the wire.
// RFC requirement: RFC8092-3-1 positive -- an export policy that adds a value the route carries, and another value twice, results in a LARGE_COMMUNITY sent with each value once.
func TestRFC8092PolicyAddOfPresentValueSentOnce(t *testing.T) {
	x, y, z := largeCommunityValue(1), largeCommunityValue(2), largeCommunityValue(3)
	received := largeCommunityBody(bytes.Join([][]byte{x, z}, nil))
	policy := largeCommunityBody(bytes.Join([][]byte{x, z, x, y, y}, nil))
	peer, conn := newAnnouncePeer(t, "192.0.2.2")
	s := peer.session
	s.egressRouteFilter = func([]byte) (bool, []byte) { return false, policy }
	sections, err := wire.ParseUpdateSections(received)
	require.NoError(t, err)
	require.NoError(t, s.SendUpdate(&message.Update{PathAttributes: sections.Attrs(received), NLRI: sections.NLRI(received)}))
	packet := conn.written()
	require.Equal(t, len(packet), int(binary.BigEndian.Uint16(packet[16:18])))
	value := sentLargeCommunities(t, packet[message.HeaderLen:])
	require.Len(t, value, 36)
	require.Equal(t, bytes.Join([][]byte{x, z, y}, nil), value)
}
