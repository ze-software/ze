// Design: docs/architecture/core-design.md — BGP server events and hooks
// Overview: event_dispatcher.go — EventDispatcher calls validation functions

package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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

// OpenValidationUnavailableError is returned by broadcastValidateOpen when a plugin that
// holds per-peer OPEN policy for this peer could not be asked about it. It is a third
// state, and the two the engine had before it both read as consent: a nil return says
// every plugin holding policy accepted, and OpenValidationError says one of them looked
// at the OPEN and refused it.
//
// A plugin holds per-peer OPEN policy for a peer when it declared a capability for that
// peer alone rather than for every peer (plugin.InjectedCapability.PeerAddr) AND it
// registered a validate-open callback (PluginRegistration.WantsValidateOpen). The caller
// resolves both conditions in Server.PluginsWithPerPeerOpenPolicy
// (internal/component/plugin/server/server.go). bgp-role is the only plugin meeting them
// today: it declares the RFC 9234 Role capability for exactly the peers that carry role
// configuration (extractRoleCapabilities, internal/component/bgp/plugins/role/config.go),
// so this state is reached for a role-configured peer and for no other peer. bgp-gr and
// bgp-softver declare per-peer capabilities too and validate no OPEN, which is why the
// callback condition is not redundant.
type OpenValidationUnavailableError struct {
	Peer    string   // the peer whose OPEN nothing could validate
	Plugins []string // the plugins that held policy and did not answer, sorted
}

// Error names the peer and the silent plugins, because the operator has to repair the
// plugin rather than the peer's configuration.
func (e *OpenValidationUnavailableError) Error() string {
	var buf textbuf.Buffer
	buf.Reset().Str("open validation unavailable for peer ").Str(e.Peer).Str(": no answer from ")
	for i, name := range e.Plugins {
		if i > 0 {
			buf.Str(", ")
		}
		buf.Str(name)
	}
	return buf.String()
}

// NotifyCodes returns OPEN Message Error (2) with the Role Mismatch subcode (11).
//
// RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker MUST reject the
// connection using the Role Mismatch Notification (code 2, subcode 11)."
//
// A peer holding an unanswered per-peer OPEN policy is a peer whose Roles ze cannot
// confirm correspond, so ze rejects the connection rather than assume they do (owner
// decision, 2026-08-30). bgp-role is the only plugin that validates an OPEN today, and
// runOpenValidator (internal/component/bgp/reactor/session_open_validation.go) already
// sends this pair for a validator error that names no codes.
func (e *OpenValidationUnavailableError) NotifyCodes() (uint8, uint8) {
	return uint8(message.NotifyOpenMessage), message.NotifyOpenRoleMismatch
}

// openMessageToRPC converts a message.Open to rpc.ValidateOpenMessage for plugin IPC.
// Extracts capabilities as raw {code, hex} pairs from the OptionalParams TLV structure.
func openMessageToRPC(open *message.Open) rpc.ValidateOpenMessage {
	asn := uint32(open.MyAS)
	if open.ASN4 > 0 {
		asn = open.ASN4
	}

	// Convert BGP Identifier (uint32) to dotted IP string
	var ridBuf textbuf.Buffer
	routerID := ridBuf.Reset().Uint(uint64((open.BGPIdentifier >> 24) & 0xFF)).Byte('.').Uint(uint64((open.BGPIdentifier >> 16) & 0xFF)).Byte('.').Uint(uint64((open.BGPIdentifier >> 8) & 0xFF)).Byte('.').Uint(uint64(open.BGPIdentifier & 0xFF)).String()

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
//
// group is the peer's enclosing group, empty for a peer that stands alone. A
// peer created from a dynamic group's template shares no other identity with
// the operator's config document. A plugin that keys per-peer policy on that
// document resolves such a peer through the group or not at all
// (rpc.ValidateOpenInput).
//
// policyPlugins names the plugins that declared a capability for THIS peer alone, which
// the reactor resolved from the same declarations it injected into the local OPEN
// (openPolicyPlugins, internal/component/bgp/reactor/peer.go). Each of them owes an
// answer, and one that does not give it refuses the OPEN rather than passing for
// consent: see OpenValidationUnavailableError.
func broadcastValidateOpen(s *pluginserver.Server, peerAddr, group string, policyPlugins []string, local, remote any) error {
	localOpen, ok := local.(*message.Open)
	if !ok {
		return fmt.Errorf("broadcastValidateOpen: local not *message.Open: %T", local)
	}

	remoteOpen, ok := remote.(*message.Open)
	if !ok {
		return fmt.Errorf("broadcastValidateOpen: remote not *message.Open: %T", remote)
	}

	// A plugin is struck off as it answers, so what is left at the end is per-peer
	// policy that nothing evaluated.
	pending := make(map[string]bool, len(policyPlugins))
	for _, name := range policyPlugins {
		pending[name] = true
	}

	// A nil process manager means the plugin phase never ran (Server.runPluginPhase
	// performs the single procManager.Store), so no plugin declared a capability and
	// pending is empty. The unanswered check below still governs this exit: it is the
	// one place that decides what an unasked policy means.
	if pm := s.ProcessManager(); pm != nil {
		input := &rpc.ValidateOpenInput{
			Peer:   peerAddr,
			Group:  group,
			Local:  openMessageToRPC(localOpen),
			Remote: openMessageToRPC(remoteOpen),
		}

		if err := askOpenValidators(s.Context(), pm.AllProcesses(), input, pending); err != nil {
			return err
		}
	}

	return unansweredOpenValidation(peerAddr, pending)
}

// askOpenValidators sends the validate-open RPC to every process that registered the
// callback, and strikes the plugin off pending when it answers. It returns the first
// rejection, and nil when no plugin refused the OPEN.
//
// A plugin that could not be asked stays in pending. Skipping it here is correct: the
// engine cannot make a plugin answer, and the caller owns what an unanswered policy
// means for the session.
func askOpenValidators(ctx context.Context, procs []*process.Process, input *rpc.ValidateOpenInput, pending map[string]bool) error {
	for _, proc := range procs {
		reg := proc.Registration()
		if reg == nil || !reg.WantsValidateOpen {
			continue
		}

		conn := proc.Conn()
		if conn == nil {
			logger().Warn("validate-open not sent: plugin has no connection",
				"plugin", proc.Name(), "peer", input.Peer)
			continue
		}

		callCtx, cancel := context.WithTimeout(ctx, validateOpenTimeout)
		output, err := conn.SendValidateOpen(callCtx, input)
		cancel()

		if err != nil {
			logger().Warn("validate-open RPC failed", "plugin", proc.Name(), "peer", input.Peer, "error", err)
			continue
		}

		delete(pending, proc.Name())

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

// unansweredOpenValidation reports the plugins that held per-peer OPEN policy for this
// peer and did not answer, and nil when every one of them did.
//
// RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker MUST reject the
// connection using the Role Mismatch Notification (code 2, subcode 11)."
//
// Returning nil for an unanswered policy establishes the session with the RFC 9234
// Section 5 route-leak guard silently absent, which is a failure reported as consent
// (ai/rules/principles.md). A peer with no per-peer declaration has no policy to leave
// unanswered, so it keeps the behavior it had before this check existed.
func unansweredOpenValidation(peerAddr string, pending map[string]bool) error {
	if len(pending) == 0 {
		return nil
	}

	return &OpenValidationUnavailableError{
		Peer:    peerAddr,
		Plugins: slices.Sorted(maps.Keys(pending)),
	}
}
