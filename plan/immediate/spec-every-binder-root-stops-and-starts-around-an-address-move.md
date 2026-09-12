# Spec: every-binder-root-stops-and-starts-around-an-address-move

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make phases 2 and 5 of the config apply order reach every component that binds a
local IP address, not BGP alone.

The order is the owner's and it is quoted verbatim in
`docs/architecture/config/apply-ordering.md` under "The requirement". That page
is the specification for this work. Its five phases are: stop the binders the
commit removes, stop the binders whose bound address is disturbed, remove the
addresses the commit removes, add the addresses it adds, start the binders
against the new addresses.

Phases 1, 3, 4 and 5 hold for every root today. Phase 2 reaches a binder that
DECOMPOSES, and BGP is the only one. The reason is structural, and the page
states it under "What is not built": both phases need one binder to take TWO
steps in one commit, a stop before the addresses move and a start after, and a
participant with no decomposer gets exactly one coarse section-apply node, which
cannot be split in two.

So an operator who moves an address that ike, l2tp, dhcp, ntp, gnmi, tftp or a
plugin listener binds gets that binder STARTED against the new address and never
STOPPED before the old one goes. The socket holding the old address is still open
while the kernel takes the address away.

The core is built and this spec does not rebuild it. What is missing is one
decomposer per binder root, each deciding for itself what stopping means.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/apply-ordering.md` - the requirement in the
  owner's words, the five phases, and "What is not built", which is the entry
  this spec exists to close.
- [ ] `docs/architecture/config/transaction-protocol.md` - the participant,
  verify, apply and rollback protocol a decomposer sits inside.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is `config`. No RFC governs the config transaction
ordering, and the plugin RPC contract is Ze's own.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/transaction/operation.go` - `DisturbedAddresses`
  reads every address a planned destroy produces. `OperationDecomposerRoots`
  names the roots the planner asks on its second pass, which is every root that
  registered a decomposer rather than every root with a diff.
- [ ] `internal/component/bgp/plugin/operation.go` - the only worked example.
  `peerBindingDisturbed` decides whether a peer's binding is disturbed,
  `peerAddressConsumes` makes the stop and the start declare the addresses that
  order them, and `bgpPeerLocalAddress` turns an absent or `auto` source into the
  fail-safe answer.
- [ ] `internal/component/plugin/server/reload_tx.go` - `operationPlannerFromTrees`
  runs the second decompose pass and `checkDisturbanceSettled` refuses a plan
  whose second pass disturbs a different set from its first.
- [ ] `internal/component/plugin/server/reload.go` - `appendDecomposingPlugins`
  joins every running plugin that declares a decomposition to the transaction
  with no section, so a binder with no diff of its own can still be asked.
- [ ] `internal/component/config/transaction/solver.go` - `operationPhase` puts an
  operation on one of three rungs, and `kahnSort` drains the lowest ready rung,
  which is what makes a stop precede an addressing change and a start follow it.

**Behavior to preserve:**
- A commit that disturbs nothing costs a decomposing participant nothing: the
  planner makes one pass and the participant receives no event.
- The fail-safe direction. Where the core cannot establish whether a binding
  context still means what it meant, the binder is treated as disturbed.
- The core reads no root's semantics and keeps no list of binders.

**Behavior to change:**
- Each binder root emits its own stop and its own start for a binding whose
  address the commit disturbs, in place of the single coarse node it takes today.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator edits the config file and sends SIGHUP, or commits from the CLI
  editor. `Server.reloadConfig` computes the diff.
- Format at entry: per-root JSON sections in `transaction.DiffSection`.

### Transformation Path
1. The planner decomposes every root with a diff.
2. `DisturbedAddresses` reads the planned destroys and returns the disturbed set.
3. The planner decomposes a second time, carrying that set to every root that
   registered a decomposer, whether or not that root has a diff.
4. The new decomposer answers with a stop and a start for each binding it holds
   against a disturbed address, each declaring what it consumes.
5. The derived edges and the three rungs place them around the address
   operations.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Core to owning component | `DecomposeRequest.DisturbedAddresses`, answered with `ConfigOperation` values | No |
| Owning component to its own runtime | whatever that component's stop and start already are | No |

### Integration Points
- `transaction.RegisterOperationDecomposer` - the root joins by registering, and
  the core keeps no list.
- The component's own apply path, which must accept a stop and a start as
  operations rather than as a whole-section apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One reload moves an address a non-BGP binder holds, with no diff in that binder's own root | The binder is stopped before the address leaves and started after it arrives |
| AC-2 | One reload changes that address in a way that leaves the address row intact | No binder is stopped |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP moving an address a non-BGP binder holds | → | that root's decomposer, through the planner's second pass | one `.ci` per root, to be named when the root is chosen |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be named per root | the root's own package | The decomposer emits a stop and a start for a disturbed binding, and nothing for an undisturbed one | not written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be named per root | `test/reload/` | The binder's socket is closed before the address it held is removed | not written |

## Files to Modify
- One `operation.go` per binder root, registering a decomposer for that root.
- That root's apply path, to accept the two operations.

## Implementation Steps

1. **Pick the order the roots are done in.** That is a question about which
   binder an operator is most likely to bind to a moving address, and it is the
   owner's to answer.
2. **One root per pass**, each with its own unit tests and its own `.ci`.

Two things each root owes, and neither is generic:

- **What "disturbed" means for it.** BGP reads each peer's
  `connection.local.ip` and falls back to "stopped whenever ANY address moves"
  where the kernel picks the source. A DHCP server tied to a device with
  `SO_BINDTODEVICE` answers a different question.
- **What a stop and a start cost.** A BGP session restart is visible to a peer.
  A TFTP listener restart is not. The decomposer decides, and that cost is what
  makes the fail-safe direction affordable or not.

Roots that bind a local address and register no decomposer today: `ike`, `l2tp`,
`dhcp`, `ntp`, `gnmi`, `tftp`, and every plugin that opens a listener of its own.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No config leaf changes. The ordering is derived from the diff |
| Functional test for new behavior | Yes | One `.ci` per root |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/apply-ordering.md`, "What is not built", loses the entry each root closes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each binder can be stopped and started without rebuilding the whole component | the BGP precedent, `applyBGPOperation` | The root needs a different decomposition than the BGP one | Reading the root's own apply path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stop and a start cost more than the disturbance they protect against | An operator reports a service bounce on an unrelated commit | The decomposer's own disturbance test is what bounds it, and it is per root |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Goal Gates (MUST pass)
- [ ] `./le verify worktree` passes
- [ ] `docs/architecture/config/apply-ordering.md` "What is not built" is shorter by the root this pass closed

## Provenance

Homed here at the closure of `spec-config-apply-ordering-covers-every-root`
(2026-09-12), which built the ordering core and states in its Work Not Done table
that it did not do this. Thomas has not commissioned it; it is written down so the
gap is countable.
