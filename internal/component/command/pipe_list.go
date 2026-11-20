// Design: docs/architecture/api/commands.md — the ordered pipe list a structured route takes
// Related: pipe_operations.go — the names a pipe list element is judged against
// Related: arguments.go — WriteInvocationValue writes each argument word
//
// pipe_list.go is the one declaration of the `pipe` list that a route taking
// structured parameters accepts: every derived MCP tool, REST execute, gRPC
// Execute and Stream. Its schema, its element and the way it becomes the chain
// of the invocation all live here, so the MCP tool schema and the OpenAPI
// document publish the same list and every adapter writes it the same way.

package command

import (
	"bytes"
	"encoding/json"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// PipeListParameter is the name of the optional, ordered pipe list a route
// taking structured parameters accepts beside them.
const PipeListParameter = "pipe"

// The keys of one pipe list element, as JSON carries it.
const (
	PipeListKeyName      = "name"
	PipeListKeyArguments = "args"
)

// PipeListElement is one element of the pipe list: an operation name and its
// argument words, in the order the operation takes them. The adapter never
// splits a word: each one reaches the operation as one word, whatever it holds.
type PipeListElement struct {
	Name string   `json:"name"`
	Args []string `json:"args,omitempty"`
}

// PipeListSchema answers the JSON Schema of the pipe list. The MCP tool input
// schema and the OpenAPI document both publish it, so it is built here once.
// Every call answers a fresh value: the caller puts it into a schema it owns.
// Every value in it is a string, a []string or a map of those, which
// json.Marshal always encodes.
func PipeListSchema() map[string]any {
	return map[string]any{
		"type": "array",
		"description": "Pipe operations to apply to the answer, in order, as if typed after the command " +
			"(`| name args`). Each name is one of the command's pipe operations: " +
			"`show command help <command>` lists them, and any other name is refused.",
		"items": map[string]any{
			"type": "object",
			"properties": map[string]any{
				PipeListKeyName: map[string]any{
					"type":        "string",
					"description": "A pipe operation of the command, such as count or match",
				},
				PipeListKeyArguments: map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "The operation's argument words; each element is one word, whatever it holds",
				},
			},
			"required": []string{PipeListKeyName},
		},
	}
}

// DecodePipeList reads the pipe list from its JSON form, as REST execute and a
// derived MCP tool receive it. An absent list (no bytes, or JSON null) answers
// nil. A key an element does not define is refused, never ignored: a misspelt
// `args` would otherwise drop the arguments and run the operation without
// them.
func DecodePipeList(raw []byte) ([]PipeListElement, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var chain []PipeListElement
	if err := decoder.Decode(&chain); err != nil {
		var problem textbuf.Buffer
		problem.Str("The ").Quoted(PipeListParameter).Str(" list cannot be read: ").Str(err.Error())
		return nil, opererr.New(problem.String(),
			"No command ran. The list is an array of objects, each with a name and an optional args array of strings.",
			`send "pipe": [{"name": "count"}] or [{"name": "match", "args": ["established"]}]`)
	}
	if decoder.More() {
		var problem textbuf.Buffer
		problem.Str("The ").Quoted(PipeListParameter).Str(" list is followed by more JSON")
		return nil, opererr.New(problem.String(),
			"No command ran. The list is one JSON array.",
			`send "pipe": [{"name": "count"}]`)
	}
	return chain, nil
}

// WritePipeList writes chain after what tb holds, as the pipe chain of the
// invocation of commandPath: ` | name word word`, one stage per element, in
// order. Each name MUST be one of PipeOperationsForCommand(commandPath), and a
// name that is not is refused naming the valid list, before anything is
// written. Each argument word is written by WriteInvocationValue, so a word
// holding a pipe, a quote or a space stays one word and opens no stage.
//
// The dispatcher still judges each stage's arguments when it partitions the
// chain: this check is the adapter's, so a caller learns the valid names from
// the route it used rather than from a chain the dispatcher refused.
func WritePipeList(tb *textbuf.Buffer, commandPath string, chain []PipeListElement) error {
	if len(chain) == 0 {
		return nil
	}
	operations := PipeOperationsForCommand(commandPath)
	for index := range chain {
		if !pipeListNameValid(operations, chain[index].Name) {
			return pipeListNameRefusal(commandPath, index, chain[index].Name, operations)
		}
	}
	for index := range chain {
		tb.Str(" | ").Str(chain[index].Name)
		for _, word := range chain[index].Args {
			tb.Byte(' ')
			WriteInvocationValue(tb, word)
		}
	}
	return nil
}

// pipeListNameValid answers whether name is one of the operations listed.
// The match is exact: the chain grammar reads an operation name as typed.
func pipeListNameValid(operations []PipeOperationEntry, name string) bool {
	for index := range operations {
		if operations[index].Name == name {
			return true
		}
	}
	return false
}

// pipeListNameRefusal names the element refused, the command, and every
// operation the command takes, in the order PipeOperationsForCommand lists
// them.
func pipeListNameRefusal(commandPath string, index int, name string, operations []PipeOperationEntry) error {
	var problem, why, fix textbuf.Buffer
	problem.Str("Pipe list element ").Int(int64(index + 1)).Str(" names ").Quoted(name).
		Str(", which is not a pipe operation of ").Quoted(commandPath)
	why.Str("No command ran. ")
	if len(operations) == 0 {
		why.Str("The command takes no pipe operation, so its pipe list must be empty.")
	} else {
		names := make([]string, len(operations))
		for i := range operations {
			names[i] = operations[i].Name
		}
		why.Str("The command takes these pipe operations: ").Join(names, ", ").Byte('.')
	}
	fix.Str("ze help command ")
	WriteInvocationValue(&fix, commandPath)
	return opererr.New(problem.String(), why.String(), fix.String())
}
