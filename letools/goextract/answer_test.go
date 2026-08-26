package goextract

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/letools/leroot"
)

// VALIDATES: the grammar of `le go-extract` keeps a keyword in front of every
// value, refuses anything it does not understand by name, and answers one
// structured payload every pipe operator renders.
// PREVENTS: a file-rewriting tool reached by three bare positionals, where the
// difference between the file that is read and the file that is written is the
// order the developer remembered.

func TestParseRequestReadsTheKeywords(t *testing.T) {
	req, err := parseRequest([]string{"source", "a.go", "dest", "b.go", "symbol", "Beta", "symbol", "Gamma"})
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Source != "a.go" || req.Dest != "b.go" {
		t.Errorf("parseRequest read source=%q dest=%q", req.Source, req.Dest)
	}
	if len(req.Symbols) != 2 || req.Symbols[0] != "Beta" || req.Symbols[1] != "Gamma" {
		t.Errorf("parseRequest read symbols %v, want [Beta Gamma]", req.Symbols)
	}
}

func TestParseRequestAcceptsTheKeywordsInAnyOrder(t *testing.T) {
	req, err := parseRequest([]string{"symbol", "Beta", "dest", "b.go", "source", "a.go"})
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Source != "a.go" || req.Dest != "b.go" || len(req.Symbols) != 1 {
		t.Errorf("parseRequest read %+v", req)
	}
}

func TestParseRequestReadsAValueThatSpellsAKeyword(t *testing.T) {
	// A Go identifier named `dest` is a legal symbol name, and a file named
	// `source` is a legal file name. Each is a VALUE because a keyword came
	// first, which is the whole reason the grammar has keywords.
	req, err := parseRequest([]string{"source", "dest", "dest", "source", "symbol", "symbol"})
	if err != nil {
		t.Fatalf("parseRequest: %v", err)
	}
	if req.Source != "dest" || req.Dest != "source" {
		t.Errorf("parseRequest read source=%q dest=%q", req.Source, req.Dest)
	}
	if len(req.Symbols) != 1 || req.Symbols[0] != "symbol" {
		t.Errorf("parseRequest read symbols %v, want [symbol]", req.Symbols)
	}
}

func TestParseRequestRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name string
		args []string
		says string
	}{
		{"no words at all", nil, "source"},
		{"a bare path", []string{"a.go"}, "needs a value"},
		{"an unknown keyword", []string{"file", "a.go"}, "unknown keyword"},
		{"a keyword with no value", []string{"source", "a.go", "dest"}, "needs a value"},
		{"no source", []string{"dest", "b.go", "symbol", "Beta"}, "source"},
		{"no dest", []string{"source", "a.go", "symbol", "Beta"}, "dest"},
		{"no symbol", []string{"source", "a.go", "dest", "b.go"}, "symbol"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRequest(tc.args)
			if err == nil {
				t.Fatalf("parseRequest(%v) was accepted", tc.args)
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal does not name %q: %v", tc.says, err)
			}
		})
	}
}

func TestAnswerMovesTheDeclarationAndAnswersTheRecord(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	payload, code := Answer([]string{"source", source, "dest", dest, "symbol", "Beta"})
	if code != 0 {
		t.Fatalf("Answer exited %d", code)
	}

	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("Answer did not answer a Report: %T", payload)
	}
	if len(report.Symbols) != 1 || report.Symbols[0].Symbol != "Beta" {
		t.Errorf("the report names %v", report.Symbols)
	}
	if !strings.Contains(read(t, dest), "func Beta") {
		t.Error("the declaration did not arrive in the destination")
	}
}

func TestAnswerReportsAMoveItCannotMake(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)

	payload, code := Answer([]string{"source", source, "dest", filepath.Join(dir, "beta.go"), "symbol", "Zeta"})
	if code != 1 {
		t.Fatalf("Answer exited %d for a symbol that is not there, want 1", code)
	}
	if payload != nil {
		t.Errorf("a refused move answered a payload: %v", payload)
	}
	if left := read(t, source); left != twoFuncs {
		t.Errorf("a refused move edited the source:\n%s", left)
	}
}

func TestTheCommandDeclaresItsAnswerShape(t *testing.T) {
	// The registration runs at init, so the shape is declared by the time any
	// test reads it. A tool whose shape is undeclared has its row operators
	// judged from the answer in hand, which for this tool means AFTER a file
	// has been rewritten.
	shape, declared := command.ShapeForCommand("go-extract")
	if !declared {
		t.Fatal("go-extract declares no answer shape")
	}
	if shape != command.ShapeMap {
		t.Errorf("go-extract declares the shape %v, want ShapeMap", shape)
	}
}

func TestTheCommandIsOneOfLesOwn(t *testing.T) {
	if !leroot.Owns("go-extract") {
		t.Errorf("le does not own go-extract; it owns %v", leroot.Owned())
	}
}

func TestTheAnswerRendersThroughTheEngine(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	var out, errOut strings.Builder
	code := leroot.Run("go-extract", Answer,
		[]string{"source", source, "dest", dest, "symbol", "Beta", "|", "json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("le go-extract | json exited %d: %s", code, errOut.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("the answer is not JSON: %v\n%s", err, out.String())
	}
	if decoded["dest"] != dest {
		t.Errorf("the JSON names dest %v, want %s", decoded["dest"], dest)
	}
	// No rendering code exists in this tool: the same payload is what Text
	// renders for a bare command.
	if strings.Contains(out.String(), "extracted 1 symbols") {
		t.Errorf("the JSON rendering carries the prose line:\n%s", out.String())
	}
}

func TestABareCommandRendersTheSummaryLine(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	var out, errOut strings.Builder
	code := leroot.Run("go-extract", Answer,
		[]string{"source", source, "dest", dest, "symbol", "Beta"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("le go-extract exited %d: %s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "extracted 1 symbols (") {
		t.Errorf("the bare command did not render the summary line:\n%s", out.String())
	}
}

func TestTheSummaryLineCountsTheLinesThatMoved(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	plan, err := PlanMove(Request{Source: source, Dest: dest, Symbols: []string{"Beta"}})
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}

	// The count is the extracted lines, which is what the destination body
	// holds under its package clause and the blank line beneath it. The
	// comparison is against the PLAN rather than the written file, because
	// goimports edits the file afterwards and would then be what is counted.
	body, ok := strings.CutPrefix(plan.Dest, "package sample\n\n")
	if !ok {
		t.Fatalf("the destination body carries no package clause:\n%s", plan.Dest)
	}
	if written := strings.Count(body, "\n") + 1; plan.Report.Lines != written {
		t.Errorf("the report counts %d lines; the destination body holds %d", plan.Report.Lines, written)
	}
	if plan.Report.Lines != 4 {
		t.Errorf("Beta and its doc comment are %d lines, want 4", plan.Report.Lines)
	}
}
