# Spec: l2tp-shaper-upload-rate-is-not-enforced

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang` declares
`leaf upload-rate` with the description "Default upload rate. When omitted,
defaults to default-rate." The sibling leaf is `default-rate`, described as
"Default download rate applied to a new session." A reader takes the pair to
mean Ze shapes a subscriber session in both directions, at the download rate one
way and the upload rate the other.

**What Ze does instead.** No interface enforces the upload rate.
`(*shaperPlugin).applyTC`
(`internal/component/l2tp/plugins/shaper/shaper.go`) takes one `rateBps`
argument and builds one `traffic.TrafficClass` from it, setting `Rate` and, for
HTB, `Ceil`. It has no second rate parameter, so it can install one direction
only. Both callers pass the download rate. `(*shaperPlugin).onSessionUp` in the
same file stores `state.uploadRate` on the session and then calls
`s.applyTC(payload.Interface, cfg.QdiscType, state.downloadRate)`.
`(*shaperPlugin).handleSubscriberSessionUp`, the `subscriber.ShaperHandler` that
PPPoE calls on SessionUp, takes its upload argument as `_` and calls `applyTC`
with the download rate.

The value is stored and reported, which is what makes this the operator's
problem. `(*shaperPlugin).showSessions` in the same file renders it as the
`upload-rate-bps` JSON key, so `show l2tp shaper` reads back a rate that no
queueing discipline is enforcing. The same gap swallows half of a RADIUS answer:
`onSessionUp` parses a Filter-Id of the form `rate:20mbit/5mbit` through
`parseFilterRate` (`internal/component/l2tp/plugins/shaper/filter_rate.go`),
logs both halves, stores both on the session, and applies the
download half alone. A subscriber the RADIUS server rate-limited upward is not
rate-limited upward.

**What closing it means.** The implementer chooses between building the behavior
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Building it is
obviously right, because the refusal does not reach the defect: the RADIUS
Filter-Id path carries an upload rate that no leaf declares, so refusing the
leaf would leave `show l2tp shaper` reporting an unenforced rate from RADIUS
with nothing to refuse. What the design owes is the mechanism. A queueing
discipline on the `pppN` interface shapes what leaves that interface, which is
the download direction from the subscriber's point of view, so the upload
direction needs either an ingress qdisc, a policer, or shaping applied on the
other side of the tunnel. That mechanism choice is the whole difficulty, and it
belongs to the design.

`plan/to-review/spec-l2tp-12-xdp-policing.md` is at Status `ready` and builds
per-session ingress policing with XDP. It declares its own YANG and reads no
leaf under `l2tp { shaper { ... } }`, so it does not close this leaf. Whether
the two should share one mechanism is a question this spec's design must put,
because two mechanisms answering the same question is one too many
(`ai/rules/simplicity.md`).

## Research Findings (2026-09-06)

The Task section names one open question: which mechanism enforces the upload
direction, and which plugin owns it. This section answers it from the producers
and states the decision the owner must take. No code changed, and the spec stays
at `skeleton`, because every edit available takes that decision.

### Verified facts

| # | Fact | Producer read |
|---|------|---------------|
| F-1 | The shaper installs one root qdisc with one class, built from a single rate. Egress on a pppN interface carries traffic toward the subscriber, which is the download direction | `(*shaperPlugin).applyTC`, `internal/component/l2tp/plugins/shaper/shaper.go` |
| F-2 | The upload direction is ingress on pppN, and the traffic data model cannot express it. `InterfaceQoS` holds one root `Qdisc` and carries no policer | `InterfaceQoS`, `Qdisc`, `TrafficClass`, `internal/component/traffic/model.go` |
| F-3 | The netlink backend refuses the two ingress-hook qdisc kinds: "attaches at the ingress hook, not at the root, and is not configurable". So no upload enforcement is reachable through `traffic.Backend` today | `translateQdisc`, `internal/plugins/traffic/netlink/translate_linux.go` |
| F-4 | The ingress hook is already shared. The mirror path installs clsact at handle `ffff:`, and the sampling path reuses it. A pppN policer is a third owner of that hook | `internal/plugins/iface/netlink/mirror_linux.go`, `internal/plugins/flowexport/sampling/tc_linux.go` |
| F-5 | Both access types terminate on a pppN interface. The L2TP path shapes `SessionUpPayload.Interface` and the PPPoE path shapes `sess.PppInterface`. One ingress mechanism on pppN covers both | `(*shaperPlugin).onSessionUp` and `(*shaperPlugin).handleSubscriberSessionUp`, `shaper.go`; `internal/component/l2tp/pppoe/subsystem.go` |
| F-6 | `spec-l2tp-12-xdp-policing.md` polices inbound L2TP data packets on the uplink NIC, keyed by tunnel id and session id. A PPPoE session carries no L2TP header, so that design cannot enforce upload for a PPPoE subscriber | `plan/to-review/spec-l2tp-12-xdp-policing.md`, Data Flow and AC-4 |
| F-7 | That spec declares its own rate leaves under `l2tp { policing { ... } }` and takes the per-session rate from `SessionRateChangePayload.UploadRate`. It reads neither `l2tp/shaper/upload-rate` nor the RADIUS Filter-Id, and its session-up map entry uses its own YANG default | same spec, AC-6 and AC-6a |
| F-8 | `subscriber.Session.UploadRate` has no producer. No assignment exists anywhere, so the PPPoE call site always hands the shaper a zero upload rate, and `show subscriber` never prints the key | `subscriber.Session`, `internal/component/l2tp/subscriber/session.go`; `internal/component/l2tp/subscriber/cmd/subscriber.go` |
| F-9 | The `ze:help` beside the leaf already states that no interface enforces the rate. The leaf `description` and `docs/guide/l2tp.md` still read as a promise of enforcement | `internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang`; `docs/guide/l2tp.md`, the Traffic shaping section |

F-8 corrects the Task section on one point. The PPPoE handler does take its
upload argument as `_`, and the argument it discards is always zero.

### The decision the owner must take

Two mechanisms answer the same question, and only one of them should exist
(`ai/rules/simplicity.md`).

| Route | Owner | What it gives | What it costs |
|-------|-------|---------------|---------------|
| A: policing on the pppN ingress hook | `l2tp-shaper` | One mechanism for L2TP and PPPoE, on the interface that already carries the download shaping, fed by the rates this plugin already parses | An ingress policer in the traffic model, netlink translation, a VPP answer, and a third owner on the shared clsact hook. It makes the line-rate bucket of spec-l2tp-12 a duplicate |
| B: XDP policing on the uplink NIC | `l2tp-policing`, spec-l2tp-12 | A design that is written and at `ready`, and drops before kernel decapsulation | A PPPoE subscriber stays unpoliced upward. `l2tp/shaper/upload-rate` and the upload half of the RADIUS Filter-Id keep no reader, so both have to move to the policing plugin or be read by it |

Under either route `show l2tp shaper` must stop reporting a rate the shaper does
not enforce. What replaces the `upload-rate-bps` key depends on the route.

### Why no code changed

Each candidate edit has a different correct form under each route: remove the
leaf, reword the leaf `description`, change or drop the show key, or install a
policer. Picking one picks the route. The route is the owner's decision
(`ai/rules/rule-precedence.md`, rung 3).

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
