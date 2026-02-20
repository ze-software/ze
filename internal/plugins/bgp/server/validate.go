// Design: docs/architecture/core-design.md — BGP server events and hooks

package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// OpenValidationError is returned by BroadcastValidateOpen when a plugin rejects an OPEN pair.
// Carries NOTIFICATION code/subcode so the engine can send the correct BGP NOTIFICATION
// without knowing the protocol-specific reason for rejection.
//
// Implements the NotifyCodes() interface used by session.go for error dispatch.
type OpenValidationError struct {
	NotifyCode    uint8  // BGP NOTIFICATION error code (e.g., 2 = OPEN Message Error)
	NotifySubcode uint8  // BGP NOTIFICATION error subcode (e.g., 11 = Role Mismatch)
	Reason        string // Human-readable reason for rejection
}

// Error returns a human-readable error message.
func (e *OpenValidationError) Error() string {
	if e.Reason != "" {
		return "open validation rejected: " + e.Reason
	}
	return "open validation rejected"
}

// NotifyCodes returns the NOTIFICATION code and subcode for this validation error.
// Used by session.go via interface assertion: interface{ NotifyCodes() (uint8, uint8) }.
func (e *OpenValidationError) NotifyCodes() (uint8, uint8) {
	return e.NotifyCode, e.NotifySubcode
}

// openMessageToRPC converts a message.Open to rpc.ValidateOpenMessage for plugin IPC.
// Extracts capabilities as raw {code, hex} pairs from the OptionalParams TLV structure.
func openMessageToRPC(open *message.Open) rpc.ValidateOpenMessage {
	asn := uint32(open.MyAS)
	if open.ASN4 > 0 {
		asn = open.ASN4
	}

	// Convert BGP Identifier (uint32) to dotted IP string
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

	// Extract capabilities from OptionalParams TLV structure.
	// RFC 5492: Optional Parameters are type(1) + length(1) + value(variable).
	// Type 2 = Capabilities Optional Parameter, containing capability TLVs.
	msg.Capabilities = extractCapabilitiesFromOptParams(open.OptionalParams)

	return msg
}

// extractCapabilitiesFromOptParams extracts raw capability {code, hex} pairs
// from the BGP OPEN OptionalParams field.
// Does NOT parse capability values — just extracts code and raw value bytes.
func extractCapabilitiesFromOptParams(optParams []byte) []rpc.ValidateOpenCapability {
	if len(optParams) == 0 {
		return nil
	}

	var caps []rpc.ValidateOpenCapability
	offset := 0

	for offset < len(optParams) {
		if offset+2 > len(optParams) {
			break
		}

		paramType := optParams[offset]
		paramLen := int(optParams[offset+1])
		offset += 2

		if offset+paramLen > len(optParams) {
			break
		}

		paramData := optParams[offset : offset+paramLen]
		offset += paramLen

		// Type 2 = Capabilities Optional Parameter (RFC 3392/5492)
		if paramType != 2 {
			continue
		}

		// Parse capability TLVs within this parameter
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

	return caps
}

// validateOpenTimeout is the timeout for a single validate-open RPC call.
const validateOpenTimeout = 5 * time.Second

// broadcastValidateOpen validates OPEN messages via all plugins that declared WantsValidateOpen.
// local and remote are *message.Open passed as any from the generic hook.
// Iterates processes, sends validate-open RPC, fails fast on first rejection.
func broadcastValidateOpen(s *plugin.Server, peerAddr string, local, remote any) error {
	localOpen, ok := local.(*message.Open)
	if !ok {
		return fmt.Errorf("broadcastValidateOpen: local not *message.Open: %T", local)
	}

	remoteOpen, ok := remote.(*message.Open)
	if !ok {
		return fmt.Errorf("broadcastValidateOpen: remote not *message.Open: %T", remote)
	}

	pm := s.ProcessManager()
	if pm == nil {
		return nil
	}

	// Build the RPC input once (shared across all plugin calls)
	input := &rpc.ValidateOpenInput{
		Peer:   peerAddr,
		Local:  openMessageToRPC(localOpen),
		Remote: openMessageToRPC(remoteOpen),
	}

	// Iterate processes, fail-fast on first rejection
	for _, proc := range pm.AllProcesses() {
		reg := proc.Registration()
		if reg == nil || !reg.WantsValidateOpen {
			continue
		}

		connB := proc.ConnB()
		if connB == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(s.Context(), validateOpenTimeout)
		output, err := connB.SendValidateOpen(ctx, input)
		cancel()

		if err != nil {
			logger().Warn("validate-open RPC failed", "plugin", proc.Name(), "peer", peerAddr, "error", err)
			continue // RPC failure = skip this plugin (don't block session)
		}

		if !output.Accept {
			return &OpenValidationError{
				NotifyCode:    output.NotifyCode,
				NotifySubcode: output.NotifySubcode,
				Reason:        output.Reason,
			}
		}
	}

	return nil
}
