// Related: helpai.go -- `le help ai`, the reference these tests pin

package leroot

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
)

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
	if !strings.Contains(text, "le help ai | json") {
		t.Errorf("the summary does not name `le help ai | json`:\n%s", text)
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
	if !strings.Contains(helpAIOf(t, "ze le").Text(), "ze le help ai | json") {
		t.Error("the ze le summary does not name `ze le help ai | json`")
	}
}
