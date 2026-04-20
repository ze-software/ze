// Design: docs/architecture/mcp/overview.md -- MCP server-initiated elicitation
// Related: session.go -- session state and correlation map for pending elicitations
// Related: streamable.go -- per-POST reply sinks and the POST->SSE upgrade mechanism

// MCP 2025-06-18 elicitation/create support.
//
// Tool handlers call session.Elicit(ctx, message, schema) mid-dispatch. The
// server validates the schema against the flat-primitive subset the spec
// permits, serializes an elicitation/create JSON-RPC request, and writes it
// to the per-POST reply sink (upgrading the POST response from
// application/json to text/event-stream on the first Elicit call). The
// handler blocks on a per-elicit channel until the client POSTs a JSON-RPC
// response correlated by id.
//
// Reference: https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrElicit* are the typed sentinels session.Elicit callers use to
// distinguish expected outcomes from infrastructure failures.
var (
	// ErrElicitUnsupported — the client did not declare the elicitation
	// capability at initialize, so the server must not emit
	// elicitation/create frames. Handlers receive this when they call
	// Elicit on a session whose client-side capability is not set.
	ErrElicitUnsupported = errors.New("mcp: elicitation: client did not declare capability")

	// ErrElicitDeclined — the client user explicitly declined the request.
	// The content field is empty per spec.
	ErrElicitDeclined = errors.New("mcp: elicitation: declined")

	// ErrElicitCanceled — the client dismissed without an explicit choice
	// (e.g. closed the dialog). Distinct from ctx cancel. Spelling matches
	// context.Canceled (US stdlib convention).
	ErrElicitCanceled = errors.New("mcp: elicitation: canceled")

	// ErrElicitSchemaInvalid — the caller's requestedSchema violates the
	// flat-primitive subset. Wrapped with the offending path.
	ErrElicitSchemaInvalid = errors.New("mcp: elicitation: schema invalid")

	// ErrElicitMalformed — the client response body was syntactically
	// valid JSON-RPC but the result was not parseable as an elicit response
	// (missing action, unknown action value, content not an object).
	ErrElicitMalformed = errors.New("mcp: elicitation: malformed client response")

	// ErrElicitTooMany — the session already has the maximum number of
	// pending elicitations; the call is rejected without sending a frame.
	ErrElicitTooMany = errors.New("mcp: elicitation: too many pending")
)

// elicitTypeString / number / integer / boolean name the JSON Schema
// primitive type values the MCP 2025-06-18 elicitation subset accepts.
// Constants (not string literals scattered in code) so the goconst
// linter stays happy and renames surface at compile time.
const (
	elicitTypeString  = "string"
	elicitTypeNumber  = "number"
	elicitTypeInteger = "integer"
	elicitTypeBoolean = "boolean"
)

// Supported primitive schema types per MCP 2025-06-18 elicitation. Enum
// values are declared with type=elicitTypeString plus an "enum" array;
// the validator accepts that shape as a special case.
var elicitPrimitiveTypes = map[string]struct{}{
	elicitTypeString:  {},
	elicitTypeNumber:  {},
	elicitTypeInteger: {},
	elicitTypeBoolean: {},
}

// Supported string formats per the spec. Empty format is allowed.
var elicitStringFormats = map[string]struct{}{
	"email":     {},
	"uri":       {},
	"date":      {},
	"date-time": {},
}

// Forbidden JSON-Schema composition keywords. Presence of any of these on
// a property (at the top level OR nested) makes the schema invalid.
var elicitForbiddenKeywords = []string{
	"oneOf", "allOf", "anyOf", "$ref", "not",
}

// validateElicitSchema enforces the MCP 2025-06-18 elicitation schema
// contract: flat object root, primitive properties only (string with
// optional format, number, integer, boolean, or enum). Nested objects,
// arrays, and JSON-Schema composition keywords are rejected with a
// wrapped ErrElicitSchemaInvalid that names the offending path.
//
// The caller passes the map form (after json.Unmarshal into map[string]any);
// this matches Ze's MCP convention of decoding external-spec bodies into
// generic maps to keep check-json-kebab.sh happy.
func validateElicitSchema(schema map[string]any) error {
	if schema == nil {
		return wrapSchemaErr("", "schema is nil")
	}
	if t, _ := schema["type"].(string); t != "object" {
		return wrapSchemaErr("", "root type must be \"object\", got "+describeType(schema["type"]))
	}
	for _, kw := range elicitForbiddenKeywords {
		if _, present := schema[kw]; present {
			return wrapSchemaErr("", "forbidden keyword "+kw+" at root")
		}
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return wrapSchemaErr("", "properties must be a non-empty object")
	}
	for name, raw := range props {
		prop, ok := raw.(map[string]any)
		if !ok {
			return wrapSchemaErr(name, "property must be an object")
		}
		if err := validateElicitProperty(name, prop); err != nil {
			return err
		}
	}
	return nil
}

// validateElicitProperty runs the per-property checks. path is the property
// name (used in error messages to help callers locate the offending spot).
func validateElicitProperty(path string, prop map[string]any) error {
	for _, kw := range elicitForbiddenKeywords {
		if _, present := prop[kw]; present {
			return wrapSchemaErr(path, "forbidden keyword "+kw)
		}
	}
	typ, _ := prop["type"].(string)
	if typ == "" {
		return wrapSchemaErr(path, "missing type")
	}
	if _, ok := elicitPrimitiveTypes[typ]; !ok {
		return wrapSchemaErr(path, "type "+typ+" not supported (need string/number/integer/boolean)")
	}
	// Strings with format: format must be in the allowlist.
	if typ == elicitTypeString {
		if f, has := prop["format"]; has {
			fs, ok := f.(string)
			if !ok {
				return wrapSchemaErr(path, "format must be string")
			}
			if _, allowed := elicitStringFormats[fs]; !allowed {
				return wrapSchemaErr(path, "format "+fs+" not supported")
			}
		}
	}
	// Enum (string + enum array) is accepted; no extra validation needed
	// beyond the primitive-type check above. enumNames is optional.
	return nil
}

// wrapSchemaErr returns an error wrapping ErrElicitSchemaInvalid with a
// human-readable path and cause. The wrapping lets callers use errors.Is
// to detect the sentinel while still rendering the path in log/error output.
func wrapSchemaErr(path, reason string) error {
	if path == "" {
		return &elicitSchemaError{path: "<root>", reason: reason}
	}
	return &elicitSchemaError{path: path, reason: reason}
}

// elicitSchemaError carries the schema-validation path and reason. Its
// Unwrap returns ErrElicitSchemaInvalid so errors.Is matches.
type elicitSchemaError struct {
	path   string
	reason string
}

func (e *elicitSchemaError) Error() string {
	return "mcp: elicitation: schema invalid at " + e.path + ": " + e.reason
}

func (e *elicitSchemaError) Unwrap() error { return ErrElicitSchemaInvalid }

// describeType renders a schema type field for error messages without
// leaking internal types.
func describeType(v any) string {
	if v == nil {
		return "missing"
	}
	if s, ok := v.(string); ok {
		return "\"" + s + "\""
	}
	return "non-string"
}

// Response action enum values. Matches the MCP spec exactly: any other
// string coming back from a client is treated as malformed.
const (
	elicitActionAccept  = "accept"
	elicitActionDecline = "decline"
	elicitActionCancel  = "cancel"
)

// Elicit sends an MCP elicitation/create request to the client and blocks
// until the client POSTs a JSON-RPC response with a matching id. Returns
// the parsed content (on accept) or a typed sentinel (ErrElicitDeclined /
// ErrElicitCanceled / ErrElicitMalformed). Fails fast with
// ErrElicitUnsupported when the client did not declare the capability and
// with ErrElicitSchemaInvalid when the schema violates the flat-primitive
// subset.
//
// Lifecycle: must be called from within a POST handler goroutine; relies on
// SetActivePostSink having bound a replySink for the current POST. On first
// Elicit within a POST, the sink is upgraded from jsonReplySink to
// sseReplySink so the elicitation/create request rides the same HTTP
// response as the terminal tool result.
//
// Reference: https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation
func (s *session) Elicit(ctx context.Context, message string, schema map[string]any) (map[string]any, error) {
	if !s.ClientSupportsElicit() {
		return nil, ErrElicitUnsupported
	}
	if err := validateElicitSchema(schema); err != nil {
		return nil, err
	}
	id, ch, err := s.RegisterElicit()
	if err != nil {
		return nil, err
	}
	// Ensure cleanup on every early-exit path; the successful-wait path
	// flips resolved=true so CancelElicit becomes a no-op on the already-
	// removed entry.
	resolved := false
	defer func() {
		if !resolved {
			s.CancelElicit(id)
		}
	}()

	frame, err := buildElicitFrame(id, message, schema)
	if err != nil {
		return nil, err
	}
	if err := s.UpgradeCurrentSinkToSSE(); err != nil {
		return nil, err
	}
	sink := s.CurrentPostSink()
	if sink == nil {
		return nil, errors.New("mcp: elicitation: no active POST sink (Elicit called outside a handler?)")
	}
	if err := sink.WriteFrame(frame); err != nil {
		return nil, fmt.Errorf("mcp: elicitation: write frame: %w", err)
	}

	select {
	case resp := <-ch:
		resolved = true
		return resolveElicitAction(resp)
	case <-ctx.Done():
		return nil, fmt.Errorf("mcp: elicitation: %w", ctx.Err())
	}
}

// resolveElicitAction maps a three-action response to a (content, error)
// pair. Separate function so the switch is testable in isolation.
func resolveElicitAction(resp elicitResponse) (map[string]any, error) {
	switch resp.Action {
	case elicitActionAccept:
		return resp.Content, nil
	case elicitActionDecline:
		return nil, ErrElicitDeclined
	case elicitActionCancel:
		return nil, ErrElicitCanceled
	default:
		return nil, fmt.Errorf("%w: action=%q", ErrElicitMalformed, resp.Action)
	}
}

// buildElicitFrame renders the JSON-RPC request for a server-initiated
// elicitation/create message. MCP uses camelCase externally; this project
// decodes via generic maps to keep check-json-kebab.sh happy, and marshals
// the same way for symmetry.
func buildElicitFrame(id, message string, schema map[string]any) ([]byte, error) {
	frame := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "elicitation/create",
		"params": map[string]any{
			"message":         message,
			"requestedSchema": schema,
		},
	}
	return json.Marshal(frame)
}
