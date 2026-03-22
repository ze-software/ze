# Spec: blob-namespaces

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/zefs-format.md` - ZeFS blob format
4. `pkg/zefs/store.go` - BlobStore API
5. `internal/component/config/storage/blob.go` - `resolveKey()`, `migrateExistingFiles()`, `List()`
6. `cmd/ze/init/main.go` - `ze init` (writes `meta/ssh/*` keys)
7. `cmd/ze/internal/sshclient/sshclient.go` - SSH client reads credentials from blob
8. `cmd/ze/data/main.go` - `ze data import` creates blob keys

## Task

Introduce structured key namespaces in the ZeFS blob. Keys follow `<namespace>/<qualifier>/<path>` convention. Two namespaces: `meta/` for instance metadata, `file/` for config files. The qualifier enables future versioning (`active`, `draft`, date stamps) without format changes.

Deliverables:
1. `meta/` namespace for instance metadata (`meta/ssh/username`, `meta/instance/name`, `meta/instance/managed`)
2. `file/` namespace with qualifier for config files (`file/active/etc/ze/router.conf`)
3. `ze init` extended: sets `meta/instance/name` and `meta/instance/managed` (value: `true`/`false`)
4. `ze data import` stores as `file/active/ze.conf`
5. `resolveKey()` idempotent: already-namespaced keys pass through unchanged
6. `List()` returns full blob key names (callers see namespaces)

## Key Structure

| Key | Meaning |
|-----|---------|
| `meta/ssh/username` | SSH credential (instance metadata) |
| `meta/ssh/password` | SSH credential |
| `meta/ssh/host` | SSH credential |
| `meta/ssh/port` | SSH credential |
| `meta/instance/name` | Instance name |
| `meta/instance/managed` | Fleet-managed flag (`true`/`false`) |
| `file/active/etc/ze/router.conf` | Current committed config |
| `file/active/etc/ze/ssh_host_ed25519_key` | SSH host key |

Future qualifiers (spec-config-versioning, not this spec):

| Key | Meaning |
|-----|---------|
| `file/draft/etc/ze/router.conf` | Live edit in progress |
| `file/20260318-100000/etc/ze/router.conf` | Historical version |

### `resolveKey()` rules

| Input | Output | Why |
|-------|--------|-----|
| `/etc/ze/router.conf` | `file/active/etc/ze/router.conf` | Filesystem path: strip `/`, prepend `file/active/` |
| `file/active/etc/ze/router.conf` | `file/active/etc/ze/router.conf` | Already namespaced: pass through |
| `meta/ssh/username` | `meta/ssh/username` | Already namespaced: pass through |
| `/file/active/etc/ze/router.conf` | `file/active/etc/ze/router.conf` | Strip `/`, already namespaced: pass through |
| `router.conf` (with configDir) | `file/active/<configDir>/router.conf` | Relative path: resolve, strip `/`, prepend `file/active/` |

Rule: after stripping leading `/`, if key starts with `file/` or `meta/`, return as-is. Otherwise prepend `file/active/`.

### `List()` returns full key names

`List()` returns raw blob keys including namespace. Callers can pass returned keys directly back to `ReadFile`/`WriteFile`/`Remove` -- `resolveKey()` sees the namespace prefix and does not double-prefix.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - ZeFS blob format
- [ ] `docs/architecture/fleet-config.md` - managed config (depends on this spec)

### RFC Summaries (MUST for protocol work)
No external RFCs apply.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/zefs/store.go` - BlobStore: flat key namespace, `fs.ValidPath` keys, hierarchical `/` tree
  -> Constraint: keys must pass `fs.ValidPath` (no leading `/`, no `..`). `file/active/etc/ze/router.conf` is valid.
- [ ] `internal/component/config/storage/blob.go` - `resolveKey()` strips leading `/` via `pathToKey()`. All Storage methods go through `resolveKey()`. `migrateExistingFiles()` imports `*.conf`, `*.conf.draft`, `rollback/*.conf`, `ssh_host_*` on first blob create. `List()` calls `resolveKey(prefix)` then `ReadDir()`, returns paths with leading `/`.
  -> Decision: `resolveKey()` prepends `file/active/` for filesystem paths, passes through already-namespaced keys.
  -> Decision: `List()` returns full blob keys (no leading `/`). Callers see namespace.
  -> Decision: `migrateExistingFiles()` writes `file/active/` prefixed keys.
- [ ] `internal/component/config/storage/storage.go` - Storage interface: `ReadFile`, `WriteFile`, `Exists`, `List`, `Remove`, `AcquireLock`, `Stat`, `Close`. No signature changes needed.
- [ ] `cmd/ze/init/main.go` - Constants `keyUsername = "ssh/username"`, etc. (lines 21-25). `runInit()` writes 4 entries directly to zefs.BlobStore (not through Storage).
  -> Decision: change constants to `meta/ssh/username`, etc. Add `meta/instance/name` and `meta/instance/managed`.
- [ ] `cmd/ze/internal/sshclient/sshclient.go` - `ReadCredentials()` hardcodes `"ssh/username"`, `"ssh/password"`, `"ssh/host"`, `"ssh/port"` (lines 132-148). Reads directly from `zefs.BlobStore`, not through Storage. Every CLI command uses this to SSH-connect to daemon.
  -> Decision: change hardcoded keys to `meta/ssh/username`, etc. CRITICAL: if init changes but this doesn't, all CLI commands break.
- [ ] `cmd/ze/data/main.go` - `ls`, `cat`, `rm` use raw blob keys (no changes needed). `import` uses `filePathToKey()` which strips `/` to produce bare keys.
  -> Decision: `ze data import` stores as `file/active/ze.conf`. `ls`/`cat`/`rm` need no changes.
- [ ] `internal/component/ssh/` - SSH **server** reads host key path via `filepath.Join(configDir, "ssh_host_ed25519_key")`, goes through `resolveKey()` in Storage. Gets `file/active/` prefix automatically. Does NOT read `ssh/username` -- that's the SSH **client** (`sshclient.go`).
  -> Decision: no SSH server code changes needed.

**Behavior to preserve:**
- ZeFS blob format unchanged (key namespaces are a convention, not a format change)
- `ze data ls/cat/rm` continue to work (with namespaced keys)
- Storage interface signatures unchanged

**Behavior to change:**
- `ze init` writes keys with `meta/` prefix
- `ze init` gains `meta/instance/name` (optional, prompted) and `meta/instance/managed` (`--managed` flag, default `false`)
- SSH client (`sshclient.go`) reads `meta/ssh/*` instead of `ssh/*`
- `resolveKey()` prepends `file/active/` for filesystem paths, passes through already-namespaced keys
- `List()` returns full blob keys (namespace included, no leading `/`)
- `migrateExistingFiles()` writes `file/active/` prefixed keys on blob create
- `ze data import` stores as `file/active/ze.conf`

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze init` writes `meta/` keys directly to BlobStore
- SSH client (`sshclient.go`) reads `meta/ssh/*` keys directly from BlobStore
- Storage layer translates filesystem paths to `file/active/` keys via `resolveKey()`
- `ze data import` writes `file/active/ze.conf` directly to BlobStore
- `migrateExistingFiles()` writes `file/active/` prefixed keys on blob create

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Storage -> Blob | `resolveKey()` prepends `file/active/` for filesystem paths, idempotent for namespaced keys | [ ] |
| `ze init` -> Blob | Writes `meta/` prefixed keys directly | [ ] |
| SSH client -> Blob | Reads `meta/ssh/*` keys directly | [ ] |
| `ze data import` -> Blob | Stores as `file/active/ze.conf` | [ ] |
| `migrateExistingFiles()` -> Blob | Writes `file/active/` prefixed keys on create | [ ] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init` | -> | Blob contains `meta/ssh/username` (not `ssh/username`) | `test/managed/init-meta-keys.ci` |
| `ze init --managed` | -> | Blob contains `meta/instance/managed` with value `true` | `test/managed/init-managed-key.ci` |
| `ze config` (any CLI command) | -> | `sshclient.ReadCredentials()` reads `meta/ssh/*` keys | `test/managed/cli-reads-meta-keys.ci` |
| `ze daemon config.conf` | -> | Config stored under `file/active/` prefix | `test/managed/file-namespace.ci` |
| `ze data import /path/to/file` | -> | Key stored as `file/active/ze.conf` | Unit test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze init` | Blob keys: `meta/ssh/username`, `meta/ssh/password`, `meta/ssh/host`, `meta/ssh/port` |
| AC-2 | `ze init` with name prompted | Blob contains `meta/instance/name` with prompted value |
| AC-3 | `ze init --managed` | Blob contains `meta/instance/managed` with value `true`. Without flag: `false`. |
| AC-4 | Config written via Storage layer | Key uses `file/active/` prefix (e.g., `file/active/etc/ze/router.conf`) |
| AC-5 | Any CLI command after `ze init` | `sshclient.ReadCredentials()` reads `meta/ssh/*` keys successfully |
| AC-6 | `ze data import /path/to/file` | Key stored as `file/active/ze.conf` |
| AC-7 | `ze data ls meta/` | Returns only `meta/*` keys. `ze data ls file/` returns only `file/*` keys. |
| AC-8 | `resolveKey("file/active/etc/ze/router.conf")` | Returns unchanged (idempotent, no double-prefix) |
| AC-9 | `List("/etc/ze")` via Storage | Returns full blob keys like `file/active/etc/ze/router.conf` |
| AC-10 | Pass `List()` result back to `ReadFile()` | Reads successfully (resolveKey passes through namespaced key) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZeInitMetaKeys` | `cmd/ze/init/main_test.go` | Init writes `meta/ssh/username` not `ssh/username` (AC-1) | |
| `TestZeInitIdentityName` | `cmd/ze/init/main_test.go` | Init writes `meta/instance/name` (AC-2) | |
| `TestZeInitManagedKey` | `cmd/ze/init/main_test.go` | Init writes `meta/instance/managed` `true` with flag, `false` without (AC-3) | |
| `TestReadCredentialsMeta` | `cmd/ze/internal/sshclient/sshclient_test.go` | `ReadCredentials()` reads `meta/ssh/*` keys (AC-5) | |
| `TestBlobStorageFilePrefix` | `internal/component/config/storage/storage_test.go` | Config paths get `file/active/` prefix in blob (AC-4) | |
| `TestDbImportFilePrefix` | `cmd/ze/data/main_test.go` | `ze data import` stores as `file/active/ze.conf` (AC-6) | |
| `TestResolveKeyIdempotent` | `internal/component/config/storage/storage_test.go` | Already-namespaced keys pass through unchanged (AC-8) | |
| `TestBlobStorageListReturnsFullKeys` | `internal/component/config/storage/storage_test.go` | `List()` returns full blob keys with namespace (AC-9) | |
| `TestBlobStorageListRoundTrip` | `internal/component/config/storage/storage_test.go` | `List()` result passed to `ReadFile()` works (AC-10) | |
| `TestBlobMigrateFilesystemPrefixed` | `internal/component/config/storage/storage_test.go` | Filesystem migration writes `file/active/` prefixed keys | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `init-meta-keys` | `test/managed/init-meta-keys.ci` | `ze init` creates blob with `meta/ssh/*` keys, `ze data ls meta/` shows them | |
| `init-managed-key` | `test/managed/init-managed-key.ci` | `ze init --managed` stores `meta/instance/managed` as `true` | |
| `cli-reads-meta-keys` | `test/managed/cli-reads-meta-keys.ci` | CLI command connects via `meta/ssh/*` credentials after init | |
| `file-namespace` | `test/managed/file-namespace.ci` | Config stored under `file/active/` prefix, visible via `ze data ls file/` | |

## Files to Modify
- `cmd/ze/init/main.go` - change key constants to `meta/ssh/*`, add `--managed` flag, add name prompt, add `meta/instance/name` and `meta/instance/managed` entries
- `cmd/ze/init/main_test.go` - update existing tests for new key names, add tests for identity/managed
- `cmd/ze/internal/sshclient/sshclient.go` - change hardcoded `"ssh/username"` etc. to `"meta/ssh/username"` etc.
- `internal/component/config/storage/blob.go` - `resolveKey()` prepends `file/active/` (idempotent), `migrateExistingFiles()` writes `file/active/` prefixed keys, `List()` returns full blob keys
- `internal/component/config/storage/storage_test.go` - tests for `file/active/` prefix, idempotent resolveKey, List round-trip, filesystem migration
- `cmd/ze/data/main.go` - `filePathToKey()` returns `file/active/ze.conf`
- `docs/architecture/zefs-format.md` - document namespace convention

## Files to Create
- `test/managed/init-meta-keys.ci` - functional test for init meta keys
- `test/managed/init-managed-key.ci` - functional test for managed flag
- `test/managed/cli-reads-meta-keys.ci` - functional test for CLI using meta credentials
- `test/managed/file-namespace.ci` - functional test for file prefix

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

### Phase 1: Init + SSH client key changes (AC-1, AC-2, AC-3, AC-5)
- Update `cmd/ze/init/main_test.go`: change existing assertions from `ssh/username` to `meta/ssh/username` etc. Add `TestZeInitIdentityName` and `TestZeInitManagedKey`.
- Add `TestReadCredentialsMeta` in `cmd/ze/internal/sshclient/sshclient_test.go`: verify `ReadCredentials()` reads `meta/ssh/*` keys.
- Tests: run and confirm FAIL
- Update `cmd/ze/init/main.go`: change constants to `meta/ssh/*`, add `--managed` flag, add name prompt, add entries for `meta/instance/name` and `meta/instance/managed`
- Update `cmd/ze/internal/sshclient/sshclient.go`: change hardcoded keys to `meta/ssh/*`
- Tests: run and confirm PASS

### Phase 2: Storage namespace + List changes (AC-4, AC-6, AC-8, AC-9, AC-10)
- Add `TestBlobStorageFilePrefix`, `TestResolveKeyIdempotent`, `TestBlobStorageListReturnsFullKeys`, `TestBlobStorageListRoundTrip` in `storage_test.go`
- Add `TestDbImportFilePrefix` in `cmd/ze/data/main_test.go`
- Add `TestBlobMigrateFilesystemPrefixed` in `storage_test.go`
- Tests: run and confirm FAIL
- Update `resolveKey()` in `blob.go`: idempotent namespace-aware resolution
- Update `List()` in `blob.go`: return full blob keys
- Update `migrateExistingFiles()` in `blob.go`: write `file/active/` prefixed keys
- Update `filePathToKey()` in `cmd/ze/data/main.go`: return `file/active/ze.conf`
- Tests: run and confirm PASS

### Phase 3: Functional tests + docs + verification (AC-7)
- Create `.ci` functional tests: `init-meta-keys`, `init-managed-key`, `cli-reads-meta-keys`, `file-namespace`
- Update `docs/architecture/zefs-format.md` with namespace convention
- Run `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | `resolveKey()` idempotent: namespaced keys pass through, filesystem paths get `file/active/` |
| Correctness | `List()` returns full blob keys, round-trip to `ReadFile()` works |
| Correctness | `sshclient.go` reads `meta/ssh/*` keys (not old `ssh/*`) |
| Naming | `meta/` keys use exact names from Key Structure table |
| Data flow | `ze init` writes directly to BlobStore, Storage layer only sees config paths |
| Rule: no-layering | No old `ssh/*` key code left anywhere |
| Rule: compatibility | No migration code, no backward compat shims |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `meta/ssh/*` keys written by init | `grep "meta/ssh/" cmd/ze/init/main.go` |
| `meta/instance/name` key written by init | `grep "meta/instance/name" cmd/ze/init/main.go` |
| `meta/instance/managed` key written by init | `grep "meta/instance/managed" cmd/ze/init/main.go` |
| `sshclient.go` reads `meta/ssh/*` | `grep "meta/ssh/" cmd/ze/internal/sshclient/sshclient.go` |
| `resolveKey()` prepends `file/active/` | `grep "file/active" internal/component/config/storage/blob.go` |
| `resolveKey()` idempotent | `TestResolveKeyIdempotent` passes |
| `List()` returns full keys | `TestBlobStorageListReturnsFullKeys` passes |
| `ze data import` uses `file/active/ze.conf` | `grep "file/active" cmd/ze/data/main.go` |
| `zefs-format.md` updated | `grep "meta/" docs/architecture/zefs-format.md` |
| 4 `.ci` functional tests exist | `ls test/managed/*.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | `resolveKey()` must not allow path traversal (`../`) to escape namespace |
| Key injection | Verify user-prompted `meta/instance/name` value cannot contain `/` or other key separators that would create unexpected keys |
| Credential exposure | `meta/ssh/password` must not appear in logs, error messages, or `ze data ls` output format |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Migration needed for old blobs | Ze has no users, no backward compat needed (`rules/compatibility.md`) | User corrected | Removed entire migration system |
| SSH server reads `ssh/username` | SSH **client** (`sshclient.go`) reads credentials, SSH server only reads host keys via Storage | Grep for all consumers | `sshclient.go` added to spec |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Flat `file/` prefix without qualifier | No room for versioning without restructuring keys later | `file/<qualifier>/` structure |
| Option C: metadata per entry in ZeFS format | Reinventing a database; SQLite exists | Option A: metadata encoded in key structure |
| Migration on blob open | No backward compat needed | Just change the code |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- `ze init` and `sshclient.ReadCredentials()` both access `zefs.BlobStore` directly (not through Storage). Both use hardcoded key strings. Both need updating together.
- `resolveKey()` is called by ALL blobStorage methods. Making it idempotent (pass through already-namespaced keys) means `List()` results can be fed back to `ReadFile()` without double-prefixing.
- `List()` currently returns filesystem-style paths with leading `/`. Changing to full blob keys is a behavior change for callers. Editor's `ListBackups()` uses `filepath.Base()` on results (still works). Keys returned by `List()` round-trip through `ReadFile()` because `resolveKey()` is idempotent.
- SSH **server** uses host key paths through Storage/`resolveKey()` -- gets `file/active/` prefix automatically. SSH **client** (`sshclient.go`) reads credential keys directly from BlobStore -- needs explicit change.
- `file/<qualifier>/` structure reserves space for `draft`, date-stamped versions without restructuring. This spec only uses `active` as qualifier; `spec-config-versioning` will add others.
- Ze has no users. No backward compat. No migration. Just change the code (`rules/compatibility.md`).

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-blob-namespaces.md`
- [ ] **Summary included in commit**
