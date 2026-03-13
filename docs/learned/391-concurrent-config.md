# 391 -- Concurrent Configuration Editing

## Objective

Replace the hierarchical text configuration format with a flat, line-oriented set+metadata format. Enable concurrent editing from multiple sessions (terminal, SSH) with write-through semantics, per-session commit, conflict detection, and authorship tracking.

## Decisions

- **Set+meta as canonical disk format:** Each config line is `[metadata-prefixes] set <path> <value>`. Metadata prefixes: `#user@origin` (who), `@ISO8601` (when), `%sessionID` (pending session), `^value` (previous value for conflict detection). Files without metadata parse identically (hand-written configs work).
- **MetaTree parallels Tree:** `config.MetaTree` mirrors `config.Tree` structure. Each leaf stores `MetaEntry{User, Time, Session, Value, Previous}`. Survives schema migrations because keys follow YANG paths.
- **Write-through protocol:** Every `set`/`delete` acquires flock, reads draft, applies change, writes draft, releases flock. No in-memory accumulation. Draft is always in set+meta format.
- **Per-session commit with conflict detection:** `CommitSession()` reads committed config (auto-detecting format), reads draft, finds this session's entries, checks for LIVE conflicts (two sessions disagree) and STALE conflicts (committed value changed since last set). Any conflict blocks the entire commit.
- **Format detection:** `DetectFormat()` scans all lines (not just first) because metadata annotations can appear after plain set lines. Returns `FormatHierarchical`, `FormatSet`, or `FormatSetMeta`.
- **Validator made format-aware:** `ConfigValidator.Validate()` detects format and uses `SetParser.ParseWithMeta()` for set/set-meta content, hierarchical parser for traditional format. Without this, all session-mode commits fail validation.
- **Migration on first session commit:** When `CommitSession()` finds the committed config is hierarchical, it runs `migration.Migrate()` (neighbor-to-peer renaming etc.) before converting to set+meta format. Subsequent commits skip migration.
- **`cmdCommitConfirmed` routes through CommitSession:** When session is active, uses `CommitSession()` instead of `Save()`, preserving set+meta format and conflict detection.
- **`--format` flag on `ze config migrate`:** Supports `set` (default) and `hierarchical` output formats.
- **Flaky editor test fix:** `processCmdWithDepth` timeout (15ms) was permanently dropping command results. Fixed by saving timed-out channels to `pending` list and draining them before expectations with progressive retry backoff (5/10/25/50ms).

## Patterns

- **Three-format ecosystem:** Hierarchical (legacy/display), set (CLI/migration), set+meta (disk canonical). `DetectFormat()` + appropriate parser at every boundary.
- **Advisory locking with flock:** `editor_lock.go` uses `syscall.Flock` for concurrent draft access. Lock acquired per operation, not held across session lifetime.
- **Settle pattern for async test commands:** Commands that exceed timeout are saved (not dropped). `Settle()` non-blocking drains pending channels. Expectations retry with exponential backoff before declaring failure.
- **Session identity:** `EditSession{ID, User, Origin}` where ID = `user:unixnano`. Embedded in metadata prefixes for conflict attribution.
- **buildCommitMeta:** Starts from existing committed metadata (preserving prior annotations), overlays this session's changes with commit timestamp, removes metadata for deleted leaves (no tombstones).

## Gotchas

- **Validator only knew hierarchical format:** `ConfigValidator.Validate()` always used the hierarchical parser. When `WorkingContent()` returns set+meta format (session active), validation failed with "unknown top-level keyword: set". Both `cmdCommitSession()` and `cmdCommitConfirmed()` were affected. Fix: detect format and use `SetParser.ParseWithMeta()` for set formats.
- **SetParser.Parse() does not handle metadata prefixes:** `@`, `%`, `^` lines cause "unknown command" errors. Must use `ParseWithMeta()` for `FormatSetMeta` content.
- **Editor test flakiness under load:** 114 concurrent tests cause file I/O to exceed the 15ms command timeout. The original code discarded the result channel (garbage collected). Fix required saving channels and draining before assertions.
- **YANG validation produces warnings for incomplete configs:** Test configs need `router-id`, `local-address`, and `peer-as` to avoid YANG semantic warnings that block commits.
- **`DetectFormat()` must scan ALL lines:** Early return on first `set` line would miss metadata annotations that appear later in the file, causing data loss (metadata lines skipped as comments).

## Files

- `internal/component/cli/editor_draft.go` (NEW) -- CommitSession, write-through, conflict detection, buildCommitMeta
- `internal/component/cli/editor_lock.go` (NEW) -- Advisory flock-based locking
- `internal/component/cli/editor_session.go` (NEW) -- EditSession struct, session management
- `internal/component/config/meta.go` (NEW) -- MetaTree parallel to Tree for per-node metadata
- `internal/component/config/serialize_set.go` (NEW) -- SerializeSet, SerializeSetWithMeta, DetectFormat
- `internal/component/config/serialize_blame.go` (NEW) -- Blame view serialization
- `internal/component/cli/validator.go` -- Made Validate() format-aware (set/hierarchical)
- `internal/component/cli/editor.go` -- WorkingContent returns set+meta when session active, SetSession initializes meta
- `internal/component/cli/model_commands.go` -- cmdCommitSession, cmdDiscardSession, session-aware commit/show
- `internal/component/cli/model_load.go` -- cmdCommitConfirmed routes through CommitSession
- `internal/component/cli/completer.go` -- Removed unused sessionActive field
- `internal/component/cli/testing/headless.go` -- Settle pattern for flaky test fix
- `internal/component/cli/testing/runner.go` -- Retry with backoff before expectations
- `internal/component/config/setparser.go` -- Parse/ParseWithMeta for set-format configs
- `cmd/ze/config/cmd_migrate.go` -- Added --format flag
- `cmd/ze/config/cmd_edit.go` -- Session setup on editor startup
