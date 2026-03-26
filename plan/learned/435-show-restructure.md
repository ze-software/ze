# 435 -- Show Restructure

## Context

The config editor's `show` command displayed set+meta format when a session was active, which is the internal save/draft format -- not a useful display for users. The metadata format used compound fields (`#user@origin`, `%user@origin:unixtime`) that couldn't be individually toggled. The goal was to restructure `show` into a clean display command with independently togglable metadata columns, pipe operators for format and compare, source selectors for viewing alternate configs, and proper tree format as the default.

## Decisions

- Split column toggles into a separate `option` command over keeping them in `show`, because `show` is a display verb (produces viewport output) while `option` is a settings verb (modifies preferences). This prevents `show author enable` from ambiguously meaning "display" or "configure".
- Legacy metadata format (`#user@origin @ISO8601 %session-id`) detected via `@` in the `#` field and `ISO8601` pattern in `@` field when Source is already set, over requiring explicit format version markers. The parser is stateful within a line: if `#` already set Source, `@` becomes Time.
- `compare <username>` builds baseline by cloning the working tree and reverting the user's changes via MetaTree `Previous` values, over filtering the draft file by session. This reuses existing MetaTree infrastructure without needing per-user draft storage.
- `show confirmed`/`show saved` added as source selectors on the `show` command via `cmdShowDisplayWithSource`, over adding them as pipe operators. Sources select WHAT to display; pipes modify HOW it's displayed.
- `parsePipeFilters` extended to consume a second arg when `compare rollback` is followed by a number, over restructuring PipeFilter to support multi-word args. Minimal change, handles the only two-word compare target.

## Consequences

- `show` always returns tree format. `option` manages column preferences. Old `show blame`/`show changes` redirected to `option` with helpful error message.
- Old draft files with compound metadata round-trip correctly through the parser. No migration tool needed.
- The `ContentWithoutUser` approach works for single-user revert but does not compose for "show config without users A and B." If multi-user revert is needed, the MetaTree walk would need to accept a set of usernames.
- `showAlternateSource` renders committed/saved content without metadata columns (no MetaTree for those sources). Adding annotated views of alternate sources would require parsing their metadata separately.

## Gotchas

- `parsePipeFilters` only handles `compare rollback N` as a special case. Any future two-word compare targets would need similar treatment.
- `revertUserChanges` traverses MetaTree containers which include both YANG containers and list-level wrappers. The Tree API differs: `GetContainer` for YANG containers, `GetList` for list maps. The traversal handles both by trying container first, then checking for list entries inside.
- The guard process for session-state.md can race with edits when multiple sessions are active, causing the pre-write-go hook to fail intermittently. Appending the spec name to session-state.md must happen right before the edit.

## Files

- `internal/component/cli/model_commands_show.go` -- show command, source selectors, compare baseline resolution
- `internal/component/cli/model_commands_option.go` -- column toggles, blame, changes (split from show)
- `internal/component/cli/editor_annotated.go` -- AnnotatedView, ContentWithoutUser, revertUserChanges
- `internal/component/config/serialize_annotated.go` -- ShowColumns, SerializeAnnotatedTree/Set
- `internal/component/config/setparser.go` -- extractMeta legacy format detection
- `internal/component/cli/model_load.go` -- parsePipeFilters rollback N handling
- `internal/component/cli/model_render.go` -- help overlay updated for option/show grammar
- `internal/component/cli/completer.go` -- rollback added to compare completions
- `test/editor/pipe/show-compare-rollback.et` -- rollback compare functional test
