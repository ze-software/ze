# Spec: ze support (tech-support bundle)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/8 |
| Updated | 2026-05-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - registration pattern, component isolation
4. `cmd/ze/doctor/doctor.go` - doctor check pattern, diagnostic types
5. `internal/component/host/inventory.go` - section detector registry pattern
6. `internal/core/diagnostic/types.go` - DoctorResult, Diagnostic types

## Task

Add `ze support` command that generates a compressed tech-support archive collecting
system state, configuration, health checks, logs, and runtime diagnostics into a
single artifact for support cases or remote troubleshooting.

Inspired by best-in-class NOS implementations (Cumulus `cl-support`, SONiC
`show techsupport`, Arista periodic collection, VyOS archive upload) but with
Ze-specific differentiators: structured JSON per module, registered module discovery,
`ze doctor` integration, privacy-by-default, and time-scoped log collection.

### Vendor Research Summary

| Vendor | Command | Key Strength |
|--------|---------|-------------|
| Cumulus | `cl-support` | Named module system (`-e`/`-d`), privacy-by-default, reason/tag in filename |
| Cisco IOS-XR | `show tech-support [subsystem]` | Custom profiles, subsystem variants, command dedup |
| Juniper | `request support information` | 25+ component selectors, brief mode, platform auto-detection |
| SONiC | `show techsupport` | Auto-trigger on crash, `--since` time scoping, % disk auto-cleanup |
| Arista | `show tech-support` | Periodic baseline every 60min, 100-file rotation |
| VyOS | `generate tech-support archive` | Ticket-number tagging, SFTP upload |
| Nokia | `admin tech-support` | Encrypted output (only TAC can read) |

Gaps no vendor fills: diff against baseline, content-aware anonymization, signed bundles,
JSON-structured archives.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - registration pattern for modules
  -> Decision: modules register via init() like all Ze components
  -> Constraint: no direct imports between modules; use registry
- [ ] `ai/rules/cli-grammar.md` - command naming conventions
  -> Constraint: action before identifier ("generate support" vs "support generate")
  -> Decision: `ze support` is a noun-command like `ze doctor`, not action+identifier
- [ ] `ai/rules/derive-not-hardcode.md` - module list derived from registry
  -> Constraint: help text, validation, module list all derive from single source of truth
- [ ] `ai/rules/json-format.md` - JSON output conventions
  -> Constraint: kebab-case keys in all JSON output
- [ ] `ai/rules/pipe-completeness.md` - pipe operator support
  -> Constraint: command must support all pipe operators

### RFC Summaries (MUST for protocol work)
N/A - no protocol work.

**Key insights:**
- Ze already has building blocks: `ze doctor` (health), `ze host` (hardware), `ze crashes` (crash logs), config loading
- Module registry pattern from `host/inventory.go` (sectionDetectors map) is the model
- `diagnostic.RegisterDoctorProvider(runChecks)` shows how doctor exposes its checks programmatically
- `diagnostic.RunDoctorChecks()` is called by `internal/component/cmd/show/doctor.go` - same pattern for support
- `ze generate` already exists as a root command for artifact generation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/doctor/doctor.go` - runs 30+ health checks, outputs text or JSON, uses diagnostic.Diagnostic
- [ ] `cmd/ze/doctor/register.go` - registers as root "doctor" in SectionSystem, registers DoctorProvider
- [ ] `internal/core/diagnostic/types.go` - DoctorResult{SchemaVersion, Ready, Diagnostics}, Diagnostic struct with Code/Severity/Message
- [ ] `internal/core/diagnostic/doctor_provider.go` - RegisterDoctorProvider/RunDoctorChecks for programmatic access
- [ ] `internal/component/host/inventory.go` - sectionDetectors map, Detect(), SectionNames(), DetectSection()
- [ ] `cmd/ze/host/host.go` - `ze host show [section]` JSON/text output via host.Detect()
- [ ] `cmd/ze/crashes/crashes.go` - `ze crashes show` reads crash files via crashlog package
- [ ] `internal/core/crashlog/` - CrashDir(), ListCrashes(), LatestCrash(), ReadCrash()
- [ ] `internal/core/paths/paths.go` - ConfigDirFromBinary(), DefaultConfigDir()
- [ ] `cmd/ze/diag/register.go` - "generate" root command already registered in SectionSystem

**Behavior to preserve:**
- `ze doctor` remains independent; support calls it, does not replace it
- `ze host show` output format unchanged; support captures its output
- crash log files read directly from disk (offline operation)
- all existing commands continue to work as before
- `ze generate` root keeps its existing subcommands (wireguard keypair)

**Behavior to change:**
- None. This is a new command that composes existing capabilities.

## Data Flow (MANDATORY)

### Entry Point
- User invokes `ze support [flags]`
- Offline command (no daemon required for base modules)
- Some modules (routing, interfaces, plugins) attempt daemon RPC and degrade gracefully

### Transformation Path
1. Parse flags: modules to include/exclude, time scope, reason, sensitive mode
2. Resolve module list from registry (all, or filtered by --module/--exclude)
3. Create temp directory for staging
4. Run each module collector in sequence, writing output to temp dir
5. Write manifest.json with metadata (version, timestamp, hostname, reason, modules, durations)
6. Create tar.gz archive from temp dir
7. Move archive to output directory
8. Clean up temp dir
9. Print archive path (text mode) or manifest JSON (--json mode)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| support -> doctor | `diagnostic.RunDoctorChecks(configPath)` via registered provider | [ ] |
| support -> host | `host.Detect()` library call | [ ] |
| support -> crashlog | `crashlog.ListCrashes()`, `crashlog.ReadCrash()` | [ ] |
| support -> config | `config.LoadConfig()` + sanitization walker | [ ] |
| support -> daemon | RPC via unix socket (optional, graceful degradation) | [ ] |
| support -> system | exec for journalctl, nft, ip, vppctl (with timeout) | [ ] |

### Integration Points
- `diagnostic.RunDoctorChecks()` - programmatic doctor access (already exists, used by show/doctor.go)
- `host.Detect()` / `host.DetectSection()` - hardware inventory (already exists)
- `crashlog.ListCrashes()` / `crashlog.ReadCrash()` - crash files (already exists)
- `config.LoadConfig()` - config parsing (already exists)
- `cmdregistry.RegisterRoot()` - command registration (already exists)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze support` CLI invocation | -> | `support.Run()` | `TestSupportCommand_ProducesArchive` |
| `ze support --module doctor` | -> | doctor module collector | `TestModuleSelection_DoctorOnly` |
| `ze support --list-modules` | -> | module registry listing | `TestListModules_DerivedFromRegistry` |
| `ze support --json` | -> | manifest JSON output | `TestJSONOutput_ManifestToStdout` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze support` with valid config | Produces `ze-support-<hostname>-<timestamp>.tar.gz` containing all module outputs |
| AC-2 | `ze support --module doctor,host` | Archive contains only doctor.json and host.json (plus manifest) |
| AC-3 | `ze support --exclude logs` | Archive contains all modules except logs |
| AC-4 | `ze support --list-modules` | Prints module names derived from registry, one per line |
| AC-5 | `ze support --json` | Outputs manifest JSON to stdout (archive path, modules collected, durations) |
| AC-6 | `ze support --since 2h` | Log module collects only entries from last 2 hours |
| AC-7 | `ze support --reason "BGP flap"` | Reason string appears in manifest.json |
| AC-8 | `ze support` (default, no --sensitive) | Config output has passwords/secrets/keys redacted |
| AC-9 | `ze support --sensitive` | Config output includes all values unredacted |
| AC-10 | `ze support` when daemon is not running | Completes successfully; daemon-dependent modules produce degraded output with explanation |
| AC-11 | `ze support --output /tmp/` | Archive written to specified directory |
| AC-12 | Module fails (e.g., journalctl not found) | Module produces error entry in its output, other modules unaffected, archive still created |
| AC-13 | `ze support` on non-Linux (macOS) | Runs with reduced module set (no kernel, vpp, ipsec); does not crash |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModuleRegistry_AllRegistered` | `cmd/ze/support/support_test.go` | All modules present in registry | |
| `TestModuleRegistry_NoDuplicates` | `cmd/ze/support/support_test.go` | No duplicate module names | |
| `TestModuleSelection_IncludeFilter` | `cmd/ze/support/support_test.go` | --module flag filters correctly | |
| `TestModuleSelection_ExcludeFilter` | `cmd/ze/support/support_test.go` | --exclude flag filters correctly | |
| `TestModuleSelection_InvalidName` | `cmd/ze/support/support_test.go` | Unknown module name returns error | |
| `TestManifest_Structure` | `cmd/ze/support/support_test.go` | Manifest JSON has required fields | |
| `TestManifest_ReasonIncluded` | `cmd/ze/support/support_test.go` | --reason appears in manifest | |
| `TestConfigSanitizer_PasswordsRedacted` | `cmd/ze/support/sanitize_test.go` | password/secret/key leaves masked | |
| `TestConfigSanitizer_SensitiveMode` | `cmd/ze/support/sanitize_test.go` | --sensitive preserves all values | |
| `TestArchiveStructure_ModuleFiles` | `cmd/ze/support/support_test.go` | tar.gz contains expected files | |
| `TestTimeParsing_RelativeDuration` | `cmd/ze/support/support_test.go` | "2h", "30m", "1d" parsed correctly | |
| `TestTimeParsing_AbsoluteDate` | `cmd/ze/support/support_test.go` | ISO date parsed correctly | |
| `TestListModules_DerivedFromRegistry` | `cmd/ze/support/support_test.go` | Output matches registry keys | |
| `TestGracefulDegradation_DaemonDown` | `cmd/ze/support/support_test.go` | Daemon modules produce error, not crash | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| --since duration | 1s to 8760h | 8760h (1 year) | 0s | N/A (clamped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-support-basic` | `test/support/basic.ci` | `ze support` produces valid tar.gz with manifest | |
| `test-support-module-filter` | `test/support/module-filter.ci` | `ze support --module doctor` produces archive with only doctor output | |
| `test-support-json` | `test/support/json-output.ci` | `ze support --json` outputs valid JSON manifest | |
| `test-support-sanitized` | `test/support/sanitized.ci` | Default mode redacts passwords from config in archive | |
| `test-support-list` | `test/support/list-modules.ci` | `ze support --list-modules` lists all registered modules | |

### Interop Tests
N/A - no protocol features.

### Future (if deferring any tests)
- Content-aware IP/ASN anonymization (separate spec)
- Signed/authenticated bundles (separate spec)
- Auto-trigger on crash (needs daemon integration, separate spec)
- Periodic baseline collection (needs scheduler, separate spec)
- Remote upload to SCP/SFTP/HTTP (needs config for destinations, separate spec)
- Disk budget enforcement (max archive size with warnings)

## Files to Modify
- `internal/core/diagnostic/types.go` - add SupportManifest, SupportModuleResult types

## Files to Create
- `cmd/ze/support/register.go` - command registration in SectionSystem
- `cmd/ze/support/support.go` - Run() entry point, module orchestration, archive creation
- `cmd/ze/support/modules.go` - module registry (map of name -> collector func)
- `cmd/ze/support/sanitize.go` - config tree sanitizer (password/secret/key redaction)
- `cmd/ze/support/modules_linux.go` - linux-only modules (kernel, vpp, ipsec, firewall)
- `cmd/ze/support/modules_other.go` - stub for non-linux (returns empty with explanation)
- `cmd/ze/support/support_test.go` - unit tests
- `cmd/ze/support/sanitize_test.go` - sanitizer tests
- `test/support/basic.ci` - functional test
- `test/support/module-filter.ci` - functional test
- `test/support/json-output.ci` - functional test
- `test/support/sanitized.ci` - functional test
- `test/support/list-modules.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (offline command, no YANG) |
| CLI commands/flags | Yes | `cmd/ze/support/register.go` |
| CLI grammar (action before identifier) | Yes | `ze support` is noun-form like `ze doctor` |
| Editor autocomplete | No | Offline command, not in CLI editor |
| Functional test for new RPC/API | Yes | `test/support/*.ci` |
| Doctor check for runtime dependencies | No | No new runtime dependencies; uses existing paths |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/support-bundle.md` (new) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (Ze vs VyOS/SONiC) |
| 12 | Internal architecture changed? | No | - |

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

1. **Phase: Wiring (MANDATORY FIRST)** - register entry points, write failing wiring tests
   - Tests: `TestSupportCommand_ProducesArchive`, `TestListModules_DerivedFromRegistry`
   - Files: `register.go`, `support.go` skeleton, `modules.go` with empty registry
   - Verify: `ze support` is reachable; wiring test fails because module logic is stub

2. **Phase: Module Registry** - module registration pattern, selection, listing
   - Tests: `TestModuleRegistry_AllRegistered`, `TestModuleRegistry_NoDuplicates`, `TestModuleSelection_*`
   - Files: `modules.go` - module map type, registration, filtering
   - Verify: --module/--exclude/--list-modules work; collectors are stubs returning empty

3. **Phase: Core Modules** - version, doctor, host, crashes, config, disk
   - Tests: `TestArchiveStructure_ModuleFiles`, `TestManifest_Structure`
   - Files: `support.go` - collector implementations for offline modules
   - Verify: archive contains JSON output from each module; manifest has durations

4. **Phase: Config Sanitization** - password/secret/key redaction
   - Tests: `TestConfigSanitizer_*`
   - Files: `sanitize.go` - tree walker that masks sensitive leaves
   - Verify: default mode redacts; --sensitive preserves

5. **Phase: System Modules (linux)** - logs (with --since), kernel, firewall, vpp, ipsec
   - Tests: `TestTimeParsing_*`, `TestGracefulDegradation_DaemonDown`
   - Files: `modules_linux.go` - system command collectors with timeout and graceful degradation
   - Files: `modules_other.go` - stubs for non-linux
   - Verify: modules degrade gracefully on non-Linux / when tools missing

6. **Phase: Daemon Modules** - routing, interfaces, plugins (RPC-dependent)
   - Tests: `TestGracefulDegradation_DaemonDown`
   - Files: `support.go` - daemon RPC collectors with fallback
   - Verify: works when daemon running; degrades when not

7. **Functional tests** - end-to-end CI tests
   - Tests: all `.ci` files from TDD plan
   - Files: `test/support/*.ci`
   - Verify: all functional tests pass

8. **Full verification** - `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Archive is valid tar.gz; manifest JSON is well-formed; sanitization covers all sensitive leaf types |
| Naming | JSON keys use kebab-case; module names are lowercase single words; archive filename follows pattern |
| Data flow | Modules are independent; failure in one does not affect others; temp dir always cleaned up |
| CLI grammar | `ze support` registered as noun-form root command in SectionSystem |
| Rule: derive-not-hardcode | Module list, help text, validation all derive from module registry map |
| Rule: no-sprintf-alloc | No fmt.Sprintf on collection hot paths |
| Rule: pipe-completeness | All pipe operators supported |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze support` produces archive | `ze support && tar tzf ze-support-*.tar.gz` |
| Archive contains manifest.json | `tar xf ... manifest.json && jq . manifest.json` |
| Module filtering works | `ze support --module doctor && tar tzf ... \| grep -c json` |
| --list-modules prints all modules | `ze support --list-modules \| wc -l` matches registry size |
| --json outputs manifest | `ze support --json \| jq .modules` |
| Config sanitized by default | `tar xf ... config.json && grep -c 'REDACTED' config.json` |
| Functional tests exist | `ls test/support/*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | --module/--exclude values validated against registry; --since parsed safely; --reason length-bounded; --output path validated (no traversal) |
| Sensitive data | Default mode must redact: password, secret, pre-shared-key, private-key, community (SNMP), shared-secret (TACACS/RADIUS) |
| Temp file safety | Temp dir created with restrictive permissions (0700); cleaned up on all exit paths including panic |
| Command injection | External commands (journalctl, nft, ip, vppctl) use exec.Command with args, never shell interpolation |
| Resource exhaustion | Timeout on external commands; size cap on log collection; bounded reason string length |
| Archive permissions | Output file created with 0600 (owner-read-write only) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
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

### Module Pattern Decision
Chose Cumulus-style named modules over Cisco-style subsystem variants. Reason: Ze's
registration pattern (single map, derive everything) maps directly to named modules.
Cumulus `-e`/`-d` flags map to `--module`/`--exclude`. SONiC's `--since` is orthogonal
and applies only to the logs module.

### Privacy Model Decision
Chose Cumulus's "exclude by default, opt-in with flag" over Nokia's "encrypted binary"
or VyOS's "claimed sanitization." Reason: operators need to read the bundle for
self-service troubleshooting. Encryption prevents that. But default-safe means
accidental leaks are harder.

### Structured JSON Decision
No NOS vendor produces machine-parseable per-module output. All produce text dumps
or opaque binaries. Ze's JSON-per-module approach enables automated analysis pipelines
that no other NOS supports. This is the primary differentiator.

### Deferred Features (each needs own spec)
- Auto-trigger on crash: needs daemon-side hook
- Periodic baselines: needs cron/scheduler infrastructure
- Remote upload: needs YANG config for destinations
- Content-aware anonymization: complex (IP ranges, ASNs, hostnames while preserving structure)
- Signed bundles: needs crypto design for non-repudiation
- Diff against baseline: needs baseline storage and comparison logic

## RFC Documentation
N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [to be filled after implementation]

### Bugs Found/Fixed
- [to be filled]

### Documentation Updates
- [to be filled]

### Deviations from Plan
- [to be filled]

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Single command produces support archive | functional test | `test-support-basic` |
| Module selection works | functional test | `test-support-module-filter` |
| JSON manifest for automation | functional test | `test-support-json` |
| Config sanitized by default | functional test | `test-support-sanitized` |
| Module discovery from registry | functional test | `test-support-list` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [to be filled]

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`cmd/ze/support/`, `internal/core/diagnostic/`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Interop tests for protocol features (or N/A: no protocol work)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/789-support.md`
- [ ] Summary included in commit
