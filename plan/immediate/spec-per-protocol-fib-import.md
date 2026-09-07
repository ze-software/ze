# Spec: which protocols reach the FIB is per-protocol and operator-settable

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | spec-connected-static-reach-the-locrib (the Loc-RIB producers and the single FIB writer) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner directive, 2026-09-06: "every backend should come with a default of fib
kernel which can be changed per protocol (ie: to not import BGP routes into the
kernel but import OSPF and not ISIS)."

The first half is built and committed: a route producer declares
`Registration.NeedsDataPlane`, a FIB plugin declares the data plane it programs,
and the engine loads the writer for the data plane `interface { backend }`
selects, so a config with static routes and no `fib { }` block programs them.

The second half has no surface at all. Nothing in Ze lets an operator say which
PROTOCOLS reach the data plane. Every path the system RIB selects is published
to the FIB plugin, whatever produced it, so an operator who wants OSPF in the
kernel and BGP out of it has no way to say so and no way to see that they
cannot.

Goal: an operator names, per protocol, whether that protocol's routes are
written to the FIB; the default is that they are; and the vocabulary derives
from the protocol registry rather than from a hand-written list.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the cross-protocol RIB and the
      publish path this filter would sit on
- [ ] `docs/architecture/rib/unified-locrib.md` - `Path.Source` is a
      `redistevents.ProtocolID` and half of `Path.key`, so the system RIB
      already knows which protocol produced every path it selects
- [ ] `docs/architecture/redistribution.md` - the `redistribute { import }`
      vocabulary, which is the OTHER place in Ze that keys a decision on a
      protocol name
- [ ] `plan/immediate/spec-connected-static-reach-the-locrib.md` - the writer
      resolution this builds on, and the "OS-installed winner produces a
      withdraw" branch that already suppresses one class of FIB write

### RFC Summaries (Scope: config)
- [ ] N-A. No RFC governs which protocols an implementation programs into its
      forwarding table.

**Key insights:**
- Three surfaces key on a protocol name, and they are three STAGES of one
  pipeline rather than three copies of one decision (owner, 2026-09-06):
  selection asks which candidate path for a prefix WINS (`rib { distance { } }`
  feeding `selectBest`), redistribution asks which routes protocol A OFFERS to
  protocol B (`ImportRule`), and FIB programming asks which of the routes that
  WON are WRITTEN. The third does not exist. It MUST NOT be folded into
  distance, because no distance value means "wins the contest and is not
  installed", which is exactly the state the owner asked for.
- A withheld route is NOT a dropped route (owner, 2026-09-06). It stays in the
  Loc-RIB, stays selectable against other protocols on distance, stays
  redistributable, and stays on the plugin API bus. Only the FIB write is
  declined. That is the property BIRD and FRR do not have, because neither has a
  plugin bus to keep serving, and it is why this is not a port of either.
- The use case that justifies it: a controller or route-collector deployment
  that wants BGP in the RIB and on the bus without programming a single kernel
  route. Ze cannot express that today.
- Separating route selection from route programming is a natural way to think
  about a router, and few implementations expose it (owner, 2026-09-06). So an
  operator already holds the concept; what they lack is a place their previous
  tools let them express it. The `ze:help` names the thing plainly and says what
  each value does, in the vocabulary a network engineer already owns -- RIB,
  FIB, selection, programming -- with no justification and no analogy. The
  `docs/guide/` section says what the default is, what changing it does, and
  what withholding a protocol leaves intact. It owes no tutorial.
- The existing per-protocol vocabulary is already incomplete against the
  registry. `redistevents.RegisterProtocol` has ten non-test callers: `kernel`,
  `connected`, `isis`, `as112`, `static`, `ospf`, `ipsec`, `l2tp`, `bgp` and
  `bmp`. `internal/component/sysrib/yang/ze-rib-conf.yang` declares six distance
  leaves: `connected`, `static`, `ebgp`, `ibgp`, `ospf`, `isis`. So four
  registered protocols have no distance leaf, and `ebgp`/`ibgp` are a split the
  registry does not carry. A hand-written list of FIB-import leaves would repeat
  that, and go stale the same way.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/sysrib/sysrib.go` - `recomputeBest` selects the winner
      for a prefix and returns an `outgoingChange`; `publishChanges` emits the
      batch on `(system-rib, best-change)`. No branch anywhere consults the
      winner's protocol to decide WHETHER to publish, except the OS-installed
      branch the connected half added.
- [ ] `internal/core/rib/locrib/candidate.go` - `Path.Source` is a
      `redistevents.ProtocolID`, so the producing protocol is already carried on
      every path and is part of the key.
- [ ] `internal/core/redistevents/registry.go` - `RegisterProtocol(name)`
      allocates the ID; `ProtocolName` and `ProtocolIDOf` map between the two.
      This is the only complete list of protocols Ze has.
- [ ] `internal/component/config/redistribute/route.go` - `ImportRule` matches
      `Source`, `Destination`, `Families`, and `Tag` when `MatchTag` is set. Its
      destination is a PROTOCOL that imports, never the FIB.
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - the six distance
      leaves, hand-written, each with its own `ze:help`.

**Behavior to preserve:**
- The default: every protocol's routes reach the FIB. An operator who writes
  nothing gets what they get today.
- The single-writer property the static half established: one plugin owns a
  main-table entry.
- An operator's explicit `fib { }` block still decides the writer.

**Behavior to change:**
- A named protocol can be excluded from the FIB write.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config leaf, per protocol, whose container this spec must choose.

### Transformation Path
1. The config resolves to a per-protocol permission set.
2. The system RIB reads the winner's `Path.Source` when it publishes.
3. A winner whose protocol is excluded produces no FIB write.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to sysrib | the same configure callback that publishes the distance table | No |
| sysrib to FIB plugins | `(system-rib, best-change)`, suppressed for an excluded protocol | No |

### Integration Points
- `internal/component/sysrib` configure callback, which already resolves a
  per-protocol table from config and publishes it through a seam.
- `internal/core/redistevents` registry, the only complete protocol list.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Unknown | design not taken |
| No unintended coupling (components stay isolated) | Unknown | design not taken |
| No duplicated functionality (extends existing, does not recreate) | Unknown | this is the central open question: a third protocol-keyed vocabulary, or an extension of one of the two that exist |
| Zero-copy preserved where applicable (refs, not copies) | Unknown | design not taken |
| Registration over hardcoding | Unknown | the vocabulary MUST derive from `redistevents`, and how a static YANG file does that is an open question |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The winner's protocol is available at the point the FIB write is decided | `Path.Source` is a `redistevents.ProtocolID` and part of `Path.key` | the filter needs a new field on the change | reading `recomputeBest` and `outgoingChange` | unvalidated |
| A-2 | An excluded protocol's routes still belong in the system RIB, and only the FIB write is suppressed | owner, 2026-09-06: the withheld routes stay on the plugin API, in the Loc-RIB, redistributable and selectable | `show rib` loses the routes, or a plugin stops seeing them | a test that reads the Loc-RIB, the bus and `show rib` with the protocol withheld | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Three places keying on a protocol name drift apart | a protocol named in one and absent from the others | the design question below is settled BEFORE code |
| R-2 | A hand-written leaf list repeats the gap the distance leaves already have | a protocol with no leaf silently defaults | derive from `redistevents` or generate the YANG from it |
| R-3 | The gate is placed where it DISCARDS the path rather than declining to program it | a withheld protocol's routes vanish from `show rib`, from redistribution, or from the plugin bus | the gate sits at the write, not at insertion and not in selection. A test asserts the route is still present on every other surface |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The kernel forwarding table. An over-broad exclusion blackholes every prefix a protocol carries |
| How is it reverted? | Single commit revert; the leaves default to "permitted", so a reverted tree forwards exactly as before |
| Who else touches this path? | `spec-connected-static-reach-the-locrib` owns the publish path and the writer resolution; `spec-fib-depth` owns `BestChangeEntry.TableID` |

## The decision this spec exists to take

The semantics are settled. The owner has answered where the gate sits (at the
RIB-to-FIB boundary, after selection) and what withholding means (the route
lives on everywhere except the kernel). What is open is the SHAPE of the
operator's surface, and the two candidates come from the two peers.

| # | Decision | The options |
|---|----------|-------------|
| D-1 | Whether the filter belongs TO THE FIB WRITER or to a central per-protocol table | (a) BIRD's model: a property of the writer, so a leaf on the `fib { kernel { } }` block, and "which routes this writer accepts" is the writer's own question; (b) FRR's model: a central table keyed by protocol, so a container beside `rib { distance { } }` that every writer reads |

(a) fits Ze's structure, because the FIB writer is already a plugin that owns
its own config container and its own YANG. (b) puts the answer in one place for
an operator running two writers. Ze can run two writers at once
(`test/plugin/fib-vpp-coexist-with-fib-kernel.ci`), which is the fact that makes
this a real choice rather than a spelling.

### Peer evidence

| Peer | Shape | Verified |
|------|-------|----------|
| BIRD 2.14 | an export filter on the `kernel` protocol, matching the read-only enumerated `source` attribute: `protocol kernel { ipv4 { export filter F; }; }` with `if source = RTS_BGP then reject;` | MEASURED, 2026-09-07, `bird -p -c` on this host: the config parses with exit 0, and the same config with `RTS_BGP` replaced by an undefined symbol is refused with a syntax error, so the parse is not permissive |
| FRR 10.3.1 | reported as `ip protocol <proto> route-map <map>` applied in zebra | UNVERIFIED. `quay.io/frrouting/frr:10.3.1` is present on this host and zebra refuses to start without `cap_net_admin`, `cap_net_raw` and `cap_sys_admin`, which the sandbox does not grant. The spelling above is second-hand and MUST be measured before it is cited |

Neither peer's SEMANTICS transfer. In both, a route the kernel filter rejects is
simply not in the kernel and there is nothing else consuming it. In Ze it is
still on the plugin bus. Copy the spelling where it fits; do not copy the
meaning.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the config leaf D-1 chooses | → | the sysrib publish decision | named once D-1 is taken |
| an operator excluding one protocol on a booted appliance | → | the whole chain, ending at netlink | a QEMU test reading `ip route` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config that names no protocol | Every protocol's routes reach the FIB, exactly as today |
| AC-2 | A config excluding one protocol, and a prefix only that protocol offers | No kernel route exists for the prefix, and the path is still in the Loc-RIB, still in `show rib`, still redistributable, and still delivered on the plugin bus |
| AC-3 | A config excluding one protocol, and a prefix two protocols offer | The excluded protocol still WINS selection when its distance is lower, and the prefix is not programmed. Selection and programming are independent |
| AC-6 | A controller deployment: BGP withheld, every peer's routes in the RIB | Not one kernel route is programmed, and a plugin attached to the bus receives every route |
| AC-4 | A protocol that registers and that no leaf names | It defaults to permitted, and the vocabulary shows it without an edit to a hand-written list |
| AC-5 | A config naming a protocol nothing registered | The commit is refused and the message names the registered protocols |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Excludes BGP from the kernel while keeping OSPF | config -> sysrib -> publish decision -> fib-kernel | a `.ci` and a QEMU test, named once D-1 and D-2 are taken |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the default is permitted | `internal/component/sysrib` | AC-1 | |
| an excluded winner produces the D-2 outcome | `internal/component/sysrib` | AC-2, AC-3 | |
| an unnamed registered protocol is permitted | `internal/component/sysrib` | AC-4 | |
| an unregistered protocol name is refused | the config layer | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the leaves are boolean or a name list, not numeric | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| one protocol excluded, another kept | `test/static/` or `test/plugin/` | the owner's own example | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible behavior changes. The external system is the Linux FIB, and the QEMU test is the equivalent proof | |

## Files to Modify

Named once D-1 is taken. The candidates are
`internal/component/sysrib/yang/ze-rib-conf.yang`,
`internal/component/sysrib/sysrib.go`, `internal/component/sysrib/register.go`,
`internal/core/redistevents/registry.go` and
`internal/component/config/redistribute/route.go`.

## Files to Create

Named once D-1 is taken.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | the leaf D-1 chooses |
| YANG validation constraints | Yes | AC-5 refuses a name nothing registered |
| CLI commands/flags | No | no new command; `show rib` already prints the source |
| Doctor check for runtime dependencies | Unknown | an exclusion that leaves a prefix unrouted may owe one |
| BGP family surface | N-A | no family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | the guide page D-1 chooses |
| 6 | Has a user guide page? | Yes | a `docs/guide/` section: the default, what changing it does, and what withholding a protocol leaves intact -- the RIB, the plugin bus, redistribution and selection. A reader of that page already knows why a router might want this |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |

## Implementation Steps

1. **Phase: take D-1 with the owner**, after measuring FRR's spelling. Nothing
   below is implementable first.
2. **Phase: the vocabulary.** Derive the protocol names from `redistevents`
   rather than hand-listing them, and prove a newly registered protocol appears
   without an edit.
3. **Phase: the publish decision**, with the kernel read as the evidence.
4. **Phase: docs.**

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Registration over hardcoding | No hand-written protocol list is added; the vocabulary derives from `redistevents` |
| Correctness: the default | A protocol nothing names is PERMITTED, and that default is a named branch rather than a zero value |
| Rule: `ai/rules/principles.md` | The number of places keying on a protocol name did not go from two to three without D-1 saying why |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| the default is unchanged behavior | the existing sysrib and static suites pass untouched |
| the operator can reach it | a `.ci` covering the owner's own example |
| the kernel agrees | a QEMU test reading `ip route` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Fail-closed | A protocol whose permission cannot be resolved MUST be permitted and logged, never silently excluded: a silent exclusion blackholes every prefix that protocol carries |

### Failure Routing

| Failure | Route To |
|---------|----------|
| D-1 unanswered | STOP. This spec cannot start |
| Test fails on behavior mismatch | Re-read Current Behavior. If misunderstood, RESEARCH |

## Design Insights

- The system RIB already holds everything the filter needs. `Path.Source` is the
  producing protocol and it is part of the key, so no new field crosses any
  boundary for the filter itself. What is missing is the operator's vocabulary,
  not the plumbing.
- The connected half of `spec-connected-static-reach-the-locrib` already built
  one branch that suppresses a FIB write for a protocol: an OS-installed winner
  produces a withdraw rather than an install. That is a per-protocol write
  decision reached by DECLARATION rather than by config, and it is the nearest
  thing in the tree to what this spec asks for.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The gate sits at the RIB-to-FIB boundary, after selection | (a) a distance value meaning "never install"; (b) a filter at Loc-RIB insertion; (c) a redistribution rule with the FIB as destination | Owner, 2026-09-06: distance decides priority for resolution in the RIB, which then sends the routes to the FIB, so the two are sequential stages. No distance value means "wins and is not installed". (b) and (c) both make the route ABSENT rather than unwritten, which loses the plugin bus, `show rib` and redistribution -- the whole point of the feature |
| This is a spec rather than an extension of `spec-connected-static-reach-the-locrib` | folding it into the static work | It needs a config subtree that does not exist, a semantics decision nobody has taken (D-2), and an edit to the publish path that spec had already committed. Widening a nearly-green commit is how a day's work becomes unreviewable |

## Known Limitations

- `rib { distance { } }` names six protocols where ten register, and splits
  `bgp` into `ebgp`/`ibgp` where the registry does not. This spec must not
  repeat that, and repairing it is not in scope here.

## RFC Documentation (Scope: config)

N-A. No RFC governs which protocols an implementation programs into its
forwarding table.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] D-1 answered by the owner
- [ ] FRR's spelling MEASURED rather than cited from memory

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

<!-- Filled at implementation time by /ze-review (BLOCKING before closure).
     Never delete this section. -->

### Round 1
| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
