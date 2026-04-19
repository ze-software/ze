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

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test mcp --port <port> [--token <token>] [--timeout <duration>]

Send commands to a running Ze daemon via MCP.
Reads commands from stdin, one per line.

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
		addr:  "127.0.0.1:" + *port,
		token: *token,
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
	addr      string
	token     string // Bearer token (empty = no auth)
	id        int
	sessionID string // populated by initialize()
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
// Mcp-Session-Id response header (required by MCP 2025-06-18).
func (c *mcpClient) initialize() error {
	sid, _, err := c.sendRaw("initialize", json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{}}`))
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
func (c *mcpClient) sendRaw(method string, params json.RawMessage) (string, json.RawMessage, error) {
	c.id++
	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      c.id,
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort cleanup

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
