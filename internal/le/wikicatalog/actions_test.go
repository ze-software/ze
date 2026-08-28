package wikicatalog

import (
	"testing"

	"github.com/ze-software/ze/internal/le/leaction"
)

func TestActionCatalogPinsCheckAndUpdateGrammar(t *testing.T) {
	list := Actions()
	if list.Area != area || len(list.Actions) != 2 {
		t.Fatalf("action list = %#v", list)
	}
	check := list.Actions[0]
	if check.Verb != "check" || check.Writes {
		t.Fatalf("check action = %#v", check)
	}
	update := list.Actions[1]
	if update.Verb != "update" || !update.Writes {
		t.Fatalf("update action = %#v", update)
	}
	if got := Subs(); got != "check | update (writes)" {
		t.Fatalf("Subs() = %q", got)
	}
}

func TestAnswerRejectsOpenOrIncompleteGrammar(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "unknown action", args: []string{"render"}, code: 2},
		{name: "positional destination", args: []string{"check", "catalog.md"}, code: 2},
		{name: "unknown keyword", args: []string{"update", "destination", "catalog.md"}, code: 2},
		{name: "missing value", args: []string{"update", "file"}, code: 2},
		{name: "duplicate file", args: []string{"check", "file", "one", "file", "two"}, code: 2},
		{name: "required file omitted", args: []string{"check"}, code: 1},
		{name: "blank file", args: []string{"update", "file", " "}, code: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			answer, code := Answer(test.args)
			if code != test.code || answer != nil {
				t.Fatalf("Answer(%q) = (%#v, %d), want (nil, %d)", test.args, answer, code, test.code)
			}
		})
	}
}

func TestRunReturnsStructuredReportAndDistinctStaleCode(t *testing.T) {
	arguments := leaction.Arguments{"file": "chosen.md"}
	answer, code := run(arguments, func(destination string) (Report, error) {
		return Report{File: destination, Stale: true, Bytes: 7}, nil
	})
	if code != StaleExit {
		t.Fatalf("stale code = %d, want %d", code, StaleExit)
	}
	report, ok := answer.(Report)
	if !ok || report.File != "chosen.md" || !report.Stale || report.Bytes != 7 {
		t.Fatalf("stale answer = %#v", answer)
	}

	answer, code = run(arguments, func(destination string) (Report, error) {
		return Report{File: destination, Written: true, Bytes: 9}, nil
	})
	if code != 0 {
		t.Fatalf("update code = %d, want 0", code)
	}
	report, ok = answer.(Report)
	if !ok || report.File != "chosen.md" || !report.Written || report.Bytes != 9 {
		t.Fatalf("update answer = %#v", answer)
	}
}
