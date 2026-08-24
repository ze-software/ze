//go:build ze_web

// Design: docs/architecture/hub-architecture.md -- editor contract adapter
// Related: service_web.go -- builds the web editor manager from this adapter
//
// The build constraint is its consumer's. newEditorFactory and
// newEditSessionFactory are called from service_web.go (ze_web) and from
// web_commit_hang_repro_test.go (ze_web), and from nowhere else. A daemon built
// without ze_web starts no editor, so an unconstrained adapter would compile
// into it with no caller.

package hub

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// editorAdapter wraps *cli.Editor to satisfy contract.Editor.
// Adapts return types that differ between concrete and interface
// (Tree returns any instead of *config.Tree, SessionChanges returns
// []contract.SessionChange instead of []config.SessionEntry).
type editorAdapter struct {
	ed *cli.Editor
}

func (a *editorAdapter) SetSession(s contract.EditSession) {
	a.ed.SetSession(cli.NewEditSession(s.User, s.Origin))
}

func (a *editorAdapter) SessionID() string               { return a.ed.SessionID() }
func (a *editorAdapter) CreateEntry(path []string) error { return a.ed.CreateEntry(path) }
func (a *editorAdapter) SetValue(path []string, key, value string) error {
	return a.ed.SetValue(path, key, value)
}
func (a *editorAdapter) DeleteValue(path []string, key string) error {
	return a.ed.DeleteValue(path, key)
}
func (a *editorAdapter) DeleteByPath(fullPath []string) error {
	return a.ed.DeleteByPath(fullPath)
}
func (a *editorAdapter) RenameListEntry(parentPath []string, listName, oldKey, newKey string) error {
	return a.ed.RenameListEntry(parentPath, listName, oldKey, newKey)
}
func (a *editorAdapter) CommitSession() (*contract.CommitResult, error) {
	return a.ed.CommitSession()
}
func (a *editorAdapter) CommitSessionCandidate(stamp time.Time) (*contract.CommitResult, string, error) {
	return a.ed.CommitSessionCandidate(stamp)
}
func (a *editorAdapter) MarkCommittedContent(content string) { a.ed.MarkCommittedContent(content) }
func (a *editorAdapter) CopyListEntry(parentPath []string, listName, srcKey, dstKey string) error {
	return a.ed.CopyListEntry(parentPath, listName, srcKey, dstKey)
}
func (a *editorAdapter) DeactivatePath(path []string) error { return a.ed.DeactivatePath(path) }
func (a *editorAdapter) ActivatePath(path []string) error   { return a.ed.ActivatePath(path) }
func (a *editorAdapter) Discard() error                     { return a.ed.Discard() }
func (a *editorAdapter) DiscardSessionPath(path []string) error {
	return a.ed.DiscardSessionPath(path)
}
func (a *editorAdapter) DisconnectSession(sessionID string) error {
	return a.ed.DisconnectSession(sessionID)
}
func (a *editorAdapter) Diff() string     { return a.ed.Diff() }
func (a *editorAdapter) SaveDraft() error { return a.ed.SaveDraft() }
func (a *editorAdapter) SetPreCommitValidate(fn func(candidate string) error) {
	a.ed.SetPreCommitValidate(fn)
}
func (a *editorAdapter) ListBackups() ([]contract.BackupInfo, error) {
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
func (a *editorAdapter) Rollback(backupPath string) error   { return a.ed.Rollback(backupPath) }
func (a *editorAdapter) Tree() any                          { return a.ed.Tree() }
func (a *editorAdapter) ContentAtPath(path []string) string { return a.ed.ContentAtPath(path) }
func (a *editorAdapter) DisplayContentAtPath(path []string) string {
	return a.ed.DisplayContentAtPath(path)
}
func (a *editorAdapter) OriginalContentAtPath(path []string) string {
	return a.ed.OriginalContentAtPath(path)
}
func (a *editorAdapter) Dirty() bool              { return a.ed.Dirty() }
func (a *editorAdapter) ActiveSessions() []string { return a.ed.ActiveSessions() }

func (a *editorAdapter) SessionChanges(sessionID string) []contract.SessionChange {
	entries := a.ed.SessionChanges(sessionID)
	changes := make([]contract.SessionChange, len(entries))
	for i, e := range entries {
		changes[i] = contract.SessionChange{
			Path:     e.Path,
			Previous: e.Entry.Previous,
			Value:    e.Entry.Value,
		}
	}
	return changes
}

func (a *editorAdapter) PendingChanges(sessionID string) []contract.PendingChange {
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
			Member:   entry.Member,
		}
	}
	return changes
}

// newEditorFactory creates a contract.EditorFactory that produces adapted editors.
// The optional validateFn is called on save to validate the candidate config before writing the draft.
func newEditorFactory(validateFn func(candidate, path string) error) contract.EditorFactory {
	return func(storeAny any, configPath string) (contract.Editor, error) {
		store, ok := storeAny.(storage.Storage)
		if !ok {
			return nil, fmt.Errorf("expected storage.Storage, got %T", storeAny)
		}
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, err
		}
		if validateFn != nil {
			ed.SetPreCommitValidate(func(candidate string) error {
				return validateFn(candidate, configPath)
			})
		}
		return &editorAdapter{ed: ed}, nil
	}
}

// newEditSessionFactory creates a contract.EditSessionFactory.
func newEditSessionFactory() contract.EditSessionFactory {
	return func(username, origin string) contract.EditSession {
		s := cli.NewEditSession(username, origin)
		return contract.EditSession{
			User:   s.User,
			Origin: s.Origin,
			ID:     s.ID,
		}
	}
}
