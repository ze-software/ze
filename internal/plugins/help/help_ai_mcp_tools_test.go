// Design: docs/features/ai-first.md -- the AI reference names only the MCP tools a server serves.
// Related: help_ai.go -- printSummary, printMCPTools and printRecipes carry the authored tool names.

package help_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/mcp"
	"github.com/ze-software/ze/internal/plugins/help"
)

// aiReferenceToolCall is a tools/call target an authored JSON-RPC example names.
var aiReferenceToolCall = regexp.MustCompile(`"name":"(ze_[a-z0-9_]+)"`)

// aiReferenceToolEntry is one entry of the MCP section's "Handcrafted tools:" list.
var aiReferenceToolEntry = regexp.MustCompile(`^ {2}(ze_[a-z0-9_]+) {2,}\S`)

// aiReferenceToolName is a tool name on a summary line that lists MCP tools.
var aiReferenceToolName = regexp.MustCompile(`ze_[a-z0-9_]+`)

// TestAIReferenceNamesOnlyServedMCPTools is the P-34 Doc-review carrier for the
// help_ai.go producer claims: the summary, the MCP section and the recipes are
// authored text, so nothing keeps the tool names they print in step with the
// tools the MCP server serves. The stale `ze_announce` recipe survived that way.
//
// Method: render `help ai` (the summary) and `help ai mcp` (the MCP section,
// recipes and common errors) through the local route, collect every tool name
// the text offers as a tools/call target, as a handcrafted tool entry or on a
// summary line listing MCP tools, then ask a real MCP server over HTTP for its
// tools/list. The server is built without a command lister, so it serves its
// handcrafted tools only: every authored name must be one of them, and every
// handcrafted tool it serves must be documented as one.
func TestAIReferenceNamesOnlyServedMCPTools(t *testing.T) {
	served := servedHandcraftedMCPTools(t)

	var summary bytes.Buffer
	if code := help.RenderAIHelp(&summary, nil); code != 0 {
		t.Fatalf("help ai exited %d:\n%s", code, summary.String())
	}
	var section bytes.Buffer
	if code := help.RenderAIHelp(&section, []string{"mcp"}); code != 0 {
		t.Fatalf("help ai mcp exited %d:\n%s", code, section.String())
	}

	named := map[string]string{}
	for line := range strings.SplitSeq(summary.String(), "\n") {
		if !strings.Contains(line, "MCP tools:") {
			continue
		}
		for _, name := range aiReferenceToolName.FindAllString(line, -1) {
			named[name] = "summary line " + strings.TrimSpace(line)
		}
	}
	documented := map[string]bool{}
	for line := range strings.SplitSeq(section.String(), "\n") {
		for _, match := range aiReferenceToolCall.FindAllStringSubmatch(line, -1) {
			named[match[1]] = "tools/call example " + strings.TrimSpace(line)
		}
		if match := aiReferenceToolEntry.FindStringSubmatch(line); match != nil {
			named[match[1]] = "handcrafted tool entry " + strings.TrimSpace(line)
			documented[match[1]] = true
		}
	}
	if len(named) == 0 {
		t.Fatalf("the AI reference names no MCP tool, so this check would prove nothing:\n%s\n%s", summary.String(), section.String())
	}

	for name, where := range named {
		if !slices.Contains(served, name) {
			t.Errorf("the AI reference names MCP tool %q (%s), which the MCP server does not serve; it serves %v", name, where, served)
		}
	}
	for _, name := range served {
		if !documented[name] {
			t.Errorf("the MCP server serves handcrafted tool %q, which the AI reference's MCP section does not list", name)
		}
	}
}

// servedHandcraftedMCPTools asks a real MCP server for tools/list over HTTP
// and returns the tool names it answers.
func servedHandcraftedMCPTools(t *testing.T) []string {
	t.Helper()
	server, err := mcp.NewStreamable(mcp.StreamableConfig{})
	if err != nil {
		t.Fatal(err)
	}
	listener := httptest.NewServer(server)
	t.Cleanup(func() {
		listener.Close()
		server.Close()
	})

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"` + mcp.ProtocolVersion + `",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, listener.URL+mcp.Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("MCP-Protocol-Version", mcp.ProtocolVersion)
	request.Header.Set("Mcp-Method", "tools/list")
	response, err := listener.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close() //nolint:errcheck // the body was read in full; a close error changes no assertion
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("tools/list answered HTTP %d: %s", response.StatusCode, raw)
	}
	var answer struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("tools/list answer is not JSON-RPC: %v: %s", err, raw)
	}
	names := make([]string, 0, len(answer.Result.Tools))
	for _, tool := range answer.Result.Tools {
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		t.Fatalf("tools/list served no tool: %s", raw)
	}
	return names
}
