// Design: docs/architecture/config/change-file-structural-ops.md -- shared and isolated views.
// Related: editor_draft.go -- saving keeps the constructor's visible-view policy.
// Related: editor_commit.go -- replay materializes the shared draft before projection.

package cli

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
)

// editorViewScope is an immutable constructor choice, not a session lifecycle.
// Both possible values are valid. Zero keeps the shared draft view, including
// for existing partial Editor values. NewIsolatedEditorWithStorage chooses the
// private candidate before returning it; callers cannot change the scope.
type editorViewScope bool

const (
	editorViewShared   editorViewScope = false
	editorViewIsolated editorViewScope = true
)

func (scope editorViewScope) isolatesSession() bool {
	return bool(scope)
}

// draftTree selects the visible result of saving into the shared draft.
func (scope editorViewScope) draftTree(current, shared *config.Tree) *config.Tree {
	if scope.isolatesSession() {
		return current
	}
	return shared
}

// workingViewMetadata leaves shared CLI annotations intact. Only the isolated
// candidate omits other owners' pending values from serialized candidate text.
// Callers MUST have an active session and nonnil metadata.
func (e *Editor) workingViewMetadata() *config.MetaTree {
	if e.viewScope.isolatesSession() {
		return e.workingSessionMetadata()
	}
	return e.meta
}

// projectSessionTree uses the same committed structural replay as staging.
// The materialized draft supplies saved addresses and list order, never another
// session's visible values. Inputs come from parsed configuration and change
// records; those records bound both the entry and structural-operation loops.
func (e *Editor) projectSessionTree(committed, materialized *config.Tree, meta *config.MetaTree, ops []config.StructuralOp) (*config.Tree, error) {
	entries := meta.SessionEntriesOrdered(e.schema, e.session.ID, materialized)
	applyEntries := func() error {
		for _, entry := range entries {
			parent := entry.Path[:len(entry.Path)-1]
			leaf := entry.Path[len(entry.Path)-1]
			if err := e.applySessionEntryToTree(committed, parent, leaf, entry.Entry); err != nil {
				return err
			}
		}
		return nil
	}
	if err := e.applyStructuralOpsAroundEntries(committed, ops, applyEntries, nil); err != nil {
		return nil, fmt.Errorf("rebuild candidate: %w", err)
	}
	return committed, nil
}
