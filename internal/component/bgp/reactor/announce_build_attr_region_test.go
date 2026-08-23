package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// regionCanary is the pattern an untouched scratch octet holds.
//
// The plan's count check compares the number WriteToWithContext RETURNS against
// the number the size query reserved, so it cannot see a write that returns the
// promised count and writes more. Only an octet outside the reserved region can
// report that, which is why every test below fills the region with a pattern
// first (ai/rules/interop-and-goal-validation.md, "Prove the test discriminates":
// a test asserting an absence needs something that would be present).
const regionCanary = 0xAA

// canaryScratch fills the plan's scratch region with regionCanary.
func canaryScratch(p *announceAttrs) {
	for i := range p.scratch {
		p.scratch[i] = regionCanary
	}
}

// assertCanaryAfter asserts every scratch octet from off onward is untouched.
func assertCanaryAfter(t *testing.T, p *announceAttrs, off int) {
	t.Helper()
	for i := off; i < len(p.scratch); i++ {
		require.EqualValues(t, regionCanary, p.scratch[i],
			"scratch octet %d was written past the reserved region", i)
	}
}

// TestAnnouncePlanAggregatorStaysInsideReservedRegion drives the AGGREGATOR write
// from the announce plan, which is the single point both announce rails reach the
// wire through.
//
// announceAttrs.add reserves attribute.ValueLenWithContext octets of the pooled
// scratch region and then calls WriteToWithContext at that offset. AGGREGATOR
// answers the size query with a constant 8 (RFC 6793 Section 4: four-octet AS plus
// a four-octet IP address), so a write that copies netip.Addr.AsSlice unbounded put
// twelve octets of an IPv6 Address into the octets the NEXT attribute reserves, and
// returned 8 regardless. The plan's own count check is structurally blind to it.
//
// VALIDATES: the AGGREGATOR value write touches exactly the eight octets the plan
// reserved, for an IPv4, an IPv4-in-IPv6, an IPv6 and the zero netip.Addr.
//
// PREVENTS: a wide Address corrupting the attribute planned after it in a shared
// scratch region, with no check reporting anything.
func TestAnnouncePlanAggregatorStaysInsideReservedRegion(t *testing.T) {
	tests := []struct {
		name    string
		address netip.Addr
		field   [4]byte
	}{
		{"IPv4", netip.MustParseAddr("192.0.2.1"), [4]byte{192, 0, 2, 1}},
		{"IPv4-in-IPv6", netip.MustParseAddr("::ffff:192.0.2.1"), [4]byte{192, 0, 2, 1}},
		{"IPv6", netip.MustParseAddr("2001:db8::1"), [4]byte{0, 0, 0, 0}},
		{"zero Addr", netip.Addr{}, [4]byte{0, 0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var plan announceAttrs
			plan.begin()
			defer plan.release()
			canaryScratch(&plan)

			plan.add(&attribute.Aggregator{ASN: 65001, Address: tt.address}, nil)

			n, ok := plan.emit(nil, make([]byte, message.MaxMsgLen))
			require.True(t, ok, "an AGGREGATOR must still be planned: %s", plan.reason)
			assert.Positive(t, n)

			require.Equal(t, 8, plan.used, "AGGREGATOR reserves eight octets")
			assert.Equal(t, []byte{0x00, 0x00, 0xFD, 0xE9}, plan.scratch[0:4],
				"the four-octet AS number (RFC 6793 Section 4)")
			assert.Equal(t, tt.field[:], plan.scratch[4:8],
				"the four-octet IP address field (RFC 4271 Section 5.1.7)")
			assertCanaryAfter(t, &plan, 8)
		})
	}
}

// TestAnnouncePlanRefusesUnencodableNextHopAttribute covers the refusal half of the
// NEXT_HOP length rule.
//
// (*NextHop).Len counts the octets WriteTo emits, so an Addr with no wire form
// counts zero and the two agree at zero. Agreement is not encodability: RFC 4271
// Section 5.1.3 gives NEXT_HOP no zero-length form, and an UPDATE missing the
// well-known attribute is a Missing Well-known Attribute at the receiver (RFC 7606
// Section 3(d)). The plan must therefore refuse the attribute, and it must refuse it
// by NAME, because a rail reports an unencodable next hop and an oversize announce
// with different words (ai/rules/cli.md).
//
// VALIDATES: a NEXT_HOP with the zero netip.Addr is refused with
// attribute.ErrUnencodableNextHop as the cause, and nothing is planned.
//
// PREVENTS: a zero-length NEXT_HOP value reaching the wire, and a next-hop refusal
// reaching the operator as "send fewer prefixes".
func TestAnnouncePlanRefusesUnencodableNextHopAttribute(t *testing.T) {
	var plan announceAttrs
	plan.begin()
	defer plan.release()

	plan.add(plan.nextHopFor(netip.Addr{}), nil)

	n, ok := plan.emit(nil, make([]byte, message.MaxMsgLen))
	require.False(t, ok, "a NEXT_HOP with no wire form must not be emitted")
	assert.Equal(t, 0, n, "a refused emit must not report a length")
	require.ErrorIs(t, plan.refusalCause(), attribute.ErrUnencodableNextHop,
		"the rail must be able to tell this refusal from an oversize one")
	assert.Empty(t, plan.plans, "a refused attribute must not be planned")
}

// TestAnnouncePlanKeepsValidNextHopAttribute is the discriminating half of the test
// above: the refusal must fire for the zero Addr alone.
//
// It is also what A-4 of the spec asks. Making (*NextHop) satisfy
// announceNextHopValidator puts a new check on the hottest announce path in the
// daemon, and a check that refused a valid address would drop every IPv4 announce.
//
// VALIDATES: an IPv4 NEXT_HOP is planned with a four-octet value that stays inside
// the reserved region, and the plan carries no refusal.
//
// PREVENTS: the zero-Addr refusal widening into a refusal of ordinary announces.
func TestAnnouncePlanKeepsValidNextHopAttribute(t *testing.T) {
	var plan announceAttrs
	plan.begin()
	defer plan.release()
	canaryScratch(&plan)

	plan.add(plan.nextHopFor(netip.MustParseAddr("192.0.2.1")), nil)

	n, ok := plan.emit(nil, make([]byte, message.MaxMsgLen))
	require.True(t, ok, "a valid IPv4 NEXT_HOP must be planned: %s", plan.reason)
	assert.Positive(t, n)
	assert.NoError(t, plan.refusalCause())

	require.Equal(t, 4, plan.used, "an IPv4 NEXT_HOP value is four octets (RFC 4271 Section 5.1.3)")
	assert.Equal(t, []byte{192, 0, 2, 1}, plan.scratch[0:4])
	assertCanaryAfter(t, &plan, 4)
}
