package job

import (
	"slices"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestJobRunHelpShowsItsKeywordsFromTheTable proves `le job run --help` reads
// the declared grammar: label, quiet and command as typed keywords, and no
// usage line standing in for an action.
//
// VALIDATES: AC-7, `job run` shows `label`, `quiet`, `command` as keywords.
// PREVENTS: the hand-written `usage: le job run ...` line rendered as an
// action named after the whole line.
func TestJobRunHelpShowsItsKeywordsFromTheTable(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 1 || list.Actions[0].Verb != runVerb {
		t.Fatalf("job declares %+v, want the one run action", list.Actions)
	}
	want := map[string]leaction.Kind{
		labelKeyword:   leaction.KindLabel,
		quietKeyword:   leaction.KindSwitch,
		commandKeyword: leaction.KindArgv,
	}
	var keywords []string
	for _, parameter := range list.Actions[0].Parameters {
		keywords = append(keywords, parameter.Keyword)
		if want[parameter.Keyword] != parameter.Kind {
			t.Errorf("%s is %v, want %v", parameter.Keyword, parameter.Kind, want[parameter.Keyword])
		}
	}
	if !slices.Equal(keywords, []string{labelKeyword, quietKeyword, commandKeyword}) {
		t.Errorf("run declares %q, want label, quiet, command", keywords)
	}
	page, declared := list.ActionPage("le", runVerb)
	if !declared {
		t.Fatal("the run action has no page")
	}
	if len(page.Sections) != 1 || len(page.Sections[0].Entries) != 3 {
		t.Errorf("the run page is %+v, want one keyword section of 3", page.Sections)
	}
}

// TestAJobArgvKeepsOptionsAndKeywordSpellings proves the child's words reach it
// unchanged through the table, a dash-leading word and a `label` included.
func TestAJobArgvKeepsOptionsAndKeywordSpellings(t *testing.T) {
	asked, ok := parseRun([]string{"label", "unit-pkg", "command", "go", "test", "-overlay", "o.json", "label", "x"})
	if !ok {
		t.Fatal("the line was refused")
	}
	want := []string{"go", "test", "-overlay", "o.json", "label", "x"}
	if !slices.Equal(asked.Argv, want) {
		t.Errorf("argv = %q, want %q", asked.Argv, want)
	}
	if asked.Label != "unit-pkg" {
		t.Errorf("label = %q, want unit-pkg", asked.Label)
	}
}
