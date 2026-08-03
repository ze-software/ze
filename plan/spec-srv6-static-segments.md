# Spec: srv6-static-segments

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
3. `internal/plugins/fib/kernel/nexthop_linux.go` - the single-SID seg6 encap builder
4. `internal/plugins/static/model.go` - static route model (no segments today)
5. `ai/rules/platform-linux.md` - seg6 encap is Linux-only; QEMU tests mandatory

## Task

**Large feature area - skeleton only. Full design not started.**

Ze's SRv6 support is BGP-driven and single-SID: a route carries exactly one IPv6
service SID end to end, installed as a one-entry seg6 lwtunnel (kernel) or an SR steer
(VPP). Two locally-driven SRv6 capabilities are absent:
- A **static route with an SRv6 segment list** (steer a prefix through an explicit list
  of SIDs), not just a single learned SID.
- A **global SRv6 encapsulation source-address** for the outer IPv6 header used when
  encapsulating into an SRH.

Implement local SRv6 steering config:
- Extend the static route model with a segment list (ordered SIDs) that installs a
  multi-segment seg6 encap (kernel) and an equivalent SR policy (VPP).
- Add a global SRv6 encapsulation source-address setting applied to the encap.

This requires multi-SID plumbing that even the BGP path lacks today, plus a new
seg6-encap path in the static netlink backend. It must go through the full `/ze-spec`
RESEARCH/DESIGN workflow. This skeleton tracks the gap; it is NOT ready to implement.

## Required Reading

### Architecture Docs
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - how a single-SID seg6 encap is built today.
  → Constraint: the segment list must extend `buildSEG6Encap` to N segments, not fork a new encoder.
- [ ] `internal/plugins/static/backend_linux.go` - the static netlink install path (no encap today).
  → Constraint: static routes must gain a seg6-encap path parallel to the existing gateway/multipath install.
- [ ] `ai/rules/platform-linux.md` - seg6 encap and forwarding are kernel behaviours; QEMU integration tests mandatory.
  → Constraint: never skip QEMU tests for "needs hardware".

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9252.md` - BGP SRv6 service SIDs (the existing single-SID control plane this extends).
- [ ] RFC 8986 (SRv6 Network Programming) and RFC 8754 (IPv6 Segment Routing Header) - add via `/ze-rfc` before implementation.

**Key insights:**
- Ze already installs a single seg6 SID; the gap is a segment LIST plus a local (static) origin for it and an encap source-address.
- The BGP SR-Policy NLRI encoder already models segment lists on the wire, but that is control-plane advertisement, not local forwarding-plane install.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/fib/kernel/nexthop_linux.go` - `buildSEG6Encap` (nexthop_linux.go) builds a `netlink.SEG6Encap` with a single-entry `Segments` list from one `netip.Addr`; there is no multi-SID path.
- [ ] `internal/plugins/static/model.go` - `staticRoute` (static/model.go) has Prefix, Table, Description, Metric, Tag, Action, NextHops; there is no segments/SID field.
- [ ] `internal/plugins/fib/vpp/srv6.go` - `addSRv6Steer` (fib/vpp/srv6.go) steers a prefix to a single SID via the VPP SR API; no segment list.

**Behavior to preserve:**
- BGP-driven single-SID SRv6 install (kernel and VPP) is unchanged.
- Existing static routes (gateway/interface/blackhole/reject) behave exactly as today.

**Behavior to change:**
- Static routes can carry an ordered SRv6 segment list installed as a multi-segment seg6 encap; a global encap source-address is applied.

## Data Flow (MANDATORY)

### Entry Point
- Config: a static route with a `segments` list of IPv6 SIDs (new leaf), plus a global SRv6 `encapsulation source-address` (new leaf).

### Transformation Path
1. The static route config is parsed into a route with an ordered SID list (new model field).
2. The global encap source-address is read into SRv6 settings.
3. On install, the kernel backend builds a multi-segment `SEG6Encap` (extending the single-SID builder) and attaches it to the route; the VPP backend installs the equivalent SR policy.
4. The encap source-address is applied to the outer IPv6 header where the backend supports it.
5. Traffic to the prefix is encapsulated and steered through the segment list.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ static model | segments list + encap source parsed into the route/settings | [ ] |
| Static model ↔ netlink | multi-segment `SEG6Encap` attached to the route | [ ] |
| Static model ↔ VPP | equivalent SR policy install | [ ] |

### Integration Points
- `internal/plugins/static/` - config model, parser, YANG for the segment list; new seg6-encap install path.
- `internal/plugins/fib/kernel/nexthop_linux.go` - extend `buildSEG6Encap` to N segments.
- `internal/plugins/fib/vpp/srv6.go` - SR policy with a segment list.
- SRv6 global settings - encap source-address leaf and its application.

### Architectural Verification
- [ ] No bypassed layers (install via the existing FIB/netlink path, extended for segments)
- [ ] No unintended coupling (segment list lives in the static model + FIB, not scattered)
- [ ] No duplicated functionality (extend the single-SID encap builder, do not fork it)
- [ ] Registration over hardcoding - static SRv6 stays in the static plugin + FIB; no per-feature field in a core struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `netlink.SEG6Encap` accepts a multi-entry `Segments` list | buildSEG6Encap already uses the type | need a different lwtunnel encoder | spike a multi-segment install during DESIGN | unvalidated |
| A-2 | The VPP SR API supports an equivalent segment-list policy | fib/vpp/srv6.go uses the SR API | VPP path limited to single steer | read the VPP SR binding during DESIGN | unvalidated |
| A-3 | An encap source-address can be applied on both backends | kernel seg6 + VPP SR | source not settable on one backend | check both encoders during DESIGN | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Multi-SID support diverges kernel vs VPP behaviour | one backend steers differently | design a common segment-list model; QEMU + VPP parity tests |
| R-2 | Scope creep into full SRv6 traffic engineering | spec churn | strict scope: static segment list + encap source only; SR policy TE is out |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| static route with `segments` list | → | multi-segment `SEG6Encap` installed on the route | `test/qemu/srv6-static-segments.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | static route with a 2-SID segment list | route installed with a 2-segment seg6 encap |
| AC-2 | traffic to the prefix | encapsulated and steered through the SID list (proven in QEMU) |
| AC-3 | global encap source-address set | outer IPv6 source matches the setting |
| AC-4 | single-SID BGP route (existing) | unchanged behaviour |
| AC-5 | invalid SID in the list | config verify rejects |
| AC-6 | VPP backend | equivalent SR policy installed |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | steers a prefix through an explicit SRv6 path via static config | config → segment list → multi-segment seg6 encap | `test/qemu/srv6-static-segments.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildSEG6EncapMultiSegment` | `internal/plugins/fib/kernel/nexthop_linux_test.go` | N-segment `SEG6Encap` built | |
| `TestStaticRouteSegmentsParse` | `internal/plugins/static/config_test.go` | segment list parsed into the model | |
| `TestSRv6EncapSourceApplied` | `internal/plugins/fib/kernel/..._test.go` | encap source-address applied | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| segment count | 1..N (design cap) | design | 0 (no segments) | over the cap |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srv6-static-segments` | `test/qemu/srv6-static-segments.ci` | prefix steered through a SID list, verified in QEMU | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local encap/forwarding feature; validated by QEMU (kernel) and VPP parity, not a peer daemon | - | - | seg6 encap is a kernel/VPP dataplane behaviour | - |

### Future (if deferring any tests)
- Phasing: kernel multi-segment install first; VPP SR-policy parity and encap source-address in follow-up sub-specs.

## Files to Modify
- `internal/plugins/static/model.go` - add a segment-list field to `staticRoute`
- `internal/plugins/static/config.go`, `internal/plugins/static/yang/ze-static-conf.yang` - parse `segments`
- `internal/plugins/static/backend_linux.go` - attach a seg6 encap on install
- `internal/plugins/fib/kernel/nexthop_linux.go` - extend `buildSEG6Encap` to N segments
- `internal/plugins/fib/vpp/srv6.go` - segment-list SR policy

## Files to Create
- SRv6 global settings for the encap source-address (component to be decided in DESIGN)
- `test/qemu/srv6-static-segments.ci` - QEMU functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - full `/ze-spec` workflow: static segment-list model, multi-segment seg6 encoder, VPP SR-policy parity, encap source-address application, QEMU + VPP test design, scope boundary vs SR-Policy TE. Not implementable as-is.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests are provisional placeholders for DESIGN.
- Multi-SID support does not exist even in the BGP path today; the segment-list dataplane is the core new work.

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
- [ ] QEMU seg6 steering test passes
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)

### Quality Gates (SHOULD pass)
- [ ] Registration over hardcoding reviewed (static SRv6 in static plugin + FIB)
- [ ] Routing/SRv6 docs updated

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (QEMU)
