// Design: docs/architecture/api/process-protocol.md — plugin process management
// Related: socketpair.go — package marker for plugin IPC
// Related: tls.go — TLS transport for external plugins

package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// PluginConn provides typed RPC communication over a plugin connection.
// It embeds *rpc.Conn for low-level newline-framed JSON RPC and adds typed methods
// for each YANG RPC in the plugin protocol.
//
// PluginConn supports two wiring modes:
//   - Direct: NewPluginConn(conn, conn) -- read and write on the same connection.
//   - Muxed: NewMuxPluginConn(mux) -- all traffic via MuxConn (production path).
//     ReadRequest reads from MuxConn.Requests(), CallRPC/SendResult delegate to MuxConn.
type PluginConn struct {
	*rpc.Conn
	mux *rpc.MuxConn // Non-nil for single-conn mode.
}

// NewPluginConn creates a PluginConn that reads from readConn and writes to writeConn.
// For per-socket wiring (matching SDK pattern), pass the same conn for both arguments.
// For cross-socket wiring (test scenarios), pass different conns.
func NewPluginConn(readConn, writeConn net.Conn) *PluginConn {
	return &PluginConn{Conn: rpc.NewConn(readConn, writeConn)}
}

// NewMuxPluginConn creates a PluginConn backed by a MuxConn for single-connection mode.
// ReadRequest reads from MuxConn.Requests(), all outbound calls go through MuxConn.CallRPC().
func NewMuxPluginConn(mux *rpc.MuxConn) *PluginConn {
	return &PluginConn{mux: mux}
}

// ReadRequest reads the next plugin request. In muxed mode, reads from
// MuxConn.Requests() channel. In direct mode, reads from the underlying connection.
func (pc *PluginConn) ReadRequest(ctx context.Context) (*rpc.Request, error) {
	if pc.mux == nil {
		return pc.Conn.ReadRequest(ctx)
	}
	select {
	case req, ok := <-pc.mux.Requests():
		if !ok {
			return nil, rpc.ErrMuxConnClosed
		}
		return req, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-pc.mux.Done():
		return nil, rpc.ErrMuxConnClosed
	}
}

// CallRPC sends an RPC and waits for the response. In single-conn mode,
// routes through MuxConn. All typed methods (SendDeliverEvent, etc.) call this.
func (pc *PluginConn) CallRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if pc.mux != nil {
		return pc.mux.CallRPC(ctx, method, params)
	}
	return pc.Conn.CallRPC(ctx, method, params)
}

// CallBatchRPC sends a batch delivery frame. In single-conn mode, converts
// [][]byte events to []json.RawMessage to preserve raw JSON embedding.
func (pc *PluginConn) CallBatchRPC(ctx context.Context, events [][]byte) (json.RawMessage, error) {
	if pc.mux != nil {
		// Convert [][]byte to []json.RawMessage so json.Marshal embeds
		// them as raw JSON values instead of base64-encoding byte slices.
		rawEvents := make([]json.RawMessage, len(events))
		for i, e := range events {
			rawEvents[i] = json.RawMessage(e)
		}
		return pc.mux.CallRPC(ctx, "ze-plugin-callback:deliver-batch", map[string]any{"events": rawEvents})
	}
	return pc.Conn.CallBatchRPC(ctx, events)
}

// SendResult sends a successful RPC response.
func (pc *PluginConn) SendResult(ctx context.Context, id uint64, data any) error {
	if pc.mux != nil {
		return pc.mux.SendResult(ctx, id, data)
	}
	return pc.Conn.SendResult(ctx, id, data)
}

// SendOK sends an empty successful RPC response.
func (pc *PluginConn) SendOK(ctx context.Context, id uint64) error {
	if pc.mux != nil {
		return pc.mux.SendOK(ctx, id)
	}
	return pc.Conn.SendOK(ctx, id)
}

// SendError sends an error RPC response.
func (pc *PluginConn) SendError(ctx context.Context, id uint64, message string) error {
	if pc.mux != nil {
		return pc.mux.SendError(ctx, id, message)
	}
	return pc.Conn.SendError(ctx, id, message)
}

// SendCodedError sends an error RPC response with a specific error code.
func (pc *PluginConn) SendCodedError(ctx context.Context, id uint64, code, message string) error {
	if pc.mux != nil {
		// MuxConn doesn't have SendCodedError, delegate to underlying conn.
		return pc.mux.SendError(ctx, id, code+": "+message)
	}
	return pc.Conn.SendCodedError(ctx, id, code, message)
}

// Close closes the underlying connection.
func (pc *PluginConn) Close() error {
	if pc.mux != nil {
		return pc.mux.Close()
	}
	return pc.Conn.Close()
}

// --- Stage RPCs ---

// SendDeclareRegistration sends Stage 1: declare-registration to the engine.
func (pc *PluginConn) SendDeclareRegistration(ctx context.Context, input *rpc.DeclareRegistrationInput) error {
	_, err := pc.CallRPC(ctx, "ze-plugin-engine:declare-registration", input)
	return err
}

// SendConfigure sends Stage 2: configure to the plugin.
func (pc *PluginConn) SendConfigure(ctx context.Context, sections []rpc.ConfigSection) error {
	input := &rpc.ConfigureInput{Sections: sections}
	_, err := pc.CallRPC(ctx, "ze-plugin-callback:configure", input)
	return err
}

// SendDeclareCapabilities sends Stage 3: declare-capabilities to the engine.
func (pc *PluginConn) SendDeclareCapabilities(ctx context.Context, input *rpc.DeclareCapabilitiesInput) error {
	_, err := pc.CallRPC(ctx, "ze-plugin-engine:declare-capabilities", input)
	return err
}

// SendShareRegistry sends Stage 4: share-registry to the plugin.
func (pc *PluginConn) SendShareRegistry(ctx context.Context, commands []rpc.RegistryCommand) error {
	input := &rpc.ShareRegistryInput{Commands: commands}
	_, err := pc.CallRPC(ctx, "ze-plugin-callback:share-registry", input)
	return err
}

// SendReady sends Stage 5: ready to the engine.
func (pc *PluginConn) SendReady(ctx context.Context) error {
	_, err := pc.CallRPC(ctx, "ze-plugin-engine:ready", nil)
	return err
}

// --- Runtime RPCs ---

// SendDeliverEvent sends a BGP event to the plugin via callback.
func (pc *PluginConn) SendDeliverEvent(ctx context.Context, eventJSON string) error {
	input := &rpc.DeliverEventInput{Event: eventJSON}
	_, err := pc.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	return err
}

// SendDeliverBatch sends multiple BGP events to the plugin in a single batch.
// Uses a pooled buffer to construct the JSON-RPC frame directly, bypassing
// json.Marshal and FrameWriter.Write allocations. One write + one ack per batch.
func (pc *PluginConn) SendDeliverBatch(ctx context.Context, events []string) error {
	rawEvents := make([][]byte, len(events))
	for i, e := range events {
		b, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event %d: %w", i, err)
		}
		rawEvents[i] = b
	}
	_, err := pc.CallBatchRPC(ctx, rawEvents)
	return err
}

// SendEncodeNLRI requests NLRI encoding from the plugin. Returns hex result.
func (pc *PluginConn) SendEncodeNLRI(ctx context.Context, family string, args []string) (string, error) {
	input := &rpc.EncodeNLRIInput{Family: family, Args: args}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:encode-nlri", input)
	if err != nil {
		return "", err
	}
	var decoded struct {
		Hex string `json:"hex"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return "", fmt.Errorf("unmarshal encode-nlri result: %w", err)
	}
	return decoded.Hex, nil
}

// SendDecodeNLRI requests NLRI decoding from the plugin. Returns JSON result.
func (pc *PluginConn) SendDecodeNLRI(ctx context.Context, family, hex string) (string, error) {
	input := &rpc.DecodeNLRIInput{Family: family, Hex: hex}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:decode-nlri", input)
	if err != nil {
		return "", err
	}
	var decoded struct {
		JSON string `json:"json"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return "", fmt.Errorf("unmarshal decode-nlri result: %w", err)
	}
	return decoded.JSON, nil
}

// SendDecodeCapability requests capability decoding from the plugin. Returns JSON result.
func (pc *PluginConn) SendDecodeCapability(ctx context.Context, code uint8, hex string) (string, error) {
	input := &rpc.DecodeCapabilityInput{Code: code, Hex: hex}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:decode-capability", input)
	if err != nil {
		return "", err
	}
	var decoded struct {
		JSON string `json:"json"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return "", fmt.Errorf("unmarshal decode-capability result: %w", err)
	}
	return decoded.JSON, nil
}

// SendExecuteCommand requests command execution from the plugin.
func (pc *PluginConn) SendExecuteCommand(ctx context.Context, serial, command string, args []string, peer string) (*rpc.ExecuteCommandOutput, error) {
	input := &rpc.ExecuteCommandInput{Serial: serial, Command: command, Args: args, Peer: peer}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:execute-command", input)
	if err != nil {
		return nil, err
	}
	var out rpc.ExecuteCommandOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal execute-command result: %w", err)
	}
	return &out, nil
}

// SendConfigVerify sends a config verification request to the plugin.
// Returns the plugin's validation result (status + optional error).
func (pc *PluginConn) SendConfigVerify(ctx context.Context, sections []rpc.ConfigSection) (*rpc.ConfigVerifyOutput, error) {
	input := &rpc.ConfigVerifyInput{Sections: sections}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:config-verify", input)
	if err != nil {
		return nil, err
	}
	var out rpc.ConfigVerifyOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal config-verify result: %w", err)
	}
	return &out, nil
}

// SendConfigApply sends a config apply request to the plugin.
// Returns the plugin's apply result (status + optional error).
func (pc *PluginConn) SendConfigApply(ctx context.Context, sections []rpc.ConfigDiffSection) (*rpc.ConfigApplyOutput, error) {
	input := &rpc.ConfigApplyInput{Sections: sections}
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:config-apply", input)
	if err != nil {
		return nil, err
	}
	var out rpc.ConfigApplyOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal config-apply result: %w", err)
	}
	return &out, nil
}

// SendValidateOpen sends a validate-open request to the plugin.
// Returns the plugin's validation result (accept/reject with optional NOTIFICATION codes).
func (pc *PluginConn) SendValidateOpen(ctx context.Context, input *rpc.ValidateOpenInput) (*rpc.ValidateOpenOutput, error) {
	result, err := pc.CallRPC(ctx, "ze-plugin-callback:validate-open", input)
	if err != nil {
		return nil, err
	}
	var out rpc.ValidateOpenOutput
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, fmt.Errorf("unmarshal validate-open result: %w", err)
	}
	return &out, nil
}

// SendBye sends a shutdown request to the plugin.
func (pc *PluginConn) SendBye(ctx context.Context, reason string) error {
	input := &rpc.ByeInput{Reason: reason}
	_, err := pc.CallRPC(ctx, "ze-plugin-callback:bye", input)
	return err
}
