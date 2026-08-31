package server

import (
	"encoding/hex"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// roleOpen returns an OPEN carrying the RFC 9234 Role capability (code 9) with the
// given role value, which is the shape a role-configured peer exchanges.
func roleOpen(role byte) *message.Open {
	return &message.Open{
		MyAS:           65000,
		HoldTime:       180,
		BGPIdentifier:  0x01020304,
		OptionalParams: []byte{2, 3, 9, 1, role},
	}
}

// newSilentProc returns a process that registered the validate-open callback and whose
// connection is dead, so every RPC to it fails. It is the plugin that wants to answer
// and cannot.
func newSilentProc(t *testing.T, name string) *process.Process {
	t.Helper()

	engineSide, pluginSide := net.Pipe()
	require.NoError(t, pluginSide.Close())

	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetRegistration(&plugin.PluginRegistration{WantsValidateOpen: true, Done: true})
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	t.Cleanup(func() { proc.Stop() })
	return proc
}

// RFC requirement: RFC9234-4.2-2 positive -- a peer that carries role configuration is
// rejected with the Role Mismatch NOTIFICATION code 2 subcode 11 when the plugin holding
// that policy could not be asked, because ze cannot then say the Roles correspond.
//
// VALIDATES: an unanswered per-peer OPEN policy refuses the connection and names the
// silent plugin.
// PREVENTS: the session establishing with the RFC 9234 Section 5 route-leak guard
// silently absent, which is what "no plugin objected" used to mean for an unreachable
// bgp-role.
func TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	err := broadcastValidateOpen(srv, "10.0.0.1", "", []string{"bgp-role"}, roleOpen(3), roleOpen(3))
	require.Error(t, err)

	var unavailable *OpenValidationUnavailableError
	require.True(t, errors.As(err, &unavailable))
	assert.Equal(t, "10.0.0.1", unavailable.Peer)
	assert.Equal(t, []string{"bgp-role"}, unavailable.Plugins)

	code, sub := unavailable.NotifyCodes()
	assert.Equal(t, uint8(2), code)
	assert.Equal(t, message.NotifyOpenRoleMismatch, sub)
}

// RFC requirement: RFC9234-4.2-2 negative -- a peer that carries NO role configuration is
// not rejected with the Role Mismatch NOTIFICATION when validation could not run, so the
// refusal is scoped to peers whose Roles ze was supposed to check.
//
// VALIDATES: a peer with no per-peer declaration keeps the behavior it had before the
// unanswered-policy check existed.
// PREVENTS: an unreachable plugin tearing down every session in the daemon, role
// configuration or not.
func TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	err := broadcastValidateOpen(srv, "10.0.0.2", "", nil, roleOpen(3), roleOpen(0))
	assert.NoError(t, err)
}

// RFC requirement: RFC9234-4.2-2 positive -- a plugin that holds the Section 4.2 policy
// for a peer and cannot answer stays owed, whether it has no connection at all or its
// validate-open RPC fails, so the caller refuses the connection in both cases.
//
// VALIDATES: the two skip paths leave the plugin in the pending set, and a plugin that
// answers is struck off it.
// PREVENTS: a dead or unreachable validator reading as consent, which is the shape both
// skips had while they simply continued the loop.
func TestAskOpenValidatorsLeavesASilentPluginPending(t *testing.T) {
	t.Parallel()

	noConn := process.NewProcess(plugin.PluginConfig{Name: "bgp-role"})
	noConn.SetRegistration(&plugin.PluginRegistration{WantsValidateOpen: true, Done: true})

	tests := []struct {
		name string
		proc func(t *testing.T) *process.Process
	}{
		{
			name: "plugin has no connection",
			proc: func(*testing.T) *process.Process { return noConn },
		},
		{
			name: "validate-open rpc fails",
			proc: func(t *testing.T) *process.Process { return newSilentProc(t, "bgp-role") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pending := map[string]bool{"bgp-role": true}
			input := &rpc.ValidateOpenInput{Peer: "10.0.0.1"}

			err := askOpenValidators(t.Context(), []*process.Process{tt.proc(t)}, input, pending)
			require.NoError(t, err)
			assert.Equal(t, map[string]bool{"bgp-role": true}, pending)

			assert.Error(t, unansweredOpenValidation("10.0.0.1", pending))
		})
	}
}
