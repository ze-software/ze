// Design: website/AI.md -- activity levels remain visible in both themes.
package site

import (
	"strings"
	"testing"
)

// TestActivityPublishesThemeAwareSequentialLevels checks the real page producer,
// including the zero level and the surfaces surrounding the calendar.
func TestActivityPublishesThemeAwareSequentialLevels(t *testing.T) {
	paths := activityFixturePaths(t)
	activityRenderedBody(t, paths)
	page := readArtifact(t, paths.Output, activityDest)
	for _, want := range []string{"#8fd5c4", "#45b69e", "#218872", "#12614f", "#083d32", "#237e6c", "#2ca58d", "#54c9ad", "#8ae3c8", "#baf5dd", `:root[data-theme="dark"] .activity-widget`, "--activity-ink: var(--text)", "--activity-card: var(--panel)", "--activity-border: var(--line-strong)", "--activity-level-0: var(--line)"} {
		if !strings.Contains(page, want) {
			t.Errorf("published activity page omits %q", want)
		}
	}
	for _, old := range []string{"--activity-ink: #09111d", "background: var(--mint-chip)", "rgba(255, 254, 254, 0.94)"} {
		if strings.Contains(page, old) {
			t.Errorf("published activity retains pale surface %q", old)
		}
	}
}
