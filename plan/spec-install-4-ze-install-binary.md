# Spec: install-4-ze-install-binary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-install-1, spec-install-2, spec-install-3 |
| Phase | - |
| Updated | 2026-05-21 |
| Parent | spec-install-0-umbrella |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/hub/main.go` - hub.Run() signature and startup flow
4. `cmd/ze/main.go` - how ze calls hub.Run() with storage
5. `internal/component/config/storage/blob.go` - Storage interface and constructors
6. `plan/spec-install-0-umbrella.md` - parent spec with architecture decisions

## Task

New `ze-install` binary at `cmd/ze-install/`. A thin CLI that accepts flags
describing a provisioning scenario (interface, network, image, SSH credentials),
generates a ze config in memory, and calls `hub.Run()` to start dhcpserver
(with PXE), tftpserver, and imageserver plugins. The same provisioning can be
achieved with `ze start` and a hand-written config; `ze-install` makes the
common case trivial.

## Required Reading

### Architecture Docs
- [ ] `cmd/ze/hub/main.go` - hub.Run() signature, storage requirements
  -> Decision: hub.Run() takes storage.Storage, configPath, plugins list, chaos params, web/mcp params
  -> Constraint: ze-install must provide a valid Storage and write config into it before calling Run()
- [ ] `cmd/ze/main.go` lines 764-806 - startup path showing hub.Run() call patterns
  -> Decision: storage resolved first, config read from storage by hub.Run()
  -> Constraint: storage must be blob storage for hub.Run() to function (IsBlobStorage check)
- [ ] `cmd/ze-perf/main.go` - thin binary pattern: main() calls os.Exit(run(args)), subcommand dispatch
  -> Decision: ze-install follows same pattern: main.go with subcommand dispatch
- [ ] `internal/component/config/storage/blob.go` - NewBlob() constructor, zefs.Create/Open
  -> Constraint: ze-install needs temp dir for zefs.Create(), cleaned up on exit
- [ ] `plan/spec-install-0-umbrella.md` - architecture overview, design decisions, data flow
  -> Decision: ze-install generates ze config from flags, all protocols are ze plugins
  -> Constraint: server IP derived from interface address unless --address override

### RFC Summaries (MUST for protocol work)
None required directly (protocol work is in child specs 1-3).

**Key insights:**
- hub.Run() reads config from storage.Storage, so ze-install must create a temp zefs, write the generated config into it, then call hub.Run()
- ze-perf pattern: main.go with os.Exit(run(args)), subcommand switch
- Server IP can be derived from the interface's first IPv4 address (net.InterfaceByName + Addrs())
- bcrypt hashing for SSH password must happen in ze-install before writing to zefs
- The generated config is standard ze brace format, written as a blob entry

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go:136` - `func Run(store storage.Storage, configPath string, plugins []string, ...)`
  -> Constraint: hub.Run reads config via store.ReadFile(configPath), expects valid ze config content
- [ ] `cmd/ze/main.go:764-806` - startup path: resolveStorage(), hub.Run() call
  -> Constraint: storage must be blob storage (IsBlobStorage check at line 767)
- [ ] `cmd/ze-perf/main.go` - thin binary pattern with subcommand dispatch
  -> Decision: same pattern for ze-install
- [ ] `internal/component/config/storage/blob.go:41` - NewBlob(blobPath, configDir) constructor
  -> Constraint: requires a directory path for zefs database file

**Behavior to preserve:**
- hub.Run() interface unchanged; ze-install is a caller, not a modifier
- All plugin behavior (dhcpserver, tftpserver, imageserver) comes from child specs
- Storage interface unchanged

**Behavior to change:**
- New binary `cmd/ze-install/` that generates config and calls hub.Run()
- Ephemeral temp zefs created for the server's lifetime

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze-install serve --interface eth0 --image /path/to/img --network 192.168.1.0/24 --ssh-username admin --ssh-password secret`
- CLI flags parsed by standard Go flag package

### Transformation Path
1. Parse CLI flags: interface, image path, network CIDR, SSH credentials, optional address override
2. Resolve server IP: read first IPv4 address from named interface (or use --address override)
3. Validate: interface exists, image file exists, network is valid CIDR, SSH credentials non-empty
4. Hash SSH password with bcrypt
5. Generate ze config string (brace format) with dhcpserver+PXE, tftpserver, imageserver sections
6. Create temp directory, call zefs.Create() to make ephemeral blob storage
7. Write generated config into storage as active config
8. Write SSH credentials into zefs using exact key structure: `KeySSHUsername.Key("127.0.0.1", "2222")` = username, `KeySSHPassword.Key("127.0.0.1", "2222")` = bcrypt hash, `KeySSHDefault.Pattern` = `"127.0.0.1/2222"` (matching `ze init` defaults and `loadZefsUsers()` expectations)
9. Call hub.Run() with the ephemeral storage and config path
10. On shutdown (signal): hub.Run() returns, temp directory cleaned up

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI flags -> config string | String template/builder with flag values | [ ] |
| Config string -> storage | store.WriteFile(activeKey, configBytes, 0) | [ ] |
| Storage -> hub.Run() | hub.Run(store, configPath, ...) | [ ] |
| SSH creds -> zefs | store.WriteFile(zefs keys for username/password) | [ ] |

### Integration Points
- `hub.Run()` in `cmd/ze/hub/main.go` - the main execution entry point
- `storage.NewBlob()` in `internal/component/config/storage/blob.go` - ephemeral storage creation
- `zefs.Create()` in `internal/component/config/zefs/` - blob database creation
- dhcpserver plugin (spec-install-1) - started by generated config
- tftpserver plugin (spec-install-2) - started by generated config
- imageserver plugin (spec-install-3) - started by generated config

### Architectural Verification
- [ ] No bypassed layers (config goes through standard storage -> hub.Run() path)
- [ ] No unintended coupling (ze-install is a caller of hub, not a modifier)
- [ ] No duplicated functionality (config generation is new; execution reuses hub.Run())
- [ ] Zero-copy preserved where applicable (not applicable; config is generated once at startup)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-install serve` CLI invocation | -> | config generation + hub.Run() start | `TestServeConfigGeneration` |
| `ze-install serve --interface eth0 ...` | -> | DHCP+TFTP+HTTP listeners active | `test-ze-install-serve` (.ci) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-install serve` with valid flags | DHCP (with PXE), TFTP, and HTTP servers listen on configured interface |
| AC-2 | `ze-install serve` without --interface | Error message, exit code 1 |
| AC-3 | `ze-install serve` without --image | Error message, exit code 1 |
| AC-4 | `ze-install serve` without --network | Error message, exit code 1 |
| AC-5 | `ze-install serve` without --ssh-username or --ssh-password | Error message, exit code 1 |
| AC-6 | `ze-install serve` with --address override | Server IP uses override instead of interface address |
| AC-7 | `ze-install serve` with valid flags | Generated config contains dhcp-server with PXE block, tftp-server, image-server sections |
| AC-8 | `ze-install serve` with --ssh-username and --ssh-password | imageserver serves /install/database.zefs containing bcrypt-hashed credentials |
| AC-9 | `ze-install` with no subcommand | Usage help printed, exit code 0 |
| AC-10 | `ze-install serve` receives SIGTERM/SIGINT | Clean shutdown, temp directory removed |
| AC-11 | `ze-install serve` with non-existent interface | Error message naming the interface, exit code 1 |
| AC-12 | `ze-install serve` with non-existent image file | Error message naming the file, exit code 1 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGenerateConfig` | `cmd/ze-install/config_test.go` | Config string has all required sections (dhcp-server, pxe, tftp-server, image-server) | |
| `TestGenerateConfigPXEBlock` | `cmd/ze-install/config_test.go` | PXE block has tftp-server IP, bootfile-bios, bootfile-uefi | |
| `TestGenerateConfigNetwork` | `cmd/ze-install/config_test.go` | DHCP range derived from network prefix length (scales with subnet size, not hardcoded .100-.200; for small subnets like /28, uses available host range minus gateway) | |
| `TestResolveServerIP` | `cmd/ze-install/config_test.go` | Returns first IPv4 from named interface; override takes precedence | |
| `TestValidateFlags` | `cmd/ze-install/config_test.go` | Missing required flags return descriptive errors | |
| `TestPasswordHashing` | `cmd/ze-install/config_test.go` | Password is bcrypt-hashed, hash verifies against original | |
| `TestGenerateConfigServerIP` | `cmd/ze-install/config_test.go` | Server IP appears in siaddr, option 54, default-router, tftp-server, image-server | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Network prefix length | /8 - /30 | /30 (4 hosts) | /31 (no pool possible) | /7 (too large) |
| DHCP pool size | 1 - 254 | 254 (.1-.254) | 0 (empty network) | N/A (bounded by /24) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-install-serve` | `test/install/serve.ci` | ze-install starts all three servers from CLI flags | |
| `test-ze-install-help` | `test/install/help.ci` | ze-install with no args prints usage | |
| `test-ze-install-missing-flags` | `test/install/missing-flags.ci` | ze-install serve without required flags exits with error | |

### Future (if deferring any tests)
- Full PXE boot integration test (requires QEMU with PXE ROM, depends on installer initrd)

## Files to Modify

None. This is a new binary. `cmd/ze/hub/main.go` is a read-only reference (hub.Run() signature).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (uses existing plugin YANG schemas) |
| CLI commands/flags | Yes | `cmd/ze-install/main.go` |
| CLI grammar (action before identifier) | Yes | `serve` is the action, no user identifiers in command path |
| Editor autocomplete | No | N/A (standalone binary) |
| Functional test for new RPC/API | No | N/A (binary, not RPC) |
| Doctor check for runtime dependencies | No | N/A (ze-install is operator tool, not daemon) |
| Go build integration | Yes | Makefile target for `ze-install` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - ze-install provisioning |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - ze-install serve |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A (plugins are child specs) |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` - provisioning guide |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `cmd/ze-install/main.go` - binary entry point, subcommand dispatch, usage
- `cmd/ze-install/serve.go` - serve subcommand: flag parsing, validation, config generation, hub.Run()
- `cmd/ze-install/config.go` - generateConfig(): builds ze config string from parameters
- `cmd/ze-install/config_test.go` - unit tests for config generation and validation
- `test/install/serve.ci` - functional test: ze-install starts all servers
- `test/install/help.ci` - functional test: usage output
- `test/install/missing-flags.ci` - functional test: missing flags error

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella spec |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table -- binary builds and starts hub |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-13. Fix/verify loop | Per phase |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- binary skeleton with serve subcommand that exits immediately
   - Tests: `go build ./cmd/ze-install/` succeeds, `ze-install serve -h` prints usage
   - Files: `cmd/ze-install/main.go`, `cmd/ze-install/serve.go`
   - Verify: binary compiles and dispatches to serve subcommand

2. **Phase: Config generation** -- generateConfig() builds valid ze config from parameters
   - Tests: `TestGenerateConfig`, `TestGenerateConfigPXEBlock`, `TestGenerateConfigNetwork`, `TestGenerateConfigServerIP`
   - Files: `cmd/ze-install/config.go`, `cmd/ze-install/config_test.go`
   - Verify: generated config string parses as valid ze config with all required sections

3. **Phase: Flag validation** -- validate required flags, resolve server IP
   - Tests: `TestValidateFlags`, `TestResolveServerIP`, `TestPasswordHashing`
   - Files: `cmd/ze-install/serve.go`, `cmd/ze-install/config_test.go`
   - Verify: missing flags produce clear errors, server IP resolved from interface

4. **Phase: Ephemeral storage and hub integration** -- create temp zefs, write config, call hub.Run()
   - Tests: `TestServeConfigGeneration` (wiring test)
   - Files: `cmd/ze-install/serve.go`
   - Verify: hub.Run() starts with generated config, cleanup on exit

5. **Functional tests** -- create after feature works
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Generated config matches ze syntax exactly; bcrypt hash is valid |
| Naming | CLI flags use --kebab-case; Go functions use camelCase |
| Data flow | Flags -> config string -> storage -> hub.Run() chain verified |
| CLI grammar | `serve` is action keyword before any arguments |
| Rule: no-sprintf-alloc | Config generation uses strings.Builder, not fmt.Sprintf loops |
| Rule: buffer-first | Not applicable (config generation, not wire encoding) |
| Cleanup | Temp directory removed on normal exit and signal |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze-install` binary compiles | `go build ./cmd/ze-install/` |
| `ze-install serve -h` prints usage | Run and check output |
| Config generation produces valid ze config | `TestGenerateConfig` passes |
| Server IP resolved from interface | `TestResolveServerIP` passes |
| SSH password bcrypt-hashed | `TestPasswordHashing` passes |
| Missing flags produce clear errors | `TestValidateFlags` passes |
| Ephemeral zefs created and cleaned up | `TestServeConfigGeneration` passes |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Credential handling | SSH password never logged in plaintext; only bcrypt hash stored in zefs |
| Path traversal | Image path validated: must exist, must be a regular file |
| Temp directory | Created with restrictive permissions (0700); cleaned up on all exit paths |
| Config injection | Interface name validated with same character set as `safeEmitName()` in `internal/component/iface/emit.go` (reject `{`, `}`, `;`, whitespace, NUL) before interpolation into config string. Image path validated as existing regular file. |
| Signal handling | SIGTERM/SIGINT trigger clean shutdown; no orphan listeners |
| Privileged ports | DHCP (67), TFTP (69), HTTP (80) require root on Linux. `ze-install` must run as root. On gokrazy, ze runs as root by default. Document in usage/help output. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
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

Not applicable for this spec (protocol RFC handling is in child specs 1-3).

## Implementation Summary

### What Was Implemented
- [Umbrella: see implementation]

### Bugs Found/Fixed
- [None yet]

### Documentation Updates
- [None yet]

### Deviations from Plan
- [None yet]

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze-install/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
