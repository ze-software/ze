package fleetui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// narrowMedia is the media query below which the shell takes its one narrow
// form, as fleet-shell.css and fleet.js (NARROW) both write it.
const narrowMedia = "@media (max-width: 60rem) {"

// mediaBlock answers the body of the first media block of sheet that opens
// with query, up to its matching brace.
func mediaBlock(t *testing.T, sheet, query string) string {
	t.Helper()

	start := strings.Index(sheet, query)
	require.GreaterOrEqual(t, start, 0, "the stylesheet holds %q", query)

	depth := 0
	for index := start + len(query) - 1; index < len(sheet); index++ {
		switch sheet[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return sheet[start : index+1]
			}
		default:
		}
	}
	t.Fatalf("the block %q never closes", query)

	return ""
}

// TestNarrowViewportHidesThePinnedEmptyPanel reads the narrow block of the
// shell stylesheet.
//
// VALIDATES: below 60rem the pinned side panel is not drawn while no row is
// open, as the unpinned one is not.
// PREVENTS: a stored pin that, on a narrow viewport where the panel covers
// the full width, puts the "click the eye" sentence over the whole content.
func TestNarrowViewportHidesThePinnedEmptyPanel(t *testing.T) {
	sheet, err := os.ReadFile(filepath.Join("assets", "fleet-shell.css"))
	require.NoError(t, err)

	block := mediaBlock(t, string(sheet), narrowMedia)
	require.Regexp(t,
		`(?m)^\s+\.fleet-panel-pinned \.fleet-side-panel\[data-fleet-panel="empty"\] \{\s*display: none;`, block)
}
