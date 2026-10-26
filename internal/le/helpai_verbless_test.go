package le

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
)

// TestHelpAiCommandsNamesTheVerbOfAVerblessAction proves `le help ai
// commands` tells an agent what a verbless action does (D-P4-areaword of
// spec-le-cli-usability): its JSON row carries an empty `verb` and the
// vocabulary verb it performs under `does`, and its text line puts a marker,
// never the summary, where a verb goes, then the verb it does.
// METHOD: dispatch `help ai commands | json` and `help ai commands` through
// le's own dispatcher and read the rows of `rollback` and `spec claim`, two
// areas whose sole action is verbless, and one verb action that declares Does.
// PREVENTS: a verbless row an agent cannot tell from a verb (an empty `verb`
// with no `does`), and a text line whose summary's first word reads as a verb.
func TestHelpAiCommandsNamesTheVerbOfAVerblessAction(t *testing.T) {
	out, code := dispatchToStdout(t, "help", "ai", "commands", "|", "json")
	if code != 0 {
		t.Fatalf("help ai commands | json: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, `"verb": ""`) {
		t.Errorf("help ai commands | json drops the empty verb key of a verbless action; consumers read it")
	}
	var ref reference.Reference
	if err := json.Unmarshal([]byte(out), &ref); err != nil {
		t.Fatalf("help ai commands | json is not the reference: %v\n%s", err, out)
	}
	text, code := dispatchToStdout(t, "help", "ai", "commands")
	if code != 0 {
		t.Fatalf("help ai commands: exit %d\n%s", code, text)
	}
	cases := []struct {
		area, verb, does string
	}{
		{area: "rollback", verb: "", does: "rollback"},
		{area: "spec claim", verb: "", does: "claim"},
		{area: "test weakened", verb: "proposed", does: "judge"},
	}
	for _, tc := range cases {
		var action reference.Action
		found := false
		for _, command := range ref.Commands {
			if command.Name != tc.area {
				continue
			}
			for _, candidate := range command.Actions {
				if candidate.Verb == tc.verb {
					action, found = candidate, true
				}
			}
		}
		if !found {
			t.Errorf("%s %q: help ai commands | json lists no such action", tc.area, tc.verb)
			continue
		}
		if action.Does != tc.does {
			t.Errorf("%s %q: help ai commands | json says does %q, want %q", tc.area, tc.verb, action.Does, tc.does)
		}
		column := tc.verb
		if column == "" {
			column = reference.VerblessColumn
		}
		line := "  " + column + "  " + action.ShortHelp + "  (does " + tc.does + ")\n"
		if !strings.Contains(text, line) {
			t.Errorf("%s %q: help ai commands (text) holds no line %q", tc.area, tc.verb, line)
		}
		if tc.verb == "" && strings.Contains(text, "\n  "+action.ShortHelp+"\n") {
			t.Errorf("%s: help ai commands (text) prints the summary where the verb goes", tc.area)
		}
	}
}
