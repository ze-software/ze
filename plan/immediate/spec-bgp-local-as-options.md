# Spec: bgp-local-as-options -- preserve the direction each option governs

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The original defect made `no-prepend` suppress the globally configured ASN on
outbound advertisements, just as `replace-as` did. The forward-path correction
and the later announce-path correction are present in the tree. This spec
retains their acceptance and outstanding functional, interop and closure proof.

RFC 7705 Section 3.3 gives the options different directions:

| Mechanism | RFC 7705 obligation | Direction |
|-----------|---------------------|-----------|
| No Prepend Inbound | `RFC7705-3.3-2` MUST NOT append the Local AS when installing an inbound route or advertising it to iBGP; `RFC7705-3.3-3` MUST still append the globally configured ASN toward other local eBGP neighbours | inbound |
| Replace Old AS | `RFC7705-3.3-4` MUST NOT append the globally configured ASN outbound to the configured peer; `RFC7705-3.3-5` MUST append only the Local AS | outbound |

The goal is this per-direction behaviour. Toward the configured peer, no option
and `no-prepend` produce the dual prepend, while `replace-as` and both options
produce the Local-AS-only prepend. Equal outbound bytes within either pair are
required. The options remain composable, and neither is removed or rejected.

Thomas deferred the inbound-append SHOULD on 2026-09-06 to the configuration
recipe in `plan/spec-bgp-local-as-inbound-append.md`. This spec does not silently
take that work back or claim that the engine performs the append. Its inbound
tests cover peers without that recipe.

This spec owns the `RFC7705-3.3-1` through `RFC7705-3.3-5` evidence.
`plan/immediate/spec-bgp-as-migration.md` enrolled RFC 7705 on 2026-09-14.
The summary and both-polarity tags now exist; a fresh passing gate and completion
of the remaining proof are still required before closure.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `ai/rules/config.md` - YANG versus environment variables, and what a config leaf owes
  → Constraint: descriptions state the direction each option governs and the permitted equal-output combinations in AC-5.
- [ ] `ai/rules/config.md` - naming for config leaves
  → Constraint: the enum names are vendor-familiar, so renaming is not free. Whatever this spec decides, `no-prepend` and `replace-as` keep meanings an operator migrating from another daemon would recognise.
- [ ] `docs/architecture/wire/attributes.md` - path attribute encoding
  → Constraint: AS_PATH is a segment sequence; a prepend inserts into or creates the leading AS_SEQUENCE. Whether one ASN or two are prepended is a wire-visible difference, so both options are testable at the byte level.
- [ ] `ai/rules/evidence.md` - functional and interop assertions must exercise the configured direction
  → Constraint: outbound equality between `replace-as` and both options is required; it cannot be used as evidence of a defect.
- [ ] `ai/rules/rfc-compliance.md` - owner decisions remain binding
  → Decision: the 2026-09-06 inbound-recipe deferral preserves both enums and leaves the SHOULD with its named owner.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7705.txt` - AS migration mechanisms and their effect on AS_PATH. The implementation summary is enrolled at `rfc/short/rfc7705.md` since 2026-09-14, and every Section 3.3 MUST it declares carries a tagged test in both polarities.
  → Constraint: Section 3.3 separates "No Prepend Inbound" (inbound, `RFC7705-3.3-2` and `RFC7705-3.3-3`) from "Replace Old AS" (outbound, `RFC7705-3.3-4` and `RFC7705-3.3-5`). They are not two spellings of one thing.
  → Constraint: `RFC7705-3.3-1` requires both mechanisms to be configurable per neighbour or per neighbour group, which the `session` container already satisfies by inheritance.
- [ ] `rfc/short/rfc4271.md` - UPDATE format, eBGP AS_PATH prepend
  → Constraint: Section 9.1.2 requires prepending the local AS when propagating to an eBGP peer. Whatever "local AS" means under a migration option, something must be prepended: neither option may produce an unprepended eBGP advertisement.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN
  → Constraint: an ASN above 65535 is encoded as AS_TRANS toward an old speaker, so a test asserting which ASNs were prepended must read AS4_PATH as well as AS_PATH when the peer is two-octet.

**Key insights:** (minimal context to resume after compaction)
- `secondaryPrependAS` reads `LocalASReplaceAS` and leaves `LocalASNoPrepend` out of outbound selection.
- Forwarding carries the result in `facts.secondaryAS`; announcement uses `localASPrependFor`, which reads the same helper.
- The engine has no automatic inbound Local-AS append. An explicit import policy can prepend; the deferred recipe and safeguards belong to `plan/spec-bgp-local-as-inbound-append.md`.
- `test/plugin/bgp-local-as-options.ci` and `test/plugin/bgp-local-as-inbound-untouched.ci` exist. The dated execution record below is not a current pass.

## Current Behavior (MANDATORY)

**Source and evidence:**
- `internal/component/bgp/reactor/peer_forward_facts.go`: `secondaryPrependAS` returns the global ASN only for a distinct local override without `replace-as`; `localASPrependFor` supplies the same pair to announcement.
- `internal/component/bgp/reactor/reactor_api_forward.go` and `forward_rs.go`: `ASPathEdit.Record` consumes the eBGP prepend intent; RS clients retain their AS_PATH transparency.
- `internal/component/bgp/reactor/reactor_api_batch.go`: `announceFacts.prepend` partitions peers and reaches the announce builder.
- `internal/component/bgp/reactor/rfc7705_local_as_test.go`: the directional tests carry both-polarity tags for Section 3.3 requirements 2 through 5, and assert both permitted equality and required inequality.
- `internal/component/bgp/config/peers_test.go`: `TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup` carries both-polarity tags for requirement 1.
- `docs/guide/configuration.md`: the local-AS section documents the directional table and the equal combined-option result.
- `rfc/short/rfc7705.md`: enrolment is recorded, with the inbound SHOULD assigned to the deferred recipe.

**Behavior to preserve:**
- A peer with `local-as` configured and no modifiers keeps today's dual prepend, with the override ASN outermost and the global ASN behind it, matching RFC 7705 Section 3.2's worked example.
- `replace-as` keeps today's behaviour: only the Local AS is prepended toward that peer.
- The per-neighbour and per-neighbour-group inheritance of the `session` container, which is what satisfies `RFC7705-3.3-1`.
- A peer with no `local-as` override at all is untouched: the `s.GlobalLocalAS != s.LocalAS` guard at `internal/component/bgp/reactor/peer_forward_facts.go` is false regardless of the flags.
- Every existing expectation under `test/parse/`, `test/plugin/` and `test/policy/`.
- The AS_TRANS handling for a four-octet local AS toward a two-octet peer.

**Remaining work:**
- Complete the outstanding functional and interop proof against AC-1 through AC-10, with the direction and permitted equalities explicit.
- Reconcile the source, schema and guide at closure without removing either enum or implementing the deferred inbound recipe here.
- Check the enrolled Section 3.3 evidence through the RFC gate; tag presence alone is not a new passing result.

## Data Flow (MANDATORY)

### Entry Point
- Configuration: `session > asn > local` and the `asnMap["local-options"]` leaf-list read at `internal/component/bgp/reactor/config.go`, inherited group to peer.
- A received UPDATE being forwarded to the configured eBGP peer, which is where the prepend is applied.

### Transformation Path
1. Config parse fills `LocalAS`, `GlobalLocalAS` and the two option flags on the peer settings, from `NewPeerSettings` at `internal/component/bgp/reactor/config.go` onward.
2. `secondaryPrependAS` computes the global-AS addition once into forward facts; `localASPrependFor` supplies the same answer to announcement.
3. Each forward rail records the resulting AS_PATH intent for its eBGP destination. The announce rail groups peers by `announceFacts`, including the prepend pair.
4. The live one-pass writer or announce builder emits the selected path. No three-valued option state is needed.
5. Inbound routes without an explicit import recipe receive no automatic Local-AS append.
6. The peer receives the UPDATE; a `.ci` reads the AS_PATH back off the wire and asserts which ASNs were prepended.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG config to peer settings | two enums to two booleans, `internal/component/bgp/reactor/config.go` | No |
| Peer settings to forward facts | `secondaryPrependAS` reads only the outbound `replace-as` option | Source inspected; runtime proof remains subject to the tests below |
| Forward facts to AS_PATH writer | `ASPathEdit.Record` takes the prepend intent; announcement takes `localASPrependFor` | Source inspected; runtime proof remains subject to the tests below |
| Engine to peer TCP | the prepended AS_PATH is what the peer stores and re-advertises | No |

### Integration Points
- `PeerSettings.LocalASNoPrepend` and `PeerSettings.LocalASReplaceAS` retain their names and independent direction semantics.
- `peerForwardFacts` is the precompute boundary; anything the encoder needs must land there, not be re-derived per destination.
- `wireu.ASPathEdit.Record` is the current forward writer. The retired whole-payload `RewriteASPath` and `RewriteASPathDual` functions are not implementation targets.
- The enrolled summary at `rfc/short/rfc7705.md` supplies the requirement IDs the tests tag; `plan/immediate/spec-bgp-as-migration.md` returned it from `rfc/pending/` and enrolled the RFC on 2026-09-14.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The option reaches the wire by the one path config already used: `parsePeerFromTree` fills `LocalASReplaceAS`, `secondaryPrependAS` folds it into `facts.secondaryAS` once per settings change, and the two forward rails read that field. No new field, no new call, and nothing re-derived per destination |
| No unintended coupling (components stay isolated) | Yes | `wireu` is unchanged apart from a comment: the fix is which ASNs `reactor` puts in `ASPathIntent.Prepend`, and `wireu` never learns that RFC 7705 exists |
| No duplicated functionality (extends existing, does not recreate) | Yes | `secondaryPrependAS` is the ONLY place either flag is read. `LocalASNoPrepend` is now read nowhere, which is correct and is stated on the field |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The prepend is recorded as intent on the accumulator and written once by the one-pass writer. No intermediate payload, and `prependBuf` is a stack array hoisted above the destination loop |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new enum, no new central case. The option resolves to a `uint32` the existing encoder choice already consumed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Both enums remain, and the missing inbound SHOULD is deferred to an operator recipe | Thomas's 2026-09-06 ruling in `plan/spec-bgp-local-as-inbound-append.md` | Reopening this choice requires an owner decision; it cannot be inferred from equal outbound bytes | The recorded ruling and the directional ACs here | resolved by owner, 2026-09-06 |
| A-2 | No automatic Local-AS append runs at ingress | The egress helpers implement the outbound mechanism; the explicit import-policy prepend is separately configured | Inbound tests must distinguish an automatic mechanism from the optional recipe | `TestLocalASNoPrependLeavesEveryOutboundPathAlone` and the inbound functional scenario | The 2026-09-05 source classification excluded the policy action from the Local-AS mechanism; current proof remains owed at closure |
| A-3 | Forward and announce use the same outbound prepend decision | `secondaryPrependAS` feeds `facts.secondaryAS` on both forward rails and `localASPrependFor` on announcement | A divergent rail can restore the original collapse | Compare all four combinations on each live rail | source inspected, 2026-09-19; current runtime proof remains owed |
| A-4 | The correction can affect configurations that relied on the former `no-prepend` outbound suppression | Before the correction either enum selected the single-ASN form | An operator may receive a longer AS_PATH after upgrade | Preserve the configuration-guide explanation and the directional wire tests | Historical change recorded on 2026-09-05; no claim about deployed operator configurations |
| A-5 | Both mechanisms are already per-neighbour and per-neighbour-group configurable, satisfying `RFC7705-3.3-1` without code change. | The `session` container is group-to-peer inherited (`internal/component/bgp/yang/ze-bgp-conf.yang`), and the existing `peers[0].LocalASNoPrepend` coverage at `internal/component/bgp/reactor/config_test.go` exercises it. | The requirement needs implementation, not just a tag. | `TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup` (`internal/component/bgp/config/peers_test.go`). | confirmed 2026-09-05: the group's `local-options` reaches an inheriting peer, a peer that states its own REPLACES the group's leaf-list rather than accumulating, and a peer outside the group carries neither. No code change was needed. |

### A-1 history: placement alternatives before the 2026-09-06 ruling

The following reasoning records the earlier design question. Thomas subsequently
chose the deferred import-policy recipe named in Task; neither alternative below
is an unresolved prerequisite of this spec.

Not "does `no-prepend` mean anything". It does. RFC 7705 Section 3.3 item 1
says the router "SHOULD append the configured 'Local AS' ASN in the AS_PATH
attribute before installing the route or advertising the UPDATE to an iBGP
neighbor", and `no-prepend` is the MUST NOT that turns that off. Ze has never
built item 1, so the option has nothing to suppress. Removing or rejecting the
enum would record an unbuilt mechanism as a decision, so it is off the table
(`ai/rules/rfc-compliance.md`).

The RFC declines to choose where the append happens: "The decision of when to
append the ASN is an implementation detail outside the scope of this document."
Both answers below are conformant, they differ in what the rest of the router
sees, and the choice is not this spec's to make.

| Option | Where the legacy AS is added | Consequence |
|--------|------------------------------|-------------|
| At RIB install | The stored route already carries the legacy AS, so every reader agrees | AS_PATH is consistent everywhere inside the AS, loop detection and best-path AS_PATH length both count the legacy AS, and `show bgp` shows what iBGP shows. It changes best-path selection: a route through the migrated session gets one AS longer and can lose to a route that used to lose to it |
| At iBGP advertisement | The stored route keeps what arrived; the legacy AS is added on the egress rail toward internal peers | Best-path is unchanged, so no migration re-converges the local RIB. The local RIB and an iBGP neighbor's RIB then disagree about the path, which is the inconsistency the RFC's own note about "consistency in the AS_PATH throughout the AS" warns about |

Ze's shape favors the second: `AdjRIBInManager` stores what it received and
`buildReplayRoutes` reproduces the egress transform from the source
(`internal/component/bgp/plugins/adj_rib_in/rib.go`), so an ingress rewrite
would be the first thing in the tree to modify a stored route. But the first
option is what makes loop detection count the legacy AS, and that is a safety
property rather than a preference.

Whichever is chosen, the work is a spec of its own: it adds an iBGP-facing
prepend to a rail that has never had one, and `RFC7705-3.3-2` and `-3.3-3` only
become both-polarity provable once it exists.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Changing what `no-prepend` does is wire-visible for any peer configured with it, and AS_PATH changes affect loop detection and path selection at the far end. | The new `.ci` files, which pin the exact ASN sequence per configuration. | Land the `.ci` files first, against current behaviour, so the change shows as an explicit expectation edit rather than an invisible drift. |
| R-2 | A later implementation reopens the inbound placement question and expands this spec | An automatic inbound append or enum removal appears in the change | Preserve the 2026-09-06 ruling and the separate recipe owner |
| R-3 | Documentation promises a distinct outbound result for every combination | AC-5 or the guide rejects the required equal-output pairs | Review the per-direction matrix, including both options together |
| R-4 | A test that asserts "two ASNs were prepended" passes for the wrong reason if the peer is two-octet and the real values hide in AS4_PATH. | A case with a four-octet local AS against a two-octet peer. | Every wire assertion reads AS_PATH and AS4_PATH together, and the boundary table below carries the four-octet case. |
| R-5 | Existing tags are mistaken for fresh proof | Closure cites enrolment alone as a passing gate | Enrolment was completed on 2026-09-14; closure still checks the current tagged tests and RFC gate |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The AS_PATH Ze advertises to an eBGP peer is wrong: too few ASNs breaks loop detection at the far end and can attract traffic that should have been rejected, too many shifts path selection against us. During an AS migration this is the exact attribute the migration depends on. |
| How is it reverted? | Single commit revert. The change is confined to the guard and the encoder choice; no persistent state carries it. Once a peer has accepted and re-advertised a wrong AS_PATH the effect propagates beyond us. |
| Who else touches this path? | `plan/immediate/spec-bgp-as-migration.md` owns the rest of RFC 7705 and the enrolment; the AS_PATH encoders moved under a resolver and that work has LANDED with the wire-edit-3 AS_PATH fold and must preserve whatever this spec decides. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator configures `local-as` with `replace-as` and the peer receives a route | → | `secondaryPrependAS` answers zero, one ASN is prepended | `test/plugin/bgp-local-as-options.ci`, conn=4 |
| An operator configures `local-as` with `no-prepend` and the peer receives a route | → | the inbound option does not reach the outbound rail, so the dual form stands | `test/plugin/bgp-local-as-options.ci`, conn=3 |
| An operator configures `local-as` with no modifiers and the peer receives a route | → | two ASNs prepended, override outermost | `test/plugin/bgp-local-as-options.ci`, conn=2 |
| An operator configures both enums together | → | the outbound result is `replace-as`, which is what the YANG description now says | `test/plugin/bgp-local-as-options.ci`, conn=5 |
| A route learned FROM the configured peer reaches an iBGP and a native eBGP neighbor | → | no inbound rewrite; the globally configured ASN toward the eBGP neighbor | `test/plugin/bgp-local-as-inbound-untouched.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer with `local-as` and `replace-as`, receiving a route | Only the Local AS is prepended toward that peer, satisfying `RFC7705-3.3-4` and `RFC7705-3.3-5`, and a tagged test proves both polarities |
| AC-2 | A peer with `local-as` and `no-prepend`, receiving a route | The dual prepend is retained: Local AS outermost, global ASN behind it. It differs from `replace-as` alone, and the configuration remains accepted |
| AC-3 | A peer with `local-as` and no modifiers | The AS_PATH carries the override ASN outermost and the global ASN immediately behind it, unchanged from today |
| AC-4 | A peer with no `local-as` override | Behaviour is byte-identical to today regardless of the option flags |
| AC-5 | All four option combinations, tested by direction and on both forward and announce rails | Outbound: no option equals `no-prepend` (dual prepend), and `replace-as` equals both options (Local AS only); the two groups differ. Inbound without the deferred recipe: the Local AS is not appended, and native eBGP readvertisement retains its normal global-AS prepend |
| AC-6 | A four-octet local AS toward a two-octet peer, under each option | AS_PATH carries AS_TRANS and AS4_PATH carries the real values, consistently with the option in force |
| AC-7 | The `session` container set at group level with a peer-level override | Both mechanisms remain per-neighbour and per-neighbour-group configurable, satisfying `RFC7705-3.3-1` |
| AC-8 | An inbound UPDATE from the configured eBGP peer with `no-prepend`, without an explicit import prepend recipe | The received AS_PATH is not modified by the Local-AS mechanism, satisfying `RFC7705-3.3-2`, and a tagged test proves it |
| AC-9 | A route learned from the configured peer and re-advertised to a different eBGP peer | The globally configured ASN is appended as normal, satisfying `RFC7705-3.3-3` |
| AC-10 | `ai/RFC-REQUIREMENTS.md` after this spec | `RFC7705-3.3-1` through `RFC7705-3.3-5` each name an enforcing test at `file:line` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Migrates a router and configures `local-as` with `replace-as` on a customer session | config, forward facts, single-ASN writer, TCP | `test/plugin/bgp-local-as-options.ci`, conn=4 and conn=5 |
| 2 | Configures `local-as` with `no-prepend` | config, forward facts, dual prepend toward the peer; unchanged inbound path | `test/plugin/bgp-local-as-options.ci`, conn=3; `test/plugin/bgp-local-as-inbound-untouched.ci` |
| 3 | Configures `local-as` alone and expects both ASNs visible to the peer | config, forward facts, dual-ASN writer, TCP | `test/plugin/bgp-local-as-options.ci`, conn=2 |
| 4 | Receives routes from the migrated peer and re-advertises them to a second eBGP neighbour | receive, no inbound AS_PATH rewrite, egress prepend with the global ASN | `test/plugin/bgp-local-as-inbound-untouched.ci` |

## 🧪 TDD Test Plan

### Unit Tests
The names below are the ones that EXIST. Where a test landed under a different
name or in a different file from the one this spec first proposed, the row says
so: the file it landed in is the one whose entry point the assertion reaches.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocalASOptionsProduceDifferentASPaths` | `internal/component/bgp/reactor/rfc7705_local_as_test.go` | AC-5 on the forward rail: one UPDATE, two peers differing by one enum, and the two AS_PATHs compared against each other rather than against a constant | green |
| `TestLocalASReplaceASSendsOnlyTheLocalAS` | `internal/component/bgp/reactor/rfc7705_local_as_test.go` | AC-1, AC-3, AC-5: `replace-as` and both options equal the Local-AS-only form; the no-option control retains two ASNs. Carries both-polarity tags for RFC7705-3.3-4 and -3.3-5 | tags inspected; no fresh run in this reconciliation |
| `TestLocalASNoPrependLeavesEveryOutboundPathAlone` | `internal/component/bgp/reactor/rfc7705_local_as_test.go` | AC-2, AC-4, AC-5, AC-8, AC-9: iBGP, native eBGP and local-AS outbound directions. Carries both-polarity tags for RFC7705-3.3-2 and -3.3-3 | tags inspected; no fresh run in this reconciliation |
| `TestLocalASFourOctetTowardTwoOctetPeer` | `internal/component/bgp/reactor/rfc7705_local_as_test.go` | AC-6, R-4. AS_TRANS in AS_PATH and the real values in AS4_PATH, per option, with a four-octet destination as the control that MUST receive no AS4_PATH | green |
| `TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup` | `internal/component/bgp/config/peers_test.go` | AC-7, RFC7705-3.3-1: group inheritance, peer override and outside-group control | both-polarity tags inspected; no fresh run in this reconciliation |
| `TestParsePeerFromTree_LocalASOptions`, `TestPeersFromTreePeerLocalASModifiers`, `TestPeersFromTreePeerLocalASNoOverride` | `internal/component/bgp/reactor/config_test.go` | the parse surface: absent, one option alone, both together, and no override | green, pre-existing |
| `TestLocalASOptionRejectedIfUnsupported` | - | NOT WRITTEN. It belonged to the A-1 arm that rejects an enum, and that arm is off the table: `no-prepend` names a real RFC mechanism ze has not built, so refusing it would refuse a conformant configuration | dropped, see A-1 |

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
| `bgp-local-as-options` | `test/plugin/bgp-local-as-options.ci` | all four outbound configurations in one run: no option, `no-prepend`, `replace-as`, both. conn=3 and conn=4 differ by one enum and by nine bytes | landed 43de0a5f8 |
| `bgp-local-as-inbound-untouched` | `test/plugin/bgp-local-as-inbound-untouched.ci` | a route learned from the migrated peer reaches an iBGP neighbor byte-identical and a native eBGP neighbor with the globally configured ASN prepended | written, execution deferred |
| `session-policy-config` | existing `test/parse/session-policy-config.ci` | the parse-level coverage keeps passing | untouched |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-local-as-replace-as-bird` | `test/interop/scenarios/` | BIRD | a real peer accepts the Local-AS-only path and installs the expected AS_PATH | |
| `NN-local-as-dual-frr` | `test/interop/scenarios/` | FRR | a real peer sees both ASNs in the documented order during a migration | |

## Files to Modify
- `internal/component/bgp/reactor/peer_forward_facts.go` - preserve `secondaryPrependAS` and `localASPrependFor` while completing proof
- `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go`, `reactor_api_batch.go` - verify the same directional contract on every live rail
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the enum descriptions say what each option actually does, in RFC 7705's vocabulary
- `docs/guide/configuration.md` - the local-as section documents the two options as distinct
- `docs/features/rfc-status.md` - the RFC 7705 row reflects what Section 3.3 now proves

## Files to Create
- `test/plugin/bgp-local-as-options.ci` - LANDED (commit 43de0a5f8). The four outbound configurations in ONE run: one UPDATE, four receivers whose config differs only in `local-options`, and four byte-level frames. This replaces the three separate files this spec first named, and it is stronger than they would have been: `no-prepend` and `replace-as` are compared against each other inside a single forward, so the collapse cannot come back as a shared expectation edit.
- `test/plugin/bgp-local-as-inbound-untouched.ci` - inbound AS_PATH preserved toward an iBGP neighbor, globally configured ASN toward a native eBGP neighbor. Written, NOT yet executed: `le-test` could not be rebuilt with its fixture because `internal/component/bgp/plugins/cmd/commit` did not compile in the working tree on 2026-09-05. The file names the command that runs it.
- `internal/test/fixture/plugin_fixture_04.go` - the `plugin/bgp-local-as-inbound-untouched` fixture registration, three peers.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: descriptions state the direction of each option; both enums remain |
| YANG validation constraints | No | The existing enumeration is unchanged |
| YANG custom validators | No | Native enumeration is sufficient |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | Yes | Automatic for the enum, but the completion text follows the description change |
| Functional test for new RPC/API | Yes | The two `.ci` files listed above |
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

1. **Phase: Wiring**: retain `test/plugin/bgp-local-as-options.ci` and `bgp-local-as-inbound-untouched.ci`; prove the configured directions from peer input to wire output.
2. **Phase: Owner boundary**: retain the 2026-09-06 recipe deferral. No automatic inbound append or enum rejection is part of this implementation.
3. **Phase: Directional forward and announce proof**: exercise `TestLocalASOptionsProduceDifferentASPaths`, `TestLocalASReplaceASSendsOnlyTheLocalAS` and `TestLocalASNoPrependLeavesEveryOutboundPathAlone`, plus the announce rail, against AC-1 through AC-5. Test required equalities as well as the `no-prepend`/`replace-as` inequality.
4. **Phase: Four-octet and multi-peer proof**: exercise `TestLocalASFourOctetTowardTwoOctetPeer` and the inbound functional scenario against AC-6, AC-8 and AC-9; decode AS_PATH and AS4_PATH together.
5. **Phase: Section 3.3 evidence**: the tags already exist in `reactor/rfc7705_local_as_test.go` and `config/peers_test.go`. Check all five requirement rows and enforcing tests for AC-10 through the current RFC gate. Enrolment is completed history, not a remaining dependency.
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
| Correctness | AC-5's per-direction matrix holds on forward and announce: required equalities stay equal, `no-prepend` and `replace-as` alone differ outbound, and a peer with no override is untouched |
| Naming | The enum names stay recognisable to an operator migrating from another daemon, and the descriptions use RFC 7705's vocabulary |
| Data flow | The distinction is computed once into the forward facts, never re-derived per destination |
| Registration over hardcoding | No per-option branch is added to a core package: the option resolves to a value the existing encoder choice already consumes |
| Rule: `ai/rules/evidence.md` | Both enums remain accepted and documented in their own directions; no test demands a third outbound result |
| Rule: `ai/rules/rfc-compliance.md` | The 2026-09-06 owner ruling remains attached to the separate inbound-recipe scope |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Directional distinctions and equalities hold | `TestLocalASOptionsProduceDifferentASPaths` plus the two directional tests and announce-path proof of AC-5 |
| Wire-level coverage exists | Run both named plugin scenarios and record their discriminating results |
| The collapse is gone | `grep -n "!s.LocalASNoPrepend && !s.LocalASReplaceAS" internal/component/bgp/reactor/peer_forward_facts.go` returns nothing |
| Section 3.3 is evidenced | `grep -c "RFC7705-3.3" ai/RFC-REQUIREMENTS.md` shows all five with enforcing tests |
| Ledger regenerated in the same commit | `./le rfc index-update` then `git diff --stat ai/RFC-REQUIREMENTS.md` |
| No unrelated regressions | `./le verify current mode full` |

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
| A change requires automatic inbound append or enum removal | Return to the owner; the deferred recipe ruling does not authorise either here |
| `no-prepend` alone produces the `replace-as` outbound result | Fix the outbound selection; equal output for the pairs in AC-5 remains required |
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
| Preserve the inbound-recipe deferral | Add automatic inbound append here; remove the enum | Thomas chose the recipe on 2026-09-06, recorded in `plan/spec-bgp-local-as-inbound-append.md` |
| Write the `.ci` files against current behaviour first | write them against the intended behaviour so they fail | Recording the collapse explicitly is what makes the later change reviewable as an expectation edit rather than a silent wire change |
| Enrolment lives in the sibling spec | enrol here with the Section 4.2 four classified `{gap}` | Enrolment admits an RFC only when every gated MUST is classified, and classifying the Section 4.2 four is the other spec's decision |

## Known Limitations

- This spec does not implement RFC 7705 Section 4.2. That is `plan/immediate/spec-bgp-as-migration.md`.
- RFC 7705 enrolment and the Section 3.3 tags were added on 2026-09-14. The earlier untagged limitation is resolved at source; AC-10 still requires the current evidence check before closure.
- `RFC7705-3.3-9`, the automatic inbound Local-AS append, remains unimplemented. Thomas deferred its operator recipe and safeguards to `plan/spec-bgp-local-as-inbound-append.md`; equal outbound results under AC-5 do not bring that scope back here.
- The ANNOUNCE rail collapsed all three configurations until 2026-09-06 and no longer does. `buildBatchASPathAttr` and `announceASPathASNs` (`internal/component/bgp/reactor/reactor_api_batch.go`) took one `localAS` and prepended it alone, so a route ze ORIGINATED toward a local-as peer with no modifier carried the legacy AS by itself and `local-as` behaved as `replace-as` there. Both now take `localASPrepend`, built by `localASPrependFor` from the same `secondaryPrependAS` the forward rails read. The row at `plan/journal/guard-added-to-one-half-of-a-pair.md` records the find and its fix.
- The retired whole-payload AS_PATH writers are no longer the forward path. Current proof must exercise `ASPathEdit.Record` through the reactor and the announce builder.
- The `.ci` files cover IPv4 unicast. Other families take the same egress prepend path, so the coverage is representative rather than exhaustive.

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
- [ ] `./le verify worktree` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
- [ ] Learned summary written to `plan/learned/NNN-bgp-local-as-options.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/immediate/spec-bgp-local-as-options.md` only (commit A preserves the spec in history)

## Historical progress, 2026-09-06

This is the dated record before the 2026-09-14 enrolment. Its enrolment statement
below is superseded by the current Task and evidence tables.

The announce rail landed in `5001022d13`. The three local-as configurations now
differ on that rail. The group key had carried a bare `localAS`, so one peer
received another peer's AS_PATH, decided by map order. Earlier work is in
`b60737ac8`.

RFC 7705 is still not enrolled, so no `RFC7705-3.3-N` tag can be written.

The inbound append is now an operator decision. It is homed in
`plan/spec-bgp-local-as-inbound-append.md`.
