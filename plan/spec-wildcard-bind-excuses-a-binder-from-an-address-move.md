# Spec: wildcard-bind-excuses-a-binder-from-an-address-move

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | `plan/immediate/spec-every-binder-root-stops-and-starts-around-an-address-move.md` |
| Phase | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Establish, per binder, whether it binds the wildcard address, and free the ones
that do from the stop and the start an address move costs them.

The owner stated the refinement on 2026-09-11, and
`docs/architecture/config/apply-ordering.md` quotes it verbatim:

> if you binded to 0.0.0.0 moving the IP is then a non-issue but you have to
> assume the binding will be specific

Nothing implements it, and the reason is that it frees nobody today. Ze's BGP
binds specific addresses, so the carve-out would apply to no BGP session. Every
other binder takes one coarse node and is never stopped at all, so there is
nothing to free there either until
`spec-every-binder-root-stops-and-starts-around-an-address-move` lands.

What is NOT established is the premise itself, for any binder except BGP. The
page says so under "What is not built", and it names one case that already looks
like the shape: `listenDHCP` binds `:67` and ties the socket to a device with
`SO_BINDTODEVICE`, which is a wildcard ADDRESS bind whose context is the device.
A carve-out written from the address alone would be wrong for it.

So the work is a reading first and a carve-out second, and the reading is what
decides whether the carve-out is one rule or several.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - the refinement in the
  owner's words, "The binding context", and "What is not built", which states
  what is unestablished.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/loader_create.go` - `CreateReactorFromTree`
  sets no global listen address, which is half of why the carve-out frees no BGP
  session.
- [ ] `internal/component/bgp/reactor/reactor.go` - `startMultiListeners` opens
  one listener per passive peer's local address, which is the other half.
- [ ] `internal/plugins/dhcpserver/socket_linux.go` - `listenDHCP` binds `:67`
  and ties the socket to a device, the one known case where the address is the
  wildcard and the context is not.
- [ ] `internal/component/bgp/plugin/operation.go` - `peerBindingDisturbed`, the
  only disturbance decision that exists today and the place a carve-out would
  sit for BGP.

**Behavior to preserve:**
- The fail-safe direction. A binding whose context cannot be established is
  treated as disturbed.

**Behavior to change:**
- A binder whose socket the move genuinely cannot reach is not stopped.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator moves an address. `DisturbedAddresses` computes the set and the
  planner carries it to each decomposing root.
- Format at entry: the disturbed address set on `DecomposeRequest`.

### Transformation Path
1. The root's decomposer reads its own bindings.
2. For each binding it asks whether this move can reach that socket.
3. A binding the move cannot reach emits no stop and no start.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Decomposer to its own listen call | the component reads its own config and its own socket options | No |

### Integration Points
- Each binder root's own decomposer. The core learns nothing new: the carve-out
  belongs to the component that knows how it binds.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A binder bound to `0.0.0.0` or `::` with no device context, and a commit moving an address | The binder is not stopped |
| AC-2 | A binder bound to the wildcard address and tied to a device, and a commit moving an address off that device | The binder IS stopped |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP moving an address while a wildcard-bound binder runs | → | that root's disturbance decision | to be named when the first wildcard binder is found |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be named | the root's own package | A wildcard binding with no device context is not disturbed | not written |
| To be named | the root's own package | A wildcard binding tied to a device IS disturbed when that device loses the address | not written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be named | `test/reload/` | A wildcard-bound service keeps serving across an unrelated address move | not written |

## Files to Modify
- The disturbance decision of each root that turns out to bind the wildcard.
- `docs/architecture/config/apply-ordering.md`, "What is not built", which loses
  the wildcard entry when the reading is complete.

## Implementation Steps

1. **Read each binder's own listen call** and record, per binder, the address it
   binds and the context that narrows it. That reading is the deliverable even
   if no carve-out follows it, because the page currently records the question as
   open and nothing answers it.
2. **Write the carve-out only where the reading supports it**, in the root's own
   decomposer, never in the core.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No config leaf changes |
| Functional test for new behavior | Yes | One per carve-out |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/apply-ordering.md`, "The binding context" and "What is not built" |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | At least one binder binds the wildcard with no narrowing context | `listenDHCP` binds `:67`, and its context is a device | The carve-out frees nobody and the deliverable is the reading alone | Reading each binder's listen call | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A carve-out written from the address alone leaves a binder holding an address that is gone | A service stops answering after a commit that was supposed to be free | The context, not the address, decides. A binding whose context cannot be established stays disturbed |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes
- [ ] Every binder's listen call is read, and the answer is recorded on the page

## Provenance

Homed here at the closure of `spec-config-apply-ordering-covers-every-root`
(2026-09-12), whose Work Not Done table names it. That spec verified the premise
for BGP alone and says so. Thomas has not commissioned this; it is written down
so the gap is countable.
