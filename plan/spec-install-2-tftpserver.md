# Spec: install-2-tftpserver

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-21 |
| Parent | spec-install-0-umbrella.md |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/dhcpserver/register.go` - plugin registration pattern reference
4. `internal/plugins/dhcpserver/handler.go` - buffer-first packet building reference
5. `internal/plugins/dhcpserver/schema/` - YANG schema + embed + register pattern
6. `rfc/short/rfc1350.md` - TFTP protocol spec

## Task

New read-only TFTP server plugin at `internal/plugins/tftpserver/`. Implements
RFC 1350 (TFTP Revision 2): accepts RRQ (read request), serves files in 512-byte
DATA blocks with stop-and-wait ACK, rejects WRQ (write request) with ERROR.
Used by PXE clients to fetch bootloader binaries during provisioning.

Follows the standard ze plugin registration pattern (see dhcpserver as reference).
YANG-configured under `service { tftp-server { ... } }`.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/dhcpserver/register.go` - plugin registration pattern
  -> Decision: registry.Registration{Name, Description, Features:"yang", YANG, ConfigRoots, InProcessConfigVerifier, RunEngine} registered in init()
  -> Constraint: ConfigRoots must be []string{"service"} for service-level plugins
- [ ] `internal/plugins/dhcpserver/handler.go` - buffer-first packet building reference
  -> Constraint: packet building uses direct byte manipulation, no fmt.Sprintf on wire path
- [ ] `internal/plugins/dhcpserver/schema/embed.go` - YANG embed pattern
  -> Decision: //go:embed loads .yang file into exported string var
- [ ] `internal/plugins/dhcpserver/schema/register.go` - YANG module registration
  -> Decision: init() calls yang.RegisterModule("filename.yang", embeddedVar)
- [ ] `internal/plugins/dhcpserver/config.go` - config parsing from JSON tree
  -> Constraint: config arrives as JSON string via sdk.ConfigSection.Data, parsed via json.Unmarshal then navigated as map[string]any

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc1350.md` - TFTP protocol (MUST CREATE before implementation)
  -> Constraint: RRQ/WRQ format, DATA/ACK/ERROR opcodes, 512-byte blocks, stop-and-wait

**Key insights:**
- TFTP is simple: 5 opcodes (RRQ=1, WRQ=2, DATA=3, ACK=4, ERROR=5)
- Read-only: WRQ rejected with ERROR opcode 4 ("Illegal TFTP operation")
- Each DATA block is 512 bytes; final block is < 512 bytes (signals end of transfer)
- Block numbers start at 1, 16-bit unsigned (wraps at 65535)
- Each packet acknowledged individually (stop-and-wait)
- Uses UDP; server listens on port 69, then creates a new TID (ephemeral port) per transfer
- Plugin pattern: init() registers, RunEngine receives net.Conn for plugin SDK communication

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/register.go` - plugin lifecycle: init() registration, RunEngine, OnConfigure callback
  -> Constraint: RunEngine signature is func(net.Conn) int
  -> Constraint: sdk.NewWithConn creates plugin handle, p.Run(ctx, Registration{...}) is the event loop
- [ ] `internal/plugins/dhcpserver/handler.go` - buffer-first DHCP packet building
  -> Constraint: direct byte slice manipulation, safeAppendOption pattern for bounded writes
- [ ] `internal/plugins/dhcpserver/config.go` - config parsing from sdk.ConfigSection.Data
  -> Constraint: root key navigation: root["service"].(map[string]any)["dhcp-server"]
- [ ] `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - YANG schema structure
  -> Constraint: top-level container is "service", plugin container nested inside

**Behavior to preserve:**
- No existing tftpserver code; nothing to preserve
- dhcpserver plugin behavior unchanged (TFTP is a separate plugin)
- Plugin registration pattern followed exactly

**Behavior to change:**
- New plugin: tftpserver registered in plugin registry
- New YANG module: ze-tftp-server-conf under service container
- New UDP listener on port 69 for TFTP protocol

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- UDP packet on port 69 (TFTP well-known port)
- Packet format: 2-byte opcode + variable payload per RFC 1350

### Transformation Path
1. UDP listener receives packet on port 69
2. Parse opcode (first 2 bytes, big-endian)
3. RRQ (opcode 1): extract filename (NUL-terminated string after opcode), mode (NUL-terminated); reject if mode != "octet" (ERROR code 4)
4. Validate filename: filepath.Clean, reject traversal outside root-directory; filepath.EvalSymlinks on resolved path, re-check still under root-directory
5. Open file from root-directory, read first 512-byte block
6. Create new UDP "connection" (ephemeral port) for this transfer (RFC 1350 Section 4)
7. Send DATA packet (opcode 3, block 1, up to 512 bytes data) from ephemeral port
8. Wait for ACK (opcode 4, block 1) from client
9. Repeat: read next block, send DATA, wait ACK, until block < 512 bytes
10. WRQ (opcode 2): respond with ERROR (opcode 5, code 4 "Illegal TFTP operation")

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> tftpserver plugin | UDP on port 69 (plugin's own listener) | [ ] |
| Plugin -> filesystem | os.Open + io.ReadFull from root-directory | [ ] |
| Plugin -> config | YANG section for tftp-server parsed from sdk.ConfigSection | [ ] |

### Integration Points
- `registry.Register()` - standard plugin registration in init()
- `sdk.NewWithConn()` / `p.Run()` - plugin SDK lifecycle
- `yang.RegisterModule()` - YANG schema registration for config editor
- `make generate` - adds tftpserver to plugin all.go (code-generated blank import)

### Architectural Verification
- [ ] No bypassed layers (TFTP server is a self-contained plugin)
- [ ] No unintended coupling (does not import dhcpserver or imageserver)
- [ ] No duplicated functionality (first TFTP implementation in ze)
- [ ] Zero-copy preserved where applicable (file read into reusable buffer, direct UDP write)

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config with tftp-server enabled | -> | plugin registers, starts listener | `TestTFTPServerRegistration` |
| UDP RRQ packet to listener | -> | `handleRRQ` reads file, sends DATA blocks | `TestTFTPReadRequest` |
| UDP WRQ packet to listener | -> | `handleWRQ` returns ERROR | `TestTFTPWriteRejected` |
| Functional: TFTP client fetches file | -> | full transfer via UDP | `test-tftp-read` in `test/install/tftp-boot.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | tftpserver plugin enabled in config with valid root-directory | Plugin starts, UDP listener binds to port 69 on configured interface(s) |
| AC-2 | TFTP RRQ for existing file in root-directory | File served in 512-byte DATA blocks with correct block numbers, transfer completes when final block < 512 bytes |
| AC-3 | TFTP RRQ for non-existent file | ERROR packet returned (code 1 "File not found") |
| AC-4 | TFTP RRQ with path traversal attempt (e.g., `../etc/passwd`) or symlink pointing outside root-directory | ERROR packet returned (code 2 "Access violation"), file not served |
| AC-5 | TFTP WRQ (write request) | ERROR packet returned (code 4 "Illegal TFTP operation") |
| AC-6 | TFTP RRQ for file exactly 512 bytes | Two DATA blocks sent: first is 512 bytes (block 1), second is 0 bytes (block 2, signals end) |
| AC-7 | TFTP RRQ for empty file (0 bytes) | One DATA block sent: 0 bytes (block 1, signals end) |
| AC-8 | Config verification with missing root-directory | Config rejected with error |
| AC-9 | Plugin disabled in config (enabled=false) | No listener started, no UDP port bound |
| AC-10 | Client does not ACK a DATA block within 5 seconds | Server retransmits the DATA block up to 3 times, then aborts the transfer |
| AC-11 | More than max-transfers (default 10) concurrent TFTP transfers | Excess RRQs receive ERROR code 0 ("Service unavailable") |
| AC-12 | RRQ with mode field other than "octet" (e.g., "netascii") | ERROR packet returned (code 4 "Illegal TFTP operation"); only "octet" mode supported |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTFTPParseRRQ` | `internal/plugins/tftpserver/handler_test.go` | RRQ packet parsing: filename + mode extraction | |
| `TestTFTPParseRRQInvalid` | `internal/plugins/tftpserver/handler_test.go` | Malformed RRQ packets rejected (no NUL, truncated) | |
| `TestTFTPReadRequest` | `internal/plugins/tftpserver/handler_test.go` | Full read transfer: small file, correct DATA/ACK sequence | |
| `TestTFTPReadLargeFile` | `internal/plugins/tftpserver/handler_test.go` | Multi-block transfer: file > 512 bytes, correct block numbering | |
| `TestTFTPReadExact512` | `internal/plugins/tftpserver/handler_test.go` | File exactly 512 bytes: two blocks (512 + 0) | |
| `TestTFTPReadEmptyFile` | `internal/plugins/tftpserver/handler_test.go` | Empty file: one block of 0 bytes | |
| `TestTFTPWriteRejected` | `internal/plugins/tftpserver/handler_test.go` | WRQ returns ERROR code 4 | |
| `TestTFTPFileNotFound` | `internal/plugins/tftpserver/handler_test.go` | RRQ for missing file returns ERROR code 1 | |
| `TestTFTPPathTraversal` | `internal/plugins/tftpserver/handler_test.go` | Path traversal attempts (`../`) return ERROR code 2 | |
| `TestTFTPSymlinkTraversal` | `internal/plugins/tftpserver/handler_test.go` | Symlink inside root-directory pointing outside returns ERROR code 2 | |
| `TestTFTPBuildDataPacket` | `internal/plugins/tftpserver/handler_test.go` | DATA packet format: opcode 3 + block number + data | |
| `TestTFTPBuildErrorPacket` | `internal/plugins/tftpserver/handler_test.go` | ERROR packet format: opcode 5 + code + message + NUL | |
| `TestTFTPConfigParse` | `internal/plugins/tftpserver/config_test.go` | Config parsing: enabled, listen-interface, root-directory | |
| `TestTFTPConfigVerify` | `internal/plugins/tftpserver/config_test.go` | Config verification: missing root-directory rejected | |
| `TestTFTPRetransmitOnTimeout` | `internal/plugins/tftpserver/handler_test.go` | Server retransmits DATA up to 3 times on missing ACK, then aborts | |
| `TestTFTPConcurrentLimit` | `internal/plugins/tftpserver/handler_test.go` | Excess concurrent transfers get ERROR code 0 | |
| `TestTFTPModeHandling` | `internal/plugins/tftpserver/handler_test.go` | RRQ with "octet" accepted; RRQ with "netascii" rejected with ERROR code 4 | |
| `TestTFTPIOErrorMidTransfer` | `internal/plugins/tftpserver/handler_test.go` | Server sends ERROR code 0 ("Not defined") if file read fails mid-transfer (e.g., file truncated during send) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Block number | 1-65535 | 65535 | 0 (invalid, never sent) | N/A (16-bit wraps) |
| Data block size | 0-512 | 512 (full block) | N/A | 513 (would violate RFC 1350) |
| Filename length | 1-255 | 255 chars | 0 (empty string, rejected) | 256 (rejected) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tftp-read` | `test/install/tftp-boot.ci` | TFTP client fetches a bootloader file from tftpserver | |
| `test-tftp-read-missing` | `test/install/tftp-boot.ci` | TFTP client requests non-existent file, gets ERROR | |
| `test-tftp-write-denied` | `test/install/tftp-boot.ci` | TFTP client attempts WRQ, gets ERROR | |

### Future (if deferring any tests)
- QEMU-based PXE boot integration test (requires QEMU with PXE ROM, deferred to spec-install-4)
- Block number wrap test for files > 32MB (edge case, low priority for bootloader use case)

## Files to Modify

None. This is a new plugin with no modifications to existing files.

(Plugin import in all.go is code-generated by `make generate`.)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new module) | Yes | `internal/plugins/tftpserver/schema/ze-tftp-server-conf.yang` |
| CLI commands/flags | No | N/A (config-driven only) |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/install/tftp-boot.ci` |
| Doctor check for runtime dependencies | No | Port 69 requires root; documented, not checked at startup |
| Plugin registration | Yes | `internal/plugins/tftpserver/register.go` |
| Plugin all.go import | Yes | `make generate` (code-generated by `scripts/codegen/plugin_imports.go`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - tftpserver plugin |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - tftpserver plugin |
| 6 | Has a user guide page? | No | Covered by ze-install guide (umbrella spec) |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc1350.md` - TFTP protocol |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `internal/plugins/tftpserver/register.go` - plugin registration, YANG schema, RunEngine
- `internal/plugins/tftpserver/handler.go` - TFTP packet handling: RRQ, DATA, ACK, ERROR
- `internal/plugins/tftpserver/config.go` - config parsing from JSON tree
- `internal/plugins/tftpserver/handler_test.go` - TFTP protocol unit tests
- `internal/plugins/tftpserver/config_test.go` - config parsing tests
- `internal/plugins/tftpserver/register_test.go` - plugin registration test
- `internal/plugins/tftpserver/schema/ze-tftp-server-conf.yang` - YANG schema
- `internal/plugins/tftpserver/schema/embed.go` - //go:embed for YANG file
- `internal/plugins/tftpserver/schema/register.go` - yang.RegisterModule() in init()
- `test/install/tftp-boot.ci` - functional test for TFTP read

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella (spec-install-0-umbrella.md) |
| 2. Audit | Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register plugin, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-13. Fix/verify loop | Per finding |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- plugin skeleton, registration, YANG schema
   - Tests: `TestTFTPServerRegistration` (plugin registers and config parses)
   - Files: `register.go`, `config.go`, `schema/` (all three files)
   - Verify: plugin loads with new YANG config, RunEngine starts and stops cleanly

2. **Phase: Packet building** -- TFTP packet construction and parsing
   - Tests: `TestTFTPParseRRQ`, `TestTFTPParseRRQInvalid`, `TestTFTPBuildDataPacket`, `TestTFTPBuildErrorPacket`
   - Files: `handler.go` (packet building/parsing functions)
   - Verify: packets correctly constructed per RFC 1350 format

3. **Phase: Read transfer** -- RRQ handling with full stop-and-wait loop, retransmission, concurrency
   - Tests: `TestTFTPReadRequest`, `TestTFTPReadLargeFile`, `TestTFTPReadExact512`, `TestTFTPReadEmptyFile`, `TestTFTPRetransmitOnTimeout`, `TestTFTPConcurrentLimit`, `TestTFTPModeHandling`
   - Files: `handler.go` (handleRRQ, serveTransfer, mode validation, semaphore, retransmit loop)
   - Verify: files served correctly over UDP with proper block numbering; retransmit on timeout (5s, 3 attempts); concurrent limit enforced; only "octet" mode accepted; duplicate ACKs ignored (Sorcerer's Apprentice fix); I/O error mid-transfer sends ERROR code 0

4. **Phase: Error handling** -- WRQ rejection, file not found, path traversal
   - Tests: `TestTFTPWriteRejected`, `TestTFTPFileNotFound`, `TestTFTPPathTraversal`
   - Files: `handler.go` (handleWRQ, path validation)
   - Verify: error conditions return correct ERROR packets per RFC 1350

5. **Phase: Config verification** -- config validation at verify time
   - Tests: `TestTFTPConfigParse`, `TestTFTPConfigVerify`
   - Files: `config.go` (verifyTFTPConfig)
   - Verify: invalid configs rejected at verification

6. **Functional tests** -- create after feature works
7. **RFC refs** -- add `// RFC 1350 Section X.Y` comments
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Packet formats match RFC 1350 exactly (opcodes, block numbers, NUL terminators) |
| Naming | YANG leaves use kebab-case, Go types follow ze conventions (unexported handler) |
| Data flow | UDP packet -> parse opcode -> dispatch -> file read -> DATA response |
| Rule: buffer-first | All packet building uses direct byte manipulation, no fmt.Sprintf on wire path |
| Rule: registration | tftpserver uses standard registry.Register(), blank import in all.go via make generate |
| Rule: path traversal | filepath.Clean + filepath.EvalSymlinks + relative check prevents escape from root-directory (including via symlinks) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| tftpserver plugin registers | `grep -rn 'registry.Register' internal/plugins/tftpserver/` |
| YANG schema loads | `grep -rn 'yang.RegisterModule' internal/plugins/tftpserver/schema/` |
| RRQ serves file | `go test ./internal/plugins/tftpserver/ -run TestTFTPReadRequest` |
| WRQ rejected | `go test ./internal/plugins/tftpserver/ -run TestTFTPWriteRejected` |
| Path traversal blocked | `go test ./internal/plugins/tftpserver/ -run TestTFTPPathTraversal` |
| Config parses | `go test ./internal/plugins/tftpserver/ -run TestTFTPConfigParse` |
| Functional test exists | `ls test/install/tftp-boot.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Path traversal | filepath.Clean + filepath.EvalSymlinks + verify resolved path starts with root-directory; reject `..` components and symlinks escaping root |
| Input validation | RRQ/WRQ packet parsing: validate NUL terminators present, filename not empty, opcode valid |
| Resource exhaustion | Limit concurrent TFTP transfers (semaphore or connection count) |
| Timeout | Per-transfer timeout (e.g., 30s total, 5s per ACK wait) to prevent resource leak |
| No write support | WRQ always returns ERROR; no file write path exists in code |

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

### YANG config (extends umbrella)
The YANG schema adds `max-transfers` leaf (uint16, default 10) under `tftp-server` container
to limit concurrent TFTP transfers. Full leaf set: `enabled`, `listen-interface` (leaf-list),
`root-directory`, `max-transfers`.

### Retransmission
RFC 1350 Section 4 requires retransmission on timeout. Implementation: 5-second per-ACK
deadline, 3 retransmit attempts, then abort with "timeout" log. Use `net.UDPConn.SetReadDeadline`
on the per-transfer ephemeral connection.

### Block number wrap (files > 32MB)
Block numbers are 16-bit unsigned (1-65535), limiting a single transfer to 65535 * 512 = ~32MB without block number wrap handling. For v1, this is acceptable: PXE bootloaders are typically 1-5MB. Files exceeding 32MB will fail with a block number wrap. If needed later, RFC 2348 (TFTP Blocksize Option) allows larger blocks, or a rollover scheme can extend the range.

### Duplicate ACK (Sorcerer's Apprentice)
Per the RFC 1350 errata and common TFTP implementations, duplicate ACKs (ACK for block N
received after DATA for block N+1 already sent) must be ignored. Do not resend the next DATA
block on a duplicate ACK, as this creates an escalating chain of duplicate packets.

## RFC Documentation

Add `// RFC 1350 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

Key RFC 1350 sections to reference:
- Section 2: opcodes (RRQ=1, WRQ=2, DATA=3, ACK=4, ERROR=5)
- Section 2: DATA block size (512 bytes, last block < 512 signals end)
- Section 2: block numbering (starts at 1)
- Section 4: initial connection on port 69, transfer on ephemeral TIDs
- Section 5: error codes (0=not defined, 1=file not found, 2=access violation, 4=illegal operation)

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
- [ ] Feature code integrated (`internal/plugins/tftpserver/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

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
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
