// Related: writeedit.go -- the proposed-edit guards these cases drive
//
// VALIDATES: the write hook reads a proposed YANG file lexically. It warns on a
// description past the character bound or the word bound, on one that is not a
// finished sentence, and on a ze:help that only repeats it.
// PREVENTS: a summary no overlay can render whole reaching the tree. Also a scan
// that stopped early reading as a file that broke no rule.
package hookruntime

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/ste"
)

// yangModulePath is a module that does not exist on disk, which is the state of
// every file the write hook judges.
const yangModulePath = "internal/component/bgp/yang/ze-demo-cmd.yang"

// yangModule writes one command node carrying a summary and a long help. An
// empty help leaves the extension out, so the module reads as one an author
// wrote before the explanation existed.
func yangModule(summary, help string) string {
	var module strings.Builder
	module.WriteString("module ze-demo-cmd {\n  namespace \"urn:ze:demo\";\n  prefix ze-demo;\n\n  container show {\n")
	module.WriteString("    description\n      \"" + summary + "\";\n")
	if help != "" {
		module.WriteString("    ze:help\n      \"" + help + "\";\n")
	}
	module.WriteString("  }\n}\n")
	return module.String()
}

// runYangWrite proposes one whole-file write of a YANG module and answers the
// hook's exit code and its message.
func runYangWrite(t *testing.T, content string) (int, string) {
	t.Helper()
	return runYangWriteAt(t, yangModulePath, content)
}

// runYangWriteAt names the module path, which decides whether a leaf renders.
func runYangWriteAt(t *testing.T, path, content string) (int, string) {
	t.Helper()
	code, _, message := runHook(t, t.TempDir(), "pretool-writeedit", map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": path, "content": content},
	})
	return code, message
}

func TestWriteEditWarnsOnAYangDescriptionPastTheCharCap(t *testing.T) {
	summary := strings.Repeat("abcdefghij ", 8) + "abcdefgh."
	if len(summary) != 97 {
		t.Fatalf("the fixture summary is %d characters, want 97", len(summary))
	}
	code, message := runYangWrite(t, yangModule(summary, ""))
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	for _, want := range []string{"container show", "char-cap", "97 characters, and the bound is 96", yangModulePath} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q: %s", want, message)
		}
	}
}

func TestWriteEditWarnsOnAYangDescriptionPastTheWordCap(t *testing.T) {
	summary := strings.Repeat("aa ", ste.MaxDescriptiveWords) + "bb."
	if got := ste.WordCount(summary); got != ste.MaxDescriptiveWords+1 {
		t.Fatalf("the fixture summary counts %d words, want %d", got, ste.MaxDescriptiveWords+1)
	}
	code, message := runYangWrite(t, yangModule(summary, ""))
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	for _, want := range []string{"container show", "word-cap", "26 words, and the bound is 25"} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q: %s", want, message)
		}
	}
}

func TestWriteEditWarnsOnAYangHelpThatRestatesItsDescription(t *testing.T) {
	summary := "Show the state of every BGP session."
	code, message := runYangWrite(t, yangModule(summary, summary))
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	for _, want := range []string{"container show", "long-restates-summary"} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q: %s", want, message)
		}
	}
}

func TestWriteEditWarnsOnAYangDescriptionWithNoFullStop(t *testing.T) {
	code, message := runYangWrite(t, yangModule("Show the state of every BGP session", ""))
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	if !strings.Contains(message, "shape, the summary does not end in a full stop") {
		t.Errorf("message missing the shape rule: %s", message)
	}
}

func TestWriteEditWarnsOnAYangDescriptionCarryingASemicolon(t *testing.T) {
	code, message := runYangWrite(t, yangModule("Show every session; the state of each one.", ""))
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	if !strings.Contains(message, "shape, the summary joins two statements with a semicolon") {
		t.Errorf("message missing the shape rule: %s", message)
	}
}

func TestWriteEditPassesAYangDescriptionWithinTheBounds(t *testing.T) {
	summary := "Show the state of every BGP session."
	help := "Each row carries the peer address, the negotiated families and the time the session came up."
	code, message := runYangWrite(t, yangModule(summary, help))
	if code != 0 {
		t.Fatalf("code = %d, want 0: %s", code, message)
	}
}

// TestWriteEditYangSummaryBoundsHoldAtTheirLastValidValue drives both bounds at
// the last value they accept and at the first they refuse. A bound written one
// out still passes a test that only feeds it a long summary.
func TestWriteEditYangSummaryBoundsHoldAtTheirLastValidValue(t *testing.T) {
	atChars := strings.Repeat("abcdefghij ", 8) + "abcdefg."
	if len(atChars) != 96 {
		t.Fatalf("the fixture summary is %d characters, want 96", len(atChars))
	}
	if code, message := runYangWrite(t, yangModule(atChars, "")); code != 0 {
		t.Errorf("a 96-character summary was refused: code=%d %s", code, message)
	}
	pastChars := strings.Repeat("abcdefghij ", 8) + "abcdefgh."
	if code, message := runYangWrite(t, yangModule(pastChars, "")); code != 1 || !strings.Contains(message, "char-cap") {
		t.Errorf("a 97-character summary was accepted: code=%d %s", code, message)
	}

	atWords := strings.Repeat("aa ", ste.MaxDescriptiveWords-1) + "bb."
	if got := ste.WordCount(atWords); got != ste.MaxDescriptiveWords {
		t.Fatalf("the fixture summary counts %d words, want %d", got, ste.MaxDescriptiveWords)
	}
	if code, message := runYangWrite(t, yangModule(atWords, "")); code != 0 {
		t.Errorf("a %d-word summary was refused: code=%d %s", ste.MaxDescriptiveWords, code, message)
	}
	pastWords := strings.Repeat("aa ", ste.MaxDescriptiveWords) + "bb."
	if code, message := runYangWrite(t, yangModule(pastWords, "")); code != 1 || !strings.Contains(message, "word-cap") {
		t.Errorf("a %d-word summary was accepted: code=%d %s", ste.MaxDescriptiveWords+1, code, message)
	}
}

// TestWriteEditSaysWhenTheProposedYangDoesNotRead holds the honesty rule. A
// scan that did not finish MUST say so. It MUST NOT answer that the file broke
// no rule (ai/rules/principles.md).
func TestWriteEditSaysWhenTheProposedYangDoesNotRead(t *testing.T) {
	code, message := runYangWrite(t, "  description\n    \"Show the state of every BGP session.\n")
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	for _, want := range []string{"does not read as YANG", "a quoted string is never closed", "no description in it was judged"} {
		if !strings.Contains(message, want) {
			t.Errorf("message missing %q: %s", want, message)
		}
	}
}

// TestWriteEditSaysWhenAYangEditNamesNoOwningStatement drives the Edit path,
// where the content is one region rather than the whole module. The bounds hold
// the statements that render on a one-line row. A text whose owner the scan
// never saw is reported as unjudged, and it is not measured.
func TestWriteEditSaysWhenAYangEditNamesNoOwningStatement(t *testing.T) {
	long := strings.Repeat("abcdefghij ", 8) + "abcdefgh."
	code, _, message := runHook(t, t.TempDir(), "pretool-writeedit", map[string]any{
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path":  yangModulePath,
			"old_string": "    description\n      \"Show the state of every BGP session.\";\n",
			"new_string": "    description\n      \"" + long + "\";\n",
		},
	})
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	if !strings.Contains(message, "one description was NOT judged") {
		t.Errorf("message does not report the unjudged text: %s", message)
	}
	if strings.Contains(message, "char-cap") {
		t.Errorf("a text with no owning statement was measured: %s", message)
	}
}

// TestWriteEditIgnoresALongModuleDescription and its revision sibling hold the
// scope the bounds carry. A module description is schema documentation that no
// one-line surface renders. Capping it invites the repair that moves the prose
// into a `//` comment, which standard YANG tooling cannot read
// (plan/spec-command-help-and-description.md, "What the Bounds Govern").
func TestWriteEditIgnoresALongModuleDescription(t *testing.T) {
	long := strings.Repeat("This module declares the BGP command tree. ", 6)
	module := "module ze-demo-cmd {\n  namespace \"urn:ze:demo\";\n  prefix ze-demo;\n\n  description\n    \"" + long + "\";\n}\n"
	if code, message := runYangWrite(t, module); code != 0 {
		t.Fatalf("a module description was judged: code=%d %s", code, message)
	}
}

func TestWriteEditIgnoresALongRevisionDescription(t *testing.T) {
	long := strings.Repeat("The revision adds the peer state leaves. ", 6)
	module := "module ze-demo-cmd {\n  namespace \"urn:ze:demo\";\n  prefix ze-demo;\n\n  revision 2026-09-03 {\n    description\n      \"" + long + "\";\n  }\n}\n"
	if code, message := runYangWrite(t, module); code != 0 {
		t.Fatalf("a revision description was judged: code=%d %s", code, message)
	}
}

// yangArgumentFixture writes one module declaring a single leaf or leaf-list
// whose description is 97 characters, which is one past the character bound.
func yangArgumentFixture(name, keyword string) string {
	long := strings.Repeat("abcdefghij ", 8) + "abcdefgh."
	return "module " + name + " {\n  namespace \"urn:ze:demo\";\n  prefix ze-demo;\n\n  " + keyword +
		" hold-time {\n    type uint16;\n    description\n      \"" + long + "\";\n  }\n}\n"
}

// judgedByModuleKind drives one over-long argument description through a
// command module, an API module and a config module. The first two must pass
// and the third must warn, naming the statement.
func judgedByModuleKind(t *testing.T, keyword string) {
	t.Helper()
	commandPath := "internal/component/bgp/yang/ze-demo-cmd.yang"
	if code, message := runYangWriteAt(t, commandPath, yangArgumentFixture("ze-demo-cmd", keyword)); code != 0 {
		t.Errorf("a %s in a command module was judged: code=%d %s", keyword, code, message)
	}
	apiPath := "internal/component/bgp/yang/ze-demo-api.yang"
	if code, message := runYangWriteAt(t, apiPath, yangArgumentFixture("ze-demo-api", keyword)); code != 0 {
		t.Errorf("a %s in an API module was judged: code=%d %s", keyword, code, message)
	}
	configPath := "internal/component/bgp/yang/ze-demo-conf.yang"
	code, message := runYangWriteAt(t, configPath, yangArgumentFixture("ze-demo-conf", keyword))
	if code != 1 {
		t.Fatalf("a %s in a config module was not judged: code=%d %s", keyword, code, message)
	}
	if !strings.Contains(message, keyword+" hold-time: char-cap") {
		t.Errorf("message does not name the config %s: %s", keyword, message)
	}
}

// TestWriteEditIgnoresALeafInACommandModuleButJudgesOneInAConfigModule holds
// the module half of the scope. extractArgDefs walks every child of a command
// container and calls argDefFor, which admits any entry with a type. A leaf
// becomes a command.ArgDef, which holds no text, so that description reaches no
// operator. The same statement in a config module renders on the completion
// row, through entryDescription.
//
// The corroboration is the gate's own silence. `leaf target` in
// internal/plugins/as112/yang/ze-as112-cmd.yang carries a semicolon and no full
// stop, and `./le docvalid help-shape` refuses nothing over it. The gate CANNOT
// stay silent over a leaf in the tree it walks.
func TestWriteEditIgnoresALeafInACommandModuleButJudgesOneInAConfigModule(t *testing.T) {
	judgedByModuleKind(t, "leaf")
}

// TestWriteEditIgnoresALeafListInACommandModuleButJudgesOneInAConfigModule is
// the sibling case. argDefFor never asks whether the child is a leaf or a
// leaf-list. A leaf-list entry carries a type, so its description dies at the
// same boundary, in the same call.
func TestWriteEditIgnoresALeafListInACommandModuleButJudgesOneInAConfigModule(t *testing.T) {
	judgedByModuleKind(t, "leaf-list")
}

// TestWriteEditJudgesALeafInsideAGrouping holds the other half of the scope.
// The owner is the NEAREST enclosing statement, so a leaf declared inside a
// grouping is judged and the grouping is not. The module is a config one,
// because a leaf renders there and in a command module it does not.
func TestWriteEditJudgesALeafInsideAGrouping(t *testing.T) {
	long := strings.Repeat("abcdefghij ", 8) + "abcdefgh."
	module := "module ze-demo-conf {\n  namespace \"urn:ze:demo\";\n  prefix ze-demo;\n\n  grouping peer-state {\n    description\n      \"" +
		strings.Repeat("The grouping carries every leaf of one peer row. ", 4) + "\";\n" +
		"    leaf hold-time {\n      type uint16;\n      description\n        \"" + long + "\";\n    }\n  }\n}\n"
	code, message := runYangWriteAt(t, "internal/component/bgp/yang/ze-demo-conf.yang", module)
	if code != 1 {
		t.Fatalf("code = %d, want 1: %s", code, message)
	}
	if !strings.Contains(message, "leaf hold-time: char-cap") {
		t.Errorf("message does not name the leaf: %s", message)
	}
	if strings.Contains(message, "grouping peer-state") {
		t.Errorf("the grouping description was judged: %s", message)
	}
}
