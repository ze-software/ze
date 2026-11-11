# Spec: rsvpte-vpp-dataplane

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | RSVP-TE (RFC 3209, RFC 4090) is a shipped capability, and on a VPP data plane two of its parts have no form or no proof: facility-backup control contexts are refused, and transit swap forwarding is unit-tested only |
| RFC gaps | - |
| Depends | `plan/release/validation/spec-fib-depth.md` (Phase 13) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/release/validation/spec-fib-depth.md` Phase 13, which makes
fib-vpp consume `(mpls-fib, entry)` label entries. Two RSVP-TE items it does
not do:

1. **Facility-backup control contexts on VPP.** RSVP-TE installs a push entry
   with a nonzero `TableID` so the PATH it sends toward a merge point through a
   bypass is selected by packet mark into a private kernel table
   (`docs/architecture/mpls/mpls-kernel.md`, the `0x5a000000/0xffff0000`
   namespace). fib-vpp refuses such an entry (spec-fib-depth AC-24), so local
   repair on a VPP data plane cannot send its control traffic through the
   bypass. The design has to answer where Ze's locally originated RSVP traffic
   is routed on a VPP box (the Linux kernel through the LCP tap,
   `docs/guide/vpp.md`, or VPP) and program the context there, or refuse the
   configuration at verify.
2. **Real-VPP transit swap forwarding proof.** spec-fib-depth proves swap and
   the backup stack through fib-vpp's subscription with a fake GoVPP channel,
   and proves a pop on real VPP. A swap on real VPP needs an RSVP-TE peer on
   each side, which the single-container deployment harness does not hold. The
   freeRouter RSVP-TE suite (`test/interop-rsvpte/scenarios/`) is the candidate
   peer.

Every section below is owed when the spec moves to `design`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/mpls/mpls-kernel.md` -- private push contexts and their mark guards
- [ ] `docs/architecture/rsvpte/mpls-rsvp-te.md` -- "Dataplane", "Interop evidence"

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/rsvpte/fib.go` -- `programPush` carries the private `TableID` of a bypass control context
- [ ] `internal/plugins/fib/kernel/mplsentry_linux.go` -- the kernel transit swap the VPP form must match

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|---------------------------------------------------------------|
| Private push contexts | `docs/architecture/mpls/mpls-kernel.md`, RSVP bypass paragraph | Mark-selected kernel tables for bypass control traffic | to be decided at design: extend to the VPP data plane or refuse at verify |

## Data Flow (MANDATORY)

### Entry Point
- `rsvp-te { }` with a bypass on a router whose `interface { backend }` is `vpp`

### Transformation Path
1. Written at design

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RSVP-TE to the label writer | `(mpls-fib, entry)` | [ ] |

### Integration Points
- `mplsfib.Entry.TableID`, the private context a facility-backup push names

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Written at design | → | Written at design | Written at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Written at design | Written at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Written at design | Written at design | Written at design | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Written at design | Written at design | Written at design | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Written at design | `test/interop-rsvpte/scenarios/` | freeRouter | Transit swap forwarded by real VPP | |

## Files to Modify
- Written at design

## Files to Create
- Written at design

## Implementation Steps

Written at design.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The targeted tests for what changed pass in the main checkout: `./le test unit pkg path ./internal/plugins/rsvpte/`, `./le test unit pkg path ./internal/plugins/fib/vpp/`, `./le test integration interop-rsvpte`, `./le test deployment vpp`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features
