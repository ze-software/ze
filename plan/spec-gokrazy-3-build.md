# Spec: gokrazy-3 -- Gokrazy Build Config

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-gokrazy-1-dhcp-wiring, spec-gokrazy-2-ntp |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-gokrazy-0-umbrella.md` -- parent spec
3. `gokrazy/ze/config.json` -- current gokrazy config
4. `docs/guide/appliance.md` -- appliance documentation

## Task

Modify gokrazy appliance config to exclude gokrazy's built-in DHCP and NTP
packages. Update the seed config to include DHCP and NTP configuration for ze.
Update documentation. Verify with QEMU boot test.

This spec cannot be implemented until both spec-gokrazy-1 (DHCP wiring) and
spec-gokrazy-2 (NTP plugin) are complete and verified.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/appliance.md` -- gokrazy build, QEMU testing, deployment
  -> Constraint: rootfs read-only, persistent /perm, A/B updates
- [ ] `gokrazy/ze/config.json` -- current config structure
  -> Constraint: no GokrazyPackages field currently (uses defaults)

### RFC Summaries (MUST for protocol work)
- N/A -- no protocol work in this spec

**Key insights:**
- Adding GokrazyPackages with only randomd and heartbeat excludes dhcp and ntp
- WaitForClock must be removed since ze now owns the clock
- Seed config in ExtraFileContents must include DHCP and NTP ze config

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/config.json` -- Packages: [serial-busybox, ze], no GokrazyPackages
- [ ] `docs/guide/appliance.md` -- documents gokrazy init as providing DHCP, NTP

**Behavior to preserve:**
- Build process (make ze-gokrazy) unchanged
- SSH and web server configuration in seed config
- Serial console, kernel, firmware settings

**Behavior to change:**
- Add GokrazyPackages field excluding cmd/dhcp and cmd/ntp
- Remove WaitForClock from ze PackageConfig
- Add DHCP and NTP config to seed config (ExtraFileContents)
- Update docs to reflect ze-owned DHCP/NTP

## Data Flow (MANDATORY)

### Entry Point
- `gokrazy/ze/config.json` read by gok build tool

### Transformation Path
1. gok reads config.json, uses GokrazyPackages list (now explicit)
2. Image built with only randomd + heartbeat as system packages
3. Ze starts, reads seed config from /etc/ze/ze.conf
4. Seed config enables DHCP on primary interface and NTP
5. Ze acquires IP via DHCP, syncs clock via NTP, starts BGP

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| config.json -> gok build | GokrazyPackages parsed by gok | [ ] |
| Seed config -> ze startup | ExtraFileContents written to /etc/ze/ze.conf | [ ] |

### Integration Points
- gokrazy build tool reads GokrazyPackages
- Ze reads seed config at startup
- DHCP plugin starts from interface config
- NTP plugin starts from environment config

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-gokrazy` with new config.json | -> | Image built without dhcp/ntp | Manual: `make ze-gokrazy` succeeds |
| QEMU boot of image | -> | Ze gets IP, syncs clock | Manual: `make ze-gokrazy-run` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-gokrazy` with new config | Image builds successfully |
| AC-2 | Image does not contain gokrazy dhcp binary | `gokrazy/ze/config.json` has explicit GokrazyPackages |
| AC-3 | Image does not contain gokrazy ntp binary | Same as AC-2 |
| AC-4 | QEMU boot | Ze starts, acquires DHCP lease |
| AC-5 | QEMU boot | Ze syncs clock via NTP |
| AC-6 | QEMU boot | Ze SSH accessible |
| AC-7 | Seed config includes DHCP | interface section has dhcp enabled |
| AC-8 | Seed config includes NTP | environment section has ntp enabled |
| AC-9 | WaitForClock removed | PackageConfig for ze has no WaitForClock |
| AC-10 | docs/guide/appliance.md updated | Documents ze-owned DHCP and NTP |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | Config-only change, no unit tests | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| gokrazy-build | Manual | `make ze-gokrazy` succeeds | |
| gokrazy-qemu | Manual | `make ze-gokrazy-run` boots, gets IP | |

### Future
- Automated QEMU boot test in CI

## Files to Modify

- `gokrazy/ze/config.json` -- add GokrazyPackages, update seed config, remove WaitForClock
- `docs/guide/appliance.md` -- update for ze-owned DHCP/NTP

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| Functional test | [ ] | Manual QEMU test |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/appliance.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- None

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Verify specs 1 and 2 are done |
| 3. Implement | Edit config.json, edit docs |
| 4. Full verification | `make ze-gokrazy` |
| 5. Critical review | QEMU boot test |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Config change** -- edit gokrazy/ze/config.json
   - Add GokrazyPackages with randomd and heartbeat only
   - Remove WaitForClock from ze PackageConfig
   - Update seed config to include DHCP and NTP sections
   - Verify: `make ze-gokrazy` builds

2. **Phase: QEMU test** -- boot and verify
   - `make ze-gokrazy-run`
   - Verify: ze acquires IP, syncs clock, SSH works
   - Document results

3. **Phase: Documentation** -- update appliance guide
   - Remove references to gokrazy-provided DHCP/NTP
   - Add section on ze's DHCP and NTP configuration
   - Verify: docs accurate

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N verified |
| Correctness | GokrazyPackages excludes dhcp and ntp |
| Config | Seed config has valid DHCP and NTP sections |
| Docs | appliance.md updated |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| GokrazyPackages in config.json | grep GokrazyPackages gokrazy/ze/config.json |
| No WaitForClock | grep -v WaitForClock gokrazy/ze/config.json |
| Image builds | make ze-gokrazy exits 0 |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Credentials | HTTPPassword still present, not removed |
| Seed config | No secrets in DHCP/NTP config |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Build fails | Check if specs 1 and 2 are truly done |
| QEMU boot fails | Debug ze startup logs |
| No DHCP lease | Check seed config DHCP section |
| 3 fix attempts fail | STOP. Ask user. |

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

(to be filled during implementation)

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] `make ze-gokrazy` passes
- [ ] QEMU boot successful
- [ ] Docs updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Minimal coupling

### TDD
- [ ] N/A (config-only change)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-3-build.md`
- [ ] Summary included in commit
