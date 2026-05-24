# Spec: install-4-ze-install-subcommand

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-install-1, spec-install-2, spec-install-3 |
| Phase | - |
| Updated | 2026-05-24 |
| Parent | spec-install-0-umbrella |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/main.go` - how ze reads stdin config (lines 538-545, looksLikeConfig("-"))
4. `cmd/ze-chaos/main.go` - fork pattern: writeConfig() with NUL sentinel (lines 710-737)
5. `plan/spec-install-0-umbrella.md` - parent spec with architecture decisions

## Task

New `ze install serve` subcommand at `cmd/ze/install/`. A thin CLI that accepts
flags describing a provisioning scenario (interface, network, image, SSH credentials),
generates a ze config string, forks `ze -` (self-fork via `os.Executable()`),
and pipes the config to stdin with a NUL sentinel. Same pattern as
`ze-chaos | ze -` but internal: one command starts everything.

The same provisioning can be achieved with `ze` and a hand-written config;
`ze install serve` makes the common case trivial.

## Required Reading

### Architecture Docs
- [ ] `cmd/ze/main.go` lines 538-545 - stdin config path: `looksLikeConfig("-")` dispatches to `hub.Run(resolveStorage(), "-", ...)`
  -> Decision: ze already reads config from stdin when arg is "-"
  -> Constraint: config piped to stdin, NUL byte marks end of config (ze keeps reading for shutdown signal)
- [ ] `cmd/ze-chaos/main.go` lines 710-737 - writeConfig() pattern: config to stdout + NUL sentinel
  -> Decision: ze install follows same fork pattern as `ze-chaos | ze -`
  -> Constraint: NUL sentinel required so ze can parse config without waiting for EOF
- [ ] `cmd/ze-chaos/main.go` lines 440-449 - auto-discover ze binary via os.Executable()
  -> Decision: ze install finds itself and forks with `exec.Command(self, "-")`
- [ ] `plan/spec-install-0-umbrella.md` - architecture overview, design decisions, data flow
  -> Decision: ze install generates ze config from flags, all protocols are ze plugins
  -> Constraint: server IP derived from interface address unless --address override

### RFC Summaries (MUST for protocol work)
None required directly (protocol work is in child specs 1-3).

**Key insights:**
- ze already handles stdin config via `ze -` (looksLikeConfig returns true for "-")
- ze-chaos writeConfig() pattern: write config + NUL byte to stdout, keep pipe open (EOF = shutdown)
- Self-fork via os.Executable() to find the ze binary (same as ze-chaos auto-discover)
- Server IP can be derived from the interface's first IPv4 address (net.InterfaceByName + Addrs())
- bcrypt hashing for SSH password happens in ze install before embedding in generated config
- No hub.Run() import, no ephemeral zefs: ze handles its own storage when reading stdin config

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go:538-545` - stdin config path: when arg is "-", calls hub.Run(resolveStorage(), "-", ...)
  -> Constraint: ze resolves its own storage; stdin config is just the config content
- [ ] `cmd/ze/main.go:475-504` - "start" subcommand dispatch pattern
  -> Decision: "install" follows same pattern: case in switch, dispatches to package Run()
- [ ] `cmd/ze-chaos/main.go:710-737` - writeConfig(): config + NUL to stdout, pipe stays open
  -> Constraint: NUL sentinel marks end of config; pipe EOF signals shutdown
- [ ] `cmd/ze-chaos/main.go:440-449` - os.Executable() for self-discovery
  -> Decision: ze install finds own binary path for the fork
- [ ] `cmd/ze-chaos/main.go:157-167` - waitForZe() with pipeline mode retry
  -> Decision: ze install uses similar wait logic after forking

**Behavior to preserve:**
- ze stdin config path unchanged; ze install is a launcher, not a modifier
- All plugin behavior (dhcpserver, tftpserver, imageserver) comes from child specs
- ze handles its own storage resolution when reading from stdin

**Behavior to change:**
- New `ze install` subcommand at `cmd/ze/install/` that generates config and forks `ze -`
- Config piped to child process stdin with NUL sentinel (ze-chaos pattern)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- `ze install serve --interface eth0 --image /path/to/img --network 192.168.1.0/24 --ssh-username admin --ssh-password secret`
- CLI flags parsed by standard Go flag package

### Transformation Path
1. Parse CLI flags: interface, image path, network CIDR, SSH credentials, optional address override
2. Resolve server IP: read first IPv4 address from named interface (or use --address override)
3. Validate: interface exists, image file exists, network is valid CIDR, SSH credentials non-empty
4. Hash SSH password with bcrypt
5. Generate ze config string (brace format) with dhcpserver+PXE, tftpserver, imageserver sections
6. Find own binary via `os.Executable()`
7. Create child process: `exec.Command(self, "-")` with StdinPipe
8. Write config + NUL sentinel to child stdin (ze-chaos writeConfig pattern)
9. Forward SIGTERM/SIGINT to child process
10. Wait for child exit; propagate exit code

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI flags -> config string | String builder with flag values | [ ] |
| Config string -> child stdin | StdinPipe + NUL sentinel | [ ] |
| Child ze reads stdin | `looksLikeConfig("-")` -> `hub.Run(store, "-", ...)` | [ ] |
| Signal forwarding | Parent catches SIGTERM/SIGINT, sends to child | [ ] |

### Integration Points
- `os.Executable()` - find ze binary for self-fork
- `exec.Command(self, "-")` - start child ze with stdin config
- `StdinPipe` + NUL sentinel - config delivery (same as ze-chaos)
- dhcpserver plugin (spec-install-1) - started by generated config
- tftpserver plugin (spec-install-2) - started by generated config
- imageserver plugin (spec-install-3) - started by generated config

### Architectural Verification
- [ ] No bypassed layers (config goes through standard ze stdin path)
- [ ] No unintended coupling (ze install is a launcher, not a modifier of ze internals)
- [ ] No duplicated functionality (config generation is new; execution reuses ze stdin path)
- [ ] Zero-copy preserved where applicable (not applicable; config is generated once at startup)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze install serve` CLI invocation | -> | config generation + child ze fork | `TestServeConfigGeneration` |
| `ze install serve --interface eth0 ...` | -> | DHCP+TFTP+HTTP listeners active via forked ze | `test-ze-install-serve` (.ci) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze install serve` with valid flags | Forks `ze -`, DHCP (with PXE), TFTP, and HTTP servers listen on configured interface |
| AC-2 | `ze install serve` without --interface | Error message, exit code 1 |
| AC-3 | `ze install serve` without --image | Error message, exit code 1 |
| AC-4 | `ze install serve` without --network | Error message, exit code 1 |
| AC-5 | `ze install serve` without --ssh-username or --ssh-password | Error message, exit code 1 |
| AC-6 | `ze install serve` with --address override | Server IP uses override instead of interface address |
| AC-7 | `ze install serve` with valid flags | Generated config contains dhcp-server with PXE block, tftp-server, image-server sections |
| AC-8 | `ze install serve` with --ssh-username and --ssh-password | SSH password bcrypt-hashed, embedded in generated config as imageserver ssh-password-hash |
| AC-9 | `ze install` with no subcommand | Usage help printed, exit code 0 |
| AC-10 | `ze install serve` receives SIGTERM/SIGINT | Signal forwarded to child ze, clean shutdown |
| AC-11 | `ze install serve` with non-existent interface | Error message naming the interface, exit code 1 |
| AC-12 | `ze install serve` with non-existent image file | Error message naming the file, exit code 1 |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGenerateConfig` | `cmd/ze/install/config_test.go` | Config string has all required sections (dhcp-server, pxe, tftp-server, image-server) | |
| `TestGenerateConfigPXEBlock` | `cmd/ze/install/config_test.go` | PXE block has tftp-server IP, bootfile-bios, bootfile-uefi | |
| `TestGenerateConfigNetwork` | `cmd/ze/install/config_test.go` | DHCP range derived from network prefix length (scales with subnet size, not hardcoded .100-.200; for small subnets like /28, uses available host range minus gateway) | |
| `TestResolveServerIP` | `cmd/ze/install/config_test.go` | Returns first IPv4 from named interface; override takes precedence | |
| `TestValidateFlags` | `cmd/ze/install/config_test.go` | Missing required flags return descriptive errors | |
| `TestPasswordHashing` | `cmd/ze/install/config_test.go` | Password is bcrypt-hashed, hash verifies against original | |
| `TestGenerateConfigServerIP` | `cmd/ze/install/config_test.go` | Server IP appears in dhcp siaddr, default-router, tftp-server, image-server listen | |
| `TestForkAndPipe` | `cmd/ze/install/serve_test.go` | Config written to pipe with NUL sentinel, child receives valid config | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Network prefix length | /8 - /30 | /30 (4 hosts) | /31 (no pool possible) | /7 (too large) |
| DHCP pool size | 1 - 254 | 254 (.1-.254) | 0 (empty network) | N/A (bounded by /24) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-install-serve` | `test/install/serve.ci` | ze install serve starts all three servers from CLI flags | |
| `test-ze-install-help` | `test/install/help.ci` | ze install with no args prints usage | |
| `test-ze-install-missing-flags` | `test/install/missing-flags.ci` | ze install serve without required flags exits with error | |

### Future (if deferring any tests)
- Full PXE boot integration test (requires QEMU with PXE ROM, depends on installer initrd)

## Files to Modify

None. This is a new subcommand. `cmd/ze/main.go` needs a new case in the dispatch switch.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (uses existing plugin YANG schemas) |
| CLI commands/flags | Yes | `cmd/ze/install/main.go` |
| CLI grammar (action before identifier) | Yes | `install serve` is action-first, no user identifiers in command path |
| Main dispatch | Yes | `cmd/ze/main.go` - add `case "install"` to switch |
| Editor autocomplete | No | N/A (operator tool, not RPC-based) |
| Functional test for new RPC/API | No | N/A (subcommand, not RPC) |
| Doctor check for runtime dependencies | No | N/A (ze install is operator tool, not daemon) |
| Go build integration | No | N/A (part of ze binary, no separate build) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - ze install provisioning |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - ze install serve |
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

- `cmd/ze/install/main.go` - subcommand entry point: Run(), subcommand dispatch, usage
- `cmd/ze/install/serve.go` - serve subcommand: flag parsing, validation, self-fork, stdin pipe
- `cmd/ze/install/config.go` - generateConfig(): builds ze config string from parameters
- `cmd/ze/install/config_test.go` - unit tests for config generation and validation
- `cmd/ze/install/serve_test.go` - unit tests for fork/pipe logic
- `test/install/serve.ci` - functional test: ze install starts all servers
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

1. **Phase: Wiring (MANDATORY FIRST)** -- subcommand skeleton with serve that exits immediately
   - Tests: `ze install serve -h` prints usage
   - Files: `cmd/ze/install/main.go`, `cmd/ze/install/serve.go`, `cmd/ze/main.go` (add case)
   - Verify: ze dispatches to install subcommand

2. **Phase: Config generation** -- generateConfig() builds valid ze config from parameters
   - Tests: `TestGenerateConfig`, `TestGenerateConfigPXEBlock`, `TestGenerateConfigNetwork`, `TestGenerateConfigServerIP`
   - Files: `cmd/ze/install/config.go`, `cmd/ze/install/config_test.go`
   - Verify: generated config string parses as valid ze config with all required sections

3. **Phase: Flag validation** -- validate required flags, resolve server IP
   - Tests: `TestValidateFlags`, `TestResolveServerIP`, `TestPasswordHashing`
   - Files: `cmd/ze/install/serve.go`, `cmd/ze/install/config_test.go`
   - Verify: missing flags produce clear errors, server IP resolved from interface

4. **Phase: Fork and pipe** -- self-fork via os.Executable(), pipe config to child stdin
   - Tests: `TestForkAndPipe`, `TestServeConfigGeneration` (wiring test)
   - Files: `cmd/ze/install/serve.go`, `cmd/ze/install/serve_test.go`
   - Verify: child ze starts with piped config, signal forwarding works, clean shutdown

5. **Functional tests** -- create after feature works
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Generated config matches ze syntax exactly; bcrypt hash is valid |
| Naming | CLI flags use --kebab-case; Go functions use camelCase |
| Data flow | Flags -> config string -> stdin pipe -> child ze chain verified |
| CLI grammar | `install serve` is action-first, no user identifiers in command path |
| Rule: no-sprintf-alloc | Config generation uses strings.Builder, not fmt.Sprintf loops |
| Rule: buffer-first | Not applicable (config generation, not wire encoding) |
| Cleanup | Signal forwarded to child, child exit code propagated |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze install serve -h` prints usage | Run and check output |
| Config generation produces valid ze config | `TestGenerateConfig` passes |
| Server IP resolved from interface | `TestResolveServerIP` passes |
| SSH password bcrypt-hashed | `TestPasswordHashing` passes |
| Missing flags produce clear errors | `TestValidateFlags` passes |
| Fork + pipe delivers config to child ze | `TestForkAndPipe` passes |
| Signal forwarding and clean shutdown | `TestServeConfigGeneration` passes |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Credential handling | SSH password never logged in plaintext; only bcrypt hash embedded in generated config |
| Path traversal | Image path validated: must exist, must be a regular file |
| Config injection | Interface name validated with same character set as `safeEmitName()` in `internal/component/iface/emit.go` (reject `{`, `}`, `;`, whitespace, NUL) before interpolation into config string. Image path validated as existing regular file. |
| Signal handling | SIGTERM/SIGINT forwarded to child ze; no orphan processes |
| Privileged ports | DHCP (67), TFTP (69), HTTP (80) require root on Linux. `ze install` must run as root. On gokrazy, ze runs as root by default. Document in usage/help output. |
| Child process | Child inherits only stdin pipe; stderr/stdout inherited for log visibility. No credential leakage via environment. |

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
- [ ] Feature code integrated (`cmd/ze/install/*` + `cmd/ze/main.go` dispatch)
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
