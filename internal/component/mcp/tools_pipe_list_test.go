// Design: docs/architecture/api/commands.md -- the ordered pipe list a structured route takes.

package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// The derived tool these tests call: group prefix, action, and the dispatch
// path they join into.
const (
	pipeListPrefix = "show fixture mcp"
	pipeListAction = "pipe"
	pipeListPath   = pipeListPrefix + " " + pipeListAction
)

// pipeListRunner answers a runner over the real dispatcher serving
// pipeListPath, a command that declares `received` and answers three rows,
// two of them received: with `received` declared it answers only those.
func pipeListRunner(t *testing.T) *server {
	t.Helper()
	command.RegisterPipeFilters([]string{pipeListPath},
		command.PipeFilter{Name: "received", Description: "Only the received rows", Argument: command.ArgNone})
	dispatcher := pluginserver.NewDispatcher()
	all := []any{
		map[string]any{"name": "a", "state": "received"},
		map[string]any{"name": "b", "state": "received"},
		map[string]any{"name": "c", "state": "advertised"},
	}
	if err := dispatcher.RegisterWithOptions(pipeListPath, func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		rows := all
		if slices.ContainsFunc(args.PipeOperations(), func(op command.PipeOperation) bool { return op.Name == "received" }) {
			rows = all[:2]
		}
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"rows": rows}), nil
	}, "Read three fixture rows", pluginserver.RegisterOptions{ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	return &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			return dispatcher.Dispatch(&pluginserver.CommandContext{}, line)
		},
	}
}

// toolAnswer answers the text of a tool result and whether it is an error.
func toolAnswer(t *testing.T, result map[string]any) (string, bool) {
	t.Helper()
	content, _ := result["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatalf("result carries no content: %v", result)
	}
	text, _ := content[0]["text"].(string)
	return text, result["isError"] == true
}

// callDerived calls the derived tool with arguments.
func callDerived(t *testing.T, runner *server, arguments map[string]any) map[string]any {
	t.Helper()
	arguments["action"] = pipeListAction
	args, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	return runner.dispatchGenerated(pipeListPrefix, map[string]bool{pipeListAction: false}, args)
}

// callExecute calls ze_execute with line and answers its JSON.
func callExecute(t *testing.T, runner *server, line string) any {
	t.Helper()
	args, err := json.Marshal(map[string]string{"command": line})
	if err != nil {
		t.Fatal(err)
	}
	text, failed := toolAnswer(t, toolHandlers[toolNameExecute](runner, args))
	if failed {
		t.Fatalf("ze_execute refused %q: %s", line, text)
	}
	var answer any
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("ze_execute answer %q is not JSON: %v", text, err)
	}
	return answer
}

// TestDerivedToolPipeList asks whether a derived MCP tool takes the ordered
// pipe list and answers what the typed line answers (AC-15).
//
// Method: the derived tool of a command that declares `received`, over the
// real dispatcher. The list `received` then `count` must answer what
// ze_execute answers for `<path> | received | count`, the count of 2; with
// the list absent, what the bare command answers; a name the command does not
// take is refused naming the valid ones; and the order is kept, so `count`
// then `received` is refused by the dispatcher, which takes a declared
// operation only before a generic one.
//
// MUTATION: drop WritePipeList from dispatchGenerated; the answer is the three
// rows. Sort the list; the reversed order succeeds.
func TestDerivedToolPipeList(t *testing.T) {
	runner := pipeListRunner(t)

	text, failed := toolAnswer(t, callDerived(t, runner, map[string]any{
		command.PipeListParameter: []map[string]any{{"name": "received"}, {"name": "count"}},
	}))
	if failed {
		t.Fatalf("the pipe list was refused: %s", text)
	}
	var answer any
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("answer %q is not JSON: %v", text, err)
	}
	if typed := callExecute(t, runner, pipeListPath+" | received | count"); !reflect.DeepEqual(answer, typed) {
		t.Errorf("pipe list answer = %v, the typed line answers %v", answer, typed)
	}
	if count, _ := answer.(map[string]any)["count"].(float64); count != 2 {
		t.Errorf("count = %v, want 2: received did not reach the handler: %v", count, answer)
	}

	text, failed = toolAnswer(t, callDerived(t, runner, map[string]any{}))
	if failed {
		t.Fatalf("the bare command was refused: %s", text)
	}
	var bare any
	if err := json.Unmarshal([]byte(text), &bare); err != nil {
		t.Fatalf("answer %q is not JSON: %v", text, err)
	}
	if typed := callExecute(t, runner, pipeListPath); !reflect.DeepEqual(bare, typed) {
		t.Errorf("answer with no list = %v, the bare command answers %v", bare, typed)
	}

	text, failed = toolAnswer(t, callDerived(t, runner, map[string]any{
		command.PipeListParameter: []map[string]any{{"name": "bogus"}},
	}))
	if !failed {
		t.Fatalf("an unknown operation was accepted: %s", text)
	}
	for _, valid := range []string{"received", "count"} {
		if !strings.Contains(text, valid) {
			t.Errorf("refusal does not name the valid operation %q: %s", valid, text)
		}
	}

	text, failed = toolAnswer(t, callDerived(t, runner, map[string]any{
		command.PipeListParameter: []map[string]any{{"name": "count"}, {"name": "received"}},
	}))
	if !failed {
		t.Errorf("count then received was accepted, so the list order was not the chain order: %s", text)
	}
}

// TestDerivedToolPipeListElementCannotOpenAChain asks whether a derived tool's
// pipe list element whose name or argument word holds a pipe character or a
// quote opens no stage of its own (AC-39).
//
// Method: a recording dispatcher captures the line the tool sends. A name
// holding `|` or `"` must be refused naming the valid list and dispatch
// nothing; an argument word holding one must arrive as one word of one stage,
// read back by TypedLineTokens.
//
// MUTATION: write each word raw; `a|b` opens a stage `b`.
func TestDerivedToolPipeListElementCannotOpenAChain(t *testing.T) {
	command.RegisterPipeFilters([]string{pipeListPath},
		command.PipeFilter{Name: "received", Description: "Only the received rows", Argument: command.ArgNone})
	var sent []string
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			sent = append(sent, line)
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"ok": true}), nil
		},
	}

	for _, name := range []string{"count | received", `count"`, "received|count"} {
		text, failed := toolAnswer(t, callDerived(t, runner, map[string]any{
			command.PipeListParameter: []map[string]any{{"name": name}},
		}))
		if !failed {
			t.Errorf("name %q was accepted", name)
		}
		// The names refused hold received and count themselves, so the valid
		// list is proven by an operation none of them spells.
		if !strings.Contains(text, "operations: ") || !strings.Contains(text, "match") {
			t.Errorf("refusal of %q does not name the valid list: %s", name, text)
		}
	}
	if len(sent) != 0 {
		t.Fatalf("a refused list dispatched %q", sent)
	}

	for _, word := range []string{"a|b", `a"b`, "a | b"} {
		sent = nil
		text, failed := toolAnswer(t, callDerived(t, runner, map[string]any{
			command.PipeListParameter: []map[string]any{{"name": "match", "args": []string{word}}},
		}))
		if failed {
			t.Fatalf("word %q was refused: %s", word, text)
		}
		if len(sent) != 1 {
			t.Fatalf("word %q dispatched %q, want one line", word, sent)
		}
		tokens, err := command.TypedLineTokens(sent[0])
		if err != nil {
			t.Fatalf("read sent line %q: %v", sent[0], err)
		}
		want := []string{"show", "fixture", "mcp", "pipe", "|", "match", word}
		if !slices.Equal(tokens, want) {
			t.Errorf("word %q sent %q, tokens %q, want %q", word, sent[0], tokens, want)
		}
	}
}

// TestDerivedToolPipeListKeyIsExact asks whether only the exact `pipe` member
// is the pipe list, so a `Pipe` member is an unknown parameter like any other
// key and is not also read as the list.
//
// Method: a recording dispatcher captures the line the tool sends for
// {"Pipe": [{"name": "count"}]}. The line must open no pipe stage, and must
// carry the `Pipe` word once, forwarded as every unknown parameter is.
//
// MUTATION: read the list through a struct field tagged `json:"pipe"`, which
// matches keys case-insensitively; the line gains a `| count` stage.
func TestDerivedToolPipeListKeyIsExact(t *testing.T) {
	var sent []string
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, line string) (*plugin.Response, error) {
			sent = append(sent, line)
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"ok": true}), nil
		},
	}

	text, failed := toolAnswer(t, callDerived(t, runner, map[string]any{
		"Pipe": []map[string]any{{"name": "count"}},
	}))
	if failed {
		t.Fatalf("a call carrying an unknown parameter was refused by the tool: %s", text)
	}
	if len(sent) != 1 {
		t.Fatalf("dispatched %q, want one line", sent)
	}
	tokens, err := command.TypedLineTokens(sent[0])
	if err != nil {
		t.Fatalf("read sent line %q: %v", sent[0], err)
	}
	if slices.Contains(tokens, "|") {
		t.Errorf("a \"Pipe\" member was read as the pipe list: sent %q", sent[0])
	}
	pipeWords := 0
	for _, token := range tokens {
		if token == "Pipe" {
			pipeWords++
		}
	}
	if pipeWords != 1 {
		t.Errorf("sent %q carries the \"Pipe\" parameter %d times, want once", sent[0], pipeWords)
	}
}

// TestDerivedToolPublishesThePipeList asks whether every derived tool's input
// schema carries the pipe list as the one schema the command package declares,
// the value the OpenAPI execute body publishes too.
//
// MUTATION: build a schema of its own in buildToolDef; the two differ.
func TestDerivedToolPublishesThePipeList(t *testing.T) {
	tool, err := buildToolDef(toolGroup{prefix: pipeListPrefix, actions: []action{{name: pipeListAction, full: pipeListPath}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := tool[schemaKeyInputSchema].(json.RawMessage)
	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("input schema %s: %v", raw, err)
	}
	encoded, err := json.Marshal(command.PipeListSchema())
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal(encoded, &want); err != nil {
		t.Fatal(err)
	}
	if got := schema.Properties[command.PipeListParameter]; !reflect.DeepEqual(got, want) {
		t.Errorf("tool publishes %v for %q, want the command package's schema %v", got, command.PipeListParameter, want)
	}
}
