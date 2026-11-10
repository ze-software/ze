// Related: helpai.go -- `le help ai`, the reference these tests pin

package leroot

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// aiPageProbe is a command registered under the ai namespace with one
// action, standing in for `ai sync` and the rest, which this package does
// not link.
const aiPageProbe = aiWord + " help-page-probe"

var aiPageProbeOnce sync.Once

// registerAIPageProbe registers aiPageProbe once per test binary, so every
// test that needs it can ask for it in any order.
func registerAIPageProbe() {
	aiPageProbeOnce.Do(func() {
		Register(aiPageProbe, GroupReport, func([]string) (any, int) { return nil, 0 },
			registry.Meta{Description: "A probe area for the help ai tests.", ShortHelp: "the help ai probe", Mode: "offline", Section: registry.SectionTest})
		area := leaction.New(aiPageProbe, leaction.Action{Summary: "show the probe", Verb: "show", Why: "show the probe", Answer: func() (any, int) { return nil, 0 }})
		RegisterActions(aiPageProbe, area.Actions)
		RegisterShape(aiPageProbe, command.ShapeDoc)
	})
}

// VALIDATES: `le help ai <command> <action>` pages that ai command's action,
// the page `le help` gives every other command, and never the reference.
// PREVENTS: `help ai` shadowing the ai namespace, so `le help ai sync update`
// was refused as a missing section while `le help spec claim` paged.
// Method: register one ai probe action, dispatch `le help ai <probe> show`
// and compare stdout byte for byte with the action's own page.
func TestHelpAiPagesAnAiCommandsAction(t *testing.T) {
	registerAIPageProbe()
	list, declared := ActionsOf(aiPageProbe)
	if !declared {
		t.Fatalf("%s declared no action table", aiPageProbe)
	}
	want := captureStdout(t, func() { writeActionPage("le", aiPageProbe, list, "show") })

	code := -1
	line := append([]string{"help"}, strings.Fields(aiPageProbe+" show")...)
	got := captureStdout(t, func() { code = Dispatch("le", line) })
	if code != 0 || got != want {
		t.Errorf("le %s = exit %d, page:\n%s\nwant exit 0 and the action's page:\n%s", strings.Join(line, " "), code, got, want)
	}
	if strings.Contains(got, "le help ai '|' json") {
		t.Errorf("le %s answered the agent reference:\n%s", strings.Join(line, " "), got)
	}
	// A section word, an option and a pipe after `help ai` stay the reference's.
	for _, words := range [][]string{{aiWord}, {aiWord, sectionVerbs}, {aiWord, "--json"}, {aiWord, pipeWord, "json"}} {
		if !asksAIReference(words) {
			t.Errorf("help %s is not read as the agent reference", strings.Join(words, " "))
		}
	}
	if asksAIReference(strings.Fields(aiPageProbe)) {
		t.Errorf("help %s is read as the agent reference, not the ai command's page", aiPageProbe)
	}
}

// VALIDATES: a word after `help ai` that is neither a section nor an ai
// command is refused with the usage code, and the refusal names every
// section and every ai command, so the reader learns both spellings.
// PREVENTS: a refusal naming only the sections, which hid that `help ai`
// also pages the ai namespace's commands.
func TestHelpAiUnknownWordNamesSectionsAndAiCommands(t *testing.T) {
	registerAIPageProbe()
	code := -1
	stderr := captureStderr(t, func() { code = Dispatch("le", []string{"help", aiWord, "no-such-word"}) })
	if code != 2 {
		t.Errorf("le help ai no-such-word: exit %d, want 2", code)
	}
	wants := append(sectionNames(), "no-such-word", strings.TrimPrefix(aiPageProbe, aiWord+" "), "le help ai <command>")
	for _, want := range wants {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, stderr)
		}
	}
}

// VALIDATES: an ai command whose first word after `ai` is a section word is
// refused at registration, and every other name is accepted.
// PREVENTS: a command `le help ai <section>` answers as the section, leaving
// its own page with no spelling after `help ai`.
func TestRegistrationRefusesAnAiCommandSpelledLikeASection(t *testing.T) {
	for _, section := range sectionNames() {
		name := aiWord + " " + section + " probe"
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%q was accepted, though `le help ai %s` is a section", name, section)
				}
			}()
			requireNoAICommandShadowsASection([]string{name})
		}()
	}
	requireNoAICommandShadowsASection([]string{"ai sync", "ai rules report", "spec commands", "ai"})
}

// helpAIOf answers the reference for words, failing the test on a refusal.
func helpAIOf(t *testing.T, program string, words ...string) reference.Reference {
	t.Helper()
	payload, code := answerHelpAI(program, words)
	if code != 0 {
		t.Fatalf("help ai %q: exit %d", words, code)
	}
	ref, ok := payload.(reference.Reference)
	if !ok {
		t.Fatalf("help ai %q answered %T", words, payload)
	}
	return ref
}

// VALIDATES: AC-38 -- bare `le help ai` is a text summary naming every
// section and the pipe that renders it as JSON.
// PREVENTS: an index that drifts from the sections the command accepts.
func TestHelpAiTextSummary(t *testing.T) {
	text := helpAIOf(t, "le").Text()
	for _, section := range sectionNames() {
		if !strings.Contains(text, "  "+section+" ") {
			t.Errorf("the summary does not name section %q:\n%s", section, text)
		}
	}
	if !strings.Contains(text, "le help ai '|' json") {
		t.Errorf("the summary does not name `le help ai '|' json`:\n%s", text)
	}
}

// VALIDATES: AC-39 -- `le help ai commands | json` holds every registered area
// (the manifest's count), each action, and each live parameter with its kind,
// values, required, repeat and help; `all` holds every section.
// PREVENTS: an agent reading a grammar narrower than the one le parses.
func TestHelpAiJSONListsEveryAreaWithTypedParameters(t *testing.T) {
	treeProbe(t)
	ref := helpAIOf(t, "le", sectionCommands)
	areas := manifestOf("le").Areas
	if len(ref.Commands) != len(areas) {
		t.Fatalf("help ai lists %d commands, the manifest %d", len(ref.Commands), len(areas))
	}
	sawProbe := false
	for index, area := range areas {
		command := ref.Commands[index]
		if command.Name != area.Name || len(command.Actions) != len(area.Actions) {
			t.Fatalf("command %d: %q with %d actions, manifest %q with %d", index, command.Name, len(command.Actions), area.Name, len(area.Actions))
		}
		if area.PassesWords && command.PassesTo == "" {
			t.Errorf("%s passes its words on, and help ai does not say where", area.Name)
		}
		for actionIndex, row := range area.Actions {
			live := 0
			for _, parameter := range row.Parameters {
				if parameter.Retired != "" {
					continue
				}
				got := command.Actions[actionIndex].Parameters[live]
				live++
				if got.Keyword != parameter.Keyword || got.Kind != parameter.Kind.String() || got.Repeat != parameter.Repeat ||
					got.Help != parameter.Help || strings.Join(got.Values, ",") != strings.Join(parameter.Values, ",") {
					t.Errorf("%s %s: parameter %+v does not carry %+v", area.Name, row.Verb, got, parameter)
				}
			}
			if live != len(command.Actions[actionIndex].Parameters) {
				t.Errorf("%s %s: help ai lists a retired keyword", area.Name, row.Verb)
			}
		}
		sawProbe = sawProbe || area.Name == "treeprobe area"
	}
	if !sawProbe {
		t.Fatal("the probe area is missing, so no typed parameter was checked")
	}
	raw, err := json.Marshal(helpAIOf(t, "le", sectionAll))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"commands"`, `"groups"`, `"verbs"`, `"exit-codes"`, `"kind":"enum"`, `"required":true`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("help ai all | json holds no %s", key)
		}
	}
}

// VALIDATES: AC-39, AC-40 -- `--json` and `--ai` are refused as dash-leading
// words with the usage code, and so is a section that does not exist.
// PREVENTS: a rendering flag beside the pipe layer (ai/rules/cli.md).
func TestHelpAiRefusesRenderingFlags(t *testing.T) {
	for _, word := range []string{"--json", "--ai", "-j"} {
		if code := helpAI("le", []string{word}); code != 2 {
			t.Errorf("help ai %s: exit %d, want 2", word, code)
		}
	}
	if _, code := answerHelpAI("le", []string{"nosuch"}); code != 2 {
		t.Errorf("help ai nosuch: exit %d, want 2", code)
	}
}

// VALIDATES: AC-48 (help ai half) -- `ze le help ai` answers the same payload
// as `le help ai`, with program reading `ze le`.
// PREVENTS: a renderer that hard-codes the program name.
func TestZeLeParity(t *testing.T) {
	le, zele := helpAIOf(t, "le", sectionAll), helpAIOf(t, "ze le", sectionAll)
	if zele.Program != "ze le" {
		t.Errorf("program reads %q, want ze le", zele.Program)
	}
	zele.Program = le.Program
	a, errA := json.Marshal(le)
	b, errB := json.Marshal(zele)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	if !bytes.Equal(a, b) {
		t.Error("ze le help ai differs from le help ai beyond the program name")
	}
	if !strings.Contains(helpAIOf(t, "ze le").Text(), "ze le help ai '|' json") {
		t.Error("the ze le summary does not name `ze le help ai '|' json`")
	}
}
