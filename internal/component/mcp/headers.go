// Design: docs/architecture/mcp/overview.md -- standard request header validation
// Related: meta.go -- parses the _meta block this file cross-checks
// Related: streamable.go -- calls validateStandardHeaders before dispatch

// MCP 2026-07-28 standard request headers.
//
// The transport mirrors selected JSON-RPC body fields into HTTP headers so
// intermediaries (load balancers, gateways, observability tooling) can route
// and inspect a request without parsing its body. The server owns the other
// half of that contract: reject any request whose headers disagree with its
// body, so an intermediary and the server can never act on different values.
//
// Every failure here is JSON-RPC -32020 (HeaderMismatch) with HTTP 400, and
// none of the messages reflect raw header bytes back to the client.

package mcp

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Standard request header names. Casing is not uniform in the specification:
// the protocol-version header is all-caps `MCP`, the other three are title
// case. http.Header.Get is case-insensitive, so these spellings govern what a
// client is told to send, not how a received header is matched.
const (
	headerProtocolVersion = "MCP-Protocol-Version"
	headerMethod          = "Mcp-Method"
	headerName            = "Mcp-Name"
	// headerParamPrefix is the prefix of the custom headers a server may
	// designate with an `x-mcp-header` annotation on a tool parameter.
	headerParamPrefix = "Mcp-Param-"
)

// Base64 sentinel markers for a header value that cannot be carried as plain
// ASCII.
//
// MCP 2026-07-28 basic/transports/streamable-http Section "Value Encoding"
// requires that "the prefix `=?base64?` and suffix `?=` indicate that the value
// is Base64-encoded", and that "these markers are case-sensitive and MUST appear
// exactly as shown (lowercase)".
const (
	base64SentinelPrefix = "=?base64?"
	base64SentinelSuffix = "?="
)

// Header rejection reasons.
//
// Each names which header disagreed with which body field without echoing
// either value: a -32020 response must not reflect unvalidated header bytes
// back to the client, and the body value is client-supplied too. The leading
// phrase is stable per condition so log scanners and tests can match on it.
var (
	errHeaderProtocolVersionMissing  = errors.New("header mismatch: MCP-Protocol-Version header is required on every POST to the MCP endpoint")
	errHeaderProtocolVersionMismatch = errors.New(`header mismatch: MCP-Protocol-Version header disagrees with params._meta["io.modelcontextprotocol/protocolVersion"]; send the same version in both`)
	errHeaderMethodMissing           = errors.New("header mismatch: Mcp-Method header is required on every POST to the MCP endpoint and must repeat the body's method")
	errBodyCarriesNoMethod           = errors.New(`header mismatch: the request body carries no "method", so no Mcp-Method header value can mirror it; this endpoint accepts JSON-RPC requests and notifications only, never a client-sent response or error frame`)
	errHeaderMethodMismatch          = errors.New("header mismatch: Mcp-Method header disagrees with the body's method; header values are compared case-sensitively")
	errHeaderNameMissing             = errors.New("header mismatch: Mcp-Name header is required for tools/call, resources/read and prompts/get, mirroring params.name or params.uri")
	errHeaderNameMismatch            = errors.New("header mismatch: Mcp-Name header disagrees with params.name (tools/call, prompts/get) or params.uri (resources/read)")
	errHeaderNameEncoding            = errors.New("header mismatch: Mcp-Name header uses the =?base64?...?= sentinel but its payload is not padded standard Base64")
	errHeaderParamValue              = errors.New("header mismatch: an Mcp-Param-* header value contains octets an HTTP field value does not permit; carry it with the =?base64?...?= sentinel")
)

// initializeEraError is the diagnostic an `initialize` POST receives.
//
// MCP 2026-07-28 basic/versioning Section "... with Initialization-Based
// Versions": "A server that supports only modern versions SHOULD name the
// protocol versions it supports in any error it returns to an `initialize`
// request, on any transport: legacy clients have no fall-forward mechanism,
// and this message may be the only diagnostic they can surface to users."
//
// The version list is derived from supportedProtocolVersions rather than
// spelled here, so it cannot drift from what the server actually accepts.
// lead is the caller's stable error phrase; the rest is common to both the
// header-validation rejection and the unknown-method rejection, which are the
// two errors an `initialize` POST can receive.
func initializeEraError(lead string) error {
	var tb textbuf.Buffer
	tb.Str(lead)
	tb.Str(`: "initialize" is not a method in this protocol era; this server supports protocol version `)
	tb.Join(supportedProtocolVersions, ", ")
	tb.Str(", which removed the initialize handshake -- send the MCP-Protocol-Version and Mcp-Method headers plus a params._meta block on every request instead")
	return errors.New(tb.String())
}

// validateStandardHeaders enforces the MCP 2026-07-28 header/body contract.
//
// MCP 2026-07-28 basic/transports/streamable-http Section "Server Validation":
// "Servers that process the request body MUST reject requests where the values
// specified in the headers do not match the corresponding values in the request
// body. ... Validation failure conditions include: a required standard header
// (MCP-Protocol-Version, Mcp-Method, Mcp-Name) is missing; a header value does
// not match the corresponding request body value ...; a header value contains
// invalid characters."
//
// params is the request's `params` object decoded leniently (nil when absent or
// not a JSON object). The `_meta` protocol version is compared only when it is
// present, because a MISSING `_meta` field is a different failure (-32602) that
// the caller reports next.
//
// Returns nil when every required header is present and agrees with the body.
func validateStandardHeaders(r *http.Request, req *request, params map[string]any) error {
	// A JSON-RPC RESPONSE frame -- `{"jsonrpc":"2.0","id":1,"result":{...}}` or
	// its error twin -- is refused here, first, and it is refused BY NAME rather
	// than as a side effect of the Mcp-Method rule below.
	//
	// This server writes no JSON-RPC request to a client and therefore never
	// asks one a question: MCP 2026-07-28 replaced server-initiated elicitation
	// with Multi Round-Trip Requests, where the server RETURNS
	// `resultType: "input_required"` and the client RETRIES its own request
	// (mrtr.go). A response frame arriving here answers a question this server
	// did not ask, so there is nothing it could legitimately be correlated with.
	//
	// The Mcp-Method contract already makes such a frame unrepresentable -- the
	// header is required on every POST and "must repeat the body's method", and
	// a response has no method for any header value to equal -- but leaving the
	// refusal implicit in that arithmetic left nothing naming the property, and
	// a later change to header validation could restore an accept path without
	// anything noticing (ai/rules/fail-closed-guards.md: make the miss explicit
	// at the producer).
	//
	// The verdict deliberately stays -32020 with HTTP 400, the code the same
	// Section "Server Validation" pins to a header/body disagreement. Only the
	// message is more specific.
	if req.Method == "" {
		return errBodyCarriesNoMethod
	}

	// MCP 2026-07-28 basic/transports/streamable-http Section "Protocol Version
	// Header": "Every POST request to the MCP endpoint MUST include an
	// MCP-Protocol-Version header."
	//
	// Absence and mismatch are the same verdict: there is no handshake left to
	// fall back on, so a missing header may not default to any revision
	// (ai/rules/fail-closed-guards.md).
	headerVersion := r.Header.Get(headerProtocolVersion)
	if headerVersion == "" {
		if req.Method == initializeMethod {
			return initializeEraError(headerMismatchLead)
		}
		return errHeaderProtocolVersionMissing
	}

	// MCP 2026-07-28 basic/transports/streamable-http Section "Standard Request
	// Headers": "| Mcp-Method | method | All requests |" and "These headers are
	// REQUIRED for compliance."
	//
	// Section "Case Sensitivity": "Header names ... are case-insensitive.
	// Clients and servers MUST use case-insensitive comparisons for header
	// names. Header values (such as method names) are case-sensitive."
	// http.Header.Get supplies the case-insensitive name lookup; the value
	// comparison below is deliberately exact.
	headerMethodValue := r.Header.Get(headerMethod)
	if headerMethodValue == "" {
		return errHeaderMethodMissing
	}
	if headerMethodValue != req.Method {
		return errHeaderMethodMismatch
	}

	if source, required := mcpNameSource(req.Method, params); required {
		raw := r.Header.Get(headerName)
		if raw == "" {
			return errHeaderNameMissing
		}
		// Section "Value Encoding": "servers MUST decode an encoded Mcp-Name or
		// Mcp-Param-{Name} value before comparing it to the corresponding
		// request body value during Server Validation."
		decoded, ok := decodeSentinel(raw)
		if !ok {
			return errHeaderNameEncoding
		}
		if decoded != source {
			return errHeaderNameMismatch
		}
	}

	// Section "Server Behavior for Custom Headers": "Servers MUST reject
	// requests with a recognized Mcp-Param-{Name} header that contains invalid
	// characters."
	if err := validateParamHeaders(r.Header); err != nil {
		return err
	}

	// Section "Protocol Version Header": "The header value MUST match the
	// io.modelcontextprotocol/protocolVersion field carried in the request
	// body's _meta. If the values do not match, the server MUST reject the
	// request with 400 Bad Request and a HeaderMismatch JSON-RPC error."
	if bodyVersion, present := metaProtocolVersion(params); present && bodyVersion != headerVersion {
		return errHeaderProtocolVersionMismatch
	}
	return nil
}

// mcpNameSource returns the body value the Mcp-Name header must mirror, and
// whether the method requires the header at all.
//
// MCP 2026-07-28 basic/transports/streamable-http Section "Standard Request
// Headers": "| Mcp-Name | params.name or params.uri | tools/call,
// resources/read, prompts/get requests |".
//
// Which of the two source fields applies is decided by the request type, not by
// the pair's order in that cell: `CallToolRequest` and `GetPromptRequest` both
// carry `name`, and only `ReadResourceRequest` carries `uri`. Reading
// prompts/get out of `params.uri` mirrors a field that is never present, so
// every prompts/get failed header validation with -32020 and the 404 the
// specification requires for an unimplemented method was unreachable.
//
// prompts/get is covered even though this server does not dispatch it: header
// validation is a transport guard that runs before dispatch, so a prompts/get
// POST is header-checked on the way to its unknown-method 404.
func mcpNameSource(method string, params map[string]any) (string, bool) {
	switch method {
	case methodToolsCall, methodPromptsGet:
		name, _ := params["name"].(string)
		return name, true
	case methodResourcesRead:
		uri, _ := params["uri"].(string)
		return uri, true
	}
	return "", false
}

// decodeSentinel decodes the `=?base64?{payload}?=` header sentinel.
//
// A value not in sentinel form is returned unchanged with ok=true: the sentinel
// is opt-in, and a conformant client only uses it when the plain value cannot
// be carried in an HTTP field. The markers are matched exactly as the
// specification prints them, so an uppercase `=?BASE64?` is a plain value and
// not a malformed sentinel.
//
// The encoding is standard Base64 WITH padding, not base64url: the
// specification's own example encodes to `PT9iYXNlNjQ/bGl0ZXJhbD89`, whose `/`
// is outside the URL-safe alphabet.
func decodeSentinel(v string) (string, bool) {
	if !strings.HasPrefix(v, base64SentinelPrefix) || !strings.HasSuffix(v, base64SentinelSuffix) {
		return v, true
	}
	if len(v) < len(base64SentinelPrefix)+len(base64SentinelSuffix) {
		// Prefix and suffix overlap: "=?base64?=" satisfies both tests but
		// leaves no payload window. Malformed, not a plain value.
		return "", false
	}
	payload := v[len(base64SentinelPrefix) : len(v)-len(base64SentinelSuffix)]
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

// validateParamHeaders rejects an Mcp-Param-* header whose value is not a legal
// HTTP field value.
//
// Ze annotates no tool parameter with `x-mcp-header` (tools.go emits no such
// key), so no Mcp-Param-* header has a body field to compare against and the
// header/body half of the contract has nothing to check here. Per the
// specification's behavior matrix, a parameter the server did not annotate is
// one the server "MUST NOT expect" a header for. The character half still
// applies to every recognized Mcp-Param-* header and is enforced.
func validateParamHeaders(h http.Header) error {
	for name, values := range h {
		// RFC 9110 field names are case-insensitive. net/http canonicalizes
		// keys on the wire path, but compare case-insensitively so a key set
		// programmatically in a non-canonical form is still recognized.
		if len(name) < len(headerParamPrefix) || !strings.EqualFold(name[:len(headerParamPrefix)], headerParamPrefix) {
			continue
		}
		for _, v := range values {
			if !validFieldValue(v) {
				return errHeaderParamValue
			}
		}
	}
	return nil
}

// validFieldValue reports whether s consists only of the octets RFC 9110
// Section 5.5 permits in an HTTP field value: visible ASCII (0x21-0x7E), space
// (0x20) and horizontal tab (0x09).
func validFieldValue(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c == 0x09 || c == 0x20 || (c >= 0x21 && c <= 0x7E) {
			continue
		}
		return false
	}
	return true
}
