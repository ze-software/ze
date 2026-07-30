// Design: docs/architecture/mcp/overview.md -- MCP form-mode elicitation
// Related: mrtr.go -- the InputRequiredResult that carries the emitted ElicitRequest
// Related: tools.go -- ze_execute, the one handler that asks a client for input

// MCP 2026-07-28 form-mode elicitation.
//
// The server never sends an `elicitation/create` JSON-RPC *request*. Under
// Multi Round-Trip Requests the ElicitRequest is a VALUE inside the
// `inputRequests` map of an InputRequiredResult. The client answers it when it
// retries the original request with `inputResponses`.
//
// The requested-schema validator below is the flat-primitive subset Ze has
// always emitted, recovered unchanged. It is deliberately NARROWER than
// 2026-07-28 permits (which also allows `oneOf`-titled enums and array
// multi-select enums). A server that uses fewer optional schema forms than the
// specification offers is conformant, and Ze's one elicitation asks for a
// single string. Per-function comments name 2025-06-18 because that is the
// revision the subset was written from. This revision does not change it.
//
// Reference: https://modelcontextprotocol.io/specification/2026-07-28/client/elicitation

package mcp

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrElicit* are the typed sentinels that distinguish expected client answers
// from infrastructure failures.
var (
	// ErrElicitDeclined -- the user explicitly declined the request. The
	// content field is empty per spec. Terminal: the server does not ask again,
	// or a user who has said no would be looped.
	ErrElicitDeclined = errors.New("mcp: elicitation: declined")

	// ErrElicitCanceled -- the user dismissed without an explicit choice (e.g.
	// closed the dialog). Distinct from ctx cancel. Spelling matches
	// context.Canceled (US stdlib convention). Terminal, like decline.
	ErrElicitCanceled = errors.New("mcp: elicitation: canceled")

	// ErrElicitSchemaInvalid -- the caller's requestedSchema violates the
	// flat-primitive subset. Wrapped with the offending path.
	ErrElicitSchemaInvalid = errors.New("mcp: elicitation: schema invalid")

	// ErrElicitMalformed -- an inputResponses entry was present but not a
	// parseable elicit response (missing action, unknown action value, entry
	// not an object).
	ErrElicitMalformed = errors.New("mcp: elicitation: malformed client response")
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
// generic maps to keep the kebab-case JSON check happy.
func validateElicitSchema(schema map[string]any) error {
	if schema == nil {
		return wrapSchemaErr("", "schema is nil")
	}
	if t, _ := schema["type"].(string); t != "object" {
		var tb textbuf.Buffer
		return wrapSchemaErr("", tb.Str(`root type must be "object", got `).Str(describeType(schema["type"])).String())
	}
	for _, kw := range elicitForbiddenKeywords {
		if _, present := schema[kw]; present {
			var tb textbuf.Buffer
			return wrapSchemaErr("", tb.Str("forbidden keyword ").Str(kw).Str(" at root").String())
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
	var tb textbuf.Buffer
	for _, kw := range elicitForbiddenKeywords {
		if _, present := prop[kw]; present {
			return wrapSchemaErr(path, tb.Reset().Str("forbidden keyword ").Str(kw).String())
		}
	}
	typ, _ := prop["type"].(string)
	if typ == "" {
		return wrapSchemaErr(path, "missing type")
	}
	if _, ok := elicitPrimitiveTypes[typ]; !ok {
		return wrapSchemaErr(path, tb.Reset().Str("type ").Str(typ).Str(" not supported (need string/number/integer/boolean)").String())
	}
	// Strings with format: format must be in the allowlist.
	if typ == elicitTypeString {
		if f, has := prop["format"]; has {
			fs, ok := f.(string)
			if !ok {
				return wrapSchemaErr(path, "format must be string")
			}
			if _, allowed := elicitStringFormats[fs]; !allowed {
				return wrapSchemaErr(path, tb.Reset().Str("format ").Str(fs).Str(" not supported").String())
			}
		}
	}
	// Enum: per MCP 2025-06-18 elicitation, only string-typed properties
	// may carry an enum (the spec illustrates it only under type=string).
	// Rejecting enum on non-string types catches a common mistake where a
	// caller writes {"type":"number","enum":[1,2]} thinking it means
	// "pick one of these numbers" -- but the spec shape is the string
	// variant with enumNames.
	if _, hasEnum := prop["enum"]; hasEnum && typ != elicitTypeString {
		return wrapSchemaErr(path, "enum is only supported on type=string")
	}
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
	var tb textbuf.Buffer
	return tb.Str("mcp: elicitation: schema invalid at ").Str(e.path).Str(": ").Str(e.reason).String()
}

func (e *elicitSchemaError) Unwrap() error { return ErrElicitSchemaInvalid }

// describeType renders a schema type field for error messages without
// leaking internal types.
func describeType(v any) string {
	if v == nil {
		return "missing"
	}
	if s, ok := v.(string); ok {
		var tb textbuf.Buffer
		return tb.Byte('"').Str(s).Byte('"').String()
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

// methodElicitationCreate is the request method an ElicitRequest carries.
//
// It is a VALUE in an inputRequests map, never a JSON-RPC method this server
// dispatches and never a frame it writes.
//
// MCP 2026-07-28 basic/transports/streamable-http Section "Listening for
// Messages from the Server": "The server MUST NOT send independent JSON-RPC
// requests on this stream. Server-to-client interactions (sampling,
// elicitation, list-roots) are embedded as input requests inside an
// InputRequiredResult per MRTR ..., not delivered as separate requests on this
// or any other stream".
//
// This constant therefore names a payload, not a route.
const methodElicitationCreate = "elicitation/create"

// elicitModeForm and elicitModeURL are the two modes the capability names.
//
// MCP 2026-07-28 client/elicitation Section "Capabilities": "Servers MUST NOT
// send elicitation requests with modes that are not supported by the client."
//
// Ze emits form mode only, and it states that mode explicitly on every request
// rather than rely on the implicit default. The url-mode gap is therefore
// visible at the call site. Url mode is not implemented. Its completion is out
// of band, which is the first flow that would need a real `requestState`.
const (
	elicitModeForm = "form"
	elicitModeURL  = "url"
)

// ElicitRequest wire keys. MCP uses camelCase externally, so these are literals
// rather than struct tags.
const (
	elicitKeyMethod          = "method"
	elicitKeyParams          = "params"
	elicitKeyMode            = "mode"
	elicitKeyMessage         = "message"
	elicitKeyRequestedSchema = "requestedSchema"
	elicitKeyAction          = "action"
	elicitKeyContent         = "content"
)

// elicitSecretMarkers name the credential-shaped property names a form-mode
// elicitation must never ask for.
//
// MCP 2026-07-28 client/elicitation Section "Security Considerations": "Servers
// MUST NOT use form mode elicitation to request sensitive information such as
// passwords, API keys, access tokens, or payment credentials."
//
// Ze normalizes each property name to lowercase and strips the separators,
// then matches these markers as substrings. `user_passphrase` and `apiKey` are
// both caught. The check constrains only what THIS server emits. A broad match
// therefore costs nothing, and a missed one is a specification violation.
var elicitSecretMarkers = []string{
	"password", "passwd", "passphrase", "secret", "token",
	"apikey", "credential", "privatekey", "cvv",
}

// elicitFieldIsSecret reports whether a requested-schema property name looks
// like a credential.
func elicitFieldIsSecret(name string) bool {
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	for _, marker := range elicitSecretMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// newElicitRequest builds one form-mode ElicitRequest for an inputRequests
// entry.
//
// newElicitRequest validates the schema first. A malformed or
// credential-shaped requested schema therefore fails here, and it never
// reaches a client. Ze refuses the credential case at the single emission
// point. The specification's MUST NOT is therefore an enforced property of the
// server, not a claim about the code as it stands today.
func newElicitRequest(message string, schema map[string]any) (map[string]any, error) {
	if err := validateElicitSchema(schema); err != nil {
		return nil, err
	}
	props, _ := schema["properties"].(map[string]any)
	for name := range props {
		if elicitFieldIsSecret(name) {
			var tb textbuf.Buffer
			return nil, wrapSchemaErr(name, tb.Str("form mode must not request sensitive information; ").
				Str("property name looks like a credential").String())
		}
	}
	return map[string]any{
		elicitKeyMethod: methodElicitationCreate,
		elicitKeyParams: map[string]any{
			elicitKeyMode:            elicitModeForm,
			elicitKeyMessage:         message,
			elicitKeyRequestedSchema: schema,
		},
	}, nil
}
