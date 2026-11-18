// Design: docs/architecture/config/change-file-structural-ops.md -- saved discard ownership.
// Related: editor_commit.go -- guarded discard before consuming change records.
// Related: editor_discard_path.go -- complete committed leaf restoration.

package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/opererr"
)

// discardSavedSessionPath removes saved ownership before its change records
// disappear. The caller MUST hold the configuration guard. Whole and path
// discard select this session only, preserving other owners of the same account.
// All loops are bounded by the parsed draft's saved entries.
func (e *Editor) discardSavedSessionPath(guard storage.WriteGuard, prefix string) error {
	path := DraftPath(e.originalPath)
	data, err := guard.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // Unsaved intent has no shared draft to clean up.
	}
	if err != nil {
		return discardSavedFailure(err)
	}
	tree, meta, err := config.NewSetParser(e.schema).ParseWithMeta(string(data))
	if err != nil {
		return discardSavedFailure(err)
	}
	affected := make(map[string]config.EditTokens)
	for _, session := range meta.AllSessions() {
		if session != e.session.ID {
			continue
		}
		for _, entry := range meta.SessionEntries(e.schema, session) {
			if prefix != "" {
				within, err := config.EditPathHasPrefix(e.schema, entry.Path.String(), prefix)
				if err != nil {
					return discardSavedFailure(err)
				}
				if !within {
					continue
				}
			}
			parts := entry.Path
			parent := walkMetaReadOnly(meta, e.schema, parts[:len(parts)-1])
			parent.RemoveSessionEntry(parts[len(parts)-1], session)
			affected[parts.String()] = parts
		}
	}
	if len(affected) == 0 {
		return nil
	}
	committedData, err := storage.ReadActiveConfigWithGuard(e.store, guard, e.originalPath)
	if err != nil {
		return discardSavedFailure(err)
	}
	committed, _, err := parseEditingConfig(string(committedData), e.schema)
	if err != nil {
		return discardSavedFailure(err)
	}
	for _, parts := range affected {
		parent := parts[:len(parts)-1]
		leaf := parts[len(parts)-1]
		target, err := e.walkOrCreateIn(tree, parent)
		if err != nil {
			return discardSavedFailure(err)
		}
		target.RestoreValue(leaf, walkPath(committed, e.schema, parent))
		// A contested leaf keeps the other owners' actual values as well as
		// their unchanged metadata, in the same precedence as the saved draft.
		if remaining := walkMetaReadOnly(meta, e.schema, parent); remaining != nil {
			for _, kept := range remaining.GetAllEntries(leaf) {
				if err := e.applySessionEntryToTree(tree, parent, leaf, kept); err != nil {
					return discardSavedFailure(err)
				}
			}
		}
	}
	content := config.SerializeSetWithMeta(tree, meta, e.schema)
	if err := guard.WriteFile(path, []byte(content), 0o600); err != nil {
		return discardSavedFailure(fmt.Errorf("write saved draft %s: %w", path, err))
	}
	return nil
}

// discardSavedFailure retains the cause while giving every editing surface
// the same storage check and retry instructions before intent is consumed.
func discardSavedFailure(cause error) *opererr.Error {
	return opererr.Wrap(cause,
		"Your saved configuration changes could not be discarded.",
		"The running configuration is unchanged. Keep the pending files; repair the reported storage or configuration error, then retry Discard all in the browser or discard all in the editor",
		"run show doctor")
}
