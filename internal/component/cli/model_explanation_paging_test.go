// Design: docs/architecture/cli/command-completion.md -- explanation reveal and key ownership.
// Related: model_render.go -- framed explanation; model_keys.go -- real key dispatch.

package cli

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// TestExplanationPagingReachesEveryDeclaredRow reveals a declaration smaller
// than the real metadata bound and pages it through the actual key handler and
// composed 80x24 view. Every authored row must be reachable without moving the
// prompt or the underlying viewport; repeated keys clamp at either end.
func TestExplanationPagingReachesEveryDeclaredRow(t *testing.T) {
	model := explanationPagingModel(t, "inspect alpha ")
	model = pressTab(t, &model)
	require.NotEmpty(t, model.Explanation())
	initial := explanationPagingFrame(t, &model)
	require.Contains(t, initial, "detail-00")
	require.NotContains(t, initial, "detail-63", "the declaration must actually require paging")
	assert.Contains(t, initial, "PgUp/PgDn", "the operator must be told how to reach more text")
	viewportOffset := model.viewport.YOffset()
	seen := make(map[string]bool)
	for page := range 65 {
		frame := explanationPagingFrame(t, &model)
		for row := range 64 {
			marker := fmt.Sprintf("detail-%02d", row)
			if strings.Contains(frame, marker) {
				seen[marker] = true
			}
		}
		if page < 64 {
			model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgDown})
		}
	}
	assert.Len(t, seen, 64, "all declared rows must be reachable, not just the first clipped page")
	assert.Contains(t, explanationPagingFrame(t, &model), "detail-63")
	assert.Equal(t, "inspect alpha ", model.InputValue())
	assert.Equal(t, viewportOffset, model.viewport.YOffset(), "explanation keys must not scroll the underlying output")
	bottom := explanationPagingFrame(t, &model)
	for range 8 {
		model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgDown})
		assert.Equal(t, bottom, explanationPagingFrame(t, &model), "PgDn must clamp at the last page")
	}
	for range 64 {
		model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgUp})
	}
	assert.Equal(t, initial, explanationPagingFrame(t, &model), "PgUp must return to and clamp at the first page")
}

// TestExplanationPagingPreservesRevealLifecycle keeps a real candidate menu
// beneath a scrolled explanation. Escape, choosing another candidate and typing
// retain their existing meanings, and each new reveal begins at its first row.
func TestExplanationPagingPreservesRevealLifecycle(t *testing.T) {
	model := explanationPagingModel(t, "inspect ")
	model = pressTab(t, &model)
	require.True(t, model.ShowDropdown())
	selected := model.SelectedIndex()
	model = pressQuestionMark(t, &model)
	initial := explanationPagingFrame(t, &model)
	model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	assert.NotEqual(t, initial, explanationPagingFrame(t, &model), "PgDn must move the explanation while its menu waits")
	assert.True(t, model.ShowDropdown())
	assert.Equal(t, selected, model.SelectedIndex())
	assert.Equal(t, "inspect ", model.InputValue())

	model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Empty(t, model.Explanation())
	assert.True(t, model.ShowDropdown(), "one Escape must retain the waiting candidate menu")
	assert.Equal(t, "inspect ", model.InputValue())
	model = pressQuestionMark(t, &model)
	assert.Equal(t, initial, explanationPagingFrame(t, &model), "reopening the same explanation resets its offset")

	model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyDown})
	model = pressQuestionMark(t, &model)
	assert.Contains(t, explanationPagingFrame(t, &model), "beta-first-row", "a different candidate starts at its own beginning")
	model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Empty(t, model.Explanation(), "typing must still dismiss the explanation")
	assert.Equal(t, "inspect x", model.InputValue())
	model.textInput.SetValue("inspect alpha ")
	model.textInput.CursorEnd()
	model.updateCompletions()
	model = pressTab(t, &model)
	assert.Contains(t, explanationPagingFrame(t, &model), "detail-00", "a reveal after editing starts at its first row")
}

// TestExplanationPagingAfterQuotedContextKeepsFrame resizes the actual editor
// before Enter navigates into an ordinary quoted list key. The changed prompt
// must remain inside that same frame while an explanation is revealed/paged,
// without needing another terminal resize to repair the input width.
func TestExplanationPagingAfterQuotedContextKeepsFrame(t *testing.T) {
	editor, err := NewEditorFromContent([]byte(`bgp {
		router-id 192.0.2.1;
		session { asn { local 65000; } }
		group "edge peer" { session { asn { remote 65001; } } }
	}`), "explanation-context.conf")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, editor.Close()) })
	model, err := NewModel(editor, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = retentionUpdatedModel(t, updated)
	model.textInput.SetValue(`edit bgp group "edge peer"`)
	model.textInput.CursorEnd()
	model, execute := pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, execute)
	result, ok := execute().(commandResultMsg)
	require.True(t, ok, "Enter must execute the actual editor command")
	require.NoError(t, result.err)
	updated, _ = model.Update(result)
	model = retentionUpdatedModel(t, updated)
	require.Equal(t, []string{"bgp", "group", "edge peer"}, model.ContextPath())

	declared := strings.Repeat("A complete context explanation row.\n", 60) + "context-tail-row"
	model.SetCommandCompleter(NewCommandCompleter(&command.Node{Children: map[string]*command.Node{
		"inspect": {Name: "inspect", Description: declared},
	}}))
	model.textInput.SetValue("run inspect ")
	model.textInput.CursorEnd()
	model.updateCompletions()
	model = pressTab(t, &model)
	require.NotEmpty(t, model.Explanation())
	for page := range 65 {
		frame := ansi.Strip(model.View().Content)
		lines := strings.Split(frame, "\n")
		require.Len(t, lines, 24)
		widest := 0
		for _, line := range lines {
			widest = max(widest, ansi.StringWidth(line))
		}
		require.LessOrEqual(t, widest, 80, "the changed context prompt must fit without a second resize")
		require.Contains(t, lines[len(lines)-1], "ze[bgp group edge peer]#", "the full ordinary context prompt must survive")
		if page == 64 {
			assert.Contains(t, frame, "context-tail-row")
			break
		}
		model, _ = pressKey(t, &model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	assert.Equal(t, "run inspect ", model.InputValue())
}

func explanationPagingModel(t *testing.T, input string) Model {
	t.Helper()
	var rows []string
	for row := range 64 {
		rows = append(rows, fmt.Sprintf("detail-%02d: a complete authored explanation row.", row))
	}
	declared := strings.Join(rows, "\n")
	require.LessOrEqual(t, len("beta-first-row\n"+declared), command.MaxDescriptionBytes,
		"the fixture must fit the existing declaration allowance")
	model := NewCommandModel(FilesystemAuthorityOperatorLocal)
	t.Cleanup(func() { require.NoError(t, model.Close()) })
	resized, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model, ok := resized.(Model)
	require.True(t, ok, "the actual resize must return a CLI model")
	model.setViewportText(strings.Repeat("underlying output\n", 80))
	model.SetCommandCompleter(NewCommandCompleter(&command.Node{Children: map[string]*command.Node{
		"inspect": {Name: "inspect", Children: map[string]*command.Node{
			"alpha": {Name: "alpha", ShortHelp: "First inspection", Description: declared},
			"beta":  {Name: "beta", ShortHelp: "Second inspection", Description: "beta-first-row\n" + declared},
		}},
	}}))
	model.textInput.SetValue(input)
	model.textInput.CursorEnd()
	model.updateCompletions()
	return model
}

func explanationPagingFrame(t *testing.T, model *Model) string {
	t.Helper()
	frame := ansi.Strip(model.View().Content)
	lines := strings.Split(frame, "\n")
	require.Len(t, lines, 24, "the complete explanation view must stay inside the terminal frame")
	widest := 0
	for _, line := range lines {
		widest = max(widest, ansi.StringWidth(line))
	}
	require.LessOrEqual(t, widest, 80, "every frame row must fit the terminal width")
	require.Contains(t, lines[len(lines)-1], "ze>", "the prompt must remain on the last terminal row")
	return frame
}
