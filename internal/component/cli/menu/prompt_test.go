// VALIDATES: AC-44 (a prompt completes from the value source), AC-46 (Ctrl-C and Ctrl-D answer nothing)
// PREVENTS: a prompt that offers what the source does not hold, or an interrupted line that runs

package menu

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// press sends keys to the prompt and answers the model and the last command.
func press(t *testing.T, p promptModel, keys ...tea.KeyPressMsg) (promptModel, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, key := range keys {
		var updated tea.Model
		updated, cmd = p.Update(key)
		next, ok := updated.(promptModel)
		if !ok {
			t.Fatalf("Update answered a %T, not the prompt", updated)
		}
		p = next
	}
	return p, cmd
}

func typed(text string) tea.KeyPressMsg { return tea.KeyPressMsg{Code: -1, Text: text} }

var tabKey = tea.KeyPressMsg{Code: tea.KeyTab}

// TestPromptCompletesFromTheValueSource proves Tab reads the prompt's source
// with the text typed so far: one match is taken whole, several are narrowed
// to their shared prefix and offered, and none leaves the text alone. Method:
// a source over three spec paths, asked after different prefixes.
func TestPromptCompletesFromTheValueSource(t *testing.T) {
	paths := []string{"plan/a/spec-one.md", "plan/a/spec-other.md", "plan/b/spec-two.md"}
	var asked []string
	source := func(partial string) []string {
		asked = append(asked, partial)
		var out []string
		for _, path := range paths {
			if strings.HasPrefix(path, partial) {
				out = append(out, path)
			}
		}
		return out
	}
	start := promptModel{prompt: Prompt{Label: "spec", Help: "the spec file", Complete: source}}

	one, _ := press(t, start, typed("plan/b"), tabKey)
	if one.text != "plan/b/spec-two.md" || one.offered != nil {
		t.Errorf("one match: text %q offered %v, want the whole path and nothing offered", one.text, one.offered)
	}
	if !slices.Equal(asked, []string{"plan/b"}) {
		t.Errorf("the source was asked %v, want [plan/b]", asked)
	}

	several, _ := press(t, start, typed("plan/a"), tabKey)
	if several.text != "plan/a/spec-o" {
		t.Errorf("several matches: text %q, want their shared prefix plan/a/spec-o", several.text)
	}
	if len(several.offered) != 2 {
		t.Errorf("several matches: offered %v, want both", several.offered)
	}
	if view := several.View().Content; !strings.Contains(view, "spec-other.md") {
		t.Errorf("the offered candidates are not drawn:\n%s", view)
	}

	none, _ := press(t, start, typed("nope"), tabKey)
	if none.text != "nope" || none.offered != nil {
		t.Errorf("no match: text %q offered %v, want the text unchanged", none.text, none.offered)
	}

	done, cmd := press(t, one, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done.done || cmd == nil {
		t.Error("Enter on a value should answer it")
	}
}

// TestPromptCancelRunsNothing proves Ctrl-C and Ctrl-D leave the prompt
// canceled with no value, whatever was typed, and that Enter on an empty line
// is not an answer, because the value is required. Method: drive each key and
// read the state Ask turns into ErrCanceled.
func TestPromptCancelRunsNothing(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'c', Mod: tea.ModCtrl},
		{Code: 'd', Mod: tea.ModCtrl},
	} {
		p, cmd := press(t, promptModel{prompt: Prompt{Label: "spec"}}, typed("plan/x"), key)
		if !p.canceled || p.done || cmd == nil {
			t.Errorf("%s: canceled %v done %v, want canceled and quitting", key.String(), p.canceled, p.done)
		}
		if view := p.View().Content; view != "" {
			t.Errorf("%s: a canceled prompt still draws %q", key.String(), view)
		}
	}

	empty, cmd := press(t, promptModel{prompt: Prompt{Label: "spec"}}, typed(" "), tea.KeyPressMsg{Code: tea.KeyEnter})
	if empty.done || cmd != nil {
		t.Error("Enter on a blank line should keep asking")
	}
}
