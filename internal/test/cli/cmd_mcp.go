// Design: docs/architecture/testing/ci-format.md -- MCP test client
// Detail: cmd_mcp_client.go -- MCP 2026-07-28 request construction, headers, transport
// Detail: cmd_mcp_calls.go -- tools/call, tools/list polling and tasks/* helpers

package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// probeOmit is the probe-* sentinel meaning "leave this out entirely".
const probeOmit = "-"

const mcpUsageHead = `Usage: ze-test mcp --port <port> [--token <token>] [--timeout <duration>] [--tasks] [--elicit <modes>]

Send commands to a running Ze daemon over MCP protocol revision `

const mcpUsageBody = `.
Every message is its own HTTP POST to /mcp: no initialize handshake, no session
id, no GET stream. Reads directives from stdin, one per line.

Stdin directives (one per line):
  <ze command>                     -- sent as a ze_execute tool call
  @<tool> [<json args>]            -- call a named MCP tool with JSON args
  wait <duration>                  -- sleep
  wait-established                 -- poll "show bgp peer list" until a peer is Established
  wait-peers                       -- poll "show bgp peer list" until at least one peer exists
  wait-tool <name>                 -- poll tools/list until <name> appears
  tools-order-stable [<calls>]     -- call tools/list <calls> times (default 3)
                                      against an unchanged daemon and print
                                      "tools-order stable=<true|false> ...".
                                      Compares the tools array BYTE for byte, so
                                      a wobbling action enum or description is
                                      caught, not just a reordered name list; the
                                      stable line carries a digest of the array.
                                      On drift the line names the diverging call,
                                      and either the index plus both tool-name
                                      sequences, or -- when only the payload
                                      moved -- the byte offset and both digests
  task-call <tool> [<json args>]   -- ordinary tools/call the SERVER must answer
                                      with resultType "task"; prints the taskId.
                                      There is no client-side task opt-in: the
                                      server decides from ze:task-support
  call-sync <tool> [<json args>]   -- ordinary tools/call the server must answer
                                      SYNCHRONOUSLY (resultType "complete", no
                                      taskId); prints the result text
  task-get <taskId>                -- call tasks/get, print status
  task-result <taskId>             -- call tasks/get and print the result a
                                      TERMINAL task carries (tasks/result is
                                      gone; the payload rides on tasks/get)
  task-update <taskId> [<json>]    -- call tasks/update with optional
                                      inputResponses; requires an empty ack
  task-cancel <taskId>             -- call tasks/cancel; requires an empty ack
  task-wait <taskId> <state>       -- poll tasks/get until state matches

Multi Round-Trip Requests. When a server answers resultType "input_required",
the client builds inputResponses from the queued answer and retries the ORIGINAL
request under a new JSON-RPC id. Without a queued answer the call fails, so a
round trip is never taken by accident.

  elicit-answer <spec>             -- queue the answer given to every following
                                      elicitation. <spec> is one of:
                                        accept <value>  supply <value> under the
                                                        property the server's
                                                        requestedSchema names
                                        decline         refuse (terminal)
                                        cancel          dismiss (terminal)
                                        omit            retry carrying an empty
                                                        inputResponses, which
                                                        drives the re-ask path
                                        none            forget the queued answer
  elicit-extra <key>               -- also send an inputResponses entry under
                                      <key>, which no server asked for; servers
                                      must ignore it. "-" clears it

Deliberately-malformed requests. Each probe-* directive queues one deviation;
the next "probe" applies every queued deviation, prints one result line, then
clears the queue. A "probe" with nothing queued sends a fully conformant
request, which is how a test asserts the success shape (resultType, serverInfo).

  probe-header <name> <value|->    -- set a request header verbatim, with no
                                      sentinel encoding; "-" omits the header
  probe-meta <key> <value|->       -- set a params._meta field; "-" omits it.
                                      Short keys protocolVersion, clientInfo and
                                      clientCapabilities expand to their
                                      io.modelcontextprotocol/ names; a key
                                      containing "/" is used verbatim. A value
                                      starting with { [ or " is parsed as JSON,
                                      anything else is sent as a string. When
                                      every _meta field is omitted, params
                                      carries no _meta at all
  probe-method <verb>              -- HTTP verb for the next probe (default POST)
  probe-body <json|->              -- send this exact request body; "-" sends an
                                      empty body. Headers still come from the
                                      probe line's method and params
  probe <method> [<json params>]   -- send one request, print one line:
                                      probe status=<http> code=<jsonrpc|ok|none>
                                        [data=<json>] [result=<json>] message=<text>

  MCP-Protocol-Version is derived from the _meta protocolVersion value, so
  "probe-meta protocolVersion 2025-06-18" sends a consistent pair that tests
  version rejection (-32022), while "probe-header MCP-Protocol-Version 2025-06-18"
  sends a header/body mismatch that tests -32020.

$LAST in any line is replaced by the previous directive's output.

Options:
`

func cmdMcp(args []string) int {
	var text textbuf.Buffer

	fs := flag.NewFlagSet("ze-test mcp", flag.ContinueOnError)
	port := fs.String("port", "", "MCP server port (required)")
	token := fs.String("token", "", "Bearer token for MCP authentication")
	timeout := fs.Duration("timeout", 10*time.Second, "Connection timeout")
	tasksHelp := text.Str("Declare the ").Str(extensionTasks).Str(" extension in every request's _meta.clientCapabilities").String()
	// Tasks is the only capability a client can usefully declare to this
	// server. There is deliberately no --resources flag: `resources` is a
	// ServerCapabilities member, so declaring it in clientCapabilities is not
	// something a conformant client does, and the daemon no longer gates on it.
	tasks := fs.Bool("tasks", false, tasksHelp)
	// The elicitation capability is mode-structured, so the flag takes modes
	// rather than being a bare bool: a client declaring only "url" supports
	// elicitation and must never be sent a form-mode request.
	elicit := fs.String("elicit", "", `Declare the elicitation capability: "form", "url", "form,url", or "empty" for {} (form mode only)`)

	fs.Usage = func() {
		var usage textbuf.Buffer
		os.Stderr.WriteString(usage.Str(mcpUsageHead).Str(mcpProtocolVersion).Str(mcpUsageBody).String()) //nolint:errcheck // CLI usage output, nothing to recover
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *port == "" {
		mcpErrln("--port is required")
		fs.Usage()
		return 1
	}

	elicitCaps, elicitDeclared, err := parseElicitCapability(*elicit)
	if err != nil {
		return mcpFail(err)
	}

	text.Reset()
	client := &mcpClient{
		addr:         text.Str("127.0.0.1:").Str(*port).String(),
		token:        *token,
		declareTasks: *tasks,
		http:         &http.Client{},
	}
	if elicitDeclared {
		client.elicitCaps = elicitCaps
	}

	if err := client.waitReady(*timeout); err != nil {
		return mcpFail(fmt.Errorf("MCP server not ready: %w", err))
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ReplaceAll(line, "$LAST", client.lastOutput)
		if err := client.directive(line, *timeout); err != nil {
			return mcpFail(err)
		}
	}
	if err := scanner.Err(); err != nil {
		return mcpFail(fmt.Errorf("read stdin: %w", err))
	}
	return 0
}

func mcpPrintln(s string) {
	var b textbuf.Buffer
	os.Stdout.WriteString(b.Str(s).Byte('\n').String()) //nolint:errcheck // CLI stdout, nothing to recover
}

func mcpErrln(msg string) {
	var b textbuf.Buffer
	os.Stderr.WriteString(b.Str("error: ").Str(msg).Byte('\n').String()) //nolint:errcheck // CLI stderr, nothing to recover
}

func mcpFail(err error) int {
	var b textbuf.Buffer
	os.Stderr.WriteString(b.Str("error: ").Err(err).Byte('\n').String()) //nolint:errcheck // CLI stderr, nothing to recover
	return 1
}

// directive executes one stdin line, printing whatever that line produces.
func (c *mcpClient) directive(line string, timeout time.Duration) error {
	if rest, ok := strings.CutPrefix(line, "wait "); ok {
		spec := strings.TrimSpace(rest)
		dur, err := time.ParseDuration(spec)
		if err != nil {
			return fmt.Errorf("wait: invalid duration %q: %w", spec, err)
		}
		time.Sleep(dur)
		return nil
	}
	if line == "wait-established" {
		return c.waitEstablished(timeout)
	}
	if line == "wait-peers" {
		return c.waitPeers(timeout)
	}
	if rest, ok := strings.CutPrefix(line, "wait-tool "); ok {
		return c.waitTool(strings.TrimSpace(rest), timeout)
	}
	if handled, err := c.orderDirective(line); handled {
		return err
	}
	if handled, err := c.elicitDirective(line); handled {
		return err
	}

	if handled, err := c.probeDirective(line); handled {
		return err
	}
	if handled, err := c.taskDirective(line, timeout); handled {
		return err
	}

	if tool, ok := strings.CutPrefix(line, "@"); ok {
		name, args, found := strings.Cut(tool, " ")
		if !found || strings.TrimSpace(args) == "" {
			args = "{}"
		}
		result, err := c.callTool(name, json.RawMessage(args))
		if err != nil {
			return err
		}
		c.lastOutput = result
		mcpPrintln(result)
		return nil
	}

	result, err := c.execute(line)
	if err != nil {
		return err
	}
	c.lastOutput = result
	mcpPrintln(result)
	return nil
}

// orderDirective handles tools-order-stable, reporting whether the line was it.
//
// The count is optional; three calls is enough to catch a wobble that Go's
// per-range map-iteration randomization would produce, and a .ci that wants
// more confidence passes a bigger number.
func (c *mcpClient) orderDirective(line string) (bool, error) {
	rest, ok := strings.CutPrefix(line, "tools-order-stable")
	if !ok {
		return false, nil
	}
	rest = strings.TrimSpace(rest)
	calls := 3
	if rest != "" {
		parsed, err := strconv.Atoi(rest)
		if err != nil {
			return true, fmt.Errorf("tools-order-stable: %q is not a call count: %w", rest, err)
		}
		calls = parsed
	}
	out, err := c.toolsOrderStable(calls)
	if err != nil {
		return true, err
	}
	c.lastOutput = out
	mcpPrintln(out)
	return true, nil
}

// taskDirective handles the task-* directives, reporting whether the line was
// one of them.
func (c *mcpClient) taskDirective(line string, timeout time.Duration) (bool, error) {
	if rest, ok := strings.CutPrefix(line, "task-call "); ok {
		tool, args, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		taskID, err := c.taskCall(tool, json.RawMessage(args))
		if err != nil {
			return true, fmt.Errorf("task-call %s: %w", tool, err)
		}
		c.lastOutput = taskID
		mcpPrintln(taskID)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "task-get "); ok {
		status, err := c.taskGet(strings.TrimSpace(rest))
		if err != nil {
			return true, fmt.Errorf("task-get: %w", err)
		}
		c.lastOutput = status
		mcpPrintln(status)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "task-result "); ok {
		result, err := c.taskResultText(strings.TrimSpace(rest))
		if err != nil {
			return true, fmt.Errorf("task-result: %w", err)
		}
		c.lastOutput = result
		mcpPrintln(result)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "task-cancel "); ok {
		ack, err := c.taskCancel(strings.TrimSpace(rest))
		if err != nil {
			return true, fmt.Errorf("task-cancel: %w", err)
		}
		// Deliberately does NOT update lastOutput, for the same reason
		// task-update does not: tasks/cancel acknowledges with an empty result
		// rather than an identifier, so $LAST must keep naming the taskId
		// task-call produced.
		mcpPrintln(ack)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "call-sync "); ok {
		tool, args, _ := strings.Cut(strings.TrimSpace(rest), " ")
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		result, err := c.taskCallSync(tool, json.RawMessage(args))
		if err != nil {
			return true, fmt.Errorf("call-sync %s: %w", tool, err)
		}
		c.lastOutput = result
		mcpPrintln(result)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "task-update "); ok {
		taskID, responsesJSON, _ := strings.Cut(strings.TrimSpace(rest), " ")
		var responses map[string]any
		if trimmed := strings.TrimSpace(responsesJSON); trimmed != "" {
			if err := json.Unmarshal([]byte(trimmed), &responses); err != nil {
				return true, fmt.Errorf("task-update: inputResponses %q is not a JSON object: %w", trimmed, err)
			}
		}
		ack, err := c.taskUpdate(taskID, responses)
		if err != nil {
			return true, fmt.Errorf("task-update: %w", err)
		}
		// Deliberately does NOT update lastOutput. tasks/update returns an empty
		// acknowledgement, not an identifier, so $LAST must keep naming the
		// taskId that task-call produced -- otherwise a second task-update
		// $LAST would substitute this line's prose as the id.
		mcpPrintln(ack)
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "task-wait "); ok {
		parts := strings.Fields(rest)
		if len(parts) != 2 {
			return true, fmt.Errorf("task-wait needs <taskId> <state>, got %q", line)
		}
		return true, c.taskWait(parts[0], parts[1], timeout)
	}
	return false, nil
}

// probeDirective handles the probe-* queue and the probe that fires it,
// reporting whether the line was one of them.
func (c *mcpClient) probeDirective(line string) (bool, error) {
	if rest, ok := strings.CutPrefix(line, "probe-header "); ok {
		name, value, found := strings.Cut(strings.TrimSpace(rest), " ")
		if !found {
			return true, fmt.Errorf("probe-header needs <name> <value|%s>, got %q", probeOmit, line)
		}
		c.pending.setHeader(name, strings.TrimSpace(value))
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "probe-meta "); ok {
		return true, c.queueMeta(strings.TrimSpace(rest))
	}
	if rest, ok := strings.CutPrefix(line, "probe-method "); ok {
		c.pending.httpMethod = strings.ToUpper(strings.TrimSpace(rest))
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "probe-body "); ok {
		body := strings.TrimSpace(rest)
		if body == probeOmit {
			body = ""
		}
		c.pending.body = &body
		return true, nil
	}
	if rest, ok := strings.CutPrefix(line, "probe "); ok {
		return true, c.probe(strings.TrimSpace(rest))
	}
	return false, nil
}

// queueMeta parses a `probe-meta <key> <value|->` argument onto the pending
// mutation. Short key names expand to the reserved io.modelcontextprotocol/
// spellings; a key already carrying a "/" is used verbatim, so a test can reach
// any _meta key at all.
func (c *mcpClient) queueMeta(rest string) error {
	key, value, found := strings.Cut(rest, " ")
	if !found {
		return fmt.Errorf("probe-meta needs <key> <value|%s>, got %q", probeOmit, rest)
	}
	value = strings.TrimSpace(value)

	full := key
	if !strings.Contains(key, "/") {
		switch key {
		case "protocolVersion":
			full = metaProtocolVersion
		case "clientInfo":
			full = metaClientInfo
		case "clientCapabilities":
			full = metaClientCapabilities
		default:
			return fmt.Errorf("probe-meta: unknown short key %q, want protocolVersion, clientInfo, clientCapabilities, or a fully-qualified key containing a slash", key)
		}
	}

	if value == probeOmit {
		c.pending.setMeta(full, nil, true)
		return nil
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") || strings.HasPrefix(value, `"`) {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return fmt.Errorf("probe-meta %s: value %q is not valid JSON: %w", key, value, err)
		}
		c.pending.setMeta(full, parsed, false)
		return nil
	}
	c.pending.setMeta(full, value, false)
	return nil
}

// probe sends one request under the queued deviations and prints a single line
// carrying the HTTP status, the JSON-RPC error code (or "ok"), the error data
// and the message, so a .ci can assert on the status and the code together.
func (c *mcpClient) probe(rest string) error {
	method, rawParams, _ := strings.Cut(rest, " ")
	method = strings.TrimSpace(method)
	if method == "" {
		return errors.New("probe needs <method> [<json params>]")
	}
	params := map[string]any{}
	if trimmed := strings.TrimSpace(rawParams); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &params); err != nil {
			return fmt.Errorf("probe %s: params %q is not a JSON object: %w", method, trimmed, err)
		}
	}

	mut := c.pending
	c.pending.reset()

	out, err := c.exchange(method, params, &mut)
	if err != nil {
		return fmt.Errorf("probe %s: %w", method, err)
	}

	line, err := formatProbeResult(out)
	if err != nil {
		return fmt.Errorf("probe %s: %w", method, err)
	}
	c.lastOutput = line
	mcpPrintln(line)
	return nil
}

// formatProbeResult renders one exchange as the single line a .ci asserts on.
func formatProbeResult(out transportResult) (string, error) {
	var b textbuf.Buffer
	b.Str("probe status=").Int(int64(out.status))
	if out.frame == nil {
		b.Str(" code=none message=").Str(sanitize(out.body))
		return b.String(), nil
	}
	raw, isErr := out.frame[keyError]
	if !isErr {
		b.Str(" code=ok result=").Str(compactJSON(out.frame["result"]))
		return b.String(), nil
	}
	parsed, err := parseRPCError(raw)
	if err != nil {
		return "", err
	}
	b.Str(" code=").Int(int64(parsed.Code))
	if len(parsed.Data) > 0 {
		b.Str(" data=").Str(compactJSON(parsed.Data))
	}
	b.Str(" message=").Str(sanitize(parsed.Message))
	return b.String(), nil
}
