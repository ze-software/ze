# Spec: bgp-as-migration -- RFC 7705 Section 4.2 Internal BGP AS Migration

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | `plan/immediate/spec-bgp-local-as-options.md` |
| Phase | - |
| Updated | 2026-10-09 |

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
five Section 3.3 requirements are `plan/immediate/spec-bgp-local-as-options.md`'s to prove;
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
- [ ] `rfc/full/rfc7705.txt` - AS migration mechanisms. The implementation summary is enrolled at `rfc/short/rfc7705.md`; this spec returned it from `rfc/pending/` on 2026-09-14.
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
- Enrolment is the exit condition, and it needs `plan/immediate/spec-bgp-local-as-options.md` landed first, because a row admits an RFC only when every gated MUST is classified.

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
- The summary at `rfc/short/rfc7705.md` supplies the requirement IDs; phase 8 returned it from `rfc/pending/` and enrolled it. `rfc/enrolled.txt` and `ai/RFC-REQUIREMENTS.md` are derived from its Meta table and are regenerated by `./le rfc index-update`.

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
| A-1 | Nothing in the tree currently rejects an OPEN on an AS mismatch, so introducing the check is a genuine behaviour change and not a duplicate. | `NotifyOpenBadPeerAS` (`internal/component/bgp/message/notification.go`) is originated nowhere, and `validateOpenIdentifier` (`internal/component/bgp/reactor/session_open_validation.go`) reads the AS only for the RFC 6286 determination. | The check already exists somewhere and this spec must extend it rather than add one. | Tree-wide grep for `NotifyOpenBadPeerAS` and for any comparison against `settings.PeerAS`, as the first implementation action. | confirmed 2026-10-09: the only send site of `NotifyOpenBadPeerAS` in the reactor is `rejectOpenPeerAS` (`session_open_as.go`), added by `11f0a65db2` with `ErrPeerASMismatch`; the only other reactor reference is the receive-side match in `noteASMigrationRejection`. No earlier check existed to extend |
| A-2 | The three iBGP determination sites can be routed through one rule without changing any existing verdict. | They are textually identical today: `internal/component/bgp/reactor/peer_settings.go`, `internal/component/bgp/reactor/peer.go`, `internal/component/bgp/reactor/session_validation.go`. | A site has a subtly different meaning and consolidating it changes behaviour for sessions that do not use the feature. | A refactor-only commit that unifies the three with no behaviour change, proven by the existing suites passing untouched. | confirmed 2026-10-09: every production iBGP verdict calls `isIBGPWith` (`session_as_migration.go`): `IsEBGP` (`peer_settings.go`), `session_validation.go`, `session_negotiate.go`, `peer_settings_negotiation.go`, `session_open_validation.go`, `session_next_hop.go`, `reactor_api.go`. `grep -rnE "LocalAS (!=\|==) .*PeerAS"` over non-test reactor code finds only a comment. `TestMigrationIBGPVerdictAgreesAcrossEverySite` pins the sites agreeing |
| A-3 | Introducing the Bad Peer AS check breaks no existing test or deployment, because a correctly configured peer advertises the AS it is configured with. | The configured `PeerAS` is what every session already assumes when deciding iBGP versus eBGP. | Sessions that work today start failing. That is a real operational risk and the reason the check lands in its own phase with its own `.ci`. | Running the full functional and interop suites after the check lands, before anything else in this spec. | confirmed 2026-10-09: the check landed in `11f0a65db2`; the test fixtures it reddened presented an AS their session was not configured for, and were corrected to present the configured one (the `configuredAS` field in `rfc7607_session_open_as_test.go` records why). No deployment-shaped `.ci` or interop scenario needed a change: the four new `.ci`, `bgp-local-as-options`, `bgp-local-as-inbound-untouched`, `dynamic-group-static-peer-wins` and interop `bgp-as-migration-local-as` (FRR, BIRD, GoBGP) pass. Refusals are now counted by `ze_bgp_open_rejected_bad_peer_as_total` |
| A-4 | A dynamic peer, whose `PeerAS` is 0 until establishment, must be exempt from the new check. | `validateOpenIdentifier` documents exactly this: the comment at `buildDynamicPeerSettings` (`internal/component/bgp/reactor/session_open_validation.go`) records that it sets `PeerAS` to 0 and that `resolveDynamicPeerSettings` fills it only at establishment. | Every dynamic peer is rejected at OPEN, which would be a severe regression. | A dedicated dynamic-peer test asserting the check is skipped when `PeerAS` is 0. | confirmed 2026-10-09: `peerASAccepted` (`session_as_migration.go`) carries an explicit dynamic-peer branch, and `TestOpenCheckSkippedForDynamicPeer` (`rfc7705_session_as_migration_test.go`) proves a dynamic peer advertising a real AS is accepted; every dynamic-group `.ci`, `dynamic-group-static-peer-wins` included, still establishes |
| A-5 | `RFC7705-4.2-5`'s fallback can be implemented within the existing connect-retry path without a new FSM state. | The retry already exists as a timer-driven reconnect; the change is which ASN the next OPEN carries. | The SHOULD is deferred with an explicit annotation rather than silently skipped, and that deferral is a compliance decision for Thomas. | A design spike on the connect-retry path before phase 5 starts. | confirmed 2026-10-09: `noteASMigrationRejection` (session_as_migration.go) toggles `Peer.asMigrationFallback` on NOTIFICATION 2/2 and `openLocalAS` reads it on the next connect-retry attempt; no FSM state was added. Proven by `TestMigrationFallbackOnBadPeerAS`, `test/plugin/bgp-as-migration-send-either.ci`, and interop `bgp-as-migration-local-as` against FRR |
| A-6 | Enrolling RFC 7705 requires `plan/immediate/spec-bgp-local-as-options.md` to have landed, because a row admits an RFC only when every gated MUST is classified. | `rfc/enrolled.txt` header: an enrolled RFC has every MUST-level requirement either covered by tagged tests or annotated. | The two specs must land together in one change, which enlarges the commit but does not change the work. | `./le rfc check` after both specs' tests exist. | confirmed 2026-09-14: enrolment waited for the Section 3.3 tags of the local-as-options spec, and both halves enrolled together; `rfc/enrolled.txt` carries the `rfc7705` row naming the Section 3.3 and 4.2 producers, and `./le rfc check` named no RFC 7705 violation (Progress, 2026-09-14) |

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
| Who else touches this path? | `plan/immediate/spec-bgp-local-as-options.md` owns the Section 3.3 half and must land first; the AS_PATH encoders moved under a resolver and that work has LANDED with the wire-edit-3 AS_PATH fold; it moves the AS_PATH encoders and consumes the iBGP verdict; `plan/spec-bgp-remote-as-auto.md` touches the same OPEN and session-establishment paths, and so does the initial-sync barrier `docs/architecture/api/architecture.md` describes (`spec-bgp-session-ready-contract`, closed 2026-09-05). |

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
| Prometheus counters/metrics | Yes | `ze_bgp_open_rejected_bad_peer_as_total` (the `ze_` prefix the naming convention in `docs/plugin-development/metrics.md` requires), labelled by peer, so the tightening in R-1 is observable rather than only logged |
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
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md`, Session Lifecycle, for the rejection counter: that page is the metric catalogue, while `docs/plugin-development/metrics.md` holds only the naming convention |
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
8. **Phase: unpark, enrol and close the ledger** -- BLOCKING, requires `plan/immediate/spec-bgp-local-as-options.md` landed
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

- RFC 7705 Section 3.3 is out of scope here; `plan/immediate/spec-bgp-local-as-options.md` owns it and must land first for enrolment to be possible.
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
- [ ] Learned summary written to `plan/learned/NNN-bgp-as-migration.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/immediate/spec-bgp-as-migration.md` only (commit A preserves the spec in history)

## Progress, 2026-09-06

Committed in `11f0a65db2`. Enrolment is not reached, and `RFC7705-3.3-9` is an
unimplemented SHOULD.

## Progress, 2026-09-14 -- phase 8 done

Enrolment is reached. The summary moved from `rfc/pending/rfc7705.md` to
`rfc/short/rfc7705.md` with `| Enrolment | enrolled |` and a `bgp-base 265`
public row, `rfc/pending/` is gone, and the extraction sign-off is
`rfc/extraction/rfc7705.json`: 12 derived sites under the `prose` register, 8
mapped and 4 excluded `not-a-requirement`, with `RFC7705-3.3-3` declared
`unsourced-ids` on section 3.3 because the splitter fuses its sentence with
`RFC7705-3.3-2`'s.

All nine gated MUSTs now carry an `RFC requirement:` tag in both polarities, and
each of the 18 tags carries a verified discrimination record in
`rfc/discrimination/rfc7705.json`. The tags added here are `RFC7705-3.3-1` on
`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`
(`internal/component/bgp/config/peers_test.go`), `RFC7705-4.2-1` on
`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`
(`internal/component/bgp/config/as_migration_test.go`), and `RFC7705-3.3-2`
through `-3.3-5` on the two forward-rail tests in
`internal/component/bgp/reactor/rfc7705_local_as_test.go`. `RFC7705-4.2-2`,
`-4.2-3` and `-4.2-4` were already tagged and gained their records here.

`./le rfc check` names no RFC 7705 violation. `docs/features/rfc-status.md` reads
`9 gated: 9 proven, 0 annotated, 0 untested`.

Remaining for this spec: AC-12 is met, and the `.ci` and interop rows of phases 3,
5, 6 and 9 are untouched by this pass.

## Progress, 2026-10-09 -- functional and interop proof

Landed in `ee15e53be9`. All four planned `.ci` files exist and pass
(`./le test bgp plugin`, 7/7 with the two local-as files and
`dynamic-group-static-peer-wins`):

| File | AC | Forced red (break in `session_as_migration.go`, then restored) |
|------|----|------------------------------------------------------------------|
| `test/plugin/bgp-open-bad-peer-as.ci` | AC-1 | `peerASAccepted` last arm `return true`: NOTIFICATION 2/2 never arrives (timeout) |
| `test/plugin/bgp-as-migration-accept-either.ci` | AC-4, AC-5 | migration arm compares `PeerAS` only: ze sends NOTIFICATION 2/2 instead of the UPDATE |
| `test/plugin/bgp-as-migration-ibgp-treatment.ci` | AC-7 | `isIBGPWith` ignores `MigrationAS`: UPDATE arrives with AS_PATH [65000] and no LOCAL_PREF |
| `test/plugin/bgp-as-migration-send-either.ci` | AC-6, AC-9 agreement, AC-10 | `noteASMigrationRejection` returns at once: conn=2 OPEN carries 0xFDE8 again |

AC-3 (dynamic exemption) is carried by every dynamic-group `.ci`; 
`dynamic-group-static-peer-wins.ci` passed in the same run.

Interop: one scenario, `test/interop/scenarios/bgp-as-migration-local-as`
(checker `internal/le/interoplab/bgp/checkers.go`), FRR + BIRD + GoBGP. FRR is
AS 65002 and refuses ze's first OPEN (65001) with Bad Peer AS; ze's fallback
OPEN carries the migration AS 65002 and the session establishes as iBGP, with
10.77.5.0/24 at FRR carrying no 65001 in its path. Green twice on 2026-10-09
(`interop: 1 passed, 0 failed`). The two `NN-*` rows of the Interop Tests table
are replaced by this one scenario: BIRD's Bad Peer AS report is covered on the
FRR leg, where Bad Peer AS is what drives the fallback.

AC-10 settled as implemented, not deferred: RFC 7705 Section 4.2 reads "the
speaker SHOULD send BGP OPEN using the globally configured ASN first, and only
send a BGP OPEN using the locally configured ASN as a fallback if the remote
neighbor responds with the BGP error "Bad Peer AS"." Ze does exactly that, and
with Ze on both ends `peerASAccepted` accepts either AS from a migrating peer,
so no OPEN pairing is refused. A-5 is confirmed above.

Still open before `/ze-close`: the integration checklist row promising a
`bgp_open_rejected_bad_peer_as_total` counter has no producer (`grep -rn
bad_peer_as internal` finds none), nor its `docs/plugin-development/metrics.md`
entry; A-1 through A-4 and A-6 still read `unvalidated` in the table though the
code and tests answer them; the closure template, Review Gate and the two
closure commits.

Since then: the counter landed in `9a783621b7` (`openBadPeerAS`,
`reactor_metrics.go`; incremented in `rejectOpenPeerAS`, `session_open_as.go`;
catalogued in `docs/guide/monitoring.md`, the metric catalogue, while
`docs/plugin-development/metrics.md` holds only the naming convention), and
A-1 to A-4 and A-6 are confirmed in the table above.

## Implementation Summary

### What Was Implemented
- `11f0a65db2`: the Bad Peer AS check (`validateOpenPeerAS`, `rejectOpenPeerAS`, `session_open_as.go`) on both OPEN rails (`handleOpen`, `processOpen`); the `session { asn { migration } }` leaf (`ze-bgp-conf.yang`, parsed by `parsePeerSettings` and checked by `setMigrationAS`); the one iBGP rule `isIBGPWith`; `peerASAccepted`; `openLocalAS` feeding header, ASN4 capability and encoder from one value; the RFC 7705 Section 4.2 fallback `noteASMigrationRejection` over `Peer.asMigrationFallback`.
- `45fc0acdff`, `e476c99b84`: unit tests at both entry points (`internal/component/bgp/config/rfc7705_as_migration_test.go`, `internal/component/bgp/reactor/rfc7705_session_as_migration_test.go`).
- `af10938607`: enrolment (`rfc/short/rfc7705.md`, `rfc/extraction/rfc7705.json`, `rfc/discrimination/rfc7705.json`).
- `ee15e53be9`: the four `.ci` files and interop scenario `bgp-as-migration-local-as` (FRR, BIRD, GoBGP).
- `9a783621b7`: `ze_bgp_open_rejected_bad_peer_as_total` and `TestOpenBadPeerASCounted`.
- Closure: stale comment on `peerASAccepted` corrected; three pages repaired (below).

### Bugs Found/Fixed
- `peerASAccepted`'s comment named `openClaimsASZero`, which does not exist, and `openAdvertisedAS` as the reader of the advertised AS; `validateOpenPeerAS` does both. Corrected in closure (comment only).
- The bgp/config red recorded in `plan/journal/gate-red-where-nothing-blocks-on-it.md` was an untagged `go test` in an export, not a defect. Under the gate's tags (`ze_core` plus `feature-gates.txt`) the package passes, `TestPeersFromConfigTree_ASMigrationPerNeighborGroup` included. The row now says so.

### Documentation Updates
- `docs/architecture/behavior/fsm-open-sent.md`, "What `handleOpen` actually validates": the ordered list lacked the peer AS check; step 4 added (anchors `validateOpenPeerAS`, `rejectOpenPeerAS`, `peerASAccepted`), later steps renumbered.
- `docs/features/bgp-protocol.md`: new section "Peer AS check and AS migration (RFC 7705)" with source anchors.
- `docs/comparison.md`: row "AS migration: local-as and iBGP dual AS (RFC 7705)", Ze Yes, other daemons `?` (not verified per daemon).
- Earlier commits: `docs/guide/configuration.md` "Internal AS Migration (RFC 7705 Section 4.2)", `docs/guide/monitoring.md` counter, `docs/features/rfc-status.md` (generated) RFC 7705 row `9 gated: 9 proven`.
- `./le doc check verify`: the only failing stage is the wiki command-catalog drift (`../wiki/command-catalog.md`), unrelated to this spec; no anchor finding names a page edited here.

### Deviations from Plan
- Test files are `rfc7705_*_test.go`, not the names in Files to Create; the planned test names map as in the Audit below.
- The two `NN-*` interop rows became one named scenario, `bgp-as-migration-local-as` (Progress, 2026-10-09).
- Core-design checklist row 12: `docs/architecture/core-design.md` does not describe OPEN validation; the page that does is `fsm-open-sent.md`, which was repaired instead.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A closure agent ran the bgp/config tests untagged in an export and recorded a red | The package is green under the gate's tags; `docs/contributing/running-commands.md` names this phantom red | Re-run under `ze_core` plus feature tags | Journal row corrected |
| approach | Docs checklist rows 1, 11, 12 answered Yes with no page edit | Feature page, comparison row and the FSM validation list were missing | Closure doc review | Pages repaired in commit A |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC7705-4.2-1 configurable per neighbour or group | Done | `config.go` `parsePeerSettings`, `ze-bgp-conf.yang` leaf `migration` | |
| RFC7705-4.2-2 accept either ASN | Done | `session_as_migration.go` `peerASAccepted` | |
| RFC7705-4.2-3 send with either ASN | Done | `session_negotiate.go` via `openLocalAS` | |
| RFC7705-4.2-4 native iBGP | Done | `session_as_migration.go` `isIBGPWith` | |
| Bad Peer AS originated | Done | `session_open_as.go` `rejectOpenPeerAS` | |
| Enrol RFC 7705 | Done | `rfc/enrolled.txt`, `rfc/short/rfc7705.md` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/plugin/bgp-open-bad-peer-as.ci`, `TestOpenBadPeerASCounted`, RFC7705-4.2-2 negative in `rfc7705_reactor_b_test.go` | |
| AC-2 | Done | `TestOpenBadPeerASCounted` "the configured AS" | |
| AC-3 | Done | `TestOpenCheckSkippedForDynamicPeer`, `dynamic-group-static-peer-wins.ci` | |
| AC-4, AC-5 | Done | `TestMigrationAcceptsEitherASN`, `bgp-as-migration-accept-either.ci` | |
| AC-6, AC-9 | Done | `TestMigrationOpenCarriesResolvedASN`, `bgp-as-migration-send-either.ci` | |
| AC-7 | Done | `TestMigrationSessionIsIBGP`, `rfc7705_live_behavior_test.go` 4.2-4 tags, `bgp-as-migration-ibgp-treatment.ci` | |
| AC-8 | Done | `TestPeersFromConfigTree_ASMigrationPerNeighborGroup` | green under gate tags 2026-10-09 |
| AC-10 | Done | `TestMigrationFallbackOnBadPeerAS`, send-either `.ci`, interop FRR leg | |
| AC-11 | Done | `TestMigrationFallbackIgnoredWithoutTheMechanism`, 4.2-3 negative polarity | |
| AC-12 | Done | `./le rfc check` names no RFC 7705 violation (2026-10-09) | its exit 2 is other RFCs |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOpenRejectedOnBadPeerAS`, `TestOpenAcceptedOnMatchingAS` | Changed | `TestOpenBadPeerASCounted`, `rfc7705_reactor_b_test.go` | |
| `TestOpenCheckSkippedForDynamicPeer` | Done | `rfc7705_session_as_migration_test.go` | |
| `TestMigrationAcceptsGlobalASN`, `...AlternateASN` | Changed | `TestMigrationAcceptsEitherASN` | one table, both polarities |
| `TestMigrationOpenCarriesResolvedASN` | Done | same file | |
| `TestMigrationSessionIsIBGP` | Done | same file | |
| `TestMigrationNoEBGPPrepend` | Changed | `rfc7705_live_behavior_test.go`, ibgp-treatment `.ci` | |
| `TestMigrationLeafPerNeighborGroup` | Changed | `TestPeersFromConfigTree_ASMigrationPerNeighborGroup` | |
| `TestIBGPVerdictSingleRule` | Changed | `TestMigrationIBGPVerdictAgreesAcrossEverySite` | |
| `TestMigrationFallbackOnBadPeerAS` | Done | same file | |
| `TestNoMigrationConfigUnchanged` | Changed | `TestMigrationFallbackIgnoredWithoutTheMechanism` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `session_as_migration.go` | Done | |
| `session_as_migration_test.go` | Changed | `rfc7705_session_as_migration_test.go` |
| four `.ci` files | Done | |
| `peer.go`, `peer_settings.go`, `session_validation.go`, `session_open_validation.go`, `session_negotiate.go`, `config.go`, YANG | Done | `11f0a65db2` |

### Audit Summary
- **Total items:** 32
- **Done:** 24
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 8 (test and file names, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Section 4.2 works against other implementations | interop | `test/interop/scenarios/bgp-as-migration-local-as`: FRR refuses ze's first OPEN with Bad Peer AS, ze reopens with the migration AS, session establishes iBGP; green twice 2026-10-09; forced red with the fallback broken (FRR shows 6 NOTIFICATIONs, Idle) |
| User path end to end | functional | four `.ci` files, each forced red by a break in `session_as_migration.go` (Progress, 2026-10-09) |
| Four gated MUSTs proven in both polarities | RFC ledger | `docs/features/rfc-status.md` RFC 7705 `9 gated: 9 proven`; `rfc/discrimination/rfc7705.json` |
| RFC 7705 enrolled | file | `rfc/enrolled.txt` `rfc7705` row |
| Tightening observable | metric | `ze_bgp_open_rejected_bad_peer_as_total`, `TestOpenBadPeerASCounted` (forced red in an export) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC and checklist row is met | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-as-migration-12d06ccf-2460-42c7-a707-30bb0a427796.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring, logic and RFC 7705 Section 4.2 text, security (attacker-supplied AS, fallback bound), stale comments, documentation drift |

Round 1 found one ISSUE and three documentation gaps; round 2, over the fixes, found 0 BLOCKER and 0 ISSUE.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Comment on `peerASAccepted` names a nonexistent `openClaimsASZero` and the wrong reader of the advertised AS | `session_as_migration.go` | comment corrected |
| 2 | ISSUE | `fsm-open-sent.md` validation order omits the peer AS check | `docs/architecture/behavior/fsm-open-sent.md` | step 4 added |
| 3 | ISSUE | Feature page and comparison carry no RFC 7705 entry though checklist rows 1 and 11 say Yes | `docs/features/bgp-protocol.md`, `docs/comparison.md` | section and row added |

NOTE: `Peer.asMigrationFallback` is never reset when a session establishes, so after a drop the next OPEN uses whichever AS last succeeded. RFC 7705 Section 4.2's "SHOULD send BGP OPEN using the globally configured ASN first" governs the first attempt, which ze meets; reopening with the AS the peer last accepted cannot deadlock, since the toggle still walks both.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `session_as_migration.go`, `rfc7705_session_as_migration_test.go` | yes | `grep -n "^func Test"` lists 9 tests in the test file |
| `test/plugin/bgp-open-bad-peer-as.ci`, `bgp-as-migration-{accept-either,send-either,ibgp-treatment}.ci` | yes | `ee15e53be9` stat |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-8 | per-group leaf with peer override | `go test -tags "ze_core <feature-gates>" -run TestPeersFromConfigTree_ASMigrationPerNeighborGroup ./internal/component/bgp/config/`: PASS; whole package `ok` (58s) |
| AC-1..AC-7, AC-9..AC-11 | reactor behaviour | `go test -tags ... -run 'Migration|BadPeerAS|PeerAS|DynamicPeer|IBGP' ./internal/component/bgp/reactor/`: `ok` 21.8s |
| AC-12 | no RFC 7705 violation | `./le rfc check` output has no `RFC7705` line |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| OPEN with unconfigured AS | `bgp-open-bad-peer-as.ci` | forced red recorded (Progress, 2026-10-09) |
| migration peer, either ASN | `bgp-as-migration-accept-either.ci` | forced red recorded |
| our OPEN toward migration peer | `bgp-as-migration-send-either.ci` | forced red recorded |
| routes on alternate-ASN session | `bgp-as-migration-ibgp-treatment.ci` | forced red recorded |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-6 | confirmed | Assumptions table above, one evidence cell each |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| feature page, comparison, FSM validation order | `validateOpenPeerAS`, `peerASAccepted`, `noteASMigrationRejection`, `openLocalAS` read in closure | yes |
| config guide, monitoring | `parsePeerSettings` "migration", `openBadPeerAS` | yes |
| `./le doc check verify` | only the unrelated wiki catalog drift fails | yes |
