// Design: docs/architecture/web-components.md -- backup workspace document scrolling.
// Related: backup_workspace_integration_test.go -- authenticated production page fixture.
//go:build integration

package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBrowserBackupWorkspaceScroll proves that an operator with a narrow
// viewport can reach the saved-backup download using the mouse wheel. Native
// clicks can scroll hidden overflow programmatically, so the test observes the
// control before any click or focus action can bring it into view.
func TestBrowserBackupWorkspaceScroll(t *testing.T) {
	uploadPath, _ := backupBrowserArtifact(t)
	browser := newBackupBrowser(t, newTestEditorManager(t))
	browser.command("set", "viewport", "390", "640")
	browser.wait(`!!document.getElementById('backup-upload-file')`, "backup upload form")
	browser.command("upload", "#backup-upload-file", uploadPath)
	browser.command("click", "#backup-upload-submit")
	browser.wait(`!!document.getElementById('backup-config-select')`, "uploaded backup")
	browser.command("select", "#backup-config-select", "router.conf")
	browser.command("click", "#backup-config-open")
	browser.wait(`!!document.getElementById('backup-content')`, "selected configuration editor")
	require.Equal(t, "true", browser.evaluate(`window.scrollY === 0`),
		"the configuration page must start at the top before the wheel proof")
	require.Equal(t, "true", browser.evaluate(`document.getElementById('backup-download').getBoundingClientRect().top > window.innerHeight`),
		"the download control must initially be below the narrow viewport")

	// The left page gutter targets the document, outside the textarea's own
	// scroll container. These native inputs must move the visible document.
	browser.command("mouse", "move", "4", "320")
	browser.command("mouse", "wheel", "2500")
	browser.command("wait", "--timeout", "3000", "--fn", `(() => {
		const link = document.getElementById('backup-download');
		const box = link.getBoundingClientRect();
		return window.scrollY > 0 && box.top >= 0 && box.bottom <= window.innerHeight &&
			box.left >= 0 && box.right <= window.innerWidth &&
			document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2) === link;
	})()`)
}
