// Design: docs/guide/config-editor.md — serializing UI reads against dispatched commits.
// Related: model.go, model_keys.go, model_commands.go, model_commands_commit.go.

package cli

import (
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// mustModel unwraps a tea.Model returned by Model.Update with a checked type assertion.
func mustModel(t *testing.T, model tea.Model) Model {
	t.Helper()
	m, ok := model.(Model)
	require.True(t, ok, "expected Model from Update")
	return m
}

// TestModelUpdateFencesEditorDuringDispatchedCommit verifies that Update-side
// keystrokes, completions, search, config refresh, pending-change checks, and
// paste-mode completion do not race a dispatched commit mutating the shared
// Editor on its Bubble Tea command goroutine.
//
// VALIDATES: Update-side editor reads and paste-mode apply are fenced by
// dispatchQueue while commit now runs concurrently under -race.
// PREVENTS: Data races between Model.Update (updateCompletions, completionTree,
// searchConfig, hasPendingChanges, showConfigContent, finishPasteMode) and
// dispatched commit goroutines mutating Editor.tree, Editor.meta, and
// Editor.workingContent.
func TestModelUpdateFencesEditorDuringDispatchedCommit(t *testing.T) {
	const seed = `bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
	peer alpha {
		connection { remote { ip 192.0.2.1; } }
		session { asn { local 65000; remote 65001; } }
	}
}`

	ed, _, path := newLoadSessionEditor(t, seed)
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.1"))

	inReload := make(chan struct{})
	releaseReload := make(chan struct{})
	var reloadOnce sync.Once
	ed.SetReloadNotifier(func() error {
		reloadOnce.Do(func() { close(inReload) })
		<-releaseReload
		return storage.PromoteCandidate(ed.store, path)
	})

	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })

	// Dispatch "commit now" via Update just as SSH Bubble Tea does.
	m.textInput.SetValue("commit now")
	nextModel, commitCmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = mustModel(t, nextModel)
	require.NotNil(t, commitCmd, "commit now must dispatch a command")

	commitDone := make(chan tea.Msg, 1)
	go func() {
		commitDone <- commitCmd()
	}()

	select {
	case <-inReload:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatched commit did not enter reload notifier")
	}

	// Unblock the commit so MarkCommittedContent mutates e.tree, e.meta, and
	// e.workingContent concurrently while Update processes keystrokes.
	close(releaseReload)

	// Drive Update-side paths that read m.editor while the commit goroutine runs.
	typing := []string{"s", "e", "t", " ", "b", "g", "p", " ", "p", "e", "e", "r", " "}
	for range 15 {
		for _, ch := range typing {
			nm, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: ch}))
			m = mustModel(t, nm)
		}
		nm, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		m = mustModel(t, nm)
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		m = mustModel(t, nm)
		// Search mode reads displaySetView when unfenced.
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "/"}))
		m = mustModel(t, nm)
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "p"}))
		m = mustModel(t, nm)
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		m = mustModel(t, nm)
		// Empty Enter invokes showConfigContent; second Esc checks hasPendingChanges.
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		m = mustModel(t, nm)
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		m = mustModel(t, nm)
		// Cancel quit confirmation if entered.
		nm, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "n"}))
		m = mustModel(t, nm)
	}

	var resultMsg tea.Msg
	select {
	case resultMsg = <-commitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatched commit did not finish")
	}

	nm, _ := m.Update(resultMsg)
	m = mustModel(t, nm)
	require.NoError(t, m.Error())
	assert.Contains(t, m.StatusMessage(), "Session committed")
	assert.Contains(t, ed.OriginalContent(), "10.0.0.1")
}

// TestModelFinishPasteModeQueuesBehindDispatchedCommit verifies that ending
// paste mode (Ctrl-D) while a dispatched commit is in flight serializes the
// paste through dispatchQueue instead of mutating m.editor directly on Update.
//
// VALIDATES: finishPasteMode queues applyLoad on m.dispatch when busy and
// applies the pasted change after the commit completes.
// PREVENTS: Concurrent mutation of Editor between finishPasteMode on Update
// and an in-flight dispatched command.
func TestModelFinishPasteModeQueuesBehindDispatchedCommit(t *testing.T) {
	const seed = `bgp {
	router-id 1.2.3.4;
	session { asn { local 65000; } }
}`

	ed, _, path := newLoadSessionEditor(t, seed)
	require.NoError(t, ed.SetValue([]string{"bgp"}, "router-id", "10.0.0.2"))

	inReload := make(chan struct{})
	releaseReload := make(chan struct{})
	var reloadOnce sync.Once
	ed.SetReloadNotifier(func() error {
		reloadOnce.Do(func() { close(inReload) })
		<-releaseReload
		return storage.PromoteCandidate(ed.store, path)
	})

	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })

	m.textInput.SetValue("commit now")
	nm, commitCmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = mustModel(t, nm)
	require.NotNil(t, commitCmd)

	commitDone := make(chan tea.Msg, 1)
	go func() { commitDone <- commitCmd() }()

	select {
	case <-inReload:
	case <-time.After(5 * time.Second):
		t.Fatal("commit did not enter reload notifier")
	}

	// Simulate Ctrl-D finishing paste mode while the commit still owns m.editor.
	m.pasteMode = true
	m.pasteModeLocation = loadLocationAbsolute
	m.pasteModeAction = loadActionMerge
	m.pasteBuffer.Reset()
	m.pasteBuffer.WriteString("set bgp router-id 10.0.0.9\n")

	nm, pasteCmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd', Mod: tea.ModCtrl}))
	m = mustModel(t, nm)
	require.NotNil(t, pasteCmd, "paste completion while busy must queue through dispatch")

	pasteDone := make(chan tea.Msg, 1)
	go func() { pasteDone <- pasteCmd() }()

	close(releaseReload)

	var commitMsg tea.Msg
	select {
	case commitMsg = <-commitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("commit did not finish")
	}
	nm, _ = m.Update(commitMsg)
	m = mustModel(t, nm)
	require.NoError(t, m.Error())

	var pasteMsg tea.Msg
	select {
	case pasteMsg = <-pasteDone:
	case <-time.After(5 * time.Second):
		t.Fatal("queued paste did not finish")
	}
	nm, _ = m.Update(pasteMsg)
	m = mustModel(t, nm)
	require.NoError(t, m.Error())
	assert.Contains(t, ed.ContentAtPath(nil), "10.0.0.9")
}
