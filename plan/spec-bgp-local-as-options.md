# Spec: bgp-local-as-options -- `no-prepend` and `replace-as` must select different behaviour

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; nothing deferred; RFC 7705 Section 4.2 was never in scope, it is a sibling spec. Create `plan/deferrals/bgp-local-as-options.md` on the first deferral) |
| Updated | 2026-07-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The `local-options` leaf-list offers two enums, `no-prepend` and `replace-as`
(`internal/component/bgp/yang/ze-bgp-conf.yang`), documented as distinct
behaviours and explicitly composable: the leaf description at
`internal/component/bgp/yang/ze-bgp-conf.yang` says "Both together: full
replacement with no prepend", which only means something if each does something
different on its own.

They do not. Both parse into separate fields at
`internal/component/bgp/reactor/config.go` and
`internal/component/bgp/reactor/config.go`, and then the one site that reads
either of them treats them as the same flag:

```
if s.GlobalLocalAS != 0 && s.GlobalLocalAS != s.LocalAS &&
    !s.LocalASNoPrepend && !s.LocalASReplaceAS {
    facts.secondaryAS = s.GlobalLocalAS
}
```

That is `internal/component/bgp/reactor/peer_forward_facts.go`. Either flag
alone clears `facts.secondaryAS`, and `secondaryAS` is the only thing that picks
the dual-ASN encoder over the single-ASN one at
`internal/component/bgp/reactor/reactor_api_forward.go` and again at
`internal/component/bgp/reactor/reactor_api_forward.go`. So a peer configured
`no-prepend`, a peer configured `replace-as`, and a peer configured with both
receive byte-identical AS_PATHs. Three documented configurations, one behaviour.

RFC 7705 Section 3.3 defines them as two mechanisms acting in two different
directions:

| Mechanism | RFC 7705 obligation | Direction |
|-----------|---------------------|-----------|
| No Prepend Inbound | `RFC7705-3.3-2` MUST NOT append the Local AS when installing an inbound route or advertising it to iBGP; `RFC7705-3.3-3` MUST still append the globally configured ASN toward other local eBGP neighbours | inbound |
| Replace Old AS | `RFC7705-3.3-4` MUST NOT append the globally configured ASN outbound to the configured peer; `RFC7705-3.3-5` MUST append only the Local AS | outbound |

Ze's single implemented behaviour, suppressing the global ASN on the outbound
prepend toward the configured peer, is **Replace Old AS**. So `replace-as` is
correct and `no-prepend` is a second, differently-named copy of it.

**Goal.** Make the two options select different behaviour, or establish that one
of them cannot meaningfully exist in Ze and remove or reject it. Either outcome
ends with the three documented configurations being distinguishable on the wire
or the configuration surface no longer claiming they are.

**Secondary goal.** There is no `.ci` anywhere that drives local-as onto the wire.
`grep -rln "local-options" test/` finds only a parse-level check at
`test/parse/session-policy-config.ci` and an interop scenario directory. A feature
that changes the AS_PATH a peer sees has no end-to-end test, which is why three
configurations could collapse into one without anything failing.

This spec also owns the `RFC7705-3.3-1` through `RFC7705-3.3-5` tagged tests.
Enrolment of RFC 7705 as a whole belongs to `plan/spec-bgp-as-migration.md`,
because `rfc/enrolled.txt` admits an RFC only when every gated MUST is classified
and the Section 4.2 four are that spec's.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/config.md` - YANG versus environment variables, and what a config leaf owes
  → Constraint: a leaf that documents a behaviour owes that behaviour. Two enums that produce identical output are a configuration surface making a promise the engine does not keep.
- [ ] `ai/rules/config.md` - naming for config leaves
  → Constraint: the enum names are vendor-familiar, so renaming is not free. Whatever this spec decides, `no-prepend` and `replace-as` keep meanings an operator migrating from another daemon would recognise.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: AS_PATH is a segment sequence; a prepend inserts into or creates the leading AS_SEQUENCE. Whether one ASN or two are prepended is a wire-visible difference, so both options are testable at the byte level.
- [ ] `ai/rules/evidence.md` - a guard must fail closed or say something
  → Constraint: if the decision is that one option cannot be implemented in Ze, it must be rejected at config-parse time with a message naming the leaf, not silently accepted and ignored. Silently accepting a no-op is the current failure.
- [ ] `ai/rules/rfc-compliance.md` - when a compliance decision needs the owner
  → Decision: choosing to leave a MUST unimplemented, or classifying it `{gap}` / `{not-applicable}`, is Thomas's call. Making Ze more conformant needs no permission. This spec's open axis A-1 is therefore a question to ask, not a choice to make.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7705.txt` - AS migration mechanisms and their effect on AS_PATH. The implementation summary is written but parked at `rfc/pending/rfc7705.md`, out of `rfc/short/` until `plan/spec-bgp-as-migration.md` enrols it; read the source text meanwhile.
  → Constraint: Section 3.3 separates "No Prepend Inbound" (inbound, `RFC7705-3.3-2` and `RFC7705-3.3-3`) from "Replace Old AS" (outbound, `RFC7705-3.3-4` and `RFC7705-3.3-5`). They are not two spellings of one thing.
  → Constraint: `RFC7705-3.3-1` requires both mechanisms to be configurable per neighbour or per neighbour group, which the `session` container already satisfies by inheritance.
- [ ] `rfc/short/rfc4271.md` - UPDATE format, eBGP AS_PATH prepend
  → Constraint: Section 9.1.2 requires prepending the local AS when propagating to an eBGP peer. Whatever "local AS" means under a migration option, something must be prepended: neither option may produce an unprepended eBGP advertisement.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN
  → Constraint: an ASN above 65535 is encoded as AS_TRANS toward an old speaker, so a test asserting which ASNs were prepended must read AS4_PATH as well as AS_PATH when the peer is two-octet.

**Key insights:** (minimal context to resume after compaction)
- One line decides everything: the compound condition at `internal/component/bgp/reactor/peer_forward_facts.go` reads both flags with the same polarity and the same effect.
- `secondaryAS` is a single field with no third state, so today the encoder choice at `internal/component/bgp/reactor/reactor_api_forward.go` is binary. Distinguishing the two options needs either a second field or a value that carries direction.
- Ze never rewrites an inbound AS_PATH. The prepend happens only on the egress path, gated on the destination being eBGP. So the conformant reading of "No Prepend Inbound" may be that it is already unconditionally true and the option has nothing to select. That is the open axis, not a conclusion.
- There is no wire-level test for any of this. Both new `.ci` files are the point of the spec as much as the code change.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the `local-options` leaf-list, with `enum no-prepend` at `internal/component/bgp/yang/ze-bgp-conf.yang` described as "Do not prepend real ASN before local-as in AS_PATH" and `enum replace-as` at `internal/component/bgp/yang/ze-bgp-conf.yang` described as "Replace real ASN entirely with local-as in AS_PATH". The leaf-list `description "Modifiers for local-as behavior.` at `internal/component/bgp/yang/ze-bgp-conf.yang` documents a third combined state.
- [ ] `internal/component/bgp/reactor/config.go` - `peerLocalAS := localAS`: the per-peer local AS starts at the global value and is overridden from `session > asn > local`.
- [ ] `internal/component/bgp/reactor/config.go` - the `local-options` leaf-list read, with `case "no-prepend"` at `internal/component/bgp/reactor/config.go` and `case "replace-as"` at `internal/component/bgp/reactor/config.go` setting two separate booleans.
- [ ] `internal/component/bgp/reactor/config.go` - `ps := NewPeerSettings(ip, peerLocalAS, peerAS, peerRouterID)`, then `ps.GlobalLocalAS = localAS` at `internal/component/bgp/reactor/config.go` preserving the router's global ASN separately from the per-peer override.
- [ ] `internal/component/bgp/reactor/config.go` - `ps.LocalASNoPrepend = localASNoPrepend`, followed by `ps.LocalASReplaceAS = localASReplaceAS` at `internal/component/bgp/reactor/config.go`: the two flags reach the settings distinctly.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - the compound `s.GlobalLocalAS != s.LocalAS` guard: `secondaryAS` is filled only when a per-peer override exists **and neither flag is set**. This is the single site that consumes either flag.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `facts.secondaryAS = s.GlobalLocalAS`: the only assignment to the field.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - the precomputed `s.IsEBGP()` fact that gates whether any prepend happens at all.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the encoder choice on the per-destination rail, branching on `secondaryAS != 0`: `RewriteASPathDual` at `internal/component/bgp/reactor/reactor_api_forward.go`, `RewriteASPath` at `internal/component/bgp/reactor/reactor_api_forward.go` otherwise.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the same `secondaryAS != 0` branch repeated on the second rail, with `RewriteASPathDual` at `internal/component/bgp/reactor/reactor_api_forward.go`.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - `RewriteASPathDual`: prepends two ASNs, ordered so the primary ends up outermost, closest to the peer. Its doc block names RFC 7705 as the reference.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - `RewriteASPath`: the single-ASN entry point taken when `secondaryAS` is zero.
- [ ] `internal/component/bgp/reactor/peer_forward_facts_test.go` - the existing unit coverage sets `LocalASNoPrepend` and `LocalASReplaceAS` from a table, so a test that distinguishes them has a place to live.
- [ ] `internal/component/bgp/reactor/config_test.go` - asserts both flags false by default, and `internal/component/bgp/reactor/config_test.go` asserts both true when both enums are configured. Neither asserts a single option in isolation, which is exactly the case that would have caught this.
- [ ] `test/parse/session-policy-config.ci` - the only functional coverage naming `local-options`, and it is parse-level: it never brings a session up or reads an AS_PATH off the wire.

**Behavior to preserve:**
- A peer with `local-as` configured and no modifiers keeps today's dual prepend, with the override ASN outermost and the global ASN behind it, matching RFC 7705 Section 3.2's worked example.
- `replace-as` keeps today's behaviour: only the Local AS is prepended toward that peer.
- The per-neighbour and per-neighbour-group inheritance of the `session` container, which is what satisfies `RFC7705-3.3-1`.
- A peer with no `local-as` override at all is untouched: the `s.GlobalLocalAS != s.LocalAS` guard at `internal/component/bgp/reactor/peer_forward_facts.go` is false regardless of the flags.
- Every existing expectation under `test/parse/`, `test/plugin/` and `test/policy/`.
- The AS_TRANS handling for a four-octet local AS toward a two-octet peer.

**Behavior to change:**
- `no-prepend` stops selecting the outbound global-ASN suppression, which is `replace-as`'s meaning.
- What `no-prepend` selects instead is A-1, the open axis below. It is a compliance decision and needs Thomas.
- The configuration surface stops documenting a distinction it does not implement, whichever way A-1 resolves.
- `RFC7705-3.3-1` through `RFC7705-3.3-5` gain tagged tests, so `ai/RFC-REQUIREMENTS.md` records evidence rather than intent.

## Data Flow (MANDATORY)

### Entry Point
- Configuration: `session > asn > local` and the `asnMap["local-options"]` leaf-list read at `internal/component/bgp/reactor/config.go`, inherited group to peer.
- A received UPDATE being forwarded to the configured eBGP peer, which is where the prepend is applied.

### Transformation Path
1. Config parse fills `LocalAS`, `GlobalLocalAS` and the two option flags on the peer settings, from `NewPeerSettings` at `internal/component/bgp/reactor/config.go` onward.
2. Forward facts are precomputed once per settings change; the `s.GlobalLocalAS != s.LocalAS` guard at `internal/component/bgp/reactor/peer_forward_facts.go` decides whether `secondaryAS` is set.
3. The destination loop asks for the eBGP wire, and the encoder choice at `internal/component/bgp/reactor/reactor_api_forward.go` reads only `secondaryAS`.
4. `RewriteASPathDual` or `RewriteASPath` writes the prepended AS_PATH.
5. **Proposed:** step 2 produces a three-valued outcome rather than a boolean, so step 3 can distinguish the two options. The shape of that value depends on A-1.
6. The peer receives the UPDATE; a `.ci` reads the AS_PATH back off the wire and asserts which ASNs were prepended.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG config to peer settings | two enums to two booleans, `internal/component/bgp/reactor/config.go` | No |
| Peer settings to forward facts | the guard at `internal/component/bgp/reactor/peer_forward_facts.go` collapses both to one field | No |
| Forward facts to AS_PATH encoder | `secondaryAS` non-zero selects the dual encoder, `internal/component/bgp/reactor/reactor_api_forward.go` | No |
| Engine to peer TCP | the prepended AS_PATH is what the peer stores and re-advertises | No |

### Integration Points
- `PeerSettings.LocalASNoPrepend` and `PeerSettings.LocalASReplaceAS` keep their names, so config parsing is unchanged whatever A-1 decides.
- `peerForwardFacts` is the precompute boundary; anything the encoder needs must land there, not be re-derived per destination.
- `wireu.RewriteASPathDual` and `wireu.RewriteASPath` are unchanged: this spec changes which is called, not what either does.
- The parked summary at `rfc/pending/rfc7705.md` supplies the requirement IDs the new tests will tag; it returns to `rfc/short/` when `plan/spec-bgp-as-migration.md` enrols the RFC.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | **OPEN AXIS, needs Thomas.** Ze never rewrites an inbound AS_PATH, so RFC 7705's "No Prepend Inbound" may already be unconditionally true, leaving the `no-prepend` enum nothing to select. | The prepend runs only on the egress path, gated on `isEBGP` (`internal/component/bgp/reactor/peer_forward_facts.go`); no ingress site writes AS_PATH. | The three candidate resolutions are in the table below. Choosing any of them is a compliance decision (`ai/rules/rfc-compliance.md`), so this is a question to ask before implementing, not a choice to make. | Thomas's ruling, recorded in this spec before implementation starts. | unvalidated |
| A-2 | No ingress path modifies AS_PATH, so `RFC7705-3.3-2` holds vacuously today. | The AS-path rewrite family is reached only from the forward and announce rails. | If an ingress rewrite exists, `no-prepend` has a real inbound meaning and A-1 resolves itself. | Tree-wide grep for `RewriteASPath`, `RewriteASPathDual` and `TranscodeASPath` call sites, classifying each as ingress or egress, as the first implementation action. | unvalidated |
| A-3 | `secondaryAS` is the only channel by which either flag reaches the wire. | `internal/component/bgp/reactor/peer_forward_facts.go` is the field's only assignment, and `internal/component/bgp/reactor/reactor_api_forward.go` and `internal/component/bgp/reactor/reactor_api_forward.go` are its only readers on the prepend path. | A second channel means a wider change and possibly a second divergence. | Grep for `secondaryAS`, `LocalASNoPrepend` and `LocalASReplaceAS` across the tree. | unvalidated |
| A-4 | No operator configuration in the wild depends on `no-prepend` meaning what it currently does. | The two enums are indistinguishable today, so any config relying on the current behaviour would work identically under `replace-as`. | The change is still wire-visible for anyone who configured `no-prepend` alone and expected today's output. Release-note it. | A note in the change description; there is no telemetry to consult. | unvalidated |
| A-5 | Both mechanisms are already per-neighbour and per-neighbour-group configurable, satisfying `RFC7705-3.3-1` without code change. | The `session` container is group-to-peer inherited (`internal/component/bgp/yang/ze-bgp-conf.yang`), and the existing `peers[0].LocalASNoPrepend` coverage at `internal/component/bgp/reactor/config_test.go` exercises it. | The requirement needs implementation, not just a tag. | Tagging the existing config tests and confirming the gate accepts them. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing what `no-prepend` does is wire-visible for any peer configured with it, and AS_PATH changes affect loop detection and path selection at the far end. | The new `.ci` files, which pin the exact ASN sequence per configuration. | Land the `.ci` files first, against current behaviour, so the change shows as an explicit expectation edit rather than an invisible drift. |
| R-2 | Implementing before A-1 is answered means implementing a compliance decision nobody took. | This spec sitting at `design` with A-1 `unvalidated`. | A-1 blocks implementation. The spec does not move to `ready` until Thomas rules. |
| R-3 | Removing or rejecting an enum is a breaking config change for anyone using it. | Config parse failing on a previously accepted file. | If A-1 resolves to rejection, it must be a named parse error naming the leaf and the alternative, never a silent ignore. |
| R-4 | A test that asserts "two ASNs were prepended" passes for the wrong reason if the peer is two-octet and the real values hide in AS4_PATH. | A case with a four-octet local AS against a two-octet peer. | Every wire assertion reads AS_PATH and AS4_PATH together, and the boundary table below carries the four-octet case. |
| R-5 | Tagging the five Section 3.3 tests enrols nothing on its own, so the tree stays red on `ze-rfc-check` until `plan/spec-bgp-as-migration.md` closes. | `make ze-rfc-check` still failing after this spec lands. | Stated up front. This spec makes the evidence exist; the other spec makes the ledger admit it. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The AS_PATH Ze advertises to an eBGP peer is wrong: too few ASNs breaks loop detection at the far end and can attract traffic that should have been rejected, too many shifts path selection against us. During an AS migration this is the exact attribute the migration depends on. |
| How is it reverted? | Single commit revert. The change is confined to the guard and the encoder choice; no persistent state carries it. Once a peer has accepted and re-advertised a wrong AS_PATH the effect propagates beyond us. |
| Who else touches this path? | `plan/spec-bgp-as-migration.md` owns the rest of RFC 7705 and the enrolment; the AS_PATH encoders moved under a resolver and that work has LANDED with the wire-edit-3 AS_PATH fold and must preserve whatever this spec decides. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator configures `local-as` with `replace-as` and the peer receives a route | → | `secondaryAS` stays zero, single-ASN encoder runs | `test/plugin/bgp-local-as-replace-as.ci` |
| An operator configures `local-as` with `no-prepend` and the peer receives a route | → | the resolution of A-1 selects its own encoder path | `test/plugin/bgp-local-as-no-prepend.ci` |
| An operator configures `local-as` with no modifiers and the peer receives a route | → | dual-ASN encoder, override outermost | `test/plugin/bgp-local-as-dual.ci` |
| An operator configures both enums together | → | the combined state the `description "Modifiers for local-as behavior.` text at `internal/component/bgp/yang/ze-bgp-conf.yang` documents | `test/plugin/bgp-local-as-replace-as.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer with `local-as` and `replace-as`, receiving a route | Only the Local AS is prepended toward that peer, satisfying `RFC7705-3.3-4` and `RFC7705-3.3-5`, and a tagged test proves both polarities |
| AC-2 | A peer with `local-as` and `no-prepend`, receiving a route | The emitted AS_PATH differs observably from the `replace-as` case, or the configuration is rejected at parse time with a message naming the leaf. Which one is A-1 |
| AC-3 | A peer with `local-as` and no modifiers | The AS_PATH carries the override ASN outermost and the global ASN immediately behind it, unchanged from today |
| AC-4 | A peer with no `local-as` override | Behaviour is byte-identical to today regardless of the option flags |
| AC-5 | Any two of the three documented configurations | No two produce identical wire output, unless A-1 resolved to removing one of them |
| AC-6 | A four-octet local AS toward a two-octet peer, under each option | AS_PATH carries AS_TRANS and AS4_PATH carries the real values, consistently with the option in force |
| AC-7 | The `session` container set at group level with a peer-level override | Both mechanisms remain per-neighbour and per-neighbour-group configurable, satisfying `RFC7705-3.3-1` |
| AC-8 | An inbound UPDATE from the configured eBGP peer | The received AS_PATH is not modified, satisfying `RFC7705-3.3-2`, and a tagged test proves it |
| AC-9 | A route learned from the configured peer and re-advertised to a different eBGP peer | The globally configured ASN is appended as normal, satisfying `RFC7705-3.3-3` |
| AC-10 | `ai/RFC-REQUIREMENTS.md` after this spec | `RFC7705-3.3-1` through `RFC7705-3.3-5` each name an enforcing test at `file:line` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Migrates a router into a new AS and configures `local-as` with `replace-as` on a customer session | config, forward facts, single-ASN encoder, TCP | `test/plugin/bgp-local-as-replace-as.ci` |
| 2 | Configures `local-as` with `no-prepend` expecting the documented distinct behaviour | config, forward facts, the A-1 resolution, TCP or a named parse error | `test/plugin/bgp-local-as-no-prepend.ci` |
| 3 | Configures `local-as` alone during a migration and expects both ASNs visible to the peer | config, forward facts, dual-ASN encoder, TCP | `test/plugin/bgp-local-as-dual.ci` |
| 4 | Receives routes from the migrated peer and re-advertises them to a second eBGP neighbour | receive, no inbound AS_PATH rewrite, egress prepend with the global ASN | `test/plugin/bgp-local-as-inbound-untouched.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocalASOptionsAreDistinct` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-5: `no-prepend` alone and `replace-as` alone produce different forward facts | |
| `TestReplaceASSuppressesGlobalASN` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-1, tagged `RFC requirement: RFC7705-3.3-4` and `RFC7705-3.3-5`, both polarities | |
| `TestNoPrependSelectsItsOwnBehaviour` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-2, shape depends on the A-1 ruling | |
| `TestLocalASDualPrependOrder` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-3: override outermost, global behind it | |
| `TestNoLocalASOverrideUnaffected` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-4: the guard stays false when `GlobalLocalAS == LocalAS` | |
| `TestLocalASOptionsPerNeighborGroup` | `internal/component/bgp/reactor/config_test.go` | AC-7, tagged `RFC requirement: RFC7705-3.3-1`, both polarities | |
| `TestInboundASPathUnmodified` | `internal/component/bgp/reactor/session_validation_test.go` | AC-8, tagged `RFC requirement: RFC7705-3.3-2`, both polarities | |
| `TestGlobalASNAppendedToOtherEBGPPeers` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-9, tagged `RFC requirement: RFC7705-3.3-3`, both polarities | |
| `TestLocalASFourOctetASTrans` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-6, R-4: AS_TRANS and AS4_PATH stay consistent under each option | |
| `TestLocalASOptionRejectedIfUnsupported` | `internal/component/bgp/reactor/config_test.go` | AC-2 rejection branch, only if A-1 resolves that way | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| local AS | 1-4294967295 | 4294967295 | 0 (config rejected) | N/A (uint32 domain) |
| mappable ASN toward a two-octet peer | 1-65535 | 65535 | 0 | 65536 (AS_TRANS, real value in AS4_PATH) |
| prepended ASN count | 1-2 | 2 (no modifiers) | 0 (never legal on an eBGP advertisement) | 3 (no configuration produces it) |
| `local-options` enum count | 0-2 | 2 (both together) | N/A | 3 (no third enum exists) |
| AS_PATH segment ASN count after prepend | 1-255 | 255 | N/A | 256 (a new segment is required) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-local-as-dual` | `test/plugin/bgp-local-as-dual.ci` | `local-as` alone: the peer sees the override ASN outermost and the global ASN behind it | |
| `bgp-local-as-replace-as` | `test/plugin/bgp-local-as-replace-as.ci` | `replace-as`: the peer sees only the Local AS, and the both-enums case is pinned here too | |
| `bgp-local-as-no-prepend` | `test/plugin/bgp-local-as-no-prepend.ci` | `no-prepend`: the peer sees whatever A-1 rules, and it is not the same as `replace-as` | |
| `bgp-local-as-inbound-untouched` | `test/plugin/bgp-local-as-inbound-untouched.ci` | a route learned from the migrated peer keeps its received AS_PATH and gains the global ASN toward a second eBGP peer | |
| `session-policy-config` | existing `test/parse/session-policy-config.ci` | the parse-level coverage keeps passing | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-local-as-replace-as-bird` | `test/interop/scenarios/` | BIRD | a real peer accepts the Local-AS-only path and installs the expected AS_PATH | |
| `NN-local-as-dual-frr` | `test/interop/scenarios/` | FRR | a real peer sees both ASNs in the documented order during a migration | |

## Files to Modify
- `internal/component/bgp/reactor/peer_forward_facts.go` - the `s.GlobalLocalAS != s.LocalAS` guard at `internal/component/bgp/reactor/peer_forward_facts.go` stops collapsing the two flags; the facts carry enough to distinguish them
- `internal/component/bgp/reactor/reactor_api_forward.go` - the encoder choice reads the distinguished fact rather than a single boolean field
- `internal/component/bgp/reactor/config.go` - only if A-1 resolves to rejecting an option, in which case the parse names the leaf
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the enum descriptions say what each option actually does, in RFC 7705's vocabulary
- `docs/guide/configuration.md` - the local-as section documents the two options as distinct
- `docs/features/rfc-status.md` - the RFC 7705 row reflects what Section 3.3 now proves

## Files to Create
- `test/plugin/bgp-local-as-dual.ci` - both ASNs, documented order
- `test/plugin/bgp-local-as-replace-as.ci` - Local AS only, plus the both-enums case
- `test/plugin/bgp-local-as-no-prepend.ci` - the A-1 behaviour, distinct from `replace-as`
- `test/plugin/bgp-local-as-inbound-untouched.ci` - inbound AS_PATH preserved, global ASN toward a second eBGP peer

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: the enum descriptions change, and an enum may be removed depending on A-1 |
| YANG validation constraints | Yes | The enumeration is already the constraint; if A-1 removes an option the enum shrinks |
| YANG custom validators | No | Native enumeration is sufficient |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | Yes | Automatic for the enum, but the completion text follows the description change |
| Functional test for new RPC/API | Yes | Four new `.ci` files listed above |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | No | No new observable state; the AS_PATH is the observable |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | An existing option changes meaning |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the local-as options |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | The local-as section of `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | The AS_PATH encoding is unchanged; which ASNs go into it changes |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` RFC 7705 row, with source anchors for Section 3.3 |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if the local-as semantics are compared against other daemons |
| 12 | Internal architecture changed? | No | One guard and one encoder choice |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `peer_forward_facts.go` and `ze-bgp-conf.yang` and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any `local-options` example in `docs/` must match the new meanings |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- pin today's wire output before changing it
   - Tests: the four `.ci` files, written against **current** behaviour so `bgp-local-as-no-prepend.ci` and `bgp-local-as-replace-as.ci` initially record the same AS_PATH
   - Files: `test/plugin/bgp-local-as-dual.ci`, `test/plugin/bgp-local-as-replace-as.ci`, `test/plugin/bgp-local-as-no-prepend.ci`, `test/plugin/bgp-local-as-inbound-untouched.ci`
   - Verify: the collapse is now visible as two `.ci` files with identical expectations, and any later change shows as an explicit expectation edit
2. **Phase: resolve A-1** -- BLOCKING, no code before this
   - Tests: none; this is a decision
   - Files: this spec's A-1 row and Key Design Decisions table
   - Verify: Thomas has ruled, the ruling is recorded here with its rationale, and the spec moves to `ready`
3. **Phase: distinguish the two options**
   - Tests: `TestLocalASOptionsAreDistinct`, `TestReplaceASSuppressesGlobalASN`, `TestNoPrependSelectsItsOwnBehaviour`, `TestLocalASDualPrependOrder`, `TestNoLocalASOverrideUnaffected`
   - Files: `internal/component/bgp/reactor/peer_forward_facts.go`, `internal/component/bgp/reactor/reactor_api_forward.go`, and `internal/component/bgp/reactor/config.go` if A-1 requires rejection
   - Verify: AC-1 through AC-5 pass; A-3 resolved by grep
4. **Phase: the four-octet and multi-peer cases**
   - Tests: `TestLocalASFourOctetASTrans`, `TestGlobalASNAppendedToOtherEBGPPeers`, `TestInboundASPathUnmodified`
   - Files: no new production files expected; this phase proves the existing paths
   - Verify: AC-6, AC-8, AC-9 pass; A-2 resolved by the ingress-versus-egress grep
5. **Phase: tag the Section 3.3 requirements**
   - Tests: every test named above gains its `RFC requirement: RFC7705-3.3-N <polarity>` tag
   - Files: `internal/component/bgp/reactor/peer_forward_facts_test.go`, `internal/component/bgp/reactor/config_test.go`, `internal/component/bgp/reactor/session_validation_test.go`
   - Verify: AC-10 passes; `make ze-rfc-index` renders the five rows with enforcing tests. `make ze-rfc-check` still fails on enrolment, which `plan/spec-bgp-as-migration.md` closes
6. **Phase: configuration surface and documentation**
   - Tests: existing `test/parse/session-policy-config.ci`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `docs/guide/configuration.md`, `docs/features/rfc-status.md`
   - Verify: the enum descriptions match the implemented behaviour; every Documentation row marked Yes is done with source anchors
7. **Phase: interop**
   - Tests: the two scenarios above
   - Files: `test/interop/scenarios/`
   - Verify: a real peer installs the AS_PATH each option promises

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and each of `RFC7705-3.3-1` through `RFC7705-3.3-5` names a tagged test |
| Feature completeness | Every user story has a passing `.ci`, including the inbound story |
| Correctness | No two documented configurations produce identical wire output; the dual-prepend order is unchanged; a peer with no override is untouched |
| Naming | The enum names stay recognisable to an operator migrating from another daemon, and the descriptions use RFC 7705's vocabulary |
| Data flow | The distinction is computed once into the forward facts, never re-derived per destination |
| Registration over hardcoding | No per-option branch is added to a core package: the option resolves to a value the existing encoder choice already consumes |
| Rule: `ai/rules/evidence.md` | If an option is unsupported it is rejected by name at parse time, never silently ignored |
| Rule: `ai/rules/rfc-compliance.md` | A-1 was ruled by Thomas before any code was written, and the ruling is recorded here |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The two options are distinguishable | `go test ./internal/component/bgp/reactor/ -run TestLocalASOptionsAreDistinct` |
| Wire-level coverage exists | `ls test/plugin/bgp-local-as-*.ci` returns four files |
| The collapse is gone | `grep -n "!s.LocalASNoPrepend && !s.LocalASReplaceAS" internal/component/bgp/reactor/peer_forward_facts.go` returns nothing |
| Section 3.3 is evidenced | `grep -c "RFC7705-3.3" ai/RFC-REQUIREMENTS.md` shows all five with enforcing tests |
| Ledger regenerated in the same commit | `make ze-rfc-index` then `git diff --stat ai/RFC-REQUIREMENTS.md` |
| No unrelated regressions | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Loop prevention | The prepend is what makes eBGP loop detection work. Any option that results in zero ASNs prepended on an eBGP advertisement is a routing-loop risk, not a configuration preference |
| Path manipulation | Suppressing the global ASN shortens the path we advertise, which attracts traffic. That is the point of the feature, but it must happen only where configured, never by default |
| Config validation | The enums come from operator input. An unsupported combination must be rejected with a message naming the leaf, not accepted and ignored |
| Blast containment | The option is per neighbour. Verify no path lets a group-level setting reach a peer that did not inherit it |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| A-1 is not yet ruled | STOP. Implementation is blocked on the ruling, by design |
| Two configurations still produce identical output | STOP. That is the defect this spec exists to remove |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop peer rejects the AS_PATH | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| An ingress AS_PATH rewrite is found during the A-2 grep | Report it. It changes A-1's premise and the spec must be re-designed around it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The defect is one compound condition. The `s.GlobalLocalAS != s.LocalAS` guard at `internal/component/bgp/reactor/peer_forward_facts.go` continues onto the next line with `!s.LocalASNoPrepend && !s.LocalASReplaceAS`, reading two independent operator intentions with one polarity and one effect. That is how two documented behaviours became one.
- The configuration surface is the thing that made this discoverable and the thing that made it invisible: the YANG description at `internal/component/bgp/yang/ze-bgp-conf.yang` promises a third combined state, and nothing tested that any of the three differed.
- The existing unit tests at `internal/component/bgp/reactor/config_test.go` and `internal/component/bgp/reactor/config_test.go` assert both flags false and both flags true. Neither exercises one alone, which is exactly the pair of cases that distinguishes the options. A table test that only walks the corners misses the edges.
- RFC 7705 separates the two mechanisms by direction, not by degree. That is the framing the enum descriptions should adopt, because "no prepend" and "replace" sound like two strengths of one knob and are not.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Treat this as a behaviour defect, not a documentation defect | reword the YANG so the two enums are documented as synonyms | Two names for one behaviour is a configuration surface that lies about its own granularity, and RFC 7705 defines them as different mechanisms |
| A-1 is a question, not a choice this spec makes | pick the reading that looks most conformant and implement it | `ai/rules/rfc-compliance.md` reserves any decision that lowers what Ze owes for Thomas, and one candidate resolution removes an option |
| Write the `.ci` files against current behaviour first | write them against the intended behaviour so they fail | Recording the collapse explicitly is what makes the later change reviewable as an expectation edit rather than a silent wire change |
| Enrolment lives in the sibling spec | enrol here with the Section 4.2 four classified `{gap}` | Enrolment admits an RFC only when every gated MUST is classified, and classifying the Section 4.2 four is the other spec's decision |

## Known Limitations

- This spec does not implement RFC 7705 Section 4.2. That is `plan/spec-bgp-as-migration.md`.
- `make ze-rfc-check` stays red until that sibling closes, because tagging tests does not enrol an RFC.
- The `.ci` files cover IPv4 unicast. Other families take the same egress prepend path, so the coverage is representative rather than exhaustive.
- If A-1 resolves to removing an enum, operators using it need a release note; there is no config migration machinery for a removed enum value.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site |
|-----|---------|-------------|------|
| 7705 | 3.3 | mechanisms MUST be configurable per neighbour or per neighbour group | `internal/component/bgp/reactor/config.go` |
| 7705 | 3.3 | "No Prepend Inbound" MUST NOT append the Local AS inbound or toward iBGP | the ingress path, per the A-1 ruling |
| 7705 | 3.3 | "No Prepend Inbound" MUST still append the globally configured ASN toward other local eBGP neighbours | `internal/component/bgp/reactor/peer_forward_facts.go` |
| 7705 | 3.3 | "Replace Old AS" MUST NOT append the globally configured ASN | `internal/component/bgp/reactor/peer_forward_facts.go` |
| 7705 | 3.3 | "Replace Old AS" MUST append only the configured Local AS | `internal/component/bgp/reactor/reactor_api_forward.go` |
| 4271 | 9.1.2 | prepend the local AS when propagating to an eBGP peer | `internal/component/bgp/wireu/aspath_rewrite.go` |
| 6793 | 4.2.2 | AS_TRANS toward an old speaker, real values in AS4_PATH | `internal/component/bgp/wireu/aspath_rewrite.go` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-bgp-local-as-options.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-bgp-local-as-options.md` only (commit A preserves the spec in history)
