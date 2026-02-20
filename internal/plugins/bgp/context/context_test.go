package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// TestEncodingContextDelegation verifies methods delegate to sub-components.
//
// VALIDATES: EncodingContext accessors return values from referenced sub-components.
//
// PREVENTS: Wrong capability values due to failed delegation.
func TestEncodingContextDelegation(t *testing.T) {
	identity := &capability.PeerIdentity{
		LocalASN:      65001,
		PeerASN:       65002,
		LocalRouterID: 0x0a000001,
		PeerRouterID:  0x0a000002,
	}
	encoding := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
			{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathBoth,
		},
		ExtendedNextHop: map[capability.Family]capability.AFI{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: nlri.AFIIPv6,
		},
	}

	ctx := NewEncodingContext(identity, encoding, DirectionRecv)

	// Test delegation to Identity
	assert.Equal(t, uint32(65001), ctx.LocalASN())
	assert.Equal(t, uint32(65002), ctx.PeerASN())
	assert.False(t, ctx.IsIBGP())

	// Test delegation to Encoding
	assert.True(t, ctx.ASN4())
	assert.Len(t, ctx.Families(), 2)
	assert.Equal(t, nlri.AFIIPv6, ctx.ExtendedNextHopFor(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}))
}

// TestEncodingContextAddPath verifies ADD-PATH direction handling.
//
// RFC 7911 Section 4: ADD-PATH mode is asymmetric
//   - Receive: check for Receive or Both mode
//   - Send: check for Send or Both mode
//
// VALIDATES: AddPath returns correct result based on mode and direction.
//
// PREVENTS: Wrong path ID handling when ADD-PATH is negotiated.
func TestEncodingContextAddPath(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}
	encoding := &capability.EncodingCaps{
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
			{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast},
			{AFI: nlri.AFIL2VPN, SAFI: nlri.SAFIEVPN},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathReceive, // Recv only
			{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}: capability.AddPathSend,    // Send only
			{AFI: nlri.AFIL2VPN, SAFI: nlri.SAFIEVPN}:   capability.AddPathBoth,    // Both
		},
	}

	tests := []struct {
		name      string
		direction Direction
		expects   map[nlri.Family]bool
	}{
		{
			name:      "receive direction",
			direction: DirectionRecv,
			expects: map[nlri.Family]bool{
				{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: true,  // Receive mode
				{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}: false, // Send mode = no recv
				{AFI: nlri.AFIL2VPN, SAFI: nlri.SAFIEVPN}:   true,  // Both mode
			},
		},
		{
			name:      "send direction",
			direction: DirectionSend,
			expects: map[nlri.Family]bool{
				{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: false, // Receive mode = no send
				{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}: true,  // Send mode
				{AFI: nlri.AFIL2VPN, SAFI: nlri.SAFIEVPN}:   true,  // Both mode
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEncodingContext(identity, encoding, tt.direction)
			for f, want := range tt.expects {
				assert.Equal(t, want, ctx.AddPath(f), "AddPath(%v)", f)
			}
		})
	}
}

// TestEncodingContextHash verifies hash consistency.
//
// VALIDATES: Same parameters produce same hash.
//
// PREVENTS: Registry deduplication failures.
func TestEncodingContextHash(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}
	encoding := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathBoth,
		},
	}

	ctx1 := NewEncodingContext(identity, encoding, DirectionRecv)
	ctx2 := NewEncodingContext(identity, encoding, DirectionRecv)

	require.NotZero(t, ctx1.Hash())
	assert.Equal(t, ctx1.Hash(), ctx2.Hash(), "Same params should produce same hash")
}

// TestEncodingContextHashDiffersByDirection verifies direction affects hash.
//
// VALIDATES: Recv and Send contexts have different hashes.
//
// PREVENTS: Incorrect zero-copy when directions differ.
func TestEncodingContextHashDiffersByDirection(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}
	encoding := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathBoth,
		},
	}

	recvCtx := NewEncodingContext(identity, encoding, DirectionRecv)
	sendCtx := NewEncodingContext(identity, encoding, DirectionSend)

	assert.NotEqual(t, recvCtx.Hash(), sendCtx.Hash(),
		"Different directions should produce different hashes")
}

// TestEncodingContextHashDiffersByAddPath verifies addPath map affects hash.
//
// VALIDATES: Different addPath produces different hash.
//
// PREVENTS: Wrong zero-copy decision when ADD-PATH differs.
func TestEncodingContextHashDiffersByAddPath(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	encoding1 := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathBoth,
		},
	}
	encoding2 := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{},
	}

	ctx1 := NewEncodingContext(identity, encoding1, DirectionRecv)
	ctx2 := NewEncodingContext(identity, encoding2, DirectionRecv)

	assert.NotEqual(t, ctx1.Hash(), ctx2.Hash(),
		"Different addPath should produce different hashes")
}

// TestEncodingContextAddPathAndASN4 verifies ADD-PATH and ASN4 accessor methods.
//
// VALIDATES: ASN4() and AddPath(family) return correct values.
//
// PREVENTS: NLRI encoding with wrong ADD-PATH or ASN4 setting.
func TestEncodingContextAddPathAndASN4(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}
	encoding := &capability.EncodingCaps{
		ASN4: true,
		Families: []capability.Family{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast},
			{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast},
		},
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathBoth,
		},
	}

	ctx := NewEncodingContext(identity, encoding, DirectionSend)

	// Check ASN4
	assert.True(t, ctx.ASN4())

	// IPv4 unicast with ADD-PATH
	assert.True(t, ctx.AddPath(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}), "IPv4 unicast should have ADD-PATH enabled")

	// IPv6 unicast without ADD-PATH
	assert.False(t, ctx.AddPath(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}), "IPv6 unicast should NOT have ADD-PATH enabled")
}

// TestEncodingContextForASN4 verifies the helper constructor.
//
// VALIDATES: EncodingContextForASN4 creates context with correct ASN4 setting.
//
// PREVENTS: Wrong ASN encoding in attribute packing.
func TestEncodingContextForASN4(t *testing.T) {
	ctx4 := EncodingContextForASN4(true)
	require.NotNil(t, ctx4)
	assert.True(t, ctx4.ASN4())

	ctx2 := EncodingContextForASN4(false)
	require.NotNil(t, ctx2)
	assert.False(t, ctx2.ASN4())
}

// TestEncodingContextAddPathFor verifies the AddPathFor alias.
//
// VALIDATES: AddPathFor returns same result as AddPath.
//
// PREVENTS: API compatibility issues.
func TestEncodingContextAddPathFor(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}
	encoding := &capability.EncodingCaps{
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}: capability.AddPathSend,
		},
	}

	ctx := NewEncodingContext(identity, encoding, DirectionSend)
	f := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	assert.Equal(t, ctx.AddPath(f), ctx.AddPathFor(f))
}

// -----------------------------------------------------------------------------
// Boundary Tests: ExtendedMessage and MaxMessageSize
// -----------------------------------------------------------------------------

// TestExtendedMessageNilContext verifies nil context returns false.
//
// VALIDATES: ExtendedMessage() returns false for nil context.
// PREVENTS: Panic on nil context dereference.
// BOUNDARY: nil is the edge case for context.
func TestExtendedMessageNilContext(t *testing.T) {
	var ctx *EncodingContext
	assert.False(t, ctx.ExtendedMessage(), "nil context should return false")
}

// TestExtendedMessageNilEncoding verifies nil encoding returns false.
//
// VALIDATES: ExtendedMessage() returns false when encoding is nil.
// PREVENTS: Panic on nil encoding dereference.
// BOUNDARY: Empty context (encoding=nil) is edge case.
func TestExtendedMessageNilEncoding(t *testing.T) {
	ctx := &EncodingContext{} // encoding is nil
	assert.False(t, ctx.ExtendedMessage(), "nil encoding should return false")
}

// TestExtendedMessageValues verifies ExtendedMessage returns correct value.
//
// VALIDATES: ExtendedMessage() reflects EncodingCaps.ExtendedMessage.
// PREVENTS: Wrong message size limit calculation.
func TestExtendedMessageValues(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	tests := []struct {
		name     string
		extended bool
	}{
		{"extended_false", false},
		{"extended_true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoding := &capability.EncodingCaps{
				ExtendedMessage: tt.extended,
			}
			ctx := NewEncodingContext(identity, encoding, DirectionSend)
			assert.Equal(t, tt.extended, ctx.ExtendedMessage())
		})
	}
}

// TestMaxMessageSizeBoundary verifies MaxMessageSize boundary values.
//
// RFC 4271: Standard max message size = 4096 bytes
// RFC 8654: Extended max message size = 65535 bytes
//
// VALIDATES: MaxMessageSize returns correct values at boundaries.
// PREVENTS: Buffer allocation too small or too large.
// BOUNDARY: nil context, ExtendedMessage=false, ExtendedMessage=true.
func TestMaxMessageSizeBoundary(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	tests := []struct {
		name     string
		ctx      *EncodingContext
		wantSize int
	}{
		{
			name:     "nil_context_returns_4096",
			ctx:      nil,
			wantSize: 4096,
		},
		{
			name: "extended_false_returns_4096",
			ctx: NewEncodingContext(identity, &capability.EncodingCaps{
				ExtendedMessage: false,
			}, DirectionSend),
			wantSize: 4096,
		},
		{
			name: "extended_true_returns_65535",
			ctx: NewEncodingContext(identity, &capability.EncodingCaps{
				ExtendedMessage: true,
			}, DirectionSend),
			wantSize: 65535,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.MaxMessageSize() // nil-safe
			assert.Equal(t, tt.wantSize, got, "MaxMessageSize mismatch")
		})
	}
}

// TestMaxMessageSizeExactValues verifies exact boundary constants.
//
// VALIDATES: MaxMessageSize returns exact RFC values, not off-by-one.
// PREVENTS: Off-by-one errors in buffer allocation.
// BOUNDARY: Exact values 4096 and 65535.
func TestMaxMessageSizeExactValues(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	// Standard: exactly 4096 (RFC 4271)
	stdCtx := NewEncodingContext(identity, &capability.EncodingCaps{
		ExtendedMessage: false,
	}, DirectionSend)
	assert.Equal(t, 4096, stdCtx.MaxMessageSize(), "Standard size must be exactly 4096")

	// Extended: exactly 65535 (RFC 8654)
	extCtx := NewEncodingContext(identity, &capability.EncodingCaps{
		ExtendedMessage: true,
	}, DirectionSend)
	assert.Equal(t, 65535, extCtx.MaxMessageSize(), "Extended size must be exactly 65535")
}

// TestExtendedMessageHashDiffers verifies ExtendedMessage affects context hash.
//
// VALIDATES: Different ExtendedMessage values produce different hashes.
// PREVENTS: Wrong zero-copy decision when message size limits differ.
func TestExtendedMessageHashDiffers(t *testing.T) {
	identity := &capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002}

	ctx1 := NewEncodingContext(identity, &capability.EncodingCaps{
		ExtendedMessage: false,
	}, DirectionSend)

	ctx2 := NewEncodingContext(identity, &capability.EncodingCaps{
		ExtendedMessage: true,
	}, DirectionSend)

	assert.NotEqual(t, ctx1.Hash(), ctx2.Hash(),
		"Different ExtendedMessage should produce different hashes")
}
