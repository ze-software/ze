# Spec: bgp-as-migration -- RFC 7705 Section 4.2 Internal BGP AS Migration

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | `plan/spec-bgp-local-as-options.md` |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; RFC7705-4.2-5 is a candidate for a RULED deferral only after A-5 resolves, and no such ruling exists yet. Create `plan/deferrals/bgp-as-migration.md` on the first deferral) |
| Updated | 2026-07-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7705 Section 4.2 defines "Internal BGP AS Migration": a router being renumbered
runs iBGP sessions that may be opened under either the globally configured ASN or a
locally configured one, in either direction, and treats the resulting session as
native iBGP regardless of which ASN won.

Ze implements none of it. The summary, parked at `rfc/pending/rfc7705.md`, records
four gated MUSTs for this section, and each is currently unmet or met by accident:

| ID | Requirement | Ze today |
|----|-------------|----------|
| `RFC7705-4.2-1` | configurable per neighbour or per neighbour group | No leaf exists. The `local`, `remote` and `local-options` leaves sit inside a `container asn` at `internal/component/bgp/yang/ze-bgp-conf.yang`, with nothing for an alternate ASN |
| `RFC7705-4.2-2` | MUST accept an OPEN whose My AS is either the global or the local ASN | Accidentally satisfied, for the wrong reason: nothing validates the advertised AS at all, so any ASN is accepted |
| `RFC7705-4.2-3` | MUST send its own OPEN using either ASN | Not implemented. `myAS` is taken from `s.settings.LocalAS` and nothing else (`internal/component/bgp/reactor/session_negotiate.go`) |
| `RFC7705-4.2-4` | MUST treat UPDATEs on such a session as native iBGP | Not implemented. iBGP is decided by a single equality, `n.LocalAS != n.PeerAS` (`internal/component/bgp/reactor/peer_settings.go`), which has no notion of an alternate ASN |

`RFC7705-4.2-2` deserves care. `NotifyOpenBadPeerAS` is defined at
`internal/component/bgp/message/notification.go` and decoded for display at
`internal/component/bgp/message/notification.go` and
`internal/component/bgp/format/decode.go`, but nothing originates it. The only
consumer of the peer's advertised AS on the OPEN rail is the RFC 6286 identifier
check, which reads it as a fallback at
`internal/component/bgp/reactor/session_open_validation.go` and never compares it
to the configured `PeerAS`. So Ze accepts an OPEN from **any** AS, which satisfies
"accept either" the way an unlocked door satisfies "let the right people in".

That makes the first implementation step counter-intuitive and load-bearing:
**introduce the Bad Peer AS check first**, so that accepting the alternate ASN is a
deliberate exception rather than the absence of a rule. Without it there is nothing
for `RFC7705-4.2-2` to be an exception to, and a tagged negative-polarity test is
impossible to write.

**Goal.** Implement Section 4.2 in full, prove all four gated MUSTs with tagged
tests in both polarities, and enrol RFC 7705 in `rfc/enrolled.txt`.

**Enrolment is the closing act, and the summary is parked until then.** A summary
sitting in `rfc/short/` while declaring nine un-enrolled gated MUSTs fails
`./le rfc check`, which runs inside `./le verify current mode full` and `./le verify current mode changed`, so it would
red the tree for every session in this checkout for as long as this spec takes.
Thomas ruled on 2026-07-28 to park it rather than carry that cost, so the summary
lives at `rfc/pending/rfc7705.md` and the source text stays at
`rfc/full/rfc7705.txt`. Neither location is scanned: summaries are globbed from
`rfc/short/` only, and `rfc/full/` is consulted only for an already-enrolled RFC.

Returning it is therefore part of this spec's work, not a precondition of it. The
five Section 3.3 requirements are `plan/spec-bgp-local-as-options.md`'s to prove;
this spec proves the Section 4.2 four, moves the summary back into `rfc/short/`,
and writes the enrolment row that admits all nine in the same change.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - session lifecycle and OPEN handling
  → Constraint: both OPEN rails must enforce the same policy. `runOpenValidator` (`internal/component/bgp/reactor/session_open_validation.go`) exists because the collision-winner rail once skipped per-peer OPEN policy entirely; a new AS check must not reintroduce that asymmetry.
- [ ] `ai/rules/config.md` - YANG versus environment variables
  → Constraint: an alternate iBGP ASN is per-neighbour operational configuration, so it is a YANG leaf under the existing `session > asn` container, not an environment variable.
- [ ] `ai/rules/config.md` - naming for config leaves
  → Constraint: the new leaf sits beside `local` and `remote` in the `container asn` at `internal/component/bgp/yang/ze-bgp-conf.yang` and must read consistently with them.
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context
  → Constraint: the ASN width negotiated for a session drives the encoding context. An alternate ASN that crosses the two-octet boundary changes what My AS carries, because a four-octet ASN is sent as AS_TRANS with the real value in the ASN4 capability (`internal/component/bgp/reactor/session_negotiate.go`).
- [ ] `ai/rules/evidence.md` - a guard must fail closed or say something
  → Constraint: introducing the Bad Peer AS check tightens behaviour for every existing session. A peer whose advertised AS does not match its configuration is currently accepted; after this spec it is rejected with a NOTIFICATION. That is the correct direction but it is a behaviour change with an operational blast radius, so it needs its own test coverage and a release note.
- [ ] `ai/rules/rfc-compliance.md` - when a compliance decision needs the owner
  → Decision: Thomas ruled on 2026-07-28 to build Section 4.2 rather than classify it `{gap}`. That ruling is why this spec exists and why no `{gap}` annotation appears in its enrolment row.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7705.txt` - AS migration mechanisms. The implementation summary is written but parked at `rfc/pending/rfc7705.md`; this spec is what returns it to `rfc/short/`.
  → Constraint: Section 4.2 requires accepting an OPEN whose My AS is either ASN, sending an OPEN with either ASN, and treating the session as native iBGP in every case.
  → Constraint: `RFC7705-4.2-5` is a SHOULD, not a MUST: to avoid a deadlock when both speakers run the mechanism, send the globally configured ASN first and fall back to the locally configured one only after the peer answers Bad Peer AS. Implementing it requires originating and consuming that NOTIFICATION subcode, which is the same machinery `RFC7705-4.2-2` needs.
- [ ] `rfc/short/rfc4271.md` - OPEN message, My Autonomous System, NOTIFICATION
  → Constraint: Section 4.2 fixes the OPEN layout and Section 6.2 defines the OPEN Message Error subcodes, of which Bad Peer AS is subcode 2. The check this spec introduces is RFC 4271's, not RFC 7705's; RFC 7705 only carves an exception out of it.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN
  → Constraint: My AS carries AS_TRANS for an ASN above 65535 and the real value rides in the ASN4 capability, so an AS comparison must read the capability, exactly as `openAdvertisedAS` (`internal/component/bgp/reactor/peer.go`) already does.
- [ ] `rfc/short/rfc4456.md` - route reflection
  → Constraint: `RFC7705-4.2-4` names RFC 4456 explicitly. A session established under the alternate ASN must take the reflection rules, including ORIGINATOR_ID and CLUSTER_LIST handling, that a native iBGP session takes.
- [ ] `rfc/short/rfc6286.md` - AS-wide unique BGP Identifier
  → Constraint: the "internal peer" test at `internal/component/bgp/reactor/session_open_validation.go` decides Section 2.2 enforcement. If a session is iBGP under an alternate ASN, that test must agree, or the identifier check silently changes meaning for exactly the speakers most likely to be renumbering.

**Key insights:** (minimal context to resume after compaction)
- Ze has no Bad Peer AS enforcement at all. That is why `RFC7705-4.2-2` looks satisfied and is not provable: there is no negative polarity to test.
- iBGP is one equality in one method, `internal/component/bgp/reactor/peer_settings.go`, mirrored on the peer object at `internal/component/bgp/reactor/peer.go` and re-derived inline at `internal/component/bgp/reactor/session_validation.go`. Three sites, one rule. An alternate ASN has to change the rule in one place and reach all three.
- The OPEN this speaker sends carries `MyAS` at `internal/component/bgp/reactor/session_negotiate.go`, taking `myAS` from `s.settings.LocalAS` at `internal/component/bgp/reactor/session_negotiate.go`, and the ASN4 capability from the same field at `capability.ASN4` (`internal/component/bgp/reactor/session_negotiate.go`). Both must move together or the OPEN contradicts itself.
- `RFC7705-4.2-5`'s fallback needs a retry that changes the ASN between connection attempts, which touches the FSM's connect-retry path, not just the OPEN builder.
- Enrolment is the exit condition, and it needs `plan/spec-bgp-local-as-options.md` landed first, because a row admits an RFC only when every gated MUST is classified.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the per-peer `asn` container: `leaf local` at `internal/component/bgp/yang/ze-bgp-conf.yang`, `leaf remote` at `internal/component/bgp/yang/ze-bgp-conf.yang`, `leaf-list local-options` at `internal/component/bgp/yang/ze-bgp-conf.yang`. There is no leaf for an alternate iBGP ASN.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the `session` container is group-to-peer inherited, which is what will satisfy `RFC7705-4.2-1` once a leaf exists.
- [ ] `internal/component/bgp/reactor/config.go` - `peerLocalAS := localAS`, the per-peer local AS defaulting to the global value, then overridden from `session > asn > local`.
- [ ] `internal/component/bgp/reactor/config.go` - `ps := NewPeerSettings(ip, peerLocalAS, peerAS, peerRouterID)`, with the router's global ASN preserved separately by `ps.GlobalLocalAS = localAS` at `internal/component/bgp/reactor/config.go`.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `myAS := uint16(s.settings.LocalAS)`, narrowed to `myAS = 23456` for AS_TRANS above 65535 at `internal/component/bgp/reactor/session_negotiate.go`.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - the OPEN's `MyAS` field, drawn from the same resolved value; the ASN4 capability is appended from `s.settings.LocalAS` at `internal/component/bgp/reactor/session_negotiate.go`.
- [ ] `internal/component/bgp/reactor/session_open_validation.go` - `validateOpenIdentifier`: the only OPEN-time consumer of the peer's AS. It reads the configured `s.settings.PeerAS` at `internal/component/bgp/reactor/session_open_validation.go`, falls back to `openAdvertisedAS(open)` at `internal/component/bgp/reactor/session_open_validation.go`, and uses it solely for the RFC 6286 `internal` determination at `internal/component/bgp/reactor/session_open_validation.go`. **It never compares the advertised AS to the configured one.**
- [ ] `internal/component/bgp/reactor/session_open_validation.go` - `runOpenValidator`: the per-peer plugin OPEN validator, shared by both OPEN rails so the collision-winner path cannot bypass policy. A new AS check belongs on the same shared rail.
- [ ] `internal/component/bgp/reactor/peer.go` - `openAdvertisedAS`: reads the ASN4 capability first and falls back to `remote.MyAS` at `internal/component/bgp/reactor/peer.go`, so a four-octet peer is judged on its real ASN rather than AS_TRANS. This is the correct comparison primitive and already exists.
- [ ] `internal/component/bgp/message/notification.go` - the `NotifyOpenBadPeerAS` constant: defined here, rendered by `case NotifyOpenBadPeerAS:` at `internal/component/bgp/message/notification.go` and by `case message.NotifyOpenBadPeerAS:` at `internal/component/bgp/format/decode.go`, and originated nowhere.
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `IsEBGP`: `return n.LocalAS != n.PeerAS`, the single rule.
- [ ] `internal/component/bgp/reactor/peer.go` - the same rule on the peer object, under the peer lock.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `isIBGP := s.settings.LocalAS == s.settings.PeerAS`, the rule re-derived inline on the RFC 7606 path.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - the precomputed `s.IsEBGP()` fact that gates the eBGP AS_PATH prepend and therefore every wire difference between iBGP and eBGP.
- [ ] `internal/component/bgp/reactor/config_test.go` - the existing `peers[0].LocalASNoPrepend` config coverage for the ASN container, which is where a new leaf's parse test joins.

**Behavior to preserve:**
- Every session that does not configure the new leaf behaves exactly as today: same OPEN, same ASN4 capability, same iBGP or eBGP determination, same wire.
- The RFC 6286 identifier validation and its `internal` determination, including the dynamic-peer fallback that reads the advertised AS.
- Both OPEN rails enforcing the same policy, so the collision winner cannot skip a check.
- AS_TRANS handling for a four-octet local AS, in both the OPEN header and the ASN4 capability.
- The `session` container's group-to-peer inheritance.
- Every existing expectation under `test/parse/`, `test/plugin/` and `test/policy/`.

**Behavior to change:**
- A new per-neighbour leaf declares an alternate iBGP ASN, satisfying `RFC7705-4.2-1`.
- The OPEN rail gains a Bad Peer AS check comparing the advertised AS to the configured `PeerAS`. **This tightens behaviour for every existing session**: a peer advertising an unexpected AS is currently accepted and will be rejected with OPEN subcode 2.
- With the new leaf set, that check accepts either the global or the alternate ASN, satisfying `RFC7705-4.2-2`.
- The OPEN this speaker sends may carry the alternate ASN, satisfying `RFC7705-4.2-3`.
- The iBGP determination accounts for the alternate ASN so such a session takes the native iBGP path, satisfying `RFC7705-4.2-4`.
- `rfc/enrolled.txt` gains an `rfc7705` row, and `ai/RFC-REQUIREMENTS.md` records an enforcing test for all nine gated MUSTs.

## Data Flow (MANDATORY)

### Entry Point
- Configuration: `session > asn`, extended with the alternate-ASN leaf, inherited group to peer.
- An inbound TCP connection carrying an OPEN, arriving on either the `handleOpen` rail or the collision-winner `processOpen` rail.
- An outbound connection attempt for which this speaker builds its own OPEN.

### Transformation Path
1. Config parse fills `LocalAS`, `PeerAS`, `GlobalLocalAS` and, **proposed**, the alternate ASN on the peer settings, via `NewPeerSettings` at `internal/component/bgp/reactor/config.go`.
2. Outbound: `myAS` is chosen at `internal/component/bgp/reactor/session_negotiate.go` and the `capability.ASN4` value at `internal/component/bgp/reactor/session_negotiate.go`. **Proposed:** the choice consults the alternate ASN, and per `RFC7705-4.2-5` prefers the globally configured ASN first.
3. Inbound: the OPEN is validated. **Proposed:** a Bad Peer AS check runs on the shared rail beside `validateOpenIdentifier` (`internal/component/bgp/reactor/session_open_validation.go`), comparing `openAdvertisedAS` against the configured `PeerAS` and, when configured, the alternate ASN.
4. On a mismatch the session sends OPEN subcode 2, the `NotifyOpenBadPeerAS` constant at `internal/component/bgp/message/notification.go`, and closes, the same shape `validateOpenIdentifier` already uses for Bad BGP Identifier.
5. **Proposed:** on receiving Bad Peer AS as the initiator, the connect-retry path retries with the alternate ASN, satisfying `RFC7705-4.2-5`.
6. Once established, the iBGP determination (`internal/component/bgp/reactor/peer_settings.go`) resolves the session as internal, so the RFC 7606 path (`internal/component/bgp/reactor/session_validation.go`), the forward facts (`internal/component/bgp/reactor/peer_forward_facts.go`) and the RFC 4456 reflection rules all take the iBGP branch.
7. UPDATEs are exchanged with no eBGP AS_PATH prepend, which is the observable that `RFC7705-4.2-4` is about.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG config to peer settings | a new ASN leaf reaches `PeerSettings` alongside `LocalAS` | No |
| Peer settings to outbound OPEN | `myAS` and the ASN4 capability must agree on which ASN is in play | No |
| Inbound OPEN to session policy | the advertised AS, read through the ASN4 capability, compared on both OPEN rails | No |
| Session policy to NOTIFICATION | OPEN subcode 2 originated for the first time | No |
| Peer settings to iBGP determination | one rule consumed by three sites | No |
| Established session to forward path | the iBGP verdict gates the AS_PATH prepend and the reflection rules | No |

### Integration Points
- `openAdvertisedAS` (`internal/component/bgp/reactor/peer.go`) is the existing, correct primitive for reading a peer's real ASN; the new check uses it rather than a second implementation.
- `validateOpenIdentifier` (`internal/component/bgp/reactor/session_open_validation.go`) and `runOpenValidator` (`internal/component/bgp/reactor/session_open_validation.go`) define the shape a shared OPEN check takes, including logging, the FSM error event and the connection close.
- The `PeerSettings` `IsEBGP` rule (`internal/component/bgp/reactor/peer_settings.go`) is the one the alternate ASN must extend; the `Peer` copy at `internal/component/bgp/reactor/peer.go` and the inline `isIBGP` derivation at `internal/component/bgp/reactor/session_validation.go` must be routed through it rather than each growing their own condition.
- The parked summary at `rfc/pending/rfc7705.md` supplies the requirement IDs and returns to `rfc/short/` in phase 8; `rfc/enrolled.txt` and `ai/RFC-REQUIREMENTS.md` are the ledger this spec closes.

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
| A-1 | Nothing in the tree currently rejects an OPEN on an AS mismatch, so introducing the check is a genuine behaviour change and not a duplicate. | `NotifyOpenBadPeerAS` (`internal/component/bgp/message/notification.go`) is originated nowhere, and `validateOpenIdentifier` (`internal/component/bgp/reactor/session_open_validation.go`) reads the AS only for the RFC 6286 determination. | The check already exists somewhere and this spec must extend it rather than add one. | Tree-wide grep for `NotifyOpenBadPeerAS` and for any comparison against `settings.PeerAS`, as the first implementation action. | unvalidated |
| A-2 | The three iBGP determination sites can be routed through one rule without changing any existing verdict. | They are textually identical today: `internal/component/bgp/reactor/peer_settings.go`, `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/session_validation.go`. | A site has a subtly different meaning and consolidating it changes behaviour for sessions that do not use the feature. | A refactor-only commit that unifies the three with no behaviour change, proven by the existing suites passing untouched. | unvalidated |
| A-3 | Introducing the Bad Peer AS check breaks no existing test or deployment, because a correctly configured peer advertises the AS it is configured with. | The configured `PeerAS` is what every session already assumes when deciding iBGP versus eBGP. | Sessions that work today start failing. That is a real operational risk and the reason the check lands in its own phase with its own `.ci`. | Running the full functional and interop suites after the check lands, before anything else in this spec. | unvalidated |
| A-4 | A dynamic peer, whose `PeerAS` is 0 until establishment, must be exempt from the new check. | `validateOpenIdentifier` documents exactly this: the comment at `buildDynamicPeerSettings` (`internal/component/bgp/reactor/session_open_validation.go`) records that it sets `PeerAS` to 0 and that `resolveDynamicPeerSettings` fills it only at establishment. | Every dynamic peer is rejected at OPEN, which would be a severe regression. | A dedicated dynamic-peer test asserting the check is skipped when `PeerAS` is 0. | unvalidated |
| A-5 | `RFC7705-4.2-5`'s fallback can be implemented within the existing connect-retry path without a new FSM state. | The retry already exists as a timer-driven reconnect; the change is which ASN the next OPEN carries. | The SHOULD is deferred with an explicit annotation rather than silently skipped, and that deferral is a compliance decision for Thomas. | A design spike on the connect-retry path before phase 5 starts. | unvalidated |
| A-6 | Enrolling RFC 7705 requires `plan/spec-bgp-local-as-options.md` to have landed, because a row admits an RFC only when every gated MUST is classified. | `rfc/enrolled.txt` header: an enrolled RFC has every MUST-level requirement either covered by tagged tests or annotated. | The two specs must land together in one change, which enlarges the commit but does not change the work. | `./le rfc check` after both specs' tests exist. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The Bad Peer AS check is a tightening that can drop sessions which work today, including ones whose configuration was quietly wrong and tolerated. | Functional or interop suites failing at the phase that introduces the check, or a session dropping in soak. | The check lands alone, in its own phase, before any RFC 7705 feature work, so a regression is attributable. It also gets a release note, because operators may be relying on the tolerance. |
| R-2 | An alternate ASN that changes the iBGP verdict changes the AS_PATH the peer sees, the reflection rules applied, and the RFC 7606 iBGP branch, all at once. | The iBGP-specific tests, and a `.ci` that reads AS_PATH off the wire on an alternate-ASN session. | `RFC7705-4.2-4` is tested as an observable (no eBGP prepend, reflection attributes applied), not as an internal flag. |
| R-3 | Both speakers running the mechanism can deadlock, which is precisely what `RFC7705-4.2-5` exists to prevent. | An interop scenario with both sides configured. | Implement the SHOULD, or record it as a deliberate deferral with Thomas's ruling. Not silently skip it. |
| R-4 | The header AS is set from `myAS` at `internal/component/bgp/reactor/session_negotiate.go` and the capability from `capability.ASN4` at `internal/component/bgp/reactor/session_negotiate.go`; changing one and not the other produces an OPEN that contradicts itself. | A test asserting the header AS and the capability ASN agree for every configuration. | Both are derived from one resolved value, computed once. |
| R-5 | The RFC 6286 `internal` determination depends on the same AS comparison; an alternate ASN changes which peers are treated as internal for identifier validation. | The RFC 6286 tests. | `RFC7705-4.2-4`'s "treat as native iBGP" is taken to include the identifier check, and a test pins it. |
| R-6 | The summary is parked outside `rfc/short/`, so nothing reminds anyone it exists and the work could be redone or forgotten. | A future session fetching RFC 7705 again, or `ls rfc/pending/` being non-empty with no spec pointing at it. | Three specs name the parked path in their Required Reading, and phase 8 below is the single step that returns it. `rfc/pending/` holding a file with no spec referencing it is the signal that something was dropped. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Sessions fail to establish, or establish under the wrong AS relationship. A session wrongly treated as iBGP skips the eBGP AS_PATH prepend, which breaks loop detection and leaks routes that should have been filtered by AS. A wrongly rejected OPEN takes a working peering down. |
| How is it reverted? | Per phase, single commit revert. The Bad Peer AS check is separately revertible from the migration feature, which is why it lands alone. Once a peer has accepted routes carrying a wrong AS_PATH the effect propagates beyond us. |
| Who else touches this path? | `plan/spec-bgp-local-as-options.md` owns the Section 3.3 half and must land first; the AS_PATH encoders moved under a resolver and that work has LANDED with the wire-edit-3 AS_PATH fold; it moves the AS_PATH encoders and consumes the iBGP verdict; `plan/spec-bgp-remote-as-auto.md` and `plan/spec-bgp-session-ready-contract.md` touch the same OPEN and session-establishment paths. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer opens a session advertising an AS that does not match its configuration | → | the new Bad Peer AS check on the shared OPEN rail | `test/plugin/bgp-open-bad-peer-as.ci` |
| A peer configured for AS migration opens with the alternate ASN | → | the check accepts either ASN and the session establishes as iBGP | `test/plugin/bgp-as-migration-accept-either.ci` |
| This speaker opens a session toward a peer configured for AS migration | → | the OPEN carries the resolved ASN in both the header and the ASN4 capability | `test/plugin/bgp-as-migration-send-either.ci` |
| Routes are exchanged on a session established under the alternate ASN | → | native iBGP treatment: no eBGP prepend, RFC 4456 rules applied | `test/plugin/bgp-as-migration-ibgp-treatment.ci` |
| A dynamic peer opens a session | → | the new check is skipped because the configured AS is not yet known | `test/plugin/bgp-open-bad-peer-as.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer advertising an AS that matches neither its configured `PeerAS` nor any configured alternate | The OPEN is rejected with OPEN Message Error subcode 2, the connection closes, and the event is logged naming both the advertised and the expected AS |
| AC-2 | A peer advertising its configured `PeerAS` | The session establishes exactly as today |
| AC-3 | A dynamic peer, whose configured AS is not yet known | The new check is skipped, and the session establishes as it does today |
| AC-4 | A peer configured for AS migration, advertising the globally configured ASN | The OPEN is accepted and the session establishes, satisfying `RFC7705-4.2-2` |
| AC-5 | The same peer advertising the locally configured alternate ASN | The OPEN is accepted and the session establishes, satisfying `RFC7705-4.2-2` in its second polarity |
| AC-6 | This speaker initiating toward a migration peer | The OPEN carries the resolved ASN, and the header My AS and the ASN4 capability agree, satisfying `RFC7705-4.2-3` |
| AC-7 | A session established under the alternate ASN, exchanging routes | UPDATEs are treated as native iBGP: no eBGP AS_PATH prepend, RFC 4456 reflection rules applied, RFC 7606 iBGP branch taken. Satisfies `RFC7705-4.2-4` |
| AC-8 | The alternate-ASN leaf set at group level with a peer-level override | Both are honoured, satisfying `RFC7705-4.2-1` |
| AC-9 | A four-octet alternate ASN | My AS carries AS_TRANS and the ASN4 capability carries the real value, consistently on both the send and the compare side |
| AC-10 | Both speakers configured for AS migration | The session establishes without deadlock, per `RFC7705-4.2-5`, or the SHOULD is recorded as a ruled deferral |
| AC-11 | A peer with no migration configuration at all | Every observable is byte-identical to before this spec, except that an AS mismatch is now rejected |
| AC-12 | `./le rfc check` after this spec and its dependency close | Exit 0. `rfc7705` is enrolled and all nine gated MUSTs name an enforcing test in `ai/RFC-REQUIREMENTS.md` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Renumbers a router into a new AS and configures the alternate iBGP ASN on its internal sessions | config, OPEN send and accept under either ASN, native iBGP treatment | `test/plugin/bgp-as-migration-accept-either.ci` |
| 2 | Peers with a router mid-migration that opens with the old ASN | inbound OPEN, either-ASN acceptance, session establishes as iBGP | `test/plugin/bgp-as-migration-accept-either.ci` |
| 3 | Exchanges routes across a migration session and expects no eBGP prepend | established session, iBGP verdict, forward path with no prepend | `test/plugin/bgp-as-migration-ibgp-treatment.ci` |
| 4 | Misconfigures a peer's remote AS and expects to be told | inbound OPEN, Bad Peer AS, NOTIFICATION and a log line naming both ASNs | `test/plugin/bgp-open-bad-peer-as.ci` |
| 5 | Runs both ends of a migration session | outbound OPEN with the global ASN first, fallback on Bad Peer AS | `test/plugin/bgp-as-migration-send-either.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOpenRejectedOnBadPeerAS` | `internal/component/bgp/reactor/session_open_validation_test.go` | AC-1: subcode 2 originated, connection closed, both ASNs logged | |
| `TestOpenAcceptedOnMatchingAS` | `internal/component/bgp/reactor/session_open_validation_test.go` | AC-2: the negative polarity of the same check | |
| `TestOpenCheckSkippedForDynamicPeer` | `internal/component/bgp/reactor/session_open_validation_test.go` | AC-3, A-4: a zero configured AS exempts the peer | |
| `TestMigrationAcceptsGlobalASN` | `internal/component/bgp/reactor/session_open_validation_test.go` | AC-4, tagged `RFC requirement: RFC7705-4.2-2 positive` | |
| `TestMigrationAcceptsAlternateASN` | `internal/component/bgp/reactor/session_open_validation_test.go` | AC-5, tagged `RFC requirement: RFC7705-4.2-2 negative` for the unconfigured-ASN case | |
| `TestMigrationOpenCarriesResolvedASN` | `internal/component/bgp/reactor/session_negotiate_test.go` | AC-6, AC-9, R-4, tagged `RFC requirement: RFC7705-4.2-3`, both polarities; header and capability agree | |
| `TestMigrationSessionIsIBGP` | `internal/component/bgp/reactor/peer_settings_test.go` | AC-7, tagged `RFC requirement: RFC7705-4.2-4`, both polarities | |
| `TestMigrationNoEBGPPrepend` | `internal/component/bgp/reactor/peer_forward_facts_test.go` | AC-7: the observable, not the flag | |
| `TestMigrationLeafPerNeighborGroup` | `internal/component/bgp/reactor/config_test.go` | AC-8, tagged `RFC requirement: RFC7705-4.2-1`, both polarities | |
| `TestIBGPVerdictSingleRule` | `internal/component/bgp/reactor/peer_settings_test.go` | A-2: the three sites agree for every combination | |
| `TestMigrationFallbackOnBadPeerAS` | `internal/component/bgp/reactor/session_negotiate_test.go` | AC-10, `RFC7705-4.2-5`, only if A-5 resolves to implementing it | |
| `TestNoMigrationConfigUnchanged` | `internal/component/bgp/reactor/session_negotiate_test.go` | AC-11: the OPEN is byte-identical without the leaf | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| alternate iBGP ASN | 1-4294967295 | 4294967295 | 0 (leaf absent, feature off) | N/A (uint32 domain) |
| My AS on the wire | 0-65535 | 65535 | N/A | N/A (AS_TRANS carries anything above) |
| mappable ASN in the OPEN header | 1-65535 | 65535 | 0 | 65536 (AS_TRANS 23456, real value in the ASN4 capability) |
| configured `PeerAS` for the new check | 0-4294967295 | 4294967295 | N/A | N/A; 0 means dynamic and exempts the check |
| accepted ASN values per session | 1-2 | 2 (global plus alternate) | 0 (no session could establish) | 3 (no configuration produces it) |
| OPEN Message Error subcode | 1-11 | 11 | 0 | 12 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-open-bad-peer-as` | `test/plugin/bgp-open-bad-peer-as.ci` | a misconfigured remote AS is rejected with subcode 2, and a dynamic peer still establishes | |
| `bgp-as-migration-accept-either` | `test/plugin/bgp-as-migration-accept-either.ci` | a migration peer establishes under either ASN | |
| `bgp-as-migration-send-either` | `test/plugin/bgp-as-migration-send-either.ci` | this speaker's OPEN carries the resolved ASN in header and capability | |
| `bgp-as-migration-ibgp-treatment` | `test/plugin/bgp-as-migration-ibgp-treatment.ci` | routes cross a migration session with no eBGP prepend and RFC 4456 rules applied | |
| `session-policy-config` | existing `test/parse/session-policy-config.ci` | the parse-level `asn` coverage keeps passing with the new leaf present | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-as-migration-ibgp-frr` | `test/interop/scenarios/` | FRR | a real iBGP peer establishes against a speaker opening with the alternate ASN and exchanges routes as native iBGP | |
| `NN-as-migration-bad-peer-as-bird` | `test/interop/scenarios/` | BIRD | a real peer receives and reports OPEN subcode 2 when the AS does not match | |

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `asn` container gains the alternate iBGP ASN leaf beside `local` and `remote`
- `internal/component/bgp/reactor/config.go` - parse the new leaf into `PeerSettings`
- `internal/component/bgp/reactor/peer_settings.go` - `IsEBGP` becomes the single rule that accounts for the alternate ASN
- `internal/component/bgp/reactor/peer.go` - the peer-object copy of the rule routes through the settings rule
- `internal/component/bgp/reactor/session_validation.go` - the inline iBGP derivation routes through the same rule
- `internal/component/bgp/reactor/session_open_validation.go` - the Bad Peer AS check, on the shared rail
- `internal/component/bgp/reactor/session_negotiate.go` - the OPEN header AS and the ASN4 capability derive from one resolved value
- `rfc/enrolled.txt` - the `rfc7705` enrolment row
- `ai/RFC-REQUIREMENTS.md` - regenerated in the same commit
- `docs/guide/configuration.md` - the AS migration configuration
- `docs/features/rfc-status.md` - the RFC 7705 row
- `docs/architecture/core-design.md` - OPEN validation gains an AS check

## Files to Create
- `internal/component/bgp/reactor/session_as_migration.go` - resolving which ASN to send and which to accept, so the logic is not spread across the OPEN builder and the validator
- `internal/component/bgp/reactor/session_as_migration_test.go` - resolution coverage
- `test/plugin/bgp-open-bad-peer-as.ci` - the tightening, including the dynamic-peer exemption
- `test/plugin/bgp-as-migration-accept-either.ci` - either-ASN acceptance
- `test/plugin/bgp-as-migration-send-either.ci` - either-ASN sending
- `test/plugin/bgp-as-migration-ibgp-treatment.ci` - native iBGP treatment on the wire

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang`: a new leaf in the per-peer `asn` container |
| YANG validation constraints | Yes | The leaf takes the `zt:asn` type, matching `local` and `remote` |
| YANG custom validators | Yes | The alternate ASN must differ from the configured `local` ASN, which the native type cannot express |
| CLI commands/flags | No | No new commands; the session state is visible through existing peer output |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | Yes | Automatic for the typed leaf |
| Functional test for new RPC/API | Yes | Four new `.ci` files listed above |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | Per-neighbour operational config belongs in YANG |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_open_rejected_bad_peer_as_total`, labelled by peer, so the tightening in R-1 is observable rather than only logged |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code; the ASN4 capability already exists |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` for AS migration support |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the new leaf |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | The session and ASN section of `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | The OPEN layout is unchanged; which ASN it carries changes |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` RFC 7705 row moves to a proven state, with source anchors, and RFC 4271 Section 6.2 Bad Peer AS becomes originated |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: AS migration is a feature other daemons list |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: OPEN validation gains an AS check on the shared rail |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the rejection counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `session_negotiate.go`, `session_open_validation.go`, `peer_settings.go` and `ze-bgp-conf.yang` and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any `session > asn` example must show the new leaf where relevant |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the current state before changing it
   - Tests: `TestIBGPVerdictSingleRule` and a test asserting no OPEN is currently rejected on an AS mismatch, both written against current behaviour
   - Files: `internal/component/bgp/reactor/peer_settings_test.go`, `internal/component/bgp/reactor/session_open_validation_test.go`
   - Verify: A-1 and A-2 resolved by grep and test; the absence of the check is now recorded, not assumed
2. **Phase: unify the iBGP rule** -- refactor only, no behaviour change
   - Tests: the existing suites, untouched
   - Files: `internal/component/bgp/reactor/peer_settings.go`, `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/session_validation.go`
   - Verify: three sites route through one rule; every existing test passes with no edit
3. **Phase: the Bad Peer AS check** -- lands alone, so a regression is attributable
   - Tests: `TestOpenRejectedOnBadPeerAS`, `TestOpenAcceptedOnMatchingAS`, `TestOpenCheckSkippedForDynamicPeer`, `test/plugin/bgp-open-bad-peer-as.ci`
   - Files: `internal/component/bgp/reactor/session_open_validation.go`
   - Verify: AC-1, AC-2, AC-3 pass; A-3 and A-4 resolved; the full functional and interop suites run before proceeding
4. **Phase: the configuration surface**
   - Tests: `TestMigrationLeafPerNeighborGroup`
   - Files: `internal/component/bgp/yang/ze-bgp-conf.yang`, `internal/component/bgp/reactor/config.go`
   - Verify: AC-8 passes; the leaf inherits group to peer; the custom validator rejects an alternate equal to `local`
5. **Phase: accept and send either ASN**
   - Tests: `TestMigrationAcceptsGlobalASN`, `TestMigrationAcceptsAlternateASN`, `TestMigrationOpenCarriesResolvedASN`, `TestNoMigrationConfigUnchanged`, plus the accept and send `.ci` files
   - Files: `internal/component/bgp/reactor/session_as_migration.go`, `internal/component/bgp/reactor/session_negotiate.go`, `internal/component/bgp/reactor/session_open_validation.go`
   - Verify: AC-4, AC-5, AC-6, AC-9, AC-11 pass; R-4 closed by the header-and-capability agreement test
6. **Phase: native iBGP treatment**
   - Tests: `TestMigrationSessionIsIBGP`, `TestMigrationNoEBGPPrepend`, `test/plugin/bgp-as-migration-ibgp-treatment.ci`
   - Files: `internal/component/bgp/reactor/peer_settings.go`
   - Verify: AC-7 passes as an observable on the wire, not as an internal flag
7. **Phase: the deadlock fallback** -- `RFC7705-4.2-5`, a SHOULD
   - Tests: `TestMigrationFallbackOnBadPeerAS`
   - Files: `internal/component/bgp/reactor/session_as_migration.go` and the connect-retry path
   - Verify: AC-10 passes, or A-5 resolves to a ruled deferral recorded here
8. **Phase: unpark, enrol and close the ledger** -- BLOCKING, requires `plan/spec-bgp-local-as-options.md` landed
   - Tests: `./le rfc index-update` then `./le rfc check`
   - Files: move `rfc/pending/rfc7705.md` back under `rfc/short/`, then `rfc/enrolled.txt` and `ai/RFC-REQUIREMENTS.md`. Re-point the three specs' Required Reading rows at the restored summary and remove `rfc/pending/` if it is left empty
   - Verify: AC-12 passes, `./le rfc check` exits 0 with `rfc7705` enrolled rather than absent, and `ls rfc/pending/` is empty
9. **Phase: documentation, counter and interop**
   - Tests: the two interop scenarios
   - Files: the doc targets above, `test/interop/scenarios/`
   - Verify: real peers on both behaviours; every Documentation row marked Yes is done with source anchors

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and each of `RFC7705-4.2-1` through `RFC7705-4.2-4` names a tagged test in both polarities |
| Feature completeness | Every user story has a passing `.ci`, including the rejection story and the dynamic-peer exemption |
| Correctness | The OPEN header AS and the ASN4 capability never disagree; a session without the leaf is unchanged; the iBGP verdict has exactly one rule |
| Naming | The new leaf reads consistently with `local` and `remote` in the same container, per `ai/rules/config.md` |
| Data flow | The ASN resolution happens in one place and is consumed by both the OPEN builder and the validator; no site re-derives it |
| Registration over hardcoding | The check joins the existing shared OPEN validation rail rather than adding a branch that one rail can skip |
| Rule: `ai/rules/evidence.md` | A mismatched AS is rejected by name with both ASNs in the log; a dynamic peer is exempted deliberately and provably, not by a zero value falling through |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}` appears in the enrolment row for Section 4.2, because Thomas ruled to build it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Bad Peer AS is originated | `grep -rn "NotifyOpenBadPeerAS" internal/component/bgp/reactor/` returns a send site |
| One iBGP rule | `grep -rn "LocalAS != .*PeerAS\|LocalAS == .*PeerAS" internal/component/bgp/reactor/` returns one production site |
| The leaf exists and inherits | `go test ./internal/component/bgp/reactor/ -run TestMigrationLeafPerNeighborGroup` |
| Either ASN accepted | `go test ./internal/component/bgp/reactor/ -run TestMigrationAccepts` |
| Wire-level coverage | `ls test/plugin/bgp-as-migration-*.ci` returns three files |
| RFC 7705 enrolled | `grep -n "^rfc7705" rfc/enrolled.txt` returns the row |
| The gate is green | `./le rfc check` exits 0 |
| Ledger regenerated in the same commit | `./le rfc index-update` then `git diff --stat ai/RFC-REQUIREMENTS.md` |
| No unrelated regressions | `./le verify current mode full` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authentication boundary | The AS in an OPEN is attacker-supplied. Accepting "either ASN" widens what a peer may claim, so the widening must be exactly two configured values, never a range or a wildcard |
| Fail-open risk | Today any AS is accepted. The new check must fail closed: an unparseable or absent AS is a rejection, not a skip. The only exemption is a dynamic peer, and that exemption must be explicit rather than a zero value falling through a comparison |
| Relationship confusion | A session wrongly resolved as iBGP skips the eBGP AS_PATH prepend and applies reflection rules, which can leak internal routes to an external party. The iBGP verdict must derive only from configured values, never from what the peer advertised |
| Denial of service | The fallback in `RFC7705-4.2-5` retries with a different ASN on rejection. It must be bounded, so a peer answering Bad Peer AS forever cannot drive an unbounded reconnect loop |
| Information disclosure | The rejection log names both the advertised and the expected AS. That is operationally necessary and discloses only what the peer already sent and what we configured |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| An existing session breaks when the Bad Peer AS check lands | STOP and present. That is A-3 broken, and it is an operational question, not a test fix |
| A dynamic peer is rejected at OPEN | STOP. A-4 is broken and the exemption is wrong |
| The header AS and the ASN4 capability disagree | STOP. R-4 has fired; the resolution must happen once |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Interop peer rejects our OPEN | STOP and present. A real peer disagreeing is stronger evidence than any unit test |
| `./le rfc check` still fails after enrolment | Read `internal/le/rfc/rfc.go` for the specific violation; a tag that does not match an ID is the common cause |
| `RFC7705-4.2-5` cannot be implemented within the existing retry path | Do not silently skip it. Report, and let Thomas rule on the deferral |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The most surprising finding is not that Section 4.2 is missing, it is that `RFC7705-4.2-2` currently passes for the wrong reason. Ze accepts an OPEN from any AS because it never checks, so "accept either ASN" is true and meaningless. Implementing the requirement honestly means first implementing the rule it is an exception to.
- That inverts the natural build order. The tightening lands before the feature, alone, because it is the change most likely to break a working deployment and the one whose blast radius is hardest to predict from the code.
- The iBGP verdict is one equality repeated in three places (`internal/component/bgp/reactor/peer_settings.go`, `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/session_validation.go`). Any feature that complicates it must unify it first, or the third site silently keeps the old rule and a migration session is iBGP for the forward path and eBGP for RFC 7606.
- `openAdvertisedAS` (`internal/component/bgp/reactor/peer.go`) already does the hard part of the comparison, reading the ASN4 capability so a four-octet peer is judged on its real ASN rather than AS_TRANS. The primitive this spec needs already exists and is already correct.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Introduce the Bad Peer AS check before the migration feature | implement "accept either ASN" against the current no-check baseline | Without the rule there is no exception, and no negative-polarity test is writable. The requirement would be tagged with a test that cannot fail |
| The check lands in its own phase and its own commit | fold it into the migration feature | It is the change most likely to break a working deployment; attribution matters more than commit count |
| Unify the iBGP rule before extending it | add the alternate-ASN condition to all three sites | Three copies of a rule is three chances for a migration session to be classified inconsistently |
| One resolution site for which ASN is in play | resolve separately in the OPEN builder and the validator | The header AS and the ASN4 capability must agree, and two resolutions is how they stop agreeing |
| A new YANG leaf beside `local` and `remote` | reuse `local-options` with a third enum | The alternate ASN carries a value, not a mode; `local-options` is an enumeration of behaviours |
| Enrolment is the last phase | enrol early with the Section 4.2 four as `{gap}` | Thomas ruled to build rather than gap. Enrolling with a `{gap}` first would record a claim the ruling rejected |

## Known Limitations

- RFC 7705 Section 3.3 is out of scope here; `plan/spec-bgp-local-as-options.md` owns it and must land first for enrolment to be possible.
- `./le rfc check` stays red in this checkout until phase 8. That cost is real and shared with every concurrent session.
- `RFC7705-4.2-5` is a SHOULD and depends on A-5. If the connect-retry path cannot carry the fallback, the outcome is a ruled deferral, not a silent skip.
- The `.ci` files cover IPv4 unicast sessions. The OPEN and the iBGP verdict are family-independent, so the coverage is representative rather than exhaustive.
- Introducing the Bad Peer AS check may reject peerings that work today because their configuration was wrong and tolerated. That is a correctness improvement with an operational cost, and it needs a release note.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site |
|-----|---------|-------------|------|
| 7705 | 4.2 | the mechanism MUST be configurable per neighbour or per neighbour group | `internal/component/bgp/reactor/config.go` |
| 7705 | 4.2 | MUST accept an OPEN whose My AS is either the global or the local ASN | `internal/component/bgp/reactor/session_open_validation.go` |
| 7705 | 4.2 | MUST send its own OPEN using either ASN | `internal/component/bgp/reactor/session_negotiate.go` |
| 7705 | 4.2 | MUST treat UPDATEs on such a session as native iBGP | `internal/component/bgp/reactor/peer_settings.go` |
| 7705 | 4.2 | SHOULD send the global ASN first and fall back on Bad Peer AS | the connect-retry path, per A-5 |
| 4271 | 6.2 | OPEN Message Error, Bad Peer AS is subcode 2 | `internal/component/bgp/message/notification.go`, originated for the first time |
| 6793 | 4.2.2 | My AS carries AS_TRANS above 65535, the real value rides in the ASN4 capability | `internal/component/bgp/reactor/session_negotiate.go` |
| 6286 | 2.2 | the internal-peer determination for BGP Identifier validation | `internal/component/bgp/reactor/session_open_validation.go` |
| 4456 | 8 | reflection rules apply to a session treated as native iBGP | `internal/component/bgp/reactor/peer_forward_facts.go` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-bgp-as-migration.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-bgp-as-migration.md` only (commit A preserves the spec in history)
