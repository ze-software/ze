package server

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestOpenValidationError verifies the OpenValidationError type.
//
// VALIDATES: Error() returns human-readable message, NotifyCodes() returns correct codes.
// PREVENTS: NOTIFICATION codes being lost when wrapping validation errors.
func TestOpenValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		notifyCode    uint8
		notifySubcode uint8
		reason        string
		wantMsg       string
	}{
		{
			name:          "role_mismatch",
			notifyCode:    2,  // OPEN Message Error
			notifySubcode: 11, // Role Mismatch
			reason:        "role mismatch: customer↔customer",
			wantMsg:       "open validation rejected: role mismatch: customer↔customer",
		},
		{
			name:          "empty_reason",
			notifyCode:    2,
			notifySubcode: 0,
			reason:        "",
			wantMsg:       "open validation rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &OpenValidationError{
				NotifyCode:    tt.notifyCode,
				NotifySubcode: tt.notifySubcode,
				Reason:        tt.reason,
			}

			// Verify Error() returns readable message
			assert.Equal(t, tt.wantMsg, err.Error())

			// Verify NotifyCodes() returns correct values
			code, sub := err.NotifyCodes()
			assert.Equal(t, tt.notifyCode, code)
			assert.Equal(t, tt.notifySubcode, sub)

			// Verify it satisfies the interface used by session.go
			var valErr interface{ NotifyCodes() (uint8, uint8) }
			assert.True(t, errors.As(err, &valErr))
			c, s := valErr.NotifyCodes()
			assert.Equal(t, tt.notifyCode, c)
			assert.Equal(t, tt.notifySubcode, s)
		})
	}
}

// TestOpenMessageToRPC verifies conversion from message.Open to rpc.ValidateOpenMessage.
//
// VALIDATES: ASN (with ASN4), RouterID, HoldTime, and capabilities are correctly extracted.
// PREVENTS: Capability TLVs being mangled during conversion to {code, hex} pairs.
func TestOpenMessageToRPC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		open     *message.Open
		wantASN  uint32
		wantRID  string
		wantHold uint16
		wantCaps []rpc.ValidateOpenCapability
	}{
		{
			name: "basic_open_with_role",
			open: &message.Open{
				MyAS:          65000,
				HoldTime:      180,
				BGPIdentifier: 0x01020304, // 1.2.3.4
				// OptionalParams: type=2 (capabilities), len=3, cap_code=9, cap_len=1, value=0x03
				OptionalParams: []byte{2, 3, 9, 1, 0x03},
			},
			wantASN:  65000,
			wantRID:  "1.2.3.4",
			wantHold: 180,
			wantCaps: []rpc.ValidateOpenCapability{
				{Code: 9, Hex: "03"},
			},
		},
		{
			name: "asn4_overrides_myas",
			open: &message.Open{
				MyAS:           23456, // AS_TRANS
				HoldTime:       90,
				BGPIdentifier:  0x0a000001, // 10.0.0.1
				ASN4:           100000,
				OptionalParams: []byte{2, 6, 65, 4, 0x00, 0x01, 0x86, 0xA0}, // ASN4 cap
			},
			wantASN:  100000,
			wantRID:  "10.0.0.1",
			wantHold: 90,
			wantCaps: []rpc.ValidateOpenCapability{
				{Code: 65, Hex: "000186a0"},
			},
		},
		{
			name: "no_capabilities",
			open: &message.Open{
				MyAS:          65001,
				HoldTime:      60,
				BGPIdentifier: 0xc0a80101, // 192.168.1.1
			},
			wantASN:  65001,
			wantRID:  "192.168.1.1",
			wantHold: 60,
			wantCaps: nil,
		},
		{
			name: "multiple_capabilities",
			open: &message.Open{
				MyAS:          65002,
				HoldTime:      180,
				BGPIdentifier: 0x05060708, // 5.6.7.8
				// Two capability optional parameters
				// Param 1: type=2, len=9
				// cap: code=1, len=4, AFI(2 bytes)=0x0001, reserved=0, SAFI=1
				// cap: code=9, len=1, value=0x00
				OptionalParams: func() []byte {
					return []byte{2, 9, 1, 4, 0x00, 0x01, 0x00, 0x01, 9, 1, 0x00}
				}(),
			},
			wantASN:  65002,
			wantRID:  "5.6.7.8",
			wantHold: 180,
			wantCaps: []rpc.ValidateOpenCapability{
				{Code: 1, Hex: "00010001"},
				{Code: 9, Hex: "00"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := openMessageToRPC(tt.open)
			assert.Equal(t, tt.wantASN, got.ASN)
			assert.Equal(t, tt.wantRID, got.RouterID)
			assert.Equal(t, tt.wantHold, got.HoldTime)
			assert.Equal(t, tt.wantCaps, got.Capabilities)
		})
	}
}

// TestBroadcastValidateOpenCapabilityHexEncoding verifies capability hex encoding in RPC.
//
// VALIDATES: Capability values are hex-encoded correctly (lowercase, no prefix).
// PREVENTS: Hex encoding issues (uppercase, 0x prefix, truncation).
func TestBroadcastValidateOpenCapabilityHexEncoding(t *testing.T) {
	t.Parallel()

	// Test that openMessageToRPC produces correct hex for various cap values
	open := &message.Open{
		MyAS:          65000,
		HoldTime:      180,
		BGPIdentifier: 0x01020304,
		// Cap: code=64 (GR), len=6, value=0x40, 0x78, 0x00, 0x01, 0x01, 0x00
		OptionalParams: []byte{2, 8, 64, 6, 0x40, 0x78, 0x00, 0x01, 0x01, 0x00},
	}

	msg := openMessageToRPC(open)
	require.Equal(t, 1, len(msg.Capabilities))
	assert.Equal(t, uint8(64), msg.Capabilities[0].Code)
	// hex.EncodeToString produces lowercase
	assert.Equal(t, hex.EncodeToString([]byte{0x40, 0x78, 0x00, 0x01, 0x01, 0x00}), msg.Capabilities[0].Hex)
}
