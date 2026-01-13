package message

import (
	"testing"

	bgpctx "codeberg.org/thomas-mangin/zebgp/pkg/bgp/context"
	"github.com/stretchr/testify/assert"
)

// TestKeepaliveWireWriter verifies Keepalive implements WireWriter interface.
//
// VALIDATES: Keepalive has Len(ctx) and WriteTo(buf, off, ctx) methods.
// PREVENTS: Interface mismatch breaking message building.
func TestKeepaliveWireWriter(t *testing.T) {
	var _ bgpctx.WireWriter = (*Keepalive)(nil)
}

// TestKeepaliveLenWithContext verifies Keepalive.Len returns 19 bytes.
//
// RFC 4271 Section 4.4: "A KEEPALIVE message consists of only the message
// header and has a length of 19 octets."
//
// VALIDATES: Len returns HeaderLen (19) regardless of context.
// PREVENTS: Incorrect buffer allocation for KEEPALIVE.
func TestKeepaliveLenWithContext(t *testing.T) {
	k := NewKeepalive()

	// Context-independent: should return 19 with nil context
	assert.Equal(t, HeaderLen, k.Len(nil), "Keepalive.Len(nil) should be HeaderLen")

	// Should also return 19 with any context
	ctx := &bgpctx.EncodingContext{}
	assert.Equal(t, HeaderLen, k.Len(ctx), "Keepalive.Len(ctx) should be HeaderLen")
}

// TestKeepaliveWriteToWithContext verifies Keepalive.WriteTo writes valid message.
//
// VALIDATES: WriteTo writes complete KEEPALIVE message to buffer.
// PREVENTS: Incomplete or corrupt KEEPALIVE messages.
func TestKeepaliveWriteToWithContext(t *testing.T) {
	k := NewKeepalive()
	buf := make([]byte, 100)

	n := k.WriteTo(buf, 0, nil)
	assert.Equal(t, HeaderLen, n, "WriteTo should write HeaderLen bytes")

	// Verify header is valid
	h, err := ParseHeader(buf[:n])
	if assert.NoError(t, err) {
		assert.Equal(t, TypeKEEPALIVE, h.Type)
		assert.Equal(t, uint16(HeaderLen), h.Length)
	}
}

// TestOpenWireWriter verifies Open implements WireWriter interface.
//
// VALIDATES: Open has Len(ctx) and WriteTo(buf, off, ctx) methods.
// PREVENTS: Interface mismatch breaking message building.
func TestOpenWireWriter(t *testing.T) {
	var _ bgpctx.WireWriter = (*Open)(nil)
}

// TestNotificationWireWriter verifies Notification implements WireWriter interface.
//
// VALIDATES: Notification has Len(ctx) and WriteTo(buf, off, ctx) methods.
// PREVENTS: Interface mismatch breaking message building.
func TestNotificationWireWriter(t *testing.T) {
	var _ bgpctx.WireWriter = (*Notification)(nil)
}

// TestUpdateWireWriter verifies Update implements WireWriter interface.
//
// VALIDATES: Update has Len(ctx) and WriteTo(buf, off, ctx) methods.
// PREVENTS: Interface mismatch breaking message building.
func TestUpdateWireWriter(t *testing.T) {
	var _ bgpctx.WireWriter = (*Update)(nil)
}

// TestRouteRefreshWireWriter verifies RouteRefresh implements WireWriter interface.
//
// VALIDATES: RouteRefresh has Len(ctx) and WriteTo(buf, off, ctx) methods.
// PREVENTS: Interface mismatch breaking message building.
func TestRouteRefreshWireWriter(t *testing.T) {
	var _ bgpctx.WireWriter = (*RouteRefresh)(nil)
}

// -----------------------------------------------------------------------------
// Behavioral Tests: WriteTo produces same output as Pack
// -----------------------------------------------------------------------------

// TestKeepaliveWriteToMatchesPack verifies WriteTo produces identical output to Pack.
//
// VALIDATES: WriteTo and Pack produce byte-identical output.
// PREVENTS: Divergence between old Pack and new WriteTo implementations.
func TestKeepaliveWriteToMatchesPack(t *testing.T) {
	k := NewKeepalive()

	// Get expected from Pack
	expected, err := k.Pack(nil)
	assert.NoError(t, err)

	// Get actual from WriteTo
	buf := make([]byte, k.Len(nil))
	n := k.WriteTo(buf, 0, nil)

	assert.Equal(t, len(expected), n, "WriteTo length mismatch")
	assert.Equal(t, expected, buf[:n], "WriteTo output mismatch")
}

// TestOpenWriteToMatchesPack verifies WriteTo produces identical output to Pack.
//
// VALIDATES: WriteTo and Pack produce byte-identical output for OPEN.
// PREVENTS: Divergence between old Pack and new WriteTo implementations.
func TestOpenWriteToMatchesPack(t *testing.T) {
	tests := []struct {
		name string
		open *Open
	}{
		{
			name: "minimal",
			open: &Open{
				Version:        4,
				MyAS:           65001,
				HoldTime:       180,
				BGPIdentifier:  0x01020304,
				OptionalParams: nil,
			},
		},
		{
			name: "with_capabilities",
			open: &Open{
				Version:        4,
				MyAS:           65001,
				HoldTime:       90,
				BGPIdentifier:  0xC0A80001,
				OptionalParams: []byte{0x02, 0x06, 0x01, 0x04, 0x00, 0x01, 0x00, 0x01},
			},
		},
		{
			name: "asn4_needs_trans",
			open: &Open{
				Version:       4,
				MyAS:          23456, // AS_TRANS
				ASN4:          400000,
				HoldTime:      180,
				BGPIdentifier: 0x01020304,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := tt.open.Pack(nil)
			assert.NoError(t, err)

			buf := make([]byte, tt.open.Len(nil))
			n := tt.open.WriteTo(buf, 0, nil)

			assert.Equal(t, len(expected), n, "WriteTo length mismatch")
			assert.Equal(t, expected, buf[:n], "WriteTo output mismatch")
		})
	}
}

// TestNotificationWriteToMatchesPack verifies WriteTo produces identical output to Pack.
//
// VALIDATES: WriteTo and Pack produce byte-identical output for NOTIFICATION.
// PREVENTS: Divergence between old Pack and new WriteTo implementations.
func TestNotificationWriteToMatchesPack(t *testing.T) {
	tests := []struct {
		name  string
		notif *Notification
	}{
		{
			name: "cease_no_data",
			notif: &Notification{
				ErrorCode:    NotifyCease,
				ErrorSubcode: NotifyCeaseAdminShutdown,
			},
		},
		{
			name: "with_data",
			notif: &Notification{
				ErrorCode:    NotifyOpenMessage,
				ErrorSubcode: NotifyOpenUnsupportedCapability,
				Data:         []byte{0x01, 0x04, 0x00, 0x01, 0x00, 0x01},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := tt.notif.Pack(nil)
			assert.NoError(t, err)

			buf := make([]byte, tt.notif.Len(nil))
			n := tt.notif.WriteTo(buf, 0, nil)

			assert.Equal(t, len(expected), n, "WriteTo length mismatch")
			assert.Equal(t, expected, buf[:n], "WriteTo output mismatch")
		})
	}
}

// TestRouteRefreshWriteToMatchesPack verifies WriteTo produces identical output to Pack.
//
// VALIDATES: WriteTo and Pack produce byte-identical output for ROUTE-REFRESH.
// PREVENTS: Divergence between old Pack and new WriteTo implementations.
func TestRouteRefreshWriteToMatchesPack(t *testing.T) {
	tests := []struct {
		name string
		rr   *RouteRefresh
	}{
		{
			name: "ipv4_unicast",
			rr: &RouteRefresh{
				AFI:     1,
				SAFI:    1,
				Subtype: RouteRefreshNormal,
			},
		},
		{
			name: "ipv6_unicast_borr",
			rr: &RouteRefresh{
				AFI:     2,
				SAFI:    1,
				Subtype: RouteRefreshBoRR,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := tt.rr.Pack(nil)
			assert.NoError(t, err)

			buf := make([]byte, tt.rr.Len(nil))
			n := tt.rr.WriteTo(buf, 0, nil)

			assert.Equal(t, len(expected), n, "WriteTo length mismatch")
			assert.Equal(t, expected, buf[:n], "WriteTo output mismatch")
		})
	}
}

// TestUpdateWriteToMatchesPack verifies WriteTo produces identical output to Pack.
//
// VALIDATES: WriteTo and Pack produce byte-identical output for UPDATE.
// PREVENTS: Divergence between old Pack and new WriteTo implementations.
func TestUpdateWriteToMatchesPack(t *testing.T) {
	tests := []struct {
		name   string
		update *Update
	}{
		{
			name:   "empty_eor",
			update: &Update{},
		},
		{
			name: "with_nlri",
			update: &Update{
				PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, // ORIGIN IGP
				NLRI:           []byte{0x18, 0xC0, 0xA8, 0x01}, // 192.168.1.0/24
			},
		},
		{
			name: "with_withdrawn",
			update: &Update{
				WithdrawnRoutes: []byte{0x18, 0xC0, 0xA8, 0x02}, // 192.168.2.0/24
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := tt.update.Pack(nil)
			assert.NoError(t, err)

			buf := make([]byte, tt.update.Len(nil))
			n := tt.update.WriteTo(buf, 0, nil)

			assert.Equal(t, len(expected), n, "WriteTo length mismatch")
			assert.Equal(t, expected, buf[:n], "WriteTo output mismatch")
		})
	}
}

// -----------------------------------------------------------------------------
// Behavioral Tests: Len matches WriteTo output
// -----------------------------------------------------------------------------

// TestAllMessagesLenMatchesWriteTo verifies Len accurately predicts WriteTo size.
//
// VALIDATES: Len(ctx) returns exact byte count that WriteTo will produce.
// PREVENTS: Buffer overflow from undersized allocation.
func TestAllMessagesLenMatchesWriteTo(t *testing.T) {
	messages := []struct {
		name string
		msg  Message
	}{
		{"keepalive", NewKeepalive()},
		{"notification_minimal", &Notification{ErrorCode: 6, ErrorSubcode: 1}},
		{"notification_with_data", &Notification{ErrorCode: 6, ErrorSubcode: 1, Data: []byte{1, 2, 3}}},
		{"routerefresh", &RouteRefresh{AFI: 1, SAFI: 1}},
		{"open_minimal", &Open{Version: 4, MyAS: 65001, HoldTime: 180, BGPIdentifier: 0x01020304}},
		{"update_empty", &Update{}},
		{"update_with_data", &Update{PathAttributes: []byte{0x40, 0x01, 0x01, 0x00}, NLRI: []byte{0x18, 0x0a}}},
	}

	for _, tt := range messages {
		t.Run(tt.name, func(t *testing.T) {
			expectedLen := tt.msg.Len(nil)
			buf := make([]byte, expectedLen+100) // Extra space to detect overwrite
			n := tt.msg.WriteTo(buf, 0, nil)

			assert.Equal(t, expectedLen, n, "Len() != WriteTo() bytes written")
		})
	}
}

// -----------------------------------------------------------------------------
// Behavioral Tests: WriteTo respects offset parameter
// -----------------------------------------------------------------------------

// TestWriteToRespectsOffset verifies WriteTo writes at correct buffer position.
//
// VALIDATES: WriteTo uses offset parameter correctly.
// PREVENTS: Buffer corruption from writing at wrong position.
func TestWriteToRespectsOffset(t *testing.T) {
	messages := []Message{
		NewKeepalive(),
		&Notification{ErrorCode: 6, ErrorSubcode: 1},
		&RouteRefresh{AFI: 1, SAFI: 1},
	}

	for _, msg := range messages {
		t.Run(msg.Type().String(), func(t *testing.T) {
			offset := 50
			buf := make([]byte, 200)

			// Fill buffer with marker bytes
			for i := range buf {
				buf[i] = 0xAA
			}

			n := msg.WriteTo(buf, offset, nil)

			// Verify bytes before offset are untouched
			for i := 0; i < offset; i++ {
				assert.Equal(t, byte(0xAA), buf[i], "byte %d before offset modified", i)
			}

			// Verify message starts at offset
			h, err := ParseHeader(buf[offset : offset+n])
			assert.NoError(t, err)
			assert.Equal(t, msg.Type(), h.Type)

			// Verify bytes after message are untouched
			for i := offset + n; i < len(buf); i++ {
				assert.Equal(t, byte(0xAA), buf[i], "byte %d after message modified", i)
			}
		})
	}
}
