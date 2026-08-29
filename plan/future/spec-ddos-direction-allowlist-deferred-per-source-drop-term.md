# Spec: ddos-direction-allowlist-deferred-per-source-drop-term -- carve the drop by source

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/ddos/local/match.go` - `buildDropTerm`

## Task

**Source-match policy rules suppress detection and mitigation but do not carve the drop
by source.** When a policy rule exempts a source, ze today either mitigates the whole
victim/incident or suppresses it entirely. There is no middle: "drop this attack but
keep letting this source through".

Verified 2026-07-16:

| Fact | Evidence |
|------|----------|
| `buildDropTerm` matches destination/proto/ports/flags only | `internal/plugins/ddos/local/match.go`: appends `MatchDestinationAddress`, `MatchProtocol`, `MatchDestinationPort`, `MatchSourcePort`, `MatchTCPFlags`. No source-address match. Its whole input is `(name string, v ddosevent.VectorTuple)` |
| The firewall primitive already exists | `internal/component/firewall/model.go`: `type MatchSourceAddress struct{ Prefix netip.Prefix }`, with its `matchMarker()` at `:379` |
| ddos never uses it | `grep -rn MatchSourceAddress internal/plugins/ddos/` returns nothing |
| **No firewall-side work is needed** | The backends already lower it: nft at `internal/plugins/firewall/nft/lower_linux.go`, VPP at `internal/plugins/firewall/vpp/translate.go` and `vpp/verify.go`. Config already parses it (`internal/component/firewall/config.go`) |

So the building block is present, already lowered by every backend, and simply unwired for
ddos. The cost of this spec is not in the firewall layer.

**The keystone gap (verified, and the reason this is not a small change):**

`ddosevent.VectorTuple` (`internal/core/ddosevent/event.go`) carries `DstPrefix`,
`Proto`, `DstPort`, `SrcPort`, `TCPFlags`. It has **no source address field at all**. The only
source data on the wire is `TopSources []netip.Addr` (`event.go`, and `:146` on
`AttackCharacterized`): bare addresses rather than prefixes, and "top" sources rather than
"exempt" ones. So no existing field names the sources a policy rule exempted. Either the
detector gains one, or the responder gains a policy view it is not architecturally allowed to
have. That choice, not the term construction, is the real work.

-> Constraint: this is a **narrowing** change. It must not become a second enforcement point
for the traffic policy. The parent spec's central decision was recorded as: "Detector is the
single enforcement point; the event carries the decision." A responder that reads policy rules
directly overturns that decision and must not be introduced here.

## Analysis 2026-08-05: the mechanism is easy, the SEMANTIC is the decision

Everything the spec says about the plumbing holds, and the firewall side is even
easier than it records. `Accept` "terminates evaluation" (`firewall.Accept`,
`internal/component/firewall/model.go`) and a chain's `Terms` is an ORDERED
slice, so "drop this attack but let this source through" needs no negated match:
it is an Accept term carrying `MatchSourceAddress`, placed before the existing
drop term in the same chain. `applyMitigation`
(`internal/plugins/ddos/local/responder.go`) builds that chain with a single
`Terms: []firewall.Term{term}` today, so this is a one-element-to-two change at
the call site rather than a rewrite of `buildDropTerm`.

**But the current behavior is not an oversight, and the spec does not say so.**
`PolicyRule.matches` sends a source rule through `allSourcesIn`
(`internal/plugins/ddos/detect/policy.go`), whose doc states the intent: a source
rule "requires that EVERY known source falls within the prefix (one hostile
out-of-prefix source keeps the attack live)". All-or-nothing is deliberate and
fail-safe. Today a source-allow rule either exempts the whole incident, because
every source is inside it, or does not fire at all. There is no partial state to
get wrong.

**What this feature would introduce, and it is not recorded anywhere.** An Accept
term keyed on source address is honoured by the kernel on the packet's CLAIMED
source. A DDoS source address is exactly the field an attacker controls and the
one most often forged, and `characterize.go` already reasons about the
"distributed/spoofed annotation" when entropy spreads across sources. So a
per-source carve-out hands any attacker who can guess an allowlisted prefix a
complete bypass of the mitigation, by spoofing into it. Nothing in
`internal/plugins/ddos/` performs or requires a uRPF or anti-spoof check: a grep
for spoof or urpf across the tree returns one comment and no enforcement.

That is the real cost of the feature, and it is a security trade rather than an
implementation detail. It does not appear in this spec, in the parent, or in the source
spec's learned summary.

**Two further facts that bear on the design.**

`TopSources` is the only source data on the wire (`ddosevent.AttackDetected` and
`AttackCharacterized`, `internal/core/ddosevent/event.go`) and it is TOP sources,
not exempt ones, and bare addresses rather than prefixes. So it cannot be reused
as-is: an Accept term must carry the RULE's prefix, which only the detector knows.

The architectural constraint the spec names is real and holds. The parent recorded
"Detector is the single enforcement point; the event carries the
decision", so the exempt prefixes must travel ON the event and the responder must
never read policy rules. That means a new field, which is a detector-to-responder
contract change.

**Recommendation, for the owner.** Do not implement this as specified without
answering the spoofing question first. Either the carve-out is gated on an
anti-spoof precondition that ze does not currently have, or it is accepted as an
operator-visible risk and documented as such at the config surface. Choosing
silently would put a forgeable bypass into the mitigation path, which is the one
place it is worth the most to an attacker.

**Provenance.** Deferred from `spec-ddos-direction-allowlist` (Known Limitations) on
2026-07-12: deliberately out of scope for v1, which exempts at the victim/incident level.
The source spec was closed in `0814dc93f`.

**A correction this spec must carry.** The deferral row asserted this limitation was
"recorded in the source spec's learned summary, Known Limitations". That is
FALSE: that summary had no Known Limitations section at all (its headings were Context,
Decisions, Consequences, Gotchas, Files), and no mention of `buildDropTerm` or per-source
narrowing. The knowledge existed ONLY in the deferral row, and the summary has since been
retired with the learned corpus. This spec is now its only home.

**Open design question.** Whether per-source narrowing belongs in the drop term at all,
or whether an exempted source should instead be an accept term ordered ahead of the drop.
The firewall model's term ordering decides this; do not assume the match-list approach.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ddos.md` - operator model for policy rules and exemptions
  → Constraint: an exemption's operator-visible meaning must not change silently
- [ ] `ai/rules/evidence.md` - `SuppressMitigation` was named so the zero value means mitigate
  → Constraint: a narrowing bug must fail toward mitigating, never toward letting an attack through
- [ ] The direction/allowlist design record this extends (retired with the learned corpus)
  → Decision: policy is evaluated once at detection and the outcome is encoded on the event (`SuppressMitigation`), so the responder does not re-evaluate policy

### RFC Summaries (MUST for protocol work)
- Not applicable: this is on-host firewall term construction, not a wire protocol.

**Key insights:**
- 1110's core decision (evaluate policy once, encode the outcome on the event) means a per-source carve-out needs the SOURCE SET carried on the event, not just a boolean. That is the main design cost.
- `TopSources` is the only source data crossing the detector/responder boundary today, and it is the wrong shape twice over: bare `netip.Addr` rather than `netip.Prefix`, and "top" (an observability ranking) rather than "exempt" (a policy outcome). Reusing it would smuggle a policy decision into a diagnostic field.
- The term shape is an open choice with a rule-ordering consequence: an nft accept term ordered ahead of the drop, or a negated source match inside the drop. The firewall model's ordering semantics decide it; do not assume the match-list approach.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/local/match.go` - `buildDropTerm` builds a firewall term from a `ddosevent.VectorTuple`, matching dst prefix, protocol, dst port, src port; never src address
- [ ] `internal/component/firewall/model.go` - `:281` defines `MatchSourceAddress`, unused by ddos
- [ ] `internal/plugins/ddos/local/responder.go` - applies the mitigation via `firewall.ApplyAll`; honors `SuppressMitigation` all-or-nothing

**Behavior to preserve:**
- The v1 contract: an exempted source suppresses the whole incident. Operators depend on this today; changing it silently would alter live mitigation behavior.
- `SuppressMitigation`'s fail-safe polarity: the bool zero value means mitigate.
- Existing drop-term construction for every non-exempt vector must produce identical firewall terms.
- The detector stays the single policy enforcement point. Responders obey the event and never re-read policy (plugins receive only their own config subtree).
- `buildDropTerm`'s exact-match TCP-flag mask contract, documented at `match.go`: `Mask == Flags` means "examine exactly these bits, require them set" (AC-9 of the parent).
- The `ze_ddos-local` table name and its ownership prefix (`local/responder.go`): renaming it strands drop rules in the kernel.

**Behavior to change:**
- Only if design approves: an exempted source is carved out of the drop rather than suppressing the whole incident. This is a behavior change requiring explicit user approval.

## Data Flow (MANDATORY)

### Entry Point
- A traffic anomaly triggers `ddosevent.AttackDetected` / `AttackCharacterized`, carrying a `VectorTuple` and (since 1110) `Direction` + `SuppressMitigation`.

### Transformation Path
1. Detector characterizes the attack and produces a `VectorTuple` (dst prefix, proto, ports).
2. Policy is evaluated ONCE and its outcome is encoded on the event as `SuppressMitigation` (1110's decision).
3. `local/responder.go` receives the event; if `SuppressMitigation` it does nothing at all.
4. Otherwise `buildDropTerm` (`match.go`) turns the tuple into a firewall term.
5. The term is installed via `firewall.ApplyAll` on the direction-appropriate hook (INPUT for local victims, FORWARD for transit).

Stage 2 is where the source set is currently collapsed to a boolean, and stage 4 is where a source match would have to be expressed. Both must change together.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Detector ↔ responder | `ddosevent` struct fields (`Direction`, `SuppressMitigation`, `VectorTuple`) | [ ] |
| Responder ↔ firewall | `firewall.Term` built from matches; installed via `ApplyAll` | [ ] |
| Policy config ↔ event | policy evaluated once at detection, outcome encoded on the event | [ ] |

### Integration Points
- `internal/core/ddosevent/event.go` - would need to carry the exempted source set, not just a bool. This is the blast radius (see "The keystone gap").
- `internal/plugins/ddos/local/match.go` - `buildDropTerm` is the single term builder; there is no second construction path to keep in sync.
- `internal/component/firewall.MatchSourceAddress` (`model.go`) - the unused primitive to wire. **No firewall-side work is needed**: nft (`internal/plugins/firewall/nft/lower_linux.go`) and VPP (`vpp/translate.go`, `vpp/verify.go`) already lower it.
- `internal/plugins/ddos/local/responder.go` - term application and hook selection; `applyMitigation` calls `buildDropTerm` and honors `SuppressMitigation` wholesale at `:99-109`.
- `ddos/detect`'s policy evaluation - where the exempt set is known, and per 1110 the only place allowed to resolve it.

### Architectural Verification
- [ ] No bypassed layers (policy stays evaluated at detection; the responder does not learn policy)
- [ ] No unintended coupling (ddos must not import policy internals)
- [ ] No duplicated functionality (reuse `MatchSourceAddress`, do not add a ddos-local source match)
- [ ] Registration over hardcoding — no new per-feature field/switch in shared firewall code (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Operators actually want per-source narrowing rather than incident-level exemption | Inferred from the deferral row; NOT stated by the user | The whole spec is a non-problem and should be cancelled | Ask Thomas before design | unvalidated |
| A-2 | The exempted source set is small enough to express as firewall matches | Unverified; a policy could exempt a large prefix list | Term explosion in the dataplane; nft ruleset size becomes a limit | Measure term count for realistic policies | unvalidated |
| A-3 | Carving the drop does not weaken mitigation under spoofing | Source addresses in a DDoS are frequently spoofed | An attacker spoofs the exempted source and bypasses mitigation entirely | Security review; this may be a reason to reject the feature | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A spoofable exemption becomes an attack vector | Security review flags it | Restrict to anti-spoofing-protected sources, or reject the feature |
| R-2 | Changing exemption semantics surprises operators relying on v1 | Behavior change in an existing config | Gate behind a new explicit policy option; keep v1 the default |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An attack with a source-exempt policy rule | → | `buildDropTerm` emitting a source-carved term | `TestBuildDropTermCarvesExemptSource` |
| Characterized event carrying an exempt source set | → | responder installs the carved term | `TestLocalResponderInstallsCarvedTerm` |
| End-to-end flood with an exempted source | → | detection → policy → term → nft | `test/plugin/ddos-source-carve.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Attack with no source exemption | Drop term is byte-identical to today's (no regression) |
| AC-2 | Attack with one exempted source prefix | The installed term drops the vector but not traffic from that prefix |
| AC-3 | Attack with an exemption covering the entire attack | Behavior matches today's `SuppressMitigation` (whole-incident suppression) |
| AC-4 | Exemption set is empty | `SuppressMitigation` polarity is unchanged: zero value still means mitigate |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator exempts a monitoring source, then a flood hits the victim | policy → detection → event → buildDropTerm → nft term | `test/plugin/ddos-source-carve.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildDropTermCarvesExemptSource` | `internal/plugins/ddos/local/match_test.go` | AC-2: term includes a source carve-out | |
| `TestBuildDropTermUnchangedWithoutExemption` | `internal/plugins/ddos/local/match_test.go` | AC-1: no regression | |
| `TestLocalResponderInstallsCarvedTerm` | `internal/plugins/ddos/local/responder_test.go` | AC-2 at the responder seam | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| exempted source prefix length (v4) | 0-32 | 32 | N/A | 33 |
| exempted source prefix length (v6) | 0-128 | 128 | N/A | 129 |
| exempted source set size | 0-N | design decides N | N/A | N+1 (reject, do not truncate) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-source-carve` | `test/plugin/ddos-source-carve.ci` | flood with one exempted source: attack dropped, exempt source still passes | |

Note: ddos flood `.ci` tests must run serially and need a UDP sink bound on the victim, and
the nft backend deadlock the parent recorded as a gotcha applies. Read
`plan/spec-fixit-ddos-test-infra.md` before authoring a flood `.ci`.

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/plugins/ddos/local/match.go` - `buildDropTerm` gains a source carve-out
- `internal/plugins/ddos/local/responder.go` - pass the exempt set through
- `internal/core/ddosevent/event.go` - carry the exempt source set on the event
- `internal/plugins/ddos/local/yang/` - policy surface for per-source exemption, if design approves

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | `internal/plugins/ddos/local/yang/` if a new policy option lands |
| Functional test for new RPC/API | [ ] | `test/plugin/ddos-source-carve.ci` |
| Prometheus counters/metrics | [ ] | consider a counter for carved-out sources |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` if the policy option lands |
| 6 | Has a user guide page? | [ ] | `docs/guide/ddos.md` |

## Files to Create
- `test/plugin/ddos-source-carve.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; resolve A-1 with Thomas FIRST |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `./le verify current mode full` |
| 14. Present summary + close | two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

1. **Phase: Confirm the problem (MANDATORY FIRST)** — resolve A-1 and A-3 with Thomas
   - Verify: Thomas confirms per-source narrowing is wanted AND that spoofing does not kill it. If either fails, CANCEL this spec rather than build it.
2. **Phase: Wiring** — carry the exempt set on the event; failing tests
   - Tests: `TestLocalResponderInstallsCarvedTerm`
   - Verify: FAILS because `buildDropTerm` ignores the set
3. **Phase: Carve the term** — wire `firewall.MatchSourceAddress`
   - Tests: `TestBuildDropTermCarvesExemptSource`, `TestBuildDropTermUnchangedWithoutExemption`
   - Verify: unit tests pass; AC-1 proves no regression
4. **Phase: Functional proof** — `ddos-source-carve.ci`
5. **Full verification** → `./le verify current mode full`
6. **Complete spec** → learned summary + the 1110 correction; TWO commits (A: code+tests+spec+learned; B: `git rm` spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-4 each have a test |
| Fail-closed | An error building the carve-out mitigates fully; it never skips the drop (`ai/rules/evidence.md`) |
| Correctness | `SuppressMitigation` zero-value polarity preserved |
| Data flow | Policy still evaluated once at detection; the responder learns no policy |
| Registration over hardcoding | No ddos-specific field added to shared firewall model (`ai/rules/plugins.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `MatchSourceAddress` wired for ddos | `grep -rn MatchSourceAddress internal/plugins/ddos/` returns hits |
| 1110 corrected | grep 1110 for the per-source limitation record |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Spoofing | An attacker forging the exempted source address bypasses mitigation (A-3). This is the central security question. |
| Input validation | Exempt prefixes validated; no unbounded set driving term explosion |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 rejected by Thomas | CANCEL the spec; record the decision in `plan/deferrals.md` as `cancelled` / `user-approved-drop` |
| A-3 shows a spoofing bypass | STOP; report to Thomas; the feature may be unsafe by construction |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The limitation was recorded in the source spec's learned summary, Known Limitations (per the deferral row) | That summary had no Known Limitations section and never mentioned it | Grep during the 2026-07-16 deferral sweep | The knowledge lived only in a deferral row; closing this spec must fix that |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- 1110's "evaluate policy once, encode the outcome on the event" decision is what makes this expensive: a boolean cannot carry a source set, so the event schema is the real blast radius, not `buildDropTerm`.
- The feature looks one line deep from the firewall side (`MatchSourceAddress` exists and every backend already lowers it) and is expensive from the event side (`VectorTuple` has no source field). Judging cost from the primitive rather than the path that must carry data to it would understate this spec badly.

## Known Limitations
- (fill during design)

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/plugins/ddos/*`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
