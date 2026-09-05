# Spec: prefix-count-installed-proven-against-a-real-peer

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The gap.** The `prefix { count installed; }` mode is proven only against the
`ze-peer` harness. `test/interop/scenarios/bgp-max-prefix-cease-frr` and
`test/interop/scenarios/bgp-max-prefix-per-family-frr` exercise a real FRR peer
against the DEFAULT mode alone: each `ze.conf` writes a `prefix { maximum N; }`
stanza and neither writes `count installed`, so no interop scenario configures
the mode at all.

**Why that is the wrong place to have no coverage.** The mode's whole purpose is
that a real peer churning attributes does not walk the count into its maximum.
Attribute churn is what a live peer produces and what a scripted harness does
not. `applyInstalledPrefixSections`
(`internal/component/bgp/reactor/session_prefix.go`) is the producer, and it is
a SET rather than a tally: each installed family holds the wire identity of
every NLRI the peer currently advertises, so a re-announcement of a prefix the
peer already has moves nothing, a withdrawal of a prefix ze never held moves
nothing, and a route refresh that replays the peer's whole table moves nothing.
That immunity is asserted today only by
`TestPrefixCountInstalledIsImmuneToReannounce` and
`test/plugin/prefix-count-installed-reannounce.ci`, both of which send the churn
ze itself decides to send.

**Why it is coverage rather than a defect.** Triaged on 2026-08-30 as an
improvement, not a release defect: the mode is correct against every message
shape tested, and the goal of the fix that found this (a refused message moves
no installed set) holds without a new scenario.
`ai/rules/interop-and-goal-validation.md` requires an interop test for protocol
behavior, and this is that requirement outstanding.

**What the work is.** One named FRR scenario that:

1. brings up a session with `count installed` configured on at least one family;
2. has FRR change an attribute on a prefix it already advertises, repeatedly,
   so the re-announcement arrives as an implicit withdraw (RFC 4271 Section 3.1);
3. asserts the count holds still across that churn, and that the session stays
   up under a maximum that a tally would have crossed;
4. discriminates: reverting the family to `count offered` makes the scenario go
   RED. `ai/rules/interop-and-goal-validation.md` requires the red to be
   OBSERVED and recorded, not asserted, and the artifact the test drives has to
   be rebuilt for the revert to take effect.

**Naming.** The scenario directory is NAMED and carries no numeric prefix, the
way the two existing ones do.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - the suites, their scenario directories, the native action, and the four vacuity traps
  → Decision: [fill during research]
  → Constraint: [fill during research]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - implicit withdraw, Section 3.1
  → Constraint: a re-announcement replaces the previous route for that NLRI, so the set size does not move

**Key insights:** (minimal context to resume after compaction)
- The two existing FRR max-prefix scenarios are the template; neither configures the mode under test.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `applyInstalledPrefixSections` settles every `PrefixCountInstalled` family of one UPDATE against a set of wire identities; a family stops taking new prefixes once its set passes its maximum, so the reported count is maximum plus one; a refused message moves no set, because `rollbackPrefixSets` undoes every mutation the message made.
- [ ] `test/interop/scenarios/bgp-max-prefix-cease-frr/ze.conf` - configures `prefix { maximum 1; }` and no counting mode.
- [ ] `test/interop/scenarios/bgp-max-prefix-per-family-frr/ze.conf` - configures two per-family maximums and no counting mode.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Both existing FRR scenarios keep testing the default mode unchanged.
- `applyInstalledPrefixSections` is not changed by this spec: it is the code under test.

**Behavior to change:** (only what the user asked for)
- None in the product. This spec adds interop coverage.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- FRR announces, re-announces and withdraws prefixes on an established BGP session, as wire UPDATE messages.
- Ze's configuration enters as a `ze.conf` with a `prefix` stanza naming `count installed`.

### Transformation Path
1. Wire parsing and per-family NLRI splitting.
2. `checkPrefixLimits` reads the message's sections and keeps the journal alive until the whole message has an answer.
3. `applyInstalledPrefixSections` settles each installed family's set.
4. The count reaches the warning threshold, the `ze_bgp_prefix_count` gauge and, over the maximum, a NOTIFICATION.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| FRR ↔ ze | BGP UPDATE messages over TCP in the interop lab | No |
| Reactor ↔ metrics | the prefix count gauge | No |

### Integration Points
- `interoplab.Discover` - matches the scenario directory by name.
- `./le integration` - the native action that takes the scenario selector.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | FRR can be scripted to churn an attribute on a prefix it already advertises | FRR route-map and static route reconfiguration | the churn has to come from another peer daemon | building the scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The scenario passes against both modes, so it proves nothing | the discrimination run stays green under `count offered` | the discrimination walk is part of the acceptance criteria, and the red is recorded |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing in the product; a scenario that does not discriminate costs lab time and false confidence |
| How is it reverted? | delete the scenario directory |
| Who else touches this path? | any spec changing the prefix counting modes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| FRR churns an attribute on an already-advertised prefix | → | `applyInstalledPrefixSections` | the named FRR scenario, run by `./le integration` |
| the family is reverted to `count offered` | → | the offered tally path | the same scenario, observed RED |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | FRR re-announces a prefix with a changed attribute, under `count installed` | the prefix count does not move |
| AC-2 | the churn repeats past what a tally would need to cross the maximum | the session stays established |
| AC-3 | the family is reverted to `count offered` and the artifact rebuilt | the scenario goes RED, and the red is recorded |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixCountInstalledIsImmuneToReannounce` | `internal/component/bgp/reactor/session_prefix_count_test.go` | the existing unit assertion this scenario extends to a real peer | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-count-installed-reannounce` | `test/plugin/prefix-count-installed-reannounce.ci` | the existing harness-driven churn | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-max-prefix-count-installed-frr` | `test/interop/scenarios/` | FRR | a live peer's attribute churn does not walk the installed count into its maximum | | <!-- doc-links: ignore (directory this skeleton plans and has not created yet) -->

## Files to Modify
- `docs/architecture/testing/interop.md` - the scenario list gains the new name

## Files to Create
- `test/interop/scenarios/bgp-max-prefix-count-installed-frr/ze.conf` - ze side, with `count installed` <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->
- `test/interop/scenarios/bgp-max-prefix-count-installed-frr/frr.conf` - FRR side, with the churn <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| Functional test for new RPC/API | | N-A: no new RPC; the coverage is an interop scenario |
| BGP family surface (new SAFI / capability / attribute) | | N-A: no new family |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | | `docs/functional-tests.md`, `docs/architecture/testing/interop.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- create the scenario directory and make `interoplab.Discover` find it
   - Tests: the scenario runs and fails for the right reason
   - Files: the two conf files above
   - Verify: `./le integration` selects the scenario by name
2. **Phase: The churn** -- FRR changes an attribute on an advertised prefix, repeatedly
3. **Phase: Discrimination** -- revert the family to `count offered`, rebuild, observe and record the RED

## Known Limitations
- [filled at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)
