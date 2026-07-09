package web

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMobileViewportCSS is the AC-7 mobile proof at the asset tier: the
// stylesheet carries a 390px breakpoint with a horizontal-scroll guard, and the
// key page templates (login, dashboard/layout, workbench) declare a responsive
// viewport. The .wb viewport assertions cover the rendered behavior; this test
// guards the static prerequisites, which is the layer runnable without a
// browser.
//
// VALIDATES: the 390px media block and the device-width viewport meta exist.
// PREVENTS: a regression that drops the mobile breakpoint or the viewport meta,
// which would reintroduce horizontal scroll on phones.
func TestMobileViewportCSS(t *testing.T) {
	css, err := fs.ReadFile(assetsFS, "assets/style.css")
	require.NoError(t, err)
	style := string(css)
	assert.Contains(t, style, "@media (max-width: 390px)", "390px mobile breakpoint must exist")
	assert.Contains(t, style, "overflow-x: hidden", "390px block needs the horizontal-scroll guard")
	assert.Contains(t, style, ".finder-table { display: block; overflow-x: auto;", "wide tables must scroll inside their box")

	for _, page := range []string{
		"templates/page/login.html",
		"templates/page/layout.html",
		"templates/page/workbench.html",
	} {
		data, readErr := fs.ReadFile(templatesFS, page)
		require.NoErrorf(t, readErr, "read %s", page)
		assert.Containsf(t, string(data), `name="viewport"`, "%s must declare a viewport meta", page)
		assert.Containsf(t, string(data), "width=device-width", "%s viewport must be responsive", page)
	}
}
