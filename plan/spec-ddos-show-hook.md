# spec-ddos-show-hook

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

> **SKELETON.** Captured intent, not a designed spec (`ai/rules/planning.md`:
> "a skeleton is captured intent, not a designed spec"). Opened on Thomas's approval,
> 2026-07-16, on **operator-usefulness grounds**. Research has NOT been done; the Current
> Behavior section below records only what was verified at the producers while opening the
> file. Status moves to `design` when someone picks it up.
>
> **READ THIS BEFORE TOUCHING THE FILE.** This idea was **REJECTED** inside
> `plan/spec-fixit-ddos-test-infra.md` (its D-1 option (c)) as "a product change to make a
> test pass". **That rejection STANDS and is not overturned by this spec's existence.**
> This spec exists on its own merits and must never become a backdoor for that test rework.
> See "Origin and the standing rejection" below before proposing any link between the two.

## Task

`show ddos local` reports **that** a mitigation is installed but never **which netfilter
hook it is on**. Add a `hook` field to the command's response so the mitigation the box
actually installed is observable through the normal command surface.

The points to complete:

| # | Point | Known constraint |
|---|-------|------------------|
| 1 | Add a `hook` field to the `show ddos local` response | The value should be the operator-facing chain name (`ingress` / `forward`), which `hookChainName` (`responder.go`) already produces for the log line. Do not invent a second spelling |
| 2 | Persist the chosen hook in responder state | **The hook is NOT stored today.** The `responder` struct (`responder.go`) holds `mu`, `cfg`, `bus`, `active`, `target` and no hook. It is computed transiently at `:111` and discarded. This is the real work; the show-handler edit is the easy half |
| 3 | Decide the field's shape when no mitigation is active | Today `active: false` omits `target` entirely (`show.go`). `hook` presumably follows the same rule. A design choice, not a given |
| 4 | Update the command's YANG description | `internal/plugins/ddos/local/cmd/yang/ze-ddos-local-cmd.yang` currently promises only "whether an nft drop rule is currently installed and the target vector (prefix / proto / port) it covers" |
| 5 | Cover it with a test | `show_test.go` exists (`internal/plugins/ddos/local/show_test.go`). `TestLocalHookByDirection` (`responder_test.go`) already covers hook SELECTION exhaustively and must stay untouched: this spec is about REPORTING the selection, not making it |

**Why an operator wants this (the justification of record):**

`ddos-local`'s whole job is to change the dataplane on the operator's behalf, automatically,
in response to an attack the operator did not schedule. Which hook the drop landed on is not
an implementation detail to them: it is the difference between "the box is protecting
itself" (`ingress` / INPUT) and "the box is protecting a downstream host" (`forward` /
FORWARD), and `hookForDirection` (`responder.go`) picks between them from the
victim's direction plus a config flag the operator set. So the box makes a consequential
choice and then does not report it.

Today the only ways to answer "which hook is my drop on?" are:
1. Read the nft ruleset directly, which needs **root** on the box.
2. Grep the responder's log for `hook=ingress` / `hook=forward` (`responder.go`),
   which requires having captured the log at the moment of installation and which reports
   the responder's INTENT rather than kernel state.

Neither is a command. A `hook` field makes the automated action observable on the same
surface the operator already uses to ask whether mitigation is active at all.

-> Constraint: this is an **observability** change, not a control change. It reports a
choice that is already made and already correct. It must not become a knob for SELECTING
the hook: that is `hookForDirection`'s job and `forward-mitigation`'s config surface.

## Origin and the standing rejection (BLOCKING context)

| Fact | Evidence |
|------|----------|
| The idea was raised and REJECTED as D-1 option (c) of the test-infra spec | `plan/spec-fixit-ddos-test-infra.md` "The rejected alternative (D-1 option c), recorded so it is not silently revived": "adding a `hook` field to `show ddos local` is **REJECTED as a means of satisfying this spec** -- it is a **product change made to make a test pass**" |
| The rejection was scoped, deliberately, to the motivation and NOT to the idea | Same section: "the rejection is **scoped to this spec's motivation, not to the idea itself**", and it records that "which hook is my drop on?" is a reasonable thing for an operator to ask. That scoping is what makes this spec legitimate rather than a revival |
| The test rework it was rejected from is SETTLED and is NOT reopened by this spec | `plan/spec-fixit-ddos-test-infra.md` D-1 is APPROVED: keep the `driver.py` + `daemon.ready` pattern, migrate its hand-rolled sleep loops to `ze_api.wait_until`. Option (a) (port to the in-daemon probe) and option (b) (keep driver.py unchanged) are both REJECTED |

-> Constraint (BLOCKING): **this spec must not be cited as a reason to change
`plan/spec-fixit-ddos-test-infra.md`'s approach.** That spec's tests read nft state from a
root `driver.py`, deliberately, because the runner keeps `driver.py` privileged for exactly
that purpose (`internal/test/runner/runner_exec.go`) while dropping the daemon's
privileges. Its AC-2 is struck and replaced by AC-2a; its D-1 is answered. If this spec
ships, that test's nft readback **stays** as the primary assertion: nft is kernel state, a
`hook` field is the responder's self-report, and `ai/rules/completion.md`
plus that spec's own AC-3 forbid weakening a kernel-state assertion to a self-report.

-> Constraint: **the test-dissolving consequence is a CONSEQUENCE, not the justification.**
Recorded honestly because it is true and a future reader will notice it anyway: the test-infra
spec observes that if `show ddos local` reported the hook, "the whole nft/root problem would
dissolve and AC-4/AC-5 would be pure dispatch assertions". That is a real and attractive side
effect. It is **not** why this spec exists, it may **not** be used to justify it, and it may
not be used to reopen the settled test design. The order matters: the field earns its place
on the operator surface first; any test convenience is a windfall the tests are not entitled
to claim. If the operator case were ever found not to hold, this spec dies even though the
test convenience would remain.

-> Constraint: do NOT edit `plan/spec-fixit-ddos-test-infra.md` from this spec. Cross-reference
it only. (At the time this file was opened, another agent may have held it.)

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` - named as the design doc by both
      `show.go` and `responder.go`
  → Constraint: read before changing the responder's state or the show surface; the
    `// Design:` anchors on both files point here and must keep resolving.
- [ ] `ai/rules/plugins.md` - the command is plugin-owned
  → Constraint: the `show ddos local` node is owned by the ddos-local plugin and
    container-merges onto the shared `show ddos` namespace owned by ddos-observe
    (`cmd/yang/ze-ddos-local-cmd.yang`). Removing the plugin must remove this node,
    its `hook` field and its handler. No new spelling in a generic package.
- [ ] `ai/rules/completion.md` - governs the cross-spec constraint above
  → Constraint: a `hook` self-report must not be allowed to replace a kernel-state
    assertion in `plan/spec-fixit-ddos-test-infra.md`'s tests.
- [ ] `ai/rules/cli.md` - the command produces output
  → Constraint: verify at DESIGN whether adding a field affects the pipe surface.

### RFC Summaries

Not applicable. This is a local observability surface, not protocol work.

**Key insights:**
- The hook is already CHOSEN and already LOGGED; it is simply not RETAINED or REPORTED.
- The work is mostly in the responder's state, not in the show handler.

## Current Behavior (MANDATORY)

**Source files read** (verified at the producers 2026-07-16 while opening this skeleton;
NOT a substitute for the DESIGN-phase research):

- [ ] `internal/plugins/ddos/local/show.go` - lines 23-37: `handleShowDdosLocal` returns
      `plugin.Map{"enabled": false, "active": false}` when `activeResponder.Load()` is nil
      (`:25-29`); otherwise `{"enabled": true, "active": active}` plus `"target"` only when
      `active` is true. **There is no `hook` key on any path.**
  -> Constraint: this is the producer of the claim "no dispatch surface reports the hook".
     Verified by reading it, not inferred from its callers.
- [ ] `internal/plugins/ddos/local/responder.go` - lines 39-45: the `responder` struct holds
      `mu`, `cfg`, `bus`, `active`, `target`. **No hook field.** Lines 198-202: `status()`
      returns `(r.active, r.target)` only, so the show handler could not report a hook even
      if it wanted to.
  -> Decision: THE key finding for scoping. Adding the field is not a one-line show-handler
     edit: the hook must first be persisted at apply time alongside `r.active` / `r.target`.
- [ ] `internal/plugins/ddos/local/responder.go` - `hook, ok := r.hookForDirection(direction)`
      inside `applyMitigation`; the value is used at `:125` (`hookChainName(hook)` as the chain
      name), `:128` (`Hook: hook`) and `:153` (the log), then **discarded when the function
      returns**.
  -> Constraint: the hook is computed on the apply path only. A design must decide what
     `hook` reports when a mitigation was installed and later cleared.
- [ ] `internal/plugins/ddos/local/responder.go` - `hookForDirection` returns
      `firewall.HookForward` for a remote victim with forward-mitigation enabled, else
      `firewall.HookInput`.
  -> Constraint: hook selection is NOT this spec's subject and must not change. It is
     exhaustively covered by `TestLocalHookByDirection` (`responder_test.go`).
- [ ] `internal/plugins/ddos/local/responder.go` - `hookChainName(hook)` returns
      `"forward"` for `HookForward`, else `"ingress"`.
  -> Decision: the operator-facing spelling already exists. Reuse it; do not invent a second.
- [ ] `internal/plugins/ddos/local/cmd/yang/ze-ddos-local-cmd.yang` - the `local`
      container's description promises status + target vector only, and carries
      `ze:command "ze-show:ddos-local"`.
  -> Constraint: the description is part of the user-visible surface and goes stale the
     moment a field is added.

**Behavior to preserve:**

- Hook SELECTION (`hookForDirection`, `responder.go`) is unchanged. This spec reports
  the choice; it does not make it.
- `TestLocalHookByDirection` (`responder_test.go`) stays the exhaustive hook-selection
  unit test and must remain green.
- The existing response keys `enabled`, `active` and `target` keep their current shape and
  their current omission rules (`show.go`). Adding a field must not rename or reshape
  them.
- The `// Design:` anchors on `show.go` and `responder.go` keep resolving.
- The nft ruleset stays the source of truth for kernel state. A `hook` field is a self-report.

**Behavior to change:**

- `show ddos local` gains a `hook` field. Nothing else. (Full list at DESIGN.)

## Data Flow (fill during design)

### Entry Point

- `show ddos local` over the dispatch surface, reaching `handleShowDdosLocal` via the
  registered RPC `ze-show:ddos-local` (`show.go`, `cmd/yang/ze-ddos-local-cmd.yang`).

### Transformation Path

1. An attack event (`AttackDetected` / `AttackCharacterized`) reaches the responder
   (`responder.go`, `:73-86`).
2. `applyMitigation` picks the hook via `hookForDirection` (`responder.go`) and installs
   the nft table.
3. The hook is logged (`responder.go`) and then **discarded** (no struct field, `:39-45`).
4. `show ddos local` calls `status()` (`responder.go`), which returns
   `active` + `target` only.
5. `handleShowDdosLocal` (`show.go`) marshals those into the response. The hook is
   absent because step 3 dropped it.

-> Constraint: step 3 is where the information is lost. That is the seam this spec must fix.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Operator to plugin | `show ddos local` -> registered RPC `ze-show:ddos-local` -> `handleShowDdosLocal` | [ ] |
| Show handler to responder state | `activeResponder.Load()` + `status()` (process-global, in-process plugin) | [ ] |
| Responder to kernel | `firewall.RegisterTables` / `ApplyAll` -> nft (`responder.go`) | [ ] |

### Integration Points

- `responder.status()` (`responder.go`) - the existing accessor to extend.
- `hookChainName` (`responder.go`) - the existing operator-facing spelling to reuse.

### Architectural Verification

- [ ] No bypassed layers (the show handler keeps reading responder state, not nft)
- [ ] No unintended coupling (no test-driven seam; see the standing-rejection constraints)
- [ ] No duplicated functionality (reuse `hookChainName`, do not add a second name mapping)
- [ ] Zero-copy preserved where applicable (value types across the boundary)
- [ ] Registration over hardcoding -- the `hook` field ships inside the plugin-owned command
      (`cmd/yang/ze-ddos-local-cmd.yang`) and its registered handler (`show.go`); no
      per-feature field, switch case or factory is added to a core/shared package. Removing
      the ddos-local plugin removes the node, the field and the handler
      (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No dispatch-command surface reports the hook today | Read the producer `handleShowDdosLocal` (`show.go`): no `hook` key on any return path | The spec has no reason to exist | Read the producer (done 2026-07-16) | **confirmed** |
| A-2 | The hook is not retained in responder state, so reporting it requires persisting it first | Read the producer: `responder` struct (`responder.go`) has no hook field; `status()` returns `active`+`target`; the value is local to `applyMitigation` | The change is a trivial show-handler edit and the scope shrinks | Read the producers (done 2026-07-16) | **confirmed** |
| A-3 | Operators actually want this reported on the command surface | Thomas approved opening this spec on operator-usefulness grounds, 2026-07-16. The test-infra spec independently records "which hook is my drop on?" as a reasonable operator question | The spec dies: the test-convenience side effect may NOT justify it on its own (see the standing-rejection constraints) | User judgement; already given for opening the file. Re-confirm the SHAPE at DESIGN | **confirmed (opening); shape unvalidated** |
| A-4 | Reporting the chain name (`ingress` / `forward`) is what an operator wants, rather than the raw `firewall.ChainHook` | `hookChainName` (`responder.go`) is already the operator-facing spelling and is what the log carries | The field reports a value operators must translate | DESIGN review | unvalidated |
| A-5 | Adding one field to the response breaks no existing consumer | Not investigated. `show_test.go` exists; other consumers (web, tests, scripts) are not surveyed | A consumer asserts an exact map shape and goes red | Grep every consumer of `ze-show:ddos-local` and of the show output at DESIGN | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | **The backdoor risk. The main risk of this spec's existence.** It is used to reopen or weaken `plan/spec-fixit-ddos-test-infra.md`'s settled design (D-1: keep driver.py, migrate to `ze_api.wait_until`), turning a rejected product-change-to-pass-a-test into a shipped one by the back door | A diff or spec edit cites THIS spec as a reason to drop that spec's nft readback, or to revive its struck AC-2 / its D-1 option (a) or (c) | The rejection STANDS (see "Origin and the standing rejection"). nft readback is kernel state and stays primary; a `hook` field is a self-report and can only ever CORROBORATE, exactly as the responder log does in that spec's D-4. Neither spec depends on the other: `Depends` is `-` deliberately, and must stay `-` |
| R-2 | The field reports the responder's intent while the kernel disagrees, and an operator trusts the field | A drop is reported on a hook that nft does not show | Be explicit in the YANG description that this is the responder's installed-state self-report. Never let it be read as a kernel readback. This is the same intent-vs-kernel distinction the test-infra spec's D-4 draws for the log |
| R-3 | Hook selection gets "improved" while reporting it | `hookForDirection` (`responder.go`) appears in the diff | Out of scope. This spec reports the choice; it does not make it. `TestLocalHookByDirection` must stay green and untouched |
| R-4 | The stale state question is missed: a mitigation is cleared but the hook lingers | `show ddos local` reports a hook while `active: false` | Task point 3. Decide the omission rule at DESIGN and pin it with a test that clears a mitigation and re-reads the surface |

## Wiring Test (MANDATORY -- fill during design)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `show ddos local` while an INPUT-hook drop is installed | -> | `handleShowDdosLocal` (`show.go`) reading the persisted hook from `status()` (`responder.go`) | (fill during design) |
| `show ddos local` while a FORWARD-hook drop is installed | -> | same path, `forward` | (fill during design) |
| `show ddos local` with no mitigation active | -> | `show.go` nil-responder path | (fill during design) |

## Acceptance Criteria

<!-- Provisional: refine at DESIGN. Each row must be a testable assertion. -->

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A local-victim attack installs a drop, then `show ddos local` | The response reports the hook as `ingress`, alongside the existing `enabled` / `active` / `target` fields |
| AC-2 | A remote-victim attack with `forward-mitigation` enabled installs a drop, then `show ddos local` | The response reports the hook as `forward` |
| AC-3 | `show ddos local` with no responder / no active mitigation | Per the omission rule chosen in Task point 3. No stale hook is ever reported for an inactive mitigation (R-4) |
| AC-4 | `TestLocalHookByDirection` (`responder_test.go`) | Still passes, untouched. Hook SELECTION is unchanged by this spec |
| AC-5 | Existing consumers of `show ddos local` | Unaffected: `enabled`, `active` and `target` keep their shape and omission rules (A-5) |
| AC-6 | The command's YANG description | Names the hook field. The described surface matches the returned surface (`cmd/yang/ze-ddos-local-cmd.yang`) |
| AC-7 | `plan/spec-fixit-ddos-test-infra.md`'s tests | Unchanged by this spec. Their nft readback remains the primary kernel-state assertion (R-1) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks "is the box mitigating, and is it protecting itself or a downstream host?" via `show ddos local`, without root and without reading nft | dispatch -> `ze-show:ddos-local` -> `handleShowDdosLocal` -> `status()` -> persisted hook | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | `internal/plugins/ddos/local/show_test.go` | AC-1/AC-2/AC-3: the response carries the hook for each direction, and omits it when inactive | proposed |
| (fill during design) | `internal/plugins/ddos/local/responder_test.go` | A-2: the responder persists the chosen hook at apply time | proposed |
| `TestLocalHookByDirection` | `internal/plugins/ddos/local/responder_test.go` (exists) | AC-4 regression guard: hook selection stays exhaustively covered and unchanged | exists |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `hook` | enumerated (`ingress` / `forward`), not numeric | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `test/plugin/*.ci` | An operator reads the installed mitigation's hook from the command surface | proposed |

-> Constraint: any functional test here is for THIS spec's operator surface. It must not be
written as, or merged into, `plan/spec-fixit-ddos-test-infra.md`'s AC-4 / AC-5 transit proof
(R-1).

### Interop Tests

Not applicable: no wire protocol behavior changes.

### Future

None deferred yet. Scope is set at DESIGN.

## Files to Modify

- `internal/plugins/ddos/local/responder.go` - persist the chosen hook in the `responder`
  struct at apply time and return it from `status()`.
- `internal/plugins/ddos/local/show.go` - add the field to the response.
- `internal/plugins/ddos/local/cmd/yang/ze-ddos-local-cmd.yang` - update the `local`
  container description; add a revision entry.
- `internal/plugins/ddos/local/show_test.go` - cover the new field.
- `internal/plugins/ddos/local/responder_test.go` - cover the persistence. Do NOT modify
  `TestLocalHookByDirection`.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (command description) | [ ] Yes | `internal/plugins/ddos/local/cmd/yang/ze-ddos-local-cmd.yang` |
| CLI commands/flags | [ ] No new command; an existing one gains a field | - |
| Functional test | [ ] Decide at DESIGN | `test/plugin/*.ci` |
| Pipe completeness | [ ] Check at DESIGN (the command produces output) | `ai/rules/cli.md` |
| Doctor check | [ ] No new runtime dependency | - |
| Prometheus counters | [ ] Decide at DESIGN | - |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | [ ] Yes (a field is added) | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] Likely (the response gains a field) | `docs/architecture/api/commands.md` |
| 15 | Registered command / runtime inventory changed? | [ ] Check at DESIGN | `docs/plugin-overview.md`, `docs/guide/status.md` |
| 16 | Changed source referenced by doc source anchors? | [ ] Grep `docs/` for the changed files | per grep |

## Files to Create

- (none identified yet; a functional `.ci` may be added at DESIGN)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

(fill during design)

1. **Phase: Wiring** -- persist the hook and surface it; failing test first.
2. **Phase: Reporting** -- the show handler and its omission rule.
3. **Phase: Docs + YANG description.**

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The reported hook is the one actually installed, including after a re-apply narrows the rule in place (`responder.go`) |
| Naming | The field reuses `hookChainName`'s spelling (`responder.go`); no second mapping |
| Data flow | The show handler reads responder state, never nft |
| Scope | `hookForDirection` (`responder.go`) is NOT modified; `TestLocalHookByDirection` is untouched |
| Registration over hardcoding | The field ships inside the plugin-owned command + registered handler; nothing added to a core/shared package (`ai/rules/plugins.md`) |
| Cross-spec | No change to `plan/spec-fixit-ddos-test-infra.md`'s design or tests (R-1) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `show ddos local` reports the hook | (fill during design) |
| Hook selection unchanged | `go test -run TestLocalHookByDirection ./internal/plugins/ddos/local/` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Information disclosure | The field reveals what mitigation the box installed. `show ddos local` is an existing read-only surface already reporting `target`; confirm at DESIGN that the hook adds no privilege-relevant detail beyond it |
| Input validation | None: the field is emitted, not accepted |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A consumer breaks on the new field (A-5) | DESIGN: reconsider the response shape. Do not delete the consumer's assertion |
| The change starts touching `hookForDirection` | STOP. Out of scope (R-3) |
| It becomes tempting to weaken the test-infra spec's nft readback | STOP. R-1. The rejection stands |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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

- The information already exists and is already correct; it is computed, used, logged, and
  then dropped on the floor (`responder.go` -> `:153`, never stored at `:39-45`). The
  gap is retention, not derivation. That is why the change is small and why it was easy to
  mistake for a test convenience.
- A surface that reports "something happened" but not "what happened" is a recurring shape
  worth watching for: `show ddos local` reports `active: true` for an automated dataplane
  change without reporting which change. The nearest analogue in this codebase is the
  responder's log, which DOES carry the hook -- the CLI simply never caught up with it.

## Known Limitations

- The field is the responder's self-report of what it installed, not a kernel readback. It
  cannot detect a rule removed behind ze's back. nft remains the source of truth for kernel
  state (R-2).
- This spec does not touch hook selection, the detection path, or the mitigation logic.

## Open Questions (research at DESIGN)

| # | Question |
|---|----------|
| 1 | Field shape when inactive: omit like `target` (`show.go`), or always present? (Task point 3, AC-3, R-4) |
| 2 | Chain name (`ingress` / `forward`) or the raw `firewall.ChainHook`? (A-4) |
| 3 | Which consumers read `ze-show:ddos-local` today, and does adding a key break any? (A-5) |
| 4 | Does the ddos-observe `show ddos` surface have a parallel gap worth fixing in the same work, or is that scope creep? |
| 5 | Should the hook be reported per installed table rather than as one value, if a future responder installs more than one? |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete (every row has a concrete test name, none deferred)
- [ ] `/ze-review` gate clean (Review Gate section filled: 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)
- [ ] R-1 honored: `plan/spec-fixit-ddos-test-infra.md` is unchanged by this work

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A: no wire protocol change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes: all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
