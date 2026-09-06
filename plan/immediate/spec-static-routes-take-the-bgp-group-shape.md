# Spec: static routes take the BGP group shape

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | spec-connected-static-reach-the-locrib (owns the arbitration point the new distance leaf needs) |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the owner asked for, 2026-09-06.** Static routes take the shape BGP peers
already have. A group carries the settings its members share, and a member
carries the NLRI. Two settings are named as shared: the next hop, and the route
preference against the other protocols. The member carries the prefix.

**What exists today.** `ze-static-conf.yang`
(`internal/plugins/static/yang/ze-static-conf.yang`) declares
`static { table <name> { route <prefix> } }`. Each `route` carries `prefix`,
`description`, `metric`, `tag`, and a mandatory `choice action` of `forward`,
`discard` or `unreachable`. The `forward` case holds `container next`, which
holds `list hop` and `list interface`. `table` names a kernel routing table and
carries no route setting, so two routes share nothing and each one writes its
own next hop.

**How BGP resolves a group.** `ResolveBGPTree`
(`internal/component/bgp/config/resolve.go`) flattens the group away before any
consumer sees it, and `PeersFromTree` (`internal/component/bgp/reactor/config.go`)
reads a flat peer map that no longer records where a value came from. Three
layers apply per peer, lowest first: the `bgp` block, the group's own fields,
then the peer's own fields. `deepMergeAt` is the merge. It writes only the keys
the higher layer holds, so an absent member leaf keeps the inherited value and a
present one replaces it. Four consequences decide this design.

| Merge behavior | Consequence for static |
|----------------|------------------------|
| Only keys present in the higher layer are written | A member cannot unset an inherited leaf |
| A YANG list lowers to a map keyed by its list key, and two levels holding the same list recurse into it | Two levels holding a next hop UNION into one ECMP set, they do not replace |
| `cumulativePaths` names the four leaf-list paths that accumulate | Accumulation is opt-in per path, and the default is replace |
| `Tree.ToMap` emits only what the tree holds, and `config.ApplyDefaults` runs per call site rather than at load | A YANG default never materializes as a member value that shadows the group |

### The nesting

`group` sits INSIDE `table`, beside `route`, and a bare `route` under `table`
stays legal. A grouped route resolves two layers, the group and the route. A
bare route resolves one. `table` gains no settings of its own: it names a kernel
table, and giving one keyword a second meaning costs the reader a guess on every
page (`ai/rules/writing.md`, habit 1). The `static` block gains no settings of
its own either, so BGP's layer 1 has no static counterpart and this spec adds
two layers rather than three.

| Question | Answer | Cost |
|----------|--------|------|
| `group` inside `table`, or beside it? | Inside. A table id is a forwarding property of every route in it, and a group that spanned tables would have to state which id each member uses | None. The nesting matches the config path an operator already types |
| Bare `route` under `table` kept? | Yes | The resolver runs the layer sequence twice, once for grouped routes and once for bare ones, which is the duplication `ResolveBGPTree` already carries. `route` is reachable at two depths, so an operator moves a route into a group by deleting it and adding it again |
| Bare `route` removed instead? | No | Every static route in the tree changes shape, 32 files below, and a single default route needs an invented group name |

### What is shared, and what is not

| Node | Group | Member | Which wins | Why |
|------|-------|--------|-----------|-----|
| `prefix` | No | Yes | n/a | It is the list key and the route's identity |
| `next` (`hop`, `interface`) | Yes | Yes | Member, replacing the whole container | Named by the owner. See the replace rule below |
| `distance` (new) | Yes | Yes | Member | Named by the owner. See the preference section |
| `metric` | Yes | Yes | Member | It is a scalar kernel priority, so the merge is unambiguous, and a set of routes toward one next hop shares it |
| `tag` | Yes | Yes | Member | A tag marks a set of routes for one policy rule, which is what a group is. See the tag section |
| `description` | No | Yes | n/a | It names one route. A note ON the group is a different concept and takes its own leaf, inherited by nothing |
| `choice action` | No | Yes | n/a | See below. Sharing it produces a silently wrong route |

**The member's `next` container REPLACES the group's, and the static resolver
states that rule rather than inheriting it.** A blind `deepMergeAt` unions the
two `hop` maps, because a YANG list lowers to a map keyed by address. An
operator who writes a next hop on a member means that this route goes there, not
that a second ECMP path is added. BGP unions `update` blocks and filter chains
on purpose, and the same default is wrong here.

**The group's `next` is applied to a member whose action is `forward` and which
names no hop of its own. It is not applied to a member whose action is
`blackhole` or `reject`.** That rule cannot be expressed by a key-by-key merge,
so the static resolver owns it.

### The action stays on the member

The `choice action` is not a group-level node, and the reason is mechanical
rather than stylistic.

`flattenChildren` and `flattenChoiceCases`
(`internal/component/config/yang_schema.go`) bypass the case wrapper, so at the
data layer `next`, `blackhole` and `reject` are plain siblings under `route` and
the built schema keeps no record that they exclude each other. `deepMergeAt`
cannot remove an inherited key, so a group declaring `blackhole` and a member
declaring `next` produce one merged route holding both. `parseRoute`
(`internal/plugins/static/config.go`) tests `blackhole`, then `reject`, then
`next`, and returns on the first hit. The merged route becomes a blackhole, the
member's next hop is discarded, and no error is raised. That is the failure
`ai/rules/principles.md` names first: a value that is silently wrong reaches a
caller that cannot tell it from an answer.

Hoisting the next hop out of the choice, as the section above does, is what lets
the owner's shared next hop exist without sharing the action.

### Preference is a new leaf, it is spelled `distance`, and lower wins

Ze declares one cross-protocol ranking vocabulary today. `rib { distance { } }`
(`internal/component/sysrib/yang/ze-rib-conf.yang`) carries one leaf per
protocol, its module description states that the lower distance wins, and
`static` defaults to 10. `internal/core/rib/distance` carries the declared value
from `sysrib` to the producers that stamp it. BIRD's `preference` is higher-wins.
Spelling the new leaf `preference` would put two names and two directions on one
concept, so the leaf is `distance`, `uint8`, range 1 to 255, lower wins, and it
overrides `rib/distance/static` for the group or the route that carries it.

**The comparison point does not exist yet, and this spec does not build it.**
Three producers were read.

| Producer | What it does | What it means |
|----------|-------------|---------------|
| `docs/architecture/static-routes.md`, "Direct FIB programming, not Loc-RIB injection" | Static routes are programmed straight into the FIB and never enter the Loc-RIB or sysrib | No Ze code ranks a static route against another protocol |
| `buildRichRoute` (`internal/plugins/fib/kernel/nexthop_linux.go`) and the static netlink backend (`internal/plugins/static/backend_linux.go`) | Both set `netlink.Route.Priority` from the route's metric, and both program protocol `RTPROT_ZE` | Today the kernel resolves a static route against an IS-IS route on METRIC, and no distance takes part |
| `effectivePriority` (`internal/component/sysrib/sysrib.go`) | Returns the declared distance whenever the schema names the protocol, and falls back to the stamped value only for a protocol the schema does not name | A per-route value carried in `Priority` would be discarded for `static`. It has to travel as its own field, and `effectivePriority` has to prefer it |

`plan/immediate/spec-connected-static-reach-the-locrib.md`, at Status `design`,
moves the static install into the Loc-RIB so that the declared distance
arbitrates. This spec depends on it. Landing the leaf first produces a setting
that parses, commits and decides nothing, which is the class
`plan/immediate/spec-config-leaf-consumption-gate.md` exists to prevent.

The owner's case reads as follows once both land: `rib { distance { static 120 } }`
demotes every static route below IS-IS at 115, and one group carrying
`distance 5` lifts its members back above it.

### Migration

Keeping the bare `route` under `table` costs the existing tree nothing. Every
config that writes a static route today stays valid, and only a config that
wants sharing is rewritten.

The count below is what REMOVING the bare `route` would cost, and it is the
number that decides the question above. 32 files carry a
`static { table { route } }` stanza. The five ExaBGP compatibility configs under
`test/exabgp-compat/etc/` are excluded: their `static { route ... }` is the
ExaBGP announcement block inside a neighbor, a different keyword with the same
spelling.

| Class | Count | Files |
|-------|-------|-------|
| Functional tests | 12 | 8 under `test/static/`, 3 under `test/parse/`, `test/editor/commands/set-list-key-keyword.et` |
| Interop scenarios | 4 | `as-path-prepend-two-octet-peer`, `bgp-redist-late-join-dynamic-frr`, `isis-redist-frr`, `ospf-redist-static-tag-frr`, each `ze.conf` |
| Generated and golden config | 2 | `contrib/netlab/ze/ze.j2`, `contrib/netlab/golden/r1.conf` |
| Go with an embedded config string or JSON path | 9 | `internal/plugins/static/config_test.go`, `config_table_test.go`, `doctor_test.go`; `internal/component/config/parser_test.go`, `toplugin_order_test.go`; `internal/component/cli/completer_test.go`, `editor_test.go`, `model_commands_test.go`; `internal/test/fixture/netfilter_fixture_static.go` |
| Documentation | 5 | `docs/architecture/static-routes.md`, `docs/guide/static-routes.md`, `docs/guide/configuration.md`, `docs/guide/policy-routing.md`, `docs/guide/quickstart.md` |

`ai/rules/cli.md` states that Ze is unreleased, so a replaced spelling is
replaced outright rather than aliased. That governs a spelling this spec
CHANGES. It does not require removing a spelling this spec keeps.

### The tag under the group shape

Commit `2bc0594b08` gave the `tag` leaf its two consumers: a redistribute import
rule selects routes by tag, and the tag becomes the External Route Tag of the
AS-external LSA when the route is redistributed into OSPF. Both read one value
per route.

`tag` is therefore the leaf a group most wants, and it is declared at both
levels with the member winning. `externalRouteTag`
(`internal/plugins/ospf/redist_wiring.go`) already resolves route over source,
returning the route's tag when it is non-zero and the per-source tag otherwise.
Adding the group makes one ladder of three rungs: route, then group, then the
per-source tag under the OSPF redistribute block.

**The group tag MUST be resolved inside the static plugin, before the route
reaches `RouteChangeEntry`.** `externalRouteTag` reads zero as "this route
carries no tag", so a group tag left unresolved would be invisible to it and the
per-source tag would win instead.

### Boundary with the route source attribute

The spec `spec-route-source-attribute`, written in parallel and not yet on
disk, owns the enumeration that names WHICH protocol produced a route. This
spec adds no member of that enumeration, renames none, and reads none. This spec owns the group level and the `distance`
leaf, which rank a route once its source is already known. Where the two meet is
the Loc-RIB path a static route becomes under
`plan/immediate/spec-connected-static-reach-the-locrib.md`: that path carries a
source from the other spec and a distance from this one, and neither spec
defines the other's field.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `path/to/file.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: wire bytes, API command, config, plugin message]
- [Format at entry]

### Transformation Path
1. [Stage 1: for example "Wire parsing in internal/component/bgp/message/"]
2. [Stage 2: ...]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [live sessions dropped / routes mis-encoded / config rejected / nothing user-visible] |
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | [what triggers the behavior] | [observable outcome] |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXxx` | `internal/.../xxx_test.go` | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-feature-peer` | `test/interop/scenarios/` | [FRR/BIRD/GoBGP/strongSwan] | [protocol behavior validated] | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/...` - [feature changes]

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/<this-spec>.md` lists them. A doc DECLARED by a changed file's `// Design:` header BLOCKS until named here; a doc that only `<!-- source: -->` mentions it is advisory. Naming it as unaffected, with the reason, satisfies the check |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | [feature-specific, for example "merge order correct", "error messages name the offending value"] |
| Naming | [feature-specific, for example "JSON keys kebab-case", "YANG leaf matches env var leaf"] |
| Data flow | [feature-specific, for example "resolution in X only, reactor unaware of Y"] |
| Rule: [relevant rule] | [what to check] |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- [What was deliberately not done and why]

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
