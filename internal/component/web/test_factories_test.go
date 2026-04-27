package web

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func testEditorFactory() contract.EditorFactory {
	return func(storeAny any, configPath string) (contract.Editor, error) {
		store, ok := storeAny.(storage.Storage)
		if !ok {
			return nil, fmt.Errorf("expected storage.Storage, got %T", storeAny)
		}
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, err
		}
		return &testEditorAdapter{ed: ed}, nil
	}
}

func testEditSessionFactory() contract.EditSessionFactory {
	return func(username, origin string) contract.EditSession {
		s := cli.NewEditSession(username, origin)
		return contract.EditSession{
			User:   s.User,
			Origin: s.Origin,
			ID:     s.ID,
		}
	}
}

// testEditorAdapter adapts *cli.Editor to contract.Editor for tests.
type testEditorAdapter struct {
	ed *cli.Editor
}

func (a *testEditorAdapter) SetSession(s contract.EditSession) {
	a.ed.SetSession(cli.NewEditSession(s.User, s.Origin))
}
func (a *testEditorAdapter) SessionID() string               { return a.ed.SessionID() }
func (a *testEditorAdapter) CreateEntry(path []string) error { return a.ed.CreateEntry(path) }
func (a *testEditorAdapter) SetValue(path []string, key, value string) error {
	return a.ed.SetValue(path, key, value)
}
func (a *testEditorAdapter) DeleteValue(path []string, key string) error {
	return a.ed.DeleteValue(path, key)
}
func (a *testEditorAdapter) RenameListEntry(parentPath []string, listName, oldKey, newKey string) error {
	return a.ed.RenameListEntry(parentPath, listName, oldKey, newKey)
}
func (a *testEditorAdapter) CommitSession() (*contract.CommitResult, error) {
	return a.ed.CommitSession()
}
func (a *testEditorAdapter) CopyListEntry(parentPath []string, listName, srcKey, dstKey string) error {
	return a.ed.CopyListEntry(parentPath, listName, srcKey, dstKey)
}
func (a *testEditorAdapter) DeactivatePath(path []string) error { return a.ed.DeactivatePath(path) }
func (a *testEditorAdapter) ActivatePath(path []string) error   { return a.ed.ActivatePath(path) }
func (a *testEditorAdapter) Discard() error                     { return a.ed.Discard() }
func (a *testEditorAdapter) DiscardSessionPath(path []string) error {
	return a.ed.DiscardSessionPath(path)
}
func (a *testEditorAdapter) DisconnectSession(sessionID string) error {
	return a.ed.DisconnectSession(sessionID)
}
func (a *testEditorAdapter) Diff() string     { return a.ed.Diff() }
func (a *testEditorAdapter) SaveDraft() error { return a.ed.SaveDraft() }
func (a *testEditorAdapter) ListBackups() ([]contract.BackupInfo, error) {
	backups, err := a.ed.ListBackups()
	if err != nil {
		return nil, err
	}
	result := make([]contract.BackupInfo, len(backups))
	for i, b := range backups {
		result[i] = contract.BackupInfo{Path: b.Path, Timestamp: b.Timestamp.Format("2006-01-02 15:04:05")}
	}
	return result, nil
}
func (a *testEditorAdapter) Rollback(backupPath string) error   { return a.ed.Rollback(backupPath) }
func (a *testEditorAdapter) Tree() any                          { return a.ed.Tree() }
func (a *testEditorAdapter) ContentAtPath(path []string) string { return a.ed.ContentAtPath(path) }
func (a *testEditorAdapter) OriginalContentAtPath(path []string) string {
	return a.ed.OriginalContentAtPath(path)
}
func (a *testEditorAdapter) ActiveSessions() []string { return a.ed.ActiveSessions() }
func (a *testEditorAdapter) SessionChanges(sessionID string) []contract.SessionChange {
	entries := a.ed.SessionChanges(sessionID)
	changes := make([]contract.SessionChange, len(entries))
	for i, e := range entries {
		changes[i] = contract.SessionChange{Path: e.Path, Previous: e.Entry.Previous, Value: e.Entry.Value}
	}
	return changes
}
func (a *testEditorAdapter) PendingChanges(sessionID string) []contract.PendingChange {
	entries := a.ed.PendingChanges(sessionID)
	changes := make([]contract.PendingChange, len(entries))
	for i, entry := range entries {
		changes[i] = contract.PendingChange{
			Kind:     contract.PendingChangeKind(entry.Kind),
			Path:     entry.Path,
			Previous: entry.Previous,
			Value:    entry.Value,
			OldPath:  entry.OldPath,
			NewPath:  entry.NewPath,
		}
	}
	return changes
}
