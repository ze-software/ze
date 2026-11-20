// Design: docs/architecture/api/commands.md -- the ordered pipe list a structured route takes.
package api

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// pipeListPath is a command only these tests register: it declares one
// operation of its own, `received`, so its pipe operations are that and the
// generics the validator admits for an unshaped command.
const pipeListPath = "show fixture api pipe list"

// pipeListEngine answers an engine whose metadata holds pipeListPath, after
// declaring its operation. The executor is never called by these tests.
func pipeListEngine(t *testing.T) *APIEngine {
	t.Helper()
	command.RegisterPipeFilters([]string{pipeListPath},
		command.PipeFilter{Name: "received", Description: "Only the received rows", Argument: command.ArgNone})
	return NewAPIEngine(nil, func() ([]CommandMeta, error) {
		return []CommandMeta{{Name: pipeListPath, ReadOnly: true, Params: []ParamMeta{{Name: "name", Type: "string"}}}}, nil
	}, nil, nil)
}

// TestPipeListWrittenAfterParamsInOrder asks whether REST and gRPC write the
// pipe list after the structured parameters, one stage per element, in the
// order the list holds, after any chain the caller typed (AC-15, engine half).
//
// Method: prepareCommand over the line, a parameter and a two-element list;
// the built line is read back by TypedLineTokens, the tokens the accountant
// and the authorizer see: the command words, then each stage.
//
// MUTATION: write the list before the parameters, or sort it; the tokens
// differ.
func TestPipeListWrittenAfterParamsInOrder(t *testing.T) {
	engine := pipeListEngine(t)
	pipe := []command.PipeListElement{{Name: "received"}, {Name: "count"}}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"no typed chain", pipeListPath, []string{"show", "fixture", "api", "pipe", "list", "name", "a", "|", "received", "|", "count"}},
		{"after the typed chain", pipeListPath + " | json", []string{"show", "fixture", "api", "pipe", "list", "name", "a", "|", "json", "|", "received", "|", "count"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			built, readOnly, err := engine.prepareCommand(test.input, map[string]string{"name": "a"}, pipe)
			if err != nil {
				t.Fatalf("prepare %q: %v", test.input, err)
			}
			if !readOnly {
				t.Errorf("the pipe list changed the read-only verdict of %q", built)
			}
			tokens, err := command.TypedLineTokens(built)
			if err != nil {
				t.Fatalf("read built line %q: %v", built, err)
			}
			if !slices.Equal(tokens, test.want) {
				t.Errorf("tokens of %q = %q, want %q", built, tokens, test.want)
			}
		})
	}

	built, _, err := engine.prepareCommand(pipeListPath, nil, nil)
	if err != nil {
		t.Fatalf("prepare with no list: %v", err)
	}
	if built != pipeListPath {
		t.Errorf("an absent list built %q, want the bare command %q", built, pipeListPath)
	}
}

// TestPipeListElementCannotOpenAChain asks whether a pipe list element whose
// name or argument word holds a pipe character or a quote is kept from opening
// a stage of its own (AC-39, security negative).
//
// Method: a name holding `|` or `"` is not an operation of the command, so it
// must be refused naming the valid list. An argument word holding one must be
// written quoted: the built line read back by TypedLineTokens holds exactly one
// stage, with the word whole.
//
// MUTATION: write each word raw (no WriteInvocationValue); `a|b` opens a stage
// `b` and the tokens gain a pipe. Drop the name check; the bad names run.
func TestPipeListElementCannotOpenAChain(t *testing.T) {
	engine := pipeListEngine(t)

	for _, name := range []string{"count | received", `count"`, "received|count", ""} {
		_, _, err := engine.prepareCommand(pipeListPath, nil, []command.PipeListElement{{Name: name}})
		if err == nil {
			t.Fatalf("name %q was accepted", name)
		}
		for _, valid := range []string{"received", "count", "match"} {
			if !strings.Contains(err.Error(), valid) {
				t.Errorf("refusal of %q does not name the valid operation %q: %v", name, valid, err)
			}
		}
	}

	for _, word := range []string{"a|b", `a"b`, "a | b", `"`} {
		built, _, err := engine.prepareCommand(pipeListPath, nil,
			[]command.PipeListElement{{Name: "match", Args: []string{word}}})
		if err != nil {
			t.Fatalf("word %q: %v", word, err)
		}
		tokens, err := command.TypedLineTokens(built)
		if err != nil {
			t.Fatalf("read built line %q: %v", built, err)
		}
		want := []string{"show", "fixture", "api", "pipe", "list", "|", "match", word}
		if !slices.Equal(tokens, want) {
			t.Errorf("word %q built %q, tokens %q, want %q", word, built, tokens, want)
		}
	}
}

// TestPipeListRefusedWithNoCommandMatched asks whether a pipe list on a line
// no command matches is refused rather than written unjudged: there is no
// operation set to judge its names against.
//
// MUTATION: append the list when meta is nil; the call succeeds.
func TestPipeListRefusedWithNoCommandMatched(t *testing.T) {
	engine := pipeListEngine(t)
	_, _, err := engine.prepareCommand("show fixture api nothing", nil, []command.PipeListElement{{Name: "count"}})
	if err == nil {
		t.Fatal("a pipe list on an unmatched line was accepted")
	}
}

// TestOpenAPIExecutePublishesThePipeList asks whether the OpenAPI document's
// execute body publishes the pipe list as the one schema the command package
// declares, the same value a derived MCP tool publishes.
//
// MUTATION: write a schema of its own in schema.go; the two differ.
func TestOpenAPIExecutePublishesThePipeList(t *testing.T) {
	data, err := OpenAPISchema(nil)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]struct {
			Post struct {
				RequestBody struct {
					Content map[string]struct {
						Schema struct {
							Properties map[string]any `json:"properties"`
						} `json:"schema"`
					} `json:"content"`
				} `json:"requestBody"`
			} `json:"post"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	published := spec.Paths["/api/v1/execute"].Post.RequestBody.Content[mediaTypeJSON].Schema.Properties[command.PipeListParameter]
	encoded, err := json.Marshal(command.PipeListSchema())
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal(encoded, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(published, want) {
		t.Errorf("execute body publishes %v for %q, want the command package's schema %v", published, command.PipeListParameter, want)
	}
}
