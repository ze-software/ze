# 391 -- Concurrent Configuration Editing

## Objective

Replace the hierarchical text configuration format with a flat, line-oriented set+metadata format. Enable concurrent editing from multiple sessions (terminal, SSH) with write-through semantics, per-session commit, conflict detection, authorship tracking, and mtime-based change detection.

## Decisions

- **Set+meta as canonical disk format:** Each config line is `[metadata-prefixes] set <path> <value>`. Metadata prefixes: `#user@origin` (who), `@ISO8601` (when), `%sessionID` (pending session), `^value` (previous value for conflict detection). Files without metadata parse identically (hand-written configs work).
- **MetaTree parallels Tree:** `config.MetaTree` mirrors `config.Tree` structure. Each leaf stores `MetaEntry{User, Time, Session, Value, Previous}`. Survives schema migrations because keys follow YANG paths.
- **Write-through via Storage.AcquireLock:** Every `set`/`delete` acquires in-process lock via `Storage.AcquireLock()` returning a `WriteGuard`. No flock -- all sessions are in-process after SSH-only migration. Draft always in set+meta format.
- **Per-session commit with dual conflict detection:** `CommitSession()` checks LIVE conflicts (two sessions disagree on same path) and STALE conflicts (committed value changed since last set). Any conflict blocks the entire commit.
- **Storage.Stat for mtime polling:** `Stat(name) (FileMeta, error)` returns `{ModTime, ModifiedBy}`. FS uses `os.Stat()`. Blob tracks per-key metadata in memory, auto-set on WriteFile. `WriteGuard.SetModifier(sessionID)` tags writes with session identity. No sidecar files, no OS calls outside storage layer.
- **SSH sessions get editors:** `createSessionModel` creates `Editor + EditSession` when `ConfigPath` and `Storage` are set on ssh.Config. Falls back to command-only with warning log if editor creation fails.
- **Session adoption:** `AdoptSession(oldSessionID)` rewrites orphaned session entries to the current session. `cmd_edit.go` prompts same-user orphaned sessions on startup (prefix match on `UserAtOrigin()+":"`).
- **editor_draft.go split into 3 files:** write-through (editor_draft.go), commit/discard/disconnect (editor_commit.go), schema walking (editor_walk.go). Hub is editor_draft.go, not editor.go.

## Patterns

- **Three-format ecosystem:** Hierarchical (legacy/display), set (CLI/migration), set+meta (disk canonical). `DetectFormat()` scans ALL lines (not just first) because metadata can appear after plain set lines.
- **Storage abstraction hides backend:** `Stat`, `SetModifier`, `AcquireLock` all work identically for FS and blob. Editor/Model never branch on storage type.
- **Settle pattern for async test commands:** Commands exceeding 15ms timeout are saved to pending list. `Settle()` drains them before expectations. Retry with backoff (5/10/25/50ms) before declaring failure.
- **Session identity format:** `user@origin:unix-ts`. Prefix match for adoption uses `UserAtOrigin()+":"` to avoid "thomas@" matching "thomasmore@".
- **.et test framework extended:** `session=name:user=X,origin=Y` for multi-session, `expect=file:path=X:contains=Y` for on-disk verification, `option=session:user=X:origin=Y` for single-session activation. Multi-key expects use `:` separator (file type only).

## Gotchas

- **Same-second session IDs collide:** `NewEditSession` uses `time.Now().Unix()` -- two sessions created in the same second get identical IDs. Tests must use different origins to ensure different IDs.
- **Validator only knew hierarchical format:** Fixed by detecting format in `Validate()` and using `SetParser.ParseWithMeta()` for set formats.
- **editor_draft.go grew to 1049L:** Exceeded split threshold. Split into 3 files BEFORE adding more code (Phase 5b), not after. Splitting first made the remaining work cleaner.
- **Port conflicts in .ci tests:** Two SSH tests using the same port fail when run in parallel. Each .ci test must use a unique SSH listen port.
- **Stale conflict untestable in .et:** Pure stale conflict requires external config mutation between set and commit. .et can only test via multi-session same-path edits (which trigger live conflicts). Unit test `TestEditorConflictStale` covers the pure code path.
- **Mtime polling on Bubble Tea:** `tea.Tick` fires as a message in the single-threaded Update loop. No concurrency issues with editor state. Tick stops automatically when `hasEditor() || HasSession()` returns false.
- **Prefix match bug in adoption:** `username + "@"` matched "thomasmore@local:...". Fixed to `session.UserAtOrigin() + ":"` which matches "thomas@local:" exactly.

## Files

- `internal/component/cli/editor_draft.go` -- Write-through, AdoptSession, CheckDraftChanged, mtime sidecar
- `internal/component/cli/editor_commit.go` -- CommitSession, DiscardSessionPath, DisconnectSession, conflict detection
- `internal/component/cli/editor_walk.go` -- walkOrCreateIn, walkOrCreateMeta, walkMetaReadOnly, walkPath, getValueAtPath
- `internal/component/cli/editor_session.go` -- EditSession struct, DraftPath, LockPath
- `internal/component/cli/editor.go` -- Session-aware SetValue/DeleteValue, draftMtime field, CheckDraftChanged
- `internal/component/cli/model.go` -- draftPollMsg tick, handleDraftPoll, hasPendingChanges
- `internal/component/cli/model_commands.go` -- cmdCommitSession, cmdShowBlame/Changes/Set, cmdWho, cmdDisconnect
- `internal/component/config/meta.go` -- MetaTree, MetaEntry, session operations
- `internal/component/config/setparser.go` -- Parse/ParseWithMeta for set-format configs
- `internal/component/config/serialize_set.go` -- SerializeSet, SerializeSetWithMeta, DetectFormat
- `internal/component/config/serialize_blame.go` -- Blame view with fixed-width gutter
- `internal/component/config/storage/storage.go` -- Stat(FileMeta), SetModifier on WriteGuard
- `internal/component/config/storage/blob.go` -- In-memory per-key metas map, auto-tracked on WriteFile
- `internal/component/ssh/session.go` -- createSessionModel with Editor when ConfigPath set
- `internal/component/ssh/ssh.go` -- ConfigPath on Config struct
- `internal/component/bgp/config/loader.go` -- configPath threading through CreateReactorFromTree
- `cmd/ze/config/cmd_edit.go` -- Adoption prompt, session setup
- `cmd/ze/config/cmd_migrate.go` -- --format flag (set default)
- `internal/component/cli/testing/{parser,runner,headless,expect}.go` -- Session/file .et extensions
- `test/editor/session/*.et` -- 13 functional session tests
- `test/plugin/config-edit-ssh-session.ci` -- SSH session daemon test
