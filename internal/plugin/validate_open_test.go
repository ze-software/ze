package plugin

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// testValidationError mirrors server.OpenValidationError for tests that cannot import
// the server package (import cycle). Implements the same NotifyCodes() interface
// used by session.go for error dispatch.
type testValidationError struct {
	NotifyCode    uint8
	NotifySubcode uint8
	Reason        string
}

func (e *testValidationError) Error() string {
	if e.Reason != "" {
		return "open validation rejected: " + e.Reason
	}
	return "open validation rejected"
}

func (e *testValidationError) NotifyCodes() (uint8, uint8) {
	return e.NotifyCode, e.NotifySubcode
}

// testOpenMessageToRPC converts message.Open to rpc.ValidateOpenMessage.
// Mirrors server.openMessageToRPC for test use (avoids import cycle).
func testOpenMessageToRPC(open *message.Open) rpc.ValidateOpenMessage {
	asn := uint32(open.MyAS)
	if open.ASN4 > 0 {
		asn = open.ASN4
	}

	routerID := fmt.Sprintf("%d.%d.%d.%d",
		(open.BGPIdentifier>>24)&0xFF,
		(open.BGPIdentifier>>16)&0xFF,
		(open.BGPIdentifier>>8)&0xFF,
		open.BGPIdentifier&0xFF,
	)

	msg := rpc.ValidateOpenMessage{
		ASN:      asn,
		RouterID: routerID,
		HoldTime: open.HoldTime,
	}

	// Extract capabilities from OptionalParams TLV structure
	if len(open.OptionalParams) > 0 {
		var caps []rpc.ValidateOpenCapability
		offset := 0

		for offset < len(open.OptionalParams) {
			if offset+2 > len(open.OptionalParams) {
				break
			}

			paramType := open.OptionalParams[offset]
			paramLen := int(open.OptionalParams[offset+1])
			offset += 2

			if offset+paramLen > len(open.OptionalParams) {
				break
			}

			paramData := open.OptionalParams[offset : offset+paramLen]
			offset += paramLen

			if paramType != 2 {
				continue
			}

			capOffset := 0
			for capOffset < len(paramData) {
				if capOffset+2 > len(paramData) {
					break
				}

				capCode := paramData[capOffset]
				capLen := int(paramData[capOffset+1])
				capOffset += 2

				if capOffset+capLen > len(paramData) {
					break
				}

				capValue := paramData[capOffset : capOffset+capLen]
				capOffset += capLen

				caps = append(caps, rpc.ValidateOpenCapability{
					Code: capCode,
					Hex:  hex.EncodeToString(capValue),
				})
			}
		}

		msg.Capabilities = caps
	}

	return msg
}

// testBroadcastValidateOpenHooks returns a BGPHooks with BroadcastValidateOpen set
// to a test implementation that mirrors server.broadcastValidateOpen.
// Avoids import cycle (plugin -> bgp/server -> plugin).
func testBroadcastValidateOpenHooks() *BGPHooks {
	return &BGPHooks{
		BroadcastValidateOpen: func(s *Server, peerAddr string, local, remote any) error {
			localOpen, ok := local.(*message.Open)
			if !ok {
				return fmt.Errorf("test: local not *message.Open: %T", local)
			}

			remoteOpen, ok := remote.(*message.Open)
			if !ok {
				return fmt.Errorf("test: remote not *message.Open: %T", remote)
			}

			pm := s.ProcessManager()
			if pm == nil {
				return nil
			}

			input := &rpc.ValidateOpenInput{
				Peer:   peerAddr,
				Local:  testOpenMessageToRPC(localOpen),
				Remote: testOpenMessageToRPC(remoteOpen),
			}

			for _, proc := range pm.AllProcesses() {
				reg := proc.Registration()
				if reg == nil || !reg.WantsValidateOpen {
					continue
				}

				connB := proc.ConnB()
				if connB == nil {
					continue
				}

				ctx, cancel := context.WithTimeout(s.Context(), 5*time.Second)
				output, err := connB.SendValidateOpen(ctx, input)
				cancel()

				if err != nil {
					continue
				}

				if !output.Accept {
					return &testValidationError{
						NotifyCode:    output.NotifyCode,
						NotifySubcode: output.NotifySubcode,
						Reason:        output.Reason,
					}
				}
			}

			return nil
		},
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
		bgpHooks: testBroadcastValidateOpenHooks(),
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
		ctx:      ctx,
		bgpHooks: testBroadcastValidateOpenHooks(),
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
// VALIDATES: Plugin rejection returns error with correct NOTIFICATION codes via NotifyCodes().
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
		ctx:      ctx,
		bgpHooks: testBroadcastValidateOpenHooks(),
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

	// Verify error carries correct NOTIFICATION codes via NotifyCodes() interface
	var valErr interface{ NotifyCodes() (uint8, uint8) }
	require.ErrorAs(t, err, &valErr)
	code, sub := valErr.NotifyCodes()
	assert.Equal(t, uint8(2), code)
	assert.Equal(t, uint8(11), sub)

	// Verify error message contains the reason
	assert.Contains(t, err.Error(), "role mismatch: customer↔customer")
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
