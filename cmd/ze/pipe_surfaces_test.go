//go:build ze_core

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/cmd/ze/internal/helpfmt"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// captureStderr runs fn with os.Stderr redirected and answers what it wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = saved
	out := <-done
	_ = r.Close()
	return out
}

// namesOperator answers whether the text names the operator as a WORD. A
// substring test is not sound here: "json" sits inside "ndjson", so a page
// naming only ndjson would report json as published too.
func namesOperator(text, name string) bool {
	return slices.Contains(strings.FieldsFunc(text, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-'
	}), name)
}

// TestPipeHelpNamesEveryOperator holds `ze pipe help` to the whole operator
// language. It published ten of sixteen, omitting raw, origin, log, no-more,
// display and fill — and display and fill appeared in no user-reachable list at
// all, though they are the two a tool author most needs, being how a caller
// asks for a stable field set.
func TestPipeHelpNamesEveryOperator(t *testing.T) {
	out := captureStderr(t, pipeUsage)
	for _, op := range command.PipeOperatorCatalog() {
		if !namesOperator(out, op.Name) {
			t.Errorf("ze pipe help does not name %q", op.Name)
		}
	}
}

// TestPipeRootSubsNamesEveryOperator holds the one-line summary in root help,
// which `ze help ai --json` also publishes, to the same set.
func TestPipeRootSubsNamesEveryOperator(t *testing.T) {
	registerLocalCommands()

	var subs string
	found := false
	for _, rc := range registry.ListRoot() {
		if rc.Name == "pipe" {
			subs = rc.Meta.ResolveSubs()
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no pipe root command registered")
	}
	for _, op := range command.PipeOperatorCatalog() {
		if !namesOperator(subs, op.Name) {
			t.Errorf("pipe root Subs does not name %q: %s", op.Name, subs)
		}
	}
}

// TestVerboseHelpNamesTheGlobalOperators holds `ze help command --verbose` to
// what it is actually reporting. It printed one hand-typed line of ten for
// every command, which was wrong in both directions: it omitted two globals
// (raw, log) and asserted four row operators (match, count, resolve, origin)
// that an answer holding one value cannot support.
//
// It now reports the command's OWN operators, split by whether they always
// apply or apply only to an answer carrying rows.
func TestVerboseHelpNamesTheGlobalOperators(t *testing.T) {
	entry := commandEntry{
		Path:        "show test",
		Description: "a command that reaches the pipe layer",
	}
	entry.Operators, entry.AnswerShape = operatorsFor(entry.Path)

	var buf bytes.Buffer
	rw := helpfmt.NewRenderWriter(&buf)
	printCommandVerbose(rw, []commandEntry{entry})
	out := buf.String()

	for _, op := range command.PipeOperatorCatalog() {
		named := namesOperator(out, op.Name)
		// An undeclared command publishes every global always, and every row
		// operator as depending on the answer. The two that need a declared
		// address field are published by neither, because nothing could honor
		// them.
		want := !op.NeedsAddressField
		if named != want {
			t.Errorf("verbose help names %q = %v, want %v", op.Name, named, want)
		}
	}
	if !strings.Contains(out, "always:") {
		t.Errorf("verbose help does not separate what always applies: %q", out)
	}
	if !strings.Contains(out, "when the answer has rows:") {
		t.Errorf("verbose help does not say which operators depend on the answer: %q", out)
	}
}

// TestPipeCatalogJSONPublishesEveryContract holds the machine surface a tool
// author reads. Before it, the only place all sixteen names reached a machine
// was an authenticated web completion endpoint; every CLI surface carried a
// hand-typed subset, and `ze help command --json` carried a boolean.
func TestPipeCatalogJSONPublishesEveryContract(t *testing.T) {
	var buf bytes.Buffer
	if code := printPipeCatalogJSON(&buf); code != 0 {
		t.Fatalf("printPipeCatalogJSON returned %d", code)
	}
	var got []pipeOperatorJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("catalog JSON does not parse: %v", err)
	}

	want := command.PipeOperatorCatalog()
	if len(got) != len(want) {
		t.Fatalf("published %d operators, catalog holds %d", len(got), len(want))
	}
	for i, op := range want {
		published := got[i]
		if published.Name != op.Name {
			t.Errorf("position %d publishes %q, catalog holds %q", i, published.Name, op.Name)
			continue
		}
		if published.Class != op.Class.String() {
			t.Errorf("%s: class published %q, catalog says %q", op.Name, published.Class, op.Class.String())
		}
		if published.Repeat != op.Repeat.String() {
			t.Errorf("%s: repeat published %q, catalog says %q", op.Name, published.Repeat, op.Repeat.String())
		}
		if published.Description != op.Description {
			t.Errorf("%s: description published %q, catalog says %q", op.Name, published.Description, op.Description)
		}
		if len(published.Shapes) != len(op.Shapes()) {
			t.Errorf("%s: publishes shapes %v, catalog says %v", op.Name, published.Shapes, op.Shapes())
		}
		// A global operator that published fewer than every shape would tell a
		// tool author it is refusable, which is the claim this class denies.
		if op.Class == command.ClassGlobal && len(published.Shapes) != 3 {
			t.Errorf("%s is global but publishes shapes %v", op.Name, published.Shapes)
		}
	}
}
