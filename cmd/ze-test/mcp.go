// Design: docs/architecture/testing/ci-format.md -- MCP test client

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// mcpCmd runs the MCP client subcommand.
// Reads Ze commands from stdin (one per line), sends each via MCP ze_execute
// to the daemon's MCP endpoint, prints responses to stdout.
//
// Special stdin lines:
//
//	# comment       -- ignored
//	wait <duration> -- pause (e.g. "wait 1s")
//
// Retries the MCP connection with backoff until the server is ready.
var _ = register("mcp", "MCP client (send commands to daemon via MCP endpoint)", mcpCmd)

func mcpCmd() int {
	fs := flag.NewFlagSet("ze-test mcp", flag.ContinueOnError)
	port := fs.String("port", "", "MCP server port (required)")
	token := fs.String("token", "", "Bearer token for MCP authentication")
	timeout := fs.Duration("timeout", 10*time.Second, "Connection timeout")
	elicit := fs.Bool("elicit", false, "Declare capabilities.elicitation={} at initialize so the server may send elicitation/create")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test mcp --port <port> [--token <token>] [--timeout <duration>] [--elicit]

Send commands to a running Ze daemon via MCP.
Reads commands from stdin, one per line.

Stdin directives (one per line):
  <ze command>                     -- sent as ze_execute
  @<tool> <json args>              -- call a named MCP tool with JSON args
  wait <duration>                  -- sleep
  wait-established                 -- poll "peer list" until a peer is Established
  elicit-accept <json content>     -- queue an accept response for the next elicit
  elicit-decline                   -- queue a decline response for the next elicit
  elicit-cancel                    -- queue a cancel response for the next elicit

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}
	if *port == "" {
		fmt.Fprintf(os.Stderr, "error: --port is required\n")
		fs.Usage()
		return 1
	}

	client := &mcpClient{
		addr:          "127.0.0.1:" + *port,
		token:         *token,
		declareElicit: *elicit,
		// No Timeout: http.Client.Timeout would cut off a valid slow tool
		// call (or a long-lived SSE stream while Phase 4 tasks stream
		// progress) before the .ci runner's outer `timeout=` fires. Rely on
		// the runner's deadline to kill a hung process.
		http: &http.Client{},
	}

	// Wait for MCP server to be ready.
	if err := client.waitReady(*timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: MCP server not ready: %v\n", err)
		return 1
	}

	// MCP handshake.
	if err := client.initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "error: MCP initialize failed: %v\n", err)
		return 1
	}

	// Process commands from stdin.
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if durStr, ok := strings.CutPrefix(line, "wait "); ok {
			dur, err := time.ParseDuration(durStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid wait duration: %v\n", err)
				return 1
			}
			time.Sleep(dur)
			continue
		}

		// wait-established: poll "show bgp peer" until at least one peer is Established.
		if line == "wait-established" {
			if err := client.waitEstablished(*timeout); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			continue
		}

		// elicit-accept <json>: queue an accept response for the next
		// elicit. The payload MUST be a JSON object; the MCP server
		// rejects non-object `result.content` with 400, and surfacing
		// that to the user as "elicit reply status 400" is cryptic.
		// Catch the shape error here instead.
		if contentStr, ok := strings.CutPrefix(line, "elicit-accept "); ok {
			raw := strings.TrimSpace(contentStr)
			var probe map[string]any
			if err := json.Unmarshal([]byte(raw), &probe); err != nil {
				fmt.Fprintf(os.Stderr, "error: elicit-accept payload must be a JSON object: %v (raw=%q)\n", err, raw)
				return 1
			}
			client.elicitQueue = append(client.elicitQueue, elicitReply{action: "accept", content: json.RawMessage(raw)})
			continue
		}
		if line == "elicit-decline" {
			client.elicitQueue = append(client.elicitQueue, elicitReply{action: "decline"})
			continue
		}
		if line == "elicit-cancel" {
			client.elicitQueue = append(client.elicitQueue, elicitReply{action: "cancel"})
			continue
		}

		// @tool_name {json} -- call a specific MCP tool with JSON arguments.
		// Otherwise: send as ze_execute command.
		if toolName, toolArgs, ok := strings.Cut(line, " "); ok && strings.HasPrefix(toolName, "@") {
			result, err := client.callTool(toolName[1:], json.RawMessage(toolArgs))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Println(result)
		} else if strings.HasPrefix(line, "@") {
			// @tool_name with no args
			result, err := client.callTool(line[1:], json.RawMessage(`{}`))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Println(result)
		} else {
			result, err := client.execute(line)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Println(result)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: stdin: %v\n", err)
		return 1
	}

	return 0
}

// mcpClient sends MCP JSON-RPC requests over Streamable HTTP (MCP 2025-06-18).
//
// The session id is assigned by the server at initialize and echoed on every
// subsequent request via the Mcp-Session-Id header.
type mcpClient struct {
	addr          string
	token         string // Bearer token (empty = no auth)
	id            int
	sessionID     string // populated by initialize()
	declareElicit bool   // declare capabilities.elicitation={} at initialize

	// http is a shared client (initialized with no Timeout; see mcpCmd).
	// Kept as a field rather than http.DefaultClient so future knobs
	// (cookie jar, custom Transport for TLS, ResponseHeaderTimeout for
	// time-to-first-byte) can land here without touching every call site.
	http *http.Client

	// elicitQueue is a FIFO of prepared responses consumed when the server
	// sends elicitation/create over an SSE-upgraded POST. Each entry is
	// POSTed back with the elicit id the server chose on arrival.
	elicitQueue []elicitReply
}

// elicitReply is one queued response the client will POST back when the
// server sends elicitation/create. Content is used only when action=="accept".
type elicitReply struct {
	action  string
	content json.RawMessage
}

// endpoint is the Streamable HTTP MCP endpoint path.
const endpoint = "/mcp"

// waitReady retries until a TCP connection to the MCP listener succeeds.
//
// Intentionally does NOT send an HTTP request: the Streamable transport
// creates a session on every successful `initialize`, so a probe that
// completes the round trip would leak an orphan session per test run.
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
				return fmt.Errorf("close probe: %w", closeErr)
			}
			return nil
		}
		time.Sleep(interval)
		if interval < time.Second {
			interval *= 2
		}
	}
	return fmt.Errorf("timeout after %v", timeout)
}

// waitEstablished polls "peer list" via MCP until at least one peer is Established.
func (c *mcpClient) waitEstablished(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.execute("peer list")
		if err == nil && strings.Contains(strings.ToLower(result), "established") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("no peer established after %v", timeout)
}

// initialize sends the MCP handshake and captures the session id from the
// Mcp-Session-Id response header (required by MCP 2025-06-18). When
// declareElicit is set, the params declare capabilities.elicitation={}
// so the server is allowed to send elicitation/create.
func (c *mcpClient) initialize() error {
	params := json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{}}`)
	if c.declareElicit {
		params = json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{"elicitation":{}}}`)
	}
	sid, _, err := c.sendRaw("initialize", params)
	if err != nil {
		return err
	}
	c.sessionID = sid
	return nil
}

// callTool calls a named MCP tool with JSON arguments.
func (c *mcpClient) callTool(tool string, args json.RawMessage) (string, error) {
	params, err := json.Marshal(map[string]any{
		"name":      tool,
		"arguments": args,
	})
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}

	result, err := c.send("tools/call", json.RawMessage(params))
	if err != nil {
		return "", err
	}

	return c.extractText(result)
}

// execute sends a Ze command via MCP ze_execute and returns the response text.
func (c *mcpClient) execute(command string) (string, error) {
	args, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		return "", fmt.Errorf("marshal args: %w", err)
	}
	return c.callTool("ze_execute", args)
}

// extractText pulls the text content from an MCP tool result.
func (c *mcpClient) extractText(result json.RawMessage) (string, error) {

	// Parse MCP tool result as map to avoid camelCase struct tags.
	// MCP protocol: {"content":[{"type":"text","text":"..."}],"isError":bool}
	var toolResult map[string]any
	if err := json.Unmarshal(result, &toolResult); err != nil {
		return "", fmt.Errorf("parse tool result: %w", err)
	}

	if isErr, ok := toolResult["isError"].(bool); ok && isErr {
		if content, ok := toolResult["content"].([]any); ok && len(content) > 0 {
			if entry, ok := content[0].(map[string]any); ok {
				return "", fmt.Errorf("command error: %s", entry["text"])
			}
		}
		return "", fmt.Errorf("command error (no detail)")
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

// send makes a JSON-RPC request and returns the result. Callers outside of
// initialize() use this wrapper; initialize uses sendRaw to capture the
// assigned session id.
func (c *mcpClient) send(method string, params json.RawMessage) (json.RawMessage, error) {
	_, result, err := c.sendRaw(method, params)
	return result, err
}

// sendRaw performs the HTTP round trip and returns (assigned session id, result, err).
// The session id is non-empty only for the initialize response; every other
// request echoes the cached c.sessionID on outgoing requests.
//
// The response may arrive as application/json (the common case) or as
// text/event-stream when a tool handler called session.Elicit mid-dispatch;
// in the SSE case the client reads frames, posts queued elicit responses
// back to the server, and returns when the terminal response frame with the
// matching request id arrives.
func (c *mcpClient) sendRaw(method string, params json.RawMessage) (string, json.RawMessage, error) {
	c.id++
	reqID := c.id
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      reqID,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return "", nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://"+c.addr+endpoint, bytes.NewReader(reqBody)) //nolint:noctx // short-lived test tool
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Advertise both shapes so the server may upgrade to SSE when a handler
	// elicits. Without text/event-stream in Accept, conforming servers keep
	// the response as application/json even after a handler elicits, which
	// would deadlock the client waiting for a response it cannot receive.
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		resultBytes, sseErr := c.readSSEResult(resp.Body, reqID)
		if sseErr != nil {
			return "", nil, sseErr
		}
		return resp.Header.Get("Mcp-Session-Id"), resultBytes, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read response: %w", err)
	}

	// Parse response as map to avoid camelCase struct tags.
	var rpcResp map[string]any
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", nil, fmt.Errorf("parse response: %w", err)
	}

	if errObj, ok := rpcResp["error"].(map[string]any); ok {
		code, _ := errObj["code"].(float64)
		msg, _ := errObj["message"].(string)
		return "", nil, fmt.Errorf("RPC error %d: %s", int(code), msg)
	}

	resultBytes, err := json.Marshal(rpcResp["result"])
	if err != nil {
		return "", nil, fmt.Errorf("marshal result: %w", err)
	}
	return resp.Header.Get("Mcp-Session-Id"), resultBytes, nil
}

// readSSEResult consumes SSE frames from body until it receives a JSON-RPC
// response whose id matches reqID. Server-initiated requests (such as
// elicitation/create) encountered along the way are answered by POSTing a
// queued response back to /mcp; when the queue is empty an elicit is
// canceled automatically so the suspended handler unblocks. Returns the
// `result` field as raw JSON on success, or a formatted error when the
// response carries `error`.
func (c *mcpClient) readSSEResult(body io.Reader, reqID int) (json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
	// SSE frames carry single-line JSON by invariant (reply_sink.go
	// godoc), but MCP frames can be larger than the default 64 KB scanner
	// buffer -- tool outputs can reach the session's 1 MB cap.
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		dataStr, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			dataStr, ok = strings.CutPrefix(line, "data:")
			if !ok {
				continue
			}
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(dataStr), &frame); err != nil {
			return nil, fmt.Errorf("parse SSE frame: %w (raw=%q)", err, dataStr)
		}
		if m, ok := frame["method"].(string); ok && m != "" {
			if err := c.answerServerRequest(m, frame); err != nil {
				return nil, err
			}
			continue
		}
		// Match our outgoing id. Ze always echoes the request id verbatim
		// via *json.RawMessage, and this client always sends integer ids,
		// so JSON unmarshal always produces float64 here. A spec-compliant
		// server MAY emit string ids for its own requests (elicitation/create
		// uses strings) -- those are handled in the method branch above.
		if idf, ok := frame["id"].(float64); ok && int(idf) == reqID {
			if errObj, hasErr := frame["error"].(map[string]any); hasErr {
				code, _ := errObj["code"].(float64)
				msg, _ := errObj["message"].(string)
				return nil, fmt.Errorf("RPC error %d: %s", int(code), msg)
			}
			return json.Marshal(frame["result"])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("SSE read: %w", err)
	}
	return nil, fmt.Errorf("SSE stream ended before response to id %d", reqID)
}

// answerServerRequest dispatches a server-initiated JSON-RPC request frame
// received over SSE. Only elicitation/create is understood; any other method
// returns an error since the fixture does not know how to satisfy it.
func (c *mcpClient) answerServerRequest(method string, frame map[string]any) error {
	if method != "elicitation/create" {
		return fmt.Errorf("server-initiated %q not supported", method)
	}
	id, _ := frame["id"].(string)
	if id == "" {
		return fmt.Errorf("elicitation/create frame missing id")
	}
	var reply elicitReply
	if len(c.elicitQueue) > 0 {
		reply = c.elicitQueue[0]
		c.elicitQueue = c.elicitQueue[1:]
	} else {
		// Nothing queued -- cancel rather than hang the suspended handler.
		reply = elicitReply{action: "cancel"}
	}
	result := map[string]any{"action": reply.action}
	if reply.action == "accept" && len(reply.content) > 0 {
		result["content"] = reply.content
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		return fmt.Errorf("marshal elicit reply: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+c.addr+endpoint, bytes.NewReader(body)) //nolint:noctx // short-lived test tool
	if err != nil {
		return fmt.Errorf("build elicit reply: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("POST elicit reply: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("elicit reply status %d", resp.StatusCode)
	}
	return nil
}
