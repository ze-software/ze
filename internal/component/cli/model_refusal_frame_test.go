// Design: docs/guide/config-editor.md -- refusals and their fix lines
// Related: fix_frame.go -- FrameFix, FramedSetRefusal
// Related: model_commands_commit.go -- CommitRefusal, issuesInViewport
// Related: validator.go -- ConfigValidationError.Text, refusalIssue
package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// routeRefreshPath is the enum leaf the refusal tests set, from the root.
var routeRefreshPath = []string{"bgp", "peer", "peer1", "session", "capability", "route-refresh"}

const (
	routeRefreshProblem = `bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)`
	routeRefreshRootFix = "fix: set bgp peer peer1 session capability route-refresh require"
	routeRefreshTopFix  = "fix: top set bgp peer peer1 session capability route-refresh require"
)

// lastLine answers the last line of text.
func lastLine(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return lines[len(lines)-1]
}

// TestValidateValueAtPathEnumRefusal proves the editor's value check is the
// schema's own: a value outside an enumeration answers the shared refusal.
//
// VALIDATES: AC-6, AC-21 -- the path as typed, the accepted values with their
// help, the suggestion and the root-form fix; errors.As finds the
// ValidationError and the operator error.
// PREVENTS: the completer heuristic's `invalid value "x" for leaf (expected
// enum)`, which named no accepted value.
func TestValidateValueAtPathEnumRefusal(t *testing.T) {
	c := newTestCompleter(t)
	err := c.ValidateValueAtPath(routeRefreshPath, "requre")
	require.Error(t, err)

	var refusal *configyang.ValidationError
	require.True(t, errors.As(err, &refusal), "the refusal is a ValidationError")
	assert.True(t, refusal.HoldsOperatorError())
	text := err.Error()
	assert.True(t, strings.HasPrefix(text, routeRefreshProblem+"\n"), "the problem line first: %s", text)
	assert.Contains(t, text, "\nroute-refresh accepts only these values:\n")
	for _, value := range []string{"enable", "disable", "require", "refuse"} {
		assert.Contains(t, text, "\n  "+value, "every accepted value is listed")
	}
	assert.Equal(t, routeRefreshRootFix, lastLine(text))
	assert.NotContains(t, text, "expected enum")

	require.NoError(t, c.ValidateValueAtPath(routeRefreshPath, "require"), "an accepted value stays accepted")
}

// TestValidateValueAtPathRangePatternRefusal proves the set check refuses a
// range miss at set, with the range refusal, and keeps accepting what the
// parser accepts.
//
// VALIDATES: AC-14, AC-35 -- the verdict moves to the parser's: a value past
// the type width is refused with the range refusal, an in-range value and a
// zero the schema allows are accepted.
// PREVENTS: the heuristic's own range table answering differently from the
// parsers.
func TestValidateValueAtPathRangePatternRefusal(t *testing.T) {
	c := newTestCompleter(t)
	path := []string{"bgp", "peer", "peer1", "timer", "receive-hold-time"}
	require.NoError(t, c.ValidateValueAtPath(path, "90"))
	require.NoError(t, c.ValidateValueAtPath(path, "0"))

	err := c.ValidateValueAtPath(path, "99999999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `bgp peer peer1 timer receive-hold-time: "99999999" is outside the accepted range`)
	assert.True(t, strings.HasPrefix(lastLine(err.Error()), "fix: set bgp peer peer1 timer receive-hold-time <"))

	require.Error(t, c.ValidateValueAtPath(path, "abc"), "a type miss stays refused")
}

// TestEditorFixIsTheCommandTypedHere proves a refused `set` answers the
// command the operator typed, corrected, in the form they typed it.
//
// VALIDATES: AC-6 -- the root form at the root, the relative form inside
// `edit bgp peer peer1`; nothing stored.
// PREVENTS: a fix inside a context naming a root path the relative `set`
// cannot take.
func TestEditorFixIsTheCommandTypedHere(t *testing.T) {
	root, _ := topTestModel(t)
	before := root.editor.WorkingContent()
	out := enterLine(t, root, "set bgp peer peer1 session capability route-refresh requre")
	require.Error(t, out.err)
	assert.True(t, strings.HasPrefix(out.err.Error(), routeRefreshProblem+"\n"))
	assert.Equal(t, routeRefreshRootFix, lastLine(out.err.Error()))
	assert.Equal(t, before, out.editor.WorkingContent(), "nothing stored")

	m, _ := topInPeer(t)
	out = enterLine(t, m, "set session capability route-refresh requre")
	require.Error(t, out.err)
	assert.True(t, strings.HasPrefix(out.err.Error(), routeRefreshProblem+"\n"), "the problem names the full path")
	assert.Equal(t, "fix: set session capability route-refresh require", lastLine(out.err.Error()))
}

// TestTopRefusedSetFixKeepsTop proves a `set` refused behind `top` frames its
// fix from the operator's position, not from the copy's emptied one.
//
// VALIDATES: AC-48 -- `top set <full path> requre` inside a context answers
// `fix: top set <full path> require`; the position stays.
// PREVENTS: the plain root form, which the operator inside a context cannot
// type as it is.
func TestTopRefusedSetFixKeepsTop(t *testing.T) {
	m, _ := topInPeer(t)
	out := enterLine(t, m, "top set bgp peer peer1 session capability route-refresh requre")
	require.Error(t, out.err)
	assert.Equal(t, routeRefreshTopFix, lastLine(out.err.Error()))
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestEditorMultiLineErrorKeepsThePromptRow proves a refusal of several lines
// is shown whole in the viewport while the feedback row keeps its first line,
// so the prompt stays in the frame.
//
// VALIDATES: AC-6 (SSH) -- the viewport holds the whole refusal, the row the
// problem line alone, and the frame ends with the prompt.
// PREVENTS: a multi-line Error: row pushing the prompt off the screen.
func TestEditorMultiLineErrorKeepsThePromptRow(t *testing.T) {
	m, _ := topTestModel(t)
	out := enterLine(t, m, "set bgp peer peer1 session capability route-refresh requre")
	require.Error(t, out.err)

	assert.Contains(t, out.viewportContent, routeRefreshProblem)
	assert.Contains(t, out.viewportContent, routeRefreshRootFix, "the whole refusal is in the viewport")
	row := out.feedbackLine()
	assert.NotContains(t, row, "\n", "the feedback row is one line")
	assert.Contains(t, row, routeRefreshProblem)
	assert.NotContains(t, row, "accepts only these values")

	frame := strings.Split(strings.TrimRight(out.View().Content, "\n"), "\n")
	assert.LessOrEqual(t, len(frame), out.height, "the frame fits the terminal")
	assert.Contains(t, strings.Join(frame[len(frame)-3:], "\n"), "#", "the prompt stays among the last rows")
}

// candidateWithRefusedValue answers m with route-refresh requre stored in the
// candidate past the set check, as a session set from another surface or an
// older file can leave it.
func candidateWithRefusedValue(t *testing.T, m Model) Model {
	t.Helper()
	require.NoError(t, m.editor.SetValue(routeRefreshPath[:len(routeRefreshPath)-1], "route-refresh", "requre"))
	return m
}

// TestCommitFixUsesTopInsideAContext proves a refused commit lists each issue
// in the viewport with an editor fix written for the operator's position.
//
// VALIDATES: AC-23 -- at the root `fix: set <full path> require`, inside a
// context `fix: top set <full path> require`; the status row holds the
// header alone; never the file-form line.
// PREVENTS: a commit refusal on one status row joined by `; `, and a fix the
// operator cannot type where they are.
func TestCommitFixUsesTopInsideAContext(t *testing.T) {
	root, _ := topTestModel(t)
	root = candidateWithRefusedValue(t, root)
	out, result := enterLineResult(t, root, "commit now")
	require.NoError(t, out.err)
	assert.True(t, strings.HasPrefix(result.statusMessage, "commit blocked: 1 error(s)"))
	assert.NotContains(t, result.statusMessage, "\n", "the header alone on the status row")
	assert.Contains(t, out.viewportContent, "  "+routeRefreshRootFix)
	assert.NotContains(t, out.viewportContent, "route-refresh require;", "never the file form")

	m, _ := topInPeer(t)
	m = candidateWithRefusedValue(t, m)
	out, _ = enterLineResult(t, m, "commit now")
	assert.Contains(t, out.viewportContent, "  "+routeRefreshTopFix)
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestTopCommitFixUsesTopSetInsideAContext proves `top commit` frames its
// fixes from the operator's position, saved before `top` empties the copy's.
//
// VALIDATES: AC-53 -- `top commit now` inside a context answers `top set`.
// PREVENTS: a framer reading the copy's root position and printing the plain
// form to an operator inside a context.
func TestTopCommitFixUsesTopSetInsideAContext(t *testing.T) {
	m, _ := topInPeer(t)
	m = candidateWithRefusedValue(t, m)
	out, _ := enterLineResult(t, m, "top commit now")
	assert.Contains(t, out.viewportContent, "  "+routeRefreshTopFix)
	assert.NotContains(t, out.viewportContent, "  "+routeRefreshRootFix)
	assert.Equal(t, topPeerContext, out.contextPath)
}

// TestCommitRefusalWritesOneIssuePerLine proves CommitRefusal writes its
// header, then each issue as a block of its own lines.
//
// VALIDATES: AC-23 (Q-7 a) -- no trailing `: ` on the header, one block per
// issue, every line of a block indented two spaces, the line number first.
// PREVENTS: issues joined by `; ` on one line.
func TestCommitRefusalWritesOneIssuePerLine(t *testing.T) {
	result := ConfigValidationResult{Errors: []ConfigValidationError{
		{Line: 3, Message: "first problem\nfirst why\nfix: first fix", Severity: severityError},
		{Message: "second problem\nfix: second fix", Severity: severityError},
	}}
	text, blocked := CommitRefusal(result, false, "commit now force", nil)
	require.True(t, blocked)
	want := "commit blocked: 2 error(s), 0 warning(s)\n" +
		"  line 3: first problem\n  first why\n  fix: first fix\n" +
		"  second problem\n  fix: second fix"
	assert.Equal(t, want, text)
}

// TestMissingMandatoryHintUsesTopInsideAContext proves the missing-mandatory
// hint is a typed fix framed when it is shown.
//
// VALIDATES: AC-51 -- `(set bgp peer ...)` at the root as before, `(top set
// bgp peer ...)` inside a context, from one issue computed once.
// PREVENTS: a root-form hint the context-relative `set` cannot take.
func TestMissingMandatoryHintUsesTopInsideAContext(t *testing.T) {
	issue := missingFieldIssue(0, "peer2", "", []string{"session", "asn", "remote"})
	atRoot := `peer peer2: missing required field "session/asn/remote" (set bgp peer peer2 session asn remote <value>)`
	assert.Equal(t, atRoot, issue.Message)
	assert.Equal(t, atRoot, issue.Text(nil))
	assert.Equal(t, `peer peer2: missing required field "session/asn/remote" (top set bgp peer peer2 session asn remote <value>)`,
		issue.Text([]string{"ospf"}))

	m, _ := topInPeer(t, "set bgp peer peer2 connection remote ip 2.2.2.2")
	out, _ := enterLineResult(t, m, "commit now")
	assert.Contains(t, out.viewportContent, "(top set bgp peer peer2 ")
	assert.NotContains(t, out.viewportContent, "(set bgp peer peer2 ")
}

// TestCommitVerifyShowsIssuesInTheViewport proves `commit verify` with issues
// shows them as a refused commit does, and the next refresh brings the
// config view back.
//
// Method: a candidate holding route-refresh requre is verified at the root
// and inside `edit bgp peer peer1`; then `option errors hints`, an incidental
// view, redraws the position's config view.
//
// VALIDATES: AC-23 and AC-54 (Q-7 a) -- the header alone on the status row;
// the header first in the viewport, then each issue on its own indented
// lines, the fix framed for the position (`set` at the root, `top set`
// inside a context); applies nothing; the next refresh shows the config view.
// PREVENTS: commit verify printing its issues on the status row, or leaving
// its report up after a later change.
func TestCommitVerifyShowsIssuesInTheViewport(t *testing.T) {
	cases := []struct {
		name    string
		inPeer  bool
		fixLine string
	}{
		{name: "at the root", fixLine: routeRefreshRootFix},
		{name: "in a context", inPeer: true, fixLine: routeRefreshTopFix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := topTestModel(t)
			if tc.inPeer {
				m = enterLine(t, m, "edit bgp peer peer1")
				require.Equal(t, topPeerContext, m.contextPath)
			}
			m = candidateWithRefusedValue(t, m)
			position := m.contextPath

			out, result := enterLineResult(t, m, "commit verify")
			require.NoError(t, out.err)
			header := "commit verify: nothing was applied; 1 error(s), "
			assert.True(t, strings.HasPrefix(result.statusMessage, header), "status %q", result.statusMessage)
			assert.NotContains(t, result.statusMessage, "\n", "the header alone on the status row")
			assert.False(t, out.showingConfig, "the report replaces the config view")

			lines := strings.Split(strings.TrimRight(out.viewportContent, "\n"), "\n")
			require.Greater(t, len(lines), 2)
			assert.Equal(t, result.statusMessage, lines[0], "the header first in the viewport")
			for _, line := range lines[1:] {
				assert.True(t, strings.HasPrefix(line, "  "), "every issue line indented: %q", line)
			}
			assert.Contains(t, out.viewportContent, routeRefreshProblem+"\n", "the problem ends its own line")
			assert.Contains(t, lines, "  "+tc.fixLine, "the fix framed for the position, on its own line")
			assert.Equal(t, position, out.contextPath)
			assert.True(t, out.editor.Dirty(), "nothing was applied")

			refreshed := enterLine(t, out, "option errors hints")
			require.NoError(t, refreshed.err)
			ref := refreshed
			ref.setViewportData(*ref.configViewAtPath(position))
			assert.True(t, refreshed.showingConfig, "the config view is back")
			assert.Equal(t, ref.viewportContent, refreshed.viewportContent)
		})
	}
}
