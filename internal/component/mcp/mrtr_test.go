package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// errDispatchShouldNotRun is returned by a dispatcher fake that must never be
// called. The t.Fatal above it is the real assertion; this exists so the fake
// does not return a nil error beside a nil response.
var errDispatchShouldNotRun = errors.New("dispatch must not run without a command")

// formCaps is the clientCapabilities a client declaring form-mode elicitation
// arrives with. Built through the real parser so a test can never declare a
// capability shape the wire parser would not have produced.
func formCaps(t *testing.T) clientCapabilities {
	t.Helper()
	return parseClientCapabilities(map[string]any{capabilityElicitation: map[string]any{}})
}

// VALIDATES: AC-1 -- an InputRequiredResult carries resultType "input_required"
// and exactly one inputRequests entry, whose value is an elicitation/create
// request with an explicit mode of "form".
// PREVENTS: the interim result being mistaken for a finished one, and the
// url-mode gap becoming invisible by relying on the implicit form default.
func TestInputRequiredResultShape(t *testing.T) {
	result, err := inputRequiredForMissingCommand()
	if err != nil {
		t.Fatalf("inputRequiredForMissingCommand: %v", err)
	}
	if got := result[resultTypeKey]; got != resultTypeInputRequired {
		t.Fatalf("resultType = %v, want %q", got, resultTypeInputRequired)
	}
	requests, ok := result[inputRequestsKey].(map[string]any)
	if !ok {
		t.Fatalf("inputRequests is not an object: %#v", result[inputRequestsKey])
	}
	// MRTR server requirement 6 is satisfied by inputRequests alone, so exactly
	// one entry is the minimum AND the maximum this server emits: ze_execute
	// asks for one value.
	if len(requests) != 1 {
		t.Fatalf("inputRequests has %d entries, want exactly 1: %#v", len(requests), requests)
	}
	entry, ok := requests[inputKeyExecuteCommand].(map[string]any)
	if !ok {
		t.Fatalf("inputRequests[%q] missing or not an object: %#v", inputKeyExecuteCommand, requests)
	}
	if entry[elicitKeyMethod] != methodElicitationCreate {
		t.Errorf("method = %v, want %q", entry[elicitKeyMethod], methodElicitationCreate)
	}
	params, ok := entry[elicitKeyParams].(map[string]any)
	if !ok {
		t.Fatalf("params is not an object: %#v", entry[elicitKeyParams])
	}
	if params[elicitKeyMode] != elicitModeForm {
		t.Errorf("mode = %v, want %q", params[elicitKeyMode], elicitModeForm)
	}
	if msg, _ := params[elicitKeyMessage].(string); msg == "" {
		t.Error("message is empty; the prompt must say what is being asked for")
	}
	schema, ok := params[elicitKeyRequestedSchema].(map[string]any)
	if !ok {
		t.Fatalf("requestedSchema is not an object: %#v", params[elicitKeyRequestedSchema])
	}
	if err := validateElicitSchema(schema); err != nil {
		t.Errorf("emitted schema fails the server's own validator: %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, has := props[elicitFieldCommand]; !has {
		t.Errorf("requestedSchema has no %q property: %#v", elicitFieldCommand, props)
	}
}

// VALIDATES: AC-1 negative -- the marshaled InputRequiredResult contains no
// requestState key at all, not an empty or null one.
// PREVENTS: a future refactor emitting an unprotected requestState, which is
// exactly the cross-principal replay primitive umbrella R-3 feared. MRTR server
// requirement 3 makes requestState a MAY, and requirement 6's at-least-one-of
// rule is met by inputRequests, so omitting it is conformant.
func TestInputRequiredResultOmitsRequestState(t *testing.T) {
	result, err := inputRequiredForMissingCommand()
	if err != nil {
		t.Fatalf("inputRequiredForMissingCommand: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), requestStateKey) {
		t.Fatalf("marshaled result mentions %q: %s", requestStateKey, encoded)
	}
	if _, present := result[requestStateKey]; present {
		t.Fatalf("result carries a %q key", requestStateKey)
	}
}

// VALIDATES: AC-4 -- any request carrying a requestState is rejected, and the
// rejection names the failure class without echoing the supplied bytes.
// PREVENTS: a silent "treat as absent" path that a future requestState
// implementation could inherit without meeting MRTR server requirement 5.
func TestUnsolicitedRequestStateRejected(t *testing.T) {
	secret := "forged-Zm9yZ2Vk-blob"
	tests := []struct {
		name    string
		params  map[string]any
		wantErr bool
	}{
		{"absent", map[string]any{"name": "ze_execute"}, false},
		{"nil params", nil, false},
		{"string value", map[string]any{requestStateKey: secret}, true},
		{"empty string", map[string]any{requestStateKey: ""}, true},
		{"null value", map[string]any{requestStateKey: nil}, true},
		{"object value", map[string]any{requestStateKey: map[string]any{"a": secret}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectUnsolicitedRequestState(tt.params)
			if tt.wantErr && err == nil {
				t.Fatalf("params %#v accepted; want rejection", tt.params)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("params %#v rejected: %v", tt.params, err)
				}
				return
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("rejection echoes the supplied value: %v", err)
			}
			if !strings.Contains(err.Error(), requestStateKey) {
				t.Errorf("rejection does not name the offending field: %v", err)
			}
		})
	}
}

// VALIDATES: AC-5 and AC-6 -- the gate is form-mode SUPPORT, not the presence
// of the elicitation key. An empty object means form only; a url-only client
// does not support form mode.
// PREVENTS: sending a form-mode request to a url-only client, which
// client/elicitation forbids ("Servers MUST NOT send elicitation requests with
// modes that are not supported by the client").
func TestElicitCapabilityFormMode(t *testing.T) {
	tests := []struct {
		name string
		caps map[string]any
		want bool
	}{
		{"absent", map[string]any{}, false},
		{"nil capabilities", nil, false},
		{"empty object means form only", map[string]any{capabilityElicitation: map[string]any{}}, true},
		{"form declared", map[string]any{capabilityElicitation: map[string]any{elicitModeForm: map[string]any{}}}, true},
		{"url only", map[string]any{capabilityElicitation: map[string]any{elicitModeURL: map[string]any{}}}, false},
		{"form and url", map[string]any{capabilityElicitation: map[string]any{elicitModeForm: map[string]any{}, elicitModeURL: map[string]any{}}}, true},
		{"unknown mode only", map[string]any{capabilityElicitation: map[string]any{"telepathy": map[string]any{}}}, false},
		{"not an object", map[string]any{capabilityElicitation: true}, false},
		{"form is not an object", map[string]any{capabilityElicitation: map[string]any{elicitModeForm: true}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := elicitationFormSupported(tt.caps); got != tt.want {
				t.Fatalf("elicitationFormSupported(%#v) = %v, want %v", tt.caps, got, tt.want)
			}
			// The same verdict must survive the struct the handlers actually read.
			if got := parseClientCapabilities(tt.caps).ElicitForm; got != tt.want {
				t.Fatalf("parseClientCapabilities(%#v).ElicitForm = %v, want %v", tt.caps, got, tt.want)
			}
		})
	}
}

// VALIDATES: AC-2, AC-7, AC-8, AC-9 -- an accepted answer yields the value; an
// absent key re-asks; decline and cancel are terminal; unrecognized keys are
// ignored.
// PREVENTS: a decline being looped as though it were an omission, and an
// unexpected extra key failing a retry that carried the answer.
func TestResolveCommandFromInputResponses(t *testing.T) {
	accept := func(v string) map[string]any {
		return map[string]any{elicitKeyAction: elicitActionAccept, elicitKeyContent: map[string]any{elicitFieldCommand: v}}
	}
	tests := []struct {
		name        string
		responses   map[string]any
		wantValue   string
		wantOutcome inputOutcome
	}{
		{"nil map re-asks", nil, "", inputMissing},
		{"empty map re-asks", map[string]any{}, "", inputMissing},
		{"other keys only re-asks", map[string]any{"unrelated": accept("show version")}, "", inputMissing},
		{"accept yields the value", map[string]any{inputKeyExecuteCommand: accept("show bgp summary")}, "show bgp summary", inputAccepted},
		{"accept with extras still yields", map[string]any{
			inputKeyExecuteCommand: accept("show version"),
			"bogus":                map[string]any{elicitKeyAction: elicitActionAccept},
			"io.example/other":     "not even an object",
		}, "show version", inputAccepted},
		{"accept with empty content re-asks", map[string]any{inputKeyExecuteCommand: accept("")}, "", inputMissing},
		{"accept with no content re-asks", map[string]any{inputKeyExecuteCommand: map[string]any{elicitKeyAction: elicitActionAccept}}, "", inputMissing},
		{"decline is terminal", map[string]any{inputKeyExecuteCommand: map[string]any{elicitKeyAction: elicitActionDecline}}, "", inputDeclined},
		{"cancel is terminal", map[string]any{inputKeyExecuteCommand: map[string]any{elicitKeyAction: elicitActionCancel}}, "", inputCanceled},
		{"unknown action is malformed", map[string]any{inputKeyExecuteCommand: map[string]any{elicitKeyAction: "maybe"}}, "", inputMalformed},
		{"missing action is malformed", map[string]any{inputKeyExecuteCommand: map[string]any{}}, "", inputMalformed},
		{"entry is not an object", map[string]any{inputKeyExecuteCommand: "show version"}, "", inputMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, outcome := resolveElicitedValue(tt.responses, inputKeyExecuteCommand, elicitFieldCommand)
			if outcome != tt.wantOutcome {
				t.Fatalf("outcome = %v, want %v", outcome, tt.wantOutcome)
			}
			if value != tt.wantValue {
				t.Fatalf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

// VALIDATES: AC-10 -- no method other than the three the specification permits
// can carry an InputRequiredResult, and the guard rewrites one that tries.
// PREVENTS: an input-required outcome leaking onto tasks/*, tools/list or
// server/discover, which "Servers MUST NOT send InputRequiredResult responses
// on any other client requests" forbids.
func TestInputRequiredOnlyOnSupportedMethods(t *testing.T) {
	// The spec's supported-requests table, verbatim.
	for _, method := range []string{methodPromptsGet, methodResourcesRead, methodToolsCall} {
		if !permitsInputRequired(method) {
			t.Errorf("permitsInputRequired(%q) = false, want true", method)
		}
	}
	// Every other method this server dispatches must refuse it.
	dispatched := []string{
		methodServerDiscover, methodToolsList, methodTasksGet,
		methodTasksUpdate, methodTasksCancel, methodResourcesList, initializeMethod,
		// The two methods MCP 2026-07-28 removed. They are no longer dispatched,
		// so an interim result must be refused on them for the additional reason
		// that they do not exist -- kept in the list so a reinstatement cannot
		// quietly gain permission it never had.
		"tasks/list", "tasks/result",
		"notifications/progress", "",
	}
	for _, method := range dispatched {
		if permitsInputRequired(method) {
			t.Errorf("permitsInputRequired(%q) = true, want false", method)
		}
	}

	srv, err := NewStreamable(StreamableConfig{})
	if err != nil {
		t.Fatalf("NewStreamable: %v", err)
	}
	defer srv.Close()
	interim, err := inputRequiredForMissingCommand()
	if err != nil {
		t.Fatalf("inputRequiredForMissingCommand: %v", err)
	}
	for _, method := range dispatched {
		t.Run("guard/"+method, func(t *testing.T) {
			got := srv.guardInputRequired(method, srv.ok(nil, interim))
			if got.Error == nil {
				t.Fatalf("method %q emitted an input-required result unguarded: %#v", method, got.Result)
			}
			if got.Error.Code != rpcInternalError {
				t.Errorf("code = %d, want %d", got.Error.Code, rpcInternalError)
			}
		})
	}
	// The permitted method passes through untouched.
	passed := srv.guardInputRequired(methodToolsCall, srv.ok(nil, interim))
	if passed.Error != nil {
		t.Fatalf("tools/call input-required was rejected: %v", passed.Error)
	}
	fields, ok := passed.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is not an object: %#v", passed.Result)
	}
	if fields[resultTypeKey] != resultTypeInputRequired {
		t.Fatalf("resultType = %v, want %q", fields[resultTypeKey], resultTypeInputRequired)
	}
}

// VALIDATES: AC-11 -- every elicitation this server can emit asks for a
// non-credential value, and the emitter refuses a credential-shaped one.
// PREVENTS: "Servers MUST NOT use form mode elicitation to request sensitive
// information such as passwords, API keys, access tokens, or payment
// credentials" being satisfied only by inspection.
func TestEmittableElicitationsRequestNoSecrets(t *testing.T) {
	// The complete emittable set: every production caller of newElicitRequest.
	// One member today; a second one added without a row here fails the length
	// assertion below rather than slipping past the prohibition.
	emittable := []func() (map[string]any, error){inputRequiredForMissingCommand}
	if len(emittable) != 1 {
		t.Fatalf("emittable set has %d members; add each new one to this table", len(emittable))
	}
	for _, build := range emittable {
		result, err := build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		requests, _ := result[inputRequestsKey].(map[string]any)
		for key, raw := range requests {
			entry, _ := raw.(map[string]any)
			params, _ := entry[elicitKeyParams].(map[string]any)
			schema, _ := params[elicitKeyRequestedSchema].(map[string]any)
			props, _ := schema["properties"].(map[string]any)
			for field := range props {
				if elicitFieldIsSecret(field) {
					t.Errorf("inputRequests[%q] asks for credential-shaped field %q", key, field)
				}
			}
		}
	}

	// The emitter refuses one, so the prohibition holds for code not yet written.
	for _, field := range []string{"password", "apiKey", "access-token", "Secret", "user_passphrase"} {
		schema := map[string]any{
			"type":       "object",
			"properties": map[string]any{field: map[string]any{"type": "string"}},
		}
		if _, err := newElicitRequest("give me it", schema); !errors.Is(err, ErrElicitSchemaInvalid) {
			t.Errorf("newElicitRequest accepted credential field %q (err=%v)", field, err)
		}
	}
}

// VALIDATES: AC-1 wiring -- a tools/call for ze_execute with no command, from a
// client declaring form-mode elicitation, comes back as an InputRequiredResult
// through the real dispatch path rather than only from the constructor.
// PREVENTS: the constructor existing while the handler still hard-fails, which
// is the library-without-wiring defect ai/rules/wiring-completeness.md names.
func TestZeExecutePromptsWhenFormModeDeclared(t *testing.T) {
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			t.Fatal("dispatch ran without a command")
			return nil, errDispatchShouldNotRun
		},
		caps: formCaps(t),
	}
	result := toolHandlers["ze_execute"](runner, json.RawMessage(`{}`))
	if result[resultTypeKey] != resultTypeInputRequired {
		t.Fatalf("resultType = %v, want %q: %#v", result[resultTypeKey], resultTypeInputRequired, result)
	}
	if _, isErr := result["isError"]; isErr {
		t.Errorf("an input-required outcome must not be a tool error: %#v", result)
	}
}

// VALIDATES: AC-5 -- a client that declared no elicitation capability gets the
// missing-argument error result, and no inputRequests entry is emitted.
// PREVENTS: breaking MRTR server requirement 7, "Servers MUST NOT send an
// inputRequests that the client has not declared support for in its
// capabilities".
func TestZeExecuteWithoutFormModeReturnsMissingArgument(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps map[string]any
	}{
		{"no capabilities", map[string]any{}},
		{"url mode only", map[string]any{capabilityElicitation: map[string]any{elicitModeURL: map[string]any{}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &server{
				dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
					t.Fatal("dispatch ran without a command")
					return nil, errDispatchShouldNotRun
				},
				caps: parseClientCapabilities(tt.caps),
			}
			result := toolHandlers["ze_execute"](runner, json.RawMessage(`{"command":""}`))
			if result[resultTypeKey] == resultTypeInputRequired {
				t.Fatalf("prompted a client without form-mode support: %#v", result)
			}
			if isErr, _ := result["isError"].(bool); !isErr {
				t.Fatalf("want a tool error result, got %#v", result)
			}
			if _, has := result[inputRequestsKey]; has {
				t.Errorf("result carries inputRequests: %#v", result)
			}
		})
	}
}

// VALIDATES: AC-2, AC-8, AC-9 wiring -- an accepted answer on the retry is
// dispatched; a declined one is a terminal error naming the outcome; extra keys
// do not disturb either.
// PREVENTS: the retry being answered with a second prompt (which would loop a
// user who already answered) or a decline being dispatched as an empty command.
func TestZeExecuteRetryOutcomes(t *testing.T) {
	var dispatched string
	newRunner := func(responses map[string]any) *server {
		return &server{
			dispatch: func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
				dispatched = cmd
				return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"ok":true}`)), nil
			},
			caps:           formCaps(t),
			inputResponses: responses,
		}
	}

	t.Run("accept dispatches", func(t *testing.T) {
		dispatched = ""
		runner := newRunner(map[string]any{
			inputKeyExecuteCommand: map[string]any{
				elicitKeyAction:  elicitActionAccept,
				elicitKeyContent: map[string]any{elicitFieldCommand: "show bgp summary"},
			},
			"unrecognized": map[string]any{elicitKeyAction: elicitActionAccept},
		})
		result := toolHandlers["ze_execute"](runner, json.RawMessage(`{}`))
		if dispatched != "show bgp summary" {
			t.Fatalf("dispatched %q, want %q", dispatched, "show bgp summary")
		}
		if result[resultTypeKey] == resultTypeInputRequired {
			t.Fatalf("re-asked after an accepted answer: %#v", result)
		}
	})

	t.Run("omitted key re-asks", func(t *testing.T) {
		dispatched = ""
		result := toolHandlers["ze_execute"](newRunner(map[string]any{}), json.RawMessage(`{}`))
		if result[resultTypeKey] != resultTypeInputRequired {
			t.Fatalf("resultType = %v, want a fresh prompt: %#v", result[resultTypeKey], result)
		}
		if dispatched != "" {
			t.Fatalf("dispatched %q on a retry with no answer", dispatched)
		}
	})

	for _, tt := range []struct {
		action string
		needle string
	}{
		{elicitActionDecline, "declined"},
		{elicitActionCancel, "canceled"},
	} {
		t.Run(tt.action+" is terminal", func(t *testing.T) {
			dispatched = ""
			runner := newRunner(map[string]any{
				inputKeyExecuteCommand: map[string]any{elicitKeyAction: tt.action},
			})
			result := toolHandlers["ze_execute"](runner, json.RawMessage(`{}`))
			if result[resultTypeKey] == resultTypeInputRequired {
				t.Fatalf("%s produced a re-ask: %#v", tt.action, result)
			}
			if isErr, _ := result["isError"].(bool); !isErr {
				t.Fatalf("%s did not produce a tool error: %#v", tt.action, result)
			}
			if dispatched != "" {
				t.Fatalf("dispatched %q after a %s", dispatched, tt.action)
			}
			text := toolResultText(t, result)
			if !strings.Contains(text, tt.needle) {
				t.Errorf("error text %q does not name the outcome %q", text, tt.needle)
			}
		})
	}
}

// Client capability literals for the descriptor test below. capsNone
// (streamable_test.go) is the "declared nothing" form and reads as NO form
// mode; these two are the declaring shapes, spelled the way a client sends
// them.
const (
	capsElicitForm    = `{"elicitation":{}}`
	capsElicitURLOnly = `{"elicitation":{"url":{}}}`
)

// TestExecuteDescriptorMatchesElicitationCapability is the regression test for
// the defect that made this whole phase unreachable.
//
// Phase 1 published `"required": ["command"]` on ze_execute unconditionally,
// when elicitation had been deleted and the argument really was mandatory for
// everyone. Phase 2 restored elicitation and did not revert the contract. The
// handler worked -- a form-declaring client that omitted `command` did get
// resultType "input_required" -- but tools/list told every client the argument
// was mandatory, so a schema-validating host would never construct the call and
// a model reading the descriptor would never try. The feature existed and no
// client could reach it. A behavior the advertised interface forbids does not
// ship.
//
// VALIDATES: the PUBLISHED inputSchema tracks the same capability the handler
// branches on. A client declaring form-mode elicitation is not told `command`
// is required; a client that declared none, or url mode only, is.
// PREVENTS: the descriptor and askForCommand drifting apart again in either
// direction -- an unconditional `required` (the original defect, which hides
// the feature), and an unconditional absence (which would promise a prompt to a
// client this server may not prompt, MRTR server requirement 7).
func TestExecuteDescriptorMatchesElicitationCapability(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	for _, tt := range []struct {
		name         string
		caps         string
		wantRequired bool
	}{
		{"form mode declared", capsElicitForm, false},
		{"no capabilities", capsNone, true},
		{"url mode only", capsElicitURLOnly, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status, parsed := postMCP(t, hs, methodToolsList, tt.caps, "")
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			schema := publishedInputSchema(t, resultOf(t, parsed), toolNameExecute)

			// The property is always advertised; only its obligation moves.
			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("inputSchema carries no properties object: %v", schema)
			}
			if _, present := properties[elicitFieldCommand]; !present {
				t.Fatalf("inputSchema.properties has no %q: %v", elicitFieldCommand, properties)
			}

			required := requiredNames(t, schema)
			if got := slices.Contains(required, elicitFieldCommand); got != tt.wantRequired {
				t.Fatalf("inputSchema.required contains %q = %v, want %v (required = %v)",
					elicitFieldCommand, got, tt.wantRequired, required)
			}
		})
	}

	// The gate must not have mutated the package-level descriptor: it is shared
	// by every concurrent request, so a write here would leak one client's
	// capability verdict onto the next.
	for _, tool := range handcraftedTools {
		if name, _ := tool[toolKeyName].(string); name != toolNameExecute {
			continue
		}
		schema, _ := tool[schemaKeyInputSchema].(map[string]any)
		if _, mutated := schema[schemaKeyRequired]; mutated {
			t.Fatalf("gateExecuteCommandRequired wrote through to the shared descriptor: %v", schema)
		}
	}
}

// publishedInputSchema pulls one named tool's inputSchema out of a tools/list
// result, so the assertion is on what a client actually receives rather than on
// the package-level literal.
func publishedInputSchema(t *testing.T, result map[string]any, name string) map[string]any {
	t.Helper()
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result carries no tools array: %v", result)
	}
	for _, raw := range tools {
		tool, isObject := raw.(map[string]any)
		if !isObject {
			continue
		}
		if got, _ := tool[toolKeyName].(string); got != name {
			continue
		}
		schema, hasSchema := tool[schemaKeyInputSchema].(map[string]any)
		if !hasSchema {
			t.Fatalf("%s carries no %s object: %v", name, schemaKeyInputSchema, tool)
		}
		return schema
	}
	t.Fatalf("%s is absent from the published tool list", name)
	return nil
}

// requiredNames reads a JSON Schema `required` array off the wire, tolerating
// its absence (which is what "nothing is required" looks like) and failing on a
// shape a client could not read.
func requiredNames(t *testing.T, schema map[string]any) []string {
	t.Helper()
	raw, present := schema[schemaKeyRequired]
	if !present {
		return nil
	}
	entries, isList := raw.([]any)
	if !isList {
		t.Fatalf("inputSchema.required = %v (%T), want an array", raw, raw)
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, isString := entry.(string)
		if !isString {
			t.Fatalf("inputSchema.required holds a non-string entry %v (%T)", entry, entry)
		}
		out = append(out, name)
	}
	return out
}

// toolResultText pulls the first text block out of a tool result map.
func toolResultText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("result carries no content: %#v", result)
	}
	text, _ := content[0]["text"].(string)
	return text
}
