# Spec: ike-multiple-authentication

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | - (RFC 4739 is reached only by the vpn-concentrator profile, which is not a release profile; Thomas brought it into scope on 2026-10-30) |
| RFC gaps | RFC4739-3.1-1, RFC4739-3.2-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 4739 lets an IKEv2 peer authenticate more than one set of credentials in a
chain of IKE_AUTH exchanges: for example a machine certificate followed by a
user credential, or one credential for the access provider and one for the
third party behind it. Support is announced with MULTIPLE_AUTH_SUPPORTED
(16404) and each further round is announced with ANOTHER_AUTH_FOLLOWS (16405).

Ze implements none of it today. `internal/component/ike/wire/payload_notify.go`
declares neither type, no engine path sends or reads either, and IKE_AUTH
completes after one authentication per side. Both MUST-level rows of
`rfc/short/rfc4739.md` (RFC4739-3.1-1, RFC4739-3.2-1) are `{gap}`. Thomas
brought the RFC into scope on 2026-10-30 and asked for 100% proven mandatory
coverage.

Goal: a per-peer, opt-in second authentication round, by certificate or
pre-shared secret, for each direction of a site-to-site peer, in both the
initiator and responder roles. It must interoperate with strongSwan 6.1.0
(the interop lab's pinned release). Both rows then carry tagged tests whose
recorded breaks go red.

**Approval:** Thomas approved this design on 2026-10-30 (relayed by the
coordinating session), including four decisions: (1) the design as written;
(2) one extra round per direction; (3) a local round configured when the peer
never sent MULTIPLE_AUTH_SUPPORTED is skipped with a warning (AC-9);
(4) ANOTHER_AUTH_FOLLOWS with no remote round configured is refused with
AUTHENTICATION_FAILED (AC-7). The spec was not claimed through
`./le spec claim`: the session marker is held by another spec. The owning
agent is recorded in the per-spec state file under
`tmp/session/2026-10-30-bc566aa5-3f69-415d-8c8b-c6f8cdf8adae/state/`.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - the IKE component's boundaries and gates
  → Constraint: `ike` is gated by `ze_ike`; the change stays inside `ike/wire`, `ike/engine` and `ike/ipsec`, and readers outside the engine see IKE state only through `ipsecinventory`/`ikeprobe`.
  → Constraint: Trap "a datagram that fails to decrypt kills a live SA" is open on the IKE_AUTH paths; the new rounds reuse the same decrypt path and must not widen it (no new `StateDead` on a decrypt failure beyond what the first round already does).
- [ ] `ai/digests/ipsec-ike.md` - current IKE_SA_INIT / IKE_AUTH flow, both roles
  → Constraint: the initiator's IKE_AUTH response is handled by `handleAuthResponse` (`engine/fsm.go`), which verifies AUTH, adopts the Child SA from SAr2/TSi/TSr and sets `StateEstablished`; the multi-round path must reach the same tail exactly once, on the final response.
  → Constraint: the responder's IKE_AUTH request is handled by `handleAuthRequest` (`engine/responder.go`); the EAP path already keeps the Child SA offer across rounds (`sa.eapTSi`, `sa.eapTSr`, `sa.eapChildRefusal` in `engine/sa.go`, set in `startResponderEAP`). The multi-round path follows the same "decide once, answer on the final response" pattern.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-14-responder.md` - engine and responder design
  → Decision: the per-peer goroutine (`PeerSession`) and the shared dispatch goroutine before adoption stay as they are; the new rounds run on the same goroutines as the EAP rounds (pre-adoption dispatch on the responder, `runInitiator` wait loop on the initiator).
- [ ] `docs/architecture/testing/interop.md` - IPsec interop suite
  → Constraint: scenarios live in `test/interop-ipsec/scenarios/<name>/` (`ze.conf`, `swanctl.conf`), are named (no numeric prefix), and each has a checker in `scenarioCheckers` (`internal/le/interoplab/ipsec/checkers.go`). Run with `./le test integration interop-ipsec` and `IPSEC_INTEROP_SCENARIO=<name>`.
  → Constraint: a vacuity proof is owed: revert the engine change, rebuild the Ze image the lab drives, show the scenario red, restore, show green.
- [ ] `docs/contributing/rfc-conformance-gates.md` - tags, discrimination records, enrolment
  → Constraint: each tagged unit needs a discrimination record written by `./le rfc record` in the same change; the claim states only what the test body checks.
  → Decision: "Bringing an optional RFC into scope is moving its name from one row to the other" (use-case profiles section), so vpn-concentrator moves from `Optional use cases` to `Use cases` in `rfc/short/rfc4739.md`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4739.md` and `rfc/full/rfc4739.txt` - the extension
  → Constraint: §2.1 "The peers announce support for this IKEv2 extension by including a MULTIPLE_AUTH_SUPPORTED notification in the IKE_SA_INIT response (responder) and the first IKE_AUTH request (initiator)."
  → Constraint: §2.1 "either of them can announce that it wishes to have a second authentication by including an ANOTHER_AUTH_FOLLOWS notification in any IKE_AUTH message that contains an AUTH payload." AAF is only put in a message that carries AUTH.
  → Constraint: §2.1 "The next IKE_AUTH message sent by this peer will contain a second identity payload (IDi or IDr) and starts another authentication exchange. The IKE_AUTH phase is considered successful only if all the individual authentication exchanges complete successfully."
  → Constraint: §2.1 "The AUTH payloads are calculated as specified in [IKEv2] Sections 2.15 and 2.16, where IDi' refers to the latest IDi payload sent by the initiator, and IDr' refers to the latest IDr payload sent by the responder." The same IKE_SA_INIT message and nonce, with the prf over the current round's ID.
  → Constraint: §3.1 and §3.2 "The Protocol ID and SPI Size fields MUST be set to zero, and there is no data associated with this Notify type." These are the two gated rows.
  → Constraint: §2.4 and §2.5 message shapes: an initiator's second round is "HDR, SK { IDi, [CERT+], AUTH }"; a responder's second round is answered by an initiator "HDR, SK { }"; SA, TSi and TSr come in the final response only.
- [ ] `rfc/short/rfc7296.md` §2.15, §2.16, §2.21.2, §3.10.1 - AUTH octets, EAP, error handling, notify
  → Constraint: §2.21.2: an authentication failure found by the responder is answered with AUTHENTICATION_FAILED in the IKE_AUTH response; one found by the initiator is reported in a separate INFORMATIONAL (existing `sendIKESATeardown`).
  → Constraint: §3.10.1: an unrecognised status notify "MUST be ignored if not recognized", so a peer that does not support RFC 4739 ignores Ze's MULTIPLE_AUTH_SUPPORTED; announcing it is safe with any peer.

### Reference implementation (strongSwan 6.1.0 source, read 2026-10-30)
- [ ] `src/libcharon/sa/ikev2/tasks/ike_auth.c` (tarball sha256 matches `test/interop-ipsec/Dockerfile.strongswan`)
  → Constraint: `multiple_auth_enabled()` reads `"%s.multiple_authentication", TRUE`: the extension is ON by default, so the interop scenarios need no charon setting.
  → Constraint: `build_r` (IKE_SA_INIT) adds MULTIPLE_AUTH_SUPPORTED with `chunk_empty`; `build_i` adds it in the first IKE_AUTH only when the IKE_SA_INIT response carried it. `message.c:add_notify` builds a zeroed notify, so Protocol ID 0 and SPI size 0.
  → Constraint: `do_another_auth()` returns FALSE unless the extension was negotiated, so strongSwan with a `local-2` round and a peer that did not announce support just skips the round. Ze takes the same reading for its own local rounds.
  → Constraint: `process_r` keeps expecting rounds while ANOTHER_AUTH_FOLLOWS is present. A completed round beyond the configured ones fails `update_cfg_candidates` ("insufficient authentication rounds"), and the responder answers AUTHENTICATION_FAILED (`peer_auth_failed`). A configured round that never happened fails the strict check at the end, with the same answer. Ze matches both.
  → Constraint: `child_create.c` sends SA/TSi/TSr in the first IKE_AUTH request only and answers them only once `COND_AUTHENTICATED` holds, which is after the last round.
  → Constraint: a strongSwan initiator requires IDr in the first response of each new responder round (`process_i`, "IDr payload missing"), and a strongSwan initiator with no rounds left answers a responder's ANOTHER_AUTH_FOLLOWS with an empty IKE_AUTH request.
  → Constraint: each round's AUTH uses the current round's ID (`set_my_id` / `set_other_id` per round; `keymat_v2.c:get_auth_octets`), confirming §2.1.
  → Constraint: on receive strongSwan does not check Protocol ID, SPI size or data of either notify (`notify_payload.c` verify falls to `default`). Ze is equally liberal on receive; the MUST binds the sender.
  → Constraint: swanctl config: a round is a `local<suffix>` or `remote<suffix>` section (`swanctl.opt`, `vici_config.c:peer_sn` prefix match), for example `local-2 { auth = psk; id = ... }`. A PSK is selected by the round's own ids (`psk_authenticator.c`), so `secrets` need `id-*` entries naming the round-2 identities.

**Key insights:**
- Rounds are positional. The initiator's round k goes in IKE_AUTH request k, the responder's round k in response k. A side with no round left sends a message with no ID and no AUTH. SA/TSi/TSr come in the first request and the final response.
- Every round signs the same IKE_SA_INIT octets with the prf over the CURRENT round's ID payload. The credential readers in `auth.go`, `cert_payload.go` and `remote_id.go` read `sa.PeerCfg.Auth` directly today, so they need a "current round" accessor.
- MULTIPLE_AUTH_SUPPORTED is safe to send to any peer (RFC 7296 §3.10.1 ignore rule), but Ze sends it only for a peer that configures a second round, as decided with Thomas.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/wire/payload_notify.go` - notify type constants and the `notifyTypeNames` registry (single source for recognition and log names); neither 16404 nor 16405 is present, so both are logged as ignored unrecognised status notifies.
- [ ] `internal/component/ike/engine/auth.go` - `buildAuthRequest` (IDi, AUTH or EAP signal, CERT, INITIAL_CONTACT, SAi2, TSi, TSr, MOBIKE and NAT notifies), `computeSignedOctets` (role-relative; local ID from `buildIDPayload`, remote ID from `sa.RemoteIDPayload`), `computeLocalAuth`/`computePSKAuth`/`computeX509Auth`, `verifyRemoteAuth` (calls `checkRemoteIdentity`, then verifies by AUTH method), `buildIDPayload` (reads `sa.PeerCfg.Auth.LocalID`).
- [ ] `internal/component/ike/engine/fsm.go` - `handleSAInitResponse` (initiator; reads the IKE_SA_INIT response notifies), `handleAuthResponse` (initiator; collects payloads, verifies AUTH, adopts Child SA, establishes or starts EAP), `startEAPExchange`/`handleEAPResponse` (the multi-message precedent on the initiator), `handleInbound` (state switch).
- [ ] `internal/component/ike/engine/responder.go` - `handleResponderInbound` (state switch: `StateSAInitReceived` → `handleAuthRequest`, `StateEAPInProgress` → replay or `handleResponderEAP`), `buildSAInitResponse`, `handleAuthRequest` (collects payloads, verifies AUTH, `buildAuthResponse`, `finishResponderEstablish`), `buildAuthIdentityPayloads`, `selectAuthChildSA`, `sendAuthFailed`, `replayCachedResponse`.
- [ ] `internal/component/ike/engine/responder_eap.go` - `startResponderEAP` stores the Child SA decision on the SA for the final response; it is the pattern for "answer SA/TS on the last response".
- [ ] `internal/component/ike/engine/sa.go` - `SAState` enum (`StateIdle` … `StateDead`) with a String switch; `RemoteIDPayload`; EAP carry fields.
- [ ] `internal/component/ike/ipsec/types.go` - `AuthConfig` (one struct carrying both local fields: LocalID, Certificate, PSK; and remote fields: RemoteID, RemoteIDType, CACertificate); `AuthConfig.Equal` is `reflect.DeepEqual`, so a new pointer field is compared deeply with no list to edit; `SiteToSitePeer.Equal` drives reload.
- [ ] `internal/component/ike/ipsec/config.go` - `parseAuthConfig`, `parsePreSharedSecret`, `readCertificateRefs`; `config_auth_policy.go` - `parseIdentityPolicy`, `parseCertificatePolicy`.
- [ ] `internal/component/ike/ipsec/validate.go` - `ValidateIdentities`, `ValidatePKIRefs`, `ValidateCertificateChains`, `IsEAPMode`; called from `validateIPsecSections` (`engine/config.go`) at commit.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `site-to-site peer <name> authentication` container (mode, pre-shared-secret, pre-shared-secret-encoding, local-id, remote-id, remote-id-type, x509 certificate/ca-certificate, EAP leaves).
- [ ] `internal/le/interoplab/ipsec/checkers.go` - `scenarioCheckers` map and `checkPSKSiteToSite` (establish, XFRM on both sides, traffic).

**Behavior to preserve:**
- A peer with no second round configured sends byte-identical IKE_SA_INIT responses and IKE_AUTH requests and responses to today's: no MULTIPLE_AUTH_SUPPORTED, no ANOTHER_AUTH_FOLLOWS.
- A peer with no second round configured that receives ANOTHER_AUTH_FOLLOWS now refuses it (see AC-7). Today the notify is ignored and the exchange completes after one round. That is a change, and it is deliberate: completing would accept an authentication the peer said is unfinished.
- First-round AUTH computation, INITIAL_CONTACT placement (first request only), CERTREQ, hash-and-URL, MOBIKE, NAT, transport mode, CP, childless handling, EAP rounds and all existing log lines stay as they are.
- `AuthConfig.Equal` semantics: a change to a second-round leaf is a peer config change that restarts the peer, the same as a first-round leaf.

**Behavior to change:**
- New opt-in config for a second authentication round per direction, with commit-time validation.
- Both roles run a second round when configured, announce and recognise the two notifies, and refuse unconfigured or missing rounds with AUTHENTICATION_FAILED.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `vpn ipsec site-to-site peer <name> authentication second-round { local { ... } remote { ... } }`, delivered to the `ike` plugin as SDK JSON on `OnConfigure` / verify.
- Wire: IKE_SA_INIT response and IKE_AUTH messages from the peer (UDP 500/4500).

### Transformation Path
1. YANG accepts the new container; `parseAuthConfig` builds a second-round value on `AuthConfig`; `validateIPsecSections` refuses invalid combinations at commit.
2. `reconcilePeers` restarts a peer whose `AuthConfig` changed (unchanged mechanism).
3. Responder: `buildSAInitResponse` adds MULTIPLE_AUTH_SUPPORTED when the peer has a second round. `handleAuthRequest` records the peer's notifies, verifies round 1, then either finishes (today's path) or answers the round and moves the SA to the multi-round state, carrying the Child SA offer. `handleResponderInbound` routes later IKE_AUTH requests in that state to the round handler, with cached-response replay.
4. Initiator: `handleSAInitResponse` records whether the responder announced support. `buildAuthRequest` adds the two notifies. `handleAuthResponse` verifies the responder's round, then sends the next round (or an empty IKE_AUTH) or, on the final response, runs today's Child SA adoption and establishment tail.
5. AUTH for round 2 is computed and verified by the existing functions, which read the current round's credential through one accessor per direction.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → ike plugin | SDK JSON config sections → `config.Tree` → `ipsec.AuthConfig` | No |
| Ze ↔ IKE peer | IKE_SA_INIT / IKE_AUTH wire messages | No |

### Integration Points
- `wire.notifyTypeNames` - the two new types register beside their constants (registry, no other list).
- `ipsec.AuthConfig` - gains one pointer field to the second-round values (keeps `SiteToSitePeer` under the hugeParam threshold, as `IntermediateCertificates` does).
- `SA` - gains round bookkeeping: whether the peer announced support, the current local and remote round, whether the peer announced another round, and the carried Child SA offer on the responder.
- Credential readers (`buildIDPayload`, `computePSKAuth`, `computeX509Auth`, `buildCertPayloads`, `checkRemoteIdentity`, `verifyPSKAuth`, `verifyX509Auth`, the CA lookups in `cert_payload.go`) - read the current round's credential through `sa` accessors instead of `sa.PeerCfg.Auth` directly.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Rounds go through the existing decrypt, collect, verify and build functions; no second AUTH implementation |
| No unintended coupling (components stay isolated) | Yes (design) | All changes inside `ike/wire`, `ike/engine`, `ike/ipsec` and the interop lab |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | AUTH compute and verify are reused, parameterised by round; config parse reuses `parsePreSharedSecret`, `parseIdentityPolicy`, `readCertificateRefs` |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | Notifies are empty; message building uses the existing `buildEncryptedMessage` |
| Registration over hardcoding, outbound | Yes (design) | New notify types enter `notifyTypeNames`, the existing registry; the interop scenarios enter `scenarioCheckers`, the suite's existing registry |
| Registration over hardcoding, inbound | To verify at implementation | Lists searched: `notifyTypeNames` (wire); `SAState` String switch and every `switch sa.State` (`fsm.go` `handleInbound` and `runResponder`, `responder.go` `handleResponderInbound`); `AuthMode` switches (`computeLocalAuth`, `IsEAPMode`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | strongSwan 6.1.0 enables RFC 4739 by default | `ike_auth.c:multiple_auth_enabled()` reads the option with default TRUE (read 2026-10-30) | Scenarios need a `strongswan.conf` setting | Interop scenarios pass with no charon setting | validated (source) |
| A-2 | strongSwan's final response carries SA/TSi/TSr only after its last round, and its initiator sends an empty IKE_AUTH when it has no rounds left | `child_create.c` `build_r` COND_AUTHENTICATED wait; `ike_auth.c` `build_i` early return | Ze's round handler would misplace the Child SA | Both interop scenarios (two rounds each way; the empty-request half is proven Ze to Ze by `TestRFC4739NonFinalResponseDoesNotEstablish`) | validated: charon's final response carried SA/TSi/TSr after its last round in `multiple-auth-ze-initiator` (pass, 14s) |
| A-3 | strongSwan selects a round-2 PSK by the round-2 identities in `secrets` (`id-*`) | `psk_authenticator.c` `get_shared(SHARED_IKE, my_id, other_id)` | Scenario config needs another secrets shape | Interop scenario establishes | validated: both scenarios pass with one round-2 secret owned by both round-2 identities |
| A-4 | The existing responder cached-response replay (`replayCachedResponse`) and the initiator retransmission loop serve a multi-round exchange without change, apart from the new state | EAP rounds already use both (`responder.go` `StateEAPInProgress`, `fsm.go` `runInitiator` retransmit) | A lost round-2 datagram kills the handshake | Unit test retransmitting a round-2 request answers it from the cache | validated: `TestRFC4739RoundTwoRetransmitReplayed` (both requests replayed byte for byte, never reprocessed) |
| A-5 | Every reader of a credential in the AUTH path can be routed through two accessors (local round, remote round) without changing round-1 results | Readers counted: `auth.go` 11, `cert_payload.go` 11, `remote_id.go` 2 reads of `sa.PeerCfg.Auth` | A round-2 AUTH signs with the round-1 key or checks the round-1 CA | Unit tests where round 1 and round 2 use different keys and CAs, each refused when crossed | validated: `TestRFC4739RoundTwoCredentialNotCrossed`, `TestRFC4739RoundTwoAuthUsesLatestID`, x509 subtest of `TestRFC4739ZeToZeTwoRoundsEachWay` |
| A-6 | Thomas's design brief (per-peer opt-in, cert or PSK rounds first, announce only when configured, refuse an unconfigured remote round with AUTHENTICATION_FAILED, strongSwan interop) is the agreed starting design | Coordinator message relaying Thomas's 2026-10-30 decision | Design changes after approval | Approval gate (Step 4) | validated: Thomas approved 2026-10-30 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The RFC 4754 and RFC 7427 agents edit `auth.go` (signature and ECDSA AUTH) at the same time, and the accessor change touches the same functions | `git diff internal/component/ike/engine/auth.go` shows foreign hunks next to the planned edits | Land the accessor refactor as its own small commit through `hunks <path> <patch>`; re-read the file right before each edit; never touch their hunks |
| R-2 | A new `SAState` value misses one of the state switches and falls into its `panic("BUG ...")` default | Exhaustive lint, `TestSAStateString`-style tests | Add the state to every switch found by grep; a unit test sends a round-2 message to each handler |
| R-3 | Round-2 AUTH is computed over the round-1 ID, so it interoperates with Ze but not with strongSwan | Interop scenario refuses round 2 | Unit test computes round-2 AUTH by hand from IDx' (round 2) and compares; interop proves it against strongSwan |
| R-4 | The initiator establishes on a non-final response (one carrying AAF), installing a Child SA before every round completed | Unit test: response with AAF must not set `StateEstablished` | Establishment only on a response with no AAF and no local round pending |
| R-5 | Accepting AAF from a peer when no remote round is configured would silently accept a partial authentication | AC-7 test | Refuse with AUTHENTICATION_FAILED (responder in the IKE_AUTH response, initiator in an INFORMATIONAL) |
| R-6 | A required remote round that the peer never performs is accepted | AC-8 test | The final message is checked against the configured remote rounds before establishment |
| R-7 | Round 2 combined with an EAP round 1 reaches EAP code that assumes one round | Commit-time validation refuses the combination | Validation refusal plus a unit test; EAP rounds go to the follow-up spec |
| R-8 | Reload while a round-2 exchange is in flight | Peer restart with a half-open SA | Unchanged mechanism: `AuthConfig.Equal` changes, peer restarts, half-open SA is reaped as today |
| R-9 | Concurrency: the responder's pre-adoption dispatch goroutine and the owner loop | Race detector in the engine unit tests | The round state lives on the SA and is touched only under `childLifecycleMu`, as `handleResponderInbound` already holds it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Site-to-site peers that configure a second round fail to establish; peers without a second round are unaffected except AC-7 (they now refuse a peer's ANOTHER_AUTH_FOLLOWS) |
| How is it reverted? | Revert the commits; the config leaves vanish with the YANG, so a config using them stops parsing (pre-release, no migration owed) |
| Who else touches this path? | RFC 4754 and RFC 7427 agents (`engine/auth.go`); any IKE spec touching `handleAuthRequest`/`handleAuthResponse` (`spec-ike-reauth.md`, `spec-ipsec-auth-piggyback.md`, `spec-ike-eap-only-authentication.md` in design or skeleton) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `authentication second-round` | → | `parseAuthConfig` → `AuthConfig.SecondRound` | `TestParseAuthConfigSecondRound` (`internal/component/ike/ipsec/config_second_round_test.go`) |
| Config commit (verify) | → | `validateIPsecSections` → second-round validation | `test/parse/ipsec-second-round-eap-refused.ci` |
| IKE_SA_INIT request to a Ze responder with a second round | → | `buildSAInitResponse` adds MULTIPLE_AUTH_SUPPORTED | `TestRFC4739ResponderAnnouncesMultipleAuth` (`internal/component/ike/engine/rfc4739_test.go`) |
| Ze initiator with a local second round, peer announced support | → | `buildAuthRequest` adds MULTIPLE_AUTH_SUPPORTED and ANOTHER_AUTH_FOLLOWS | `TestRFC4739InitiatorRequestCarriesBothNotifies` |
| Ze initiator ↔ Ze responder, two rounds each way, in process | → | `handleAuthRequest`, round handler, `handleAuthResponse` | `TestRFC4739ZeToZeTwoRoundsEachWay` |
| strongSwan initiator / responder | → | full stack | interop `multiple-auth-ze-initiator`, `multiple-auth-ze-responder` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A site-to-site peer with `authentication second-round local` or `second-round remote` configured, Ze responding | The IKE_SA_INIT response carries one MULTIPLE_AUTH_SUPPORTED notify with Protocol ID 0, SPI size 0 and no data |
| AC-2 | The same peer, Ze initiating | The first IKE_AUTH request carries one MULTIPLE_AUTH_SUPPORTED notify with Protocol ID 0, SPI size 0 and no data; later IKE_AUTH requests do not carry it |
| AC-3 | A peer with no `second-round` | No IKE_SA_INIT response or IKE_AUTH message Ze builds carries either notify, and the messages are byte-identical to today's for the same keys and nonces |
| AC-4 | Ze initiator with `second-round local`, the responder announced MULTIPLE_AUTH_SUPPORTED | The first IKE_AUTH request carries ANOTHER_AUTH_FOLLOWS (Protocol ID 0, SPI size 0, no data) alongside its AUTH; the next request carries IDi (round-2 identity), CERT when the round is x509, and AUTH computed over the round-2 IDi; it carries no SA, TSi or TSr |
| AC-5 | Ze responder with `second-round local`, the initiator's first IKE_AUTH carried MULTIPLE_AUTH_SUPPORTED | The first IKE_AUTH response carries IDr, CERT/AUTH of round 1 and ANOTHER_AUTH_FOLLOWS (Protocol ID 0, SPI size 0, no data), and no SA, TSi or TSr; the response to the next request carries IDr (round-2 identity), round-2 CERT/AUTH computed over that IDr, and SA, TSi, TSr |
| AC-6 | Ze with `second-round remote`, the peer sends ANOTHER_AUTH_FOLLOWS and a round-2 IDx, CERT/AUTH | Ze verifies the round-2 AUTH against the `second-round remote` credential (PSK, or certificate chain to its `ca-certificate` plus its `id`) and establishes only when both rounds verified; a round-2 AUTH valid only under the round-1 credential is refused with AUTHENTICATION_FAILED |
| AC-7 | Ze with no `second-round remote`, the peer's IKE_AUTH carries ANOTHER_AUTH_FOLLOWS | Responder: the IKE_AUTH response is AUTHENTICATION_FAILED and the SA ends. Initiator: Ze sends an INFORMATIONAL with AUTHENTICATION_FAILED and the SA ends. No Child SA is installed |
| AC-8 | Ze with `second-round remote`, the peer finishes without announcing ANOTHER_AUTH_FOLLOWS | Same refusal as AC-7, and the log names the missing remote round and the config leaf |
| AC-9 | Ze with `second-round local`, the peer did not announce MULTIPLE_AUTH_SUPPORTED | Ze completes with one local round, sends no ANOTHER_AUTH_FOLLOWS, and logs one warning naming the peer, that the round was skipped because the peer does not support RFC 4739, and the leaf to remove if the round is not wanted |
| AC-10 | A peer with a second round sends ANOTHER_AUTH_FOLLOWS again in its round-2 message | Refused as AC-7 (no third round is configured) |
| AC-11 | Config: `second-round` on a peer whose round-1 mode is an EAP mode; a round with mode x509 and no certificate (local) or no ca-certificate (remote); a round with mode pre-shared-secret and no secret | Commit is refused with a message naming the peer, the leaf and the fix |
| AC-12 | A retransmitted round-2 IKE_AUTH request reaches the Ze responder | The cached response is sent again and the request is not processed twice |
| AC-13 | strongSwan 6.1.0 initiator with `local-2` and `remote-2` against a Ze responder with both second rounds | The IKE SA and Child SA establish, charon logs both of Ze's authentications and Ze logs both of strongSwan's, and traffic flows through the tunnel |
| AC-14 | Ze initiator with both second rounds against a strongSwan 6.1.0 responder with `remote-2` and `local-2` | Same as AC-13 with the roles swapped |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator configures a machine certificate plus a user PSK towards a strongSwan gateway | config → `parseAuthConfig` → initiator rounds → strongSwan | `multiple-auth-ze-initiator` |
| 2 | Operator runs Ze as the gateway that requires a second credential from strongSwan clients | config → responder rounds → Child SA install | `multiple-auth-ze-responder` |
| 3 | Operator forgets the second credential on one side | the refusal in AC-7/AC-8 and its log line | `TestRFC4739UnconfiguredRemoteRoundRefused`, `TestRFC4739MissingRemoteRoundRefused` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNotifyTypeNamesMultipleAuth` | `internal/component/ike/wire/payload_notify_test.go` | 16404 and 16405 are recognised and named | |
| `TestRFC4739ResponderAnnouncesMultipleAuth` | `internal/component/ike/engine/rfc4739_test.go` | AC-1; tag `RFC4739-3.1-1` (built by `buildSAInitResponse`, parsed back, fields checked) | |
| `TestRFC4739InitiatorRequestCarriesBothNotifies` | `internal/component/ike/engine/rfc4739_test.go` | AC-2, AC-4; tags `RFC4739-3.1-1` and `RFC4739-3.2-1` (built by `buildAuthRequest`, decrypted, fields checked) | |
| `TestRFC4739ResponderAnotherAuthFollows` | `internal/component/ike/engine/rfc4739_test.go` | AC-5; tag `RFC4739-3.2-1` | |
| `TestRFC4739NoSecondRoundNoNotifies` | `internal/component/ike/engine/rfc4739_test.go` | AC-3 | |
| `TestRFC4739ZeToZeTwoRoundsEachWay` | `internal/component/ike/engine/rfc4739_test.go` | AC-4, AC-5, AC-6 in process, Ze against Ze, PSK and x509 rounds | |
| `TestRFC4739RoundTwoAuthUsesLatestID` | `internal/component/ike/engine/rfc4739_test.go` | AC-6: round-2 AUTH computed over the round-1 ID is refused; over the round-2 ID accepted | |
| `TestRFC4739RoundTwoCredentialNotCrossed` | `internal/component/ike/engine/rfc4739_test.go` | AC-6: a round-2 signature by the round-1 key or a chain to the round-1 CA is refused | |
| `TestRFC4739UnconfiguredRemoteRoundRefused` | `internal/component/ike/engine/rfc4739_test.go` | AC-7, both roles | |
| `TestRFC4739MissingRemoteRoundRefused` | `internal/component/ike/engine/rfc4739_test.go` | AC-8, both roles | |
| `TestRFC4739PeerWithoutSupportSkipsLocalRound` | `internal/component/ike/engine/rfc4739_test.go` | AC-9 | |
| `TestRFC4739ThirdRoundRefused` | `internal/component/ike/engine/rfc4739_test.go` | AC-10 | |
| `TestRFC4739RoundTwoRetransmitReplayed` | `internal/component/ike/engine/rfc4739_test.go` | AC-12 | |
| `TestRFC4739NonFinalResponseDoesNotEstablish` | `internal/component/ike/engine/rfc4739_test.go` | R-4 | |
| `TestParseAuthConfigSecondRound` | `internal/component/ike/ipsec/config_second_round_test.go` | parse of every new leaf, PSK encoding reuse | |
| `TestValidateSecondRound` | `internal/component/ike/ipsec/config_second_round_test.go` | AC-11 refusals and their messages | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | The feature adds no numeric leaf (`id-type` is an enumeration reused from `remote-id-type`) | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-second-round-accepted` | `test/parse/ipsec-second-round-accepted.ci` | A config with both second rounds validates | |
| `ipsec-second-round-eap-refused` | `test/parse/ipsec-second-round-eap-refused.ci` | `second-round` on an EAP peer is refused with the fix named | |
| `ipsec-second-round-x509-no-certificate` | `test/parse/ipsec-second-round-x509-no-certificate.ci` | An x509 local round with no certificate is refused | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `multiple-auth-ze-initiator` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | AC-14: Ze initiator, round 1 x509 (Ze `client.pem`, strongSwan `server.pem`), round 2 PSK each way with distinct round-2 identities; strongSwan `remote-2` and `local-2` | pass |
| `multiple-auth-ze-responder` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | AC-13: strongSwan initiator, round 1 X.509 both ways (strongSwan `server.pem`, Ze `client.pem`), round 2 PSK both ways (`swan-user.example.com`, `ze-user.example.com`); see Key Design Decisions for why strongSwan's round 1 cannot be a PSK | pass |

Each checker asserts the XFRM state on both sides, charon's log of the
round-2 authentication of Ze's round-2 identity, Ze's log of the
round-2 verification, and traffic. The vacuity proof: with the engine change
reverted and the Ze image rebuilt, strongSwan's `remote-2` is unsatisfied and
the scenario goes red.

## Files to Modify
- `internal/component/ike/wire/payload_notify.go` - constants 16404, 16405 and their `notifyTypeNames` entries
- `internal/component/ike/ipsec/types.go` - second-round type and the `AuthConfig` pointer field
- `internal/component/ike/ipsec/config.go` - parse `second-round`
- `internal/component/ike/ipsec/validate.go` - second-round validation (EAP combination, credential presence, identity rules reused from `ValidateIdentities`, PKI refs reused from `ValidatePKIRefs`)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `second-round` container with `local` and `remote`
- `internal/component/ike/engine/config.go` - call the new validation in `validateIPsecSections`
- `internal/component/ike/engine/sa.go` - round bookkeeping fields, new state for "IKE_AUTH rounds in progress", its String case
- `internal/component/ike/engine/auth.go` - accessors for the current local and remote round credential; `buildAuthRequest` notifies; round-2 request builder
- `internal/component/ike/engine/cert_payload.go`, `internal/component/ike/engine/remote_id.go` - read the credential through the accessors
- `internal/component/ike/engine/fsm.go` - `handleSAInitResponse` records support; `handleAuthResponse` round logic; `handleInbound` state routing
- `internal/component/ike/engine/responder.go` - `buildSAInitResponse` notify; `handleAuthRequest` round logic; `handleResponderInbound` state routing and replay; round-2 response builder
- `internal/le/interoplab/ipsec/checkers.go` - two checkers
- `rfc/short/rfc4739.md` - tags, support rows, `{gap}` notes removed when proven
- `docs/guide/ipsec.md` - the `second-round` configuration
- `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/architecture/ike/ipsec-14-responder.md` - the round flow
- `ai/digests/ipsec-ike.md` - flow items 8, 9 and the responder paragraph
- `docs/architecture/ike/ipsec-3-data-model.md` - the `AuthConfig` second-round field
- `features/ipsec-eap-authentication.md` or a new `features/ipsec-multiple-authentication.md` (level from `./le feature report`)
- `features/ipsec-interop-testing.md` - scenario list, if it enumerates scenarios

## Files to Create
- `internal/component/ike/engine/rfc4739_test.go` - engine tests above
- `internal/component/ike/ipsec/config_second_round_test.go` - parse and validation tests
- `test/parse/ipsec-second-round-accepted.ci`, `test/parse/ipsec-second-round-eap-refused.ci`, `test/parse/ipsec-second-round-x509-no-certificate.ci`
- `test/interop-ipsec/scenarios/multiple-auth-ze-initiator/{ze.conf,swanctl.conf}`
- `test/interop-ipsec/scenarios/multiple-auth-ze-responder/{ze.conf,swanctl.conf}`
- `rfc/discrimination/rfc4739.json` - written by `./le rfc record`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `second-round` container |
| YANG validation constraints | Yes | `mode` enumeration (pre-shared-secret, x509), `pre-shared-secret-encoding` enumeration, `id-type` enumeration reused from `remote-id-type`, `ze:sensitive` on the secrets |
| YANG custom validators | Yes | Cross-leaf rules (credential present for mode, no EAP round 1) in `validate.go`, called at verify; certificate and CA names complete from the PKI tree as the round-1 leaves do |
| CLI commands/flags | N-A | No command is added; `show vpn ipsec sa` output is unchanged |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | Yes | Automatic for the enumerations; certificate and ca-certificate reuse the round-1 completion |
| Functional test for new RPC/API | N-A | No RPC; config `.ci` tests cover the config surface |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, module or binary; certificates come from the existing PKI store and its existing checks |
| Prometheus counters/metrics | N-A | No new observable counter; the refusals log (existing IKE log lines pattern) |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` page for IPsec authentication (level from `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/guide/ipsec.md` (the IPsec configuration page); `docs/architecture/config/syntax.md` N-A, no grammar change |
| 3 | CLI command added/changed? | No | No command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | The `ike` plugin's registration is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` (notifies and round messages), with the ASCII notify layout in the code comment |
| 8 | Plugin SDK/protocol changed? | No | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4739.md` and its generated `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | No | Two scenarios use the existing lab; no harness change |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, if it lists IKEv2 authentication features |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-14-responder.md`, `ai/digests/ipsec-ike.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by the changed files' `// Design:` headers: `docs/architecture/ike/ipsec-3-data-model.md` is AFFECTED (`AuthConfig` gains the second-round field: update it); `docs/architecture/ike/ipsec-6-ikev2-crypto.md` is unaffected (no transform, PRF or key derivation changes; AUTH octets keep the RFC 7296 §2.15 formula with the round's ID); `docs/architecture/wire/buffer-writer.md` is unaffected (the two notifies use the existing `PayloadNotify` writer, no new encoder). Re-derive with `./le spec citation anchors spec plan/post-release/features/spec-ike-multiple-authentication.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` authentication examples, checked against the YANG |

## Implementation Steps

1. **Phase: Wiring** -- wire constants and registry entries, YANG container, `AuthConfig` field, parse, failing wiring tests
   - Tests: `TestParseAuthConfigSecondRound`, `TestNotifyTypeNamesMultipleAuth`, `TestRFC4739ResponderAnnouncesMultipleAuth` (fails: no notify yet)
   - Files: `payload_notify.go`, `types.go`, `config.go`, `ze-ipsec-conf.yang`
   - Verify: config parses; the announce test fails on the missing notify
2. **Phase: Validation** -- commit-time refusals (AC-11)
   - Tests: `TestValidateSecondRound`, the three `.ci` files
   - Files: `validate.go`, `engine/config.go`
3. **Phase: Credential accessors** -- route every AUTH-path credential read through the current-round accessors, round 1 unchanged
   - Tests: the existing engine auth tests stay green, unchanged; `TestRFC4739NoSecondRoundNoNotifies`
   - Files: `auth.go`, `cert_payload.go`, `remote_id.go`. Commit alone, by hunks if a foreign session holds `auth.go` (R-1)
4. **Phase: Announce** -- MULTIPLE_AUTH_SUPPORTED in both places (AC-1, AC-2, AC-3)
   - Files: `responder.go` `buildSAInitResponse`, `auth.go` `buildAuthRequest`, `fsm.go` `handleSAInitResponse`
5. **Phase: Responder rounds** -- AC-5 to AC-8, AC-10, AC-12 on the responder; new state; Child SA carried to the final response
   - Files: `responder.go`, `sa.go`
6. **Phase: Initiator rounds** -- AC-4, AC-6 to AC-10 on the initiator
   - Files: `fsm.go`, `auth.go`
7. **Phase: Ze-to-Ze and negatives** -- `TestRFC4739ZeToZeTwoRoundsEachWay` and the negative tests
8. **Phase: Interop** -- the two scenarios and checkers, vacuity proof recorded
9. **Phase: RFC records and docs** -- tags on the four tagged tests, `./le rfc record` for each, `{gap}` notes removed, Support rows updated, docs edited in the phase that changes each behaviour

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both user stories pass their interop scenario |
| Correctness | Round-2 AUTH uses the round-2 ID on both sides; SA/TSi/TSr only in the first request and the final response; AAF only in a message carrying AUTH (RFC 4739 §2.1, §3.2) |
| Correctness | Establishment happens once, on the final response, after every configured remote round verified |
| Naming | YANG `second-round`, `local`, `remote`, leaves mirror round-1 names (`mode`, `pre-shared-secret`, `pre-shared-secret-encoding`, `id`, `id-type`, `certificate`, `ca-certificate`) |
| Data flow | Credential readers use the accessors only; no `sa.PeerCfg.Auth` read left in the AUTH path that should follow the round |
| Rule: rfc-compliance | Every new MUST-enforcing statement carries the RFC 4739 quote; every new implementing function quotes the text it implements (functions added after 2026-09-24) |
| Rule: user-facing-errors | Every refusal log and commit error names the peer, the cause and the leaf that fixes it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Both rows proven | `./le rfc tree` shows rfc4739 mandatory 2 of 2 |
| Discrimination records | `rfc/discrimination/rfc4739.json` exists; `./le rfc check` clean for rfc4739 |
| Interop | `IPSEC_INTEROP_SCENARIO=multiple-auth-ze-initiator ./le test integration interop-ipsec`, same for `multiple-auth-ze-responder` |
| Engine tests | `./le test unit` scoped to `internal/component/ike/engine` and `internal/component/ike/ipsec` with the `ze_ike` tag |
| Config tests | the three `test/parse` `.ci` files pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed authentication | An unconfigured round, a missing round, a third round, or a round-2 AUTH under the wrong credential never establishes |
| Credential crossing | Round-2 verification reads only the `second-round remote` CA, id and PSK; round-1 verification is unchanged |
| Identity disclosure | RFC 4739 §5: the responder reveals its identity before every initiator round completes; documented in the guide as the RFC states, no mitigation owed |
| Secrets | Round-2 PSKs are `ze:sensitive` and never logged |
| Unprotected input | Round messages are accepted only inside the SK payload of the IKE SA, through the existing decrypt path |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A `second-round` container with `local` and `remote` under the peer's `authentication` | A list of numbered rounds; a second `authentication` container on the peer | One extra round per direction covers every RFC example and both stated use cases; a list adds an ordering and indexing surface nobody asked for. The follow-up spec can turn it into a list if more rounds are wanted |
| Read the current round's credential through two SA accessors | Swap `sa.PeerCfg.Auth` for the round; duplicate the AUTH functions for round 2 | Swapping mutates the config value that reload compares and that rekey and DPD read; duplicating creates a second AUTH implementation |
| Announce MULTIPLE_AUTH_SUPPORTED only for a peer with a second round | Always announce, as strongSwan does by default | Thomas's brief; an announced capability with no round configured would only lead to refusals (AC-7) |
| Refuse an unconfigured remote round with AUTHENTICATION_FAILED | Ignore the notify and finish | Thomas's brief; strongSwan's responder does the same (`update_cfg_candidates`); finishing would accept a partial authentication |
| Skip a local round, with a warning, when the peer did not announce support | Fail the exchange | strongSwan's `do_another_auth()` does the same; a peer that cannot run the round cannot be the one requiring it, so failing would only break interop |
| Liberal on receive: accept either notify whatever its Protocol ID, SPI size or data | Refuse a non-zero field | §3.1/§3.2 bind the sender; strongSwan does not check either; refusing would break interop for no protection |
| Round 1 must not be EAP when a second round is configured | Support EAP rounds now | Thomas's brief ("certificate or PSK second rounds first"); EAP rounds are the follow-up spec |
| vpn-concentrator becomes a required use case of RFC 4739 | Keep it optional but in scope | `docs/contributing/rfc-conformance-gates.md`: bringing an optional RFC into scope is moving its name to `Use cases` |
| An empty `second-round local id` is refused at commit | Fall back to the peer name, as round 1 does | Main-thread decision 2026-10-30: strongSwan selects a round's PSK by identity (`psk_authenticator.c`), so the fallback would assert the round-1 identity twice and look up the round-1 key; `validateSecondRoundLocal` refuses it and names the leaf |
| Hash-and-URL stays round 1; a round-2 chain always goes inline | Publish a second certificate URL per round | Main-thread decision 2026-10-30: `certificate-url` publishes the round-1 certificate, and inline is valid with every peer (`buildCertPayloads`) |
| The Notify encoder is the second producer of the zero Protocol ID and SPI Size | Rely on the builder literal alone | `wire.PayloadNotify.WriteTo` writes both as zero whenever no SPI follows (RFC 7296 Section 3.10), so a break of the builder's literal stays green; the discrimination records break the builder's "no data" half instead |
| `multiple-auth-ze-responder` uses X.509 round 1 and PSK round 2 both ways | strongSwan PSK round 1 with a `local-2 pubkey` round | Observed 2026-10-30: charon's initiator runs its strict all-rounds check when its current local round is a PSK (`ike_auth.c` `require_strict`) and refused Ze's first response ("insufficient authentication rounds"); and a `local-2` that complies with `local` is never sent (`do_another_auth`), with no third key pair in the lab. Ze's x509 second round is proven Ze to Ze |
| A second-exchange message that fails to decrypt is dropped and the SA kept | Kill the SA as the first exchange does | RFC 7296 Section 2.21.4; does not widen the open trap "a datagram that fails to decrypt kills a live SA" |

## Known Limitations
- EAP authentication rounds (RFC 4739 Examples 1 and 2) and more than one additional round per direction are not built here. They are the follow-up spec `plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md`.
- Remote-access peers (`vpn ipsec remote-access`, EAP-only) get no second round. That is the same follow-up spec, because their round 1 is EAP.

## RFC Documentation (Scope: protocol)

Add `// RFC 4739 Section X: "<quoted requirement>"` above enforcing code. The
notify builders quote §3.1 and §3.2 and carry the ASCII layout of the Notify
payload with Protocol ID and SPI Size at their offsets. The round logic quotes
§2.1 at the announce, the AAF placement, the latest-ID rule and the
all-rounds-succeed rule.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- Wire: `NotifyMultipleAuthSupported` (16404) and `NotifyAnotherAuthFollows` (16405) in `notifyTypeNames` (`internal/component/ike/wire/payload_notify.go`).
- Config: `authentication second-round { local remote }` (YANG), `AuthConfig.Additions.SecondRound` read through `SecondRound()`, parse in `parseSecondRound`, commit refusals in `ValidateSecondRounds` (`internal/component/ike/ipsec/second_round.go`), including an empty round-2 local `id`.
- Engine (`internal/component/ike/engine/multiple_auth.go`): round accessors `localAuth`/`remoteAuth`, the two notify builders, `settleAnotherAuth`, `verifySecondExchangeRound`, `buildAuthRoundRequest`, `continueInitiatorAuth`, `answerFirstAuthExchange`, `handleResponderAuthRound`; `handleAuthResponse` (`fsm.go`) and `handleAuthRequest`/`handleResponderInbound`/`buildFinalAuthResponse` (`responder.go`) route the second exchange; `handleResponderEAP` refuses ANOTHER_AUTH_FOLLOWS.
- Interop: `multiple-auth-ze-initiator`, `multiple-auth-ze-responder` with `checkMultipleAuthZeInitiator`/`checkMultipleAuthZeResponder` (`internal/le/interoplab/ipsec/multiauth.go`).

### Bugs Found/Fixed
- The hand mutant on the builders' Protocol ID stayed green: `wire.PayloadNotify.WriteTo` already writes Protocol ID and SPI Size as zero for an empty SPI. Not a product bug; the records break the builders' "no data" half (`rfc/discrimination/rfc4739/`).

### Documentation Updates
- `docs/guide/ipsec.md` "A second authentication round (RFC 4739)"; `docs/architecture/ike/ipsec-3-data-model.md`, `ipsec-7-ikev2-engine.md`, `ipsec-14-responder.md` decision paragraphs; `ai/digests/ipsec-ike.md` items 7-9; `docs/architecture/testing/interop.md` RFC 4739 section; `features/ikev2-engine.md` (RFCs, description, Interop); `features/ipsec-interop-testing.md`; `rfc/short/rfc4739.md` Meta rows. Each carries `<!-- source: -->` anchors to the producing symbols. Six existing claims re-read and recorded in `plan/doc-reviewed/bc7daf12.md`.

### Deviations from Plan
- `multiple-auth-ze-responder` runs X.509 round 1 and PSK round 2 both ways, not PSK round 1 with a strongSwan certificate round 2: charon's `require_strict` and `do_another_auth` make the planned shape unrunnable (Key Design Decisions). Ze's x509 second round is proven Ze to Ze.
- `StateAuthReceived`, an existing state never set before, carries the responder's pending second exchange instead of a new state.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Responder scenario round 1 was a PSK on strongSwan's side | charon's initiator runs its strict all-rounds check while its local round is a PSK and refuses the responder's first response | first interop run: "insufficient authentication rounds" | scenario redesigned; reason in `swanctl.conf` and `interop.md` |
| approach | strongSwan round 2 reused `server.pem` | charon treats a round that complies with a completed one as done (`do_another_auth`) and never sends it | second interop run: no ANOTHER_AUTH_FOLLOWS in request 1 | round 2 made a PSK under its own identity |
| approach | Discrimination break on the builders' Protocol ID | the encoder zeroes both fields for an empty SPI, so the break stays green | `./le rfc record` refused: "stayed GREEN" | break the "no data" half; comment names both producers |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-peer opt-in second round, PSK or x509, each direction | Done | `ipsec/second_round.go` `parseSecondRound`; YANG `second-round` | |
| Both roles | Done | `engine/multiple_auth.go` `continueInitiatorAuth`, `handleResponderAuthRound` | |
| strongSwan 6.1.0 interop | Done | `test/interop-ipsec/scenarios/multiple-auth-ze-*` | both pass, both walked red |
| Both MUST rows proven | Done | `rfc/discrimination/rfc4739/` | `./le rfc tree`: mandatory 2 of 2 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC4739ResponderAnnouncesMultipleAuth` | `buildSAInitResponse` |
| AC-2 | Done | `TestRFC4739InitiatorRequestCarriesBothNotifies`, `TestRFC4739ZeToZeTwoRoundsEachWay` (no MAS in request 2) | |
| AC-3 | Done | `TestRFC4739NoSecondRoundNoNotifies` | |
| AC-4 | Done | `TestRFC4739ZeToZeTwoRoundsEachWay` | request 2: IDi round 2, AUTH, no SA/TS |
| AC-5 | Done | `TestRFC4739ResponderAnotherAuthFollows`, `TestRFC4739ZeToZeTwoRoundsEachWay` | |
| AC-6 | Done | `TestRFC4739RoundTwoAuthUsesLatestID`, `TestRFC4739RoundTwoCredentialNotCrossed` | |
| AC-7 | Done | `TestRFC4739UnconfiguredRemoteRoundRefused` | both roles, AUTHENTICATION_FAILED read off the wire |
| AC-8 | Done | `TestRFC4739MissingRemoteRoundRefused` | both roles, log names the leaf |
| AC-9 | Done | `TestRFC4739PeerWithoutSupportSkipsLocalRound`, `TestRFC4739LocalRoundSkippedWithoutPeerSupport` | |
| AC-10 | Done | `TestRFC4739ThirdRoundRefused` | both roles |
| AC-11 | Done | `TestValidateSecondRound`; `test/parse/ipsec-second-round-*.ci` | |
| AC-12 | Done | `TestRFC4739RoundTwoRetransmitReplayed` | |
| AC-13 | Done | interop `multiple-auth-ze-responder` | pass 13s |
| AC-14 | Done | interop `multiple-auth-ze-initiator` | pass 14s |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All 16 unit tests of the TDD plan | Done | `engine/rfc4739_test.go`, `engine/rfc4739_rounds_test.go`, `ipsec/config_second_round_test.go`, `wire/payload_notify_test.go` | plus `ipsec-second-round-local-id-required.ci` |
| Interop scenarios | Done | `test/interop-ipsec/scenarios/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify / Create | Done | `internal/le/interoplab/ipsec/multiauth.go` holds the checkers instead of `checkers.go` (2988 lines); `checkers.go` carries the two registry entries |

### Audit Summary
- **Total items:** 14 AC, 4 requirements
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** responder scenario shape (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Second round, Ze initiator, strongSwan 6.1.0 | interop | `multiple-auth-ze-initiator`: pass (14s); with `multipleAuthRequestNotifies`/`settleAnotherAuth`/`moreAuthRounds` reverted by overlay and the image rebuilt: FAIL, charon "insufficient authentication rounds" |
| Second round, Ze responder, strongSwan 6.1.0 | interop | `multiple-auth-ze-responder`: pass (13s); reverted: FAIL, Ze "responder established SA" after one round while charon sent `IKE_AUTH request 2 [ IDi IDr AUTH ]` |
| Fail-closed on a partial authentication | unit, negative | `TestRFC4739UnconfiguredRemoteRoundRefused`, `TestRFC4739MissingRemoteRoundRefused`, `TestRFC4739ThirdRoundRefused`: AUTHENTICATION_FAILED decrypted off the wire, SA dead |
| Both MUST rows proven | discrimination | 4 records; `./le rfc tree`: rfc4739 mandatory 2 of 2 (100.0%) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| EAP authentication rounds, remote-access peers, more than one extra round per direction | Out of this spec's approved scope (Known Limitations) | `plan/post-release/features/spec-ike-multiple-authentication-eap-rounds.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ike-multiple-authentication-bc566aa5-3f69-415d-8c8b-c6f8cdf8adae.md` (20 files, verdict clean) |
| `./le spec review check` | clean: "review_gate: OK (7 code files, clean, hashes match)" |
| Rounds | 3: round 1 found 2 ISSUE (round-2 leaf names, untested second-exchange guards); round 2 found 0 BLOCKER, 0 ISSUE; round 3 re-read a lint-only rename in the lab checker (unparam), 0 findings |
| Reviewer lenses used | independent ze-read reviewer running /ze-review: RFC 4739 conformance, fail-closed auth, replay and message ids, concurrency, style pass, docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Round-2 refusals named round-1 leaves (`remote-id`, `remote-id-type`, `ca-certificate`) | `engine/remote_id.go` `checkRemoteIdentity`, `engine/cert_payload.go` `getRemoteCert` | `(*SA).remoteLeafName` (`multiple_auth.go`); `TestRFC4739RoundTwoRefusalNamesTheRoundLeaf` |
| 2 | ISSUE | Second-exchange guards (decrypt drop both roles, initiator message id, responder message id) had no test | `engine/fsm.go` `handleAuthResponse`, `engine/multiple_auth.go` `handleResponderAuthRound` | `TestRFC4739SecondExchangeGuards`; three mutants dropping the guards turn it red |

NOTEs recorded, not blocking: round-1 NOTE 4 (leniency stated in `settleAnotherAuth`) and NOTE 5 (`continueInitiatorAuth` now sends the INFORMATIONAL) were applied; NOTE 3 and the A-2 row corrected; NOTE 6 (round state spread over five SA fields) accepted as a peer-driven state machine. Round 2 NOTEs: the sub-tree and class-mismatch hints in `checkRemoteIdentity`/`classMismatchHint` still say "remote-id" in round 2 while the main sentence names the right leaf; no test drives the `continueInitiatorAuth` build-failure INFORMATIONAL.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `engine/multiple_auth.go`, `engine/rfc4739_test.go`, `engine/rfc4739_rounds_test.go`, `ipsec/second_round.go`, `ipsec/config_second_round_test.go` | yes | committed in 9e3ed47790 (`git show --stat 9e3ed47790`) |
| `test/parse/ipsec-second-round-{accepted,eap-refused,x509-no-certificate,local-id-required}.ci` | yes | `./le test bgp parse --pattern ipsec-second-round`: pass 4/4 |
| `test/interop-ipsec/scenarios/multiple-auth-ze-{initiator,responder}/` | yes | ze.conf, swanctl.conf, strongswan.conf, ze-env in each |
| `rfc/discrimination/rfc4739/RFC4739-3.1-1.json`, `RFC4739-3.2-1.json` | yes | `./le rfc discriminate stem rfc4739`: 4 records, none stale or unproven |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-12 | engine and config tests pass | `go test -v -run RFC4739` engine: 15 top-level tests PASS (26 with subtests); ipsec `ok` |
| AC-13, AC-14 | interop | both scenarios pass; both red under the revert overlay; after the review fixes, both pass again on HEAD plus the closing files (initiator 7s, responder 8s), built through an overlay that maps other sessions' in-flight edits back to HEAD |
| Review fixes | guards and leaf names | `TestRFC4739SecondExchangeGuards` and `TestRFC4739RoundTwoRefusalNamesTheRoundLeaf` pass under `-race`; mutants dropping each guard, and one making `remoteLeafName` ignore the round, turn them red |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config `authentication second-round` at commit | `test/parse/ipsec-second-round-eap-refused.ci`, `-local-id-required.ci` | runs `ze config validate -` on the config; expects the leaf named in stdout |
| strongSwan initiator / responder, full stack | interop scenarios | checkers read charon's per-round lines and Ze's `ike: second authentication round verified` line |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | both scenarios ran with no charon setting |
| A-2 | confirmed | `multiple-auth-ze-initiator` pass: charon answered SA/TS in its final response |
| A-3 | confirmed | both scenarios pass with one round-2 secret owned by both round-2 identities |
| A-4 | confirmed | `TestRFC4739RoundTwoRetransmitReplayed` |
| A-5 | confirmed | `TestRFC4739RoundTwoCredentialNotCrossed`, x509 subtest of `TestRFC4739ZeToZeTwoRoundsEachWay` |
| A-6 | confirmed | approved by Thomas 2026-10-30 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide section: refusals, id required, hash-and-url round 1 only | `settleAnotherAuth`, `validateSecondRoundLocal`, `buildCertPayloads` | yes |
| `docs/architecture/testing/interop.md` scenario table and charon policies | both swanctl.conf files; charon `require_strict`, `do_another_auth` | yes |
| `./le doc wiring` | no IKE finding after `plan/doc-reviewed/bc7daf12.md` | yes |
