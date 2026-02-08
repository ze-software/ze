# Spec: exabgp-feature-parity

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/exabgp/migrate.go` — ExaBGP→Ze config conversion
4. `internal/config/bgp.go` — config capability parsing
5. `internal/config/loader.go` — config→PeerSettings, static route building
6. `internal/plugin/bgp/reactor/session.go` — OPEN message building (sendOpen line 1246)
7. `internal/plugin/bgp/schema/ze-bgp-conf.yang` — YANG schema

## Task

Fix all 17 remaining ExaBGP compatibility test failures to achieve full parity (minus multisession, which is intentionally not implemented). Tests compare exact wire bytes, so Ze must produce identical BGP messages to ExaBGP for the same config.

**Target: 37/37 tests passing** (or 36/37 if watchdog requires too much new infrastructure).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` — config format
- [ ] `docs/architecture/wire/capabilities.md` — capability wire encoding
- [ ] `docs/architecture/wire/nlri.md` — NLRI wire encoding
- [ ] `docs/architecture/wire/attributes.md` — attribute wire encoding

### Source Files
- [ ] `internal/exabgp/migrate.go` — migration logic, capability handling (lines 282-332)
- [ ] `internal/config/bgp.go` — capability parsing (lines 624-680), route parsing
- [ ] `internal/config/loader.go` — capability→OPEN objects (lines 470-594), static routes (lines 628+)
- [ ] `internal/config/routeattr.go` — RD/label/prefix-sid parsing
- [ ] `internal/plugin/bgp/reactor/session.go` — sendOpen() (lines 1246-1323)
- [ ] `internal/plugin/bgp/capability/capability.go` — hostname (629), software-version (709)
- [ ] `internal/plugin/bgp/nlri/` — NLRI wire encoding per family

**Key insights:**
- Ze ALREADY HAS encoding for: hostname cap (73), software-version cap (75), ADD-PATH, VPN NLRI, PREFIX-SID, FlowSpec decode, MVPN wire types, MUP wire types, route splitting
- Most failures are config migration gaps or config→wire path bugs, NOT missing encoders
- `cmd:` lines in .ci files are IGNORED by the test runner — only `raw:` wire bytes are compared
- The exabgp wrapper runs `ze <config>` (not `ze bgp server`), so static routes are sent on startup

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/exabgp-compat/bin/bgp` — mock peer validates wire bytes via exact hex comparison
- [ ] `test/exabgp-compat/bin/functional` — test runner, starts bgp + exabgp wrapper
- [ ] `test/exabgp-compat/bin/exabgp` — wrapper: migrate config → run ze → compare wire output
- [ ] `internal/exabgp/migrate.go` — migration drops: link-local-nexthop cap, hostname fields partially, flow routes, L2VPN routes; `asn4 disable` handling is incorrect

**Behavior to preserve:**
- All 20 currently-passing ExaBGP tests
- All 224 Ze functional tests
- All unit tests

**Behavior to change:**
- 17 failing tests should pass (wire bytes match expectations)

## Data Flow

### Entry Point
- `make functional-exabgp` → `./test/exabgp-compat/bin/functional encoding --timeout 60`

### Transformation Path
1. `functional` reads `.ci` file, extracts `option=file:` config references
2. `functional` starts `bgp` server (mock peer) with `.ci` wire expectations
3. `functional` starts `exabgp` wrapper with `.conf` config
4. `exabgp` wrapper calls `ze exabgp migrate <conf>` → Ze config on stdout
5. `exabgp` wrapper writes migrated config to temp file
6. `exabgp` wrapper runs `ze <temp.conf>` → Ze starts, connects to mock peer
7. Ze sends OPEN with negotiated capabilities
8. Ze sends UPDATE with static routes from config
9. Mock peer compares each received message against expected `raw:` hex bytes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ExaBGP config → Ze config | `ze exabgp migrate` subprocess | [ ] |
| Ze config → PeerSettings | `loader.go` config loading | [ ] |
| PeerSettings → OPEN message | `session.go:sendOpen()` | [ ] |
| StaticRoutes → UPDATE messages | `reactor/session.go` static route encoding | [ ] |
| Ze → mock peer | TCP BGP session | [ ] |

### Integration Points
- `internal/exabgp/migrate.go` — ExaBGP→Ze config format conversion
- `internal/config/loader.go` — Ze config→runtime PeerSettings
- `internal/plugin/bgp/reactor/session.go` — OPEN + UPDATE wire encoding
- `internal/plugin/bgp/capability/` — capability wire types
- `internal/plugin/bgp/nlri/` — NLRI wire encoding per family

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestMigrateCapabilityLinkLocalNexthop | `internal/exabgp/migrate_test.go` | link-local-nexthop capability migration | |
| TestMigrateASN4Disable | `internal/exabgp/migrate_test.go` | `asn4 disable` preserved in migration | |
| TestMigrateFlowRoutes | `internal/exabgp/migrate_test.go` | flow route blocks converted | |
| TestMigrateL2VPNRoutes | `internal/exabgp/migrate_test.go` | L2VPN/VPLS routes converted | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ExaBGP full suite | `make functional-exabgp` | 37/37 ExaBGP compat tests pass | |
| Ze suite | `make functional` | All 224+ tests still pass (no regression) | |

## Test Failure Analysis

### Phase 1: OPEN Capability Fixes (tests 3, 4, C)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| 4 | conf-cap-software-version | Capability exists but not included in OPEN | Debug loader.go capability building; ensure SoftwareVersion config triggers capability object |
| C | conf-hostname | hostname plugin provides cap at runtime; not started in test mode | Inject hostname capability from config fields in loader, or ensure plugin starts |
| 3 | conf-cap-link-local-nexthop | Migration drops capability; not in Ze schema | Add to migration enableFields, YANG schema, config parser, loader, capability type |

### Phase 2: Simple Encoding Fixes (tests Q, U, L)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| Q | conf-no-asn4 | `asn4 disable` migration broken (only truthy values handled) | Fix migration to emit `asn4 disable;`; verify AS_PATH uses 2-byte encoding |
| U | conf-split | Split exists in Ze; encoding order may differ | Debug wire bytes, fix attribute/NLRI ordering if needed |
| L | conf-llnh-update | IPv6 link-local NH: 16 bytes sent, 32 expected | Encode both global + link-local NH in MP_REACH_NLRI (RFC 2545 Section 3) |

### Phase 3: VPN/MPLS/ADD-PATH Encoding (tests 0, Z, R, T)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| 0 | conf-addpath | path-id not in NLRI from config routes | Trace StaticRouteConfig.PathInformation → wire NLRI; fix encoding path |
| Z | conf-vpn | VPN NLRI truncated (82 vs 99 bytes) | Fix RD+label+prefix encoding in config→wire path |
| R | conf-parity | IPv6 VPN multiple attribute mismatches | Fix IPv6 VPN encoding; may share root cause with Z |
| T | conf-prefix-sid | PREFIX-SID attribute missing from UPDATE | Fix config PrefixSID → wire attribute encoding |

### Phase 4: Route Announcement Fixes (tests M, V, W)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| M | conf-mvpn | Sends withdrawal instead of announcement | Fix static route encoder for MVPN family |
| V | conf-srv6-mup-v3 | Sends withdrawal instead of announcement | Fix static route encoder for MUP family |
| W | conf-srv6-mup | Sends withdrawal instead of announcement | Same as V |

### Phase 5: FlowSpec Route Migration (tests 7, 8)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| 7 | conf-flow-redirect | Flow rules not migrated from ExaBGP format | Add flow block parsing to migrate.go; convert to Ze static flow routes |
| 8 | conf-flow | Same as 7 | Same as 7 |

### Phase 6: L2VPN/VPLS Route Migration (test I)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| I | conf-l2vpn | VPLS routes not migrated from ExaBGP format | Add L2VPN block parsing to migrate.go; convert endpoint/base/offset/size |

### Phase 7: Watchdog (test a)

| Test | Name | Root Cause | Fix |
|------|------|-----------|-----|
| a | conf-watchdog | ExaBGP watchdog process model not bridged | Bridge process model in exabgp wrapper or implement watchdog route control |

## Files to Modify

- `internal/exabgp/migrate.go` — fix capability migration, add flow/L2VPN route migration
- `internal/exabgp/migrate_test.go` — unit tests for migration fixes
- `internal/config/bgp.go` — add link-local-nexthop capability parsing
- `internal/config/loader.go` — fix capability object building, fix static route encoding for VPN/MVPN/MUP families
- `internal/plugin/bgp/schema/ze-bgp-conf.yang` — add link-local-nexthop leaf to capability
- `internal/plugin/bgp/reactor/session.go` — verify OPEN and UPDATE encoding
- `internal/plugin/bgp/capability/` — may need link-local-nexthop type
- `test/exabgp-compat/bin/exabgp` — watchdog bridge (Phase 7)

## Files to Create

- None expected (extend existing files)

## Implementation Steps

### Step 1: Phase 1 — OPEN Capability Fixes
- Debug and fix software-version, hostname, link-local-nexthop in OPEN
- Run tests 3, 4, C individually
- **Review:** Do capabilities match expected bytes?

### Step 2: Phase 2 — Simple Encoding Fixes
- Fix `asn4 disable` migration, split encoding, IPv6 link-local NH
- Run tests Q, U, L individually
- **Review:** Do UPDATE bytes match?

### Step 3: Phase 3 — VPN/MPLS/ADD-PATH
- Fix VPN NLRI, ADD-PATH path-id, PREFIX-SID attribute encoding
- Run tests 0, Z, R, T individually
- **Review:** Are all attributes present and correctly encoded?

### Step 4: Phase 4 — Route Announcement
- Fix MVPN and MUP static route announcement path
- Run tests M, V, W individually
- **Review:** Are routes announced (not withdrawn)?

### Step 5: Phase 5 — FlowSpec Migration
- Add flow block parsing to migrate.go
- Run tests 7, 8 individually
- **Review:** Are flow rules correctly converted?

### Step 6: Phase 6 — L2VPN Migration
- Add L2VPN/VPLS block parsing to migrate.go
- Run test I individually
- **Review:** Are VPLS routes correctly converted?

### Step 7: Phase 7 — Watchdog
- Investigate and implement watchdog process bridge
- Run test a individually
- **Review:** Does watchdog route sequencing work?

### Step 8: Full verification
- `make functional-exabgp` — all 37 pass
- `make functional` — no regression
- `make test` — no regression
- `make lint` — clean

## Expected Results

| Category | Count | Tests |
|----------|-------|-------|
| Pass | 37 | All tests |
| Fail | 0 | None (or 1 if watchdog deferred) |

## Implementation Summary

<!-- Fill after implementation -->

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix OPEN capability tests (3, 4, C) | | | |
| Fix simple encoding tests (Q, U, L) | | | |
| Fix VPN/MPLS/ADD-PATH tests (0, Z, R, T) | | | |
| Fix route announcement tests (M, V, W) | | | |
| Fix FlowSpec migration (7, 8) | | | |
| Fix L2VPN migration (I) | | | |
| Fix watchdog (a) | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| ExaBGP suite 37/37 | | | |
| Ze suite no regression | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/exabgp/migrate.go | | |
| internal/config/bgp.go | | |
| internal/config/loader.go | | |
| internal/plugin/bgp/schema/ze-bgp-conf.yang | | |
| internal/plugin/bgp/reactor/session.go | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion
- [ ] Implementation Audit completed
- [ ] Spec updated with Implementation Summary
