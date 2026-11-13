package job

import (
	"slices"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestJobAreaDeclaresRunAndReport proves the job area declares exactly two
// actions, and that `le job run --help` reads the declared grammar: label,
// quiet and command as typed keywords, and no usage line standing in for an
// action. The report action takes no parameter and has a page of its own.
//
// VALIDATES: AC-7, `job run` shows `label`, `quiet`, `command` as keywords;
// spec-main-thread-schedules-heavy-work AC-15, the area also declares the
// read-only `report` action, and nothing else.
// PREVENTS: the hand-written `usage: le job run ...` line rendered as an
// action named after the whole line, and a third action that arrives in the
// area with no test naming it.
func TestJobAreaDeclaresRunAndReport(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 2 {
		t.Fatalf("job declares %+v, want the run and report actions and no other", list.Actions)
	}
	run := slices.IndexFunc(list.Actions, func(row leaction.Row) bool { return row.Verb == runVerb })
	if run == -1 {
		t.Fatalf("job declares %+v, want a run action", list.Actions)
	}
	report := slices.IndexFunc(list.Actions, func(row leaction.Row) bool { return row.Verb == reportVerb })
	if report == -1 {
		t.Fatalf("job declares %+v, want a report action", list.Actions)
	}

	want := map[string]leaction.Kind{
		labelKeyword:   leaction.KindLabel,
		quietKeyword:   leaction.KindSwitch,
		commandKeyword: leaction.KindArgv,
	}
	var keywords []string
	for _, parameter := range list.Actions[run].Parameters {
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

	if parameters := list.Actions[report].Parameters; len(parameters) != 0 {
		t.Errorf("report declares %+v, want no parameter", parameters)
	}
	if summary := list.Actions[report].Summary; summary == "" || len(summary) > 60 {
		t.Errorf("the report summary is %q (%d characters), want 1 to 60", summary, len(summary))
	}
	if _, declared := list.ActionPage("le", reportVerb); !declared {
		t.Error("the report action has no page")
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
