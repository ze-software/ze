// Design: docs/architecture/testing/ci-format.md -- MCP test client
// Overview: cmd_mcp.go -- the ze-test mcp command and its stdin directives
// Related: cmd_mcp_client.go -- the send() transport these method helpers ride on

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func (c *mcpClient) callTool(tool string, args json.RawMessage) (string, error) {
	result, err := c.send(methodToolsCall, map[string]any{
		keyName:           tool,
		toolCallArguments: args,
	})
	if err != nil {
		return "", err
	}
	return extractText(result)
}

func (c *mcpClient) execute(command string) (string, error) {
	args, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		return "", fmt.Errorf("marshal ze_execute arguments: %w", err)
	}
	return c.callTool("ze_execute", args)
}

// extractText pulls the first text block out of a CallToolResult, turning a
// tool-level error into a Go error so a directive fails loudly.
func extractText(result json.RawMessage) (string, error) {
	var toolResult map[string]any
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return "", fmt.Errorf("parse tool result: %w", err)
	}
	if isErr, ok := toolResult["isError"].(bool); ok && isErr {
		if content, ok := toolResult["content"].([]any); ok && len(content) > 0 {
			if entry, ok := content[0].(map[string]any); ok {
				if text, ok := entry["text"].(string); ok {
					var b textbuf.Buffer
					return "", errors.New(b.Str("command error: ").Str(text).String())
				}
			}
		}
		return "", errCommandErrorNoDetail
	}
	content, ok := toolResult["content"].([]any)
	if !ok || len(content) == 0 {
		return "", nil
	}
	entry, ok := content[0].(map[string]any)
	if !ok {
		return "", nil
	}
	text, _ := entry["text"].(string)
	return text, nil
}

func (c *mcpClient) waitEstablished(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.execute("show bgp peer list")
		if err == nil && strings.Contains(strings.ToLower(result), "established") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("wait-established: no peer reached Established within %v", timeout)
}

func (c *mcpClient) waitPeers(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.execute("show bgp peer list")
		if err == nil && !strings.Contains(result, `"peers":{}`) && strings.Contains(result, `"peers":{`) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("wait-peers: no peer configured within %v", timeout)
}

func (c *mcpClient) waitTool(name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.send(methodToolsList, map[string]any{})
		if err == nil {
			var listed struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			if json.Unmarshal(result, &listed) == nil {
				for _, tool := range listed.Tools {
					if tool.Name == name {
						return nil
					}
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("wait-tool: %q absent from tools/list after %v", name, timeout)
}

// toolsSnapshot is one tools/list answer, kept in the two forms the stability
// check needs. The first form is the RAW bytes of the `tools` array. The second
// is the tool names in wire order, for the drift diagnostic.
//
// The raw bytes are the assertion. This check used to compare names alone, and
// names are weaker than the acceptance criterion the check stands for
// (spec-mcp2026-4-caching-apps AC-6: "byte-identical tools array, including
// every action enum and every description string"). A wobble in an action enum,
// a description, or a nested `_meta` would leave the name sequence identical.
// That wobble would still defeat the client caching this revision enables, and
// the LLM prompt-cache hits that motivate the SHOULD. That is the whole point
// of the requirement.
type toolsSnapshot struct {
	raw   []byte
	names []string
}

// toolsList returns one tools/list answer. Order is the point. MCP 2026-07-28
// server/tools says "Servers SHOULD return tools in a deterministic order (i.e.,
// the same ordering across requests when the underlying set of tools has not
// changed)". So nothing here is sorted and the array is never re-marshaled.
// A re-encode would canonicalize away exactly the drift this check exists to
// find.
func (c *mcpClient) toolsList() (toolsSnapshot, error) {
	result, err := c.send(methodToolsList, map[string]any{})
	if err != nil {
		return toolsSnapshot{}, err
	}
	var listed struct {
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(result, &listed); err != nil {
		return toolsSnapshot{}, fmt.Errorf("tools/list: decode result: %w", err)
	}
	if len(listed.Tools) == 0 {
		return toolsSnapshot{}, errToolsListEmpty
	}
	var named []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(listed.Tools, &named); err != nil {
		return toolsSnapshot{}, fmt.Errorf("tools/list: decode tools: %w", err)
	}
	names := make([]string, len(named))
	for i, tool := range named {
		names[i] = tool.Name
	}
	return toolsSnapshot{raw: listed.Tools, names: names}, nil
}

// errToolsListEmpty guards the comparison below: two empty answers are
// byte-identical, so an empty tools array would make this check pass without
// ever comparing a tool.
var errToolsListEmpty = errors.New("tools/list: the result carries no tools array; a stability check over an empty list asserts nothing")

// toolsDigest is the first 8 bytes of the SHA-256 of a tools array, hex encoded.
// Printed on the stable line so a .ci can assert that a byte-level comparison
// actually happened, and so two runs can be compared by eye.
func toolsDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return textbuf.StringHex(sum[:8])
}

// toolsOrderStable calls tools/list `calls` times against an unchanged daemon
// and reports whether every call returned a BYTE-IDENTICAL tools array.
//
// The comparison lives here rather than in the .ci. A .ci can only match
// patterns, and Go's RE2 has no backreference, so "these two responses are
// identical" is not expressible as an expectation. A comparison of two server
// responses is client work in any case, the same way wait-established is.
//
// Byte identity, not name identity. spec-mcp2026-4-caching-apps AC-6 asks for a
// "byte-identical tools array, including every action enum and every
// description string". This check used to compare only the name sequence, which
// TestToolOrderDeterministic already covers at generation level. No test covered
// the same property ON THE WIRE. There a wobbling action enum (built from a map
// range) or a drifting description defeats the client caching this revision
// enables, while every tool name stays put.
//
// On drift the line names what diverged. It names the call, the first differing
// tool index and both name sequences when the names moved. It names the byte
// offset plus both digests when only the payload moved. Names alone can never
// report that second case.
func (c *mcpClient) toolsOrderStable(calls int) (string, error) {
	if calls < 2 {
		return "", fmt.Errorf("tools-order-stable needs at least 2 calls, got %d", calls)
	}
	first, err := c.toolsList()
	if err != nil {
		return "", err
	}
	var b textbuf.Buffer
	for call := 2; call <= calls; call++ {
		next, nextErr := c.toolsList()
		if nextErr != nil {
			return "", nextErr
		}
		if bytes.Equal(first.raw, next.raw) {
			continue
		}
		b.Str("tools-order stable=false calls=").Int(int64(calls))
		b.Str(" diverged-on-call=").Int(int64(call))
		if index, same := firstDifference(first.names, next.names); !same {
			b.Str(" index=").Int(int64(index))
			b.Str(" first=").Str(textbuf.Join(first.names, ","))
			b.Str(" differing=").Str(textbuf.Join(next.names, ","))
			return b.String(), nil
		}
		// Same tools, same order, different bytes: an enum, a description or a
		// nested object moved. Report where, because the name sequences are
		// identical and printing them twice would say nothing.
		b.Str(" names=identical byte-offset=").Int(int64(firstByteDifference(first.raw, next.raw)))
		b.Str(" first-digest=").Str(toolsDigest(first.raw))
		b.Str(" differing-digest=").Str(toolsDigest(next.raw))
		return b.String(), nil
	}
	b.Str("tools-order stable=true calls=").Int(int64(calls))
	b.Str(" tools=").Int(int64(len(first.names)))
	b.Str(" digest=").Str(toolsDigest(first.raw))
	return b.String(), nil
}

// firstByteDifference reports the offset at which two payloads first differ, or
// the length of the shorter one when one is a prefix of the other.
func firstByteDifference(a, b []byte) int {
	limit := min(len(a), len(b))
	for i := range limit {
		if a[i] != b[i] {
			return i
		}
	}
	return limit
}

// firstDifference reports the index at which two tool-name sequences diverge,
// and whether they are identical. A length change counts as a difference at the
// first missing position.
func firstDifference(a, b []string) (int, bool) {
	limit := min(len(a), len(b))
	for i := range limit {
		if a[i] != b[i] {
			return i, false
		}
	}
	if len(a) != len(b) {
		return limit, false
	}
	return 0, true
}

// taskCall makes an ORDINARY tools/call and requires the server to answer with a
// task handle.
//
// There is no `task` member in the params, because there is no client-side
// opt-in any more. MCP 2026-07-28 moved tasks onto the
// io.modelcontextprotocol/tasks extension. There the SERVER decides that a call
// runs in the background, from the command's `ze:task-support` annotation. The
// client declares the extension once per request (--tasks) and handles
// whichever result shape arrives.
//
// The resultType is checked, not just the taskId. A server that regressed to a
// synchronous answer would return a valid `complete` result with no taskId. The
// check "carries no taskId" alone would report that as a malformed response
// rather than as the missing feature it is.
func (c *mcpClient) taskCall(tool string, args json.RawMessage) (string, error) {
	result, err := c.send(methodToolsCall, map[string]any{
		keyName:           tool,
		toolCallArguments: args,
	})
	if err != nil {
		return "", err
	}
	var created map[string]any
	if err := json.Unmarshal(result, &created); err != nil {
		return "", fmt.Errorf("parse CreateTaskResult: %w", err)
	}
	if got, _ := created["resultType"].(string); got != "task" {
		return "", fmt.Errorf("tools/call %s: resultType = %q, want %q (server did not create a task): %s",
			tool, got, "task", compactJSON(result))
	}
	taskID, _ := created[keyTaskID].(string)
	if taskID == "" {
		return "", fmt.Errorf("CreateTaskResult carries no taskId: %s", compactJSON(result))
	}
	return taskID, nil
}

// taskCallSync makes the same ordinary tools/call but requires the server NOT to
// create a task, returning the synchronous result text instead.
//
// This is the positive half of the no-extension and forbidden-command
// assertions. An assertion of "no task handle" alone would pass against a
// server that failed the request, so the test also has to prove the work ran.
func (c *mcpClient) taskCallSync(tool string, args json.RawMessage) (string, error) {
	result, err := c.send(methodToolsCall, map[string]any{
		keyName:           tool,
		toolCallArguments: args,
	})
	if err != nil {
		return "", err
	}
	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		return "", fmt.Errorf("parse tools/call result: %w", err)
	}
	if rt, _ := got["resultType"].(string); rt != "complete" {
		return "", fmt.Errorf("tools/call %s: resultType = %q, want %q (server created a task it should not have): %s",
			tool, rt, "complete", compactJSON(result))
	}
	if id, _ := got[keyTaskID].(string); id != "" {
		return "", fmt.Errorf("tools/call %s: synchronous result carries a taskId %q: %s", tool, id, compactJSON(result))
	}
	return extractText(result)
}

func (c *mcpClient) taskGet(taskID string) (string, error) {
	result, err := c.send("tasks/get", map[string]any{keyTaskID: taskID})
	if err != nil {
		return "", err
	}
	var task map[string]any
	if err := json.Unmarshal(result, &task); err != nil {
		return "", fmt.Errorf("parse tasks/get result: %w", err)
	}
	status, _ := task["status"].(string)
	return status, nil
}

// taskResultText returns the tool output a TERMINAL task carries on tasks/get.
//
// tasks/result is gone (changelog Major change 6 replaced it with polling), so
// the payload rides on the same call that reports the status. A terminal task
// carries `result` when it completed and `error` when it failed.
func (c *mcpClient) taskResultText(taskID string) (string, error) {
	raw, err := c.send("tasks/get", map[string]any{keyTaskID: taskID})
	if err != nil {
		return "", err
	}
	var task map[string]any
	if err := json.Unmarshal(raw, &task); err != nil {
		return "", fmt.Errorf("parse tasks/get result: %w", err)
	}
	status, _ := task["status"].(string)
	if !taskTerminalStates[status] {
		return "", fmt.Errorf("task %s is %q, not terminal: no result to read", taskID, status)
	}
	inner, ok := task["result"]
	if !ok {
		if msg, ok := task["error"].(string); ok {
			return "", fmt.Errorf("task %s ended %s: %s", taskID, status, msg)
		}
		return "", fmt.Errorf("terminal task %s carries neither result nor error: %s", taskID, compactJSON(raw))
	}
	encoded, err := json.Marshal(inner)
	if err != nil {
		return "", fmt.Errorf("re-encode task result: %w", err)
	}
	return extractText(encoded)
}

// taskCancel sends tasks/cancel and requires the empty acknowledgment the
// extension specifies.
//
// The shape is checked here rather than discarded, which is what the reply used
// to be. MCP 2026-07-28 ext-tasks tells a server to "Acknowledge cancellation
// requests with an empty result". A client that discarded the result cannot
// tell an empty acknowledgment from a server that volunteers a status field. A
// status in the acknowledgment would also be a stale snapshot. Cancellation is
// cooperative, so tasks/get is the method that answers "what state is it in
// now".
func (c *mcpClient) taskCancel(taskID string) (string, error) {
	raw, err := c.send("tasks/cancel", map[string]any{keyTaskID: taskID})
	if err != nil {
		return "", err
	}
	if err := requireEmptyAck("tasks/cancel", raw); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	return b.Str("task-cancel acknowledged task=").Str(taskID).String(), nil
}

// taskUpdate sends tasks/update and requires the empty acknowledgement the
// extension specifies. `responses` is sent verbatim as `inputResponses`, so a
// test can prove unknown keys are tolerated rather than rejected.
func (c *mcpClient) taskUpdate(taskID string, responses map[string]any) (string, error) {
	params := map[string]any{keyTaskID: taskID}
	if responses != nil {
		params["inputResponses"] = responses
	}
	raw, err := c.send("tasks/update", params)
	if err != nil {
		return "", err
	}
	// The acknowledgement is empty apart from the two envelope fields every
	// result carries. Anything else would mean the server acted on the
	// inputResponses, which it has nothing to match them against.
	if err := requireEmptyAck("tasks/update", raw); err != nil {
		return "", err
	}
	var b textbuf.Buffer
	return b.Str("task-update acknowledged task=").Str(taskID).String(), nil
}

// requireEmptyAck fails unless a result carries nothing beyond the two envelope
// fields ok() stamps on every result.
//
// Shared by the two tasks/* methods the extension specifies an empty
// acknowledgement for, so the two cannot drift into checking different things.
func requireEmptyAck(method string, raw json.RawMessage) error {
	var ack map[string]any
	if err := json.Unmarshal(raw, &ack); err != nil {
		return fmt.Errorf("parse %s result: %w", method, err)
	}
	for key := range ack {
		if key != keyResultType && key != "_meta" {
			return fmt.Errorf("%s acknowledgement is not empty: unexpected key %q in %s", method, key, compactJSON(raw))
		}
	}
	return nil
}

var taskTerminalStates = map[string]bool{
	"completed": true,
	"failed":    true,
	"cancelled": true, //nolint:misspell // MCP spec wire value
}

func (c *mcpClient) taskWait(taskID, targetState string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := c.taskGet(taskID)
		if err == nil && status == targetState {
			return nil
		}
		if err == nil && taskTerminalStates[status] && status != targetState {
			return fmt.Errorf("task %s reached terminal state %q, wanted %q", taskID, status, targetState)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("task %s did not reach %q within %v", taskID, targetState, timeout)
}
