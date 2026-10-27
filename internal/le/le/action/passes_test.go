package leaction

import (
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: AC-15, AC-19 -- a pass-through table (PassesWords) publishes
// `passes-words` and the program it hands the words to, holds no action, gives
// every help word to the program, and renders a page that says where the words
// go, under a usage that takes the program's words rather than an action.
// PREVENTS: a forwarding area le describes as an empty table, so its help says
// nothing, or one whose help word le answers in place of the program.
func TestPassesWordsDeclaresWhereTheWordsGo(t *testing.T) {
	list := PassesWords("mrt", "the MRT analyzer")

	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal the pass-through table: %v", err)
	}
	if got := string(raw); got != `{"area":"mrt","actions":[],"passes-words":true,"passes-to":"the MRT analyzer"}` {
		t.Errorf("the pass-through table marshals as %s", got)
	}

	for _, words := range [][]string{{"--help"}, {"help"}, {"statistics", "--help"}, {"statistics", "-h"}} {
		if list.TrailingWordAsksForHelp(words) {
			t.Errorf("%q asks le for help, want it handed to the program", words)
		}
	}

	page := list.Page("le")
	if len(page.Usage) != 1 || page.Usage[0] != "le mrt [<words>...] ['|' json|yaml|table]" {
		t.Errorf("the usage is %q, want the program's words after the area", page.Usage)
	}
	section := list.Section()
	if len(section.Entries) != 1 || section.Entries[0].Name != "[<words>...]" {
		t.Fatalf("the section is %+v, want one entry for the words", section)
	}
	if desc := section.Entries[0].Desc; !strings.Contains(desc, "every word passes to the MRT analyzer") || !strings.Contains(desc, "help word asks it") {
		t.Errorf("the section says %q, want where the words go and who answers help", desc)
	}

	// A table with actions publishes neither key.
	raw, err = json.Marshal(New("spec", Action{Verb: "show", Why: "show it.", Answer: func() (any, int) { return nil, 0 }}).Actions())
	if err != nil {
		t.Fatalf("marshal an action table: %v", err)
	}
	if strings.Contains(string(raw), "passes-") {
		t.Errorf("an action table publishes %s, want no passes- key", raw)
	}
}

// VALIDATES: a pass-through table that names no area or no program is refused
// at init, so help can never print a pass-through line naming nobody.
// PREVENTS: an empty program in help, which reads as a working declaration.
func TestPassesWordsRefusesAnEmptyName(t *testing.T) {
	for _, tc := range []struct{ area, program string }{{"", "the MRT analyzer"}, {"mrt", ""}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("PassesWords(%q, %q) did not panic", tc.area, tc.program)
				}
			}()
			PassesWords(tc.area, tc.program)
		}()
	}
}
