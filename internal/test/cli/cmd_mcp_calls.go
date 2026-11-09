// Design: docs/architecture/testing/ci-format.md -- MCP test client
// Overview: cmd_mcp.go -- the le test mcp command and its stdin directives
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

// The structural limits real MCP clients are known to enforce on a tool's input
// schema. Source: REA's "Compact input schema profile" (REA
// docs/reference/mcp-contracts.md): a 13,312-byte per-tool input schema budget,
// measured below Moonshot's cap, and a ten-level nesting limit. A value exactly
// at a limit is within it; one above it breaks it.
const (
	toolSchemaBytesMax = 13312
	toolSchemaDepthMax = 10
)

// The limit names a tools-size violation line carries. Each matches a key of
// the summary line, so a reader maps a violation to the figure it broke.
const (
	toolsSizeLimitSchemaBytes = "schema-bytes"
	toolsSizeLimitDepth       = "depth"
	toolsSizeLimitBareEnum    = "bare-enum"
	toolsSizeLimitCombinators = "combinators"
)

// toolsSizeViolation is one tool breaking one structural limit.
type toolsSizeViolation struct {
	tool  string
	limit string
	value int
}

// toolsSize is the measurement of one served tools array. Every byte figure
// counts the bytes as the server sent them, never a re-encoding.
type toolsSize struct {
	count          int
	bytes          int // the whole tools array, brackets and separators included
	largest        string
	largestBytes   int
	schemaBytesMax int
	depthMax       int
	bareEnums      int // enum keywords with no sibling type, over every tool
	combinators    int // oneOf, allOf and anyOf keywords, over every tool
	violations     []toolsSizeViolation
}

// toolsSizeReport calls tools/list once and renders its measurement.
func (c *mcpClient) toolsSizeReport() (string, error) {
	snapshot, err := c.toolsList()
	if err != nil {
		return "", err
	}
	size, err := measureToolsSize(snapshot.raw)
	if err != nil {
		return "", err
	}
	return size.render(), nil
}

// measureToolsSize measures a tools array as the server served it. It is pure,
// so a unit test feeds it canned JSON. The array is split into json.RawMessage
// elements, and each element's inputSchema is kept raw too: json.Unmarshal
// copies a RawMessage verbatim, so every length below is a length of served
// bytes.
//
// A malformed answer is an error naming what could not be read, never a panic
// and never a zero figure that would read as a small catalog.
func measureToolsSize(raw []byte) (toolsSize, error) {
	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		return toolsSize{}, fmt.Errorf("tools-size: the tools/list answer is not a JSON array of tools: %w", err)
	}
	size := toolsSize{count: len(tools), bytes: len(raw)}
	for index, tool := range tools {
		var fields struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		}
		if err := json.Unmarshal(tool, &fields); err != nil {
			return toolsSize{}, fmt.Errorf("tools-size: tool %d is not a JSON object: %w", index, err)
		}
		if fields.Name == "" {
			return toolsSize{}, fmt.Errorf("tools-size: tool %d carries no name", index)
		}
		if len(fields.InputSchema) == 0 {
			return toolsSize{}, fmt.Errorf("tools-size: tool %q carries no inputSchema, which MCP requires", fields.Name)
		}
		shape, err := measureSchemaShape(fields.InputSchema)
		if err != nil {
			return toolsSize{}, fmt.Errorf("tools-size: tool %q: %w", fields.Name, err)
		}
		size.add(fields.Name, len(tool), len(fields.InputSchema), shape)
	}
	return size, nil
}

// add folds one tool's figures into the totals and records each limit it breaks.
func (s *toolsSize) add(name string, toolBytes, schemaBytes int, shape schemaShape) {
	if toolBytes > s.largestBytes {
		s.largest = name
		s.largestBytes = toolBytes
	}
	s.schemaBytesMax = max(s.schemaBytesMax, schemaBytes)
	s.depthMax = max(s.depthMax, shape.depth)
	s.bareEnums += shape.bareEnums
	s.combinators += shape.combinators
	if schemaBytes > toolSchemaBytesMax {
		s.violations = append(s.violations, toolsSizeViolation{name, toolsSizeLimitSchemaBytes, schemaBytes})
	}
	if shape.depth > toolSchemaDepthMax {
		s.violations = append(s.violations, toolsSizeViolation{name, toolsSizeLimitDepth, shape.depth})
	}
	if shape.bareEnums > 0 {
		s.violations = append(s.violations, toolsSizeViolation{name, toolsSizeLimitBareEnum, shape.bareEnums})
	}
	if shape.combinators > 0 {
		s.violations = append(s.violations, toolsSizeViolation{name, toolsSizeLimitCombinators, shape.combinators})
	}
}

// render prints the summary line, then one line per violation in tool order.
func (s *toolsSize) render() string {
	var b textbuf.Buffer
	b.Str("tools-size count=").Int(int64(s.count))
	b.Str(" bytes=").Int(int64(s.bytes))
	b.Str(" largest=").Str(s.largest)
	b.Str(" largest-bytes=").Int(int64(s.largestBytes))
	b.Str(" max-schema-bytes=").Int(int64(s.schemaBytesMax))
	b.Str(" max-depth=").Int(int64(s.depthMax))
	b.Str(" bare-enum=").Int(int64(s.bareEnums))
	b.Str(" combinators=").Int(int64(s.combinators))
	for _, v := range s.violations {
		b.Str("\ntools-size violation tool=").Str(v.tool)
		b.Str(" limit=").Str(v.limit)
		b.Str(" value=").Int(int64(v.value))
	}
	return b.String()
}

// schemaShape is the structure of one input schema.
type schemaShape struct {
	depth       int
	bareEnums   int
	combinators int
}

// schemaNode is a schema object waiting to be walked, with its nesting level.
type schemaNode struct {
	schema map[string]any
	level  int
}

// measureSchemaShape walks one input schema and returns its depth, its bare
// enum count and its combinator count.
//
// Depth counts schema levels, not JSON nesting. The input schema itself is
// level 1. A schema reached through a keyword that holds subschemas is one
// level below the schema holding it: each value of properties,
// patternProperties, $defs and definitions; items, additionalProperties and not
// when they hold an object; each member of an items array, prefixItems, oneOf,
// anyOf and allOf. Ze's served shape, an object of flat typed properties, is
// depth 2. Only schema objects are walked, so a property NAMED enum or oneOf is
// a name, never a keyword.
//
// The walk uses an explicit stack rather than recursion, because the server
// chooses the nesting. encoding/json refuses nesting deeper than 10000, which
// bounds the stack.
func measureSchemaShape(raw json.RawMessage) (schemaShape, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return schemaShape{}, fmt.Errorf("inputSchema is not valid JSON: %w", err)
	}
	top, ok := root.(map[string]any)
	if !ok {
		return schemaShape{}, errors.New("inputSchema is not a JSON object")
	}
	var shape schemaShape
	stack := []schemaNode{{schema: top, level: 1}}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		shape.depth = max(shape.depth, node.level)
		_, typed := node.schema["type"]
		if _, enumerated := node.schema["enum"]; enumerated && !typed {
			shape.bareEnums++
		}
		for _, keyword := range schemaCombinators {
			if _, present := node.schema[keyword]; present {
				shape.combinators++
			}
		}
		stack = appendSubschemas(stack, node)
	}
	return shape, nil
}

// schemaCombinators are the keywords strict clients refuse in an input schema.
var schemaCombinators = []string{"oneOf", "allOf", "anyOf"}

// Keywords whose value is a map of name to subschema, a single subschema, or an
// array of subschemas. items appears twice because JSON Schema allows both
// forms.
var (
	subschemaMapKeywords   = []string{"properties", "patternProperties", "$defs", "definitions"}
	subschemaValueKeywords = []string{"items", "additionalProperties", "not"}
	subschemaListKeywords  = []string{"items", "prefixItems", "oneOf", "anyOf", "allOf"}
)

// appendSubschemas pushes every subschema object of node, one level below it.
// A subschema that is not an object (a boolean schema, or a malformed value)
// has no keywords to measure and adds no level.
func appendSubschemas(stack []schemaNode, node schemaNode) []schemaNode {
	level := node.level + 1
	for _, keyword := range subschemaMapKeywords {
		members, _ := node.schema[keyword].(map[string]any)
		for _, member := range members {
			if schema, ok := member.(map[string]any); ok {
				stack = append(stack, schemaNode{schema: schema, level: level})
			}
		}
	}
	for _, keyword := range subschemaValueKeywords {
		if schema, ok := node.schema[keyword].(map[string]any); ok {
			stack = append(stack, schemaNode{schema: schema, level: level})
		}
	}
	for _, keyword := range subschemaListKeywords {
		members, _ := node.schema[keyword].([]any)
		for _, member := range members {
			if schema, ok := member.(map[string]any); ok {
				stack = append(stack, schemaNode{schema: schema, level: level})
			}
		}
	}
	return stack
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
