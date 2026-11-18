// Design: docs/architecture/cli/command-completion.md -- paged explanation ownership.
// Related: model_keys.go -- page-key admission; model_render.go -- framed projection.

package cli

// explanationPage is a projection of one revealed text at the current terminal
// geometry. Its bounds come from that text and frame, never a second metadata
// allowance. It retains no state after the key or render that requested it.
type explanationPage struct {
	lines     []string
	offset    int
	size      int
	maxOffset int
	total     int
	footer    bool
}

func (m Model) explanationPage(availableHeight int) explanationPage {
	text := sanitizeForDisplay(m.explanation)
	// Each wrapped row consumes at least one input byte. This bounds even an
	// explanation supplied outside the registered declaration-size gate.
	wrapped := wrapForBox(text, m.overlayInnerWidth(), max(len(text), 1))
	rows := max(availableHeight-2, 1)
	footer := false
	if len(wrapped) > rows {
		if rows > 1 {
			rows--
			footer = true
		}
	}
	maxOffset := max(len(wrapped)-rows, 0)
	offset := min(max(m.explanationOffset, 0), maxOffset)
	end := min(offset+rows, len(wrapped))
	return explanationPage{
		lines: wrapped[offset:end], offset: offset, size: rows,
		maxOffset: maxOffset, total: len(wrapped), footer: footer,
	}
}

// scrollExplanation moves by whole visible pages. The common projection owns
// both page size and clamping, so a resize cannot leave an unreadable offset.
func (m *Model) scrollExplanation(pages int) {
	page := m.explanationPage(max(m.screenHeight()-1, 3))
	m.explanationOffset = min(max(page.offset+pages*page.size, 0), page.maxOffset)
}
