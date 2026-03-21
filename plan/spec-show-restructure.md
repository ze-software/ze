# Spec: show-restructure

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - config editor architecture
4. `internal/component/cli/model_commands.go` - current show dispatch
5. `internal/component/config/serialize_blame.go` - blame serializer (gutter pattern)
6. `internal/component/config/serialize_set.go` - set+meta serializer (metadata prefix format)
7. `internal/component/config/meta.go` - MetaEntry, MetaTree
8. `internal/component/cli/editor_session.go` - EditSession (User, Origin, ID)

## Task

Restructure the `show` command in the config editor. Currently `show` displays set+meta format when a session is active, which is the save/draft format. The display should instead use hierarchical (JUNOS-style) tree format by default, with individually togglable metadata columns.

Three changes:
1. **Metadata format**: Replace compound session ID (`%user@origin:unixtime`) with three atomic fields: `#user`, `@timestamp`, `%source` (origin IP or "local").
2. **Display columns**: Four independently togglable columns (author, date, source, changes), each stored as a sticky preference in the blob DB at `/meta/show/<column>`. All default to disabled.
3. **Show command grammar**: `show [<column> <enable|disable>] [all|none] [| format <tree|config>] [| compare [rollback <N>]]`. Pipes are per-invocation and stackable.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - editor architecture
  → Constraint: editor uses bubbletea Model, commands return commandResult
  → Decision: tree is canonical in-memory representation, serializers produce display strings
- [ ] `docs/architecture/config/syntax.md` - config formats
  → Constraint: three formats (hierarchical, set, set+meta), auto-detected by DetectFormat

### RFC Summaries (MUST for protocol work)
N/A - not protocol work.

**Key insights:**
- Editor returns `commandResult` with either `output` (plain text) or `configView` (viewportData with original for diff)
- `configViewAtPath` provides diff gutter via `setViewportData` when `hasOriginal` is true
- Blame serializer already implements the gutter pattern: fixed-width left margin + hierarchical tree content
- MetaEntry has User, Time, Session, Previous, Value fields
- EditSession has User, Origin, ID (compound `user@origin:unixtime`), StartTime
- Metadata is stored per-leaf in MetaTree, which mirrors Tree structure

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/model_commands.go` - show command dispatch (lines 235-260, 668-738)
  → Constraint: `cmdShow` dispatches to `cmdShowBlame`, `cmdShowChanges`, `cmdShowSet`, or default configView
- [ ] `internal/component/config/serialize_blame.go` - blame serializer (519 lines)
  → Constraint: fixed-width 29-char gutter (user 14 + date 5 + time 5 + marker 1 + spacing)
  → Decision: opening brace inherits first child entry, closing brace inherits last child entry
- [ ] `internal/component/config/serialize_set.go` - set+meta serializer
  → Constraint: DONE -- writeMetaPrefix now emits `#user @source %time ^previous` tokens
- [ ] `internal/component/config/meta.go` - MetaEntry struct, MetaTree navigation
  → Constraint: DONE -- MetaEntry has User, Source, Time fields. SessionKey() = `user@source%RFC3339time`
- [ ] `internal/component/cli/editor_session.go` - EditSession
  → Constraint: DONE -- ID format `user@origin%RFC3339time`. UserAtOrigin() is dead code.
- [ ] `internal/component/cli/editor_draft.go` - writeThroughSet, writeThroughDelete
  → Constraint: DONE -- builds MetaEntry with User=session.User, Source=session.Origin, Time=session.StartTime
- [ ] `internal/component/cli/editor.go` - WorkingContent, ContentAtPath, BlameView, SetView
  → Constraint: WorkingContent still returns SerializeSetWithMeta when session active (to change for show restructure)
- [ ] `internal/component/cli/model_render.go` - setViewportData, configViewAtPath
  → Constraint: diff gutter applied when hasOriginal && original != content (unchanged)
- [ ] `internal/component/config/setparser.go` - extractMeta parses `#user @source %time ^prev`
  → Constraint: DONE -- `#` stores User, `@` stores Source, `%` parses Time

**Behavior to preserve:**
- `show changes` and `show changes all` - display pending session changes (list format)
- `show blame` abbreviated gutter format (14-char user, date, time, marker)
- Diff gutter logic in `setViewportData` (annotateContentWithTreeDiff, annotateContentWithGutter)
- `^previous` in metadata for tracking what changed
- Draft file format (set+meta) for persistence and concurrent editing
- MetaTree concurrent session support (multiple entries per leaf from different sessions)
- Tab completion for show subcommands

**Behavior to change:**
- `show` default: currently set+meta when session active, change to hierarchical tree always
- `show set`: replaced by `show | format config`
- `show save`: gone (use `show author enable` + `show source enable` + `show date enable` + `show | format config`)
- ~~Metadata format: `#user@origin` becomes `#user`, `%user@origin:unixtime` becomes `%origin`~~ DONE: `#user @source %time`
- Display columns independently togglable via DB preferences
- Show accepts pipe operators `| format` and `| compare`

## Data Flow (MANDATORY)

### Entry Point
- User types `show [args] [| pipes]` in the editor prompt
- Editor dispatch in `model_commands.go:dispatchCommand`

### Transformation Path
1. Command tokenized, pipe index found if present
2. `cmdShow` parses arguments: column toggles, or display request
3. For column toggles: read/write `/meta/show/<column>` in blob DB, return status message
4. For display: determine enabled columns from DB, apply per-invocation pipe overrides
5. Select serializer based on format (tree or config) and enabled columns
6. If `changes` column enabled or `| compare` present: wrap in configView with original/rollback for diff
7. Return commandResult with output or configView

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor ↔ DB | blob storage read/write for `/meta/show/*` preferences | [ ] |
| Editor ↔ Serializer | tree + meta + schema + column flags → display string | [ ] |
| Editor ↔ Render | configView with hasOriginal triggers diff annotation | [ ] |

### Integration Points
- `config.SerializeBlame` - existing blame gutter pattern to extend
- `config.Serialize` - existing bare hierarchical serializer
- `config.SerializeSet` - existing bare set serializer
- `config.SerializeSetWithMeta` - existing set+meta serializer (for `| format config` with all columns)
- `storage.Storage` - blob DB for reading/writing preferences
- `model_render.go:setViewportData` - diff annotation layer
- `model_load.go:dispatchWithPipe` - pipe dispatch

### Architectural Verification
- [ ] No bypassed layers (show goes through standard command dispatch)
- [ ] No unintended coupling (column preferences use existing blob DB)
- [ ] No duplicated functionality (extends existing blame gutter pattern, not a new serializer from scratch)
- [ ] Zero-copy preserved where applicable (serializers write to strings.Builder)

## Design

### Metadata Format Change

~~Current format (save/draft):~~

~~| Prefix | Current meaning | Current example |~~
~~|--------|----------------|-----------------|~~
~~| `#` | user@origin (compound) | `#thomas@local` |~~
~~| `@` | change timestamp | `@2026-03-18T23:52:58Z` |~~
~~| `%` | session ID (compound) | `%thomas@local:1773877970` |~~
~~| `^` | previous value | `^oldvalue` |~~

Implemented format (commit `e856ff2a`):

| Prefix | Meaning | Example |
|--------|---------|---------|
| `#` | username only | `#thomas` |
| `@` | connection source (host/origin) | `@local` or `@192.168.1.5` |
| `%` | session start time (ISO 8601) | `%2026-03-18T23:00:00Z` |
| `^` | previous value | `^oldvalue` |

Save format line example:

`#thomas @local %2026-03-18T23:00:00Z set bgp peer 127.0.0.4 peer-as 3245`

Session key derived from all three fields: `user@source%RFC3339time` (e.g., `thomas@local%2026-03-18T23:00:00Z`). Used internally for grouping concurrent edits. `%` separator chosen to avoid conflict with netcapstring which uses `:`.

### Display Columns

Four independently togglable columns, each stored in blob DB:

| Column | DB Key | Default | Width | Content |
|--------|--------|---------|-------|---------|
| author | `/meta/show/author` | disable | 14 chars (padded) | Username from `#` field |
| date | `/meta/show/date` | disable | 11 chars (`MM-DD HH:MM`) | Formatted from `%` field (session start time) |
| source | `/meta/show/source` | disable | variable | Origin from `@` field |
| changes | `/meta/show/changes` | disable | 1 char | `+`/`-`/`*` diff marker |

Column order is always: author, date, source, changes, then config content.

Only enabled columns appear. Disabled columns leave no gap.

Lines without metadata get blank padding (spaces) to maintain alignment.

Container opening `{` inherits first child entry metadata. Container closing `}` inherits last child entry metadata.

### Display Examples

All columns disabled (default):

| Output |
|--------|
| `bgp {` |
| `  peer 127.0.0.1 {` |
| `    local-as 1234` |
| `    peer-as 3245` |
| `  }` |
| `}` |

All columns enabled (`show all`):

| Output |
|--------|
| `  thomas        03-18 23:52  local  +  bgp {` |
| `  thomas        03-18 23:52  local  +    peer 127.0.0.1 {` |
| `  thomas        03-18 23:52  local  +      local-as 1234` |
| `  thomas        03-18 23:53  local  +      peer-as 3245` |
| `  thomas        03-18 23:52  local  +    }` |
| `  thomas        03-18 23:52  local  +  }` |

Only author + changes:

| Output |
|--------|
| `  thomas        +  bgp {` |
| `  thomas        +    peer 127.0.0.1 {` |
| `  thomas        +      local-as 1234` |
| `  thomas        +      peer-as 3245` |
| `  thomas        +    }` |
| `  thomas        +  }` |

### Show Command Grammar

`show [confirmed|saved|edit] [<column> <enable|disable>] [all|none] [| format <tree|config>] [| compare <committed|saved|rollback N|user>]`

#### Source (which version to display)

| Source | Meaning | Default |
|--------|---------|---------|
| `edit` | Current in-memory working config (what you are editing) | Yes |
| `saved` | Saved draft file on disk (last draft save) | No |
| `confirmed` | Committed config (what daemon runs) | No |

`show` = `show edit`. The source selects which version of the config to display in the viewport. Metadata columns and format pipes apply to whichever source is selected.

#### Column toggles (sticky)

| Syntax | Effect | Sticky |
|--------|--------|--------|
| `show <column> enable` | Enable a display column | Yes (DB) |
| `show <column> disable` | Disable a display column | Yes (DB) |
| `show all` | Enable all four columns | Yes (DB) |
| `show none` | Disable all four columns | Yes (DB) |

#### Pipes (per-invocation, stackable)

| Syntax | Effect |
|--------|--------|
| `\| format tree` | Display as hierarchical tree (default) |
| `\| format config` | Display as flat set commands |
| `\| compare committed` | Diff against committed config |
| `\| compare saved` | Diff against saved draft file |
| `\| compare rollback <N>` | Diff against rollback N from backup history |
| `\| compare <username>` | Diff against a specific user's pending changes |

Pipes are stackable: `show | compare rollback 2 | format config`

Column toggle commands are configuration commands (modify DB, show status message). Display and source commands show config in viewport.

#### Examples

| Command | What it displays |
|---------|-----------------|
| `show` | Working config, tree format, enabled columns |
| `show confirmed` | Committed config, tree format, enabled columns |
| `show saved` | Draft on disk, tree format, enabled columns |
| `show confirmed \| format config` | Committed config as set commands |
| `show \| compare thomas` | Working config with diff markers vs thomas's changes |
| `show saved \| compare committed` | Draft on disk with diff markers vs committed config |

Note: filesystem changes for `confirmed`/`saved` sources are out of scope for this spec and will be addressed when the config file model changes. The grammar and dispatch are defined here; the actual file reading is deferred.

### Serialization Matrix

For each combination of enabled columns and format, select the serializer:

| Columns enabled | format=tree | format=config |
|----------------|-------------|---------------|
| none | `Serialize` (existing) | `SerializeSet` (existing) |
| any combination | `SerializeAnnotatedTree` (new) | `SerializeAnnotatedSet` (new) |

The two new serializers accept a column flags parameter to control which metadata columns appear.

### Changes Column and Compare

The `changes` column shows diff markers (`+`/`-`/`*`). The diff baseline depends on the `| compare` pipe:

| Compare target | Baseline | Use case |
|---------------|----------|----------|
| (none, default) | committed config | What changed vs running daemon config |
| `committed` | committed config (`.conf` file) | Explicit: all changes including other sessions |
| `saved` | saved draft file (`.draft` file) | What changed since last draft save |
| `rollback <N>` | rollback N from backup history | What changed vs a historical version |
| `<username>` | another user's pending changes | See what a specific user changed |

When `changes` is disabled AND no `| compare` pipe is present, no diff annotation occurs. When `| compare` is used, it implicitly enables diff markers for that invocation regardless of the sticky `changes` setting.

### MetaEntry Struct Change (DONE)

~~Current fields: User, Time, Session, Previous, Value~~

Implemented fields: User (username only), Source (origin), Time (session start), Previous, Value

The Session field is removed. `SessionKey()` method derives the grouping key as `user@source%RFC3339time`. Active session entries have Source set; committed entries have User+Time but no Source. Detection of active vs committed uses `Source != ""`.

Added `RemoveEntry(name)` method to MetaTree for unconditional leaf metadata removal (used by commit delete path).

### EditSession Change (DONE)

~~Current: User, Origin, ID (`user@origin:unixtime`), StartTime~~

Implemented: User, Origin, ID (`user@origin%RFC3339time`), StartTime

MetaEntry built from EditSession:
- MetaEntry.User = EditSession.User
- MetaEntry.Source = EditSession.Origin
- MetaEntry.Time = EditSession.StartTime (not time.Now())

### Gutter Strategy

The new `SerializeAnnotatedTree` reuses the blame serializer's tree-walking logic with a configurable gutter. Instead of the fixed 29-char blame gutter, it writes only the enabled columns:

| Column | Field | Gutter segment |
|--------|-------|---------------|
| author | `#` User | username left-padded to 14 chars + 2 spaces |
| date | `%` Time | `MM-DD HH:MM` (11 chars) + 2 spaces |
| source | `@` Source | origin string + 2 spaces |
| changes | (computed) | marker char + 2 spaces |

Each segment is only emitted when its column is enabled. The tree-walking logic (container/list/leaf dispatch) is shared.

### Backward Compatibility of Save Format

The save format changed from `#user@origin @ISO8601 %user@origin:unixtime` to `#user @source %ISO8601`. Legacy migration is NOT yet implemented. Existing draft files with the old format will:
- `#user@origin` loads into User as `user@origin` (compound, not split)
- Old `@ISO8601` loads into Source (now means source, not time)
- Old `%session-id` loads into Time parse (will fail, ignored)

This means old drafts will not round-trip correctly. Legacy migration needs to detect old format and split fields. Deferred to implementation phase.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show` command in editor prompt | → | `cmdShow` dispatches to annotated serializer | `test/parse/show-default-tree.ci` |
| `show author enable` in editor | → | writes `/meta/show/author` to blob DB | `test/parse/show-column-toggle.ci` |
| `show all` in editor | → | writes all four DB keys | `test/parse/show-all-none.ci` |
| `show \| format config` in editor | → | `SerializeAnnotatedSet` with enabled columns | `test/parse/show-format-config.ci` |
| `show \| compare committed` in editor | → | diff against committed config | `test/parse/show-compare-committed.ci` |
| `show \| compare saved` in editor | → | diff against saved draft | `test/parse/show-compare-saved.ci` |
| `show \| compare rollback 1` in editor | → | diff against rollback baseline | `test/parse/show-compare-rollback.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show` with no columns enabled, session active | Displays bare hierarchical tree (no metadata, no set+meta format) |
| AC-2 | `show author enable` | Writes `enable` to `/meta/show/author` in blob DB, confirms with status message |
| AC-3 | `show` with author column enabled, session active | Each line prepended with 14-char padded username from metadata |
| AC-4 | `show all` | All four columns enabled in DB, display shows all metadata columns |
| AC-5 | `show none` | All four columns disabled in DB, display shows bare tree |
| AC-6 | `show \| format config` | Display uses flat set-command format instead of tree |
| AC-7 | `show \| format config` with author enabled | Each set line prepended with username |
| AC-8 | `show \| compare committed` | Diff markers shown against committed config |
| AC-9 | `show \| compare saved` | Diff markers shown against saved draft file |
| AC-10a | `show \| compare rollback 1` | Diff markers shown against rollback 1 content |
| AC-10b | `show \| compare rollback 2 \| format config` | Pipes stack: set format + diff against rollback 2 |
| AC-11 | Save format uses `#user @source %time` | Draft file written with new metadata prefix format (DONE: commit `e856ff2a`) |
| AC-12 | Load old format `#user@origin @time %user@origin:unixtime` | Parser extracts user, time, source correctly from legacy format (NOT DONE: legacy migration deferred) |
| AC-13 | Lines without metadata | Blank padding maintains column alignment |
| AC-14 | Container `{` and `}` lines | Opening inherits first child metadata, closing inherits last child metadata |
| AC-15 | `show` respects `edit` context path | Display scoped to current edit location |
| AC-16 | `show changes enable` then `show` | Diff markers (+/-/*) shown comparing against committed config by default |
| AC-17 | `show date enable` | Each line shows `MM-DD HH:MM` formatted session start time from `%` field |
| AC-18 | `show source enable` | Each line shows origin (e.g., `local`, `192.168.1.5`) from `@` field |
| AC-19 | `show \| compare` implicitly shows changes | Diff markers appear even if `changes` column is disabled |
| AC-20 | `show confirmed` | Displays the committed config (deferred: depends on FS changes) |
| AC-21 | `show saved` | Displays the saved draft file (deferred: depends on FS changes) |
| AC-22 | `show` (no source) | Equivalent to `show edit` -- displays current working config |
| AC-23 | `show \| compare <username>` | Diff markers show changes made by the specified user |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSerializeAnnotatedTree` | `internal/component/config/serialize_annotated_test.go` | Tree format with various column combinations | |
| `TestSerializeAnnotatedSet` | `internal/component/config/serialize_annotated_test.go` | Set format with various column combinations | |
| `TestSerializeAnnotatedTreeNoMeta` | `internal/component/config/serialize_annotated_test.go` | Tree with columns enabled but no metadata = blank padding | |
| `TestAnnotatedGutterWidth` | `internal/component/config/serialize_annotated_test.go` | Fixed-width columns maintain alignment across lines | |
| `TestAnnotatedContainerInheritance` | `internal/component/config/serialize_annotated_test.go` | Opening brace gets first child meta, closing gets last | |
| `TestMetaEntryNewFormat` | `internal/component/config/meta_test.go` | MetaEntry with User (plain), Source (origin), no Session | |
| `TestExtractMetaNewFormat` | `internal/component/config/setparser_test.go` | Parser handles `#user @time %source` format | |
| `TestExtractMetaLegacyFormat` | `internal/component/config/setparser_test.go` | Parser handles old `#user@origin @time %user@origin:unixtime` format | |
| `TestWriteMetaPrefixNewFormat` | `internal/component/config/serialize_set_test.go` | writeMetaPrefix emits `#user @time %source` | |
| `TestCmdShowDefault` | `internal/component/cli/model_commands_test.go` | `show` returns hierarchical tree, not set+meta | |
| `TestCmdShowColumnToggle` | `internal/component/cli/model_commands_test.go` | `show author enable` writes DB key | |
| `TestCmdShowAllNone` | `internal/component/cli/model_commands_test.go` | `show all` / `show none` toggle all columns | |
| `TestCmdShowPipeFormat` | `internal/component/cli/model_commands_test.go` | `show \| format config` uses set serializer | |
| `TestCmdShowPipeCompareCommitted` | `internal/component/cli/model_commands_test.go` | `show \| compare committed` uses committed config as diff baseline | |
| `TestCmdShowPipeCompareSaved` | `internal/component/cli/model_commands_test.go` | `show \| compare saved` uses saved draft as diff baseline | |
| `TestCmdShowPipeCompareRollback` | `internal/component/cli/model_commands_test.go` | `show \| compare rollback 1` uses rollback as diff baseline | |
| `TestCmdShowPipeStack` | `internal/component/cli/model_commands_test.go` | `show \| compare rollback 2 \| format config` stacks both | |
| `TestSessionMetaEntryFields` | `internal/component/cli/editor_draft_test.go` | writeThroughSet creates MetaEntry with User (plain), Source (origin) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rollback N | 1 to backup count | last backup index | 0 | backup count + 1 |
| author width | 0-14 padded | 14 char username | N/A | 15+ char truncated to 14 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-default` | `test/parse/show-default-tree.ci` | Config loaded, `show` displays hierarchical tree | |
| `test-show-column-toggle` | `test/parse/show-column-toggle.ci` | `show author enable` then `show` includes author column | |
| `test-show-all-none` | `test/parse/show-all-none.ci` | `show all` then `show` includes all columns, `show none` removes them | |
| `test-show-format-config` | `test/parse/show-format-config.ci` | `show \| format config` displays set commands | |
| `test-show-compare-committed` | `test/parse/show-compare-committed.ci` | `show \| compare committed` shows diff markers vs committed | |
| `test-show-compare-saved` | `test/parse/show-compare-saved.ci` | `show \| compare saved` shows diff markers vs draft | |
| `test-show-compare-rollback` | `test/parse/show-compare-rollback.ci` | `show \| compare rollback 1` shows diff markers vs rollback | |

### Future (if deferring any tests)
- Property tests for gutter width consistency across all node types (non-essential, correctness covered by unit tests)

## Files to Modify
- `internal/component/config/meta.go` - Replace Session field with Source field in MetaEntry
- `internal/component/config/serialize_set.go` - Update writeMetaPrefix for new format (`#user @time %source`)
- `internal/component/config/serialize_blame.go` - Refactor to share tree-walking with annotated serializer
- `internal/component/config/setparser.go` - Update extractMeta to handle new format + legacy migration
- `internal/component/cli/model_commands.go` - Refactor cmdShow for new grammar
- `internal/component/cli/model_load.go` - Update dispatchWithPipe for show pipe support
- `internal/component/cli/editor.go` - WorkingContent no longer returns set+meta for display; add column preference methods
- `internal/component/cli/editor_session.go` - Remove UserAtOrigin, update NewEditSession
- `internal/component/cli/editor_draft.go` - Build MetaEntry with User (plain) and Source (origin)
- `internal/component/cli/editor_commit.go` - Update metadata handling for new MetaEntry fields
- `internal/component/cli/model.go` - Add display preferences state (cached from DB)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | N/A |
| RPC count in architecture docs | [ ] No | N/A |
| CLI commands/flags | [ ] No | N/A (editor command, not CLI subcommand) |
| CLI usage/help text | [x] Yes | `model_render.go` help overlay |
| API commands doc | [ ] No | N/A |
| Plugin SDK docs | [ ] No | N/A |
| Editor autocomplete | [x] Yes | completer needs `show` subcommand updates |
| Functional test for new RPC/API | [ ] No | N/A |

## Files to Create
- `internal/component/config/serialize_annotated.go` - New annotated serializer (tree + set) with column flags
- `internal/component/config/serialize_annotated_test.go` - Tests for annotated serializer
- `test/parse/show-default-tree.ci` - Functional test: show displays tree format
- `test/parse/show-column-toggle.ci` - Functional test: column toggle
- `test/parse/show-all-none.ci` - Functional test: show all / show none
- `test/parse/show-format-config.ci` - Functional test: pipe format config
- `test/parse/show-compare-committed.ci` - Functional test: pipe compare committed
- `test/parse/show-compare-saved.ci` - Functional test: pipe compare saved
- `test/parse/show-compare-rollback.ci` - Functional test: pipe compare rollback

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Metadata format** -- Change MetaEntry, writeMetaPrefix, extractMeta, editor_draft
   - Tests: `TestMetaEntryNewFormat`, `TestExtractMetaNewFormat`, `TestExtractMetaLegacyFormat`, `TestWriteMetaPrefixNewFormat`, `TestSessionMetaEntryFields`
   - Files: `meta.go`, `serialize_set.go`, `setparser.go`, `editor_session.go`, `editor_draft.go`, `editor_commit.go`
   - Verify: tests fail → implement → tests pass
   - Note: existing tests will break due to MetaEntry field rename. Update test expectations.

2. **Phase: Annotated serializer** -- New serialize_annotated.go with column-aware gutter
   - Tests: `TestSerializeAnnotatedTree`, `TestSerializeAnnotatedSet`, `TestSerializeAnnotatedTreeNoMeta`, `TestAnnotatedGutterWidth`, `TestAnnotatedContainerInheritance`
   - Files: `serialize_annotated.go` (new), `serialize_blame.go` (refactor to share tree walk)
   - Verify: tests fail → implement → tests pass

3. **Phase: Show command restructure** -- New cmdShow grammar, column toggles, pipe support
   - Tests: `TestCmdShowDefault`, `TestCmdShowColumnToggle`, `TestCmdShowAllNone`, `TestCmdShowPipeFormat`, `TestCmdShowPipeCompare`, `TestCmdShowPipeStack`
   - Files: `model_commands.go`, `model_load.go`, `editor.go`, `model.go`
   - Verify: tests fail → implement → tests pass

4. **Phase: DB integration** -- Read/write column preferences from blob storage
   - Tests: column toggle persists across editor restart (functional test)
   - Files: `editor.go`, `model.go`
   - Verify: tests fail → implement → tests pass

5. **Phase: Help and completion** -- Update help overlay, tab completion for show subcommands
   - Files: `model_render.go`, completer files
   - Verify: manual check + existing completion tests

6. **Functional tests** → Create .ci tests covering user-visible behavior.

7. **Full verification** → `make ze-verify`

8. **Complete spec** → Fill audit tables, write learned summary to `plan/learned/`, delete spec.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 17 ACs have implementation with file:line |
| Correctness | Column widths are consistent, gutter alignment maintained |
| Naming | DB keys use `/meta/show/<column>` format |
| Data flow | Show reads DB preferences, passes column flags to serializer |
| Rule: no-layering | Old show set/show save/show blame dispatch fully replaced |
| Rule: compatibility | Legacy metadata format in existing draft files still parseable |
| Rule: buffer-first | Serializers write to strings.Builder (not encoding code, but follow pattern) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `serialize_annotated.go` exists | `ls internal/component/config/serialize_annotated.go` |
| `show` displays tree by default | `test/parse/show-default-tree.ci` passes |
| Column toggles write to DB | `test/parse/show-column-toggle.ci` passes |
| Legacy metadata format loads | `TestExtractMetaLegacyFormat` passes |
| New metadata format round-trips | `TestWriteMetaPrefixNewFormat` + `TestExtractMetaNewFormat` pass |
| Help text updated | grep `format` in help overlay |
| All `make ze-verify` passes | test output |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `show <column>` rejects unknown column names |
| Input validation | `show \| format <x>` rejects unknown format names |
| Input validation | `show \| compare rollback <N>` validates N is a positive integer within backup range |
| DB key injection | Column names are validated against a fixed set, not used as raw DB key segments |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A -- not protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-23 all demonstrated (AC-20, AC-21 deferred pending FS changes)
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
