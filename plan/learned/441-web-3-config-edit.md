# 441 -- Config Editing

## Context

Read-only config view was useful but incomplete. Users need to set values, delete entries, and commit changes through the browser. The CLI editor already has full draft/commit/discard semantics with per-user change files -- the web layer wraps this with HTTP handlers and per-user mutex serialization.

## Decisions

- Per-user `sync.Mutex` wrapping all Editor method calls over channel+worker pattern -- simpler, sufficient for HTTP request serialization.
- `EditorManager` as a thin wrapper over CLI Editor over reimplementing edit logic -- reuses all validation, conflict detection, and change file management.
- Inline diff (color + hover for old value) over separate compare view -- always-visible changes reduce user errors.
- Commit as dedicated page with diff + confirmation over inline commit button -- forces user to review all changes before committing.
- HTML checkbox converted to "true"/"false" before Editor.SetValue() -- Editor expects config-canonical boolean strings.

## Consequences

- Editor's `SetValue`, `DeleteValue`, `CommitSession`, `Discard` are the only mutation paths -- no web-specific config modification code.
- New login invalidates previous session, old page stays visible as read-only ghost with dismissible login overlay.
- Maximum 50 concurrent editor sessions with 1-hour idle eviction -- prevents unbounded memory growth.

## Gotchas

- Editor method names: `SetValue()` not `Set()`, `CommitSession()` not `Commit()` -- the spec had wrong names initially, caught during deep review.
- `Editor.SetSession()` must be called after construction -- session is not a constructor parameter.
- The `RLock` fast path for session lookup had a write to `lastActivity` -- data race caught by `-race` detector, fixed by using full `Lock`.
- `CommitSession()` returns `*CommitResult` with `Conflicts` slice -- empty conflicts means success, non-empty means re-render with error.

## Files

- `internal/component/web/editor.go` (EditorManager, per-user sessions)
- `internal/component/web/handler_config.go` (POST handlers for set/delete/commit/discard)
- `internal/component/web/templates/commit.html`, `notification.html`
