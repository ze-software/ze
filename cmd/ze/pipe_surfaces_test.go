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

// TestVerboseHelpNamesTheGlobalOperators holds `ze help command --verbose` to
// what it is actually reporting. It printed one hand-typed line of ten for
// every command, which was wrong in both directions: it omitted catalog
// operators and asserted row operators that an answer holding one value cannot
// support.
//
// It now reports the command's OWN operators. Answer availability and
// local-process restrictions remain independent dimensions.
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
	if !strings.Contains(out, "  pipes:\n") {
		t.Errorf("verbose help omits the pipes heading: %q", out)
	}
	if !strings.Contains(out, "    always: json, ndjson, table, text, yaml, raw, no-more, save\n") {
		t.Errorf("verbose help always line does not include local-only save: %q", out)
	}
	if !strings.Contains(out, "    local process only: save\n") {
		t.Errorf("verbose help local-process line does not contain exactly save: %q", out)
	}

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
	if !strings.Contains(out, "while the command keeps answering:") {
		t.Errorf("verbose help drops the streaming qualifier: %q", out)
	}
	if !strings.Contains(out, "local process only:") {
		t.Errorf("verbose help presents save as available on remote surfaces: %q", out)
	}
}

// VALIDATES: A local printing shortcut does not suppress a separately
// registered daemon handler's reachable pipe layer.
// PREVENTS: `show version` publishing no operators because its local shortcut
// is inspected before its daemon RPC registration.
func TestDualRegisteredDaemonCommandPublishesOperators(t *testing.T) {
	ensureLocalCommandsRegistered()
	if !registry.HasLocal("show version") {
		t.Fatal("show version has no local shortcut; this test no longer covers the dual-registration case")
	}
	if command.HasLocalData("show version") {
		t.Fatal("show version now has a local data handler; this test no longer covers a plain local shortcut")
	}
	if !daemonHandlesPath("show version") {
		t.Fatal("show version has no daemon handler; this test no longer covers dual registration")
	}
	ops, _ := operatorsFor("show version")
	if len(ops) == 0 {
		t.Fatal("show version publishes no operators although its daemon handler reaches the pipe layer")
	}
	var saveLocal, logStreaming bool
	for _, op := range ops {
		switch op.Name {
		case "save":
			saveLocal = op.LocalOnly
		case "log":
			logStreaming = op.Available == "when-streaming"
		}
	}
	if !saveLocal {
		t.Fatal("show version does not publish save's local-only surface restriction")
	}
	if !logStreaming {
		t.Fatal("show version does not publish log's streaming qualifier")
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

// TestACommandServedWithoutDataPublishesNoOperators holds the catalog to what
// the product can honor. A command served only by a plain local handler reaches
// no pipe layer. A dual-registered path is covered separately because its
// daemon surface can still pipe.
//
// `show data cat` and `show yang doc` are local-only plain handlers by design,
// and they published fifteen operators each before this.
func TestACommandServedWithoutDataPublishesNoOperators(t *testing.T) {
	ensureLocalCommandsRegistered()

	for _, path := range []string{"show data cat", "show yang doc"} {
		if !registry.HasLocal(path) {
			t.Fatalf("%s is not served locally; this test no longer covers its case", path)
		}
		if command.HasLocalData(path) {
			t.Fatalf("%s answers with data now; move it out of this test rather than deleting the check", path)
		}
		if daemonHandlesPath(path) {
			t.Fatalf("%s has a daemon handler now; move it to the dual-registration test", path)
		}
		ops, shape := operatorsFor(path)
		if len(ops) != 0 {
			t.Errorf("%s publishes %d operators and reaches no pipe layer", path, len(ops))
		}
		if shape != "" {
			t.Errorf("%s publishes shape %q and reaches no pipe layer", path, shape)
		}
	}

	// The converted sibling still publishes, so the check above is not passing
	// because operatorsFor answers nothing for everything.
	if ops, _ := operatorsFor("show data registered"); len(ops) == 0 {
		t.Error("show data registered publishes no operators; it answers with data")
	}
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
