# Spec: fixit-local-asn-config-key

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 (implementation complete) |
| Updated | 2026-07-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/role/config.go` - the role plugin's copy of `extractLocalASN`
4. `internal/component/bgp/plugins/gr/gr_llgr.go` - the gr plugin's copy of `extractLocalASN`
5. `internal/component/bgp/reactor/config.go` - the reader that shows the real key path

## Task

**Two BGP plugins read a config key that does not exist, so both silently degrade to
a zero local AS.** `extractLocalASN` is near-duplicated in the `role` and `gr`
plugins -- NOT verbatim. Both read `bgpSubtree["local-as"]`, but the copies diverge:
role's coerces string-delivered leaves and warns on every miss
(`internal/component/bgp/plugins/role/config.go:249-257`); gr's has neither a `string`
case nor any warning (`internal/component/bgp/plugins/gr/gr_llgr.go:261-279`). The
config tree carries the global local AS at `bgp/session/asn/local`, never at
`bgp/local-as`. Both copies therefore return 0 for every real configuration, and each
caller treats 0 as a valid answer rather than a lookup failure
(`ai/rules/fail-closed-guards.md`: a zero value must never be a valid-looking answer).
The divergence matters for the fix: because gr lacks the `string` case, a naive key
rename in gr still returns 0 for string-delivered leaves, so the two copies cannot be
repaired by the same one-line change.

Found on 2026-07-16 while verifying an unrelated deferral row (AS-Confederation OTC,
now `plan/spec-bgp-deferred-confederation-otc.md`). Not previously in any spec's scope.

**Verified at the producer, 2026-07-16:**

| # | Fact | Evidence |
|---|------|----------|
| 1 | The role plugin reads key `local-as` from the BGP subtree | `internal/component/bgp/plugins/role/config.go:236` |
| 2 | The gr plugin reads the same non-existent key, in a NEAR-duplicate (no `string` case, no warning) | `internal/component/bgp/plugins/gr/gr_llgr.go:261-279` (cf. role's `string` case, `role/config.go:249-257`) |
| 3 | The real tree carries the global local AS at `bgp` > `session` > `asn` > `local` | `internal/component/bgp/reactor/config.go:480-486`, whose own comment states it |
| 4 | The YANG declares it at `session/asn/local`; no top-level `local-as` leaf exists | `internal/component/bgp/yang/ze-bgp-conf.yang:85` (the only other `local` leaf, `:446`, is a per-peer override) |
| 5 | Callers: role at `role.go:155`, gr at `gr.go:175` | both assign the 0 result into live filter state |

**Verified at runtime, 2026-07-17 (readiness pass).** Ran the committed functional
test that is supposed to guard this behavior and read the raw wire capture:
`bin/ze-test bgp plugin 390` (`role-otc-egress-stamp`), 3/3 deterministic PASS,
with `--save` artifacts inspected. Findings that CONFIRM the root cause and resolve
the Design-Insights contradiction below:

| # | Runtime fact | Evidence |
|---|--------------|----------|
| R1 | The OTC attribute is NOT on the wire. The dest ze-peer received the forwarded UPDATE with `ORIGIN, AS_PATH=[65000 65001], NEXT_HOP` and **no** ATTR_35, reporting `Differences: - ATTR_35: 0000fde8 (missing)` | save artifact `peer-stderr.log` for test 390 |
| R2 | Confirms Consequence 1: `getLocalASN()` returns 0 (top-level `local-as` absent), so `otc.go:430` `localASN > 0` skips the stamp, exactly as the producer analysis predicts | `otc.go:429-436`, corroborated by R1 |
| R3 | The AS_PATH prepend of the local AS 65000 DOES work on the same UPDATE. Only the role plugin's key read is broken; the reactor forward path parses the per-peer effective local AS correctly. This is why the "populate `PeerFilterInfo.LocalAS`" fix is viable | R1 (AS_PATH shows 65000 prepended); `peer_forward_facts.go:134` (`s.GlobalLocalAS` in scope) |
| R4 | Test 390 reports PASS anyway (3/3), deterministically. The dest peer detects the mismatch but its failure does not govern the test verdict, so the `.ci` guards nothing today. This is a VACUOUS test in the exit-code-masking defect class (`peer_contract.go:10-17`, overlaps `plan/spec-fixit-redistribute-establishment-stall.md`) | `bin/ze-test bgp plugin 390` exit 0 while R1 holds |

Consequence for implementation: the Wiring-first phase MUST first make test 390 actually
RED (its dest wire assertion must both run and govern the verdict) BEFORE the reader fix,
otherwise a green test proves nothing. See AC-2 and Known Limitations.

**Consequence 1: OTC egress stamping never happens (RFC 9234 R008).**
`getLocalASN` (`internal/component/bgp/plugins/role/role.go:66`) returns 0, and the
egress stamp is skipped by the `localASN > 0` guard
(`internal/component/bgp/plugins/role/otc.go:429-436`). Ze never adds the OTC
attribute on egress to a customer, peer, or RS-client, whatever the configuration.

**Consequence 2: the LLGR partial-deployment iBGP path is dead (RFC 9494 Section 4.5.3).**
`gr_egress.go:83` computes `isIBGP := dest.PeerAS == s.localAS`. With `s.localAS`
always 0, no real peer is ever classified iBGP. A stale route destined for a
non-LLGR **iBGP** peer therefore falls through to the eBGP branch and is converted
to a withdrawal (`gr_egress.go:97`), instead of being delivered with NO_EXPORT and
LOCAL_PREF=0 as Section 4.5.3 requires.

**Why the tests did not catch it.** `role/config_test.go:597` feeds a hand-written
`{"bgp":{"local-as":65001}}`, a shape the real config tree never produces, so the
unit test proves only that the parser can read its own fixture. This is the
"wrong production path" entry in the mistake log (`ai/rules/project-knowledge.md`):
the test must be driven from the tree the reactor actually builds.

**Scope:** one root cause, two call sites, one duplicated helper. Decide during design
whether the helper belongs in one shared place (both plugins reading the same tree
path) or whether each plugin keeps its own reader, per
`ai/rules/plugin-self-containment.md`. Deleting one copy in favor of an import across
plugin boundaries may not be legal here: check the tier rules first.

→ AUTONOMOUS DEFAULT (2026-07-17): Adopt the "Preferred fix" below (populate
`filterapi.PeerFilterInfo.LocalAS` on the forward path; both plugins read `dest.LocalAS`;
delete BOTH `extractLocalASN` copies). Do NOT introduce a shared cross-plugin reader and
do NOT import one plugin's helper into the other. Rationale: (a) it removes the root cause
rather than renaming a broken key in two places; (b) it needs no cross-plugin import, so it
sidesteps the tier / self-containment question entirely (each plugin still reads only the
`filterapi` value the reactor already hands it); (c) R3 above proves the reactor already
computes the correct per-peer effective local AS, and the forward-path builder already has
it in scope (`peer_forward_facts.go:134`), so the field can be filled for the forward path
just as `reactor_api_batch.go:829-833` already fills it for the readvertise path; (d) it is
the only option that gets the per-peer local-as override case right. Thomas: override if wrong.

## Key Design Decision

**Preferred fix: populate the local AS the plugins already receive, do NOT add a second
raw-tree read.** The filter API already carries the local AS for exactly this purpose:
`filterapi.PeerFilterInfo.LocalAS` (`internal/component/bgp/filterapi/filterapi.go:36`,
commented "Local AS number (for iBGP detection)"). The reactor already parses the local
AS and populates this field on the readvertise/stale path
(`internal/component/bgp/reactor/reactor_api_batch.go:829-833` sets `LocalAS: localAS`).
The forward-path builder OMITS it: `buildForwardFacts` constructs `PeerFilterInfo` with
Address, PeerAS, Name and GroupName only
(`internal/component/bgp/reactor/peer_forward_facts.go:123-128`) -- and the reactor
already has the value in scope there (`s.GlobalLocalAS`, read four lines below at `:134`).

Preferred approach: fill `LocalAS` on the forward path from the reactor's already-parsed
effective local AS, then have both plugins read `dest.LocalAS` instead of re-parsing raw
JSON. This deletes BOTH `extractLocalASN` copies (root cause removed, not renamed), avoids
reimplementing group/peer inheritance of the local AS inside plugin JSON, and fixes iBGP
detection under a per-peer local-as override: the reactor computes the EFFECTIVE local AS
per peer, whereas a global `local-as` / `session/asn/local` key read hard-codes the global
value and mis-detects iBGP whenever a peer overrides it (`ze-bgp-conf.yang:446-450`).

Fallback (raw-tree-key fix): rename the key each copy reads to `session/asn/local`. Lower
blast radius, but keeps two readers, reimplements inheritance in JSON, and gets the
per-peer-override case WRONG. Take it only if the forward-path field cannot be populated
for some peer class; justify against `ai/rules/plugin-self-containment.md` and the tier
rules first.

→ AUTONOMOUS DEFAULT (2026-07-17): Take the Preferred fix, NOT the Fallback. Rationale:
the spec already RECOMMENDs it and the readiness-pass runtime evidence (R1-R4) shows it is
safe and correct: the reactor already parses the effective local AS and already populates
`PeerFilterInfo.LocalAS` on the readvertise path, so the forward-path builder only needs the
one-line analogue at `peer_forward_facts.go:123-128` (fill `LocalAS: s.GlobalLocalAS`, or the
per-peer effective `s.LocalAS` when iBGP detection must honor a per-peer override, decide by
reading `peersettings.go:236-243`, which distinguishes `GlobalLocalAS` from the effective
`LocalAS`). Both plugins then drop `extractLocalASN` and read `dest.LocalAS`. The Fallback is
rejected because it reimplements group/peer inheritance in plugin JSON and mis-detects iBGP
under a per-peer override. Thomas: override if wrong.
→ PROVISIONAL sub-choice: for the OTC egress stamp (a single global-scope local AS, RFC 9234
R008) use `s.GlobalLocalAS`; for LLGR iBGP detection (which compares against the peer's own
AS) use the effective per-peer `s.LocalAS`. If a single field must serve both, prefer the
effective `s.LocalAS` and revisit OTC once an override peer is exercised. Reversible in code;
override before it ships if interop shows otherwise.

**Related spec / ordering.** `plan/spec-bgp-deferred-confederation-otc.md` schedules this
same role-plugin key fix as its Phase-1 "Prerequisite" (that spec's Implementation step 1
and assumption A-3 both depend on OTC egress stamping working first). That spec should
declare `Depends` on THIS spec, and this one should land first.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - config tree to plugin data flow
  → Constraint: config reaches plugins as JSON sections, not as typed structs
- [ ] `ai/rules/fail-closed-guards.md` - why a 0 from a failed lookup must not read as an answer
  → Constraint: a guard must fail closed or say something; silence plus a zero value is the bug
- [ ] `ai/rules/plugin-self-containment.md` - whether a shared reader is legal across plugins

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - Route Leak Prevention (OTC), R008 egress stamping
  → Constraint: on egress to customer/peer/RS-client with no OTC present, OTC MUST be set to the local AS
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart, Section 4.5.3 partial deployment
  → Constraint: a stale route to a non-LLGR iBGP peer is delivered with NO_EXPORT and LOCAL_PREF=0, not withdrawn

**Key insights:** (minimal context to resume after compaction)
- The bug is a config KEY mismatch, not a protocol bug. Both symptoms are downstream.
- `local-as` is a real user-facing concept but lives under `session/asn/local` in the tree.
- Fixing the key revives two code paths that have never run in production. Expect their first real exercise to surface further issues.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/role/config.go` - `extractLocalASN` reads `bgpSubtree["local-as"]` at `:236`, returns 0 on miss with no warning
- [ ] `internal/component/bgp/plugins/role/role.go` - `:155` stores the result; `getLocalASN` at `:66` serves it to the filter
- [ ] `internal/component/bgp/plugins/role/otc.go` - `:429-436` skips the egress stamp when the ASN is 0
- [ ] `internal/component/bgp/plugins/gr/gr_llgr.go` - `:261-279` near-duplicate of the same broken reader, but WITHOUT the `string` case or warnings that role has
- [ ] `internal/component/bgp/plugins/gr/gr.go` - `:175` assigns it into `egressFilterState.localAS`
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `:83` iBGP detection; `:97` the withdrawal branch it wrongly takes
- [ ] `internal/component/bgp/reactor/config.go` - `:480-486` the correct read of `session/asn/local`

**Behavior to preserve:**
- The per-peer local AS override (`session/asn/local` under a peer, `ze-bgp-conf.yang:446-450`) keeps its current meaning. This spec is about the GLOBAL local AS only.
- OTC stamping stays skipped when no local AS is configured at all. The guard is correct; its input is not.
- Non-stale routes continue to pass the LLGR egress filter untouched (`gr_egress.go:70-72`).
- `extractLocalASN`'s range validation (reject negative or above `math.MaxUint32`) is correct and stays.

**Behavior to change:**
- Both readers must read the local AS from the path the config tree actually uses.
- A failed lookup must be distinguishable from a configured 0.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator sets the global local AS in config: `bgp/session/asn/local` (`ze-bgp-conf.yang:85`).
- The value reaches plugins as a JSON config section delivered to the plugin's config handler.

### Transformation Path
1. Config file is parsed into the tree, then resolved to `map[string]any`.
2. The reactor reads `bgp` > `session` > `asn` > `local` for its own use (`reactor/config.go:480-486`).
3. The same section is delivered as JSON to the `role` and `gr` plugins.
4. Each plugin's `extractLocalASN` parses the BGP subtree and looks up `local-as`, which is absent, so returns 0.
5. `role` stores 0 in `filterLocalASN`; `gr` stores 0 in `egressFilterState.localAS`.
6. On egress, `otc.go:429-436` skips stamping, and `gr_egress.go:83` misclassifies every iBGP peer as eBGP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ BGP plugin | JSON config section, key lookup by string | [ ] |
| Plugin state ↔ egress filter | `filterapi` attribute mod ops on the forward path | [ ] |
| Ze ↔ peer (wire) | OTC attribute presence; announce vs withdrawal | [ ] |

### Integration Points
- `internal/core/bgp/configjson` `ParseBGPSubtree` - the shared subtree parser both copies call
- `filterapi` attribute modification ops - how both plugins act on the value

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No configuration path produces a top-level `bgp/local-as` key | `reactor/config.go:480-486`, `ze-bgp-conf.yang:85`, grep of the YANG | The readers are correct for some path and the fix is narrower | grep every producer of the BGP subtree JSON | confirmed -- grep of role/gr found no production `"local-as"` read after the fix; the value now flows only from `session/asn/local` via the reactor |
| A-2 | Fixing the key does not break peers relying on today's no-OTC behavior | RFC 9234 R008 says the stamp is required | An operator sees new OTC attributes on egress and route-leak filtering changes | interop test against a peer that reads OTC | confirmed (functional) -- test 390 `role-otc-egress-stamp` now GREEN with OTC on the wire; the stamp is REQUIRED per R008, so the new behavior is the correct one |
| A-3 | The LLGR iBGP branch is otherwise correct and has simply never run | `gr_egress.go:85-92` reads as complete | Reviving it surfaces a second bug behind the first | unit test driving a stale route to a non-LLGR iBGP peer | confirmed -- `TestLLGREgressIBGPClassification` + `TestLLGREgressFilter_IBGPPartial` drive the revived branch; live end-to-end via committed test 250 `llgr-readvertise-multipeer.ci`. No second bug surfaced. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fix silently changes egress on live sessions (OTC appears, LLGR withdrawals become announces) | Interop or functional tests show new attributes | Land behind tests that assert the RFC behavior explicitly; call it out in the release notes |
| R-2 | Two plugins are fixed but a third reader exists elsewhere | grep finds another `["local-as"]` | `model_dashboard_render.go:248` reads `local-as` from a DIFFERENT map (a rendered dashboard payload, not the config tree). Confirm it is unrelated before touching it |

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config sets `bgp/session/asn/local` | → | `role.extractLocalASN` via the delivered section | `TestRoleLocalASNFromReactorTree` |
| Config sets `bgp/session/asn/local` | → | `gr.extractLocalASN` into `egressFilterState.localAS` | `TestLLGREgressIBGPClassification` |
| Operator config to OTC on the wire | → | `role/otc.go` egress stamp | `test/plugin/role-otc-egress-stamp.ci` (exists -- extend, do not duplicate) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config tree built by the real config path with a global local AS | Both plugins' readers return that ASN, not 0 |
| AC-2 | Egress to a customer/peer/RS-client with no OTC present, local AS configured | OTC attribute stamped with the local AS (RFC 9234 R008) |
| AC-3 | A stale route to a non-LLGR iBGP peer | Delivered with NO_EXPORT and LOCAL_PREF=0, NOT withdrawn (RFC 9494 4.5.3) |
| AC-4 | No global local AS configured at all | Lookup failure is distinguishable from 0; behavior is the documented skip, and it says so |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractLocalASNFromRealTree` | `internal/component/bgp/plugins/role/config_test.go` | Reader is driven from a reactor-built tree, not a hand-written fixture | |
| `TestLLGREgressIBGPClassification` | `internal/component/bgp/plugins/gr/gr_egress_test.go` | A non-LLGR iBGP peer takes the NO_EXPORT branch | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `otc-egress-stamp` | extend `test/plugin/role-otc-egress-stamp.ci` (already committed) -- do NOT add a duplicate `test/bgp/otc-egress-stamp.ci` | Operator configures a global local AS and a customer peer, and the announced route carries OTC set to that AS | |
| `llgr-stale-ibgp-noexport` | `test/bgp/llgr-stale-ibgp-noexport.ci` | A stale route to a non-LLGR iBGP peer arrives with NO_EXPORT and LOCAL_PREF=0 rather than as a withdrawal | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `role-otc-egress` (extend the existing `20-role-frr`) | `test/interop/scenarios/20-role-frr/` | FRR | With a global local AS set and a Customer/RS-client session, the route Ze advertises to FRR carries OTC = local AS, and FRR accepts it | |

## Files to Modify
- `internal/component/bgp/plugins/role/config.go` - read the correct tree path
- `internal/component/bgp/plugins/gr/gr_llgr.go` - same, or share one reader
- `internal/component/bgp/plugins/role/config_test.go` - drive from a real tree
- `internal/component/bgp/plugins/gr/` - egress filter test for the iBGP branch

Added 2026-07-17 for the chosen Preferred approach (populate `PeerFilterInfo.LocalAS`
on the forward path; both plugins read `dest.LocalAS`; delete both `extractLocalASN`):
- `internal/component/bgp/reactor/peer_forward_facts.go` - fill `LocalAS` on the forward-path `PeerFilterInfo` (today omitted at `:123-128`; value already in scope as `s.GlobalLocalAS`/`s.LocalAS` at `:134`). No new `filterapi` field: `PeerFilterInfo.LocalAS` already exists (`filterapi.go:36`).
- `internal/component/bgp/plugins/role/otc.go` - stamp from `dest.LocalAS` (or `src`), not `getLocalASN()`; drop the package-global `filterLocalASN`/`getLocalASN` plumbing (`role.go:52,55-70,155,158`)
- `internal/component/bgp/plugins/gr/gr_egress.go` + `gr/gr.go` - compare `dest.PeerAS` against `dest.LocalAS`, not the captured `egressFilterState.localAS`
- `internal/component/bgp/plugins/gr/gr_llgr.go` - delete `extractLocalASN` (root cause removed, not renamed)
- `test/plugin/role-otc-egress-stamp.ci` - make the dest peer's ATTR_35 wire assertion actually GOVERN the verdict (today it detects the mismatch but the test still reports PASS, R4); this is the Wiring-first failing test for AC-2
- `test/interop/scenarios/20-role-frr/` - extend to assert OTC on egress to FRR

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - write the failing test that drives each reader from a tree the real config path builds
   - Verify: the test fails today, returning 0
2. **Phase: Fix the readers** - read `session/asn/local`; make a lookup miss distinguishable from a configured 0
   - Verify: readers return the configured ASN; AC-1 passes
3. **Phase: Prove the revived paths** - OTC stamping (AC-2) and the LLGR iBGP branch (AC-3)
   - Verify: both paths run for the first time; watch for bugs behind the bug (A-3)
4. **Phase: De-duplicate or justify** - one reader or two, decided against `ai/rules/plugin-self-containment.md` and the tier rules
   - → AUTONOMOUS DEFAULT (2026-07-17): DELETE both `extractLocalASN` copies (no shared reader, no cross-plugin import). Each plugin instead reads the `filterapi.PeerFilterInfo.LocalAS` value the reactor hands it. Self-containment is preserved because neither plugin imports the other; both consume the same reactor-provided field. See the resolved Key Design Decision. Thomas: override if wrong.
5. **Functional + interop tests**
6. **Full verification** → `make ze-verify`

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read source from Current Behavior |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (role/gr) the BGP subtree carries `local-as` at its top level | It carries `session/asn/local` | Verifying an unrelated OTC deferral row, 2026-07-16 | Two RFC behaviors inert in production |

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- A unit test that hand-writes its input tree proves the parser reads its own fixture, nothing more. Both copies of this reader had passing tests throughout.
- A committed functional test already asserts the OTC egress stamp on the wire:
  `test/plugin/role-otc-egress-stamp.ci` (added in cf17879de) expects `C023040000FDE8`
  (OTC = AS 65000) at the destination Customer peer (`:25`). Its config sets NO global
  local AS and NO top-level `local-as`; it sets only per-peer `session/asn/local 65000`
  (`:109`, `:139`). Two consequences for this spec:
  1. The proposed `test/bgp/otc-egress-stamp.ci` DUPLICATES this file. Extend the existing
     one; do not add a second copy.
  2. If that test genuinely passes, it CONTRADICTS Consequence 1 ("Ze never adds the OTC
     attribute ... whatever the configuration"): OTC is stamped from a per-peer local AS
     with no global key configured, while `otc.go:429` guards on `getLocalASN()` (which
     reads the global-key `filterLocalASN`, `role.go:60-70`). Either the stamp reaches the
     wire via a path other than `getLocalASN()` -- making the root-cause analysis
     incomplete -- OR the test passed vacuously through the exit-code-masks-BGP-assertions
     defect (`internal/test/runner/peer_contract.go:10-17`). It DOES carry
     `expect=bgp:...hex=...` wire assertions (not exit-code-only), so vacuity is not
     obvious; resolve which during design BEFORE trusting Consequence 1.

  → RESOLVED (2026-07-17, readiness pass, verified by running the test): Consequence 1
    STANDS -- OTC is NOT stamped. Ran `bin/ze-test bgp plugin 390` (3/3 PASS) and read the
    `--save` wire capture: the dest ze-peer received the UPDATE with `AS_PATH=[65000 65001]`
    but reported `ATTR_35: 0000fde8 (missing)`. So the wire assertion FAILS at the peer, yet
    the test still reports PASS. The `.ci` therefore passes VACUOUSLY: its `expect=bgp` ATTR_35
    assertion does not govern the verdict (the check-mode dest peer's failure is not propagated
    -- the same masking class as `peer_contract.go:10-17`). The root-cause analysis is CORRECT,
    not incomplete: the AS_PATH prepend works because the reactor forward path uses the per-peer
    effective local AS, while OTC stays 0 because `getLocalASN()` reads the absent top-level key.
    Action for the implementer: in the Wiring-first phase, first make test 390 RED (assertion
    must run AND govern), then land the reader fix so it goes green for the right reason. See
    Current Behavior "Verified at runtime" rows R1-R4.

- IMPLEMENTED (2026-07-18): The Preferred fix landed. Reactor fills
  `filterapi.PeerFilterInfo.LocalAS` with the effective per-peer `s.LocalAS` on
  BOTH forward-path construction sites (`peer_forward_facts.go` dest,
  `reactor_api_forward.go` src). `role/otc.go` stamps `dest.LocalAS` (fail-closed
  WARN when 0); `gr/gr_egress.go` classifies iBGP as `dest.PeerAS == dest.LocalAS`.
  BOTH `extractLocalASN` copies DELETED. The masking defect that made test 390
  vacuous (Design Insights R4) was already fixed upstream in `peer_contract.go`
  (`isSelfValidated` returns false when a check-mode ze-peer is present), so test
  390 now genuinely governs and is GREEN for the right reason -- no additional
  harness change was needed from this spec.
- IMPLEMENTED note on AC-3 functional coverage: `test/bgp/` does not exist; BGP
  `.ci` tests live in `test/plugin/`. The committed `llgr-readvertise-multipeer.ci`
  (test 250, rib-arch-7) already exercises a non-LLGR iBGP peer receiving a
  depreferenced (NO_EXPORT + LOCAL_PREF=0) readvertised stale route and passes with
  the fix, so no new duplicate `.ci` was added; the deterministic classification
  guard is the new `TestLLGREgressIBGPClassification` unit test.

## Known Limitations
- **The committed AC-2 guard is vacuous today.** `test/plugin/role-otc-egress-stamp.ci`
  reports PASS while the OTC attribute is missing on the wire (verified 2026-07-17, R4).
  Until its dest-peer wire assertion both runs and governs the verdict, it proves nothing;
  the fix is not "done" merely because this test stays green. Fixing the masking may overlap
  `plan/spec-fixit-redistribute-establishment-stall.md`; scope the minimum needed to make
  THIS test enforce ATTR_35, and note any broader harness gap for that spec.
- **Reviving two never-run paths may surface further bugs (A-3).** OTC egress stamping and
  the LLGR iBGP branch have never executed against a real config. Their first real exercise
  (AC-2, AC-3) may expose a second defect behind this one; treat a green reader fix as
  necessary, not sufficient, until the functional and interop tests assert the RFC behavior.
- **`GlobalLocalAS` vs effective `LocalAS` is a per-callsite choice.** OTC (R008) wants the
  global local AS; LLGR iBGP detection wants the peer's effective AS under a per-peer override.
  This spec does not unify them into one field; see the PROVISIONAL sub-choice in Key Design
  Decision. A per-peer local-as override interacting with OTC is out of scope here and left
  for a follow-up once an override peer is exercised.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING, before ANY commit)
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
