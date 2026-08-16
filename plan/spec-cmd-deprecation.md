# Spec: cmd-deprecation

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-21 |

STALE BASELINE -- corrected in-body (2026-07-22 plan review; Migration
table corrected 2026-07-22): all three originally cited ad-hoc deprecation
sites are GONE (`show.go` and `clear.go` carry no deprecation pattern;
`cache.go`'s `dispatchCacheByID` no longer takes a `deprecated` bool -- now
`dispatchCacheByID(ctx, action, idStr, extraArgs)`, `cache.go`). The only
surviving ad-hoc site is the local `withDeprecation` closure at
`internal/component/bgp/plugins/cmd/commit/commit.go`. The Migration
table below is now corrected: the vanished show/clear/cache rows are struck
through and a row for the `commit.go` closure is added. Other sections
(Current Behavior, Files to Modify, AC-9, TDD plan, Deliverables) still name
the vanished sites; retarget them to `commit.go` during implementation.
The framework itself (`internal/component/command/deprecation.go`,
`Response.Deprecated`, `LookupLocalMeta`) has not landed, so the spec's goal
stands.

**Scope:** Command deprecation only. Config migration integration is a future child spec (`spec-deprecation-config-migration`). The shared `Deprecation` type is designed here to be reusable, but config-specific concerns (schema stamp format, migration rejection semantics, config-specific replacement shape) are out of scope.

## DECISION (Thomas, 2026-07-22): migrations are permanent; old forms migrate transparently

Two rulings that supersede parts of the design below:

1. **An up-version migration is never removed.** Once a deprecation entry
   (the structured old-form -> new-form Replacement mapping) is added, it is
   permanent. No phase of the lifecycle ever deletes the mapping, the
   registration, or the old-grammar detection code. The "can be deleted
   entirely after RemoveAt + one release cycle" clauses in the Removal
   Semantics section are struck below.
2. **Post-release, migration from the initial version to the latest is
   transparent.** A config or command form valid in the first release must
   keep working on every later binary: the old form is accepted and
   transparently rewritten to the current form via the Replacement mapping
   (with a deprecation warning), not rejected. The structured Replacement
   format is therefore load-bearing: it exists to power automatic rewriting,
   not just a helpful error message.

Design consequences to rework before this spec leaves `design`:
- `RemoveAt` as "the command is treated as removed" contradicts ruling 2.
  Either drop `RemoveAt`, or redefine it as the date the warning escalates
  in visibility -- never as execution ceasing while the mapping could
  rewrite it. The state table's `Removed` row, AC-2, AC-6, AC-12, and the
  removed-command tests must be redesigned around transparent rewrite.
- The already-shipped `withDeprecation` closure
  (`internal/component/bgp/plugins/cmd/commit/commit.go`) is the
  model: the legacy `commit <name> <action>` grammar still executes,
  re-routed to the current dispatch with a warning attached.
- This ruling equally constrains the future config child spec
  (`spec-deprecation-config-migration`) and matches
  `ai/rules/config.md`: "No version numbers in config. Design for
  machine-transformable migration."

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/internal/cmdregistry/registry.go` - local command registration
4. `internal/component/command/node.go` - command tree node type
5. `internal/component/cmd/show/show.go` - existing ad-hoc deprecation

## Task

Introduce a systematic command deprecation lifecycle across all dispatch paths (shell one-shot, daemon RPC, interactive CLI). Currently deprecation is ad-hoc: individual handlers manually inject `"deprecated": "use: <newForm>"` into response JSON data. This mixes deprecation metadata with command output, has no dates, no structured replacement format, and no shell-side rendering.

The new system provides:
- A single deprecation type shared across both dispatch paths
- Date-driven lifecycle (warning phase, ~~then removal~~ never removal -- see the 2026-07-22 DECISION: old forms keep executing via transparent rewrite) with no manual state management
- Structured replacement format (command keywords + argument mapping) parseable by automation
- Shell stderr output for one-shot invocations
- Interactive CLI feedback line for deprecation warnings
- Automatic state derivation from dates (no enum to maintain)

Design constraint: ze uses date-based versioning, so `Since` is simply the date the deprecation code is added (the previous release does not have it; the next one does). ~~`RemoveAt` is a date; any binary built after that date treats the command as removed.~~ (Superseded by the 2026-07-22 DECISION: no binary ever treats a mapped command as removed; the mapping rewrites it transparently. `RemoveAt`'s fate -- dropped, or redefined as a warning-escalation date -- is a design question to settle before `ready`.)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern
  → Constraint: all metadata attaches at registration site via init()
- [ ] `ai/patterns/registration.md` - init + registry + blank import
  → Constraint: Deprecation must follow same pattern
- [ ] `ai/patterns/cli-command.md` - CLI command structure
  → Constraint: deprecation must work with both static dispatch and registry dispatch
- [ ] `ai/rules/evidence.md` - derive from registry
  → Constraint: deprecation status must be derivable from the registry, not hardcoded in help text

**Key insights:**
- Registration is the unifying pattern; deprecation metadata belongs at the registration site
- Two dispatch paths: shell (cmdregistry + static switch) and daemon (command.Node tree)
- Interactive CLI has a feedback line (Line 1 of message area) for command results

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/internal/cmdregistry/registry.go` - Meta struct with Description/Mode/Subs; LookupLocal does longest-prefix match
  → Constraint: Meta has no deprecation fields; adding one must not break existing registrations
- [ ] `internal/component/command/node.go` - Node struct for command tree; no deprecation field
- [ ] `internal/component/cmd/show/show.go` - `withDeprecation()` injects `"deprecated": "use: <newForm>"` into response Data map
- [ ] `internal/component/iface/cmd/clear.go` - same pattern, sets `deprecated` bool then injects into response
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go` - same pattern via `dispatchCacheByID` with deprecated flag
- [ ] `cmd/ze/main.go` - shell dispatch: YANG verbs to cmdutil.RunCommand, static switch, then cmdregistry.LookupLocal fallback
- [ ] `internal/component/cli/model_render.go` - feedbackLine() renders status messages with styles (success, error, warn, welcome)
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - RunCommand for YANG verbs; does not extract or render "deprecated" from responses

**Behavior to preserve:**
- Deprecated commands still execute during the warning phase
- Existing help output format is unchanged for non-deprecated commands

**Pre-release note:** Ze has not released yet. No backward compatibility constraints on the `"deprecated"` key in Response.Data or on the Response JSON envelope. The old ad-hoc pattern can be replaced cleanly without a transition period.

**Behavior to change:**
- Replace ad-hoc `"deprecated": "use: <newForm>"` string injection with structured deprecation metadata
- Add stderr deprecation output for shell invocations (currently silent)
- Add feedback line deprecation display for interactive CLI (currently not shown)
- After RemoveAt date: command returns an error instead of executing

## Data Flow (MANDATORY)

### Two Levels of Deprecation

Deprecation operates at two levels, both using the same `Deprecation` type:

| Level | Where it lives | Who decides | Use case |
|-------|---------------|-------------|----------|
| Registration-level | `cmdregistry.Meta`, `command.Node` | Set at init() | Entire command path deprecated (e.g., `ze run`) |
| Handler-level | `plugin.Response.Deprecated` field | Handler decides at runtime | Grammar variant deprecated (e.g., `show interface <name>` vs `show interface detail <name>`) |

Registration-level deprecation is checked BEFORE the handler runs. If Removed, the handler is never invoked. Handler-level deprecation is returned in the response; the handler ran and chose to flag the grammar used.

The existing deprecation sites (show interface, clear interface, cache) are handler-level: the same handler receives both old and new grammar and decides which was used. These stay handler-level but migrate from ad-hoc string injection (`data["deprecated"] = "use: ..."`) to the structured `Response.Deprecated` field.

### Entry Point
- Registration-level: set at command registration time (init())
- Handler-level: set by handler in `Response.Deprecated` when it detects old grammar
- State is derived at check time from current date vs Since/RemoveAt dates

### Transformation Path
1. Registration: handler registers with optional Deprecation pointer on Meta (cmdregistry) or Node (command tree)
2. Dispatch (registration check): before invoking handler, dispatch layer checks Meta/Node Deprecation. If Removed, emit error and stop. If Warning, emit warning and continue to handler.
3. Handler execution: handler runs. If it detects deprecated grammar, it sets `Response.Deprecated` to a Deprecation struct.
4. Dispatch (response check): after handler returns, dispatch layer checks `Response.Deprecated`. If set, emit warning to stderr (shell) or feedback line (interactive CLI).
5. Rendering: the dispatch layer renders the deprecation info. The response Data is rendered normally, without any `"deprecated"` key mixed in.

### Daemon RPC Path

The daemon dispatch path goes through `RPCRegistration` (handler dispatch) and `command.Node` (help/completion). These serve different purposes:

| Struct | Used for | Gets Deprecation? |
|--------|---------|-------------------|
| `command.Node` | Help output, completion, command tree navigation | Yes: shows `[deprecated]` in help, hides removed commands |
| `RPCRegistration` | Handler dispatch via WireMethod | No: RPC is the transport, not the command identity |
| `plugin.Response` | Handler return value | Yes: new `Deprecated` field for handler-level deprecation |

The handler decides deprecation at runtime (it knows which grammar the user typed). It returns deprecation info in `Response.Deprecated`. The CLI/shell rendering layer extracts it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Registration -> Dispatch | Deprecation pointer on Meta/Node, checked before handler | [ ] |
| Handler -> Response | Handler sets Response.Deprecated when it detects old grammar | [ ] |
| Response -> Rendering | Dispatch layer extracts Response.Deprecated, renders to stderr/feedback | [ ] |

### Integration Points
- `cmdregistry.Meta` - add Deprecation pointer field (registration-level)
- `command.Node` - add Deprecation pointer field (help/completion)
- `plugin.Response` - add Deprecated pointer field (handler-level, replaces ad-hoc Data injection)
- `cmdregistry.LookupLocalMeta` - new function returning (handler, args, meta); LookupLocal unchanged for backward compat
- `cmd/ze/main.go` dispatch - check registration-level deprecation at registry fallback; check response deprecation after handler
- `cmdutil.RunCommand` - check registration-level deprecation from local handler match; daemon path gets response deprecation via CLI client
- `cli/model_render.go` feedbackLine() - render deprecation warnings from response
- `show/show.go`, `clear.go`, `cache.go` - migrate from ad-hoc Data injection to Response.Deprecated field

### LookupLocal Callers (ISSUE-1)

Two callers exist (verified via LSP findReferences):

| Caller | File | Line | Impact |
|--------|------|------|--------|
| `matchLocalHandler` | `cmd/ze/internal/cmdutil/cmdutil.go` | 38 | Needs Meta to check registration-level deprecation |
| Direct call | `cmd/ze/main.go` | 567 | Needs Meta to check registration-level deprecation |

Solution: add `LookupLocalMeta(words) (handler, args, *Meta)` as a new function. Keep `LookupLocal` unchanged (returns handler, args only) so no forced migration of callers that don't need Meta. Both callers above migrate to `LookupLocalMeta`.

### Static Dispatch Commands (ISSUE-2)

Commands in the `switch arg` block of main.go (`bgp`, `config`, `cli`, etc.) bypass cmdregistry and cannot carry registration-level deprecation. These commands are not currently deprecated. When one needs deprecation, it should be migrated to cmdregistry first (register a local handler that calls the existing Run function). The existing `ze run` removal (main.go) is a hardcoded message that predates this system; it can be migrated as a follow-up.

### Architectural Verification
- [ ] No bypassed layers (deprecation flows through dispatch or response, not patched into Data)
- [ ] No unintended coupling (deprecation type is in a shared package, not cmd-specific)
- [ ] No duplicated functionality (replaces withDeprecation, does not layer on top)
- [ ] Zero-copy preserved where applicable (Deprecation is a pointer, not copied)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `cmdregistry.RegisterLocalMeta` with Deprecation | -> | `LookupLocalMeta` returns deprecation info | `TestLookupLocalMetaReturnsDeprecation` |
| Shell dispatch of deprecated command | -> | stderr deprecation output | `TestShellDeprecatedCommandStderr` |
| Shell dispatch of removed command | -> | stderr error + no execution | `TestShellRemovedCommandNoExecution` |
| Daemon handler sets Response.Deprecated | -> | Response carries structured deprecation | `TestResponseDeprecationSerialized` |
| Interactive CLI deprecated command | -> | feedback line shows warning | `TestCLIDeprecatedFeedback` |
| Handler sets Response.Deprecated (warning phase) | -> | dispatch layer renders warning | `TestResponseDeprecationRendered` |
| Handler detects old grammar past RemoveAt | -> | error response, no execution | `TestHandlerRemovedGrammarRejectsExecution` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Command registered with Deprecation (Since in past, RemoveAt in future) | Command executes normally; deprecation warning emitted |
| AC-2 | Command registered with Deprecation (RemoveAt in past) | Command does not execute; error with replacement info emitted |
| AC-3 | Command registered without Deprecation | No deprecation output; identical to current behavior |
| AC-4 | Deprecated command invoked from shell | Structured deprecation line written to stderr (separate from stdout output) |
| AC-5 | Deprecated command invoked from interactive CLI | Deprecation warning shown in feedback line (Line 1) |
| AC-6 | Removed command invoked from shell | Stderr shows removal message with replacement; exit code non-zero |
| AC-7 | Deprecated command with Replacement | Replacement rendered as concrete command with args substituted from invocation |
| AC-8 | Deprecated command without Replacement (message only) | Warning shows message; no replacement suggestion |
| AC-9 | Existing deprecation sites (show interface, clear interface, cache) | Migrated to Response.Deprecated; `withDeprecation` deleted |
| AC-10 | `ze help` or `ze show help` for deprecated command | Help output includes deprecation status and replacement |
| AC-11 | Handler detects old grammar and sets Response.Deprecated | Deprecation info in Response.Deprecated, NOT in Response.Data |
| AC-12 | Handler detects old grammar past RemoveAt | Handler returns error response with replacement info; old grammar not executed |

## Deprecation Type

### Fields

| Field | Type | Zero value meaning |
|-------|------|--------------------|
| Since | date (year, month, day) | Not set (should not happen in practice) |
| RemoveAt | date (year, month, day) | No planned removal (permanent deprecation warning) |
| Replacement | pointer to Replacement | No replacement available (message only) |
| Message | string | No additional explanation |

### Replacement Fields

| Field | Type | Purpose |
|-------|------|---------|
| Command | list of strings | New command keywords in order |
| Args | list of Arg | How old invocation arguments map to new command |

### Arg Fields

| Field | Type | Purpose |
|-------|------|---------|
| Position | integer | Position in the old command's argument list (0-based) |
| Name | string | Semantic label for documentation and machine parsing |

### State Derivation

`State()` takes a reference date as parameter. For command deprecation, the caller passes the current date (which maps to binary version via date-based versioning). The type is designed to accept any reference date, making it reusable for config migration in a future spec (where the reference would be the config's own version date).

| Condition | Derived State |
|-----------|---------------|
| Deprecation pointer is nil | Active |
| RemoveAt is zero or reference is before RemoveAt | Warning (deprecated) |
| Reference is on or after RemoveAt | Removed |

## Stderr Output Format (Shell)

One structured line to stderr, prefixed with `ze:` for filtering.

Warning phase format:

| Field | Format | Example |
|-------|--------|---------|
| prefix | `ze: deprecated` | `ze: deprecated` |
| since | `since=YYYY-MM-DD` | `since=2026-03-15` |
| remove | `remove=YYYY-MM-DD` (omitted if zero) | `remove=2026-09-01` |
| command | `command="<words>"` (omitted if no Replacement) | `command="show interface counters"` |
| args | `arg=<pos>:<name>` (repeated per arg; omitted if no Replacement) | `arg=0:name` |
| message | `message="<text>"` (omitted if empty) | `message="action before identifier"` |

Removed phase format:

| Field | Format | Example |
|-------|--------|---------|
| prefix | `ze: removed` | `ze: removed` |
| since | `since=YYYY-MM-DD` | `since=2026-03-15` |
| removed | `removed=YYYY-MM-DD` | `removed=2026-09-01` |
| command | same as warning | same as warning |
| args | same as warning | same as warning |
| message | same as warning | same as warning |

Full example: `ze: deprecated since=2026-03-15 remove=2026-09-01 command="show interface counters" arg=0:name`

## Interactive CLI Feedback

Deprecation warnings render in the feedback line (Line 1 of the message area) using `warnStyle`. Format: `deprecated: use "show interface counters <name>" (removal: 2026-09-01)`. The concrete replacement is computed by substituting actual argument values into the Replacement template.

## Help Output

Deprecated commands in help listings get a `[deprecated]` suffix after the description. Removed commands do not appear in help output.

When help is shown for a specific deprecated command, an additional line appears: `Deprecated since YYYY-MM-DD. Use: <replacement>. Removal: YYYY-MM-DD.`

## Migration of Existing Sites

All existing deprecation sites are handler-level: the same RPC handler receives both old and new grammar, detects which was used, and flags the response. They stay handler-level but migrate from ad-hoc `data["deprecated"] = "use: ..."` injection into `Response.Data` to setting `Response.Deprecated` with a structured Deprecation value.

| Current site | File | Current mechanism | Migration |
|--------------|------|-------------------|-----------|
| ~~show interface detail~~ | ~~`internal/component/cmd/show/show.go`~~ | ~~`withDeprecation(resp, "show interface detail "+args[0])`~~ | ~~Handler sets `resp.Deprecated` ...~~ (gone as of 2026-07-22 -- site removed; `show.go` carries no deprecation pattern) |
| ~~show interface counters~~ | ~~`internal/component/cmd/show/show.go`~~ | ~~`withDeprecation(resp, "show interface counters "+args[0])`~~ | ~~Handler sets `resp.Deprecated` ...~~ (gone as of 2026-07-22 -- site removed; `show.go` carries no deprecation pattern) |
| ~~clear interface counters~~ | ~~`internal/component/iface/cmd/clear.go`~~ | ~~`deprecated` bool + manual injection~~ | ~~Handler sets `resp.Deprecated` ...~~ (gone as of 2026-07-22 -- site removed; `clear.go` carries no deprecation pattern) |
| ~~cache actions~~ | ~~`internal/component/bgp/plugins/cmd/cache/cache.go`~~ | ~~`dispatchCacheByID(ctx, action, id, args, true)`~~ | ~~Handler sets `resp.Deprecated` ...~~ (gone as of 2026-07-22 -- `dispatchCacheByID(ctx, action, idStr, extraArgs)` at `cache.go` no longer takes a deprecated flag) |
| commit actions (legacy `commit <name> <action>` grammar) | `internal/component/bgp/plugins/cmd/commit/commit.go` | local `withDeprecation` closure at `commit.go` inside `dispatchCommitAction(ctx, action, name, extraArgs, deprecated bool)`; injects `data["deprecated"] = "use: commit <action> <name>"` when the legacy grammar was used | Handler sets `resp.Deprecated` with Replacement: command=`commit <action>`, arg 0=name; the closure and the `deprecated` bool parameter are deleted |

After migration: the local `withDeprecation` closure and the `deprecated` bool
parameter of `dispatchCommitAction` in `commit.go` are deleted. No transition
period needed (pre-release). ~~`withDeprecation()` in show.go is deleted. The
`deprecated` bool parameter in `dispatchCacheByID` is deleted.~~ (superseded
2026-07-22: those sites no longer exist.)

## Relationship to Config Migration (OUT OF SCOPE)

Config migration (`internal/component/config/migration/`) is a related but separate system. Both share the same lifecycle question: "is this old form still accepted?" Both can use the same `Deprecation` type. But config migration has its own concerns:

- The reference for config deprecation is the **config's own version** (schema stamp), not the binary's build date
- The schema stamp (`# ze-schema: 1`) currently uses integers; evolving to dates is a separate change
- Config rejection semantics differ (reject at load vs reject with "run `ze config migrate`")
- Config replacements describe path renames, not command grammar changes; the Replacement struct doesn't fit

These concerns belong in a child spec: `spec-deprecation-config-migration`. This spec designs the `Deprecation` type to be reusable (lives in `internal/component/command/`, a leaf package importable by both command dispatch and config migration). The child spec adds config-specific replacement types and schema stamp evolution.

### Handler Cleanup Lifecycle

~~Registration-level: after RemoveAt + one release cycle, the registration and handler code can be deleted entirely. The command path disappears from dispatch and help.~~

~~Handler-level: after RemoveAt for a grammar variant, the handler stops accepting the old grammar and returns an error. After one more release cycle, the old-grammar detection code can be deleted.~~

(Struck 2026-07-22 per the DECISION block at the top: an up-version migration
is never removed. The registration, the Replacement mapping, and the
old-grammar acceptance code are permanent; the old form keeps executing via
transparent rewrite with a warning. There is no cleanup phase.)

## Package Placement

The Deprecation type lives in `internal/component/command/` alongside Node. This package is already imported by both `cmd/ze/internal/cmdregistry/` and the CLI component. It is a leaf package with no dependencies on cmd or component internals.

The stderr formatter lives in `internal/component/command/` as well (it formats a Deprecation struct to a writer).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeprecationStateActive` | `internal/component/command/deprecation_test.go` | nil Deprecation returns Active | |
| `TestDeprecationStateWarning` | `internal/component/command/deprecation_test.go` | Since in past, RemoveAt in future returns Warning | |
| `TestDeprecationStateRemoved` | `internal/component/command/deprecation_test.go` | RemoveAt in past returns Removed | |
| `TestDeprecationStateNoRemoveAt` | `internal/component/command/deprecation_test.go` | RemoveAt zero, Since in past returns Warning (permanent) | |
| `TestDeprecationFormatStderr` | `internal/component/command/deprecation_test.go` | Formats structured stderr line correctly | |
| `TestDeprecationFormatStderrNoReplacement` | `internal/component/command/deprecation_test.go` | Message-only deprecation omits command/args fields | |
| `TestDeprecationFormatStderrRemoved` | `internal/component/command/deprecation_test.go` | Removed state uses "removed" prefix | |
| `TestReplacementRender` | `internal/component/command/deprecation_test.go` | Substitutes actual args into replacement template | |
| `TestLookupLocalMetaReturnsDeprecation` | `cmd/ze/internal/cmdregistry/registry_test.go` | LookupLocalMeta returns deprecation info from registration | |
| `TestResponseDeprecationSerialized` | `internal/component/plugin/types_test.go` | Response.Deprecated serializes correctly in JSON envelope | |
| `TestResponseDeprecationRendered` | `cmd/ze/internal/cmdutil/cmdutil_test.go` | Response.Deprecated rendered to stderr by dispatch layer | |
| `TestHandlerRemovedGrammarRejectsExecution` | `internal/component/cmd/show/show_test.go` | Handler with removed grammar returns error, not data | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Arg.Position | 0 to len(args)-1 | len(args)-1 | N/A (unsigned) | len(args) (skip gracefully) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-deprecated-command` | `test/cli/deprecated-command.ci` | User runs deprecated command from shell; sees warning on stderr, output on stdout | |
| `test-removed-command` | `test/cli/removed-command.ci` | User runs removed command; sees error on stderr, non-zero exit | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/command/node.go` - add Deprecation pointer field to Node
- `cmd/ze/internal/cmdregistry/registry.go` - add Deprecation pointer field to Meta; add LookupLocalMeta function
- `cmd/ze/main.go` - check deprecation at dispatch points (YANG verb, static switch, registry fallback)
- `internal/component/cmd/show/show.go` - migrate withDeprecation to structured Deprecation; delete withDeprecation
- `internal/component/iface/cmd/clear.go` - migrate deprecated bool to structured Deprecation
- `internal/component/bgp/plugins/cmd/cache/cache.go` - migrate deprecated parameter to structured Deprecation
- `internal/component/cli/model_commands.go` - extract deprecation from command response, route to feedback line
- `internal/component/cli/model_render.go` - render deprecation warnings in feedbackLine()
- `internal/component/plugin/types.go` - add Deprecated pointer field to Response struct
- `cmd/ze/internal/cmdutil/cmdutil.go` - extract deprecation from response, emit to stderr

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/cli/deprecated-command.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - command deprecation lifecycle |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A (existing commands gain deprecation metadata, not new commands) |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - deprecation lifecycle as part of registration pattern |

## Files to Create
- `internal/component/command/deprecation.go` - Deprecation type, State derivation, formatters
- `internal/component/command/deprecation_test.go` - unit tests
- `test/cli/deprecated-command.ci` - functional test: deprecated command from shell
- `test/cli/removed-command.ci` - functional test: removed command from shell

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: `TestDeprecationStateActive`, `TestLookupLocalMetaReturnsDeprecation`
   - Files: `internal/component/command/deprecation.go`, `cmd/ze/internal/cmdregistry/registry.go`
   - Verify: Deprecation type exists; LookupLocal returns it; tests fail because State logic is a stub

2. **Phase: Core types and state derivation** -- implement Deprecation, Replacement, Arg, State()
   - Tests: `TestDeprecationStateActive`, `TestDeprecationStateWarning`, `TestDeprecationStateRemoved`, `TestDeprecationStateNoRemoveAt`
   - Files: `internal/component/command/deprecation.go`
   - Verify: all state derivation tests pass

3. **Phase: Stderr formatting** -- implement structured stderr output
   - Tests: `TestDeprecationFormatStderr`, `TestDeprecationFormatStderrNoReplacement`, `TestDeprecationFormatStderrRemoved`, `TestReplacementRender`
   - Files: `internal/component/command/deprecation.go`
   - Verify: formatter produces correct structured lines

4. **Phase: Response.Deprecated field** -- add Deprecated field to plugin.Response
   - Tests: `TestResponseDeprecationRendered`
   - Files: `internal/component/plugin/types.go`
   - Verify: Response can carry structured deprecation; JSON serialization includes it

5. **Phase: Shell dispatch integration** -- wire both registration-level and response-level deprecation into cmd/ze/main.go dispatch
   - Tests: `TestShellDeprecatedCommandStderr`, `TestShellRemovedCommandNoExecution`
   - Files: `cmd/ze/main.go`, `cmd/ze/internal/cmdregistry/registry.go` (add LookupLocalMeta)
   - Verify: registration-level: deprecated commands emit stderr warning, removed commands return error. Response-level: handler deprecation info rendered to stderr after handler returns.

6. **Phase: Interactive CLI integration** -- render deprecation in feedback line
   - Tests: `TestCLIDeprecatedFeedback`
   - Files: `internal/component/cli/model_commands.go`, `internal/component/cli/model_render.go`
   - Verify: feedback line shows deprecation warning with correct style

7. **Phase: Handler migration** -- convert existing ad-hoc sites to Response.Deprecated
   - Tests: existing tests for show interface, clear interface, cache (must still pass); `TestHandlerRemovedGrammarRejectsExecution`
   - Files: `show/show.go`, `clear.go`, `cache.go`
   - Verify: old withDeprecation deleted; `"deprecated"` key no longer in Response.Data; all existing deprecation tests pass with new mechanism

8. **Phase: Help integration** -- deprecation info in help output via command.Node
   - Tests: help output tests for deprecated commands
   - Files: `internal/component/command/node.go`, help rendering code
   - Verify: deprecated commands show `[deprecated]` suffix; removed commands hidden from help

9. **Functional tests** -- create end-to-end .ci tests
10. **Full verification** -- `make ze-precommit-verify`
11. **Complete spec** -- audit, learned summary, spec closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | State derivation matches date comparison semantics (on RemoveAt = removed, not warning) |
| Two-level separation | Registration-level deprecation on Meta/Node; handler-level on Response.Deprecated. No mixing. |
| Naming | Deprecation, Replacement, Arg names consistent across packages |
| Data flow | Deprecation never injected ad-hoc into Response.Data; only via Response.Deprecated field |
| Rule: no-layering | withDeprecation() fully deleted, not wrapped; `"deprecated"` key gone from Data |
| Rule: derive-not-hardcode | Deprecation status derived from dates, not manual enum |
| LookupLocal compat | LookupLocal signature unchanged; new LookupLocalMeta added alongside |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Deprecation type in command package | `grep -rn 'type Deprecation' internal/component/command/` |
| Meta.Deprecation field | `grep -n 'Deprecation' cmd/ze/internal/cmdregistry/registry.go` |
| Node.Deprecation field | `grep -n 'Deprecation' internal/component/command/node.go` |
| Response.Deprecated field | `grep -n 'Deprecated' internal/component/plugin/types.go` |
| LookupLocalMeta function | `grep -n 'LookupLocalMeta' cmd/ze/internal/cmdregistry/registry.go` |
| Stderr output on deprecated shell command | run deprecated command, check stderr |
| withDeprecation deleted | `grep -rn 'withDeprecation' internal/` returns nothing |
| Ad-hoc "deprecated" key gone from Response.Data | `grep -rn '"deprecated"' internal/` returns nothing |
| Functional tests exist | `ls test/cli/deprecated-command.ci test/cli/removed-command.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Replacement.Command is set at registration (compile-time); no user input in deprecation output |
| Injection via Message | Message field rendered to stderr; ensure no format-string injection (use direct write, not Sprintf with user data) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-deprecation.md`
- [ ] Summary included in commit
