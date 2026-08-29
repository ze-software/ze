// Related: session_prefix.go — forEachPrefixEntry, countPrefixEntries
//
// VALIDATES: the per-family prefix maximum counts the NLRIs a peer actually
// sent, for a typed family whose framing is not [prefix-length][address].
// PREVENTS: RFC 9552 Section 8.2.6's "An implementation MUST have the means to
// limit inbound updates" being satisfied on paper only. A limit compared
// against a number derived by reading an NLRI Type as a prefix length does not
// bind, and cannot be made to bind by configuring it lower.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
)

// lsSection builds a Link-State NLRI section holding n NLRIs, each framed as
// RFC 9552 Section 5.1 requires: [NLRI Type:2][Total NLRI Length:2][value].
func lsSection(n int) []byte {
	var out []byte
	for i := range n {
		out = append(out,
			0x00, 0x01, // NLRI Type 1, Node
			0x00, 0x09, // Total NLRI Length 9
			0x02,                                              // Protocol-ID
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, byte(i), // Identifier
		)
	}
	return out
}

// TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes is the requirement's whole
// point. Three Link-State NLRIs must count as three.
//
// RFC requirement: RFC9552-8.2.6-1 positive -- the inbound-update limit is
// compared against the number of Link-State NLRIs received, so a configured
// maximum can actually bind on this family (§8.2.6).
func TestBGPLSPrefixCountCountsNLRIsNotPrefixBytes(t *testing.T) {
	fk := familyKey(family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState})

	for _, want := range []int{0, 1, 3, 7} {
		assert.Equal(t, want, countPrefixEntries(fk, lsSection(want), false),
			"%d Link-State NLRIs must count as %d. The CIDR walk reads octet 0 as a "+
				"prefix length, which for a Link-State NLRI is the high byte of the "+
				"NLRI Type, so the number it produces is unrelated to what the peer "+
				"sent", want, want)
	}
}

// TestTheCIDRWalkWouldMiscountThisSection is the discrimination case, and it is
// what makes the test above mean something. It asserts the OLD reading is
// wrong, so a fix that merely renamed things could not pass both.
func TestTheCIDRWalkWouldMiscountThisSection(t *testing.T) {
	section := lsSection(3)

	// The old walk, reproduced: octet 0 is a prefix length in bits, and the entry
	// is one length octet plus ceil(bits/8) address octets.
	cidrCount := 0
	for offset := 0; offset < len(section); {
		offset += 1 + (int(section[offset])+7)/8
		if offset > len(section) {
			break
		}
		cidrCount++
	}

	require.NotEqual(t, 3, cidrCount,
		"if the CIDR walk happened to agree with the NLRI count for this section, "+
			"the fixture proves nothing and must be rebuilt around a section where "+
			"the two readings differ")
}

// TestUnicastCountingIsUnchanged keeps the common path. Unicast IS
// [prefix-length][address], so routing it through the family's splitter must
// give the same answer the CIDR walk always gave.
func TestUnicastCountingIsUnchanged(t *testing.T) {
	fk := familyKey(family.IPv4Unicast)

	for name, tc := range map[string]struct {
		data []byte
		want int
	}{
		"single /24":    {[]byte{24, 10, 0, 0}, 1},
		"two /24":       {[]byte{24, 10, 0, 0, 24, 10, 0, 1}, 2},
		"mixed lengths": {[]byte{8, 10, 16, 10, 0, 24, 10, 0, 0}, 3},
		"default route": {[]byte{0}, 1},
		"empty":         {nil, 0},
	} {
		assert.Equal(t, tc.want, countPrefixEntries(fk, tc.data, false), name)
	}
}

// TestBGPLSPrefixLimitActuallyFires is the other half, and the one that makes
// "the means to limit" mean something. A counter nobody enforces is not a means.
//
// RFC requirement: RFC9552-8.2.6-1 negative -- a Link-State section carrying
// more NLRIs than the configured bgp-ls maximum is refused, so the limit binds
// rather than merely being configurable (§8.2.6).
func TestBGPLSPrefixLimitActuallyFires(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{"bgp-ls/bgp-ls": 2}
	ps.PrefixWarning = map[string]uint32{"bgp-ls/bgp-ls": 1}
	s := NewSession(ps)

	// Three Link-State NLRIs against a maximum of two.
	body := makeUpdateBody(nil, mpReachAttrs(lsFam, lsSection(3)), nil)

	notif, _ := s.checkPrefixLimits(testWireUpdate(body))

	require.NotNil(t, notif,
		"three Link-State NLRIs against a maximum of two must be refused. Before the "+
			"count was framed per family this compared the maximum against a number "+
			"derived by reading an NLRI Type as a prefix length, so no configured "+
			"value could bind")
	assert.Equal(t, message.NotifyCease, notif.ErrorCode)
	assert.Equal(t, message.NotifyCeaseMaxPrefixes, notif.ErrorSubcode)
}

// TestBGPLSPrefixLimitLeavesAConformingSectionAlone keeps the guard honest: a
// section within the maximum must pass, or the test above would be satisfied by
// refusing everything.
func TestBGPLSPrefixLimitLeavesAConformingSectionAlone(t *testing.T) {
	ps := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0)
	ps.PrefixMaximum = map[string]uint32{"bgp-ls/bgp-ls": 5}
	ps.PrefixWarning = map[string]uint32{"bgp-ls/bgp-ls": 4}
	s := NewSession(ps)

	body := makeUpdateBody(nil, mpReachAttrs(lsFam, lsSection(3)), nil)

	notif, drop := s.checkPrefixLimits(testWireUpdate(body))

	assert.Nil(t, notif, "three NLRIs against a maximum of five must be accepted")
	assert.False(t, drop)
}
