# Spec: wire-edit-3-aspath-fold-deferred-ebgp-wire-cache-removal -- delete the unreachable EBGP wire cache

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-02 |

Deferral holder created at the closure of `plan/learned/1319-wire-edit-3-aspath-fold.md` on 2026-08-02
(`ai/rules/deferral-tracking.md`, "Creating the Deferral Spec"). The source spec
was removed by its closure commit, so the work below lives here.

## Task

`plan/learned/1319-wire-edit-3-aspath-fold.md` AC-9 asked for the two atomic eBGP wire
slots to be gone once the AS-path rewrite became a generate slot. The rewrite
landed; the deletion did not.

What is left, and what it costs:

| Symbol | State |
|--------|-------|
| `ReceivedUpdate.EBGPWire` | exported, zero non-test callers |
| `ReceivedUpdate.ebgpSlotASN4` / `ebgpSlotASN2` | never populated in production |
| `ebgpWireSlot` | the struct they hold |
| the two slot-release branches in `recent_cache.go` | release buffers nothing ever stores |

There is no behavioral effect: the slots are never written, so the release
branches are no-ops and the cache is correct. The cost is dead code plus an
exported symbol with no non-test caller, which `ai/rules/wiring-completeness.md`
treats as a defect.

Deferred rather than done at closure because the deletion touches the read-pool
buffer lifetime in `recent_cache.go`, which is the exact area a prior fix had to
repair (child 3's own R-2 and R-3). It wants an implementation phase with the
poison-read soak, not a closure-time edit.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

- [ ] `docs/architecture/memory/lifetime-contracts.md` - the read-pool handle contract
- [ ] `plan/learned/1319-wire-edit-3-aspath-fold.md` - why the slots became unreachable

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; verify before trusting)

- [ ] `internal/component/bgp/reactor/received_update.go` (`EBGPWire`, `ebgpWireSlot`, `ebgpSlotASN4`, `ebgpSlotASN2`)
- [ ] `internal/component/bgp/reactor/recent_cache.go` (the two slot-release branches)

**Behavior to preserve:** every read-pool buffer is still returned exactly once, and no cache eviction path loses a handle.

## Data Flow (MANDATORY)

### Entry Point
Cache eviction of a `ReceivedUpdate`, and any surviving caller of `EBGPWire`.

### Transformation Path
(fill during design)

### Boundaries Crossed
| From | To | Format |
|------|----|--------|
| (fill during design) | (fill during design) | (fill during design) |

### Integration Points
| Point | Component |
|-------|-----------|
| (fill during design) | (fill during design) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing outside the tests reaches `EBGPWire` or the two slots. | `grep -rn "\.EBGPWire(" internal/ cmd/ pkg/` on 2026-08-02 returned test files only. | The caller found is the reason the slots exist, and the deletion becomes a behavior change rather than a cleanup. | (fill during design) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Removing the release branches while a path still stores a handle would leak a read-pool buffer per evicted entry. | (fill during design) | (fill during design) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A received UPDATE forwarded to eBGP peers, then evicted from the cache | -> | no slot is written and no handle is leaked | existing `internal/component/bgp/reactor/forward_readbuf_leak_test.go`, extended |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | After the change | `grep -rn "EBGPWire\|ebgpWireSlot\|ebgpSlot" internal/` returns nothing |
| AC-2 | A soak under the debug buffer poison | No poison read and no leaked read-pool handle |
| AC-3 | The existing forward and route-server `.ci` corpus | Unchanged, with no expectation edited |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestReceivedUpdateReleasesEveryAdoptedHandle | `internal/component/bgp/reactor/forward_readbuf_leak_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `bgp-rs-fastpath-ebgp-shared` | `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | eBGP forward is byte-identical after the dead cache is deleted | |

## Files to Modify
- `internal/component/bgp/reactor/received_update.go`
- `internal/component/bgp/reactor/recent_cache.go`
- `internal/component/bgp/reactor/received_update_test.go`
- `internal/component/bgp/reactor/forward_body_test.go`

## Implementation Steps

1. (fill during design)

## Checklist

### Goal Gates (MUST pass)
- [ ] Every AC demonstrated
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
