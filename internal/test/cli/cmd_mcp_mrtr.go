// Design: docs/architecture/testing/ci-format.md -- MCP test client
// Overview: cmd_mcp_client.go -- request construction, headers, transport
// Related: cmd_mcp_calls.go -- the tools/* and tasks/* helpers that ride send()

// Multi Round-Trip Requests, client half.
//
// Written from the specification text (basic/patterns/mrtr and
// client/elicitation), NOT from Ze's server code, so that a functional test
// asserting on this behavior is an independent reading of the protocol rather
// than a restatement of the implementation under test.
//
// The loop: send the request; if the result is `input_required`, construct the
// inputs its `inputRequests` asks for, then retry THE ORIGINAL REQUEST carrying
// `inputResponses` under a new JSON-RPC id. Nothing is echoed back to the
// server except the answers -- when a result carries no `requestState`, "the
// client MUST NOT include one in the retry".

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// send issues a request and drives the multi round-trip loop to a final result.
//
// Client requirement 1: "If a client receives an InputRequiredResult that
// contains the inputRequests field, the client MUST construct the requested
// inputs before retrying the original request."
//
// Client requirement 3: "The JSON-RPC id MUST be different between the initial
// request and the retry, as they are independent requests." Satisfied by
// construction: every exchange takes the next id from the counter.
func (c *mcpClient) send(method string, params map[string]any) (json.RawMessage, error) {
	attempt := params
	for range maxInputRounds {
		result, err := c.sendOnce(method, attempt)
		if err != nil {
			return nil, err
		}
		resultType, fields, err := classifyResult(result, c.declareTasks)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", method, err)
		}
		if resultType != resultTypeInputRequired {
			return result, nil
		}
		if !c.elicit.queued {
			return nil, fmt.Errorf("%s: %w", method, errInputRequired)
		}
		responses, err := c.answerInputRequests(fields)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", method, err)
		}
		// The retry is the ORIGINAL request plus inputResponses. Rebuilt from
		// `params` every round rather than mutated in place, so a second round
		// cannot inherit the previous round's answers.
		retry := map[string]any{}
		maps.Copy(retry, params)
		retry[keyInputResponses] = responses
		attempt = retry
	}
	return nil, fmt.Errorf("%s: server still required input after %d attempts", method, maxInputRounds)
}

// elicitDirective handles the elicit-* directives, reporting whether the line
// was one of them.
//
// The answer is QUEUED rather than consumed: a server may prompt on more than
// one round (server requirement 8), and a test that answered once should not
// silently stop answering.
func (c *mcpClient) elicitDirective(line string) (bool, error) {
	if rest, ok := strings.CutPrefix(line, "elicit-answer "); ok {
		action, value, _ := strings.Cut(strings.TrimSpace(rest), " ")
		value = strings.TrimSpace(value)
		switch action {
		case elicitAccept:
			if value == "" {
				return true, fmt.Errorf("elicit-answer accept needs a value, got %q", line)
			}
			c.elicit.queued, c.elicit.action, c.elicit.value = true, elicitAccept, value
		case elicitDecline, elicitCancel:
			c.elicit.queued, c.elicit.action, c.elicit.value = true, action, ""
		case "omit":
			c.elicit.queued, c.elicit.action, c.elicit.value = true, "", ""
		case "none":
			c.elicit = elicitPlan{}
		default:
			return true, fmt.Errorf("elicit-answer: unknown action %q, want accept, decline, cancel, omit or none", action)
		}
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "elicit-extra "); ok {
		key := strings.TrimSpace(rest)
		if key == probeOmit {
			key = ""
		}
		c.elicit.extra = key
		return true, nil
	}
	return false, nil
}

// answerInputRequests builds the inputResponses object for one retry.
//
// Every entry the server asked for is answered from the queued plan. The plan's
// "omit" form answers nothing, which is how a test drives the server's re-ask
// path; an unexpected extra key is added when the plan names one, because a
// server must tolerate unrecognized InputResponses parameters.
func (c *mcpClient) answerInputRequests(result map[string]any) (map[string]any, error) {
	requests, ok := result[keyInputRequests].(map[string]any)
	if !ok || len(requests) == 0 {
		// Requirement 1's other half: with no inputRequests the client MAY
		// retry immediately. There is nothing to answer, so retry bare.
		return map[string]any{}, nil
	}
	responses := map[string]any{}
	if c.elicit.extra != "" {
		responses[c.elicit.extra] = map[string]any{keyAction: elicitAccept, keyContent: map[string]any{}}
	}
	if c.elicit.action == "" {
		// "omit": answer nothing the server asked for. The extra key, if any,
		// still rides, which is how a test proves it is ignored rather than
		// mistaken for the answer.
		return responses, nil
	}
	// Sorted for determinism: a test asserting on output must not depend on Go
	// map iteration order.
	keys := make([]string, 0, len(requests))
	for key := range requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		request, ok := requests[key].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("inputRequests[%q] is not a request object", key)
		}
		answer, err := c.answerElicitRequest(key, request)
		if err != nil {
			return nil, err
		}
		responses[key] = answer
	}
	return responses, nil
}

// answerElicitRequest answers one elicitation/create request.
//
// It checks the mode the server asked in against what this client declared:
// "Servers MUST NOT send elicitation requests with modes that are not supported
// by the client." A client that answered a mode it never declared could not
// prove the server honors that, so the check lives here rather than in a test
// assertion.
func (c *mcpClient) answerElicitRequest(key string, request map[string]any) (map[string]any, error) {
	params, _ := request[keyParams].(map[string]any)
	mode, _ := params[keyMode].(string)
	if mode == "" {
		// The specification lets a server "omit the `mode` field for form mode
		// elicitation requests", so an absent mode reads as form.
		mode = mcpModeForm
	}
	if !c.supportsElicitMode(mode) {
		return nil, fmt.Errorf("server asked for %q mode elicitation on inputRequests[%q], which this client did not declare", mode, key)
	}

	switch c.elicit.action {
	case elicitDecline:
		return map[string]any{keyAction: elicitDecline}, nil
	case elicitCancel:
		return map[string]any{keyAction: elicitCancel}, nil
	case elicitAccept:
		field, err := singleSchemaProperty(params)
		if err != nil {
			return nil, fmt.Errorf("inputRequests[%q]: %w", key, err)
		}
		return map[string]any{
			keyAction:  elicitAccept,
			keyContent: map[string]any{field: c.elicit.value},
		}, nil
	default:
		return nil, fmt.Errorf("unknown elicit-answer action %q", c.elicit.action)
	}
}

// singleSchemaProperty returns the one property name a requestedSchema
// declares. The client reads the field name off the schema rather than assuming
// one, which is what keeps it a generic MCP client instead of a Ze-shaped one.
func singleSchemaProperty(params map[string]any) (string, error) {
	schema, ok := params[keyRequestedSchema].(map[string]any)
	if !ok {
		return "", errors.New("requestedSchema is missing or not an object")
	}
	props, ok := schema[keyProperties].(map[string]any)
	if !ok || len(props) == 0 {
		return "", errors.New("requestedSchema declares no properties")
	}
	if len(props) != 1 {
		return "", fmt.Errorf("requestedSchema declares %d properties; this client answers single-property forms only", len(props))
	}
	for name := range props {
		return name, nil
	}
	return "", errors.New("unreachable: properties is non-empty")
}

// parseElicitCapability turns the --elicit flag value into the capability
// object sent on every request.
//
// declared says whether the `elicitation` member is sent at all; it is separate
// from the map because an EMPTY declared object is a real declaration ("an
// empty capabilities object is equivalent to declaring support for `form` mode
// only"), so len(caps)==0 cannot stand in for "absent".
//
// "" declares nothing. "empty" declares `{}`. Anything else is a
// comma-separated mode list.
func parseElicitCapability(spec string) (caps map[string]any, declared bool, err error) {
	if spec == "" {
		return nil, false, nil
	}
	if spec == elicitCapabilityEmpty {
		return map[string]any{}, true, nil
	}
	caps = map[string]any{}
	for _, mode := range splitCommaList(spec) {
		switch mode {
		case mcpModeForm, mcpModeURL:
			caps[mode] = map[string]any{}
		default:
			return nil, false, fmt.Errorf("unknown elicitation mode %q, want %q, %q, %q or a comma-separated pair",
				mode, mcpModeForm, mcpModeURL, elicitCapabilityEmpty)
		}
	}
	if len(caps) == 0 {
		return nil, false, fmt.Errorf("--elicit %q names no mode", spec)
	}
	return caps, true, nil
}

// elicitCapabilityEmpty is the --elicit value that declares `elicitation: {}`,
// the form-mode-only shape.
const elicitCapabilityEmpty = "empty"

// splitCommaList splits a comma-separated flag value, dropping empty fields.
func splitCommaList(spec string) []string {
	var out []string
	var field textbuf.Buffer
	flush := func() {
		if field.Len() > 0 {
			out = append(out, field.String())
			field.Reset()
		}
	}
	for i := range len(spec) {
		if spec[i] == ',' || spec[i] == ' ' {
			flush()
			continue
		}
		if err := field.WriteByte(spec[i]); err != nil {
			return out
		}
	}
	flush()
	return out
}
