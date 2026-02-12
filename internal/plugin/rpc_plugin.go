package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// PluginConn provides typed RPC communication over a dual-socket connection.
// It embeds *rpc.Conn for low-level NUL-framed JSON RPC and adds typed methods
// for each YANG RPC in the plugin protocol.
//
// PluginConn supports two wiring modes via the embedded rpc.Conn:
//   - Per-socket: NewPluginConn(conn, conn) — read and write on the same socket.
//     Used by the engine for internal plugins.
//   - Cross-socket: NewPluginConn(readConn, writeConn) — read from one socket,
//     write to another. Used in tests to simulate the two-socket architecture.
type PluginConn struct {
	*rpc.Conn
}

// NewPluginConn creates a PluginConn that reads from readConn and writes to writeConn.
// For per-socket wiring (matching SDK pattern), pass the same conn for both arguments.
// For cross-socket wiring (test scenarios), pass different conns.
func NewPluginConn(readConn, writeConn net.Conn) *PluginConn {
	return &PluginConn{Conn: rpc.NewConn(readConn, writeConn)}
}

// --- Stage RPCs ---

// SendDeclareRegistration sends Stage 1: declare-registration to the engine.
func (pc *PluginConn) SendDeclareRegistration(ctx context.Context, input *rpc.DeclareRegistrationInput) error {
	raw, err := pc.CallRPC(ctx, "ze-plugin-engine:declare-registration", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// SendConfigure sends Stage 2: configure to the plugin.
func (pc *PluginConn) SendConfigure(ctx context.Context, sections []rpc.ConfigSection) error {
	input := &rpc.ConfigureInput{Sections: sections}
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:configure", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// SendDeclareCapabilities sends Stage 3: declare-capabilities to the engine.
func (pc *PluginConn) SendDeclareCapabilities(ctx context.Context, input *rpc.DeclareCapabilitiesInput) error {
	raw, err := pc.CallRPC(ctx, "ze-plugin-engine:declare-capabilities", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// SendShareRegistry sends Stage 4: share-registry to the plugin.
func (pc *PluginConn) SendShareRegistry(ctx context.Context, commands []rpc.RegistryCommand) error {
	input := &rpc.ShareRegistryInput{Commands: commands}
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:share-registry", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// SendReady sends Stage 5: ready to the engine.
func (pc *PluginConn) SendReady(ctx context.Context) error {
	raw, err := pc.CallRPC(ctx, "ze-plugin-engine:ready", nil)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// --- Runtime RPCs ---

// SendDeliverEvent sends a BGP event to the plugin via callback.
func (pc *PluginConn) SendDeliverEvent(ctx context.Context, eventJSON string) error {
	input := &rpc.DeliverEventInput{Event: eventJSON}
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:deliver-event", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}

// SendEncodeNLRI requests NLRI encoding from the plugin. Returns hex result.
func (pc *PluginConn) SendEncodeNLRI(ctx context.Context, family string, args []string) (string, error) {
	input := &rpc.EncodeNLRIInput{Family: family, Args: args}
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:encode-nlri", input)
	if err != nil {
		return "", err
	}
	result, err := rpc.ParseResponse(raw)
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
func (pc *PluginConn) SendDecodeNLRI(ctx context.Context, family string, hex string) (string, error) {
	input := &rpc.DecodeNLRIInput{Family: family, Hex: hex}
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:decode-nlri", input)
	if err != nil {
		return "", err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:decode-capability", input)
	if err != nil {
		return "", err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:execute-command", input)
	if err != nil {
		return nil, err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:config-verify", input)
	if err != nil {
		return nil, err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:config-apply", input)
	if err != nil {
		return nil, err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:validate-open", input)
	if err != nil {
		return nil, err
	}
	result, err := rpc.ParseResponse(raw)
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
	raw, err := pc.CallRPC(ctx, "ze-plugin-callback:bye", input)
	if err != nil {
		return err
	}
	return rpc.CheckResponse(raw)
}
