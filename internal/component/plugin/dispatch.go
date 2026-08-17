// Design: docs/architecture/api/commands.md -- shared command dispatch contract

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

// Authorizer is the request-carried policy decision used by the shared command
// dispatcher. AAA authorizers satisfy it without making this infrastructure
// package depend on the AAA component.
type Authorizer interface {
	Authorize(username, remoteAddr, command string, isReadOnly bool) bool
}

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
	// Authorizer is the policy generation accepted with this identity. Carrying
	// it with the request prevents a reload publication between authentication
	// and dispatch from authorizing the caller against a different generation.
	Authorizer Authorizer
}

type callerAuthorizerContextKey struct{}

// WithCallerAuthorizer carries a session-bound authorizer through handlers that
// construct CallerIdentity at their dispatch edge.
func WithCallerAuthorizer(ctx context.Context, authorizer Authorizer) context.Context {
	if authorizer == nil {
		return ctx
	}
	return context.WithValue(ctx, callerAuthorizerContextKey{}, authorizer)
}

// CallerAuthorizer returns the session-bound authorizer carried by ctx.
func CallerAuthorizer(ctx context.Context) Authorizer {
	if ctx == nil {
		return nil
	}
	authorizer, _ := ctx.Value(callerAuthorizerContextKey{}).(Authorizer)
	return authorizer
}

// CommandDispatcher executes a command on behalf of an authenticated caller and
// returns the typed response. It is the single dispatcher every surface
// consumes; the hub supplies one built from the plugin server dispatcher.
//
// Returning *Response (not a flattened string) lets the API surface carry typed
// Data to its transport edge unchanged, and lets text surfaces render at their
// own edge via CommandDispatcher.JSON.
type CommandDispatcher func(ctx context.Context, caller CallerIdentity, command string) (*Response, error)

// RenderedResponse carries flattened text and the accepted action that belongs
// to the transport writing that text.
type RenderedResponse struct {
	Output   string
	Response *Response
}

// TransportComplete releases the accepted action after Output reaches the
// caller. Repeated calls are harmless.
func (r *RenderedResponse) TransportComplete() {
	if r != nil && r.Response != nil {
		r.Response.TransportComplete()
	}
}

// JSON dispatches a command and flattens the typed response while retaining
// transport completion ownership. The caller must call TransportComplete only
// after it writes Output to its response transport.
func (d CommandDispatcher) JSON(ctx context.Context, caller CallerIdentity, command string) (*RenderedResponse, error) {
	resp, err := d(ctx, caller, command)
	output, renderErr := ResponseJSON(resp, err)
	return &RenderedResponse{Output: output, Response: resp}, renderErr
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
