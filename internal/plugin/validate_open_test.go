package plugin

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
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
				// Param 1: type=2, len=7, [cap1: code=1, len=4, AFI=1 SAFI=1] [cap2: code=9, len=1, value=0x00]
				OptionalParams: func() []byte {
					// Build: type=2, len=9
					// cap: code=1, len=4, AFI(2 bytes)=0x0001, reserved=0, SAFI=1
					// cap: code=9, len=1, value=0x00
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

// TestBroadcastValidateOpenNoProcesses verifies BroadcastValidateOpen with no processes.
//
// VALIDATES: When no procManager exists, BroadcastValidateOpen returns nil.
// PREVENTS: Nil pointer panic when no plugins are running.
func TestBroadcastValidateOpenNoProcesses(t *testing.T) {
	t.Parallel()

	s := &Server{}
	open := &message.Open{MyAS: 65000, HoldTime: 180, BGPIdentifier: 0x01020304}
	err := s.BroadcastValidateOpen("10.0.0.1", open, open)
	assert.NoError(t, err)
}

// TestBroadcastValidateOpenNoInterested verifies behavior when no plugin wants validate-open.
//
// VALIDATES: Plugins without WantsValidateOpen are skipped.
// PREVENTS: All plugins being called regardless of WantsValidateOpen flag.
func TestBroadcastValidateOpenNoInterested(t *testing.T) {
	t.Parallel()

	s := &Server{
		procManager: &ProcessManager{
			processes: map[string]*Process{
				"rib": {
					config:       PluginConfig{Name: "rib"},
					registration: &PluginRegistration{WantsValidateOpen: false},
				},
			},
		},
	}

	open := &message.Open{MyAS: 65000, HoldTime: 180, BGPIdentifier: 0x01020304}
	err := s.BroadcastValidateOpen("10.0.0.1", open, open)
	assert.NoError(t, err)
}

// TestBroadcastValidateOpenAccepted verifies broadcast when plugin accepts.
//
// VALIDATES: Plugin with WantsValidateOpen=true that returns accept=true → nil error.
// PREVENTS: Accept response being misinterpreted as rejection.
func TestBroadcastValidateOpenAccepted(t *testing.T) {
	t.Parallel()

	// Create socket pairs for the mock process
	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	proc := &Process{
		config:       PluginConfig{Name: "role"},
		registration: &PluginRegistration{WantsValidateOpen: true},
		engineConnB:  NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	s := &Server{
		ctx: ctx,
		procManager: &ProcessManager{
			processes: map[string]*Process{"role": proc},
		},
	}

	// Simulate plugin responding with accept on callback socket
	pluginConn := NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)
	go func() {
		req, err := pluginConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConn.SendResult(ctx, req.ID, &rpc.ValidateOpenOutput{Accept: true})
	}()

	localOpen := &message.Open{
		MyAS: 65000, HoldTime: 180, BGPIdentifier: 0x01020304,
		OptionalParams: []byte{2, 3, 9, 1, 0x03}, // Role: customer
	}
	remoteOpen := &message.Open{
		MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x05060708,
		OptionalParams: []byte{2, 3, 9, 1, 0x00}, // Role: provider
	}

	err = s.BroadcastValidateOpen("10.0.0.1", localOpen, remoteOpen)
	assert.NoError(t, err)
}

// TestBroadcastValidateOpenRejected verifies broadcast when plugin rejects.
//
// VALIDATES: Plugin rejection returns OpenValidationError with correct NOTIFICATION codes.
// PREVENTS: Rejection not propagating or losing NOTIFICATION code/subcode.
func TestBroadcastValidateOpenRejected(t *testing.T) {
	t.Parallel()

	pairs, err := NewInternalSocketPairs()
	require.NoError(t, err)
	t.Cleanup(func() { pairs.Close() })

	proc := &Process{
		config:       PluginConfig{Name: "role"},
		registration: &PluginRegistration{WantsValidateOpen: true},
		engineConnB:  NewPluginConn(pairs.Callback.EngineSide, pairs.Callback.EngineSide),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	s := &Server{
		ctx: ctx,
		procManager: &ProcessManager{
			processes: map[string]*Process{"role": proc},
		},
	}

	// Simulate plugin responding with rejection
	pluginConn := NewPluginConn(pairs.Callback.PluginSide, pairs.Callback.PluginSide)
	go func() {
		req, err := pluginConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		_ = pluginConn.SendResult(ctx, req.ID, &rpc.ValidateOpenOutput{
			Accept:        false,
			NotifyCode:    2,
			NotifySubcode: 11,
			Reason:        "role mismatch: customer↔customer",
		})
	}()

	localOpen := &message.Open{
		MyAS: 65000, HoldTime: 180, BGPIdentifier: 0x01020304,
		OptionalParams: []byte{2, 3, 9, 1, 0x03},
	}
	remoteOpen := &message.Open{
		MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x05060708,
		OptionalParams: []byte{2, 3, 9, 1, 0x03}, // same role = conflict
	}

	err = s.BroadcastValidateOpen("10.0.0.1", localOpen, remoteOpen)
	require.Error(t, err)

	// Verify it's an OpenValidationError with correct codes
	var valErr *OpenValidationError
	require.True(t, errors.As(err, &valErr))
	assert.Equal(t, uint8(2), valErr.NotifyCode)
	assert.Equal(t, uint8(11), valErr.NotifySubcode)
	assert.Equal(t, "role mismatch: customer↔customer", valErr.Reason)

	// Verify NotifyCodes() interface works
	code, sub := valErr.NotifyCodes()
	assert.Equal(t, uint8(2), code)
	assert.Equal(t, uint8(11), sub)
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

// TestRegistrationFromRPCWantsValidateOpen verifies WantsValidateOpen propagation.
//
// VALIDATES: WantsValidateOpen from RPC input is copied to PluginRegistration.
// PREVENTS: WantsValidateOpen being ignored during Stage 1 conversion.
func TestRegistrationFromRPCWantsValidateOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input bool
		want  bool
	}{
		{"true_propagates", true, true},
		{"false_propagates", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := &rpc.DeclareRegistrationInput{
				WantsValidateOpen: tt.input,
			}
			reg := registrationFromRPC(input)
			assert.Equal(t, tt.want, reg.WantsValidateOpen)
		})
	}
}
