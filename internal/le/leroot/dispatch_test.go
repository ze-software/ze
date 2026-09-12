// VALIDATES: the le root resolves local-data handlers and preserves payloads and codes.
// PREVENTS: standalone and tagged dispatch diverging or flattening a tool verdict.
package leroot

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/le/leaction"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = writer
	done := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = buffer.ReadFrom(reader)
		done <- buffer.String()
	}()
	fn()
	_ = writer.Close()
	os.Stdout = saved
	out := <-done
	_ = reader.Close()
	return out
}

func TestRegisterShapeDistinguishesFullToolPaths(t *testing.T) {
	const docName = "shape-doc-discriminator"
	const mapName = "shape-map-discriminator"
	RegisterShape(docName, command.ShapeDoc)
	RegisterShape(mapName, command.ShapeMap)

	docShape, docDeclared := command.ShapeForCommand(CommandPath(docName))
	mapShape, mapDeclared := command.ShapeForCommand(CommandPath(mapName))
	if !docDeclared || docShape != command.ShapeDoc {
		t.Errorf("%s shape = %v/%v, want doc/declared", docName, docShape, docDeclared)
	}
	if !mapDeclared || mapShape != command.ShapeMap {
		t.Errorf("%s shape = %v/%v, want map/declared", mapName, mapShape, mapDeclared)
	}
	if shape, declared := command.ShapeForCommand("le"); declared {
		t.Errorf("root le inherited tool shape %v", shape)
	}
}

func TestDispatchReachesRegisteredLocalDataAndPreservesNonzeroPayload(t *testing.T) {
	const name = "dispatch-local-data-probe"
	var got []string
	Register(name, GroupReport, func(args []string) (any, int) {
		got = args
		return map[string]any{"probe": "ran", "code": 3}, 3
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeDoc)

	code := 0
	out := captureStdout(t, func() { code = Dispatch("le", []string{name, "alpha", "beta", "|", "json"}) })
	if code != 3 {
		t.Errorf("Dispatch answered %d, want the handler's 3", code)
	}
	if strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("handler arguments = %q, want [alpha beta]", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("nonzero payload is not JSON: %v\n%s", err, out)
	}
	if payload["probe"] != "ran" || payload["code"] != float64(3) {
		t.Errorf("rendered payload = %#v", payload)
	}
}

func TestDispatchUsesSharedPipeRenderers(t *testing.T) {
	const name = "pipe-local-data-probe"
	Register(name, GroupReport, func([]string) (any, int) {
		return map[string]any{
			"actions":        2,
			"native-actions": []string{"tier/check", "repository/check"},
		}, 0
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeMap)

	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			code := 0
			out := captureStdout(t, func() { code = Dispatch("le", []string{name, "|", format}) })
			if code != 0 {
				t.Fatalf("%s rendering answered %d: %s", format, code, out)
			}
			if !strings.Contains(out, "actions") || !strings.Contains(out, "tier/check") {
				t.Errorf("%s rendering dropped structured data: %q", format, out)
			}
		})
	}
	out := captureStdout(t, func() {
		if code := Dispatch("le", []string{name, "|", "match", "tier"}); code != 0 {
			t.Errorf("match rendering answered %d", code)
		}
	})
	if !strings.Contains(out, "tier/check") {
		t.Errorf("match rendering dropped the matching row: %q", out)
	}
}

func TestDispatchRefusesTwoFormatOperators(t *testing.T) {
	const name = "pipe-refusal-local-data-probe"
	Register(name, GroupReport, func([]string) (any, int) {
		return map[string]string{"probe": "ran"}, 0
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(name, command.ShapeDoc)

	code := 0
	stdout := captureStdout(t, func() {
		captureStderr(t, func() {
			code = Dispatch("le", []string{name, "|", "json", "|", "yaml"})
		})
	})
	if code != 1 {
		t.Errorf("two format operators answered %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("refused pipe chain wrote stdout: %q", stdout)
	}
}

func TestDispatchRefusesUnknownAndHandlesHelp(t *testing.T) {
	if code := Dispatch("le", []string{"no-such-tool"}); code != 1 {
		t.Errorf("unknown tool answered %d, want 1", code)
	}
	if code := Dispatch("le", []string{"--help"}); code != 0 {
		t.Errorf("help answered %d, want 0", code)
	}
}

func TestHelpRendersTheNodeWithoutRunningIt(t *testing.T) {
	const name = "help-page-local-data-probe"
	ran := 0
	Register(name, GroupReport, func([]string) (any, int) {
		ran++
		return map[string]string{"probe": "ran"}, 0
	}, registry.Meta{
		Description: "a probe that must not run for a help word",
		Mode:        "offline", Section: registry.SectionTest,
		Subs: "alpha | beta (writes)",
	})
	RegisterShape(name, command.ShapeDoc)

	for _, args := range [][]string{{name, "--help"}, {name, "-h"}, {name, "help"}, {"help", name}} {
		code := 0
		page := captureStderr(t, func() { code = Dispatch("le", args) })
		if code != 0 {
			t.Errorf("%v answered %d, want 0", args, code)
		}
		for _, want := range []string{"le " + name, "must not run", "alpha", "beta (writes)"} {
			if !strings.Contains(page, want) {
				t.Errorf("%v page missing %q: %s", args, want, page)
			}
		}
	}
	if ran != 0 {
		t.Errorf("a help word ran the command %d time(s)", ran)
	}
}

func TestHelpNamesWhatANamespaceHolds(t *testing.T) {
	const member = "help-namespace-probe member"
	Register(member, GroupReport, func([]string) (any, int) { return nil, 0 },
		registry.Meta{Description: "the member's own summary", Mode: "offline", Section: registry.SectionTest})
	RegisterShape(member, command.ShapeDoc)

	code := 0
	page := captureStderr(t, func() { code = Dispatch("le", []string{"help-namespace-probe", "--help"}) })
	if code != 0 {
		t.Errorf("namespace help answered %d, want 0", code)
	}
	for _, want := range []string{"member", "the member's own summary"} {
		if !strings.Contains(page, want) {
			t.Errorf("namespace page missing %q: %s", want, page)
		}
	}
}

// ─── The root answers a payload, and a help word runs nothing ───────────────

// VALIDATES: a bare invocation answers the manifest as a payload on stdout with
// exit 0, and the text it prints is the root help.
// PREVENTS: the root writing finished text to stderr, which is what made
// `./le | json` answer `unknown command: |`.
func TestBareRootAnswersTheManifestAsAPayload(t *testing.T) {
	code := 1
	out := captureStdout(t, func() { code = Dispatch("le", nil) })
	if code != 0 {
		t.Errorf("a bare invocation answered %d, want 0", code)
	}
	if want := manifestOf("le").Text(); out != want {
		t.Errorf("the bare root printed\n%q\nwant the manifest's own text\n%q", out, want)
	}
	if !strings.Contains(out, "the Ze repository and development entry point") {
		t.Errorf("the bare root printed no root help:\n%s", out)
	}
}

// VALIDATES: the manifest reaches the pipe operators, so `./le | json` renders
// one document naming every area, its group and its actions.
// PREVENTS: a payload the operator chain cannot reach, which is a rendering
// picked for the reader (ai/rules/cli.md).
func TestRootManifestRendersThroughTheJSONOperator(t *testing.T) {
	const name = "manifest-json-probe"
	Register(name, GroupReport, func([]string) (any, int) { return nil, 0 },
		registry.Meta{Description: "a probe the manifest names", Mode: "offline", Section: registry.SectionTest})

	code := 1
	out := captureStdout(t, func() { code = Dispatch("le", []string{"|", "json"}) })
	if code != 0 {
		t.Fatalf("`le | json` answered %d: %s", code, out)
	}

	var manifest struct {
		Program string `json:"program"`
		Areas   []struct {
			Name        string `json:"name"`
			Group       string `json:"group"`
			Description string `json:"description"`
		} `json:"areas"`
	}
	if err := json.Unmarshal([]byte(out), &manifest); err != nil {
		t.Fatalf("`le | json` is not one JSON document: %v\n%s", err, out)
	}
	if manifest.Program != "le" {
		t.Errorf("the document names the program %q, want le", manifest.Program)
	}
	for _, area := range manifest.Areas {
		if area.Name != name {
			continue
		}
		if area.Group != string(GroupReport) || area.Description != "a probe the manifest names" {
			t.Errorf("the rendered area is %+v", area)
		}
		return
	}
	t.Errorf("the rendered document does not name %q: %s", name, out)
}

// VALIDATES: a help word at the end of an invocation renders usage and returns
// WITHOUT calling the handler, for an area that declares its actions and for
// one that hand-rolls its own dispatch.
// PREVENTS: AC-6, the reason this spec exists: `./le stress-repro run suite
// --help` reads the help word as the suite name and starts a multi-hour burn.
func TestATrailingHelpWordNeverReachesTheHandler(t *testing.T) {
	const burner = "trailing-help-burn-probe"
	burns := 0
	Register(burner, GroupSuite, func([]string) (any, int) {
		burns++
		return map[string]string{"probe": "the burn started"}, 0
	}, registry.Meta{
		Description: "a probe that starts work from its first argument",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterShape(burner, command.ShapeDoc)

	// The hand-rolled shape: no action table is registered, so the dispatcher
	// can guard the area but cannot render its grammar.
	for _, args := range [][]string{
		{burner, "run", "suite", "gating", "--help"},
		{burner, "run", "suite", "gating", "-h"},
		{burner, "run", "suite", "gating", "-help"},
		{burner, "run", "suite", "gating", "help"},
		{burner, "run", "--help"},
		{burner, "--help"},
		{burner, "run", "--help", "|", "json"},
	} {
		code := 1
		page := captureStderr(t, func() { code = Dispatch("le", args) })
		if code != 0 {
			t.Errorf("%v answered %d, want 0", args, code)
		}
		if burns != 0 {
			t.Fatalf("%v reached the handler: the burn ran %d time(s)", args, burns)
		}
		if !strings.Contains(page, "le "+burner) {
			t.Errorf("%v printed no usage: %q", args, page)
		}
	}

	// The declared shape: the dispatcher renders the action's own grammar from
	// the listing the area registered, and still calls nothing.
	area := leaction.New("trailing-help-grammar-probe", leaction.Action{
		Verb: "run", Why: "run the probe over one scope",
		Parameters: []leaction.Parameter{
			{Keyword: "scope", Value: "packages", Requirement: leaction.Required},
			{Keyword: "timeout", Value: "duration", Requirement: leaction.Optional},
		},
		AnswerArgs: func(leaction.Arguments) (any, int) { return nil, 0 },
	})
	reached := 0
	Register(area.Name(), GroupSuite, func(args []string) (any, int) {
		reached++
		return area.Answer(args)
	}, registry.Meta{
		Description: "an area that declares its actions",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)

	code := 1
	page := captureStderr(t, func() {
		code = Dispatch("le", []string{area.Name(), "run", "scope", "./internal", "--help"})
	})
	if code != 0 {
		t.Errorf("a declared area answered %d for a help word, want 0", code)
	}
	if reached != 0 {
		t.Errorf("the area's handler ran %d time(s) for a help word", reached)
	}
	want := "usage: le trailing-help-grammar-probe run scope <packages>" +
		" [timeout <duration>] [| json | yaml | table]\n" +
		"  run the probe over one scope\n"
	if page != want {
		t.Errorf("the dispatcher rendered\n%q\nwant the area's own grammar\n%q", page, want)
	}
}

// VALIDATES: a trailing help word that a declared keyword introduced reaches
// the handler as that keyword's value, and the same word outside a value slot
// still stops before the handler.
// PREVENTS: the guard refusing an invocation whose last word is data. `le
// source-rewrite replace file <path> old beta new help` answered 0, wrote
// nothing and changed no file. No caller can tell that from the replacement it
// asked for (ai/rules/principles.md).
func TestATrailingHelpWordInAValueSlotReachesTheHandler(t *testing.T) {
	var got leaction.Arguments
	area := leaction.New("trailing-help-value-probe", leaction.Action{
		Verb: "replace", Why: "replace one word in one file", Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "file", Value: "path", Requirement: leaction.Required},
			{Keyword: "old", Value: "text", Requirement: leaction.Required},
			{Keyword: "new", Value: "text", Requirement: leaction.Required},
			{Keyword: "apply"},
		},
		AnswerArgs: func(args leaction.Arguments) (any, int) {
			got = args
			return map[string]string{"new": args.One("new")}, 0
		},
	})
	Register(area.Name(), GroupGenerate, area.Answer, registry.Meta{
		Description: "an area whose last keyword takes a value",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)
	RegisterShape(area.Name(), command.ShapeDoc)

	code := 1
	out := captureStdout(t, func() {
		code = Dispatch("le", []string{area.Name(), "replace",
			"file", "probe.txt", "old", "beta", "new", "help"})
	})
	if code != 0 {
		t.Errorf("an invocation ending in a value answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the action did not run: stdout was %q", out)
	}
	if got.One("new") != "help" {
		t.Errorf("new carries %q, want the word the operator typed", got.One("new"))
	}

	// The same word outside a value slot is the question it has always been.
	got = nil
	page := captureStderr(t, func() {
		code = Dispatch("le", []string{area.Name(), "replace", "file", "probe.txt", "--help"})
	})
	if code != 0 {
		t.Errorf("a help word at a keyword position answered %d, want 0", code)
	}
	if got != nil {
		t.Errorf("a help word at a keyword position ran the action with %#v", got)
	}
	if !strings.Contains(page, "usage: le trailing-help-value-probe replace") {
		t.Errorf("a help word at a keyword position printed %q", page)
	}
}

// VALIDATES: the dispatcher draws the value-slot exemption at the SPELLING. The
// bare word `help` in a declared keyword's value slot reaches the handler as
// that keyword's value. `-h` and `--help` in the same slot render usage and
// reach nothing.
// PREVENTS: `le verify status check path --help` running the check over a path
// named `--help`. `ai/rules/cli.md` bans a flag from being grammar or a value.
// A help option is the question in the trailing slot. Every other option is
// refused (TestAnOptionIsRefusedInAValueSlotAnywhereOnTheLine).
func TestAFlagSpellingInAValueSlotNeverReachesTheHandler(t *testing.T) {
	var got leaction.Arguments
	area := leaction.New("flag-spelling-value-probe", leaction.Action{
		Verb: "check", Why: "check one path",
		Parameters: []leaction.Parameter{
			{Keyword: "path", Value: "path", Requirement: leaction.Required},
		},
		AnswerArgs: func(args leaction.Arguments) (any, int) {
			got = args
			return map[string]string{"path": args.One("path")}, 0
		},
	})
	Register(area.Name(), GroupReport, area.Answer, registry.Meta{
		Description: "an area whose only keyword takes a value",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)
	RegisterShape(area.Name(), command.ShapeDoc)

	code := 1
	out := captureStdout(t, func() {
		code = Dispatch("le", []string{area.Name(), "check", "path", "help"})
	})
	if code != 0 {
		t.Errorf("the bare word in a value slot answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the bare word in a value slot did not run the action: stdout was %q", out)
	}
	if got.One("path") != "help" {
		t.Errorf("path carries %q, want the word the operator typed", got.One("path"))
	}

	for _, flag := range []string{"-h", "--help"} {
		got = nil
		page := captureStderr(t, func() {
			code = Dispatch("le", []string{area.Name(), "check", "path", flag})
		})
		if code != 0 {
			t.Errorf("%s in a value slot answered %d, want 0", flag, code)
		}
		if got != nil {
			t.Errorf("%s in a value slot ran the action with %#v", flag, got)
		}
		if !strings.Contains(page, "usage: le flag-spelling-value-probe check") {
			t.Errorf("%s in a value slot printed %q", flag, page)
		}
	}
}

// VALIDATES: the dispatcher refuses an OPTION in a value slot wherever it
// stands on the line, whatever it spells. It answers 2, and the handler never
// runs. A bare `help` in the same slot is still the value the keyword
// introduced, and the action runs.
// PREVENTS: `le source-rewrite replace file <path> old beta new --help apply`
// writing the text `--help` into a file, and `le verify status check path
// --help path internal` checking a path named `--help`. Both were measured on
// commit 8036f6c2f, where only the last word was read, and only for two
// spellings.
func TestAnOptionIsRefusedInAValueSlotAnywhereOnTheLine(t *testing.T) {
	var got leaction.Arguments
	area := leaction.New("flag-spelling-anywhere-probe", leaction.Action{
		Verb: "replace", Why: "replace one word in one file", Writes: true,
		Parameters: []leaction.Parameter{
			{Keyword: "file", Value: "path", Requirement: leaction.Required},
			{Keyword: "old", Value: "text", Requirement: leaction.Required},
			{Keyword: "new", Value: "text", Requirement: leaction.Required},
			{Keyword: "apply"},
		},
		AnswerArgs: func(args leaction.Arguments) (any, int) {
			got = args
			return map[string]string{"new": args.One("new")}, 0
		},
	})
	Register(area.Name(), GroupGenerate, area.Answer, registry.Meta{
		Description: "an area whose keywords take values before the last word",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterActions(area.Name(), area.Actions)
	RegisterShape(area.Name(), command.ShapeDoc)

	for _, option := range []string{"-h", "--help", "-help", "--list", "-6"} {
		got = nil
		code := 0
		page := captureStderr(t, func() {
			code = Dispatch("le", []string{area.Name(), "replace",
				"file", "probe.txt", "old", "beta", "new", option, "apply"})
		})
		if code != 2 {
			t.Errorf("%s in a non-trailing value slot answered %d, want 2", option, code)
		}
		if got != nil {
			t.Errorf("%s in a non-trailing value slot ran the action with %#v", option, got)
		}
		if !strings.Contains(page, `keyword "new"`) || !strings.Contains(page, option) {
			t.Errorf("%s was refused with %q, which names neither the keyword nor the option", option, page)
		}
	}

	got = nil
	code := 1
	out := captureStdout(t, func() {
		code = Dispatch("le", []string{area.Name(), "replace",
			"file", "probe.txt", "old", "beta", "new", "help", "apply"})
	})
	if code != 0 {
		t.Errorf("the bare word in a non-trailing value slot answered %d, want 0", code)
	}
	if got == nil {
		t.Fatalf("the bare word in a non-trailing value slot did not run the action: stdout was %q", out)
	}
	if got.One("new") != "help" {
		t.Errorf("new carries %q, want the word the operator typed", got.One("new"))
	}
	if !got.Has("apply") {
		t.Error("the keyword after the bare word was not parsed")
	}
}

// VALIDATES: a trailing option that is NOT one of the four help spellings
// reaches an area that published no action table. That is what `command
// <argv...>` means: the words after the keyword are the child's. The four
// spellings still stop before the area.
// PREVENTS: the help guard swallowing `le job run label encode-list command
// bin/ze-test bgp encode --list`, the recipe docs/contributing/testing.md
// prints. It also prevents `le job run label x command echo -html=cover.out`
// answering 0 with the page and no child, which review round 4 measured. An
// option that only LOOKS like a help request is a child's option. A table-less
// area publishes no grammar, so nothing tells le's own line from a forwarded
// one.
func TestATrailingOptionThatIsNotTheQuestionReachesATableLessArea(t *testing.T) {
	const forwarder = "argv-forwarding-probe"
	var carried []string
	Register(forwarder, GroupSuite, func(args []string) (any, int) {
		carried = args
		return map[string]int{"argv": len(args)}, 0
	}, registry.Meta{
		Description: "an area that hands the words after its keyword to a child",
		Mode:        "offline", Section: registry.SectionTest,
	})
	RegisterShape(forwarder, command.ShapeDoc)

	for _, option := range []string{
		"--list", "-count=1", "-6", "-1",
		// Round 4: each of these opens with `h` or carries one, and each is a
		// child's option rather than a help request.
		"-html=cover.out", "-host", "-headers", "-xh", "-xyz=1",
	} {
		carried = nil
		code := 1
		out := captureStdout(t, func() {
			code = Dispatch("le", []string{forwarder, "run", "command", "child", option})
		})
		if code != 0 {
			t.Errorf("a trailing %s answered %d, want the area's own 0", option, code)
		}
		if carried == nil {
			t.Fatalf("a trailing %s never reached the area: stdout was %q", option, out)
		}
		if carried[len(carried)-1] != option {
			t.Errorf("the area received %v, want the line ending in %s", carried, option)
		}
	}

	// The help question is still the question there, however it is spelled.
	for _, question := range []string{"--help", "-h", "-help", "help"} {
		carried = nil
		page := captureStderr(t, func() {
			if code := Dispatch("le", []string{forwarder, "run", "command", "child", question}); code != 0 {
				t.Errorf("a trailing %s answered %d, want 0", question, code)
			}
		})
		if carried != nil {
			t.Errorf("a trailing %s reached the area with %v", question, carried)
		}
		if !strings.Contains(page, "le "+forwarder) {
			t.Errorf("a trailing %s printed no page: %q", question, page)
		}
	}
}
