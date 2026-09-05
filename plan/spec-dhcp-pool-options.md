# Spec: dhcp-pool-options

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - current fixed-option surface
4. `internal/plugins/dhcpserver/handler.go` - option emission (fixed codes + PXE opt 43)

## Task

**Skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Ze's DHCPv4 server can only hand out a fixed set of well-known options
(default-router, dns-server, domain-name, lease-time) plus a hardcoded PXE
vendor-option block. Operators cannot configure arbitrary DHCP options
(vendor-specific option 43 payloads, TFTP option 66/67 outside the PXE
container, NTP option 42, SIP option 120, classless static routes option 121,
proprietary CPE options, etc.).

Add per-subnet arbitrary option configuration to the DHCPv4 server:

- A list of options per subnet: option code, encoding, value.
- Encodings: `ascii` (raw string bytes) and `hex` (separator-tolerant hex string).
- Validation: option code range, payload length cap (255 bytes for v4), and a
  denylist of codes the server already emits from its own fields so an operator
  cannot double-emit (at minimum: 1 subnet-mask, 3 router, 6 dns, 15 domain-name,
  51 lease-time, 53 message-type, 54 server-id, and the PXE-owned codes when the
  `pxe` container is enabled).
- Design question: subnet-level only, or also range-level and static-mapping-level
  overrides (decide at design time; subnet-level is the minimum).

DHCPv6 options are NOT in scope here: `plan/spec-dhcpv6-server.md` already lists
v6 options (DNS, time-zone, vendor options) in its task; that design should adopt
the same `{code, encoding, value}` config shape decided here for consistency.

Reference implementation: osvbng commit db1a5fb (per-pool DHCPv4 + DHCPv6 vendor
options) uses `{tag, encoding: ascii|hex, value}` per v4 pool with exactly this
denylist approach.

### Added 2026-08-01 (VyOS July 2026 comparison)

Two findings, both read from `internal/plugins/dhcpserver/handler.go` and its
YANG rather than inferred:

- **Option 26 (interface-mtu, RFC 2132) is absent from Ze entirely.** It has no
  entry in the option constant block in `handler.go`, no YANG leaf, and no emit
  path, so Ze cannot hand an MTU to a DHCP client at all. Arbitrary-option
  support as designed above covers it, and no separate spec is needed. VyOS
  T9093 widened their own option-26 validator from 9000 to 16000 because 9216
  jumbo fabrics were rejected. If this spec instead adds a named leaf for option
  26, give it the full 16-bit range and not a 9000 cap. Ze's interface `mtu` leaf
  is already `range "68..16000"` (`ze-iface-conf.yang`), so the two would then
  agree.
- **`lease-time` is capped below the protocol, and this is in scope here.** The
  YANG declares `range "60..604800"` (7 days) and `parseSubnet` re-checks the
  same bound in Go. RFC 2132 section 9.2 makes option 51 a full `uint32`, and
  `0xFFFFFFFF` means an infinite lease. Ze can express neither a lease longer
  than 7 days nor an infinite one. This is the same defect class T9093 fixed: a
  validator narrower than the protocol it validates.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/config-option.md` - structural template for new config options.
  → Constraint: every leaf gets maximum native YANG validation (range/length/pattern).
- [ ] `ai/rules/config.md` - naming for the new list and leaves.
  → Constraint: kebab-case full words; decide `option` vs `dhcp-option` list name at design.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2132 (DHCP options and BOOTP vendor extensions) - option code semantics.
  → Constraint: create `rfc/short/rfc2132.md` during DESIGN if missing.

**Key insights:**
- The wire path already has a bounded option appender (`safeAppendOption`), so the
  feature is mostly config surface + validation + a generic emission loop.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10; re-read at design time)
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - subnet exposes only fixed leaves: `lease-time` (:96), `default-router` (:104), `dns-server` (:109), `domain-name` (:114), plus `range` (:76), `static-mapping` (:119) and a top-level `pxe` container (:28). No generic option list.
- [ ] `internal/plugins/dhcpserver/handler.go` - `optVendorSpecific = 43` (:47) is emitted ONLY inside `appendPXEOptions` (:292, emission at :325); no operator-configurable option path exists.
- [ ] `internal/plugins/dhcpserver/config.go` - subnet config struct carries the fixed fields only (re-verify field list at design time).

**Behavior to preserve:**
- Existing fixed leaves keep working unchanged; the new list is additive.
- PXE container behaviour unchanged; denylist prevents collision with it.

**Behavior to change:**
- Subnets gain an arbitrary-options list (additive).

## Data Flow (MANDATORY)

### Entry Point
- Config: new per-subnet option list in `ze-dhcp-server-conf.yang` (code, encoding, value).

### Transformation Path
1. YANG list parsed into subnet config (decode + validate encoding/length/denylist).
2. Hex values decoded once at config time, not per packet.
3. Reply builder appends configured options via the existing bounded appender after the fixed options.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ dhcpserver | YANG list → subnet config struct | [ ] |
| Config ↔ wire | pre-decoded bytes → option appender | [ ] |

### Integration Points
- `internal/plugins/dhcpserver/handler.go` reply construction - append configured options.
- `internal/plugins/dhcpserver/config.go` subnet struct - carry parsed options.

### Architectural Verification
- [ ] No bypassed layers (config via the standard path)
- [ ] No unintended coupling (feature stays inside the dhcpserver plugin)
- [ ] No duplicated functionality (reuse the existing option appender)
- [ ] Registration over hardcoding - no new core surface; plugin-local config only.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The reply buffer/appender tolerates additional options up to the DHCP size limit | `safeAppendOption` bounded writes in handler.go | need reply-size accounting first | unit test appending max-size options | unvalidated |
| A-2 | Subnet-level granularity matches operator need (range/static-mapping overrides not required for v1) | osvbng ships pool-level only | add override levels in design | user confirmation at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Operator-supplied options conflict with protocol-critical codes | client misbehaviour in test | denylist rejected at config verify, not at runtime |
| R-2 | Large options overflow the reply packet | truncated replies in pcap | length cap at verify + bounded appender at runtime |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| subnet configured with an option (e.g. 42, hex value) + client DISCOVER | → | OFFER/ACK carries the option verbatim | `test/plugin/dhcp-pool-options.ci` |
| option code on the denylist | → | config verify rejects | `test/plugin/dhcp-pool-options-reject.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | subnet option with `ascii` encoding | option bytes = the literal string in OFFER/ACK |
| AC-2 | subnet option with `hex` encoding (with `:`/`-`/space separators) | separators stripped, bytes decoded; odd nibble count rejected at verify |
| AC-3 | option code already auto-emitted (e.g. 3, 6, 53, 54) | config verify rejects with a clear message |
| AC-4 | payload longer than 255 bytes | config verify rejects |
| AC-5 | no options configured | replies byte-identical to today |
| AC-6 | option 43 configured while `pxe` enabled | config verify rejects (PXE owns 43) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures NTP option 42 for a subnet; client requests a lease | config → subnet options → reply builder → client sees option 42 | `test/plugin/dhcp-pool-options.ci` |
| 2 | mis-configures a denylisted or oversized option | config verify rejects before commit | `test/plugin/dhcp-pool-options-reject.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPoolOptionHexDecode` | `internal/plugins/dhcpserver/config_test.go` | hex parsing, separator stripping, odd-nibble rejection | |
| `TestPoolOptionDenylist` | `internal/plugins/dhcpserver/config_test.go` | auto-emitted codes rejected | |
| `TestReplyCarriesConfiguredOptions` | `internal/plugins/dhcpserver/handler_test.go` | options appear in OFFER/ACK | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| option code | design (candidate 1-254 minus denylist) | 254 | 0 | 255 |
| payload length | 0-255 bytes | 255 | N/A | 256 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dhcp-pool-options` | `test/plugin/dhcp-pool-options.ci` | client receives configured options | |
| `dhcp-pool-options-reject` | `test/plugin/dhcp-pool-options-reject.ci` | invalid option config rejected at verify | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| decide at design | `test/interop/scenarios/` | dhclient/udhcpc | real client accepts and surfaces the option | |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - per-subnet option list
- `internal/plugins/dhcpserver/config.go` - parse + validate options
- `internal/plugins/dhcpserver/handler.go` - append configured options to replies

## Files to Create
- `test/plugin/dhcp-pool-options.ci` - functional test
- `test/plugin/dhcp-pool-options-reject.ci` - verify-rejection test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: confirm the config shape (list name, encoding enum, granularity), enumerate the exact denylist from the reply builder, then fill ACs/tests above.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- DHCPv6 options deliberately excluded (owned by `plan/spec-dhcpv6-server.md`).

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `./le verify worktree` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] RFC 2132 summary exists

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
