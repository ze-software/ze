package fleetui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTaskPanelPreviewTabsAreInert renders the task panel the states page
// previews, and the panel a page holds.
//
// VALIDATES: the tabs of the preview ask no fragment, while the page's own
// panel keeps its live tabs.
// PREVENTS: a preview tab that swaps in the page's real task panel, so the
// page holds two fleet-task-panel elements, the header button finds the
// gallery's, and the preview becomes a fixed overlay.
func TestTaskPanelPreviewTabsAreInert(t *testing.T) {
	preview := NewGalleryView().Overlays.Tasks
	require.True(t, preview.Preview)
	require.NotEmpty(t, preview.Tabs)

	markup := componentMarkup(t, TaskPanel(preview))
	require.NotContains(t, markup, `hx-target="#`+taskPanelID, "a preview tab swaps no panel")
	require.Equal(t, len(preview.Tabs), strings.Count(markup, `aria-disabled="true"`), "every preview tab says it is inert")
	require.NotContains(t, markup, `id="`+taskPanelID+`"`, "the preview never takes the page panel's id")
	require.Equal(t, len(preview.Tabs), strings.Count(markup, `role="tab"`))

	live := preview
	live.Preview = false
	require.Equal(t, len(live.Tabs), strings.Count(componentMarkup(t, TaskPanel(live)), `hx-target="#`+taskPanelID+`"`), "the page's panel keeps its live tabs")
}
