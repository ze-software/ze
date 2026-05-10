# Spec: appliance-4-device-config

| Field | Value |
|-------|-------|
| Status | design |
| Depends | appliance-1-builder, appliance-2-remote |
| Phase | - |
| Updated | 2026-05-10 |
| Split | Split from appliance-1-builder. Device-side runtime behavior for config loading and revert. |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/675-appliance-1-builder.md` - learned summary
4. `cmd/ze/main.go` (lines 644-736, 991-1024) - cmdStart and bootstrapConfigFromTemplate
5. `cmd/ze/appliance/cmd_assemble.go` - assembleZeFS (add last-known-good hash write)
6. `pkg/zefs/keys.go` - ZeFS key registry (add KeyConfigLastKnownGood)
7. `internal/component/config/loader.go` - LoadConfig for pushed config validation

## Task

Device-side config management for ze appliances: config loading priority (pushed > seed), validation of pushed configs before applying, automatic revert on runtime failure, and last-known-good hash verification at boot.

Split from `spec-appliance-1-builder` (design session 2026-05-09). The bastion side of config-push (SSH upload) is in `spec-appliance-2-remote`. This spec covers what the device does when it receives a pushed config.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/675-appliance-1-builder.md` - Builder learned summary
  -> Decision: ZeFS contains meta/config/last-known-good with SHA-256 of validated seed config
  -> Constraint: ZeFS seed config is immutable (the ultimate fallback)

### Source Files
- [ ] `cmd/ze/main.go` (1034L) - cmdStart (line 644): resolveStorage -> resolveDefaultConfig -> bootstrapConfigFromTemplate -> hub.Run. Bootstrap reads file/template/ze.conf from ZeFS, merges with interface discovery, writes to file/active/<name>.
  -> Decision: config-pushed.conf support must integrate into this flow, after bootstrap but before hub.Run
  -> Constraint: first-boot bootstrap (template + discovery) must remain unchanged when no pushed config exists
- [ ] `cmd/ze/main.go:991` - bootstrapConfigFromTemplate: reads KeyFileTemplate.Key("ze.conf"), runs iface discovery, writes to KeyFileActive.Key(configName)
  -> Decision: pushed config check goes after bootstrap (bootstrap creates the fallback; pushed config overrides it)
- [ ] `cmd/ze/appliance/cmd_assemble.go` (197L) - assembleZeFS writes to KeyFileTemplate.Key("ze.conf") via store.WriteFile
  -> Decision: add meta/config/last-known-good hash write after seed config write
- [ ] `cmd/ze/appliance/cmd_build.go` (142L) - calls assembleZeFS; manifest.ConfigHash already computed
  -> Constraint: last-known-good hash must use same hash as manifest.ConfigHash (SHA-256 of seed config)
- [ ] `pkg/zefs/keys.go` (25L) - Registered keys: KeyFileTemplate, KeyFileActive, KeyInstanceManaged, etc.
  -> Decision: add KeyConfigLastKnownGood for meta/config/last-known-good
  -> Decision: add KeyConfigActiveHash for meta/config/active-hash (optional: may be /perm/ze/ file instead)
- [ ] `pkg/zefs/store.go` - BlobStore with ReadFile, WriteFile, Exists, List
  -> Constraint: runtime reads use store.ReadFile for ZeFS keys
- [ ] `internal/component/config/loader.go` (200L) - LoadConfig parses config string, returns Tree + Plugins
  -> Constraint: pushed config validation uses LoadConfig (same parse + YANG validation as normal config)
- [ ] `internal/component/config/storage/` - Storage interface with Blob and Filesystem backends
  -> Decision: pushed config read/write uses os.ReadFile/os.WriteFile on /perm/ze/ (not ZeFS blob storage)
- [ ] `internal/plugins/bgp/reactor/` - BGP reactor session state management (needed for auto-revert phase)
  -> Constraint: reactor is started inside hub.Run; auto-revert must hook into reactor session state events
  -> Decision: research needed at implementation time to determine subscription mechanism (callback, channel, or event bus)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` (1034L) - cmdStart (line 644-736): resolveStorage(), check managed mode, resolveDefaultConfig(store), check if config exists, bootstrapConfigFromTemplate if not, hub.Run(). No pushed config support yet.
- [ ] `cmd/ze/main.go:991` - bootstrapConfigFromTemplate: reads file/template/ze.conf from ZeFS, runs interface discovery (iface.DiscoverInterfaces), merges template + discovered interfaces, writes to file/active/<configName>.
- [ ] `cmd/ze/main.go:959` - resolveDefaultConfig: reads meta/instance/name from ZeFS; returns `<name>.conf` if valid, or "ze.conf" as fallback. Appliances with identity.name set will use `<name>.conf` as their config name.
- [ ] `cmd/ze/appliance/cmd_assemble.go:72` - assembleZeFS: writes seed config to KeyFileTemplate.Key("ze.conf"). Does NOT write meta/config/last-known-good hash yet.
- [ ] `cmd/ze/appliance/manifest.go:62` - ConfigHash: computes sha256 of seed config string, returns "sha256:<hex>".
- [ ] `pkg/zefs/keys.go` - No KeyConfigLastKnownGood or KeyConfigActiveHash registered yet.
- [ ] `internal/plugins/ntp/ntp.go:49` - Uses /perm/ze/timefile as persistent storage path (precedent for /perm/ze/ usage on device).

**Current config loading flow (device boot):**

| Step | Action | Result |
|------|--------|--------|
| 1 | cmdStart() calls resolveStorage() | BlobStore at {configDir}/database.zefs |
| 2 | isManaged(store)? | If true: cmdStartManaged (separate path) |
| 3 | resolveDefaultConfig(store) | Returns `<name>.conf` or "ze.conf" fallback |
| 4 | store.Exists(configName)? | If yes: skip to step 6 |
| 5 | bootstrapConfigFromTemplate(store, configName) | Reads file/template/ze.conf from ZeFS, runs iface.DiscoverInterfaces(), merges template + discovery, writes to file/active/<configName> |
| 6 | hub.Run(store, configName, ...) | Starts daemon with resolved config |

**No pushed config support exists.** There is no /perm/ze/config-pushed.conf check, no last-known-good hash, no auto-revert.

**Behavior to preserve:**
- First-boot bootstrap (template + discovery) unchanged when no pushed config
- Managed mode path (isManaged -> cmdStartManaged) unchanged
- resolveDefaultConfig returns `<name>.conf` (or "ze.conf" fallback)
- Config parsing via config.LoadConfig unchanged

**Behavior to change:**
- After bootstrap, check /perm/ze/config-pushed.conf; if valid, use it instead of file/active
- assembleZeFS writes meta/config/last-known-good SHA-256 hash into ZeFS
- Register KeyConfigLastKnownGood in pkg/zefs/keys.go
- Write /perm/ze/config-active-hash after loading effective config
- Auto-revert on runtime failure (health check timeout or BGP flap within 30s)

## Data Flow (MANDATORY)

### Entry Point
- Device boot: cmdStart() in cmd/ze/main.go (line 644)
- Config reload: after config-push delivers /perm/ze/config-pushed.conf
- Build-time: assembleZeFS() writes last-known-good hash

### Transformation Path

**Build-time (AC-70):**
1. assembleZeFS() resolves seed config via resolveSeedConfig()
2. Compute SHA-256 of seed config (reuse manifest.ConfigHash logic)
3. Write hash to ZeFS at meta/config/last-known-good (new KeyConfigLastKnownGood)
4. Existing flow continues: write template, secrets, metadata to ZeFS

**Device boot (AC-73, AC-74):**
1. cmdStart() calls resolveStorage(), gets BlobStore
2. Existing: check managed mode, resolveDefaultConfig -> "ze.conf"
3. Existing: if config not in blob, bootstrapConfigFromTemplate (template + discovery -> file/active)
4. NEW: check /perm/ze/config-pushed.conf exists (os.Stat)
5. NEW: if pushed config exists, validate it via config.LoadConfig(pushedData, "", nil)
6. NEW: if valid, use pushed config as effective config (write to file/active in blob store, or pass directly to hub.Run)
7. NEW: if invalid, log warning, delete invalid pushed config, use file/active (seed-derived config)
8. NEW: compute SHA-256 of effective config, write to /perm/ze/config-active-hash
9. hub.Run(store, configName, ...) with effective config

**Auto-revert (AC-71):**
1. After config-push applies new config, set a revert timer (30s)
2. Monitor BGP session state changes during the 30s window
3. If all sessions establish (or remain established): cancel timer, update /perm/ze/last-known-good-pushed
4. If any session flaps within 30s: revert to config-previous.conf (tier 1), or if that also fails, to ZeFS seed config (tier 2)
5. Log revert reason to device log

**Last-known-good update after successful push (AC-72):**
1. Config-push delivers new config, device applies it
2. Health check passes (30s window, no flaps)
3. Write SHA-256 of new config to /perm/ze/last-known-good-pushed
4. New pushed config becomes the tier-1 revert target for next push

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ZeFS file/template/ze.conf -> runtime config | store.ReadFile(KeyFileTemplate.Key("ze.conf")) in bootstrapConfigFromTemplate | [ ] |
| ZeFS meta/config/last-known-good -> hash verification | store.ReadFile(KeyConfigLastKnownGood.Pattern) at boot | [ ] |
| /perm/ze/config-pushed.conf -> runtime config | os.ReadFile + config.LoadConfig validation | [ ] |
| Effective config -> /perm/ze/config-active-hash | os.WriteFile with SHA-256 hex string | [ ] |
| Seed config -> ZeFS meta/config/last-known-good | store.WriteFile at assemble time | [ ] |
| BGP session state -> revert trigger | reactor event subscription (session flap within 30s) | [ ] |

### Integration Points
- `cmd/ze/main.go:cmdStart` - insert pushed config check after bootstrap, before hub.Run
- `cmd/ze/main.go:bootstrapConfigFromTemplate` - unchanged; creates the seed-derived fallback
- `cmd/ze/appliance/cmd_assemble.go:assembleZeFS` - add last-known-good hash write
- `cmd/ze/appliance/manifest.go:ConfigHash` - reuse for last-known-good hash computation
- `pkg/zefs/keys.go` - register KeyConfigLastKnownGood
- `internal/component/config/loader.go:LoadConfig` - validate pushed config
- BGP reactor - subscribe to session state events for auto-revert window

### Architectural Verification
- [ ] No bypassed layers (pushed config goes through config.LoadConfig, same validation as normal config)
- [ ] No unintended coupling (pushed config support in cmdStart only; assemble writes hash but doesn't know about pushed configs)
- [ ] No duplicated functionality (hash computation reuses ConfigHash; validation reuses LoadConfig)
- [ ] First-boot path unchanged (no pushed config -> existing bootstrap flow)
- [ ] Managed mode path unchanged (isManaged check happens before pushed config check)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device boot with seed config only | -> | config loader | `TestBootWithSeedConfigOnly` |
| Device boot with valid pushed config | -> | config loader | `TestBootWithValidPushedConfig` |
| Device boot with invalid pushed config | -> | config loader | `TestBootWithInvalidPushedConfigFallsBack` |
| Config apply triggers revert on failure | -> | health monitor | `TestAutoRevertOnRuntimeFailure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-70 | `ze appliance build lab` produces last-known-good hash | ZeFS contains `meta/config/last-known-good` with SHA-256 of validated seed config |
| AC-71 | `ze appliance config-push lab` with config that passes validation but causes runtime failure | Device detects failure (health check timeout), reverts to last-known-good config from ZeFS seed, prints revert reason to device log |
| AC-72 | `ze appliance config-push lab` updates last-known-good | After device confirms applied config is healthy, device updates /perm/ze/last-known-good with new config hash |
| AC-73 | Device boots with no config-pushed.conf | Uses ZeFS seed config (last-known-good baseline); normal boot path unchanged |
| AC-74 | Device boots with config-pushed.conf that fails validation | Ignores pushed config, uses ZeFS seed config, logs warning |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBootWithSeedConfigOnly` | `cmd/ze/pushed_config_test.go` | AC-73: normal boot with seed config, no pushed config | |
| `TestBootWithValidPushedConfig` | `cmd/ze/pushed_config_test.go` | Pushed config preferred over seed when valid | |
| `TestBootWithInvalidPushedConfigFallsBack` | `cmd/ze/pushed_config_test.go` | AC-74: invalid pushed config deleted, seed config used | |
| `TestLastKnownGoodHashVerification` | `cmd/ze/pushed_config_test.go` | Hash in ZeFS matches seed config SHA-256 at boot | |
| `TestAutoRevertOnRuntimeFailure` | `cmd/ze/health_revert_test.go` | AC-71: BGP flap within 30s triggers revert | |
| `TestConfigActiveHashWritten` | `cmd/ze/pushed_config_test.go` | /perm/ze/config-active-hash updated after boot | |
| `TestBuildWritesLastKnownGood` | `cmd/ze/appliance/cmd_assemble_lkg_test.go` | AC-70: ZeFS contains meta/config/last-known-good after build | |
| `TestAssembleWritesLastKnownGood` | `cmd/ze/appliance/cmd_assemble_lkg_test.go` | ZeFS contains meta/config/last-known-good after assemble | |
| `TestLastKnownGoodHashMatchesSeedConfig` | `cmd/ze/appliance/cmd_assemble_lkg_test.go` | Hash matches SHA-256 of file/template/ze.conf content | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-appliance-boot-seed-only` | `test/appliance/boot-seed-only.ci` | Boot with no pushed config, verify seed config loaded | |
| `test-appliance-boot-pushed-config` | `test/appliance/boot-pushed-config.ci` | Boot with valid pushed config, verify pushed config loaded | |

## Files to Modify

- `cmd/ze/main.go` - cmdStart (line 716-728): insert pushed config check between bootstrap and hub.Run; write config-active-hash after loading effective config
- `cmd/ze/appliance/cmd_assemble.go` - assembleZeFS (line 72): add meta/config/last-known-good hash write after seed config write
- `pkg/zefs/keys.go` - Register KeyConfigLastKnownGood for meta/config/last-known-good
- `internal/plugins/bgp/reactor/` - (Phase 3 only) May need modification to expose session state events for auto-revert. Exact files TBD after research during implementation.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (boot behavior, not config syntax) |
| CLI commands/flags | No | N/A (implicit at boot) |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |
| ZeFS key registration | Yes | `pkg/zefs/keys.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/appliance.md` - device-side config behavior, auto-revert |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - config loading priority |

## Files to Create

- `cmd/ze/appliance/cmd_assemble_lkg_test.go` - Tests for last-known-good hash write in assembleZeFS (AC-70)
- `cmd/ze/pushed_config.go` - Pushed config loading: check /perm/ze/config-pushed.conf, validate, apply or fallback
- `cmd/ze/pushed_config_test.go` - Tests for pushed config loading (AC-73, AC-74)
- `cmd/ze/health_revert.go` - Auto-revert: 30s health window, BGP flap detection, revert to previous/seed
- `cmd/ze/health_revert_test.go` - Tests for auto-revert (AC-71, AC-72)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Last-known-good hash** -- assembleZeFS writes SHA-256 of seed config to meta/config/last-known-good in ZeFS; register KeyConfigLastKnownGood
   - Tests: `TestBuildWritesLastKnownGood`, `TestAssembleWritesLastKnownGood`, `TestLastKnownGoodHashMatchesSeedConfig`
   - Files: `pkg/zefs/keys.go`, `cmd/ze/appliance/cmd_assemble.go`, `cmd/ze/appliance/cmd_assemble_lkg_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Pushed config loading** -- cmdStart checks /perm/ze/config-pushed.conf, validates, uses if valid, falls back to seed if invalid
   - Tests: `TestBootWithSeedConfigOnly`, `TestBootWithValidPushedConfig`, `TestBootWithInvalidPushedConfigFallsBack`, `TestConfigActiveHashWritten`
   - Files: `cmd/ze/pushed_config.go`, `cmd/ze/pushed_config_test.go`, update `cmd/ze/main.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Auto-revert** -- 30s health window after config apply, BGP flap detection triggers revert to previous or seed config
   - Tests: `TestAutoRevertOnRuntimeFailure`, `TestLastKnownGoodHashVerification`
   - Files: `cmd/ze/health_revert.go`, `cmd/ze/health_revert_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -> Create after feature works.
5. **Full verification** -> `make ze-verify` (lint + all ze tests)
6. **Complete spec** -> Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Config loading priority | Pushed config preferred when valid; seed config used as fallback |
| Validation before apply | Invalid pushed config never activates |
| Auto-revert triggers | Health check timeout and BGP flap within 30s both trigger revert |
| Two-tier revert chain | Previous pushed -> seed config fallback order |
| Last-known-good integrity | Hash is SHA-256 of validated seed config |
| No boot regression | Normal boot (no pushed config) unchanged |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Config loading priority logic | Boot test with/without pushed config |
| Auto-revert on failure | Health check failure test |
| Last-known-good hash in ZeFS | `bin/ze data cat --path <db> meta/config/last-known-good` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| **Last-known-good integrity** | Hash in meta/config/last-known-good is SHA-256 of the validated seed config; device trusts this as the fallback |
| **Config push validation** | Device validates pushed config (parse + semantic check) before applying; invalid config never activates |
| **Config push revert safety** | Device saves previous config before applying new; auto-reverts on validation failure; no config loss |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Pushed config at /perm/ze/config-pushed.conf (filesystem, not ZeFS) | /perm/ is gokrazy's persistent partition; survives reboots; ZeFS is read-only after build |
| Validation via config.LoadConfig before applying | Same parse + YANG validation as normal config; no weaker path for pushed configs |
| Two-tier revert: previous pushed -> ZeFS seed | Gives operator one "undo" (config-previous) before falling back to immutable seed |
| 30s health window for auto-revert | Long enough for BGP sessions to establish (typical: 5-10s); short enough to limit damage |
| Last-known-good hash in ZeFS (meta/config/last-known-good) | Immutable after build; device can always verify its seed config integrity |
| config-active-hash at /perm/ze/config-active-hash | Writable; updated each boot; fleet management reads this for drift detection |
| Pushed config check after bootstrap, before hub.Run | Bootstrap creates the fallback; pushed config overrides it; ordering matters |
| Build-time hash uses ConfigHash (same as manifest) | Single hash function; no divergence between build.json and ZeFS hash |

### Config Loading Priority (Device Boot)

| Step | Action | Condition | Result |
|------|--------|-----------|--------|
| 1 | resolveStorage() | | BlobStore |
| 2 | isManaged(store)? | true | cmdStartManaged (unchanged, separate path) |
| 3 | resolveDefaultConfig(store) | | `<name>.conf` or "ze.conf" |
| 4 | store.Exists(configName)? | false | bootstrapConfigFromTemplate [unchanged] |
| 5 | NEW: checkPushedConfig() | /perm/ze/config-pushed.conf exists | Validate via config.LoadConfig |
| 5a | | valid | Use pushed config (write to file/active in store) |
| 5b | | invalid | Log warning, delete invalid pushed config, use file/active |
| 5c | | not present | Use file/active (seed-derived, from bootstrap) |
| 6 | NEW: write config-active-hash | | SHA-256 of effective config to /perm/ze/config-active-hash |
| 7 | hub.Run(store, configName, ...) | | Start daemon with effective config |

### Auto-Revert Mechanism

After config-push delivers a new config and the device applies it:

1. Set a 30-second timer
2. Subscribe to BGP session state change events from the reactor
3. During the 30s window, monitor for:
   - Any BGP session that was Established before the config change and is now not Established (flap)
   - Health check timeout (no BGP sessions establish within 30s)
4. If any trigger fires:
   - Read /perm/ze/config-previous.conf (tier 1 revert target)
   - Validate it; if valid, apply it, log "reverted to previous config: <reason>"
   - If config-previous is also invalid or missing: read ZeFS seed config (tier 2)
   - Apply seed config, log "reverted to seed config: <reason>"
5. If 30s passes without triggers:
   - Config is considered healthy
   - Write SHA-256 of new config to /perm/ze/last-known-good-pushed
   - This becomes the tier-1 revert target for the next push

### Scope

#### In scope (device-side runtime)
- Config loading priority: /perm/ze/config-pushed.conf (if valid) > ZeFS file/template/ze.conf
- Validation of pushed config before applying (parse + semantic check)
- Auto-revert on validation failure: delete invalid pushed config, use seed config
- Auto-revert on runtime failure: health check timeout, BGP session flap within 30s of apply
- Two-tier revert chain: previous pushed config -> ZeFS seed config
- Last-known-good hash verification at boot
- /perm/ze/config-active-hash for fleet drift detection
- Build-time: writing meta/config/last-known-good SHA-256 hash into ZeFS (AC-70)

#### Out of scope
- Bastion-side SSH upload (appliance-2-remote)
- Config drift detection and mandatory resync (ze fleet spec)
- Staged rollout coordination (ze fleet spec)
- Config reload without restart (hot reload is a separate feature)

## Resolved Questions

| # | Question | Answer |
|---|----------|--------|
| Q1 | Where does pushed config live? | /perm/ze/config-pushed.conf (gokrazy persistent partition, survives reboots) |
| Q2 | How does the device validate pushed config? | config.LoadConfig(data, "", nil) -- same validation as interactive config |
| Q3 | What triggers auto-revert? | BGP session flap within 30s of apply, or no sessions establish within 30s |
| Q4 | Does pushed config override managed mode? | No. Managed mode (isManaged check) runs before pushed config check. Managed devices get config from hub, not from push. |
| Q5 | What about the auto-revert mechanism's BGP integration? | Needs research into reactor event subscription. The reactor is started inside hub.Run; revert must hook into the running reactor's session state events. This may require a callback or channel from the reactor. Design will be finalized during implementation. |

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
- [ ] AC-70..AC-74 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
