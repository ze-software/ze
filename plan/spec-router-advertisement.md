# Spec: router-advertisement

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/l2tp/ppp/ra.go` - the only existing RA builder (BNG, prefix-less)
4. `internal/component/l2tp/ppp/ra_linux.go` - the RA raw-socket send loop
5. `ai/rules/qemu-testing.md` - RA is Linux-only; QEMU integration tests mandatory

## Task

**Large feature area - skeleton only. Full design not started.**

Ze cannot advertise IPv6 prefixes on a LAN interface. There is no radvd-equivalent
that periodically sends ICMPv6 Router Advertisements to drive SLAAC, advertise
on-link prefixes, DNS (RDNSS/DNSSL), MTU, or the managed/other-config flags. The
only RA emitter in the tree is the L2TP/PPP BNG subscriber path, and it deliberately
sends prefix-less RAs that steer the subscriber to DHCPv6; there is no prefix-option
type anywhere in the codebase.

Implement a LAN Router Advertisement sender:
- Per-interface RA config (managed/other flags, default lifetime, intervals, hop limit).
- One or more advertised prefixes with on-link/autonomous flags and lifetimes.
- Optional RDNSS/DNSSL and MTU options.
- Respond to Router Solicitations and send unsolicited RAs on a timer.

This is a new subsystem (most plausibly a new `internal/plugins/radvd/` plugin). It
must go through the full `/ze-spec` RESEARCH/DESIGN workflow. This skeleton tracks the
gap; it is NOT ready to implement.

## Required Reading

### Architecture Docs
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - the existing RA raw-socket sender, RS listener, and send loop.
  → Constraint: reuse the `BuildRA` + raw-socket/RS-listener pattern, but generalise it to a LAN interface and add prefix options.
- [ ] `internal/core/sysctl/known_linux.go` - host-side RA acceptance sysctls (accept_ra, autoconf, forwarding).
  → Constraint: sending RAs is distinct from accepting them; a router sending RAs must have IPv6 forwarding on and typically does not accept RAs on the same interface.
- [ ] `ai/rules/qemu-testing.md` - RA delivery is a kernel/link behaviour; QEMU integration tests are mandatory.
  → Constraint: never skip QEMU tests for "needs hardware".

### RFC Summaries (MUST for protocol work)
- [ ] RFC 4861 (Neighbor Discovery for IPv6): RA message and options - add the summary via `/ze-rfc` before implementation.
- [ ] RFC 4862 (IPv6 SLAAC) and RFC 8106 (RDNSS/DNSSL) inform the prefix and DNS options - add via `/ze-rfc` as needed.

**Key insights:**
- The RA message builder and raw-socket send loop already exist for BNG; the missing pieces are the LAN config surface and the Prefix Information option (RFC 4861 Section 4.6.2), which has no type in the codebase today.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ra.go` - `BuildRA` (ra.go:37) builds an ICMPv6 type-134 RA with header, flags, and RDNSS only; its comment (ra.go:34-36) states no Prefix Information is included because BNG uses DHCPv6-PD. There is no `PrefixInfo`/prefix-option type anywhere.
- [ ] `internal/component/l2tp/ppp/ra_linux.go` - `raSenderLoop` (ra_linux.go:100-138) sends RAs to `ff02::1` on a timer and on-demand to Router Solicitations, bound to the point-to-point `pppN` device; not a LAN broadcast sender.
- [ ] `internal/core/sysctl/known_linux.go` - only host-side RA behaviour is exposed (accept_ra at known_linux.go:73, autoconf at :69, forwarding at :77); nothing advertises prefixes.

**Behavior to preserve:**
- The BNG PPP RA path is unchanged (it intentionally sends prefix-less M+O RAs).
- Host-side RA acceptance sysctls keep working as today.

**Behavior to change:**
- A new per-interface RA sender advertises configured prefixes on a LAN, driven by config.

## Data Flow (MANDATORY)

### Entry Point
- Config: per-interface RA settings plus one or more advertised prefixes (new config surface, most likely a `router-advert` service or an iface IPv6 sub-block).

### Transformation Path
1. Config parsed into per-interface RA state (flags, intervals, lifetimes, prefixes, DNS, MTU).
2. An RA sender is started per configured interface, joining all-routers and listening for Router Solicitations.
3. RA messages are built with the header + flags + Prefix Information option(s) (new) + optional RDNSS/DNSSL/MTU.
4. Unsolicited RAs are sent on a randomised timer; solicited RAs answer Router Solicitations.
5. On config change/removal, senders are reconfigured or stopped (final RA with zero lifetime as appropriate).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ RA sender | per-interface RA state built from config | [ ] |
| RA sender ↔ kernel | raw ICMPv6 socket sends type-134 to `ff02::1` | [ ] |
| RA options ↔ hosts | Prefix Information option drives SLAAC on receivers | [ ] |

### Integration Points
- New `internal/plugins/radvd/` (or an iface IPv6 RA sub-block) - config surface + sender lifecycle.
- Reuse of the `BuildRA`/raw-socket/RS-listener pattern from `internal/component/l2tp/ppp/`.
- A new Prefix Information option type (absent today).

### Architectural Verification
- [ ] No bypassed layers (RA sent through a raw ICMPv6 socket like the BNG path)
- [ ] No unintended coupling (RA sender self-contained in its plugin)
- [ ] No duplicated functionality (reuse the RA builder/socket pattern, extended with prefix options)
- [ ] Registration over hardcoding - the RA feature registers as a plugin; no per-feature field in a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The BNG RA builder/socket pattern generalises to a LAN sender | ra.go / ra_linux.go exist | need a fresh RA stack | spike a LAN sender during DESIGN | unvalidated |
| A-2 | A Prefix Information option can be added cleanly to the RA builder | BuildRA is option-extensible | builder rework needed | prototype the option encoder | unvalidated |
| A-3 | RA sending coexists with host-side accept_ra settings | sysctl model | interface loops/conflicts | test forwarding-on + accept_ra-off in QEMU | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Misconfigured RA disrupts a LAN (wrong prefix/lifetime) | hosts get bad SLAAC addresses | validate prefix/lifetime; QEMU SLAAC test before ship |
| R-2 | RA sender and host RA-acceptance conflict on the same interface | routing loops / address churn | design guidance: forwarding on, accept_ra off on advertising interfaces |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set` RA on an interface with a prefix | → | RA sender emits type-134 with a Prefix Information option | `test/qemu/router-advertisement.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RA enabled with prefix P on an interface | periodic RAs carry P with on-link/autonomous flags |
| AC-2 | a host on the link | forms a SLAAC address from P (proven in QEMU) |
| AC-3 | a Router Solicitation arrives | a solicited RA is sent promptly |
| AC-4 | RDNSS configured | RA carries the RDNSS option |
| AC-5 | RA config removed | sender stops (final zero-lifetime RA) |
| AC-6 | invalid prefix/lifetime | config verify rejects |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | advertises a /64 to a LAN so hosts autoconfigure | config → RA sender → Prefix Information option → host SLAAC | `test/qemu/router-advertisement.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildRAPrefixOption` | `internal/plugins/radvd/ra_test.go` | Prefix Information option encodes per RFC 4861 | |
| `TestRAConfigParse` | `internal/plugins/radvd/config_test.go` | per-interface RA config parsed | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length | 0..128 | 128 | - | 129 |
| valid lifetime (s) | 0..4294967295 | 4294967295 | - | overflow rejected |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `router-advertisement` | `test/qemu/router-advertisement.ci` | host SLAAC from an advertised prefix, verified in QEMU | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| a Linux host autoconfigures from Ze's RA | - | Linux kernel host (QEMU) | RA + Prefix Information interops with a standard receiver | - |

### Future (if deferring any tests)
- Phasing: prefix advertisement + SLAAC first; RDNSS/DNSSL, MTU, route information options in follow-up sub-specs.

## Files to Modify
- `internal/core/sysctl/known_linux.go` - ensure IPv6 forwarding is set on advertising interfaces (design decision)
- iface IPv6 config surface - reference the new RA feature (design decision)

## Files to Create
- `internal/plugins/radvd/` - new plugin: config, RA sender lifecycle, prefix-option builder
- `internal/plugins/radvd/ra_test.go` - unit tests
- `test/qemu/router-advertisement.ci` - QEMU functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - full `/ze-spec` workflow: config surface, Prefix Information option encoding, sender lifecycle, RS handling, forwarding/accept_ra interaction, QEMU SLAAC test design, phasing. Not implementable as-is.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests are provisional placeholders for DESIGN.
- The Prefix Information option does not exist in the codebase yet; it is core to this feature and must be designed.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] QEMU SLAAC test passes
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] Registration over hardcoding reviewed (RA registers as a plugin)
- [ ] Interface/IPv6 docs updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (QEMU)
