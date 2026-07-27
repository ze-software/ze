// Design: docs/architecture/testing/ci-format.md -- MCP test client

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errCommandErrorNoDetail            = errors.New("command error (no detail)")
	errElicitationCreateFrameMissingId = errors.New("elicitation/create frame missing id")
)

func cmdMcp(args []string) int {
	fs := flag.NewFlagSet("ze-test mcp", flag.ContinueOnError)
	port := fs.String("port", "", "MCP server port (required)")
	token := fs.String("token", "", "Bearer token for MCP authentication")
	timeout := fs.Duration("timeout", 10*time.Second, "Connection timeout")
	elicit := fs.Bool("elicit", false, "Declare capabilities.elicitation={} at initialize so the server may send elicitation/create")
	tasks := fs.Bool("tasks", false, "Declare capabilities.tasks={} at initialize for task-augmented tools/call")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test mcp --port <port> [--token <token>] [--timeout <duration>] [--elicit] [--tasks]

Send commands to a running Ze daemon via MCP.
Reads commands from stdin, one per line.

Stdin directives (one per line):
  <ze command>                     -- sent as ze_execute
  @<tool> <json args>              -- call a named MCP tool with JSON args
  wait <duration>                  -- sleep
  wait-established                 -- poll "show bgp peer list" until a peer is Established
  wait-peers                       -- poll "show bgp peer list" until at least one peer exists
  wait-tool <name>                 -- poll tools/list until <name> appears
  elicit-accept <json content>     -- queue an accept response for the next elicit
  elicit-decline                   -- queue a decline response for the next elicit
  elicit-cancel                    -- queue a cancel response for the next elicit
  task-call <tool> <json args>     -- call tool with task:{}, print taskId
  task-get <taskId>                -- call tasks/get, print status
  task-result <taskId>             -- call tasks/result, print result
  task-cancel <taskId>             -- call tasks/cancel
  task-list                        -- call tasks/list, print task ids
  task-wait <taskId> <state>       -- poll tasks/get until state matches
  sse-listen                       -- open GET /mcp SSE stream (server-initiated frames)
  sse-expect <method>              -- wait for a server-initiated frame with <method>, print it

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
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
		declareTasks:  *tasks,
		http:          &http.Client{},
	}

	if err := client.waitReady(*timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: MCP server not ready: %v\n", err)
		return 1
	}

	if err := client.initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "error: MCP initialize failed: %v\n", err)
		return 1
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ReplaceAll(line, "$LAST", client.lastOutput)

		if durStr, ok := strings.CutPrefix(line, "wait "); ok {
			dur, err := time.ParseDuration(durStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid wait duration: %v\n", err)
				return 1
			}
			time.Sleep(dur)
			continue
		}

		if line == "wait-established" {
			if err := client.waitEstablished(*timeout); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			continue
		}

		if line == "wait-peers" {
			if err := client.waitPeers(*timeout); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			continue
		}

		if toolArg, ok := strings.CutPrefix(line, "wait-tool "); ok {
			if err := client.waitTool(strings.TrimSpace(toolArg), *timeout); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			continue
		}

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

		if rest, ok := strings.CutPrefix(line, "task-call "); ok {
			toolName, toolArgs, _ := strings.Cut(rest, " ")
			if toolArgs == "" {
				toolArgs = "{}"
			}
			taskID, err := client.taskCall(toolName, json.RawMessage(toolArgs))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: task-call: %v\n", err)
				return 1
			}
			client.lastOutput = taskID
			fmt.Println(taskID)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "task-get "); ok {
			status, err := client.taskGet(strings.TrimSpace(rest))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: task-get: %v\n", err)
				return 1
			}
			client.lastOutput = status
			fmt.Println(status)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "task-result "); ok {
			result, err := client.taskResult(strings.TrimSpace(rest))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: task-result: %v\n", err)
				return 1
			}
			client.lastOutput = result
			fmt.Println(result)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "task-cancel "); ok {
			if err := client.taskCancel(strings.TrimSpace(rest)); err != nil {
				fmt.Fprintf(os.Stderr, "error: task-cancel: %v\n", err)
				return 1
			}
			continue
		}
		if line == "task-list" {
			ids, err := client.taskList()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: task-list: %v\n", err)
				return 1
			}
			client.lastOutput = ids
			fmt.Println(ids)
			continue
		}
		if rest, ok := strings.CutPrefix(line, "task-wait "); ok {
			parts := strings.Fields(rest)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "error: task-wait needs <taskId> <state>\n")
				return 1
			}
			if err := client.taskWait(parts[0], parts[1], *timeout); err != nil {
				fmt.Fprintf(os.Stderr, "error: task-wait: %v\n", err)
				return 1
			}
			continue
		}

		if line == "sse-listen" {
			if err := client.startSSE(); err != nil {
				var tb textbuf.Buffer
				os.Stderr.WriteString(tb.Str("error: sse-listen: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI error output
				return 1
			}
			continue
		}
		if method, ok := strings.CutPrefix(line, "sse-expect "); ok {
			if err := client.sseExpect(strings.TrimSpace(method), *timeout); err != nil {
				var tb textbuf.Buffer
				os.Stderr.WriteString(tb.Str("error: sse-expect: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI error output
				return 1
			}
			continue
		}

		if toolName, toolArgs, ok := strings.Cut(line, " "); ok && strings.HasPrefix(toolName, "@") {
			result, err := client.callTool(toolName[1:], json.RawMessage(toolArgs))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
			fmt.Println(result)
		} else if strings.HasPrefix(line, "@") {
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

type mcpClient struct {
	addr          string
	token         string
	id            int
	sessionID     string
	declareElicit bool
	declareTasks  bool
	http          *http.Client
	lastOutput    string
	elicitQueue   []elicitReply
	// sseFrames receives server-initiated JSON-RPC frames read off the GET /mcp
	// SSE stream (opened by sse-listen). nil until sse-listen runs.
	sseFrames chan map[string]any
	sseErrCh  chan error
}

type elicitReply struct {
	action  string
	content json.RawMessage
}

const mcpEndpoint = "/mcp"

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

func (c *mcpClient) waitEstablished(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.execute("show bgp peer list")
		if err == nil && strings.Contains(strings.ToLower(result), "established") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("no peer established after %v", timeout)
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
	return fmt.Errorf("no peers configured after %v", timeout)
}

func (c *mcpClient) waitTool(name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.send("tools/list", json.RawMessage("{}"))
		if err == nil {
			var resp struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			if json.Unmarshal(result, &resp) == nil {
				for _, t := range resp.Tools {
					if t.Name == name {
						return nil
					}
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("tool %q not found after %v", name, timeout)
}

func (c *mcpClient) initialize() error {
	caps := map[string]any{}
	if c.declareElicit {
		caps["elicitation"] = map[string]any{}
	}
	if c.declareTasks {
		caps["tasks"] = map[string]any{}
	}
	paramMap := map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    caps,
	}
	params, _ := json.Marshal(paramMap)
	sid, _, err := c.sendRaw("initialize", params)
	if err != nil {
		return err
	}
	c.sessionID = sid
	return nil
}

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

func (c *mcpClient) execute(command string) (string, error) {
	args, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		return "", fmt.Errorf("marshal args: %w", err)
	}
	return c.callTool("ze_execute", args)
}

func (c *mcpClient) extractText(result json.RawMessage) (string, error) {
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

func (c *mcpClient) send(method string, params json.RawMessage) (json.RawMessage, error) {
	_, result, err := c.sendRaw(method, params)
	return result, err
}

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

	req, err := http.NewRequest(http.MethodPost, "http://"+c.addr+mcpEndpoint, bytes.NewReader(reqBody)) //nolint:noctx // short-lived test tool
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
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

func (c *mcpClient) readSSEResult(body io.Reader, reqID int) (json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
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

func (c *mcpClient) answerServerRequest(method string, frame map[string]any) error {
	if method != "elicitation/create" {
		return fmt.Errorf("server-initiated %q not supported", method)
	}
	id, _ := frame["id"].(string)
	if id == "" {
		return errElicitationCreateFrameMissingId
	}
	var reply elicitReply
	if len(c.elicitQueue) > 0 {
		reply = c.elicitQueue[0]
		c.elicitQueue = c.elicitQueue[1:]
	} else {
		reply = elicitReply{action: "cancel"}
	}
	result := map[string]any{"action": reply.action}
	if reply.action == "accept" && len(reply.content) > 0 {
		result["content"] = reply.content
	}
	respBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	if err != nil {
		return fmt.Errorf("marshal elicit reply: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+c.addr+mcpEndpoint, bytes.NewReader(respBody)) //nolint:noctx // short-lived test tool
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

// startSSE opens the server-to-client GET /mcp SSE stream (MCP 2025-06-18
// Streamable HTTP), which delivers server-initiated frames (task status
// notifications, task-context elicitation) off the session outbound queue. A
// background goroutine reads frames onto c.sseFrames; sseExpect drains them.
func (c *mcpClient) startSSE() error {
	if c.sseFrames != nil {
		return errors.New("sse-listen already active")
	}
	var tb textbuf.Buffer
	url := tb.Str("http://").Str(c.addr).Str(mcpEndpoint).String()
	req, err := http.NewRequest(http.MethodGet, url, http.NoBody) //nolint:noctx // short-lived test tool
	if err != nil {
		return fmt.Errorf("build GET request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		tb.Reset()
		req.Header.Set("Authorization", tb.Str("Bearer ").Str(c.token).String())
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET /mcp: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() //nolint:errcheck,gosec // cleanup on error
		return fmt.Errorf("GET /mcp status %d", resp.StatusCode)
	}
	c.sseFrames = make(chan map[string]any, 32)
	c.sseErrCh = make(chan error, 1)
	go c.readSSEStream(resp.Body)
	return nil
}

// readSSEStream parses `data:` frames off the GET SSE stream and forwards each
// server-initiated JSON-RPC frame onto c.sseFrames.
func (c *mcpClient) readSSEStream(body io.ReadCloser) {
	defer body.Close() //nolint:errcheck // best-effort cleanup on stream end
	scanner := bufio.NewScanner(body)
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
			continue
		}
		select {
		case c.sseFrames <- frame:
		default: // drop if the consumer is not draining fast enough
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case c.sseErrCh <- err:
		default:
		}
	}
}

// sseExpect blocks until a server-initiated frame with the given JSON-RPC
// method arrives on the GET stream (or timeout), then prints it so a .ci can
// assert on the framing.
func (c *mcpClient) sseExpect(method string, timeout time.Duration) error {
	if c.sseFrames == nil {
		return errors.New("sse-expect requires sse-listen first")
	}
	deadline := time.After(timeout)
	for {
		select {
		case frame := <-c.sseFrames:
			if m, _ := frame["method"].(string); m == method {
				raw, err := json.Marshal(frame)
				if err != nil {
					return fmt.Errorf("marshal frame: %w", err)
				}
				fmt.Println(string(raw))
				return nil
			}
		case err := <-c.sseErrCh:
			return fmt.Errorf("sse stream error: %w", err)
		case <-deadline:
			return fmt.Errorf("timed out waiting for server frame %q", method)
		}
	}
}

func (c *mcpClient) taskCall(tool string, args json.RawMessage) (string, error) {
	params, err := json.Marshal(map[string]any{
		"name":      tool,
		"arguments": args,
		"task":      map[string]any{},
	})
	if err != nil {
		return "", fmt.Errorf("marshal params: %w", err)
	}
	result, err := c.send("tools/call", json.RawMessage(params))
	if err != nil {
		return "", err
	}
	var r map[string]any
	if err := json.Unmarshal(result, &r); err != nil {
		return "", fmt.Errorf("parse CreateTaskResult: %w", err)
	}
	taskID, _ := r["taskId"].(string)
	if taskID == "" {
		return "", fmt.Errorf("CreateTaskResult missing taskId: %s", result)
	}
	return taskID, nil
}

func (c *mcpClient) taskGet(taskID string) (string, error) {
	params, _ := json.Marshal(map[string]any{"taskId": taskID})
	result, err := c.send("tasks/get", json.RawMessage(params))
	if err != nil {
		return "", err
	}
	var r map[string]any
	if err := json.Unmarshal(result, &r); err != nil {
		return "", err
	}
	status, _ := r["status"].(string)
	return status, nil
}

func (c *mcpClient) taskResult(taskID string) (string, error) {
	params, _ := json.Marshal(map[string]any{"taskId": taskID})
	result, err := c.send("tasks/result", json.RawMessage(params))
	if err != nil {
		return "", err
	}
	return c.extractText(result)
}

func (c *mcpClient) taskCancel(taskID string) error {
	params, _ := json.Marshal(map[string]any{"taskId": taskID})
	_, err := c.send("tasks/cancel", json.RawMessage(params))
	return err
}

func (c *mcpClient) taskList() (string, error) {
	result, err := c.send("tasks/list", json.RawMessage(`{}`))
	if err != nil {
		return "", err
	}
	var r map[string]any
	if err := json.Unmarshal(result, &r); err != nil {
		return "", err
	}
	mcpTasks, _ := r["tasks"].([]any)
	var ids []string
	for _, t := range mcpTasks {
		if m, ok := t.(map[string]any); ok {
			if id, ok := m["taskId"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	return textbuf.Join(ids, " "), nil
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
