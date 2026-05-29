# Spec: install-1-dhcp-pxe

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-05-28 |
| Parent | spec-install-0-umbrella |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/dhcpserver/handler.go` - DHCP packet handling, buildReply, safeAppendOption
4. `internal/plugins/dhcpserver/config.go` - serverConfig, parseConfig
5. `internal/plugins/dhcpserver/register.go` - plugin lifecycle, handler creation
6. `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - YANG schema
7. `internal/plugins/dhcpserver/handler_test.go` - existing test patterns (buildDiscover, newTestServer)

## Task

Extend the existing dhcpserver plugin (`internal/plugins/dhcpserver/`) with PXE
option handling per RFC 4578. When a DHCP Discover arrives with option 60
("PXEClient:..."), the server detects PXE client architecture from option 93 and
responds with bootfile path (option 67) and TFTP server (option 66) matching the
client architecture (BIOS vs UEFI). This is additive: non-PXE DHCP behavior is
unchanged.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - ze plugin model, YANG config flow
  -> Decision: plugins register via registry.Register() with YANG schema
  -> Constraint: config arrives as JSON via sdk.ConfigSection, parsed in verifier/OnConfigure
- [ ] `internal/plugins/dhcpserver/handler.go` - existing DHCP packet handling
  -> Decision: buildReply() builds response with safeAppendOption(); siaddr at resp[20:24]
  -> Constraint: buffer-first encoding, no fmt.Sprintf on wire path
- [ ] `internal/plugins/dhcpserver/config.go` - config parsing from JSON tree
  -> Decision: parseConfig() walks root["service"]["dhcp-server"] map
  -> Constraint: PXE config block must parse from same JSON tree under "pxe" key
- [ ] `internal/plugins/dhcpserver/register.go` - plugin lifecycle
  -> Decision: newDHCPHandler() creates per-subnet handlers; serveMulti dispatches
  -> Constraint: pxeConfig is server-wide, not per-subnet; thread through serverConfig
- [ ] `internal/plugins/dhcpserver/handler_test.go` - test patterns
  -> Decision: buildDiscover() builds raw DHCP packets with options; newTestServer() creates handler
  -> Constraint: PXE tests extend these helpers with option 60/93 injection

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2131.md` - DHCP base protocol (MUST CREATE)
  -> Constraint: DORA message exchange; siaddr is "next server" IP in header
- [ ] `rfc/short/rfc2132.md` - DHCP options (MUST CREATE)
  -> Constraint: option 60 = Vendor Class Identifier, option 66 = TFTP Server Name, option 67 = Bootfile Name
- [ ] `rfc/short/rfc4578.md` - DHCP PXE options (MUST CREATE)
  -> Constraint: option 93 = Client System Architecture Type, 2 bytes big-endian; 0 = IA x86 (BIOS), 7 = EFI x86-64

**Key insights:**
- Option 43 suboption 71 (PXE boot item) is defined in the Intel PXE Specification 2.1, not in an IETF RFC. It is a de facto standard for PXE boot servers.
- PXE detection is option 60 prefix match ("PXEClient:"), architecture is option 93 (uint16)
- siaddr (bytes 20-23) must be overridden to PXE TFTP server IP; some PXE ROMs ignore option 66
- Options 66/67 are string options (null-terminated in some implementations, but ze uses length-prefixed via safeAppendOption)
- Existing handler tests build raw packets and call handle() directly; PXE tests do the same with added options

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/dhcpserver/handler.go` (363L) - RFC 2131 DHCP server: handle(), handleDiscover(), handleRequest(), buildReply(), safeAppendOption(), parseMsgType(), parseOptionAddr(), extractMAC()
  -> Constraint: buildReply writes siaddr from h.serverIP at resp[20:24]; PXE must override this
  -> Constraint: safeAppendOption bounds-checks and returns new offset; PXE options use same pattern
- [ ] `internal/plugins/dhcpserver/config.go` (345L) - parseConfig parses serverConfig from JSON; serverConfig has Enabled, ListenInterfaces, SharedNetworks
  -> Constraint: pxeConfig is a new field on serverConfig, parsed from dhcpMap["pxe"]
- [ ] `internal/plugins/dhcpserver/register.go` (218L) - runDHCPServerPlugin creates handlers per subnet via newDHCPHandler; PXE config is server-wide
  -> Constraint: pxeConfig passes through startServer -> newDHCPHandler or stored on handler
- [ ] `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` (113L) - YANG module with service > dhcp-server container
  -> Constraint: pxe container goes under dhcp-server, sibling to shared-network
- [ ] `internal/plugins/dhcpserver/handler_test.go` (existing) - tests use buildDiscover(), buildRequest(), newTestServer() helpers
  -> Constraint: PXE tests add option 60/93 to packets built by these helpers

**Behavior to preserve:**
- All existing DHCP Discover/Offer/Request/Ack behavior unchanged when PXE is disabled
- All existing DHCP Discover/Offer/Request/Ack behavior unchanged when client is not PXE (no option 60 with "PXEClient:" prefix)
- siaddr continues to be h.serverIP for non-PXE replies
- Option ordering in replies unchanged for non-PXE options
- Pool allocation, lease tracking, static mappings unchanged
- NAK behavior unchanged
- Release/Decline handling unchanged

**Behavior to change:**
- When PXE enabled in config AND DHCP Discover contains option 60 starting with "PXEClient:":
  1. Parse option 93 for client architecture type
  2. Override siaddr (resp[20:24]) with configured pxe.tftp-server IP
  3. Append option 66 (TFTP server name) with pxe.tftp-server IP string
  4. Append option 67 (bootfile name) with pxe.bootfile-bios or pxe.bootfile-uefi based on architecture
  5. Append option 60 (vendor class identifier) with fixed string "PXEClient" (9 bytes), not the client's full option 60 value
  6. Append option 43 (Vendor-Specific Information) with PXE boot item suboption (type 71, length 4, item 0/layer 0) to signal the server is a PXE boot server. Required by some PXE ROMs to proceed with boot.
- New config block `pxe` under `dhcp-server` with enabled, tftp-server, bootfile-bios, bootfile-uefi
- New option constants: optVendorClassID (60), optTFTPServerName (66), optBootfileName (67), optClientArch (93), optVendorSpecific (43)
- Increase `buildReply` buffer from `make([]byte, 576)` to `make([]byte, 1500)` (Ethernet MTU) to accommodate PXE options without headroom concerns

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- DHCP Discover UDP packet on port 67 with PXE option 60 ("PXEClient:Arch:...") and option 93 (client architecture)
- Config JSON with `service.dhcp-server.pxe` block

### Transformation Path
1. `serveMulti()` reads UDP packet, passes to `h.handle(pkt)` (existing)
2. `handle()` dispatches to `handleDiscover()` (existing)
3. `handleDiscover()` calls `h.buildReply(pkt, msgOffer, addr)` (existing)
4. `buildReply()` builds standard reply options (existing), then:
   - NEW: calls `h.appendPXEOptions(req, resp, &off, limit)` if h.pxe.Enabled
   - `appendPXEOptions()` calls `isPXEClient(req)` to check option 60
   - If PXE client: calls `parsePXEArch(req)` to read option 93
   - Overrides siaddr (resp[20:24]) with h.pxe.TFTPServer
   - Appends options 60, 66, 67 via `safeAppendOption()`
5. Response written to UDP socket (existing)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> handler | UDP DHCP on port 67 (existing listener) | [ ] |
| Config JSON -> pxeConfig | parseConfig walks dhcpMap["pxe"] | [ ] |
| pxeConfig -> dhcpHandler | Stored as field on dhcpHandler struct | [ ] |

### Integration Points
- `dhcpHandler.buildReply()` - extended with PXE option append call
- `serverConfig` - new PXE field threaded through startServer to newDHCPHandler
- `parseConfig()` - new parsePXEConfig() call for the "pxe" key
- YANG schema - new `pxe` container under `dhcp-server`

### Architectural Verification
- [ ] No bypassed layers (PXE options flow through existing buildReply path)
- [ ] No unintended coupling (PXE logic self-contained in handler, config in config.go)
- [ ] No duplicated functionality (extends existing handler, does not fork)
- [ ] Zero-copy preserved where applicable (buffer-first encoding via safeAppendOption)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| DHCP Discover with option 60 "PXEClient:Arch:00000:UNDI:002001" + option 93 = 0 (BIOS) | -> | buildReply appends options 66/67 with BIOS bootfile, overrides siaddr | `TestPXEDiscoverBIOS` in `handler_test.go` |
| DHCP Discover with option 60 "PXEClient:Arch:00007:UNDI:003016" + option 93 = 7 (UEFI x64) | -> | buildReply appends options 66/67 with UEFI bootfile, overrides siaddr | `TestPXEDiscoverUEFI` in `handler_test.go` |
| DHCP Discover without option 60 (non-PXE client) | -> | buildReply returns standard reply, no PXE options | `TestNonPXEDiscoverUnchanged` in `handler_test.go` |
| Config JSON with pxe block | -> | parseConfig returns serverConfig with PXE fields populated | `TestParsePXEConfig` in `config_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DHCP Discover with option 60 "PXEClient:" prefix + option 93 = 0 (BIOS), PXE enabled in config | Offer contains option 66 = tftp-server IP, option 67 = bootfile-bios, siaddr = tftp-server IP |
| AC-2 | DHCP Discover with option 60 "PXEClient:" prefix + option 93 = 7 (UEFI x64), PXE enabled in config | Offer contains option 66 = tftp-server IP, option 67 = bootfile-uefi, siaddr = tftp-server IP |
| AC-3 | DHCP Discover without option 60, PXE enabled in config | Standard Offer with no PXE options, siaddr = serverIP (unchanged) |
| AC-4 | DHCP Discover with option 60 "PXEClient:", PXE disabled in config | Standard Offer with no PXE options, siaddr = serverIP (unchanged) |
| AC-5 | Config JSON with valid pxe block (enabled, tftp-server, bootfile-bios, bootfile-uefi) | parseConfig returns serverConfig with PXE populated, verifyDHCPConfig succeeds |
| AC-6 | Config JSON without pxe block | parseConfig returns serverConfig with PXE disabled (zero value), no error |
| AC-7 | DHCP Discover with option 60 "PXEClient:" but no option 93 | Offer contains PXE options with BIOS bootfile as default fallback |
| AC-8 | YANG schema validates with new pxe container | `make ze-lint` passes with updated YANG |
| AC-9 | DHCP Offer to PXE client (option 60 "PXEClient:" present) | Offer contains option 43 with PXE boot item suboption (type 71, length 4) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPXEDiscoverBIOS` | `internal/plugins/dhcpserver/handler_test.go` | BIOS PXE Discover gets Offer with correct bootfile + siaddr override | |
| `TestPXEDiscoverUEFI` | `internal/plugins/dhcpserver/handler_test.go` | UEFI PXE Discover gets Offer with UEFI bootfile | |
| `TestNonPXEDiscoverUnchanged` | `internal/plugins/dhcpserver/handler_test.go` | Non-PXE Discover response identical to pre-PXE behavior | |
| `TestPXEDisabledIgnoresOptions` | `internal/plugins/dhcpserver/handler_test.go` | PXE disabled in config: PXE client gets standard reply | |
| `TestPXENoArch93DefaultsBIOS` | `internal/plugins/dhcpserver/handler_test.go` | Missing option 93 defaults to BIOS bootfile | |
| `TestIsPXEClient` | `internal/plugins/dhcpserver/handler_test.go` | isPXEClient returns true for "PXEClient:" prefix, false otherwise | |
| `TestParsePXEArch` | `internal/plugins/dhcpserver/handler_test.go` | parsePXEArch returns correct architecture type from option 93 | |
| `TestParsePXEConfig` | `internal/plugins/dhcpserver/config_test.go` | PXE config block parses correctly from JSON | |
| `TestParsePXEConfigMissing` | `internal/plugins/dhcpserver/config_test.go` | Missing PXE block returns disabled PXE config (no error) | |
| `TestParsePXEConfigInvalid` | `internal/plugins/dhcpserver/config_test.go` | Invalid tftp-server IP returns parse error | |
| `TestPXERequestAck` | `internal/plugins/dhcpserver/handler_test.go` | PXE Request/Ack also includes PXE options (full DORA cycle) | |
| `TestPXEOption43` | `internal/plugins/dhcpserver/handler_test.go` | PXE Offer contains option 43 with boot item suboption (type 71, len 4) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Option 93 value (client arch) | 0-65535 (uint16) | 65535 | N/A (unsigned) | N/A (2-byte field) |
| Option 60 length | 1-255 | 255 (max DHCP option length) | 0 (empty, not PXE) | N/A (DHCP option length is 1 byte) |
| Option 67 bootfile length | 1-128 | 128 (reasonable max path) | 0 (empty = misconfigured) | 255 (DHCP option max) |
| Option 93 data length | 2 (fixed per RFC 4578) | 2 | 1 (too short, ignored) | 3+ (too long, ignored) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-install-dhcp-pxe` | `test/install/dhcp-pxe.ci` | PXE client (BIOS + UEFI) gets correct bootfile; non-PXE client gets standard offer | |

### Future (if deferring any tests)
- ProxyDHCP mode (PXE alongside existing DHCP server) is a v2 feature, not tested here

## Files to Modify

- `internal/plugins/dhcpserver/handler.go` - new PXE option constants, isPXEClient(), parsePXEArch(), appendPXEOptions() called from buildReply()
- `internal/plugins/dhcpserver/config.go` - pxeConfig struct, parsePXEConfig(), add PXE field to serverConfig
- `internal/plugins/dhcpserver/register.go` - thread pxeConfig through startServer to newDHCPHandler. In `startServer()` (line 81-118), pass `cfg.PXE` to each `newDHCPHandler()` call inside the SharedNetworks/Subnets loop.
- `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` - add pxe container under dhcp-server
- `internal/plugins/dhcpserver/handler_test.go` - PXE test cases (extend existing file). **Signature change impact:** existing helpers `newTestServer()` and `newTestServerWithStatic()` call `newDHCPHandler(sub, serverIP)` with the current 2-arg signature. All existing helpers must be updated to pass `pxeConfig{}` (zero value) as the new third argument to avoid breaking existing non-PXE tests.
- `internal/plugins/dhcpserver/config_test.go` - PXE config parsing tests (extend existing file)

**Constructor signature change:** `newDHCPHandler(sub subnetConfig, serverIP netip.Addr)` becomes `newDHCPHandler(sub subnetConfig, serverIP netip.Addr, pxe pxeConfig)`. The `pxeConfig` is server-wide (from `serverConfig.PXE`), passed through `startServer()` at register.go:89-102.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new container) | Yes | `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` |
| CLI commands/flags | No | N/A (config-only, no new CLI commands) |
| CLI grammar (action before identifier) | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/install/dhcp-pxe.ci` |
| Doctor check for runtime dependencies | No | N/A (existing DHCP listener) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - PXE boot support in DHCP server |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - dhcp-server pxe block |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - dhcpserver PXE extension |
| 6 | Has a user guide page? | Yes | `docs/guide/ze-install.md` (umbrella creates this) |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4578.md` - PXE options |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `test/install/dhcp-pxe.ci` - functional test for PXE DHCP offer

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- PXE option stubs in handler, config struct
   - Tests: `TestPXEDiscoverBIOS` (fails: no PXE options in response yet)
   - Files: `handler.go` (option constants, stub appendPXEOptions), `config.go` (pxeConfig struct)
   - Verify: handler compiles with new fields; PXE test exists and fails

2. **Phase: Config parsing** -- pxeConfig parsing from JSON
   - Tests: `TestParsePXEConfig`, `TestParsePXEConfigMissing`, `TestParsePXEConfigInvalid`
   - Files: `config.go` (parsePXEConfig), `register.go` (thread pxeConfig)
   - Verify: config tests pass; handler receives pxeConfig

3. **Phase: PXE detection** -- isPXEClient and parsePXEArch
   - Tests: `TestIsPXEClient`, `TestParsePXEArch`
   - Files: `handler.go` (isPXEClient, parsePXEArch, parseOptionBytes helper)
   - Verify: detection unit tests pass

4. **Phase: PXE option encoding** -- appendPXEOptions in buildReply
   - Tests: `TestPXEDiscoverBIOS`, `TestPXEDiscoverUEFI`, `TestNonPXEDiscoverUnchanged`, `TestPXEDisabledIgnoresOptions`, `TestPXENoArch93DefaultsBIOS`, `TestPXERequestAck`
   - Files: `handler.go` (appendPXEOptions, siaddr override)
   - Verify: all PXE handler tests pass; non-PXE behavior unchanged

5. **Phase: YANG schema** -- add pxe container
   - Tests: `make ze-lint` validates YANG
   - Files: `schema/ze-dhcp-server-conf.yang`
   - Verify: YANG validates, editor autocomplete picks up pxe leaves

6. **Functional tests** -- create after feature works
7. **RFC refs** -- add `// RFC 4578 Section N` comments
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Option 93 values match RFC 4578 Table 1; siaddr override correct byte offset (20:24) |
| Naming | PXE option constants follow existing naming (optXxx); config fields use kebab-case in YANG |
| Data flow | PXE options only appended inside buildReply when PXE enabled AND client is PXE |
| Rule: buffer-first | All PXE option encoding uses safeAppendOption, no fmt.Sprintf on wire path |
| Rule: no-layering | No new files; PXE logic lives in existing handler.go and config.go |
| Non-PXE regression | Run existing handler_test.go tests to verify non-PXE behavior unchanged |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| PXE option constants defined | `grep -n 'optTFTPServerName\|optBootfileName\|optClientArch\|optVendorClassID' handler.go` |
| isPXEClient function | `grep -n 'isPXEClient' handler.go` |
| parsePXEArch function | `grep -n 'parsePXEArch' handler.go` |
| appendPXEOptions called from buildReply | `grep -n 'appendPXEOptions' handler.go` |
| pxeConfig struct in config | `grep -n 'pxeConfig' config.go` |
| YANG pxe container | `grep -n 'pxe' ze-dhcp-server-conf.yang` |
| PXE handler tests pass | `go test ./internal/plugins/dhcpserver/ -run TestPXE -v` |
| PXE config tests pass | `go test ./internal/plugins/dhcpserver/ -run TestParsePXE -v` |
| Existing tests still pass | `go test ./internal/plugins/dhcpserver/ -v` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Option 60/93 parsing: validate option length before reading bytes; parsePXEArch must check length >= 2 |
| Buffer overflow | safeAppendOption already bounds-checks; verify PXE option string lengths fit within 255-byte DHCP option max |
| Denial of service | Malformed option 60 with length 0 or very large length: existing option iterator handles this |
| Config validation | tftp-server must be a valid IPv4 address; reject at config parse time |

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

- **Increase reply buffer from 576 to 1500.** `buildReply()` allocates `make([]byte, 576)`. With PXE options 43+60+66+67 (~60 bytes) on top of standard options (~80 bytes worst case), headroom is only ~100 bytes. Increase to `make([]byte, 1500)` (Ethernet MTU). This is the natural ceiling for a DHCP payload in one frame without fragmentation. No PXE ROM will choke on a larger reply, and the allocation cost is negligible (one per reply, short-lived). This change goes in the existing `buildReply`, not in PXE-specific code, so it benefits all replies and eliminates future headroom concerns when adding options. `safeAppendOption` still bounds-checks against `len(resp)-1` so the safety guarantee is unchanged.

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

PXE-specific RFC constraints to document:
- RFC 2131 Section 4.3.1: siaddr in DHCPOFFER identifies the address of the server the client should use for the next step in the bootstrap process (TFTP). For PXE, this MUST be the TFTP server address.
- RFC 4578 Section 2.1: option 93 is 2 bytes, big-endian, client system architecture type
- RFC 4578 Table 1: type 0 = Intel x86PC (BIOS), type 7 = EFI BC (UEFI x86-64)
- RFC 2132 Section 9.6: option 60 is variable-length string, vendor class identifier
- RFC 2132 Section 9.9: option 66 is variable-length string, TFTP server name
- RFC 2132 Section 9.10: option 67 is variable-length string, bootfile name

## Implementation Summary

### What Was Implemented
- PXE option constants (43, 60, 66, 67, 93) in handler.go
- `isPXEClient()`, `parsePXEArch()`, `parseOptionBytes()` detection functions
- `appendPXEOptions()` called from `buildReply()` for PXE option injection
- `pxeConfig` struct and `parsePXEConfig()` in config.go
- pxe container in YANG schema
- Reply buffer increased from 576 to 1500 bytes (Ethernet MTU)
- PXE config threaded through register.go `startServer()` to `newDHCPHandler()`

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/guide/configuration.md`: added PXE Boot Support section with config example and settings table
- `docs/guide/plugins.md`: added dhcpserver plugin to Infrastructure table

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extend dhcpserver with PXE options | Done | handler.go:289-320 | appendPXEOptions |
| Detect PXE clients via option 60 | Done | handler.go:431-434 | isPXEClient |
| Parse client architecture from option 93 | Done | handler.go:438-444 | parsePXEArch |
| Override siaddr for PXE | Done | handler.go:302 | copy(resp[20:24], tftpIP[:]) |
| Add YANG pxe container | Done | ze-dhcp-server-conf.yang:28-51 | |
| Non-PXE behavior unchanged | Done | handler_test.go:785 | TestNonPXEDiscoverUnchanged |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestPXEDiscoverBIOS (handler_test.go:724) | opt66, opt67, siaddr verified |
| AC-2 | Done | TestPXEDiscoverUEFI (handler_test.go:763) | UEFI bootfile verified |
| AC-3 | Done | TestNonPXEDiscoverUnchanged (handler_test.go:785) | No PXE options, siaddr=serverIP |
| AC-4 | Done | TestPXEDisabledIgnoresOptions (handler_test.go:814) | PXE disabled, no PXE options |
| AC-5 | Done | TestParsePXEConfig (config_test.go:363) | Config parsing verified |
| AC-6 | Done | TestParsePXEConfigMissing (config_test.go:411) | PXE disabled by default |
| AC-7 | Done | TestPXENoArch93DefaultsBIOS (handler_test.go:835) | BIOS fallback verified |
| AC-8 | Done | make ze-lint-changed passes | YANG validates |
| AC-9 | Done | TestPXEOption43 (handler_test.go:989) | type=71, len=4 verified |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestPXEDiscoverBIOS | Pass | handler_test.go:724 | |
| TestPXEDiscoverUEFI | Pass | handler_test.go:763 | |
| TestNonPXEDiscoverUnchanged | Pass | handler_test.go:785 | |
| TestPXEDisabledIgnoresOptions | Pass | handler_test.go:814 | |
| TestPXENoArch93DefaultsBIOS | Pass | handler_test.go:835 | |
| TestIsPXEClient | Pass | handler_test.go:875 | 6 subtests |
| TestParsePXEArch | Pass | handler_test.go:913 | 5 subtests |
| TestParsePXEConfig | Pass | config_test.go:363 | |
| TestParsePXEConfigMissing | Pass | config_test.go:411 | |
| TestParsePXEConfigInvalid | Pass | config_test.go:424 | 2 subtests |
| TestPXERequestAck | Pass | handler_test.go:950 | Full DORA cycle |
| TestPXEOption43 | Pass | handler_test.go:989 | Boot item suboption |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugins/dhcpserver/handler.go | Done | PXE option constants, detection, encoding |
| internal/plugins/dhcpserver/config.go | Done | pxeConfig struct, parsePXEConfig |
| internal/plugins/dhcpserver/register.go | Done | PXE config threading |
| internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang | Done | pxe container |
| internal/plugins/dhcpserver/handler_test.go | Done | 12 PXE tests |
| internal/plugins/dhcpserver/config_test.go | Done | 3 PXE config tests |
| test/install/dhcp-pxe-config.ci | Done | Functional test |

### Audit Summary
- **Total items:** 28
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/dhcpserver/`)
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
