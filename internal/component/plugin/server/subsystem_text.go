// Design: docs/architecture/api/process-protocol.md — text-mode plugin handshake
// Overview: subsystem.go — JSON-mode completeProtocol and SubsystemHandler
// Related: startup_text.go — text-mode handleTextProcessStartup

package server

import (
	"context"
	"fmt"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// textProtocolResult holds parsed data from a text-mode 5-stage handshake.
// Used by subsystem.go to extract commands and schema from the registration.
type textProtocolResult struct {
	reg   rpc.DeclareRegistrationInput
	caps  rpc.DeclareCapabilitiesInput
	ready rpc.ReadyInput
}

// completeTextProtocol runs the 5-stage startup protocol using text framing.
// Socket A (tcA): engine reads plugin stages 1, 3, 5 and sends "ok" responses.
// Socket B (tcB): engine sends stages 2, 4 and reads "ok" responses from plugin.
//
// Returns the parsed data from all stages so the caller can extract
// commands, schema, capabilities, and subscription info.
//
// This mirrors completeProtocol() in subsystem.go but uses TextConn + text
// format/parse functions instead of PluginConn + JSON-RPC.
func completeTextProtocol(ctx context.Context, tcA, tcB *rpc.TextConn) (*textProtocolResult, error) {
	var result textProtocolResult

	// Stage 1: Read text registration from Socket A.
	regText, err := tcA.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("text stage 1 read: %w", err)
	}
	reg, err := rpc.ParseRegistrationText(regText)
	if err != nil {
		if writeErr := tcA.WriteLine(ctx, "error "+err.Error()); writeErr != nil {
			return nil, fmt.Errorf("text stage 1 error response: %w", writeErr)
		}
		return nil, fmt.Errorf("text stage 1 parse: %w", err)
	}
	result.reg = reg
	if err := tcA.WriteLine(ctx, "ok"); err != nil {
		return nil, fmt.Errorf("text stage 1 respond: %w", err)
	}

	// Stage 2: Send text configure on Socket B.
	configText, err := rpc.FormatConfigureText(rpc.ConfigureInput{})
	if err != nil {
		return nil, fmt.Errorf("text stage 2 format: %w", err)
	}
	if err := tcB.WriteMessage(ctx, configText); err != nil {
		return nil, fmt.Errorf("text stage 2 write: %w", err)
	}
	resp, err := tcB.ReadLine(ctx)
	if err != nil {
		return nil, fmt.Errorf("text stage 2 response read: %w", err)
	}
	if resp != "ok" {
		return nil, fmt.Errorf("text stage 2: plugin responded %q", resp)
	}

	// Stage 3: Read text capabilities from Socket A.
	capsText, err := tcA.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("text stage 3 read: %w", err)
	}
	caps, err := rpc.ParseCapabilitiesText(capsText)
	if err != nil {
		if writeErr := tcA.WriteLine(ctx, "error "+err.Error()); writeErr != nil {
			return nil, fmt.Errorf("text stage 3 error response: %w", writeErr)
		}
		return nil, fmt.Errorf("text stage 3 parse: %w", err)
	}
	result.caps = caps
	if err := tcA.WriteLine(ctx, "ok"); err != nil {
		return nil, fmt.Errorf("text stage 3 respond: %w", err)
	}

	// Stage 4: Send text registry on Socket B.
	regText2, err := rpc.FormatRegistryText(rpc.ShareRegistryInput{})
	if err != nil {
		return nil, fmt.Errorf("text stage 4 format: %w", err)
	}
	if err := tcB.WriteMessage(ctx, regText2); err != nil {
		return nil, fmt.Errorf("text stage 4 write: %w", err)
	}
	resp, err = tcB.ReadLine(ctx)
	if err != nil {
		return nil, fmt.Errorf("text stage 4 response read: %w", err)
	}
	if resp != "ok" {
		return nil, fmt.Errorf("text stage 4: plugin responded %q", resp)
	}

	// Stage 5: Read text ready from Socket A.
	readyText, err := tcA.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("text stage 5 read: %w", err)
	}
	ready, err := rpc.ParseReadyText(readyText)
	if err != nil {
		if writeErr := tcA.WriteLine(ctx, "error "+err.Error()); writeErr != nil {
			return nil, fmt.Errorf("text stage 5 error response: %w", writeErr)
		}
		return nil, fmt.Errorf("text stage 5 parse: %w", err)
	}
	result.ready = ready
	if err := tcA.WriteLine(ctx, "ok"); err != nil {
		return nil, fmt.Errorf("text stage 5 respond: %w", err)
	}

	return &result, nil
}
