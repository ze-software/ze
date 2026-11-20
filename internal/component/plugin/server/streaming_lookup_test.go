// Design: docs/architecture/api/commands.md -- the stream route's one lookup partitions the typed line.

package server

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestStreamingLookupPartitions asks whether the stream route's one lookup,
// handed a typed line, answers a value carrying the operations the monitor
// declares with no chain word among its arguments, runs the residual chain
// over the events with the chain's `pipe` metadata, answering each event as
// the one-shot route answers the same line, and refuses an unfinished quote
// in the chain (AC-35).
//
// Method: a monitor declares `only`; the line holds an argument, that
// operation and a generic `| json`. The handler writes one compact event.
// MUTATION: drop WithPipeOperations in GetStreamingHandlerRendered; the
// declared list comparison fails.
func TestStreamingLookupPartitions(t *testing.T) {
	const prefix = "monitor test streaming-lookup-partitions"
	var handed command.ValidatedArgs
	RegisterStreamingHandler(prefix, OneShotStreamingOnly,
		func(_ context.Context, _ *Server, output io.Writer, _ string, args command.ValidatedArgs) error {
			handed = args
			_, err := io.WriteString(output, "{\"state\":\"up\"}\n")
			return err
		})
	t.Cleanup(func() { UnregisterStreamingHandler(prefix) })
	t.Cleanup(command.ResetPipeFiltersForTest)
	command.RegisterPipeFilters([]string{prefix},
		command.PipeFilter{Name: "only", Description: "Keep the events of one state", Argument: command.ArgText})

	handler, validated, err := GetStreamingHandlerForCommand(prefix + " alpha | only up | json")
	if err != nil {
		t.Fatalf("the typed line was refused: %v", err)
	}
	if handler == nil {
		t.Fatal("no streaming handler found for the typed line")
	}
	var output bytes.Buffer
	if err := handler(t.Context(), nil, &output, "alice", validated); err != nil {
		t.Fatal(err)
	}
	if got := handed.Tokens(); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("arguments = %q, want [alpha]", got)
	}
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	if got := handed.PipeOperations(); !slices.EqualFunc(want, got, func(a, b command.PipeOperation) bool {
		return a.Name == b.Name && slices.Equal(a.Args, b.Args)
	}) {
		t.Errorf("declared list = %+v, want %+v", got, want)
	}
	// The declared `only up` shaped the event, so the `| json` answer names
	// it under "pipe", as every stream route did before the partition and as
	// the one-shot route answers the same line (collectPipeMeta).
	wantEvents := "{\n  \"pipe\": [\n    {\n      \"arg\": \"up\",\n      \"op\": \"only\"\n    }\n  ],\n  \"state\": \"up\"\n}\n"
	if output.String() != wantEvents {
		t.Errorf("events = %q, want %q: the residual `| json` was not applied with the chain's metadata",
			output.String(), wantEvents)
	}
	_, oneShot, msg := command.ProcessPipesDefaultFormatChecked(prefix+" alpha | only up | json", "")
	if msg != "" {
		t.Fatalf("the one-shot route refused the line: %s", msg)
	}
	if got := oneShot("{\"state\":\"up\"}") + "\n"; got != output.String() {
		t.Errorf("stream event = %q, one-shot answer = %q: the two routes disagree", output.String(), got)
	}

	if _, _, err := GetStreamingHandlerForCommand(prefix + ` alpha | match "up`); err == nil {
		t.Error("an unfinished quote in the chain was accepted")
	}
}

// TestEveryPipeSplitGoesThroughThePartition asks R-11's question of the stream
// route: does every reader of the streaming-handler registry reach it through
// the one lookup that partitions the chain? A function of this package that
// reads the registry or matches a prefix outside the registry's own functions
// and GetStreamingHandlerRendered is a handler lookup that bypasses the
// partition, and fails this test.
//
// Method: parse the package's non-test files and name every function that
// refers to streamingHandlers or matchStreamingCommand.
func TestEveryPipeSplitGoesThroughThePartition(t *testing.T) {
	allowed := map[string][]string{
		// The registry's own functions: they store, list or remove entries.
		"streamingHandlers": {"registerStreaming", "StreamingOneShot", "StreamingPluginCommand",
			"StreamingOnlyPrefixes", "UnregisterStreamingHandler", "StreamingPrefixes", "matchStreamingCommand"},
		// The one lookup, and the yes/no question that runs no handler.
		"matchStreamingCommand": {"GetStreamingHandlerRendered", "IsStreamingCommand"},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	seen := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(files, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			owner := "a package-level value"
			var roots []ast.Node
			switch d := decl.(type) {
			case *ast.FuncDecl:
				owner = d.Name.Name
				roots = append(roots, d)
			case *ast.GenDecl:
				// A declaration names the registry; only a value it is
				// initialized from can read it.
				for _, spec := range d.Specs {
					if value, isValue := spec.(*ast.ValueSpec); isValue {
						for _, expr := range value.Values {
							roots = append(roots, expr)
						}
					}
				}
			}
			for _, root := range roots {
				ast.Inspect(root, func(node ast.Node) bool {
					ident, isIdent := node.(*ast.Ident)
					if !isIdent {
						return true
					}
					callers, guarded := allowed[ident.Name]
					if !guarded {
						return true
					}
					seen++
					if owner == ident.Name || slices.Contains(callers, owner) {
						return true
					}
					t.Errorf("%s: %s refers to %s, a streaming lookup around GetStreamingHandlerRendered",
						files.Position(ident.Pos()), owner, ident.Name)
					return true
				})
			}
		}
	}
	if seen == 0 {
		t.Fatal("no reference to the streaming registry found: the scan read nothing")
	}
}
