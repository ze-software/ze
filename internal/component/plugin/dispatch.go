// Unified command-result envelope and dispatcher.
//
// This file holds the single, shared command surface every user-facing entry
// point (web, mcp, looking-glass, REST/gRPC, chaos, SSH) consumes: the
// CommandDispatcher func type, the CallerIdentity value it carries, and the one
// flatten helper that projects a typed *Response into the JSON string the text
// surfaces render. Before unification these were declared five times across
// component packages with incompatible shapes; they live here now because the
// plugin package is shared infrastructure that every surface may import, while
// internal/core (the bottom tier) may not hold a type that returns *Response.

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// CallerIdentity carries trusted caller metadata for a command request.
// Populated by the transport after authentication; it is not an auth
// credential. Surface names the originating transport for audit attribution;
// when empty the dispatcher constructor supplies a fixed per-surface default.
type CallerIdentity struct {
	Username   string
	RemoteAddr string
	Surface    string
	// ReadOnly means the transport admitted the caller with read-only access
	// only. Used by the API engine to deny writes from no-auth/read-only callers.
	ReadOnly bool
}

// CommandDispatcher executes a command on behalf of an authenticated caller and
// returns the typed response. It is the single dispatcher every surface
// consumes; the hub supplies one built from the plugin server dispatcher.
//
// Returning *Response (not a flattened string) lets the API surface carry typed
// Data to its transport edge unchanged, and lets text surfaces render at their
// own edge via CommandDispatcher.JSON.
type CommandDispatcher func(ctx context.Context, caller CallerIdentity, command string) (*Response, error)

// JSON dispatches command and flattens the typed response into the JSON string
// text surfaces render. It preserves, byte for byte, what the historical hub
// adapters produced:
//   - a dispatch error, or a Response with a non-empty Error, surfaces as a Go
//     error (empty string output);
//   - a Response with Status=="error" and no Error message surfaces as the
//     "unknown error" Go error;
//   - a nil Response, or one with nil Data, yields an empty string and no error;
//   - otherwise Data is JSON-marshaled and returned as the output string.
//
// Callers MUST guard against a nil dispatcher before calling JSON (invoking a
// nil func value panics); every surface already nil-checks its injected
// dispatcher before use.
func (d CommandDispatcher) JSON(ctx context.Context, caller CallerIdentity, command string) (string, error) {
	resp, err := d(ctx, caller, command)
	return ResponseJSON(resp, err)
}

// ResponseJSON is the single flatten sequence shared by every text surface and
// by CommandDispatcher.JSON. It is the one place the (error / nil / Status /
// Data) projection lives after unification. See JSON for the exact semantics.
func ResponseJSON(resp *Response, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}
	if resp.Status == StatusError {
		return "", errStatusErrorNoMessage
	}
	if resp.Data == nil {
		return "", nil
	}
	// Pre-rendered text renders verbatim; marshaling it would re-quote and
	// escape it (breaking human-readable output such as the web BGP decoder).
	if t, ok := resp.Data.(Text); ok {
		return string(t), nil
	}
	b, jsonErr := json.Marshal(resp.Data)
	if jsonErr != nil {
		return "", fmt.Errorf("marshal response: %w", jsonErr)
	}
	return string(b), nil
}

// errStatusErrorNoMessage matches the historical "unknown error" text the hub
// adapters returned for a Status=error response that carried no Error message.
var errStatusErrorNoMessage = errors.New("unknown error")
