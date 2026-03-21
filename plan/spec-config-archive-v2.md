# Spec: Config Archive v2 — Named Archives with Triggers and System Identity

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/hub/schema/ze-hub-conf.yang` - current YANG schema
4. `internal/component/config/editor/archive.go` - current archive logic
5. `internal/component/config/environment.go` - environment config pattern
6. `internal/component/config/environment_extract.go` - tree extraction pattern
7. `cmd/ze/config/cmd_archive.go` - current CLI command

## Task

Redesign the config archive feature from flat `archive { location X; }` to named archive blocks with per-destination config, configurable filename format, triggers (commit/manual/daily/hourly), and change-based filtering. Add a `system {}` top-level block for hostname/domain identity (with `$ENV` variable expansion). `archive {}` is nested under `system {}` in config but implemented as its own Go component/package. CLI becomes `ze config archive <name>` via daemon RPC.

### User Requirements (verbatim)

- `archive` is nested under `system {}` in config: `system { host; domain; archive { <name> { ... } } }`
- `archive` is its own component in Go code structure (separate package from system)
- Named archive blocks: `system { archive { <name> { ... } } }`
- One location per named block
- Configurable filename format with tokens
- Trigger types: `commit`, `manual`, `daily`, `hourly`
- `on-change true/false` boolean for time-based triggers (skip if unchanged when true, default false)
- Time-based triggers fire on daemon boot, tracked in memory
- `timeout` moves from CLI flag to config per-block
- CLI: `ze config archive <name>` (no config-file arg, daemon knows)
- No `--location` or `--timeout` CLI flags — all in config
- `system { host <val>; domain <val>; }` — new block, `$ENV` expansion, no `os.Hostname()`
- Remove `os.Hostname()` calls — use `system.host` from config

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing format
  → Constraint: config uses `keyword { ... }` block syntax, `;` terminators
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema design
  → Decision: YANG schemas define config structure, parsed by config package
- [ ] `docs/architecture/config/environment.md` - environment variable handling
  → Constraint: env variables use `ze.bgp.<section>.<option>` format

### Source Files
- [ ] `internal/component/hub/schema/ze-hub-conf.yang` - current hub YANG
  → Constraint: `archive` currently top-level with flat `leaf-list location`, `environment` container exists, no `system` container. `archive` will move under `system` in YANG but be a separate Go component
- [ ] `internal/component/config/environment.go` - env config loading pattern
  → Decision: `Environment` struct with typed fields, loaded via `LoadEnvironment()`
- [ ] `internal/component/config/environment_extract.go` - tree extraction
  → Decision: `ExtractEnvironment(tree)` walks `tree.GetContainer("environment")` to extract values
- [ ] `internal/component/config/editor/archive.go` - current archive logic
  → Constraint: `ArchiveToFile`, `ArchiveToHTTP`, `ArchiveToLocations` are the core uploaders
- [ ] `cmd/ze/config/cmd_archive.go` - current CLI
  → Decision: current CLI takes `<config-file>` arg with `--location` and `--timeout` flags

**Key insights:**
- Config tree access via `tree.GetContainer()` → `Get()` for leaves, `GetContainer()` for sub-blocks
- Named blocks under a container appear as sub-containers keyed by name
- `$ENV` expansion is a new pattern — not currently used in config parsing
- Time-based triggers require a scheduler goroutine in the daemon
- The daemon already has the config file path (passed via `--config` flag)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/editor/archive.go` - flat fan-out model, `ExtractArchiveLocations` text parser, `ArchiveFilename` hardcoded format, `ArchiveToFile`/`ArchiveToHTTP` uploaders
- [ ] `cmd/ze/config/cmd_archive.go` - standalone CLI with `--location`/`--timeout`, reads config file, calls `ArchiveToLocations` directly
- [ ] `cmd/ze/config/cmd_edit.go:321-328` - wires `ArchiveNotifier` at editor startup using `ExtractArchiveLocations(ed.OriginalContent())`
- [ ] `internal/component/config/editor/editor.go:40` - `onArchive ArchiveNotifier` field
- [ ] `internal/component/config/editor/model_commands.go:456-463` - calls `NotifyArchive()` in `cmdCommit()`
- [ ] `internal/component/hub/schema/ze-hub-conf.yang:13-20` - flat `archive { location leaf-list }`

**Behavior to preserve:**
- `ArchiveToFile` — file:// upload with `os.MkdirAll` + `os.WriteFile`
- `ArchiveToHTTP` — HTTP POST with `text/plain` Content-Type + `X-Archive-Filename` header
- Fan-out error collection pattern (errors per-location, non-fatal)
- Archive on commit in editor (now per named block with `trigger commit`)
- Body drain pattern for HTTP responses

**Behavior to change:**
- Top-level flat `archive { location X; }` → named blocks under `system { archive { <name> { ... } } }`
- Hardcoded `ArchiveFilename` → configurable format with token substitution
- `os.Hostname()` → `system.host` from config
- CLI `ze config archive <config-file>` → `ze config archive <name>` via daemon RPC
- Remove `--location` and `--timeout` CLI flags
- `ExtractArchiveLocations` text parser → tree-based extraction of named blocks under `system.archive`
- `NewArchiveNotifier` → archive scheduler with trigger awareness

## Data Flow (MANDATORY)

### Entry Points

**Manual trigger (CLI):**
- User runs `ze config archive <name>`
- CLI sends RPC to daemon via unix socket
- Daemon looks up named archive block, reads current config, archives

**Commit trigger (editor):**
- User runs `commit` in editor
- `cmdCommit()` → `NotifyArchive()` → fires all archive blocks with `trigger commit`

**Time-based trigger (daemon scheduler):**
- Daemon starts → scheduler goroutine starts
- On boot: fires all time-based archives immediately
- On interval: checks `on-change` flag, skips if unchanged, archives otherwise

### Transformation Path

1. Config parsed → `system.host`/`system.domain` resolved (with `$ENV` expansion)
2. Archive blocks extracted from `system.archive` sub-container → `ArchiveConfig` structs
3. Filename format tokens substituted → concrete filename
4. Content dispatched to location via `ArchiveToFile` or `ArchiveToHTTP`

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI → Daemon | RPC over unix socket (`ze config archive <name>`) | [ ] |
| Config → Runtime | Tree extraction → `ArchiveConfig` struct | [ ] |
| Editor → Archive | `ArchiveNotifier` callback on commit | [ ] |

### Integration Points

- `ze-hub-conf.yang` — `system` container with `archive` nested inside
- `environment_extract.go` pattern — tree walking for `system.archive` named sub-containers
- `cmd_edit.go` — wiring archive notifier at editor startup
- `model_commands.go` — commit trigger dispatch
- Daemon main loop — scheduler for time-based triggers
- RPC registration — `ze config archive <name>` handler

### Architectural Verification

- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Design

### YANG Schema — `system {}` block (new top-level container)

New top-level container in `ze-hub-conf.yang`. Contains identity leaves and nested `archive` sub-container.

**`system` identity leaves:**

| Leaf | Type | Description |
|------|------|-------------|
| `host` | string | Hostname, supports `$ENV_VAR` expansion |
| `domain` | string | Domain name, supports `$ENV_VAR` expansion |

If `host` is unset, default to `"unknown"`. No `os.Hostname()` fallback — explicit config only.

**`system.archive` sub-container — named archive blocks:**

Replaces current flat top-level `archive { leaf-list location }`. Each named child is a separate archive destination.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `location` | string | (required) | Archive destination URL (`file://`, `http://`, `https://`) |
| `filename` | string | `"{name}-{host}-{date}-{time}"` | Filename format with token substitution |
| `timeout` | string | `"30s"` | HTTP upload timeout (Go duration) |
| `trigger` | string | `"manual"` | When to archive: `commit`, `manual`, `daily`, `hourly` |
| `on-change` | boolean | `false` | Time-based only: when `true`, skip archive if config unchanged since last archive |

**Config format:** `system { host <val>; domain <val>; archive { <name> { location <url>; trigger <keyword>; } } }`

Example paths: `system.host` = `router1`, `system.archive.local-backup.location` = `file:///backups`, `system.archive.offsite.trigger` = `daily`

### Code Structure — Separate Components

Despite `archive` being nested under `system` in config, they are separate Go components:

| Component | Package | Responsibility |
|-----------|---------|----------------|
| System | `internal/component/config/system/` | Identity config: host, domain, `$ENV` expansion |
| Archive | `internal/component/config/archive/` | Archive logic: uploaders, filename format, scheduler, change tracker |

Archive imports system to resolve `{host}` and `{domain}` filename tokens. System has no knowledge of archive.

### Archive Events

On archive trigger (commit, manual, or time-based), the archive system emits an event that plugins can subscribe to. This allows plugins to implement custom archive backends (S3, git push, SFTP, etc.).

| Event | Payload | When |
|-------|---------|------|
| `config/archive` | Config content + archive name + filename | Before built-in upload |

Plugins subscribe to `config/archive` and receive the config content. Built-in `file://` and `http(s)://` uploaders remain as core logic — plugin events are in addition to (not replacing) the built-in handlers.

### Filename Tokens

| Token | Value | Example |
|-------|-------|---------|
| `{name}` | Config file basename (no extension) | `config` |
| `{host}` | `system.host` value | `router1` |
| `{domain}` | `system.domain` value | `dc1.example.com` |
| `{date}` | `YYYYMMDD` | `20260310` |
| `{time}` | `HHMMSS` | `143045` |
| `{archive}` | Archive block name | `local-backup` |

Default format: `"{name}-{host}-{date}-{time}"`. Output always gets `.conf` extension appended.

### `$ENV` Variable Expansion

For `system.host` and `system.domain` leaves:
- If value starts with `$`, treat remainder as OS environment variable name
- `$HOSTNAME` → `os.Getenv("HOSTNAME")`
- If env var is empty or unset, use the literal string (e.g., `$HOSTNAME` stays as `"$HOSTNAME"`)
- Non-`$` values used as-is (e.g., `router1` stays as `"router1"`)

### CLI — `ze config archive <name>`

- No config-file argument (daemon knows its config)
- No `--location` or `--timeout` flags (all in config)
- Sends RPC to daemon: `ze-hub-conf:archive-trigger` with `name` parameter
- Daemon looks up the named block, archives current config content
- Exit 0 on success, exit 2 on error

### Trigger Behavior

| Trigger | When | Boot | `on-change` |
|---------|------|------|-------------|
| `commit` | After editor commit | No | No (always archives) |
| `manual` | CLI `ze config archive <name>` | No | No |
| `daily` | Every 24h from boot | Yes (always) | Yes (optional) |
| `hourly` | Every 1h from boot | Yes (always) | Yes (optional) |

**Boot behavior:** All time-based archives fire immediately on daemon start, regardless of `on-change`. This ensures a baseline archive exists after restart.

**Change detection:** SHA-256 hash of config content. In-memory tracker per archive name. Resets on daemon restart (boot always archives, so first interval check has a baseline).

### ArchiveConfig Struct

Runtime representation of one named archive block:

| Field | Type |
|-------|------|
| `Name` | `string` |
| `Location` | `string` |
| `Filename` | `string` |
| `Timeout` | `time.Duration` |
| `Trigger` | `string` |
| `OnChange` | `bool` |

### Archive Scheduler

Long-lived goroutine in daemon, started after config load:
- Receives `[]ArchiveConfig` for all time-based triggers
- On start: fires all immediately (boot archive)
- Uses `time.Ticker` per interval type (hourly/daily)
- On tick: check `on-change` (hash comparison), archive if changed or `on-change` not set
- Stopped on daemon shutdown (context cancellation)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config archive <name>` CLI | → | daemon RPC → archive dispatch | `test/parse/cli-config-archive-named.ci` |
| Editor `commit` with `trigger commit` | → | `NotifyArchive` → per-block filter | `TestArchiveOnCommit_TriggerFilter` |
| Config with `system { host X; }` | → | `ExtractSystemConfig` reads host | `test/parse/cli-config-system.ci` |
| Config with `system { archive { <name> { ... } } }` | → | Parsed by YANG, extracted at runtime | `test/parse/cli-config-archive-named.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `system { host router1; }` | `host` value accessible as `"router1"` at runtime |
| AC-2 | Config with `system { host $HOSTNAME; }` | `host` resolved from `HOSTNAME` env var |
| AC-3 | Config with `system { archive { <name> { ... } } }` | Named block parsed with all fields (location, filename, timeout, trigger, on-change) |
| AC-4 | `ze config archive <name>` | Sends RPC to daemon, daemon archives to named block's location |
| AC-5 | Editor commit with `trigger commit` blocks | Only `trigger commit` blocks fire, `manual`/`daily`/`hourly` skipped |
| AC-6 | Filename format `"{name}-{host}-{date}"` | Tokens substituted, `.conf` appended |
| AC-7 | `on-change` set, config unchanged | Time-based archive skipped |
| AC-8 | `on-change` set, config changed | Time-based archive fires |
| AC-9 | Daemon boot with `trigger daily` | Archive fires immediately on boot regardless of `on-change` |
| AC-10 | No `system.host` configured | Default `"unknown"` used in filename |
| AC-11 | Invalid location scheme | Rejected at config validation time |
| AC-12 | Archive triggered (any trigger type) | `config/archive` event emitted for plugin subscribers |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExpandEnvValue` | `editor/archive_test.go` | `$VAR` expansion for system values | |
| `TestExpandEnvValue_NoPrefix` | `editor/archive_test.go` | Non-`$` values returned as-is | |
| `TestExpandEnvValue_EmptyEnv` | `editor/archive_test.go` | Empty env var returns literal `$VAR` | |
| `TestFormatArchiveFilename` | `editor/archive_test.go` | Token substitution in filename format | |
| `TestFormatArchiveFilename_Default` | `editor/archive_test.go` | Default format when none specified | |
| `TestFormatArchiveFilename_AllTokens` | `editor/archive_test.go` | All 6 tokens substituted correctly | |
| `TestExtractSystemConfig` | `config/system_test.go` or `editor/archive_test.go` | `system { host X; domain Y; }` extraction | |
| `TestExtractSystemConfig_EnvExpansion` | same | `$ENV` expansion in host/domain | |
| `TestExtractSystemConfig_Missing` | same | No system block → defaults | |
| `TestExtractArchiveConfigs` | `editor/archive_test.go` | Named blocks extracted with all fields | |
| `TestExtractArchiveConfigs_Defaults` | `editor/archive_test.go` | Missing optional fields get defaults | |
| `TestExtractArchiveConfigs_MultipleBlocks` | `editor/archive_test.go` | Multiple named blocks all extracted | |
| `TestValidateTrigger` | `editor/archive_test.go` | Valid trigger keywords accepted | |
| `TestValidateTrigger_Invalid` | `editor/archive_test.go` | Invalid trigger keyword rejected | |
| `TestArchiveChangeTracker` | `editor/archive_test.go` | Hash-based change detection | |
| `TestArchiveChangeTracker_Changed` | `editor/archive_test.go` | Different content detected as changed | |
| `TestArchiveChangeTracker_Boot` | `editor/archive_test.go` | First check always reports changed (boot) | |
| `TestCommitTriggerFilter` | `editor/archive_test.go` | Only `trigger commit` blocks selected | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `trigger` | enum: commit, manual, daily, hourly | `"hourly"` | `""` | `"weekly"` |
| `timeout` | duration > 0 | `"1s"` | `"0s"` | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-config-archive-named` | `test/parse/cli-config-archive-named.ci` | Config with `system { archive { <name> { ... } } }` parses | |
| `cli-config-system` | `test/parse/cli-config-system.ci` | Config with `system { host X; domain Y; }` parses | |
| `cli-config-system-archive` | `test/parse/cli-config-system-archive.ci` | Config with `system { host; domain; archive { ... } }` parses | |

## Files to Modify

- `internal/component/hub/schema/ze-hub-conf.yang` — replace top-level `archive` with `system` container (host, domain, nested archive with named sub-containers)
- `internal/component/config/editor/archive.go` — redesign: named configs, filename format, change tracker, trigger filter
- `internal/component/config/editor/archive_test.go` — update all tests for new model
- `cmd/ze/config/cmd_archive.go` — rewrite: `ze config archive <name>` via daemon RPC
- `cmd/ze/config/cmd_archive_test.go` — update tests
- `cmd/ze/config/cmd_edit.go` — update archive notifier wiring for named blocks + trigger filter
- `internal/component/config/editor/editor.go` — update `ArchiveNotifier` type if needed
- `internal/component/config/editor/model_commands.go` — commit trigger dispatch update

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new containers) | [x] | `internal/component/hub/schema/ze-hub-conf.yang` |
| CLI commands/flags | [x] | `cmd/ze/config/cmd_archive.go` |
| CLI usage/help text | [x] | `cmd/ze/config/cmd_archive.go`, `cmd/ze/config/main.go` |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for CLI | [x] | `test/parse/cli-config-archive-named.ci` |

## Files to Create

- `internal/component/config/system.go` — `SystemConfig` struct, `ExtractSystemConfig()`, `$ENV` expansion
- `internal/component/config/system_test.go` — system config unit tests
- `test/parse/cli-config-archive-named.ci` — functional test: `system { archive { <name> { ... } } }` parses
- `test/parse/cli-config-system.ci` — functional test: `system { host; domain; }` parses
- `test/parse/cli-config-system-archive.ci` — functional test: `system { host; domain; archive { ... } }` parses

## Implementation Steps

### Phase 1 — System Block + Filename Format

1. **YANG:** Replace top-level `archive` with `system` container containing `host`, `domain`, and nested `archive` with named sub-containers
2. **System config:** `ExtractSystemConfig(tree)` with `$ENV` expansion for `host`/`domain`
3. **Filename format:** Replace `ArchiveFilename` with token-based `FormatArchiveFilename`
4. **Tests:** All system and filename tests

### Phase 2 — Named Archive Blocks

5. **YANG:** Named sub-containers under `system.archive` (already added in Phase 1 YANG change)
6. **Extract:** `ExtractArchiveConfigs(tree)` returning `[]ArchiveConfig` from `system.archive` sub-tree
7. **Validation:** trigger keywords, location scheme, timeout parsing
8. **Update CLI:** `ze config archive <name>` (initially can work standalone with config file arg while daemon RPC is deferred)
9. **Update editor wiring:** filter by `trigger commit` on commit
10. **Tests:** All named block tests + functional tests

### Phase 3 — Time-Based Triggers + Change Detection

11. **Change tracker:** SHA-256 hash comparison, per-archive-name memory
12. **Archive scheduler:** long-lived goroutine with `time.Ticker`
13. **Boot behavior:** fire all time-based on start
14. **Daemon integration:** wire scheduler into daemon startup
15. **Tests:** scheduler and change detection tests

### Phase 4 — Daemon RPC

16. **RPC handler:** `ze-hub-conf:archive-trigger` with `name` param
17. **Wire into daemon:** register RPC, dispatch to archive logic
18. **CLI update:** send RPC instead of direct archive
19. **Tests:** RPC functional tests

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix syntax/types |
| Test fails wrong reason | Fix test |
| YANG parse error | Check schema syntax |
| Scheduler race condition | Add synchronization, use channel pattern |

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

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
