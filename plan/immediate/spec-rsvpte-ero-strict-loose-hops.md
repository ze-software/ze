# Spec: RSVP-TE strict-hop adjacency and loose-hop expansion (RFC 3209 Sections 4.3.3.1, 4.3.4.2)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The started implementation replaces the old ERO-only next-hop selection with `internal/plugins/rsvpte/routing.go::resolveExplicitPath` and the transport's native route query. Strict hops must resolve to an adjacent member of their abstract node. Loose hops insert the resolved adjacent next hop before the remaining loose subobject; expansion is bounded. `resolveReceivedPath` validates and removes local subobjects before forwarding. Five PATH wire tests cover the two MUST rows with revert-route records, and the Docker suite `interop-rsvpte` proves strict forward, strict refusal and loose expansion at a Ze transit against freeRouter, each with a recorded red run (see "Interop status"). This is not a claim of CSPF or full RFC 3209 support.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.3.3.1-1 | "The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node." (§4.3.3.1) | `routing.go::resolveExplicitPath` rejects a strict subobject whose resolved next hop is outside that abstract node |
| RFC3209-4.3.4.2-1 | "Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node." (§4.3.4.2) | `routing.go::resolveReceivedPath` removes locally satisfied subobjects; `resolveExplicitPath` expands the remaining route using native adjacency information; source and error paths require validation |

## Required Reading

| Source | Why |
|--------|-----|
| `rfc/full/rfc3209.txt` Sections 4.3.3.1, 4.3.4.1, 4.3.4.2 | The two sentences the rows quote, and step 5b for loose hops |
| `docs/contributing/rfc-conformance-gates.md` "The discrimination record" | Tag form, record fields, the revert route |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Configured explicit routing |

## Current Behavior

- [ ] `internal/plugins/rsvpte/routing.go` -- `resolveReceivedPath`, `resolveExplicitPath`
- [ ] `internal/plugins/rsvpte/peer_identity.go` -- `peerInPrefix`, abstract-node membership
- [ ] `internal/plugins/rsvpte/engine.go` -- the PATH handler turning a selection error into PathErr 24

The producer was committed in 2dc7ec1802; this work adds proof only.

## Data Flow

### Entry Point
An RSVP PATH carrying an EXPLICIT_ROUTE object reaching `engine.handlePacket` at a transit.

### Transformation Path
`DecodeMessage` -> `resolveReceivedPath` -> `resolveExplicitPath` (native `Transport.ResolveRoute`) -> forwarded PATH via `SendPath`, or `sendPathErr` with Routing Problem.

### Boundaries Crossed
Wire bytes in and out; the native route query on the transport.

### Integration Points
`transport.ResolveRoute`, `transport.SendPath`, `engine.sendPathErr`.

## Wiring Test

| Entry Point | -> Feature Code | -> Test |
|-------------|-----------------|---------|
| `engine.handlePacket` with an encoded PATH | -> `routing.go::resolveExplicitPath` | -> `rfc3209_explicit_hops_test.go`, decoding the PATH or PathErr the fake transport sent |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Requirement | Polarity |
|------|-------------|----------|
| `TestRFC3209StrictHopReachedDirectly` | RFC3209-4.3.3.1-1 | positive |
| `TestRFC3209StrictHopThroughOutsideNodeRefused` | RFC3209-4.3.3.1-1 | negative |
| `TestRFC3209InteriorHopStaysInAbstractNode` | RFC3209-4.3.4.2-1 | positive |
| `TestRFC3209InteriorHopLeavingAbstractNodeRefused` | RFC3209-4.3.4.2-1 | negative |
| `TestRFC3209LooseHopExpandedWithNextHop` | none (step 5b contrast) | untagged |

## Files to Modify

| File | Change |
|------|--------|
| `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` | New: the five wire tests |
| `rfc/discrimination/rfc3209.json` | Four revert-route records |
| `rfc/short/rfc3209.md` | Two rows lose `{gap}`; Support remaining updated |

## Implementation Steps

1. Write each test as a received PATH with a fixture route, decoding what the transit sends.
2. Record each tag with `./le rfc discriminate-record` (revert route).
3. Prove the strict refusal by Go overlay mutation of the `!inside && !target.Loose` guard.
4. Remove the two `{gap}` annotations.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL under a break of the producer (the discrimination records)
- [ ] Tests PASS
- [ ] `./le verify worktree` (owed by the main thread)

### Integration Checklist

- [ ] `./le rfc check` reports no stale or owed record for RFC 3209

### Documentation Update Checklist

- [ ] Producer code changed in 9b8bfe250c (`buildPathErr` sends the ADSPEC of the PATH in error, and a ResvTear carries its FLOWSPEC, so a freeRouter relay accepts both); its page edit is `docs/architecture/rsvpte/mpls-rsvp-te.md`, in the same commit
- [ ] `rfc/short/rfc3209.md` Support remaining and `docs/guide/rsvp-te.md` state what the interop suite proves (53c35bc0db)

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Five PATH wire tests in `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` | `grep -n '^func TestRFC3209' internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` lists the five names of the TDD Test Plan; `go test ./internal/plugins/rsvpte/` under `./le job run` passes |
| Four revert-route records, one per tagged test | `grep -o 'TestRFC3209[A-Za-z]*Hop[A-Za-z]*' rfc/discrimination/rfc3209.json` names each of the four tagged tests once; `./le rfc check` reports no violation naming rfc3209 |
| Rows RFC3209-4.3.3.1-1 and RFC3209-4.3.4.2-1 carry no `{gap}` | `grep -n 'RFC3209-4.3.3.1-1' rfc/short/rfc3209.md` and the same for RFC3209-4.3.4.2-1 |
| Interop: strict forward, strict refusal and loose expansion at a Ze transit | `RSVPTE_INTEROP_SCENARIO=<name> ./le test integration interop-rsvpte` for `transit-strict-hop-forwarded`, `transit-strict-hop-outside-refused` and `transit-loose-ero-expansion` |
| Each of the three scenarios goes red when its producer is broken | The red and green logs named in "Interop status" below; each red run used `ZE_REPO_ROOT` on a scratch copy of the tree carrying the break, so Ze's image was rebuilt from the broken copy |
| Public pages state what the interop proves and no more | `rfc/short/rfc3209.md` Support remaining and the `docs/guide/rsvp-te.md` Status note name the freeRouter suite, say freeRouter does not enforce strict hops, and name the test-only freeRouter patch |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The ERO is untrusted wire input. `resolveReceivedPath` refuses an empty ERO (Bad EXPLICIT_ROUTE object) and a first subobject that does not contain this node (Bad initial subobject) before any route lookup |
| Fail closed | A strict hop whose native next hop is outside both abstract nodes returns Bad strict node and the PATH is not forwarded (`TestRFC3209StrictHopThroughOutsideNodeRefused`, `transit-strict-hop-outside-refused`), so a strict path never falls through to native routing |
| Resource exhaustion | Loose expansion adds one subobject and is refused once the list reaches `maxExplicitRouteHops` (64, `wire.go`); the local-prefix loop only consumes the received list, so a crafted ERO cannot grow work or memory without bound |
| Loop back to self | A route whose next hop is a local address is refused, so an ERO cannot make the transit forward the PATH to itself |
| Error leakage | The PathErr carries the error code, value and this node's address as error node; nothing from the route table or internal state |

## Risks & Assumptions

| Item | Note |
|------|------|
| Not CSPF | Next-hop selection is the native route query; no TE path computation is claimed |
| Interop | Proven against freeRouter only, in `./le test integration interop-rsvpte` (see "Interop status"). freeRouter names its configured hops loose and prepends one strict subobject for its own next hop (`ipFwdTab.fillRsvpFrst`), routes each hop without reading the strict bit (`rtrRsvpIface.getHop`), and never originates PathErr, so it does not enforce strict hops and the strict scenarios prove freeRouter accepts and relays what a Ze transit sends, not that a second implementation agrees on strict-hop validation |

## Acceptance Criteria

Evidence: `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go`, which drives
a PATH into a transit and decodes the PATH or PathErr it sends. Each tagged
test carries a revert-route record in `rfc/discrimination/rfc3209.json`.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (RFC3209-4.3.3.1-1) | ERO [self/32, strict 10.0.0.9/32] with native next hop 10.0.0.9; separately, next hop 10.0.0.7, outside both nodes | PATH sent to 10.0.0.9 with ERO [10.0.0.9/32] and no PathErr (`TestRFC3209StrictHopReachedDirectly`); no PATH, and PathErr 24/2 to the ingress (`TestRFC3209StrictHopThroughOutsideNodeRefused`) |
| AC-2 (RFC3209-4.3.4.2-1) | ERO [10.1.0.0/16 holding the transit, strict 10.9.0.9/32] with next hop 10.1.0.2 inside the /16; separately, next hop 10.2.0.2 outside it | PATH sent to 10.1.0.2, every subobject ahead of the strict node a subset of 10.1.0.0/16 (`TestRFC3209InteriorHopStaysInAbstractNode`); no PATH, and PathErr 24/2 (`TestRFC3209InteriorHopLeavingAbstractNodeRefused`) |
| AC-3 (loose hop, Section 4.3.4.1 step 5b) | The AC-1 refusal topology with the L bit set | PATH sent to 10.0.0.7 with ERO [10.0.0.7/32, loose 10.0.0.9/32] (`TestRFC3209LooseHopExpandedWithNextHop`, untagged because no MUST row binds it) |
| AC-4 | `rfc/short/rfc3209.md` | Both rows carry no `{gap}`; Support remaining no longer calls their proof open |

Semantic discrimination beyond the revert records, by Go overlay on 2026-10-08:
disabling the `!inside && !target.Loose` refusal in `resolveExplicitPath`
reddens both negative tests. The overlay exposed no producer defect. Producer
code did change for interop: 9b8bfe250c makes `buildPathErr` send the ADSPEC of
the PATH in error and a ResvTear carry its FLOWSPEC, because freeRouter drops
either message without that object; its page edit is
`docs/architecture/rsvpte/mpls-rsvp-te.md`.

## Closure Review (2026-10-08): closure withheld on interop

Independent review of 48c344b081 by a session that wrote none of it.

| Check | Result |
|-------|--------|
| Claim against assertion, 4 tagged tests | Each `RFC requirement:` claim states what its test body asserts and no more; the untagged loose test asserts step 5b expansion only |
| RFC quotes | Both quotes found verbatim in `rfc/full/rfc3209.txt` (Sections 4.3.3.1, 4.3.4.2) |
| `./le rfc check` | No violation names rfc3209 |
| Scoped tests | `go test ./internal/plugins/rsvpte/` green, `go vet` clean |

**Interop gap: this spec does not close.** `ai/rules/interop-and-goal-validation.md`
requires a scenario against another implementation, and this spec's own Risks
row says strict and loose ERO interop is unproven. The only RSVP-TE carrier,
`freertr_interop_integration_linux_test.go::TestRSVPFreeRouterInterop`, makes
Ze the egress: the peer's ERO ends at Ze, so no strict or loose transition is
resolved at a Ze transit. Scenario needed: freeRouter ingress signalling
through Ze as transit to a second node, asserting at the peers (1) a strict hop
to a node Ze reaches directly: the downstream peer receives the PATH with the
trimmed ERO and the LSP comes up, (2) a strict hop whose native next hop is
outside both abstract nodes: the ingress peer receives PathErr 24/2, and (3)
the same hop loose: the downstream peer receives the PATH with the expanded
ERO. The carrier needs root, so it was not run in this review.

## Interop status (2026-10-08): strict forward, strict refusal and loose expansion proven

`./le test integration interop-rsvpte` (`internal/le/interoplab/rsvpte/`,
catalog suite `rsvpte`) ran on 2026-10-08 over a tree carrying the MPLS
path-MTU probe (f3caa99352), so a Ze transit installs its swap and the LSP comes
up. The suite now has back-to-back scenarios: Ze originates, freeRouter
is the independent implementation that parses the message, keeps the
state it needs and re-encodes it, and a second Ze captures what freeRouter sent.
freeRouter relays what Ze originates; it neither originates PathErr nor enforces
strict hops itself. It names its configured ERO hops loose
(`clntMplsTeP2p.workDoer`, `ipFwdTab.fillRsvpPack`) and prepends one strict
subobject for its own next hop unless the first hop is already strict
(`ipFwdTab.fillRsvpFrst`), and it routes each hop without reading the strict bit
(`rtrRsvpIface.getHop`). The evidence is that freeRouter accepts and relays Ze's
messages. Run log: session scratch
`rsvpte-run8.log` (3 passed, 2 failed) before commit 9b8bfe250c, and
`rsvpte-run9.log` (5 passed, 0 failed) after it. Discrimination for (3):
`rsvpte-disc-run10.log`. Discrimination for (1) and (2), 2026-10-08: each red
run used a scratch copy of the working tree with one break in
`internal/plugins/rsvpte/routing.go`, run as `ZE_REPO_ROOT=<copy>
RSVPTE_INTEROP_SCENARIO=<name> ./le test integration interop-rsvpte`, so the
harness cross-compiled Ze and rebuilt its image from the broken copy; each
green run is the same command over this checkout. All logs are in
`tmp/session/2026-10-07-450bc92b-6ac1-4190-bd40-b427ecba17bf/scratch/`.

The image every scenario runs is built with
`test/interop-rsvpte/freertr/ze-interop-resv.patch`. Its two knobs are off
unless a scenario's `<role>-env.txt` sets them, and none of the three scenarios
here sets one, so freeRouter runs its upstream behavior in all three.

| Needed (Closure Review) | Scenario | Result |
|-------------------------|----------|--------|
| (1) strict direct hop | `transit-strict-hop-forwarded` | PASS. The Ze ingress names every hop strict through a freeRouter relay; the freeRouter egress captures the Ze transit's PATH with the ERO trimmed to strict `172.29.81.14` alone, the Ze ingress captures the labelled RESV freeRouter relays, and the Ze transit holds a swap via `.14`. Discrimination: with `resolveReceivedPath` changed to keep the transit's own subobject in the forwarded ERO (a transit-only break; the Ze ingress originates through `resolveOriginatingPath` and is untouched), the run is red, `strict-fwd-red4.log`: "egress received ERO [... Strict, 172.29.81.3/32 ... Strict, 172.29.81.14/32 ...], want only strict 172.29.81.14". Over this checkout it passes, `strict-fwd-green.log` (1 passed, 0 failed). Three earlier red runs are not counted as discrimination: `strict-fwd-red.log` and `strict-fwd-red2.log` broke the trim inside `resolveExplicitPath`, which the Ze ingress also runs, and `strict-fwd-red3.log` disabled step 4 there, which made the Ze ingress refuse its own PATH ("strict node 172.29.81.15/32 is not adjacent"); in all three the checker never read the egress capture, so the red does not isolate the transit |
| (2) strict hop outside -> PathErr 24/2 | `transit-strict-hop-outside-refused` | PASS (`rsvpte-run9.log`, and `strict-refused-green.log` on 2026-10-08: 1 passed, 0 failed). The Ze transit sends PathErr Routing Problem / Bad strict node (24/2), error node `172.29.81.3`, and never forwards the PATH; the Ze ingress captures that PathErr as freeRouter relays it. Discrimination: with the guard `if !inside && !target.Loose` in `resolveExplicitPath` disabled, the run is red, `strict-refused-red.log`: "wait for Ze ingress receives the PathErr freeRouter relays timed out", while the relay capture holds the PATH it relayed to the transit with ERO [strict `172.29.81.3`, strict `198.51.100.9`] and no PathErr from the transit. `rsvpte-run8.log` is not a discrimination of this guard: it was red because Ze's PathErr then carried no ADSPEC, which `packRsvp.parseDatPatErr` requires (RFC 2205 Section 3.1.3: "<sender descriptor> ::= <SENDER_TEMPLATE> <SENDER_TSPEC> [ <ADSPEC> ]"); since 9b8bfe250c `buildPathErr` copies the ADSPEC of the PATH in error, per the owner's decision of 2026-10-08 |
| (3) loose hop expanded | `transit-loose-ero-expansion` | PASS. freeRouter ingress, Ze transit, freeRouter egress: from the ingress ERO `[172.29.81.3 strict, 172.29.81.3 loose, 198.51.100.4 loose]` (Ze is `172.29.81.3`; ingress capture in `rsvpte-disc-run10.log`) the egress captures `[172.29.81.14 strict, 198.51.100.4 loose]`, the ingress captures a labelled RESV, and Ze holds the swap. Discrimination: with the replacement in `resolveExplicitPath` removed the check goes red (the egress receives `[172.29.81.3 loose, 198.51.100.4 loose]`); restored, it passes |

All three needed scenarios pass with no freeRouter test knob set, and each goes
red when its producer in `routing.go` is broken.

---

## Implementation Summary

### What Was Implemented
- Five PATH wire tests in `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go`, four tagged against RFC3209-4.3.3.1-1 and RFC3209-4.3.4.2-1, each driving `engine.handlePacket` and decoding what the fake transport sent (48c344b081).
- Four revert-route records in `rfc/discrimination/rfc3209.json`; both rows in `rfc/short/rfc3209.md` lose `{gap}`.
- The Docker suite `interop-rsvpte` (`internal/le/interoplab/rsvpte/`, `test/interop-rsvpte/`) with the three transit scenarios of "Interop status", each with a recorded red (19c033724c, 3d9a305b1d, 5947c4577e, 4689d133a6).
- `buildPathErr` sends the ADSPEC of the path state it reports on and a ResvTear carries its FLOWSPEC (9b8bfe250c), so freeRouter relays both.
- RFC 3209 and RFC 4090 records and verdicts re-recorded or re-judged after the 9b8bfe250c test-file shifts (59e43cbb2e, fe4b806641, 06e0ae00f9).

### Bugs Found/Fixed
- Ze's PathErr carried no ADSPEC, which freeRouter's `packRsvp.parseDatPatErr` requires, so `transit-strict-hop-outside-refused` was red (`rsvpte-run8.log`). Fixed in 9b8bfe250c; covered by `error_descriptor_test.go`.

### Documentation Updates
- `rfc/short/rfc3209.md` Support remaining and `docs/guide/rsvp-te.md` Status note: what the suite proves (53c35bc0db, db038c1164).
- `docs/architecture/rsvpte/mpls-rsvp-te.md`: PathErr ADSPEC (9b8bfe250c).
- `docs/architecture/testing/interop.md`: the freeRouter suite and the ERO its ingress signals (3dd0d9443f).
- Closure: `features/mpls-ldp-rsvp-te.md` drops the "ERO strict and loose hops" Scope gap, restates the Defect review citation as the bare stem, replaces the Description sentence that put all freeRouter interop "outside the interop suites" with the suite and what it proves (source anchor `internal/le/interoplab/rsvpte/checkers.go`), and carries a Doc review dated 2026-10-08.
- `./le doc check links`: one broken reference, in `internal/le/rfc/check_reference.go` (`docs/reference/`), another session's; nothing names a file of this spec. `./le doc check verify` fails on `../gh-pages` wire-method drift from another spec's rename.

### Deviations from Plan
- The plan named three files; interop needed a new Docker suite and a producer change (9b8bfe250c), both recorded above and in "Interop status".

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Interop was first left to the Linux carrier, which makes Ze the egress and never ran | A Ze transit needed freeRouter on both sides | Closure Review 2026-10-08 | Docker suite `interop-rsvpte` built |
| approach | Three strict-forward red runs broke code the Ze ingress also runs | The red must isolate the transit | Review R1 | `strict-fwd-red4.log` breaks `resolveReceivedPath` only |
| assumption | Public prose said freeRouter originates only loose hops and cited the never-run carrier as evidence | `ipFwdTab.fillRsvpFrst` prepends a strict hop; the carrier has no recorded pass | Review R2 | db038c1164, 83a8771b8b |
| approach | The feature declaration still said freeRouter interop runs "outside the interop suites" after the suite landed | The phase that added the suite owed the declaration edit | Closure review | Repaired in closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC3209-4.3.3.1-1 proven | Done | `rfc3209_explicit_hops_test.go`, the two `TestRFC3209StrictHop*` tests | producer `routing.go::resolveExplicitPath`, the `!inside && !target.Loose` guard |
| RFC3209-4.3.4.2-1 proven | Done | `rfc3209_explicit_hops_test.go`, the two `TestRFC3209InteriorHop*` tests | producer `resolveExplicitPath`, `resolveReceivedPath` |
| Interop at a Ze transit | Done | `internal/le/interoplab/rsvpte/checkers.go` | three scenarios, reds recorded |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC3209StrictHopReachedDirectly`, `TestRFC3209StrictHopThroughOutsideNodeRefused` | records in `rfc/discrimination/rfc3209.json` |
| AC-2 | Done | `TestRFC3209InteriorHopStaysInAbstractNode`, `TestRFC3209InteriorHopLeavingAbstractNodeRefused` | records in `rfc/discrimination/rfc3209.json` |
| AC-3 | Done | `TestRFC3209LooseHopExpandedWithNextHop` | untagged |
| AC-4 | Done | `rfc/short/rfc3209.md`, the two requirement rows and Support remaining | no `{gap}` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the five `TestRFC3209*` tests | Done | `rfc3209_explicit_hops_test.go` | package green |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` | Done | |
| `rfc/discrimination/rfc3209.json` | Done | four records |
| `rfc/short/rfc3209.md` | Done | |

### Audit Summary
- **Total items:** 11
- **Done:** 11
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (additions in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A strict hop resolves only to an adjacent member of its abstract node | unit wire test + interop | the four tagged tests with revert records; `transit-strict-hop-forwarded` (`strict-fwd-green.log`, red `strict-fwd-red4.log`) and `transit-strict-hop-outside-refused` (`strict-refused-green.log`, red `strict-refused-red.log`) |
| A loose hop is expanded with the adjacent next hop, bounded | unit wire test + interop | `TestRFC3209LooseHopExpandedWithNextHop`; `transit-loose-ero-expansion` (`rsvpte-run9.log`, red `rsvpte-disc-run10.log`); bound `maxExplicitRouteHops` checked in `resolveExplicitPath` |
| Local subobjects removed before forwarding | interop | `strict-fwd-red4.log`: keeping the transit's own subobject in `resolveReceivedPath` reddens `transit-strict-hop-forwarded` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None in scope. CSPF is excluded by the Task; a second implementation enforcing strict hops is unavailable (freeRouter does not) and is stated on the public pages | not in scope | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rsvpte-ero-strict-loose-hops-450bc92b-6ac1-4190-bd40-b427ecba17bf.md` (2 files, verdict clean) |
| `./le spec review check` | clean |
| Rounds | 1 (closure round over the commit-A population after R1-R3, which refused and whose findings were fixed in 53c35bc0db, 59e43cbb2e, 4689d133a6, db038c1164, 83a8771b8b, fe4b806641, 06e0ae00f9, 3dd0d9443f) |
| Reviewer lenses used | logic over `resolveReceivedPath`, `resolveExplicitPath`, `buildPathErr` and its callers; security (ERO bound, self-loop, fail-closed strict); RFC quotes against `rfc/full/rfc3209.txt`; public pages and the feature declaration against `checkers.go` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Feature Description said freeRouter interop runs "outside the interop suites"; the Doc review attested it | `features/mpls-ldp-rsvp-te.md` | Description and Doc review rewritten in closure |

NOTEs: the Scope gap "cross-vendor RSVP-TE interop outside the interop suites" is left for the owner, since it may name vendors other than freeRouter; `docs/architecture/rsvpte/mpls-rsvp-te.md` does not describe strict/loose transit handling, which predates this proof-only spec (2dc7ec1802); `./le repo check`, `./le commit audit` and `./le feature check` findings all name other sessions' files.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` | yes | `grep -n '^func TestRFC3209'` lists the five names |
| `test/interop-rsvpte/scenarios/transit-strict-hop-*`, `transit-loose-ero-expansion` | yes | setup scripts present |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | tests pass | `./le job run ... go test -count=1 ./internal/plugins/rsvpte/`: ok (`close-gotest.log`) |
| AC-4, records | no `{gap}`, records fresh | `grep -o` names each tagged test once in `rfc/discrimination/rfc3209.json`; `./le rfc check` names no rfc3209, rfc4090 or rfc2205 requirement (`close-rfccheck2.log`, 86 violations, all other RFCs) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `engine.handlePacket` | none: the unit tests feed encoded PATH bytes to `handlePacket`; the daemon path is the `interop-rsvpte` suite | test body read at closure: `explicitHopPath` builds with `buildPath`, the transit decodes via `handlePacket` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| Not CSPF | confirmed | `resolveExplicitPath` calls only `transport.ResolveRoute` |
| Interop | confirmed | freeRouter only; strict enforcement by freeRouter absent, stated on the guide, `rfc/short/rfc3209.md` and the feature declaration |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature Description interop sentence | `checkers.go` scenario constants; the `docs/guide/rsvp-te.md` Status note | yes |
| PathErr carries ADSPEC | `build.go::buildPathErr`, `engine.go::sendPathErr` passes `msg.AdspecRaw` | yes |
| Description source anchors | every `<!-- source: -->` path in `features/mpls-ldp-rsvp-te.md` exists | yes |
