//go:build ze_web

// Design: docs/architecture/web-interface.md -- isolated working trees.
// Related: web_editor_recovery_test.go -- production editor fixture.

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebSaveDraftKeepsOwningCandidate reaches save on a live, already isolated
// candidate before any cold-session recovery. The shared durable draft and the
// same account's unrelated SSH file remain owned by their original sessions.
func TestWebSaveDraftKeepsOwningCandidate(t *testing.T) {
	f := newWebEditorRecoveryFixture(t)
	bob := f.owner(t, "bob", "web", 1)
	require.NoError(t, bob.SetValue([]string{"system"}, "host", "bob-contested"))
	require.NoError(t, bob.SaveDraft())
	f.bobEntries = f.entries(t, cli.DraftPath("config.conf"), bob.SessionID())
	ssh := f.owner(t, "carol", "ssh", 4)
	require.NoError(t, ssh.SetValue([]string{"bgp", "group", "carol-ssh"}, "description", "ssh-kept"))
	sshEntries := f.entries(t, cli.ChangePath("config.conf", "carol"), ssh.SessionID())
	require.Len(t, sshEntries, 1)
	var current contract.Editor
	factory := newEditorFactory(zeconfigcmd.ValidateContent)
	manager := zeweb.NewEditorManager(f.store, "config.conf", f.schema,
		func(store any, path string) (contract.Editor, error) {
			editor, err := factory(store, path)
			if err != nil {
				return nil, err
			}
			current = editor
			return editor, nil
		}, newEditSessionFactory())
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	require.NoError(t, manager.SetValue("carol", []string{"system"}, "host", "visible-own"))
	before := manager.ContentAtPath("carol", nil)
	require.Contains(t, before, "visible-own")
	require.NotContains(t, before, "bob-saved")
	require.NotContains(t, before, "carol-ssh")
	require.Equal(t, 1, manager.ChangeCount("carol"))
	require.NotNil(t, current)
	working := current.WorkingContent()
	require.Contains(t, working, "visible-own")
	require.NotContains(t, working, "bob-contested")
	require.NoError(t, manager.SaveDraft("carol"))
	after := manager.ContentAtPath("carol", nil)
	assert.Equal(t, before, after, "save must preserve the isolated live candidate")
	assert.Contains(t, after, "visible-own")
	assert.NotContains(t, after, "bob-saved")
	assert.NotContains(t, after, "carol-ssh")
	assert.Equal(t, working, current.WorkingContent(), "validation must still receive only this candidate")
	assert.NotContains(t, current.WorkingContent(), "bob-contested")
	assert.Equal(t, 1, manager.ChangeCount("carol"))
	assert.Contains(t, manager.Diff("carol"), "visible-own")
	assert.NotContains(t, manager.Diff("carol"), "bob-only")
	f.checkOwners(t)
	assert.ElementsMatch(t, sshEntries,
		f.entries(t, cli.ChangePath("config.conf", "carol"), ssh.SessionID()))
}
