// Design: docs/architecture/cli/command-completion.md -- prompt and input frame ownership.
// Related: model_render.go -- composed view and hardware cursor; model_keys.go -- input admission.

package cli

import "charm.land/lipgloss/v2"

// fitPromptInput derives one input allowance from the current rendered prompt.
// Resize, keys and View MUST share this producer: navigation and mode changes
// alter the prompt without another terminal size event. The extra cell belongs
// to textinput's cursor, which is drawn in addition to its padded value width.
func (m *Model) fitPromptInput() {
	width := m.width
	if width <= 0 {
		width = terminalWidthFallback
	}
	width = max(width-lipgloss.Width(m.buildPrompt())-1, 1)
	if m.textInput.Width() == width {
		return
	}
	m.textInput.SetWidth(width)
	// SetWidth alone leaves the old horizontal offsets intact. Reflow from
	// the end, then restore the actual cursor: this also handles a cursor in
	// the middle of a value wider than the new allowance without editing it.
	position := m.textInput.Position()
	m.textInput.CursorEnd()
	m.textInput.SetCursor(position)
}
