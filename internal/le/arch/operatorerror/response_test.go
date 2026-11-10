// VALIDATES: the operator-error scan reads the error answers a registered
// handler builds (plugin.Response with Status plugin.StatusError and a nil Go
// error), classifies each as typed (plugin.OperatorErrorResponse), plain (text
// the site wrote) or pass-through (another producer's error text), refuses a
// plain answer a change adds, and only reports the plain answers its parent
// already held.
// PREVENTS: a handler refusal that names no fix reaching the operator through
// Response.Error, where a returned-error scan never looks.
package archoperatorerror

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// answersTree is a tree of one plugin whose handlers refuse by answering.
const answersTree = "testdata/answers"

const answersFile = "internal/plugins/answers/answers.go"

// scanAnswers scans the answering fixture tree.
func scanAnswers(t *testing.T) Inventory {
	t.Helper()
	inventory, err := Scan(answersTree)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return inventory
}

// responses answers the sites of the error-response form.
func responses(sites []Site) []Site {
	var answers []Site
	for _, site := range sites {
		if site.Form == FormResponse {
			answers = append(answers, site)
		}
	}
	return answers
}

// answersSide reads the answering fixture from disk, with old replaced by
// replacement in path.
func answersSide(t *testing.T, path, old, replacement string) changecheck.Side {
	t.Helper()
	return func(read string) (string, bool, error) {
		content, err := os.ReadFile(filepath.Join(answersTree, filepath.FromSlash(read)))
		if err != nil {
			return "", false, err
		}
		text := string(content)
		if read == path {
			if !strings.Contains(text, old) {
				t.Fatalf("fixture %s holds no %q", read, old)
			}
			text = strings.Replace(text, old, replacement, 1)
		}
		return text, true, nil
	}
}

// TestOperatorErrorClassifiesErrorResponses holds the classification: the
// declared route is typed; a literal, a built string, a missing Error, and the
// Error() of an error the site built (opererr's included, which bypasses the
// route) are plain; another producer's error text is pass-through; and a done
// answer, an answer in a nested literal or an unregistered function, is no
// site.
func TestOperatorErrorClassifiesErrorResponses(t *testing.T) {
	answers := responses(scanAnswers(t).Sites)
	want := []struct {
		handler, expr string
		status        Status
	}{
		{"answerTyped", "plugin.OperatorErrorResponse(opererr.New(", StatusTyped},
		{"answerPlain", `Error: "dispatcher unavailable"`, StatusPlain},
		{"answerPlain", `Error: "peer " + args.Get(0)`, StatusPlain},
		{"answerPlain", "(no Error field)", StatusPlain},
		{"answerPlain", `Error: errors.New("bad argument").Error()`, StatusPlain},
		{"answerPlain", `Error: opererr.New("peer refused"`, StatusPlain},
		{"answerPassThrough", "Error: err.Error()", StatusPassThrough},
		{"answerPassThrough", "Error: forwarded.Error", StatusPassThrough},
		{"answerPassThrough", "Error: errUnavailable.Error()", StatusPassThrough},
	}
	if len(answers) != len(want) {
		t.Fatalf("error responses = %d, want %d: %+v", len(answers), len(want), answers)
	}
	for _, expected := range want {
		site := siteOf(t, answers, expected.handler, expected.expr)
		if site.Status != expected.status {
			t.Errorf("%s %s: status %s, want %s", expected.handler, expected.expr, site.Status, expected.status)
		}
		if site.Kind != KindRPC {
			t.Errorf("%s %s: kind %s, want rpc-handler", expected.handler, expected.expr, site.Kind)
		}
	}
	built := siteOf(t, answers, "answerPlain", "args.Get(0)")
	content, err := os.ReadFile(filepath.Join(answersTree, answersFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	if !strings.Contains(lines[built.Line-1], "Error:") {
		t.Errorf("a multi-line literal's site is line %d (%q), want the Error field's line", built.Line, lines[built.Line-1])
	}
}

// TestOperatorErrorReportsExistingErrorResponses holds that a plain answer
// already in the tree is reported and never refused: report lists it with its
// form, an empty change set refuses nothing, and a commit that adds no plain
// answer is admitted though its parent holds four.
func TestOperatorErrorReportsExistingErrorResponses(t *testing.T) {
	inventory := scanAnswers(t)
	report := TreeReport{Registrations: len(inventory.Registrations), Sites: inventory.Sites, Unresolved: inventory.Unresolved}
	plain := siteOf(t, inventory.Sites, "answerPlain", "dispatcher unavailable")
	line := "plain " + answersFile + ":" + strconv.Itoa(plain.Line) + " answerPlain (rpc-handler, error-response) Error: \"dispatcher unavailable\""
	text := report.Text()
	for _, part := range []string{line, "of those, error responses: 1 typed, 5 plain, 3 pass-through"} {
		if !strings.Contains(text, part) {
			t.Errorf("report does not say %q:\n%s", part, text)
		}
	}
	if empty := checkLines(inventory.Sites, repochanged.ChangedLines{}); len(empty.Findings) != 0 {
		t.Errorf("an empty change set refused %+v", empty.Findings)
	}
	unchanged := answersSide(t, "", "", "")
	clean := changecheck.Change{Root: answersTree, Paths: []string{answersFile}, Before: unchanged, After: unchanged}
	refusal, err := judgeChangeAt(clean, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if refusal != "" {
		t.Errorf("a commit adding no plain answer was refused:\n%s", refusal)
	}
}

// TestOperatorErrorRefusesAddedPlainErrorResponse holds the refusal: a
// commit that turns a pass-through answer into text the site wrote is
// refused, naming the file and line, the answer, and the declared route as
// the fix, and not the plain answers its parent held. A changed line holding
// a plain answer is refused by the verify stage's line scope too, while the
// typed answer on a changed line is not.
func TestOperatorErrorRefusesAddedPlainErrorResponse(t *testing.T) {
	passed := siteOf(t, scanAnswers(t).Sites, "answerPassThrough", "Error: err.Error()")
	adding := changecheck.Change{
		Root:   answersTree,
		Paths:  []string{answersFile},
		Before: answersSide(t, "", "", ""),
		After:  answersSide(t, answersFile, "Error: err.Error()}", "Error: \"work failed\"}"),
	}
	refusal, err := judgeChangeAt(adding, 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	site := answersFile + ":" + strconv.Itoa(passed.Line) + ": answerPassThrough (rpc-handler registered at"
	for _, part := range []string{site, `answers an error response with Error: "work failed"`, "plugin.OperatorErrorResponse(opererr.New(problem, why, fix))"} {
		if !strings.Contains(refusal, part) {
			t.Errorf("refusal does not say %q:\n%s", part, refusal)
		}
	}
	if strings.Contains(refusal, "dispatcher unavailable") {
		t.Errorf("refusal names a plain answer the parent already held:\n%s", refusal)
	}

	inventory := scanAnswers(t)
	every := checkLines(inventory.Sites, repochanged.ChangedLines{answersFile: {{From: 1, To: 10000}}})
	if len(every.Findings) != 5 {
		t.Errorf("findings on every changed line = %d, want the 5 plain answers: %+v", len(every.Findings), every.Findings)
	}
	for _, finding := range every.Findings {
		if finding.Handler == "answerTyped" {
			t.Errorf("the declared route was refused: %+v", finding)
		}
	}
}
