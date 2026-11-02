//go:build ze_core

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

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

// VALIDATES: `ze pipe help` preserves the stream class instead of grouping it
// with row operators.
// PREVENTS: `log` being presented as a row operator or as unconditional on a
// command that answers only once.
func TestPipeHelpSeparatesStreamingOperators(t *testing.T) {
	out := captureStderr(t, pipeUsage)
	if !strings.Contains(out, "Streaming Operators") {
		t.Fatalf("ze pipe help has no streaming-operator section:\n%s", out)
	}
	if !strings.Contains(out, "where the command keeps answering") {
		t.Fatalf("ze pipe help drops the streaming availability qualifier:\n%s", out)
	}
}

// TestPipeRootSubsNamesEveryOperator holds the one-line summary in root help,
// which `ze help ai --json` also publishes, to the same set.
func TestPipeRootSubsNamesEveryOperator(t *testing.T) {
	ensureLocalCommandsRegistered()

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
		if published.LocalOnly != op.LocalOnly {
			t.Errorf("%s: local-only published %v, catalog says %v", op.Name, published.LocalOnly, op.LocalOnly)
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

// VALIDATES: Machine-readable operator contracts publish the catalog's surface
// restriction rather than describing save as available on remote surfaces.
// PREVENTS: SSH and web callers learning `save` is unconditional even though
// the daemon effect-boundary guard refuses it.
func TestPipeCatalogJSONPublishesLocalOnlySave(t *testing.T) {
	var buf bytes.Buffer
	if code := printPipeCatalogJSON(&buf); code != 0 {
		t.Fatalf("printPipeCatalogJSON returned %d", code)
	}
	var got []pipeOperatorJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("catalog JSON does not parse: %v", err)
	}
	for _, op := range got {
		if op.Name == "save" {
			if !op.LocalOnly {
				t.Fatal("save is not published as local-only")
			}
			return
		}
	}
	t.Fatal("save is absent from the published operator catalog")
}

type standalonePTRResolver struct{}

func (standalonePTRResolver) ResolvePTR(string) ([]string, error) {
	return []string{"standalone.example."}, nil
}

type standaloneOriginResolver struct{}

func (standaloneOriginResolver) LookupOrigin(_ context.Context, _ string) (command.OriginResult, error) {
	return command.OriginResult{ASN: 64500, Name: "standalone", Prefix: "192.0.2.0/24"}, nil
}

// TestStandalonePipeAddressOperatorsWalkArbitraryFields drives runPipe over its
// stdin boundary rather than inventing a command declaration for a synthetic
// command name.
//
// VALIDATES: IR2-12 -- resolve and origin enrich every address-valued field on
// standalone JSON, while command validation still refuses an undeclared path.
// PREVENTS: an exemption for "_" in validateDeclaredShape making command paths
// guess address fields again.
func TestStandalonePipeAddressOperatorsWalkArbitraryFields(t *testing.T) {
	command.SetPTRResolver(standalonePTRResolver{})
	command.SetOriginResolver(standaloneOriginResolver{})
	t.Cleanup(func() {
		command.SetPTRResolver(nil)
		command.SetOriginResolver(nil)
	})

	input := `{"router-id":"192.0.2.1","nested":{"peer":"198.51.100.2","state":"up"}}`
	stdout, stderr, code := runPipeCaptured(t, input, []string{"resolve", "|", "origin"})
	if code != 0 {
		t.Fatalf("runPipe returned %d: %s", code, stderr)
	}
	for _, key := range []string{
		"router-id-name", "router-id-asn", "router-id-prefix",
		"peer-name", "peer-asn", "peer-prefix",
	} {
		if !strings.Contains(stdout, `"`+key+`"`) {
			t.Errorf("standalone output lacks %q: %s", key, stdout)
		}
	}
	if !strings.Contains(stdout, `"state":"up"`) {
		t.Errorf("standalone enrichment changed unrelated data: %s", stdout)
	}

	if _, _, errMsg := command.ProcessPipesDefaultFormatLocal("_ | resolve", ""); !strings.Contains(errMsg, "resolve") {
		t.Errorf("generic command validation accepted the synthetic spelling: %q", errMsg)
	}
}

func runPipeCaptured(t *testing.T, input string, args []string) (string, string, int) {
	t.Helper()
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	if _, err := stdinWrite.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := stdinWrite.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	savedStdin, savedStdout, savedStderr := os.Stdin, os.Stdout, os.Stderr
	os.Stdin, os.Stdout, os.Stderr = stdinRead, stdoutWrite, stderrWrite
	type readResult struct {
		text string
		err  error
	}
	stdoutResult := make(chan readResult, 1)
	stderrResult := make(chan readResult, 1)
	go func() {
		data, readErr := io.ReadAll(stdoutRead)
		stdoutResult <- readResult{text: string(data), err: readErr}
	}()
	go func() {
		data, readErr := io.ReadAll(stderrRead)
		stderrResult <- readResult{text: string(data), err: readErr}
	}()

	code := runPipe(args)
	stdoutCloseErr := stdoutWrite.Close()
	stderrCloseErr := stderrWrite.Close()
	os.Stdin, os.Stdout, os.Stderr = savedStdin, savedStdout, savedStderr
	stdinCloseErr := stdinRead.Close()
	stdout := <-stdoutResult
	stderr := <-stderrResult
	stdoutReadCloseErr := stdoutRead.Close()
	stderrReadCloseErr := stderrRead.Close()
	for _, closeErr := range []error{
		stdoutCloseErr, stderrCloseErr, stdinCloseErr,
		stdoutReadCloseErr, stderrReadCloseErr, stdout.err, stderr.err,
	} {
		if closeErr != nil {
			t.Fatalf("capture pipe: %v", closeErr)
		}
	}
	return stdout.text, stderr.text, code
}
