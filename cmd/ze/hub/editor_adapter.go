// Design: docs/architecture/hub-architecture.md -- editor contract adapter
// Related: session_factory.go -- uses the adapter for web editor creation

package hub

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/cli/contract"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
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
func (a *editorAdapter) RenameListEntry(parentPath []string, listName, oldKey, newKey string) error {
	return a.ed.RenameListEntry(parentPath, listName, oldKey, newKey)
}
func (a *editorAdapter) CommitSession() (*contract.CommitResult, error) {
	return a.ed.CommitSession()
}
func (a *editorAdapter) Discard() error                     { return a.ed.Discard() }
func (a *editorAdapter) Diff() string                       { return a.ed.Diff() }
func (a *editorAdapter) Tree() any                          { return a.ed.Tree() }
func (a *editorAdapter) ContentAtPath(path []string) string { return a.ed.ContentAtPath(path) }

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
		}
	}
	return changes
}

// newEditorFactory creates a contract.EditorFactory that produces adapted editors.
func newEditorFactory() contract.EditorFactory {
	return func(storeAny any, configPath string) (contract.Editor, error) {
		store, ok := storeAny.(storage.Storage)
		if !ok {
			return nil, fmt.Errorf("expected storage.Storage, got %T", storeAny)
		}
		ed, err := cli.NewEditorWithStorage(store, configPath)
		if err != nil {
			return nil, err
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
