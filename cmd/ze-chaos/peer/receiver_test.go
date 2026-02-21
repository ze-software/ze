package peer

import (
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseIPv4Prefix verifies wire-format IPv4 prefix parsing.
//
// VALIDATES: Prefix length and address bytes are correctly decoded.
// PREVENTS: Off-by-one in byte-length calculation or address masking.
func TestParseIPv4Prefix(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want netip.Prefix
		n    int
	}{
		{
			name: "slash_24",
			data: []byte{24, 10, 0, 1}, // 10.0.1.0/24
			want: netip.MustParsePrefix("10.0.1.0/24"),
			n:    4,
		},
		{
			name: "slash_16",
			data: []byte{16, 172, 16}, // 172.16.0.0/16
			want: netip.MustParsePrefix("172.16.0.0/16"),
			n:    3,
		},
		{
			name: "slash_8",
			data: []byte{8, 10}, // 10.0.0.0/8
			want: netip.MustParsePrefix("10.0.0.0/8"),
			n:    2,
		},
		{
			name: "slash_32",
			data: []byte{32, 192, 168, 1, 1}, // 192.168.1.1/32
			want: netip.MustParsePrefix("192.168.1.1/32"),
			n:    5,
		},
		{
			name: "slash_0",
			data: []byte{0}, // 0.0.0.0/0 (default route)
			want: netip.MustParsePrefix("0.0.0.0/0"),
			n:    1,
		},
		{
			name: "empty",
			data: []byte{},
			n:    0,
		},
		{
			name: "invalid_prefix_len_33",
			data: []byte{33, 10, 0, 0, 0},
			n:    0,
		},
		{
			name: "truncated",
			data: []byte{24, 10}, // Needs 3 bytes but only 1 available.
			n:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, n := parseIPv4Prefix(tt.data)
			assert.Equal(t, tt.n, n)
			if tt.n > 0 {
				assert.Equal(t, tt.want, prefix)
			}
		})
	}
}

// TestParseUpdatePrefixesAnnounce verifies parsing announced NLRI from an UPDATE body.
//
// VALIDATES: Announced prefixes after attributes are extracted correctly.
// PREVENTS: Wrong offset calculation skipping attributes section.
func TestParseUpdatePrefixesAnnounce(t *testing.T) {
	// Build a minimal UPDATE body (after header):
	// withdrawn-len(2) + attr-len(2) + attrs(N) + NLRI
	// No withdrawals, minimal attributes, one NLRI.
	body := []byte{
		0, 0, // withdrawn routes length = 0
		0, 7, // total path attribute length = 7
		// Minimal ORIGIN attribute (flags=0x40, code=1, len=1, value=0)
		0x40, 1, 1, 0,
		// Minimal empty AS_PATH (flags=0x40, code=2, len=0)
		0x40, 2, 0,
		// NLRI: 10.0.1.0/24
		24, 10, 0, 1,
	}

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 3, events, new(atomic.Int64))
	close(events)

	var received []Event
	for ev := range events {
		received = append(received, ev)
	}

	require.Len(t, received, 1)
	assert.Equal(t, EventRouteReceived, received[0].Type)
	assert.Equal(t, 3, received[0].PeerIndex)
	assert.Equal(t, netip.MustParsePrefix("10.0.1.0/24"), received[0].Prefix)
}

// TestParseUpdatePrefixesWithdraw verifies parsing withdrawn routes from an UPDATE body.
//
// VALIDATES: Withdrawn prefixes at the start of the body are extracted correctly.
// PREVENTS: Withdrawn routes being misidentified as announcements.
func TestParseUpdatePrefixesWithdraw(t *testing.T) {
	// UPDATE body with one withdrawal, no attributes, no NLRI.
	body := []byte{
		0, 4, // withdrawn routes length = 4
		// Withdrawn: 10.0.2.0/24
		24, 10, 0, 2,
		0, 0, // total path attribute length = 0
		// No NLRI.
	}

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 5, events, new(atomic.Int64))
	close(events)

	var received []Event
	for ev := range events {
		received = append(received, ev)
	}

	require.Len(t, received, 1)
	assert.Equal(t, EventRouteWithdrawn, received[0].Type)
	assert.Equal(t, 5, received[0].PeerIndex)
	assert.Equal(t, netip.MustParsePrefix("10.0.2.0/24"), received[0].Prefix)
}

// TestParseUpdatePrefixesMultiple verifies parsing an UPDATE with both
// withdrawals and announcements.
//
// VALIDATES: Mixed withdraw+announce UPDATEs are fully parsed.
// PREVENTS: Early termination after withdrawals, missing NLRI section.
func TestParseUpdatePrefixesMultiple(t *testing.T) {
	body := []byte{
		0, 4, // withdrawn routes length = 4
		// Withdrawn: 10.0.3.0/24
		24, 10, 0, 3,
		0, 7, // total path attribute length = 7
		// Minimal ORIGIN + AS_PATH
		0x40, 1, 1, 0,
		0x40, 2, 0,
		// NLRI: 10.0.4.0/24 and 10.0.5.0/24
		24, 10, 0, 4,
		24, 10, 0, 5,
	}

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 0, events, new(atomic.Int64))
	close(events)

	var withdrawals, announcements []netip.Prefix
	for ev := range events {
		switch ev.Type {
		case EventRouteWithdrawn:
			withdrawals = append(withdrawals, ev.Prefix)
		case EventRouteReceived:
			announcements = append(announcements, ev.Prefix)
		case EventEstablished, EventRouteSent, EventEORSent, EventDisconnected, EventError,
			EventChaosExecuted, EventReconnecting, EventWithdrawalSent, EventRouteAction:
			// Not expected in this test.
		}
	}

	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.3.0/24")}, withdrawals)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.4.0/24"),
		netip.MustParsePrefix("10.0.5.0/24"),
	}, announcements)
}

// TestParseUpdatePrefixesEOR verifies that an empty UPDATE (End-of-RIB)
// produces no events.
//
// VALIDATES: EOR marker (empty UPDATE) is harmless.
// PREVENTS: Spurious events from empty UPDATE bodies.
func TestParseUpdatePrefixesEOR(t *testing.T) {
	// Empty UPDATE body: withdrawn-len=0, attr-len=0, no NLRI.
	body := []byte{0, 0, 0, 0}

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 0, events, new(atomic.Int64))
	close(events)

	count := 0
	for range events {
		count++
	}
	assert.Equal(t, 0, count, "EOR should produce no events")
}

// TestParseIPv6Prefix verifies wire-format IPv6 prefix parsing.
//
// VALIDATES: Prefix length and address bytes are correctly decoded for IPv6.
// PREVENTS: Off-by-one in byte-length calculation or address masking for 128-bit addresses.
func TestParseIPv6Prefix(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want netip.Prefix
		n    int
	}{
		{
			name: "slash_64",
			data: []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00},
			want: netip.MustParsePrefix("2001:db8:1::/64"),
			n:    9,
		},
		{
			name: "slash_48",
			data: []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02},
			want: netip.MustParsePrefix("2001:db8:2::/48"),
			n:    7,
		},
		{
			name: "slash_128",
			data: []byte{128, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			want: netip.MustParsePrefix("2001:db8::1/128"),
			n:    17,
		},
		{
			name: "slash_0",
			data: []byte{0},
			want: netip.MustParsePrefix("::/0"),
			n:    1,
		},
		{
			name: "empty",
			data: []byte{},
			n:    0,
		},
		{
			name: "invalid_prefix_len_129",
			data: []byte{129, 0x20, 0x01},
			n:    0,
		},
		{
			name: "truncated",
			data: []byte{64, 0x20, 0x01}, // Needs 8 bytes but only 2 available.
			n:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, n := parseIPv6Prefix(tt.data)
			assert.Equal(t, tt.n, n)
			if tt.n > 0 {
				assert.Equal(t, tt.want, prefix)
			}
		})
	}
}

// TestParseUpdatePrefixesMPReach verifies that MP_REACH_NLRI attributes
// for IPv6/unicast are parsed and emit EventRouteReceived events.
//
// VALIDATES: IPv6 routes arriving via MP_REACH_NLRI are counted by the receiver.
// PREVENTS: Route reflection appearing broken because only IPv4/unicast NLRI was parsed.
func TestParseUpdatePrefixesMPReach(t *testing.T) {
	// Build an UPDATE body with no IPv4 NLRI, but an MP_REACH_NLRI
	// attribute carrying one IPv6/unicast prefix.
	//
	// MP_REACH_NLRI (type 14):
	//   AFI=2 (IPv6), SAFI=1 (unicast), NH-len=16, NH=::1, reserved=0,
	//   NLRI: 2001:db8:1::/48
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                           // SAFI = 1 (unicast)
		0x10,                                           // NH length = 16
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, // NH = ::1
		0x00,                                   // Reserved
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, // 2001:db8:1::/48
		64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00, // 2001:db8:2::/64
	}

	// Path attributes: ORIGIN(3 bytes) + MP_REACH_NLRI (extended-length).
	attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN = IGP
	}
	// MP_REACH_NLRI: flags=0x90 (optional, transitive, extended-length), code=14
	attrs = append(attrs, 0x90, 14, byte(len(mpReach)>>8), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)

	body := []byte{0, 0} // withdrawn-len = 0
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)
	// No trailing IPv4 NLRI.

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 7, events, new(atomic.Int64))
	close(events)

	var received []netip.Prefix
	for ev := range events {
		require.Equal(t, EventRouteReceived, ev.Type)
		assert.Equal(t, 7, ev.PeerIndex)
		received = append(received, ev.Prefix)
	}

	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("2001:db8:1::/48"),
		netip.MustParsePrefix("2001:db8:2::/64"),
	}, received)
}

// TestParseUpdatePrefixesMPUnreach verifies that MP_UNREACH_NLRI attributes
// for IPv6/unicast are parsed and emit EventRouteWithdrawn events.
//
// VALIDATES: IPv6 withdrawn routes arriving via MP_UNREACH_NLRI are counted.
// PREVENTS: Missing withdrawal tracking for non-IPv4 families.
func TestParseUpdatePrefixesMPUnreach(t *testing.T) {
	// MP_UNREACH_NLRI (type 15):
	//   AFI=2, SAFI=1, withdrawn: 2001:db8:3::/48
	mpUnreach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                   // SAFI = 1 (unicast)
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x03, // 2001:db8:3::/48
	}

	// Path attributes: only MP_UNREACH_NLRI.
	attrs := []byte{0x90, 15} // flags=optional+transitive+extended-length, code=15
	attrs = append(attrs, byte(len(mpUnreach)>>8), byte(len(mpUnreach)))
	attrs = append(attrs, mpUnreach...)

	body := []byte{0, 0} // withdrawn-len = 0 (no IPv4 withdrawals)
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 2, events, new(atomic.Int64))
	close(events)

	var withdrawn []netip.Prefix
	for ev := range events {
		require.Equal(t, EventRouteWithdrawn, ev.Type)
		assert.Equal(t, 2, ev.PeerIndex)
		withdrawn = append(withdrawn, ev.Prefix)
	}

	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("2001:db8:3::/48"),
	}, withdrawn)
}

// TestParseUpdatePrefixesMPReachVPN verifies that MP_REACH_NLRI for
// non-unicast families (e.g., IPv4/VPN) emits exactly one counted event
// with the correct family tag, without attempting to parse individual NLRI.
//
// VALIDATES: Non-unicast MP_REACH produces one event per UPDATE with family tag.
// PREVENTS: Garbage prefixes from misinterpreting VPN/EVPN NLRI as unicast prefixes.
func TestParseUpdatePrefixesMPReachVPN(t *testing.T) {
	// MP_REACH_NLRI with AFI=1, SAFI=128 (IPv4/VPN).
	mpReach := []byte{
		0x00, 0x01, // AFI = 1 (IPv4)
		0x80,                                // SAFI = 128 (VPN)
		0x0c,                                // NH length = 12
		0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1, // NH
		0x00,                 // Reserved
		24, 0x0a, 0x00, 0x01, // VPN NLRI bytes (not parsed as prefix)
	}

	attrs := []byte{0x90, 14}
	attrs = append(attrs, byte(len(mpReach)>>8), byte(len(mpReach)))
	attrs = append(attrs, mpReach...)

	body := []byte{0, 0}
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 0, events, new(atomic.Int64))
	close(events)

	var got []Event
	for ev := range events {
		got = append(got, ev)
	}
	require.Len(t, got, 1, "VPN MP_REACH should produce exactly 1 counted event")
	assert.Equal(t, EventRouteReceived, got[0].Type)
	assert.Equal(t, "ipv4/vpn", got[0].Family)
	assert.False(t, got[0].Prefix.IsValid(), "VPN event should not carry a parsed prefix")
}

// TestAfiSafiFamily verifies the AFI/SAFI to family string mapping.
//
// VALIDATES: All 7 supported families map correctly, unknown returns empty.
// PREVENTS: Typo in AFI/SAFI constants silently dropping route counts.
func TestAfiSafiFamily(t *testing.T) {
	tests := []struct {
		name string
		afi  uint16
		safi uint8
		want string
	}{
		{"ipv4_unicast", 1, 1, "ipv4/unicast"},
		{"ipv6_unicast", 2, 1, "ipv6/unicast"},
		{"ipv4_vpn", 1, 128, "ipv4/vpn"},
		{"ipv6_vpn", 2, 128, "ipv6/vpn"},
		{"l2vpn_evpn", 25, 70, "l2vpn/evpn"},
		{"ipv4_flow", 1, 133, "ipv4/flow"},
		{"ipv6_flow", 2, 133, "ipv6/flow"},
		{"unknown_afi", 99, 1, ""},
		{"unknown_safi", 1, 99, ""},
		{"zero", 0, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, afiSafiFamily(tt.afi, tt.safi))
		})
	}
}

// TestParseUpdatePrefixesMultipleAttrs verifies the attribute walker correctly
// iterates over multiple attributes to find MP_REACH_NLRI.
//
// VALIDATES: Attribute walker skips non-MP attributes and finds MP_REACH_NLRI.
// PREVENTS: Off-by-one in attribute length parsing breaking subsequent attributes.
func TestParseUpdatePrefixesMultipleAttrs(t *testing.T) {
	// MP_REACH_NLRI carrying one IPv6 prefix.
	mpReach := []byte{
		0x00, 0x02, // AFI = 2 (IPv6)
		0x01,                                           // SAFI = 1
		0x10,                                           // NH length = 16
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, // NH = ::1
		0x00,                                   // Reserved
		48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, // 2001:db8:1::/48
	}

	// Build attributes: ORIGIN + AS_PATH + MP_REACH_NLRI + MED.
	// Tests that the walker correctly steps over variable-length attributes.
	var attrs []byte

	// ORIGIN(flags=0x40,code=1,len=1,IGP) + AS_PATH(flags=0x40,code=2,len=6,AS_SEQ[65001]) + MP_REACH header.
	attrs = append(attrs,
		0x40, 1, 1, 0, // ORIGIN
		0x40, 2, 6, 2, 1, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH
		0x90, 14, byte(len(mpReach)>>8), byte(len(mpReach)), // MP_REACH_NLRI header
	)
	attrs = append(attrs, mpReach...)

	// MED: flags=0x80(optional), code=4, len=4, value=100.
	attrs = append(attrs, 0x80, 4, 4, 0, 0, 0, 100)

	body := []byte{0, 0} // withdrawn-len = 0
	body = append(body, byte(len(attrs)>>8), byte(len(attrs)))
	body = append(body, attrs...)

	events := make(chan Event, 10)

	parseUpdatePrefixes(body, 5, events, new(atomic.Int64))
	close(events)

	var got []Event
	for ev := range events {
		got = append(got, ev)
	}
	require.Len(t, got, 1, "should find MP_REACH_NLRI despite surrounding attributes")
	assert.Equal(t, EventRouteReceived, got[0].Type)
	assert.Equal(t, "ipv6/unicast", got[0].Family)
	assert.Equal(t, netip.MustParsePrefix("2001:db8:1::/48"), got[0].Prefix)
}
