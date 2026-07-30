// Design: docs/architecture/mcp/overview.md -- per-request protocol metadata
// Related: headers.go -- cross-checks the same body fields against HTTP headers
// Related: streamable.go -- calls parseRequestMeta on every POST before dispatch
// Related: apps.go -- resolves the MCP Apps extension settings into UIApps

// Per-request `_meta` parsing for MCP 2026-07-28.
//
// The revision removed the initialize handshake, so a request's protocol
// version, client identity and client capabilities no longer arrive once at
// session creation: every request carries them itself, inside its `params`
// object under the `_meta` key.

package mcp

import (
	"encoding/json"
	"errors"
)

// metaKey is the `_meta` member. On a request it sits inside `params`; on a
// result it sits inside `result`.
const metaKey = "_meta"

// Reserved `_meta` keys the MCP specification defines for the per-request and
// per-response protocol fields. Spelled verbatim: the
// `io.modelcontextprotocol/` prefix is reserved by the specification and these
// strings are part of the wire contract, not Ze names.
const (
	metaKeyProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaKeyClientInfo         = "io.modelcontextprotocol/clientInfo"
	metaKeyClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	metaKeyServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// capabilityElicitation is the `elicitation` member of a capabilities object.
//
// Unlike `tasks` it is NOT a bare presence flag: its value is
// `{ form?: JSONObject; url?: JSONObject; }` and the server must gate on the
// MODE it is about to use, not on the key. elicitationFormSupported (mrtr.go)
// owns that reading.
const capabilityElicitation = "elicitation"

// capabilityExtensionsKey is the `extensions` member of a capabilities object:
// a map of extension identifier to per-extension settings.
const capabilityExtensionsKey = "extensions"

// extensionTasks is the specification-registered identifier for the Tasks
// extension, and the ONLY way a client may declare task support: this
// identifier, under `extensions`. The bare `tasks` member of 2025-11-25 is not
// accepted -- see parseClientCapabilities below for why honoring it would be a
// fail-open.
//
// The same identifier is what server/discover advertises (discover.go), which is
// what makes `resultType: "task"` a value a client is entitled to parse.
const extensionTasks = "io.modelcontextprotocol/tasks"

// `_meta` rejection reasons. Each names the offending field, the shape it must
// have, and what to send instead, so a client can correct the request from the
// error line alone. All three are answered with JSON-RPC -32602 and HTTP 400.
var (
	errMetaMissing            = errors.New(`invalid params: params._meta is required and must be a JSON object carrying "io.modelcontextprotocol/protocolVersion" and "io.modelcontextprotocol/clientCapabilities"`)
	errMetaProtocolVersion    = errors.New(`invalid params: params._meta["io.modelcontextprotocol/protocolVersion"] is required and must be a non-empty version string`)
	errMetaClientCapabilities = errors.New(`invalid params: params._meta["io.modelcontextprotocol/clientCapabilities"] is required and must be a JSON object; send {} to declare no capabilities`)
)

// clientInfo is the optional `io.modelcontextprotocol/clientInfo` value: the
// client's self-reported name and version.
//
// Value type; the zero value means the client did not identify itself, which
// the specification permits (clients only SHOULD send it). Self-reported and
// unverified, so it is carried for display and logging only and never reaches
// an authorization or ownership decision.
type clientInfo struct {
	Name    string
	Version string
}

// clientCapabilities is the capability set one request declared in its
// `io.modelcontextprotocol/clientCapabilities` object.
//
// Value type with a fail-closed zero value: every field is a bool that stays
// false until the client declares the capability, so a handler holding a zero
// clientCapabilities denies rather than serves. There is deliberately no
// pointer and no third "unknown" state -- absence and non-declaration are the
// same verdict (ai/rules/fail-closed-guards.md).
//
// Only capabilities a CLIENT can actually declare belong here. A server
// capability (resources, tools, prompts) is something this server offers, not
// something it may demand of a caller.
type clientCapabilities struct {
	// Tasks reports whether the client declared the
	// `io.modelcontextprotocol/tasks` extension identifier under `extensions`.
	//
	// A false verdict means the client never receives a task handle: a command
	// this server would otherwise run as a task runs synchronously instead, and
	// the caller gets the ordinary `resultType: "complete"` answer (D-2). It is
	// not an error and it never costs the client its result.
	Tasks bool
	// UIApps reports whether the client declared the MCP Apps extension
	// (`io.modelcontextprotocol/ui` under `extensions`) in a form compatible
	// with the HTML bundles this server serves. It is the resolved verdict of
	// the five-case settings gate in apps.go, not the raw presence of the key,
	// so a client declaring only mime types Ze cannot produce reads false.
	//
	// A false verdict removes `_meta.ui` from every tool descriptor and changes
	// nothing else: the tool is still listed and still callable.
	UIApps bool
	// ElicitForm reports whether the client declared FORM-mode elicitation. It
	// is the resolved verdict of the mode gate in mrtr.go, not the presence of
	// the `elicitation` key: a client declaring `{"url":{}}` supports
	// elicitation and reads false here, because "Servers MUST NOT send
	// elicitation requests with modes that are not supported by the client".
	//
	// A false verdict means a tool that needs input answers with the
	// missing-argument error instead of an InputRequiredResult.
	ElicitForm bool
}

// requestMeta is the parsed per-request `_meta` block. Value type, built once
// per request before dispatch and passed by value from there on.
type requestMeta struct {
	ProtocolVersion string
	ClientInfo      clientInfo
	Capabilities    clientCapabilities
}

// decodeParamsObject decodes a request's `params` into a generic map.
//
// Returns nil when params is absent or is not a JSON object (an array, a
// string, a number). Both cases mean the request carries no `_meta` block and
// no Mcp-Name source field, which every caller reads as "field absent" and
// rejects on its own terms. MCP uses camelCase wire keys, so the decode is
// generic rather than into a tagged struct.
func decodeParamsObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// parseRequestMeta extracts the per-request protocol metadata from a request's
// already-decoded `params` object.
//
// MCP 2026-07-28 basic/index Section "_meta": the per-request protocol fields
// table marks "io.modelcontextprotocol/protocolVersion" and
// "io.modelcontextprotocol/clientCapabilities" Required=Yes and
// "io.modelcontextprotocol/clientInfo" Required=No. "A request missing any
// required field is malformed; the server MUST reject it with JSON-RPC error
// code -32602 (Invalid params). On HTTP, the response status MUST be 400 Bad
// Request."
//
// A missing `_meta` field is NOT a header mismatch: -32020 is reserved for the
// header/body contract, and the two failures stay distinguishable so a client
// can tell "you sent the wrong header" from "you omitted a required field".
func parseRequestMeta(params map[string]any) (requestMeta, error) {
	meta, ok := params[metaKey].(map[string]any)
	if !ok {
		return requestMeta{}, errMetaMissing
	}
	version, ok := meta[metaKeyProtocolVersion].(string)
	if !ok || version == "" {
		return requestMeta{}, errMetaProtocolVersion
	}
	caps, ok := meta[metaKeyClientCapabilities].(map[string]any)
	if !ok {
		return requestMeta{}, errMetaClientCapabilities
	}
	return requestMeta{
		ProtocolVersion: version,
		ClientInfo:      parseClientInfo(meta),
		Capabilities:    parseClientCapabilities(caps),
	}, nil
}

// metaProtocolVersion returns the protocol version a request body's `_meta`
// declares, and whether it is present as a non-empty string.
//
// Separate from parseRequestMeta because header validation runs first and must
// distinguish "the body declares a different version" (a -32020 mismatch) from
// "the body declares none" (a -32602 malformed request, reported afterwards by
// parseRequestMeta).
func metaProtocolVersion(params map[string]any) (string, bool) {
	meta, ok := params[metaKey].(map[string]any)
	if !ok {
		return "", false
	}
	v, ok := meta[metaKeyProtocolVersion].(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// parseClientInfo reads the optional client Implementation object. A missing
// or malformed value yields the zero clientInfo rather than an error: the
// specification makes clientInfo a SHOULD, so its absence cannot fail a
// request.
func parseClientInfo(meta map[string]any) clientInfo {
	info, ok := meta[metaKeyClientInfo].(map[string]any)
	if !ok {
		return clientInfo{}
	}
	name, _ := info["name"].(string)
	version, _ := info["version"].(string)
	return clientInfo{Name: name, Version: version}
}

// parseClientCapabilities reads the declared capability set.
//
// A capability counts as declared only when its value is a JSON object; the
// specification's shape is `"resources": {}`, so a null, a bool or a string
// under the same key is not a declaration. Anything unrecognized is ignored,
// which is what keeps the zero value fail-closed: an unparseable capability is
// an undeclared capability.
func parseClientCapabilities(caps map[string]any) clientCapabilities {
	var out clientCapabilities
	// Tasks is declared through the `extensions` map and ONLY through it.
	//
	// The bare `tasks` member was the 2025-11-25 core-protocol spelling and is
	// no longer accepted. That is a fail-closed decision, not tidying: under the
	// server-directed model (D-1) this bit decides whether the server may hand
	// back an unsolicited `resultType: "task"` handle. A 2025-11-25-era client
	// declaring `{"tasks":{}}` was opting into a model where IT asked for each
	// task; honoring that declaration now would push an unsolicited task handle
	// at a client that never agreed to receive one, which is exactly the failure
	// the per-request extension check exists to prevent.
	//
	// `tasks` is not a ClientCapabilities member in this revision in any case:
	// the five are experimental, roots, sampling, elicitation and extensions.
	if ext, ok := caps[capabilityExtensionsKey].(map[string]any); ok {
		if _, declared := ext[extensionTasks].(map[string]any); declared {
			out.Tasks = true
		}
	}
	out.UIApps = clientSupportsUIApps(caps)
	out.ElicitForm = elicitationFormSupported(caps)
	return out
}
