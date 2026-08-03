// Design: docs/architecture/mcp/overview.md -- multi round-trip requests
// Related: elicit.go -- the ElicitRequest values this file carries and validates
// Related: streamable_tools.go -- runMethod, the guard site, and the ok() envelope

// MCP 2026-07-28 Multi Round-Trip Requests (MRTR).
//
// A server that needs more input does NOT send a JSON-RPC request to the
// client. It RETURNS `resultType: "input_required"` with an `inputRequests`
// map, and that answer ends the original request. The client then
// gathers the input and retries the original request with `inputResponses` and
// a different JSON-RPC id. Everything the server needs to process the retry is
// in the retry.
//
// Ze emits NO `requestState`. Its one elicitation suspends nothing. The value
// it asks for is a tool argument the retry carries anyway, so there is no
// continuation state to sign. And `inputRequests` alone satisfies the
// at-least-one-of rule of MRTR server requirement 6. A design with no token
// cannot leak one. Read rejectUnsolicitedRequestState for the obligation that
// lands on whoever mints the first real state.
//
// Reference: https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr

package mcp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"maps"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// mrtrLogger carries the guard warnings. A rejected `requestState` and a
// suppressed input-required result are both protocol violations that must be
// visible to an operator, not silently absorbed. Both guards satisfy
// ai/rules/evidence.md's "fail closed or say something" twice over,
// by answering the caller with an error AND logging.
var mrtrLogger = slogutil.Logger("mcp.mrtr")

// MRTR wire keys. MCP is camelCase on the wire, so these are literals rather
// than struct tags. Nothing here is a Ze name.
const (
	// resultTypeKey is the discriminator every result carries.
	resultTypeKey = "resultType"
	// resultTypeInputRequired marks an interim result: the request is
	// incomplete and more information is needed.
	resultTypeInputRequired = "input_required"
	// inputRequestsKey holds the server-assigned map of requests the client
	// must fulfill before it retries.
	inputRequestsKey = "inputRequests"
	// inputResponsesKey holds the client's answers on the retry. It sits in the
	// request's `params`, beside `name` and `arguments`.
	inputResponsesKey = "inputResponses"
	// requestStateKey is the opaque server-continuation field. Ze never mints
	// one, and rejects any that arrives.
	requestStateKey = "requestState"
)

// inputKeyExecuteCommand is the server-assigned `inputRequests` key under which
// ze_execute asks for its command, and the key a retry echoes in
// `inputResponses`.
//
// MCP 2026-07-28 basic/patterns/mrtr Section "Server Requirements":
// "`inputRequests` keys are server assigned identifiers and MUST be unique
// within the scope of the request." Ze emits exactly one entry per result, so
// uniqueness is structural.
const inputKeyExecuteCommand = "ze_execute_command"

// elicitFieldCommand is the single property of the requested schema, and the
// key the accepted `content` object carries the answer under.
const elicitFieldCommand = "command"

// elicitPromptCommand is the message shown to the user.
const elicitPromptCommand = "Which ze command should be run? For example: show bgp summary"

// inputOutcome is what one `inputResponses` lookup decided.
//
// The zero value is inputMissing, which re-asks. That is the safe default: a
// zero-valued outcome can never dispatch, only prompt again
// (ai/rules/evidence.md).
type inputOutcome uint8

const (
	// inputMissing -- the client did not answer this key, or accepted with an
	// empty value. MCP 2026-07-28 basic/patterns/mrtr Section "Error Handling"
	// treats missing requested information as a case to ask again rather than
	// an error. Server requirement 8 explicitly permits the server to repeat
	// the prompt: "Servers MAY choose to return an InputRequiredResult on
	// multiple attempts at the same request". A re-ask is therefore conformant.
	inputMissing inputOutcome = iota
	// inputAccepted -- the client supplied a usable value.
	inputAccepted
	// inputDeclined -- the user explicitly refused. Terminal: re-asking would
	// loop a user who has said no.
	inputDeclined
	// inputCanceled -- the user dismissed the dialog and made no choice.
	// Terminal, like decline.
	inputCanceled
	// inputMalformed -- the entry was present but not a parseable elicit
	// response. Terminal: a client that cannot be understood is not re-asked.
	inputMalformed
)

func (o inputOutcome) String() string {
	switch o {
	case inputMissing:
		return "missing"
	case inputAccepted:
		return "accepted"
	case inputDeclined:
		return "declined"
	case inputCanceled:
		return "canceled"
	case inputMalformed:
		return "malformed"
	default:
		return "unknown"
	}
}

// errUnsolicitedRequestState is the stable rejection for a `requestState` this
// server never issued.
//
// The message names the failure class and the offending field, and it carries
// NO part of the supplied value. That value is attacker-controlled input, and
// an error that echoes it becomes a reflection surface
// (ai/rules/cli.md).
var errUnsolicitedRequestState = errors.New(
	`invalid params: params.requestState is not accepted; this server issues no requestState, so a retry must not carry one`)

// newInputRequiredResult wraps an inputRequests map as an InputRequiredResult.
//
// MCP 2026-07-28 basic/patterns/mrtr Section "Server Requirements": "6. Servers
// MUST include at least one of `inputRequests` or `requestState` in every
// `InputRequiredResult` response."
//
// A non-empty `inputRequests` satisfies that on its own. No `requestState` is
// therefore emitted here or anywhere. Requirement 3 makes it a MAY ("The
// InputRequiredResult MAY include a requestState field"), and Ze has nothing
// to put in it.
//
// The result deliberately carries no `isError`: an input-required outcome is an
// interim success, not a tool failure.
func newInputRequiredResult(requests map[string]any) map[string]any {
	return map[string]any{
		resultTypeKey:    resultTypeInputRequired,
		inputRequestsKey: requests,
	}
}

// inputRequiredForMissingCommand builds the one InputRequiredResult this server
// can emit: ze_execute asking for the `ze` command to run.
//
// This is the single production caller of newElicitRequest. The table test of
// AC-11 enumerates the emittable set from here. A second elicitation that does
// not also update that table therefore fails, and it cannot pass the form-mode
// prohibition on sensitive information.
func inputRequiredForMissingCommand() (map[string]any, error) {
	request, err := newElicitRequest(elicitPromptCommand, map[string]any{
		"type": "object",
		"properties": map[string]any{
			elicitFieldCommand: map[string]any{
				"type":        elicitTypeString,
				"title":       "ze command",
				"description": "The ze command to execute, for example 'show bgp peer list'.",
			},
		},
		"required": []any{elicitFieldCommand},
	})
	if err != nil {
		return nil, err
	}
	return newInputRequiredResult(map[string]any{inputKeyExecuteCommand: request}), nil
}

// resolveExecuteCommand decides ze_execute's effective command from the two
// places it can arrive: the `command` tool argument, then the `inputResponses`
// entry a Multi Round-Trip retry carries.
//
// This is what makes the handler re-entrant. It is a pure function of
// (arguments, inputResponses), and both of those live in the current request.
// A retry is therefore servable with nothing carried over from the attempt
// that prompted for it. And that is why no `requestState` is needed.
func (s *server) resolveExecuteCommand(argument string) (string, inputOutcome) {
	if argument != "" {
		return argument, inputAccepted
	}
	return resolveElicitedValue(s.inputResponses, inputKeyExecuteCommand, elicitFieldCommand)
}

// askForCommand answers a call that supplied no command.
//
// With form-mode elicitation declared, askForCommand returns the
// InputRequiredResult that asks for one. Without it, askForCommand returns the
// missing-argument error result.
//
// MCP 2026-07-28 basic/patterns/mrtr Section "Server Requirements" says "7.
// Servers MUST NOT send an inputRequests that the client has not declared
// support for in its capabilities". And client/elicitation adds "Servers MUST
// NOT send elicitation requests with modes that are not supported by the
// client".
//
// A fresh prompt on every unanswered attempt is deliberate and safe. Server
// requirement 8 permits it. And no state is held between rounds, so an
// unanswered prompt costs this server nothing.
func (s *server) askForCommand() map[string]any {
	if !s.caps.ElicitForm {
		return ErrResult("missing required argument: command")
	}
	result, err := inputRequiredForMissingCommand()
	if err != nil {
		mrtrLogger.Warn("could not build the elicitation for a missing command", slog.Any("error", err))
		return ErrResult("missing required argument: command")
	}
	return result
}

// schemaKeyRequired / schemaKeyInputSchema are the two JSON Schema members the
// descriptor gate below rewrites. MCP descriptors are camelCase on the wire, so
// these are the specification's spellings rather than Ze names.
const (
	schemaKeyInputSchema = "inputSchema"
	schemaKeyRequired    = "required"
	toolKeyName          = "name"
)

// gateExecuteCommandRequired marks ze_execute's `command` argument required for
// a client this server cannot prompt.
//
// The published descriptor and the handler MUST agree, and the handler's
// answer depends on one capability. askForCommand returns the
// InputRequiredResult when the client declared form-mode elicitation, and it
// returns the missing-argument error when the client did not. A single static
// `required: ["command"]` therefore cannot be correct for both.
//
// It was, for a while, published unconditionally. That made the whole MRTR
// feature unreachable through the advertised interface. A schema-validating
// host will not construct a call its own schema rejects, and a model that
// reads "required" will not try. The descriptor is the interface. A behavior
// no client can address does not exist.
//
// So the shape is derived from the same input the handler branches on:
//
//	| Client declared form-mode elicitation | inputSchema.required | Omitting `command` yields |
//	|---------------------------------------|----------------------|---------------------------|
//	| yes                                    | absent               | resultType input_required |
//	| no                                     | ["command"]          | missing-argument error    |
//
// The gate is applied to the handcrafted descriptors only, and never to a
// ToolProvider's. A provider owns its own tool set with its own handlers, and
// a same-named tool there is not this handler.
//
// Nothing is mutated in place. handcraftedTools is package-level and shared by
// every concurrent request. Ze therefore copies the descriptor that changes,
// along with the one nested map that changes with it. gateUIMeta (apps.go)
// uses the same copy-on-write discipline for the same reason.
func gateExecuteCommandRequired(tools []map[string]any, elicitForm bool) []map[string]any {
	if elicitForm {
		return tools
	}
	out := tools
	copied := false
	for i, tool := range tools {
		marked, changed := withRequiredCommand(tool)
		if !changed {
			continue
		}
		if !copied {
			out = make([]map[string]any, len(tools))
			copy(out, tools)
			copied = true
		}
		out[i] = marked
	}
	return out
}

// withRequiredCommand returns a copy of the ze_execute descriptor with
// `inputSchema.required` set to the one argument the handler will insist on,
// reporting whether anything changed. Every other descriptor is returned
// untouched, so the gate allocates nothing for them.
func withRequiredCommand(tool map[string]any) (map[string]any, bool) {
	if name, _ := tool[toolKeyName].(string); name != toolNameExecute {
		return tool, false
	}
	schema, ok := tool[schemaKeyInputSchema].(map[string]any)
	if !ok {
		return tool, false
	}
	if _, already := schema[schemaKeyRequired]; already {
		return tool, false
	}
	markedSchema := make(map[string]any, len(schema)+1)
	maps.Copy(markedSchema, schema)
	markedSchema[schemaKeyRequired] = []any{elicitFieldCommand}

	marked := make(map[string]any, len(tool))
	maps.Copy(marked, tool)
	marked[schemaKeyInputSchema] = markedSchema
	return marked, true
}

// isInputRequiredResult reports whether a handler's result is the MRTR interim
// result rather than a finished one.
func isInputRequiredResult(result map[string]any) bool {
	value, _ := result[resultTypeKey].(string)
	return value == resultTypeInputRequired
}

// permitsInputRequired reports whether the specification allows an
// InputRequiredResult on this JSON-RPC method.
//
// MCP 2026-07-28 basic/patterns/mrtr Section "Supported Requests" lists exactly
// three: `prompts/get`, `resources/read` and `tools/call`. "Servers MUST NOT
// send InputRequiredResult responses on any other client requests."
//
// The predicate mirrors the specification rather than Ze's current reach. Ze
// does not implement `prompts/get` (a POST for it is answered 404 before
// dispatch), so that method can never produce one. But the name keeps the rule
// readable against the spec table it comes from. The predicate is fail-closed
// by construction: anything not named here is refused.
func permitsInputRequired(method string) bool {
	switch method {
	case methodPromptsGet, methodResourcesRead, methodToolsCall:
		return true
	default:
		return false
	}
}

// guardInputRequired refuses to let an InputRequiredResult leave on a method
// the specification forbids it on.
//
// It runs on EVERY dispatched method, on the single path out of runMethod. A
// handler therefore cannot emit one by accident, and no per-method switch is
// duplicated anywhere else. The WARN branch means a Ze bug, not a client one.
// It answers -32603 and says so rather than strip the field quietly.
func (s *Streamable) guardInputRequired(method string, resp *response) *response {
	if resp == nil || resp.Error != nil {
		return resp
	}
	result, ok := resp.Result.(map[string]any)
	if !ok || !isInputRequiredResult(result) {
		return resp
	}
	if permitsInputRequired(method) {
		return resp
	}
	mrtrLogger.Warn("suppressed an input-required result on a method the specification forbids it on",
		slog.String("method", method))
	return s.fail(resp.ID, rpcInternalError,
		"internal error: an input-required result is not permitted on this method")
}

// rejectUnsolicitedRequestState refuses any request carrying a `requestState`.
//
// MCP 2026-07-28 basic/patterns/mrtr Section "Client Requirements": "2. ... If
// the InputRequiredResult does not contain a requestState field, the client
// MUST NOT include one in the retry." Ze never emits one, so every arriving
// value came from a confused or hostile client.
//
// Section "Server Requirements": "4. If a client request contains a
// requestState field, servers MUST treat requestState as an attacker-controlled
// input. ... and MUST reject state that fails verification."
//
// With no key and no minting path, verification fails vacuously for every
// value. Rejection is therefore the conformant answer as well as the
// fail-closed one (ai/rules/evidence.md). A guard that ignored the
// field would leave a silent accept path, and a future implementation can
// inherit that path unnoticed.
//
// OBLIGATION FOR WHOEVER MINTS THE FIRST REAL requestState (spec R-1): this
// guard is the tripwire. A change that relaxes it into an accept path makes
// MRTR server requirement 5 apply IN FULL in that same change. Requirement 5
// covers three fields:
//
//   - the authenticated principal
//   - a short expiry
//   - an identifier for the originating request
//
// Each field is carried inside integrity-protected state (HMAC or AEAD). Each
// field is verified on receipt. And each field carries a negative test.
func rejectUnsolicitedRequestState(params map[string]any) error {
	if params == nil {
		return nil
	}
	if _, present := params[requestStateKey]; present {
		return errUnsolicitedRequestState
	}
	return nil
}

// elicitationFormSupported reports whether a client's declared capabilities
// include FORM-mode elicitation.
//
// MCP 2026-07-28 client/elicitation Section "Capabilities": the capability is
// `elicitation?: { form?: JSONObject; url?: JSONObject; }`, and the
// specification states that an empty capabilities object "is equivalent to
// declaring support for `form` mode only".
//
// "Servers MUST NOT send elicitation requests with modes that are not
// supported by the client". The question is therefore form-mode support, and
// never the presence of the `elicitation` key. A client that declares
// `{"url":{}}` supports elicitation and does not support form mode.
//
// The reader is fail-closed on every shape that is not a declaration. A
// non-object capability value, a non-object `form` value, and an object naming
// only modes this server does not recognize all read as "no form mode". Ze
// implements no url mode, so a url-only client is never prompted at all.
func elicitationFormSupported(caps map[string]any) bool {
	modes, ok := caps[capabilityElicitation].(map[string]any)
	if !ok {
		return false
	}
	if len(modes) == 0 {
		return true
	}
	_, declared := modes[elicitModeForm].(map[string]any)
	return declared
}

// decodeInputResponses pulls the `inputResponses` object out of a request's raw
// `params`.
//
// It decodes generically rather than through a tagged struct. `inputResponses`
// is a camelCase MCP wire key, and its VALUES are open-ended per-request
// objects keyed by server-assigned identifiers. No fixed Go shape exists to
// unmarshal into. This is the same convention the rest of this package uses
// for spec-defined bodies (meta.go).
//
// Returns nil when the field is absent or is not a JSON object. Both read as
// "the client answered nothing", which re-asks rather than fails. MCP
// 2026-07-28 basic/patterns/mrtr Section "Error Handling" also tells servers
// to tolerate unexpected InputResponses parameters. Unrecognized entries are
// therefore carried through and ignored by the resolver, and not rejected
// here.
func decodeInputResponses(rawParams json.RawMessage) map[string]any {
	responses, _ := decodeParamsObject(rawParams)[inputResponsesKey].(map[string]any)
	return responses
}

// resolveElicitedValue reads one answer out of an `inputResponses` map.
//
// key is the server-assigned `inputRequests` identifier. field is the property
// of the requested schema whose value the caller wants. Unrecognized keys in
// the map are ignored, per the error-handling clause on unexpected
// InputResponses parameters.
//
// An `accept` carrying no usable value is reported as inputMissing rather than
// inputAccepted, so an empty answer prompts again instead of dispatching an
// empty command. A `decline` or `cancel` is terminal and stays distinguishable
// from an omission, which is what stops an explicit refusal from being looped.
func resolveElicitedValue(responses map[string]any, key, field string) (string, inputOutcome) {
	raw, present := responses[key]
	if !present {
		return "", inputMissing
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return "", inputMalformed
	}
	action, _ := entry[elicitKeyAction].(string)
	switch action {
	case elicitActionAccept:
		content, _ := entry[elicitKeyContent].(map[string]any)
		value, _ := content[field].(string)
		if value == "" {
			return "", inputMissing
		}
		return value, inputAccepted
	case elicitActionDecline:
		return "", inputDeclined
	case elicitActionCancel:
		return "", inputCanceled
	default:
		return "", inputMalformed
	}
}
