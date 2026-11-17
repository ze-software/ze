// Design: docs/architecture/storage-backends.md -- editor records beside config mirrors.
// Related: ../../cli/editor_session.go -- the editor's path producers use these suffixes.

package storage

import "strings"

// EditorDraftSuffix identifies the shared saved draft belonging to a config.
const EditorDraftSuffix = ".draft"

// EditorChangeSeparator separates a config name from its pending-change account.
const EditorChangeSeparator = ".change."

// artifactEditorRecord recognizes only siblings owned by a name in this census.
// Suffixes alone do not reserve names: a standalone mirror remains a config.
func artifactEditorRecord(name string, names map[string]bool) bool {
	if owner, ok := strings.CutSuffix(name, EditorDraftSuffix); ok {
		if names[owner] {
			return true
		}
	}
	// An account may itself contain the change separator, so examine every
	// split. The loop consumes one separator per iteration, bounded by name.
	offset := 0
	for remaining := name; remaining != ""; {
		prefix, account, found := strings.Cut(remaining, EditorChangeSeparator)
		if !found {
			return false
		}
		offset += len(prefix)
		if account != "" {
			if names[name[:offset]] {
				return true
			}
		}
		offset += len(EditorChangeSeparator)
		remaining = account
	}
	return false
}
