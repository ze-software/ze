package peer

import (
	"context"
	"net/netip"
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
	ctx := context.Background()

	parseUpdatePrefixes(body, 3, events, ctx)
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
	ctx := context.Background()

	parseUpdatePrefixes(body, 5, events, ctx)
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
	ctx := context.Background()

	parseUpdatePrefixes(body, 0, events, ctx)
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
	ctx := context.Background()

	parseUpdatePrefixes(body, 0, events, ctx)
	close(events)

	count := 0
	for range events {
		count++
	}
	assert.Equal(t, 0, count, "EOR should produce no events")
}
