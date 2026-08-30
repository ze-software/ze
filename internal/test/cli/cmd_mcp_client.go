// Design: docs/architecture/testing/ci-format.md -- MCP test client
// Overview: cmd_mcp.go -- the ze-test mcp command and its stdin directives
// Related: cmd_mcp_calls.go -- the tools/* and tasks/* helpers built on send()

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MCP protocol revision 2026-07-28 wire constants.
//
// This client is written from the specification text rather than from Ze's
// server. A functional test that asserts on this client's behavior is
// therefore an independent reading of the protocol. It is not a restatement of
// the implementation under test.
const (
	// mcpProtocolVersion rides both in the MCP-Protocol-Version header and in
	// params._meta. The two MUST agree, or the server answers -32020.
	mcpProtocolVersion = "2026-07-28"
	mcpEndpoint        = "/mcp"

	// Reserved _meta keys, from basic/index "Per-request protocol fields".
	// protocolVersion and clientCapabilities are required on every request;
	// clientInfo is optional, but clients SHOULD send it.
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientInfo         = "io.modelcontextprotocol/clientInfo"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"

	// extensionTasks is the Tasks extension identifier, advertised through
	// clientCapabilities.extensions (basic/versioning "Extension Negotiation").
	extensionTasks = "io.modelcontextprotocol/tasks"

	// Standard request headers (streamable-http "Standard Request Headers").
	// The casing is deliberately not Go's canonical MIME form. These names are
	// written straight into the header map, so the bytes on the wire match the
	// specification's examples. Header field names are case-insensitive per
	// RFC 9110, so a conforming server accepts either spelling.
	headerProtocolVersion = "MCP-Protocol-Version"
	headerMethod          = "Mcp-Method"
	headerName            = "Mcp-Name"

	// Base64 sentinel markers for values that cannot ride as a plain ASCII
	// header field value (streamable-http "Value Encoding"). The markers are
	// case-sensitive and MUST appear exactly as shown, in lowercase.
	sentinelPrefix = "=?base64?"
	sentinelSuffix = "?="

	// resultType values defined by the core protocol (basic/index "ResultType").
	// A resultType the client does not recognize MUST be considered invalid, and
	// an absent resultType MUST be treated as "complete".
	resultTypeComplete      = "complete"
	resultTypeInputRequired = "input_required"

	// resultTypeTask comes from the io.modelcontextprotocol/tasks EXTENSION,
	// not from the core protocol. The value is legal for this client only when
	// the client declared that extension (--tasks). basic/index "ResultType":
	// "The set of supported ResultType values MUST be created from the set
	// defined in the core protocol and include any additional values of
	// supported extensions that are advertised via capabilities".
	resultTypeTask = "task"

	// Wire keys and method names repeated across these files. MCP wire keys are
	// camelCase by protocol, which is why they are literals here rather than
	// struct tags.
	methodToolsCall = "tools/call"
	methodToolsList = "tools/list"
	keyName         = "name"
	// toolCallArguments is the tools/call parameter that carries the tool arguments.
	toolCallArguments = "arguments"
	keyTaskID         = "taskId"
	keyError          = "error"

	// Multi Round-Trip Requests (basic/patterns/mrtr). An InputRequiredResult
	// carries inputRequests. The client answers when it retries the ORIGINAL
	// request with a different JSON-RPC id, and that retry carries
	// inputResponses.
	keyInputRequests   = "inputRequests"
	keyInputResponses  = "inputResponses"
	keyResultType      = "resultType"
	keyAction          = "action"
	keyContent         = "content"
	keyParams          = "params"
	keyMode            = "mode"
	keyRequestedSchema = "requestedSchema"
	keyProperties      = "properties"

	// Elicitation capability modes (client/elicitation "Capabilities"). A
	// client declaring the capability MUST support at least one of them, and an
	// empty object is equivalent to declaring form mode only.
	capElicitation = "elicitation"
	mcpModeForm    = "form"
	mcpModeURL     = "url"

	// Elicitation response actions.
	elicitAccept  = "accept"
	elicitDecline = "decline"
	elicitCancel  = "cancel"

	// maxInputRounds bounds the retry loop. A server MAY prompt repeatedly
	// (server requirement 8), so a client that answered and was asked again
	// must not loop forever. The bound turns "the server keeps asking" into a
	// named failure rather than a hang.
	maxInputRounds = 4
)

var (
	errCommandErrorNoDetail = errors.New("command error (no detail)")
	errEmptyResponseBody    = errors.New("server returned no response body")
	// errInputRequired fires when a server asks for input that this client has
	// no queued answer for. The elicit-answer directive drives the retry loop.
	// Without that directive, a report of the call as complete would be false,
	// so the client fails closed.
	errInputRequired = errors.New(`server returned resultType "input_required" and no elicit-answer is queued`)
)

// mcpClient speaks MCP revision 2026-07-28 over Streamable HTTP. It holds no
// session state by construction. Only these fields outlive a request: the
// endpoint, the credential, the declared capabilities, the JSON-RPC id counter,
// and the deviations queued for the next probe.
type mcpClient struct {
	addr         string
	token        string
	id           int
	declareTasks bool
	http         *http.Client
	lastOutput   string
	pending      probeMutation
	// elicitCaps is the declared `elicitation` capability object, nil when
	// --elicit was absent. A non-nil EMPTY map is a real declaration meaning
	// form mode only, which is why absence is nil rather than len()==0.
	elicitCaps map[string]any
	// elicit is the answer this client gives the next time a server asks. It is
	// queued by the elicit-answer directive and survives across rounds, so a
	// server that prompts twice is answered twice.
	elicit elicitPlan
}

// elicitPlan is how the client answers an inputRequests entry.
//
// The zero value of elicitPlan is "no answer queued". That zero value makes an
// InputRequiredResult a reported failure rather than a silent retry. A test
// that did not ask for the round trip must not get one.
type elicitPlan struct {
	queued bool
	// action is accept, decline or cancel. Empty with queued set means "omit":
	// retry carrying an empty inputResponses object, which is what drives the
	// server's re-ask path.
	action string
	// value is the string supplied on accept, placed under the single property
	// name the server's requestedSchema declares.
	value string
	// extra names an inputResponses key the server never asked for. Servers
	// must tolerate it (error-handling clause on unexpected InputResponses
	// parameters), so a test can prove they do.
	extra string
}

// probeMutation holds the deliberate deviations queued by the probe-*
// directives, applied by the next probe and then cleared.
type probeMutation struct {
	headers    map[string]string
	dropHeader map[string]bool
	meta       map[string]any
	dropMeta   map[string]bool
	httpMethod string
	body       *string
}

func (m *probeMutation) reset() { *m = probeMutation{} }

func (m *probeMutation) setHeader(name, value string) {
	if value == probeOmit {
		if m.dropHeader == nil {
			m.dropHeader = map[string]bool{}
		}
		m.dropHeader[name] = true
		delete(m.headers, name)
		return
	}
	if m.headers == nil {
		m.headers = map[string]string{}
	}
	m.headers[name] = value
	delete(m.dropHeader, name)
}

func (m *probeMutation) setMeta(key string, value any, omit bool) {
	if omit {
		if m.dropMeta == nil {
			m.dropMeta = map[string]bool{}
		}
		m.dropMeta[key] = true
		delete(m.meta, key)
		return
	}
	if m.meta == nil {
		m.meta = map[string]any{}
	}
	m.meta[key] = value
	delete(m.dropMeta, key)
}

// rpcError is the JSON-RPC error object a server returns. Every key is a single
// lowercase word, so the struct tags stay kebab-clean.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func parseRPCError(raw json.RawMessage) (rpcError, error) {
	var parsed rpcError
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return parsed, fmt.Errorf("parse JSON-RPC error object: %w", err)
	}
	return parsed, nil
}

// sanitize folds a server-supplied message onto one line so a .ci can match it.
func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func (c *mcpClient) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 100 * time.Millisecond
	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		conn, err := dialer.DialContext(ctx, "tcp", c.addr)
		cancel()
		if err == nil {
			if closeErr := conn.Close(); closeErr != nil {
				return fmt.Errorf("close probe connection to %s: %w", c.addr, closeErr)
			}
			return nil
		}
		time.Sleep(interval)
		if interval < time.Second {
			interval *= 2
		}
	}
	return fmt.Errorf("no listener on %s after %v", c.addr, timeout)
}

// clientCapabilities builds the io.modelcontextprotocol/clientCapabilities
// value sent on every request. Every field of ClientCapabilities is optional,
// so an empty object is the conformant default. The --tasks flag adds the one
// capability the daemon gates on, the Tasks extension.
//
// Nothing here declares `resources`. That name is a ServerCapabilities member,
// and it is not one of the five ClientCapabilities members (`experimental`,
// `roots`, `sampling`, `elicitation`, `extensions`). A conformant client
// therefore never sends `resources`, and the daemon never asks for it.
func (c *mcpClient) clientCapabilities() map[string]any {
	caps := map[string]any{}
	if c.declareTasks {
		caps["extensions"] = map[string]any{extensionTasks: map[string]any{}}
	}
	// "Clients that support elicitation MUST declare the `elicitation`
	// capability in _meta.io.modelcontextprotocol/clientCapabilities on each
	// request" -- on EACH request, so it is built here rather than once.
	if c.elicitCaps != nil {
		caps[capElicitation] = c.elicitCaps
	}
	return caps
}

// supportsElicitMode reports whether this client declared the given mode.
//
// An empty declared object means form mode only. The check therefore reads the
// declaration the way the specification defines it, rather than as a key
// lookup. This check polices the server: "Servers MUST NOT send elicitation
// requests with modes that are not supported by the client". A client that
// silently answered a mode it never declared cannot prove that rule.
func (c *mcpClient) supportsElicitMode(mode string) bool {
	if c.elicitCaps == nil {
		return false
	}
	if len(c.elicitCaps) == 0 {
		return mode == mcpModeForm
	}
	_, declared := c.elicitCaps[mode]
	return declared
}

// transportResult is one HTTP exchange: the status line plus, when the body
// carried one, the JSON-RPC frame that answered the request. body holds the raw
// body when it was not a JSON-RPC frame, as a plain 405 page is not.
type transportResult struct {
	status int
	frame  map[string]json.RawMessage
	body   string
}

// sendOnce issues one conformant request and returns its result object. It does
// NOT interpret resultType: the multi round-trip loop in cmd_mcp_mrtr.go owns
// that, because an "input_required" result is not an answer to this attempt.
func (c *mcpClient) sendOnce(method string, params map[string]any) (json.RawMessage, error) {
	out, err := c.exchange(method, params, nil)
	if err != nil {
		return nil, err
	}
	if out.frame == nil {
		if out.body != "" {
			return nil, fmt.Errorf("%s: HTTP %d with a non-JSON body: %s", method, out.status, sanitize(out.body))
		}
		return nil, fmt.Errorf("%s: HTTP %d: %w", method, out.status, errEmptyResponseBody)
	}
	if raw, isErr := out.frame[keyError]; isErr {
		parsed, parseErr := parseRPCError(raw)
		if parseErr != nil {
			return nil, fmt.Errorf("%s: %w", method, parseErr)
		}
		var b textbuf.Buffer
		b.Str(method).Str(": rpc error: status ").Int(int64(out.status))
		b.Str(", code ").Int(int64(parsed.Code)).Str(": ").Str(sanitize(parsed.Message))
		if len(parsed.Data) > 0 {
			b.Str(" data=").Str(compactJSON(parsed.Data))
		}
		return nil, errors.New(b.String())
	}
	result, ok := out.frame["result"]
	if !ok {
		return nil, fmt.Errorf("%s: response carries neither result nor error", method)
	}
	return result, nil
}

// classifyResult enforces the ResultType contract. An absent resultType means
// "complete", because servers on earlier revisions omit the field. A value that
// the client does not recognize is invalid.
//
// tasksDeclared widens the legal set by exactly one value. basic/index
// "ResultType" builds a client's supported set from "the set defined in the core
// protocol" plus "any additional values of supported extensions that are
// advertised via capabilities". The value "task" is therefore legal for a client
// that declared io.modelcontextprotocol/tasks, and invalid for one that did not.
//
// The gate stays here rather than accepting "task" unconditionally, and that is
// what lets task-no-extension.ci mean something. This client catches a server
// that pushes a task handle at a non-declaring client, so the behavior is not
// merely unasserted.
//
// Returns the resultType and the decoded result object. The caller can then
// read inputRequests from an interim result and decode it only once.
func classifyResult(result json.RawMessage, tasksDeclared bool) (string, map[string]any, error) {
	var fields map[string]any
	if err := json.Unmarshal(result, &fields); err != nil {
		return "", nil, fmt.Errorf("parse result object: %w", err)
	}
	raw, present := fields[keyResultType]
	if !present {
		return resultTypeComplete, fields, nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", nil, fmt.Errorf("resultType is not a string: %s", compactJSON(result))
	}
	switch value {
	case resultTypeComplete, resultTypeInputRequired:
		return value, fields, nil
	case resultTypeTask:
		if !tasksDeclared {
			return "", nil, fmt.Errorf("server returned resultType %q to a client that did not declare the %s extension",
				resultTypeTask, extensionTasks)
		}
		return value, fields, nil
	default:
		return "", nil, fmt.Errorf("unrecognized resultType %q, want %q, %q or (with the tasks extension) %q",
			value, resultTypeComplete, resultTypeInputRequired, resultTypeTask)
	}
}

// exchange builds one request under the given deviations (nil for none), sends
// it, and reads back the frame that answered it.
func (c *mcpClient) exchange(method string, params map[string]any, mut *probeMutation) (transportResult, error) {
	if mut == nil {
		mut = &probeMutation{}
	}

	meta := map[string]any{
		metaProtocolVersion:    mcpProtocolVersion,
		metaClientCapabilities: c.clientCapabilities(),
		metaClientInfo:         map[string]any{keyName: "ze-test-mcp", "version": mcpProtocolVersion},
	}
	maps.Copy(meta, mut.meta)
	for key := range mut.dropMeta {
		delete(meta, key)
	}

	body := map[string]any{}
	maps.Copy(body, params)
	if len(meta) > 0 {
		body["_meta"] = meta
	}

	c.id++
	reqID := c.id
	encoded, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  method,
		"params":  body,
	})
	if err != nil {
		return transportResult{}, fmt.Errorf("marshal %s request: %w", method, err)
	}
	if mut.body != nil {
		encoded = []byte(*mut.body)
		reqID = -1 // the raw body owns the id, so accept the first response frame
	}

	headers := c.standardHeaders(method, params, meta)
	maps.Copy(headers, mut.headers)
	for name := range mut.dropHeader {
		delete(headers, name)
	}

	httpMethod := mut.httpMethod
	if httpMethod == "" {
		httpMethod = http.MethodPost
	}
	return c.do(httpMethod, headers, encoded, reqID)
}

// standardHeaders builds the request metadata headers that the transport mirrors
// out of the body. MCP-Protocol-Version is derived from the _meta value, so the
// pair is consistent by construction. A test forces a mismatch with probe-header.
func (c *mcpClient) standardHeaders(method string, params, meta map[string]any) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		// A client MUST list both content types: the server MAY answer with a
		// single JSON object or with an SSE stream scoped to this request.
		"Accept": "application/json, text/event-stream",
	}
	version := mcpProtocolVersion
	if declared, ok := meta[metaProtocolVersion].(string); ok {
		version = declared
	}
	headers[headerProtocolVersion] = version
	headers[headerMethod] = method
	if name, ok := mcpNameFor(method, params); ok {
		headers[headerName] = encodeHeaderValue(name)
	}
	if c.token != "" {
		var b textbuf.Buffer
		headers["Authorization"] = b.Str("Bearer ").Str(c.token).String()
	}
	return headers
}

// mcpNameFor returns the Mcp-Name source value for the three methods that
// require the header: params.name for tools/call and prompts/get, params.uri
// for resources/read.
func mcpNameFor(method string, params map[string]any) (string, bool) {
	switch method {
	case methodToolsCall, "prompts/get":
		name, ok := params[keyName].(string)
		return name, ok
	case "resources/read":
		uri, ok := params["uri"].(string)
		return uri, ok
	default:
		return "", false
	}
}

// encodeHeaderValue applies the Base64 sentinel encoding to two kinds of value.
// The first is any value that cannot ride as a plain ASCII header field value.
// The second is any plain-ASCII value that a reader would otherwise mistake for
// an encoded one. The encoding is standard Base64 with padding over the UTF-8
// bytes, not base64url.
func encodeHeaderValue(value string) string {
	if headerSafe(value) {
		return value
	}
	var b textbuf.Buffer
	return b.Str(sentinelPrefix).Str(base64.StdEncoding.EncodeToString([]byte(value))).Str(sentinelSuffix).String()
}

func headerSafe(value string) bool {
	if value == "" {
		return true
	}
	// A value that already looks encoded must itself be encoded, or a server
	// would decode a literal that was never encoded.
	if strings.HasPrefix(value, sentinelPrefix) && strings.HasSuffix(value, sentinelSuffix) {
		return false
	}
	for i := range len(value) {
		c := value[i]
		if c == '\t' {
			continue
		}
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return !isOWS(value[0]) && !isOWS(value[len(value)-1])
}

func isOWS(c byte) bool { return c == ' ' || c == '\t' }

func (c *mcpClient) do(httpMethod string, headers map[string]string, body []byte, reqID int) (transportResult, error) {
	var url textbuf.Buffer
	endpoint := url.Str("http://").Str(c.addr).Str(mcpEndpoint).String()

	var reader io.Reader = http.NoBody
	if httpMethod == http.MethodPost {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(httpMethod, endpoint, reader) //nolint:noctx // short-lived test tool
	if err != nil {
		return transportResult{}, fmt.Errorf("build %s %s: %w", httpMethod, endpoint, err)
	}
	// Assigned into the map rather than through Set, so the header names keep
	// the casing that the specification prints. Set would canonicalize them.
	for name, value := range headers {
		req.Header[name] = []string{value}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return transportResult{}, fmt.Errorf("%s %s: %w", httpMethod, endpoint, err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	out := transportResult{status: resp.StatusCode}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		frame, sseErr := readSSEResponse(resp.Body, reqID)
		if sseErr != nil {
			return out, sseErr
		}
		out.frame = frame
		return out, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("read response body: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return out, nil
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(raw, &frame); err != nil {
		out.body = string(raw)
		return out, nil
	}
	out.frame = frame
	return out, nil
}

// sseScanner is the only SSE parser in this client. Per the SSE specification, a
// line that begins with a colon is a comment and carries no event data. Servers
// emit such lines as keep-alives. Consecutive `data:` lines accumulate into one
// payload, and the next blank line dispatches that payload.
type sseScanner struct {
	scanner *bufio.Scanner
}

func newSSEScanner(body io.Reader) *sseScanner {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	return &sseScanner{scanner: scanner}
}

// next returns the next event's data payload, or io.EOF at end of stream.
func (s *sseScanner) next() (string, error) {
	var lines []string
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if strings.HasPrefix(line, ":") {
			continue
		}
		if line == "" {
			if len(lines) == 0 {
				continue
			}
			return textbuf.Join(lines, "\n"), nil
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			lines = append(lines, strings.TrimPrefix(value, " "))
		}
		// event:, id: and retry: carry no payload this client acts on.
	}
	if err := s.scanner.Err(); err != nil {
		return "", fmt.Errorf("read SSE stream: %w", err)
	}
	if len(lines) > 0 {
		return textbuf.Join(lines, "\n"), nil
	}
	return "", io.EOF
}

// readSSEResponse consumes a per-request SSE response stream. Request-scoped
// notifications can precede the final response, and the final response ends the
// stream. A reqID below zero accepts the first response frame, which is what a
// raw-body probe needs because the probe owns its own id.
func readSSEResponse(body io.Reader, reqID int) (map[string]json.RawMessage, error) {
	scanner := newSSEScanner(body)
	for {
		data, err := scanner.next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("SSE stream ended before the response to id %d", reqID)
		}
		if err != nil {
			return nil, err
		}
		var frame map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			return nil, fmt.Errorf("parse SSE frame: %w (raw=%q)", err, data)
		}
		if _, isNotification := frame["method"]; isNotification {
			continue // notifications/progress and notifications/message
		}
		_, isError := frame[keyError]
		raw, hasID := frame["id"]
		if hasID {
			var id int
			if json.Unmarshal(raw, &id) == nil && (reqID < 0 || id == reqID) {
				return frame, nil
			}
		}
		// A malformed-request error carries a null or unreadable id and is still
		// the terminal frame for this stream.
		if isError {
			return frame, nil
		}
	}
}
