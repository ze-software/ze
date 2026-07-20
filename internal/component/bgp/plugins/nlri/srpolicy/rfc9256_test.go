// VALIDATES: the RFC 9256 SR Policy architecture obligations that ze's SR Policy
// originator carries: the <Color, Endpoint> identification tuple in the NLRI key,
// symbolic policy / candidate-path names never acting as identifiers, and the
// Type A / Type B restriction on the segments ze emits.
// PREVENTS: a symbolic name leaking into the NLRI key (which would make two names for
// one policy look like two policies); an SR Policy accepted without a complete
// identification tuple; a segment type ze can neither encode nor resolve being emitted.

package srpolicy

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// key returns the wire NLRI bytes an SR Policy is identified by.
func key(t *testing.T, cfg string) []byte {
	t.Helper()
	pr, err := parseConfigRoute(cr(strings.Fields(cfg), "192.0.2.1", false))
	require.NoError(t, err)
	return pr.NLRI
}

// attr returns the Tunnel Encapsulation attribute value built for an SR Policy config.
func attr(t *testing.T, cfg string) []byte {
	t.Helper()
	pr, err := parseConfigRoute(cr(strings.Fields(cfg), "192.0.2.1", false))
	require.NoError(t, err)
	require.Len(t, pr.Attrs, 1)
	return pr.Attrs[0].Value
}

// TestRFC9256IdentificationTuple pins that the color and the endpoint are what
// distinguish one SR Policy from another on the wire. The headend is the node the
// NLRI is advertised to, so in ze's originator the observable identification is the
// <Color, Endpoint> pair carried in the NLRI key.
func TestRFC9256IdentificationTuple(t *testing.T) {
	t.Parallel()

	base := "distinguisher 7 color 100 endpoint 10.0.0.1"

	// RFC requirement: RFC9256-2.1-2 positive -- at one headend the color and the endpoint alone separate two SR Policies; nothing else in the config changes the key
	// The <Headend, Color, Endpoint> tuple of RFC9256-2.1-1 is not asserted here and cannot be:
	// SRPolicy carries no headend field, so that line is annotated {not-applicable} in rfc/short/rfc9256.md.
	assert.Equal(t, key(t, base), key(t, base), "same <color, endpoint> identifies the same policy")
	assert.NotEqual(t, key(t, base), key(t, "distinguisher 7 color 200 endpoint 10.0.0.1"),
		"a different color is a different SR Policy")
	assert.NotEqual(t, key(t, base), key(t, "distinguisher 7 color 100 endpoint 10.0.0.2"),
		"a different endpoint is a different SR Policy")

	// The preference sub-TLV rides in the attribute, so it must not perturb the key.
	assert.Equal(t, key(t, base), key(t, base+" preference 400"),
		"attribute content is not part of the identification tuple")

	sp := New(family.AFIIPv4, 7, 100, netip.MustParseAddr("10.0.0.1"))
	parsed, err := Parse(family.AFIIPv4, sp.Bytes()[1:])
	require.NoError(t, err)
	assert.Equal(t, uint32(100), parsed.Color())
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), parsed.Endpoint())
}

// TestRFC9256IdentificationTupleIncomplete pins that an SR Policy which cannot be
// identified is refused rather than given a partial identity.
func TestRFC9256IdentificationTupleIncomplete(t *testing.T) {
	t.Parallel()

	// A config that names no color or no endpoint cannot identify an SR Policy and is
	// rejected instead of encoding a partial tuple. This says nothing about the headend
	// half of RFC9256-2.1-1, which ze has no field for, so no tag for it here.
	_, err := parseConfigRoute(cr(strings.Fields("distinguisher 0 endpoint 10.0.0.1"), "192.0.2.1", false))
	require.ErrorIs(t, err, errSRPolicyMissingFields, "no color: the policy has no identity")

	_, err = parseConfigRoute(cr(strings.Fields("distinguisher 0 color 100"), "192.0.2.1", false))
	require.ErrorIs(t, err, errSRPolicyMissingFields, "no endpoint: the policy has no identity")

	// An NLRI whose body is too short to hold the <Color, Endpoint> pair is rejected.
	// That is a buffer-bounds check returning ErrSRPolicyTruncated, not a statement about
	// identification, so it is no counter-pole for RFC9256-2.1-2, which is annotated
	// {single-polarity: positive} in rfc/short/rfc9256.md.
	_, err = Parse(family.AFIIPv4, make([]byte, 11))
	require.ErrorIs(t, err, ErrSRPolicyTruncated)
	_, err = Parse(family.AFIIPv6, make([]byte, 23))
	require.ErrorIs(t, err, ErrSRPolicyTruncated)

	// An endpoint of the wrong address family cannot name a node in the policy's AFI.
	_, err = parseConfigRoute(cr(strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1"), "2001:db8::2", true))
	require.Error(t, err, "an IPv4 endpoint cannot identify an IPv6 SR Policy")
}

// TestRFC9256SymbolicNamesAreNotIdentifiers pins that the policy name and the
// candidate-path name are debugging aids carried in the Tunnel Encapsulation
// attribute, never part of what identifies the policy or the candidate path.
func TestRFC9256SymbolicNamesAreNotIdentifiers(t *testing.T) {
	t.Parallel()

	base := "distinguisher 7 color 100 endpoint 10.0.0.1 preference 100"
	named := base + " policy-name alpha candidate-path-name primary"
	renamed := base + " policy-name omega candidate-path-name backup"

	// RFC requirement: RFC9256-2.6-3 positive -- two candidate paths whose symbolic names differ produce byte-identical NLRI keys, so the name is not part of the identity
	assert.Equal(t, key(t, named), key(t, renamed),
		"renaming a policy or candidate path must not change what identifies it")
	assert.Equal(t, key(t, base), key(t, named),
		"adding names must not change what identifies the policy")

	// The names are still carried -- they are signaled, just not identifying.
	assert.Contains(t, string(attr(t, named)), "alpha", "the policy name is signaled")
	assert.Contains(t, string(attr(t, named)), "primary", "the candidate-path name is signaled")
	assert.NotEqual(t, attr(t, named), attr(t, renamed), "the names differ in the attribute")

	// RFC requirement: RFC9256-2.6-3 negative -- two candidate paths sharing one symbolic name are still distinct identities when their discriminators differ, so a shared name never merges them
	sameName := "distinguisher 8 color 100 endpoint 10.0.0.1 preference 100 policy-name alpha candidate-path-name primary"
	assert.NotEqual(t, key(t, named), key(t, sameName),
		"an identical name must not make two candidate paths one")
}

// TestRFC9256SegmentTypesAreAOrB pins that every SID ze puts in a segment list is a
// Type A MPLS label or a Type B SRv6 SID. ze verifies no SID's reachability, so
// restricting the encoder to the two types that carry the SID value outright is what
// keeps that honest.
func TestRFC9256SegmentTypesAreAOrB(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9256-5.1-4 positive -- the segment encoder emits only Type A (sub-TLV 1, MPLS label) and Type B (sub-TLV 13, SRv6 SID), the two types that carry the SID outright, and refuses any other segment type
	sl, consumed, err := parseSegmentList(strings.Fields("weight 1 segment type-a mpls 16001 segment type-b srv6 fc00::1"))
	require.NoError(t, err)
	require.Equal(t, 10, consumed)
	require.Len(t, sl.segments, 2)
	assert.True(t, sl.segments[0].typeA, "first segment is Type A")
	assert.True(t, sl.segments[1].typeB, "second segment is Type B")

	assert.Equal(t, segSubTLVTypeA, buildSegmentSubSubTLV(sl.segments[0])[0],
		"Type A encodes as segment sub-TLV 1")
	assert.Equal(t, segSubTLVTypeBSID, buildSegmentSubSubTLV(sl.segments[1])[0],
		"Type B encodes as segment sub-TLV 13")

	for _, unsupported := range []string{"type-c", "type-d", "type-i", "type-k"} {
		_, _, err := parseSegmentList(strings.Fields("weight 1 segment " + unsupported + " 10.0.0.1 0"))
		require.Error(t, err, "%s is not a type whose SID ze carries outright", unsupported)
	}
}
