# Spec: Cap transmit MTU when the peer omits MRU

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol, config, docs |
| Priority | P1 |
| Release reason | A working subscriber can transmit packets larger than an accepted 1492 session limit. |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Blocker

Formal closure returns to implementation because native RFC freshness is not
complete. `./le rfc check` reports a stale RFC1661-6.1-1 audit over both claims
of `rfc1661_proxy_policy_driver_test.go` `TestReceiveFullFrameBelowConfiguredMRU`
and 53 producer-changed discrimination entries in 32 record files for the
shared PPP producers changed in `18ea5e830a`. The two new full-receive semantic
discrimination records verify; they do not refresh the older audit or the
other producers' records.

| Affected surface | Exact repair population |
|------------------|-------------------------|
| Audit | Re-judge RFC1661-6.1-1 against the changed `TestReceiveFullFrameBelowConfiguredMRU` and both claims, using `ze-rfc-audit`; this is a changed function, not a line shift, so mechanical resealing cannot repair it. |
| `rfc/discrimination/rfc1332/` | Refresh `RFC1332-2.1-3.json` and `RFC1332-2.1-4.json`; producer `session_run.go` `afterLCPOpen`. |
| `rfc/discrimination/rfc1334/` | Refresh `RFC1334-2.2.1-3.json`, `RFC1334-2.2.1-6.json`, `RFC1334-2.3-1.json`, `RFC1334-2.3-2.json`; producer `pap.go` `runPAPAuthPhase`. |
| `rfc/discrimination/rfc1661/` | Refresh IDs `2-1`, `3.1-2`, `3.5-3`, `3.5-6`, `3.6-2`, `3.6-3`, `3.7-1`, `4.3-1`, `5.2-2`, `5.4-1`, `5.6-1`, `5.6-3`, `5.7-1`, `5.7-3`, `5.7-4`, `5.8-2`, `6.4-2`, `6.4-8` (each file is `RFC1661-<ID>.json`). Producers are `session_run.go` `afterLCPOpen`, `handleFrame`, `handleLCPPacket`, `sendEchoReply`, `sendProtocolReject`, `supportsProtocol`, `writeCodeReject`, and `ncp.go` `runNCPPhase`, `sendNCPConfigureAck`. |
| `rfc/discrimination/rfc1994/` | Refresh IDs `2-1`, `4.1-10`, `4.1-3`, `4.1-8`, `4.2-1`, `4.2-2`, `4.2-5` (each file is `RFC1994-<ID>.json`); producers `chap.go` `runCHAPAuthPhase` and `session_run.go` `handleFrame`. |
| `rfc/discrimination/rfc5072/` | Refresh `RFC5072-3-1.json`; producers `session_run.go` `handleFrame` and `ncp.go` `runNCPPhase`. |

The same native output reports RFC1334-x-1's byte-identical auth-selection
audit as SHIFTED; implementation can reseal that result mechanically. Foreign
PPPoE-client and BGP audit/discrimination failures are outside the producer
population above. Root will commit this handback with the additive guard
regression, release the claim, repair the owned RFC evidence in a separate
implementation phase and resume an independent closure. Clearing the stale
audit and these producer records, with a successful scoped freshness check,
clears this blocker. No spec removal or closure CLEAN artifact is authorized
by the current evidence.

## Task

Own finding F-1 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction and review evidence. A peer that omits MRU must establish a useful session whose transmit MTU respects its admitted session cap. L2TP configured to 1492 and PPPoE's fixed 1492 cap must both hold.

This spec owns the shared transmit-cap implementation. F-2's proxy regression and protected test judgment belong to `plan/release/fixes/spec-home-cpe-16b-proxy-mru-transmit-cap.md`; implement one shared cap, not a second proxy-specific policy.

## Required Reading

- [x] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`.
  → Constraint: keep transport-independent PPP setup shared between L2TP and PPPoE.
- [x] `docs/guide/l2tp.md`, PPP negotiation and reload; `docs/guide/pppoe.md`, MRU ceiling.
  → Constraint: preserve admission-time policy isolation and correct each cap description with its implementation.
- [x] RFC 1661 Section 6.1, RFC 2516 Section 7 and RFC 1332 in the repository authoritative texts.
  → Constraint: distinguish transmit limits from required receive capacity; verify existing requirement ownership before adding RFC tags.

## Pre-implementation Behavior

| Source checked | Behavior |
|----------------|----------|
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.afterLCPOpen` | Falls back to 1500 for absent negotiated MRU, then installs the larger of that value and 68. It does not bound interface MTU by the session cap. |
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.handleLCPPacket` | An accepted peer request without MRU records the PPP default of 1500. |
| `internal/component/l2tp/ppp/manager.go`, session construction | Defaults an absent `StartSession.MaxMRU` to 1500 and stores the session cap separately. |

Preserve authentication allow-list enforcement, receive capacity, valid subscriber traffic, and the distinction between negotiated peer MRU and installed transmit MTU.

## Existing Mechanisms

| Mechanism | Location | Current role | Disposition |
|-----------|----------|--------------|-------------|
| Interface setup after NCP | `pppSession.afterLCPOpen` | Installs MTU and brings the PPP interface up. | Extend this shared path with the admitted session's transmit cap. |
| Session policy snapshot | `StartSession.MaxMRU` and `pppSession.maxMRU` | Retains the session's admitted policy. | Use existing state rather than reading current global config. |

## Data Flow

Transport config/default → admitted session cap → normal LCP peer MRU or PPP default → post-NCP interface MTU installation → kernel subscriber traffic. Trace restart, reload, userspace transmit helpers and every MTU writer before design approval. A negotiated peer value remains factual even when local policy imposes a smaller transmit MTU.

## Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | L2TP cap 1492; peer omits MRU | Authentication and IPCP complete, installed MTU is 1492, and traffic succeeds without transmitting an Information field larger than 1492. | The session remains admitted because omitted MRU supplies a peer receive default, not permission to ignore local policy. |
| AC-2 | PPPoE peer omits MRU | The session passes useful traffic with installed MTU at most 1492. | Default pppd behavior remains interoperable within the transport cap. |
| AC-3 | Cap 1492; peer requests MRU 1400, 1492 or 1500 | Installed MTU is 1400, 1492 or at most 1492 respectively after valid negotiation; valid-sized traffic succeeds. | Smaller peer limits remain binding; a larger request can converge within the local cap. |
| AC-4 | LCP restarts or configuration reloads during established service | Replacement negotiation uses the admitted session policy; new sessions use the new policy. Required receive capacity and auth behavior remain intact. | Existing sessions retain their admitted policy, as the reload contract requires. |

## Test Plan

| Planned proof | Scope | Required discrimination |
|---------------|-------|-------------------------|
| `TestOmittedMRUTransmitCap` | Shared PPP driver with observed backend MTU | Both transport inputs reach the real post-NCP setter; uncapped code installs 1500 and fails. |
| `TestTransmitCapRestartIsolation` | Driver restart/reload and event consumers | Replacement negotiation cannot bypass the stored cap. |
| Omitted-MRU live cases | L2TP and PPPoE policy scenarios; name exact cases during design | Observe omitted MRU on wire, installed `pppN` MTU, successful data traffic and oversize handling on Ze's kernel. |

Run scoped PPP/L2TP/PPPoE tests under race and prove live assertions fail against a rebuilt defective artifact. Existing interop results are historical; no tests ran for this split.

## Files to Modify and Documentation

- `internal/component/l2tp/ppp/session_run.go`: shared interface transmit-cap enforcement; census sibling transmit/MTU paths before editing.
- Shared PPP driver tests and existing L2TP/PPPoE policy scenarios: omitted-option and smaller-limit regression proof.
- `docs/guide/l2tp.md` and `docs/guide/pppoe.md`: explain effective transmit cap separately from negotiated peer receive MRU; update corresponding wiki pages and affected source anchors with implementation.

## Risks & Assumptions

| ID | Risk or assumption | Signal / validation | Mitigation / impact |
|----|--------------------|---------------------|---------------------|
| R-1 | Replacing negotiation state with configured cap misreports what the peer agreed. | Unequal configured/peer values expose copied output. | Keep negotiated state separate; output proof is owned by child 16f. |
| R-2 | A sibling writer or restart restores 1500. | Writer census or restart regression finds another setter. | Extend the shared path and prove every reached setter uses it. |
| A-1 | Confirmed: the shared interface setter reaches both transports. | `l2tpReactor.handleKernelSuccess` supplies the L2TP policy; `InterfaceServer.handlePADR` supplies PPPoE's 1492 limit; both enter `Driver.spawnSession` and `pppSession.afterLCPOpen`. | Both live omission scenarios measured Ze MTU 1492, failed with measured 1500 after the cap was removed, and passed after restoration. |

## Research Resolution

The following implementation census, central verification and closure audit resolve this skeleton's research requirements. Child 16d owns protocol-floor validation; this closure checks that the shared cap preserves its boundaries without closing that child's spec.

## Implementation Work (2026-11-04)

The owner requested one agent per issue, with all compilation and execution
centralized in the main thread. This agent authored changes without running
Go tools, tests, builds, linters, formatting, the daemon or Docker.

### Diagnosis

| Part | Finding |
|------|---------|
| Symptom | An omitted peer option publishes the correct PPP default 1500 but installs MTU 1500 despite session cap 1492. |
| Root cause / owner | `pppSession.afterLCPOpen` uses only negotiated peer MRU for the IP interface setter. |
| Source fix | Bound the installed MTU by the admitted `maxMRU` without replacing negotiated peer state. |
| Rejected workaround | Reject omitted options or add proxy admission restrictions; this drops interoperable sessions without enforcing the shared interface policy. |

### Wiring and Source Census

| Entry / consumer | Source | Disposition |
|------------------|--------|-------------|
| L2TP admission | `reactor_kernel.go`, `MaxMRU: policy.MRU` | Existing policy snapshot reaches the shared driver. |
| PPPoE admission | `pppoe/server.go`, `MaxMRU: PPPoEMaxMTU` | Existing 1492 cap reaches the shared driver. |
| Session construction | `Driver.spawnSession` | Preserves the admitted cap; an unspecified cap defaults to 1500. |
| Normal peer omission | `pppSession.handleLCPPacket` | Preserves negotiated peer MRU 1500. |
| IP interface setter | `pppSession.afterLCPOpen` | Installs the smaller of negotiated MRU and admitted cap, retaining the existing 68 floor until child 16d resolves that boundary. |
| Code/Protocol-Reject writers | `sendProtocolReject`, `writeCodeReject` | Use the same effective transmit cap while preserving original rejected headers and the fitting payload prefix. |
| Echo reply writer | `sendEchoReply` | Preserve fitting Data and bound its reflected prefix by the shared transmit cap. |
| Effective transmit limit | `pppSession.transmitMRU` | Keep the peer's negotiated receive value factual and return its minimum with admitted policy. |

The static search covered `SetMTU`, `negotiatedMRU` and `MaxMRU` in the shared
PPP package plus both transport admission sites. The single shared production
`SetMTU` call is the positive control. The PPP negotiated event/snapshot fields
and kernel receive MRU remain unchanged.

### Authored Tests and Central Verification Owed

| AC | Authored carrier | Current evidence |
|----|------------------|------------------|
| AC-1 / AC-2 | `TestOmittedMRUTransmitCap` | Driver admission, actual LCP packets, negotiated event, post-NCP interface MTU and admin-up assertions; NOT RUN. |
| AC-3 | `TestTransmitCapPeerMRUBoundaries` | Smaller/equal/larger request, correlated Nak convergence, 1500 default control and 1280 admitted-cap control; NOT RUN. |
| AC-4 | `TestTransmitCapRestartIsolation` | Real LCP restart and second interface installation retain 1492 with negotiated peer default 1500; NOT RUN. |
| AC-4 reload | Existing `TestReactorPPPPolicyReloadIsolation` | Main thread must rerun existing new/old-session policy isolation coverage. |
| Control transmit size | `TestControlRepliesRespectTransmitCap`, `TestControlReplyFittingDataIsPreserved` | LCP/IPCP/IPv6CP Code-Reject, Protocol-Reject and Echo-Reply respect the admitted cap for omitted/1500/9000 peer MRU; smaller peer limits and fitting content remain binding; NOT RUN. |
| Live L2TP traffic | `l2tp-lns-omitted-mru-xl2tpd`, authored by child 16e's agent | Wire omission, Ze-side measured MTU, negotiated output and useful traffic remain required; no run claimed here. |
| Live PPPoE traffic | `pppoe-omitted-mru-ze-ac` through `checkOmittedPeerMRU` | Correlated omitted MRU request/Ack, CHAP/IPCP, measured Ze MTU1492, bidirectional traffic and complete kernel teardown; authored but NOT RUN. |

### Critical Review Checklist

| Check | Requirement |
|-------|-------------|
| Direction | Preserve peer receive MRU in events/snapshots and apply local cap only to effective transmit size. |
| Shared path | Both transports and accepted proxy sessions reach the same setter. |
| Receive capacity | Keep the kernel receive MRU at 1500 while lowering the transmit MTU. |
| Lifecycle | A restarted session retains its admitted cap; only new sessions use reloaded policy. |
| Useful service | Central live runs must prove valid traffic, not only absence of oversized frames. |

The control writers use the same `transmitMRU` method; it takes the session
lock and must be called without that lock held. Coordinated child 16d edits
also select `MinIPv4MRU` in the peer negotiation policy and local Nak adoption;
child 16d owns that constant and its validation behavior.

The original full-receive Echo test expects a 1500-octet reflection despite
the local cap. Its protected expectation needs independent judgment before
changing it. Required full reception must remain proved separately from the
length of the capped reply.

Documentation edits cover the L2TP MRU, output and proxy descriptions, plus
PPPoE's fixed transmit cap. Central review, formatting, race tests, live
discrimination, corresponding wiki updates and verification records remain
unrun. This is an uncommitted implementation handoff, not a green result.

### PPPoE Live Carrier Handoff

`check_mru_omitted.go` registers the new PPPoE scenario through the existing
tagged checker registry. It reuses the native AC topology, typed pppd dial,
peer queries, traffic and cleanup helpers. The only peer option override is
`default-mru`, with `nopersist` and `noipv6` to bound the attempt. Pinned
pppd 2.5.5's `lcp_addci` emits no MRU when `neg_mru` is false; its PPPoE
plugin would otherwise set MRU1492 even if the command line omitted it.
The same flag rejects Ze's initial MRU option, so the checker proves the
resulting replacement negotiation and usable IPv4 session.

New tests `TestOmittedMRULCPEvidence`, `TestOmittedMRUZeLink` and
`TestOmittedMRUScenarioMeasuresZeAndTraffic` exercise the actual checker.
The last test rejects an uncapped Ze MTU1500 even while the peer's local
interface is1492 and authentication succeeded. These tests were not run.

Central live selector: `ZE_PPPOE_INTEROP_SCENARIO=pppoe-omitted-mru-ze-ac`.
Use the named working-tree build and registered
`test deployment docker-pppoe-accel` action. Run the PPPoE checker package
tests with `ze_l2tp` and the full applicable feature tags. Existing
protected fixtures were not changed for this carrier.

### Authentication and Final Writer Census (2026-11-05)

The coordinator's review found further transmit-size consumers in the shared
authentication senders. Their optional result messages could use the full
pooled frame even when the session cap was smaller. The same source fix
now bounds these producers before encoding; it does not cut arbitrary
serialized authentication packets.

| Writer family | Size policy / preserved behavior |
|---------------|----------------------------------|
| `runPAPAuthPhase` | Bound optional Message by effective transmit MRU minus the five-byte header and by its one-byte length; preserve Ack/Nak and Identifier. |
| `reanswerPAP` | Existing empty Message produces a five-byte result and preserves the cached outcome. |
| `runCHAPAuthPhase` | Pass a bounded pooled-buffer view and trim only display text; keep challenge, response and decision unchanged. |
| `runMSCHAPv2AuthPhase` | Preserve the full success authenticator and structured failure prefix; bound only human-readable text. An unfit mandatory prefix fails explicitly. |
| LCP/NCP Configure-Ack | Fail visibly when the entire unchanged option set cannot fit, aborting before AckSent/Opened; a fitting option set is still echoed exactly. |
| LCP/NCP Configure-Nak/Reject | Preserve whole unacceptable options; fail before committing the transition when its complete response cannot fit. |
| Code-Reject, Protocol-Reject, Echo-Reply | Use the admitted transmit limit, keeping their required headers and fitting payload prefix. |
| Local LCP/NCP requests, termination and CHAP challenges | Fixed generated options/headers and challenge name fit the supported 68-octet minimum. |
| `writeFrame` | Defensive final check refuses a missed oversized producer, without cutting its packet or emitting bytes. No caller at the searched write sites holds the session mutex. |

RFC 1334 Section 2.2.2 and RFC 1994 Section 4.2 make display messages
independent of the authentication decision. RFC 2759 Sections 5 and 6
instead require the authenticator response and structured failure challenge
fields to survive; their following human-readable text alone is shortened.

New authored driver tests `TestAuthenticationResultTransmitCap` cover PAP,
CHAP-MD5 and MS-CHAPv2 acceptance/rejection at local cap68/1492, a smaller
peer1400 limit and fitting text. They check actual handler requests, wire
results, identifiers, complete MS fields and unchanged outcome events.
`TestCachedPAPResultFitsMinimumTransmitCap` covers both cached reply codes.
`TestMSCHAPv2FailurePrefaceCannotBeTruncated` checks structured prefix limits.

`TestConfigureRepliesRespectTransmitCap` exercises the three protocols'
Ack/Reject writers with explicit failure for oversized options and unchanged
fitting controls on fresh equivalent sessions. `TestWriteFrameRefusesMissedTransmitCap` proves the final boundary
refuses an oversized packet while sending a fitting packet unchanged.

The coordinator's independent review exposed false success after an unsent
Ack: the writer returned true, so the handler could commit AckSent/Opened
and enter authentication or publish an address without sending the Ack.
`TestUnsentLCPAckCannotAdvanceNegotiation` and
`TestUnsentIPCPAckCannotAssignAddress` reached those real handlers from
ReqSent and AckRcvd. The coordinator observed the oversized cases fail and
all fitting controls pass before the source fix. The four Ack/Nak/Reject
size-failure branches now publish session failure and return false, stopping
the existing action loops before their state commit. The handler regressions
are unchanged. The sender-test correction was independently approved by the
coordinator and preserves all six oversized inputs and exact fitting outputs.

All new tests, formatting and execution remain with the coordinator. A
mutation removing each result buffer/message bound, or removing each
reflection bound plus the final writer guard, must fail the corresponding
positive driver/result-size or no-oversized-response assertions.

## Central Verification (2026-11-05)

The coordinator ran all formatting, compilation and tests; the issue agent
authored without execution. These observed results supersede earlier NOT RUN
handoffs. Runtime changes are committed as `18ea5e830a`, live carriers as
`3cc99259b0`, and the corresponding wiki corrections as `312639d`.

All L2TP packages and both live-checker packages passed under race. The
driver tests cover omission on both transports, smaller/equal/larger peers,
restart isolation, full receive capacity, bounded control/authentication
messages and the final writer boundary. Independent review exposed false
FSM advancement after an unsent oversized Ack: the new handler regressions
first failed for Ack-Sent/Opened, auth and address side effects, then passed
after the source fix, retaining fitting-request positive controls.

Both normal omitted-MRU scenarios passed on Ze's kernel with actual Ze
MTU 1492, completed authentication/IPCP, useful traffic and cleanup. Rebuilding
the actual daemon with the admitted cap removed made the PPPoE carrier fail
on measured MTU 1500; the restored PPPoE build passed again. The omitted-L2TP
carrier separately failed on measured MTU 1500 rather than its required 1492.
The accepted-proxy carrier also completed its own green/red/restored-green
proof for larger and omitted peer limits. No halted producer is counted as
transmit-behavior proof.

PPP statement coverage measured 82.6%; `transmitMRU`, `writeFrame`, and the
LCP/NCP Configure-Ack helpers had every measured statement covered. The
changed RFC1661 full-receive test retains actual full-frame read and kernel
receive-MRU assertions. Both native claim records now use the hand-written
semantic ioctl break: setting receive MRU to 1492 fails the preserved 1500
assertion while the unmodified test passes.

Evidence under `tmp/session/2026-11-04-222204/scratch/` includes
`job-cpe16-postreview-runtime-race-f3e7163f.log`,
`job-cpe16-live-pppoe-omitted-final-8044ad91.log`,
`job-cpe16-pppoe-cap-disabled-0d495f97.log`,
`job-cpe16-pppoe-cap-restored-8044ad91.log` and
`job-cpe16-l2tp-omitted-cap-disabled-22d28569.log`.
The final restored L2TP artifact passed refusal, omitted-MRU MTU 1492 and
distinct negotiated-MRU checks in
`job-cpe16-final-artifacts-restored-718e9ad2.log`. Both transport omission
carriers therefore have observed green/red/restored-green evidence. Central
runtime verification is complete. Closure later found the RFC proof-freshness
debt recorded in Blocker; that evidence still requires implementation work.

### Retained commands and observed output

The canonical carriers are the committed driver/checker tests in
`18ea5e830a` and `3cc99259b0`, not the temporary log locations. Root ran the
race command below with `ze_core` and all 35 unique feature tags from
`feature-gates.txt`; tag order does not affect selection. It reported `ok`
for all L2TP packages and both checker packages, including PPP and every
cap, restart, authentication, unsent-Ack and full-receive test named above.

| Command | Directly observed result |
|---------|--------------------------|
| `./le --name cpe16 job run label cpe16-postreview-runtime-race quiet command go test -race -tags ze_core,ze_anomaly,ze_as112,ze_bfd,ze_bgp,ze_bmp,ze_copp,ze_cos,ze_ddos,ze_dhcpserver,ze_exabgp,ze_flowexport,ze_geodns,ze_gnmi,ze_grpc,ze_ike,ze_isis,ze_l2tp,ze_ldp,ze_lg,ze_mcp,ze_mpls,ze_ntp,ze_ospf,ze_policyroute,ze_pxe,ze_radius,ze_rest,ze_rsvpte,ze_ssh,ze_tacacs,ze_telemetry,ze_trafficusage,ze_vpp,ze_vrrp,ze_web ./internal/component/l2tp/... ./internal/le/interoplab/l2tp ./internal/le/interoplab/pppoe -v -count=1 -timeout=180s` | PPP `ok` (17.771s); L2TP checker `ok` (3.186s); PPPoE checker `ok` (11.129s); all selected packages pass. |
| `env -u GOFLAGS -u NO_BUILD ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-chap-only-xl2tpd SESSION_TIMEOUT=300 ./le --name cpe16 test deployment docker-l2tp-ppp` | Final clean image rebuild/restoration: refusal scenario passes. |
| `env -u GOFLAGS NO_BUILD=1 ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-omitted-mru-xl2tpd SESSION_TIMEOUT=300 ./le --name cpe16 test deployment docker-l2tp-ppp` | Same restored image: actual Ze MTU1492; omission scenario passes (25s), `1 passed, 0 failed`. |
| `env -u GOFLAGS NO_BUILD=1 ZE_L2TP_INTEROP_SCENARIO=l2tp-lns-negotiated-mru-xl2tpd SESSION_TIMEOUT=300 ./le --name cpe16 test deployment docker-l2tp-ppp` | Same restored image: unequal negotiated-MRU scenario passes (29s), `1 passed, 0 failed`. |

The identical omission checkers also observed their intended counterexamples:
L2TP reported `measured PPP MTU is 1500, want 1492`; PPPoE reported
`Ze PPP interface does not install the 1492 transmit cap` with actual
`MTU:1500`. Restored PPPoE output measured `"ifname":"ppp0","mtu":1492`
and reported `1 passed, 0 failed`. These are semantic transmitter failures,
with session establishment intact, rather than a stopped producer.

## Deliverables Checklist

| Deliverable | Verification | Result |
|-------------|--------------|--------|
| Shared effective transmit limit | Read `session_run.go` `pppSession.transmitMRU` and the sole shared `SetMTU` in `afterLCPOpen`. | Done: minimum of peer receive MRU (default 1500) and admitted cap; raw negotiated state is retained. |
| L2TP and PPPoE omission service | Read `TestOmittedMRUTransmitCap`, `checkLNSOmittedMRU` and `checkOmittedPeerMRU`; central live green/red/restored-green evidence above. | Done: both real transports install Ze MTU 1492 and carry traffic; cap-disabled daemons fail on measured MTU 1500. |
| Smaller peer limit and lifetime isolation | Read `TestTransmitCapPeerMRUBoundaries`, `TestTransmitCapRestartIsolation` and `TestReactorPPPPolicyReloadIsolation`. | Done: smaller peer limits bind; restart retains the admitted cap; reload changes new sessions only. |
| Complete, bounded userspace writes | Read control/authentication/configuration senders and `writeFrame`; central race and discrimination evidence above. | Done: optional text/reflected tails are bounded; unrepresentable complete replies fail before state commit. |
| Full receive capacity | Read `afterLCPOpen` and `TestReceiveFullFrameBelowConfiguredMRU`. | Done: actual receive ioctl remains 1500; semantic ioctl1492 break fails the retained 1500 assertion. |
| Guides, live procedures and wiki | Source anchors in both guides and both lab pages; commits `18ea5e830a`, `3cc99259b0`, wiki `312639d`. | Done: documented receive/transmit distinction and actual Ze-side omission proof. |

## Security Review Checklist

| Concern | Producer reviewed | Result |
|---------|-------------------|--------|
| Peer-controlled reflection and reply amplification | `sendProtocolReject`, `writeCodeReject`, `sendEchoReply`, LCP/NCP Configure senders | Bounded pooled writes; required headers/options are retained or the session fails. No partial Ack or successful transition after an unsent reply. |
| Authentication integrity | `runPAPAuthPhase`, `runCHAPAuthPhase`, `runMSCHAPv2AuthPhase`, `reanswerPAP` | Only optional display text is shortened. Identifier, result Code, challenge and complete MS authenticator/error fields remain intact. |
| Bounds, integer conversion and allocation | `transmitMRU`, `writeFrame`, result encoders | Admitted 68..1500 cap bounds lengths; pooled frame capacity is 1502 including Protocol. New bounds create no peer-sized allocation. |
| Mutex and lifetime ordering | `transmitMRU`, `afterLCPOpen`, `applyTransition`, NCP handler action loop | All reviewed transmit callers release the session lock before taking the snapshot. Cancellation, descriptor ownership, receive sizing and teardown remain in the shared path. |
| Hostile-input panic | `runMSCHAPv2AuthPhase`, challenge producers | Exact 20-byte plugin blob validation precedes the success encoder; challenge encoders receive fixed stack arrays. New bounds cannot expose a peer-triggered panic. |
| Injection, paths, privilege, cryptography and leakage | Runtime diff in `18ea5e830a` | This cap adds no shell/path/authorization/cryptographic operation or credential logging. Existing auth decisions and allow-list enforcement remain binding. |

## Documentation Update Checklist

| # | Category | Applies? | Source-aware disposition |
|---|----------|----------|--------------------------|
| 1 | Feature declaration | No | Existing L2TP/PPPoE capability corrected; no new capability or maturity claim. The cap uses existing `StartSession.MaxMRU`. |
| 2 | Config syntax | No | This child changes no grammar; protocol-floor schema changes are owned by child 16d. |
| 3 | CLI reference | No | No new command or grammar; negotiated-output proof belongs to child 16f. |
| 4 | API/RPC | No | Existing negotiated event fields are preserved; no type/registration change. |
| 5 | Plugin guide | No | Shared PPP leaf and subscriber handler boundary remain unchanged. |
| 6 | User guides | Yes | `docs/guide/l2tp.md`, PPP negotiation/authentication/proxy policy, and `docs/guide/pppoe.md`, MRU ceiling; source-anchored changes in `18ea5e830a`. Wiki counterparts landed in `312639d`. |
| 7 | Wire format | No | Existing PPP formats and mandatory fields are unchanged; only allowed reply text/prefix lengths and effective transmit size change. Guides explain these bounds. |
| 8 | SDK/process protocol | No | No SDK, process message, event declaration or RPC change. |
| 9 | RFC behavior/proof | Yes | Existing RFC1661-6.1-1 full-receive claims retain their assertion and have refreshed semantic receive-ioctl discrimination in `18ea5e830a`. No new RFC tag or claim is introduced by this child. Native RFC freshness is recorded below. |
| 10 | Test infrastructure | Yes | Existing lab procedures now include both omitted-MRU scenarios and actual Ze-side MTU/traffic checks in `docs/labs/l2tp-interop.md` and `docs/labs/pppoe-interop.md`, commit `3cc99259b0`. No new runner is introduced. |
| 11 | Comparison | No | No supported protocol, role or feature scope changes. |
| 12 | Architecture | No | Existing shared PPP admission/setup and transport boundaries are preserved. |
| 13 | Route metadata | No | No route metadata producer/consumer changes. |
| 14 | Metrics | No | No counter/gauge registration changes in the cap. |
| 15 | Runtime inventory/doctor | No | No plugin/command/event/capability or new runtime dependency; existing PPP kernel interface operations are reused. |
| 16 | Source anchors | Yes | Source-anchor census in both guides identifies `session_run.go`, `manager.go`, `pap.go`, `chap.go`, `mschapv2.go`, `ncp.go` and the two omission checkers. Claims match the producing functions read during closure. Declared design `docs/research/l2tpv2-ze-integration.md` remains unaffected: the control/data split and IPv6 service guard are preserved. Related `docs/architecture/l2tp/bng-1-radius-attributes.md` still classifies negotiation/auth failures as NAS Error through `fail`; related `docs/research/l2tpv2-implementation-guide.md` still describes the unchanged FSM code/event mapping. The transmit-policy correction is documented in the guides. |
| 17 | Existing examples | Yes | Existing MRU usage remains valid. Both omission scenario configs match the admitted 1492 cap; PPPoE still has no configurable MRU knob. |

---

## Implementation Summary

### What Was Implemented

One shared `pppSession.transmitMRU` supplies the installed IP MTU and userspace
reply limits. The negotiated peer receive MRU remains factual, including its
1500 default. All transmit producers either build a complete fitting packet,
shorten optional text/reflected Data, or fail visibly without committing an
unsent Configure reply. Both omission scenarios prove useful kernel service.

### Bugs Found/Fixed

The original interface setter ignored the admitted cap. The writer census
also found uncapped control/authentication output. Independent implementation
review then found false FSM advancement after an unsent oversized Ack;
`TestUnsentLCPAckCannotAdvanceNegotiation` and
`TestUnsentIPCPAckCannotAssignAddress` failed before the source fix and now
retain exact fitting-Ack controls. All fixes landed in `18ea5e830a`.

### Documentation Updates

Both user guides landed with runtime changes in `18ea5e830a`; both lab pages
landed with their producers in `3cc99259b0`. Wiki corrections are in `312639d`.
Root owns the native documentation-check result, recorded in Pre-Commit
Verification below. Child 16b's fulfilled runtime dependency now points to the
durable guide; the umbrella retains this still-open spec as the RFC repair owner.

### Deviations from Plan

The source census expanded the cap from the single IP setter to all userspace
transmit writers and the final defensive boundary; otherwise a valid small
session could still emit an oversized control/authentication frame. The old
68 floor was replaced by child 16d's explicit admission/negotiation checks;
the installed MTU never silently exceeds an accepted peer limit. The shared
unit carrier disables NCP to isolate installation; real CHAP/IPCP, traffic
and teardown are separately proved by both live omission scenarios.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first cap fixed only the interface setter. | Control and authentication senders also produce PPP Information fields. | Complete writer census and central driver tests. | Apply the same cap at each producer and refuse any missed oversized frame at `writeFrame`. |
| approach | Oversized required Configure replies could report success without sending. | The FSM must stop before AckSent/Opened and later-phase side effects. | Central pre-fix failures in the two real-handler Ack regressions. | Sender failures return false and publish session failure before transition commit. |
| approach | Only the helper test drove refusal of an unfit MS-CHAPv2 Failure prefix. | The RADIUS backend can supply a raw Reply-Message longer than the admitted transmit size, so its driver entrypoint also needs a refusing case. | Closure traced `radiusAuth.doRADIUS` Access-Reject through `startAuthDrain` and `Driver.AuthResponse` to the unchanged result guard. | Root adds a real-driver raw 80-byte failure case at MRU 68 with a fitting structured control and semantic discrimination. |
| approach | Implementation refreshed the changed full-receive discrimination records but left earlier records and its audit stale. | A passing test and two fresh claim records do not refresh proof fingerprints for every changed producer. | Native `./le rfc check` reports the precise owned audit and producer-changed population in Blocker. | Hand back to implementation for the RFC audit and 32 affected record files before another closure attempt. |
| none | No A-N assumption was broken. | A-1 is confirmed by source and both live carriers. | Closure source review and recorded kernel MTU measurements. | No scope reduction or replacement spec is needed. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Useful capped omitted-MRU L2TP service | Done | `session_run.go` `afterLCPOpen`; `lns_omitted_mru.go` `checkLNSOmittedMRU` | Actual Ze MTU 1492, CHAP/IPCP, traffic and teardown; cap-disabled measured 1500 fails. |
| Useful capped omitted-MRU PPPoE service | Done | `pppoe/server.go` `handlePADR`; `check_mru_omitted.go` `checkOmittedPeerMRU` | Fixed 1492 admission reaches the same setter; reciprocal useful traffic and cleanup. |
| One shared cap without replacing peer state | Done | `session_run.go` `transmitMRU`, `handleLCPPacket` | Raw peer default remains 1500; same implementation also serves proxy sessions owned by child 16b. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestOmittedMRUTransmitCap/l2tp`; `l2tp-lns-omitted-mru-xl2tpd` | Both installed cap and actual useful kernel service proved. |
| AC-2 | Done | `TestOmittedMRUTransmitCap/pppoe`; `pppoe-omitted-mru-ze-ac` | Ze-side 1492 assertion fails against the uncapped daemon. |
| AC-3 | Done | `TestTransmitCapPeerMRUBoundaries`; live MRU Nak and negotiated 1400 scenarios | 1400/1492/1500 inputs, correlated Nak convergence, 1492 cap and useful live controls. |
| AC-4 | Done | `TestTransmitCapRestartIsolation`, `TestReactorPPPPolicyReloadIsolation`, `TestReceiveFullFrameBelowConfiguredMRU` | Restart retains cap; reload isolates established policy; actual receive capacity stays 1500. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOmittedMRUTransmitCap` | Done | `ppp/transmit_cap_test.go` | Both transport inputs reach post-NCP setter and admin-up. |
| `TestTransmitCapRestartIsolation` | Done | `ppp/transmit_cap_test.go` | Two installations retain 1492 and raw peer 1500. |
| Omitted-MRU live cases | Done | `lns_omitted_mru.go`, `check_mru_omitted.go` and their scenario directories | Exact final names are the two AC carriers above. |
| Race and semantic discrimination | Done | Central Verification | Already-green unchanged runs are retained; no duplicate test run is owed by prose-only closure. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/ppp/session_run.go` | Done | Shared cap, MTU setter, reply bounds and final writer guard in `18ea5e830a`. |
| Shared PPP driver tests and live scenarios | Done | Five `transmit_*_test.go` files plus both omission checkers/tests and scenario configs; commits `18ea5e830a`, `3cc99259b0`. |
| `docs/guide/l2tp.md`, `docs/guide/pppoe.md` and wiki counterparts | Done | Directional cap descriptions with source anchors; commits `18ea5e830a`, `312639d`. |

### Audit Summary

All 14 audited requirement, AC, planned-test and file-group rows are Done;
none are Partial or Skipped. The source-census expansion is recorded above.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Omitted MRU establishes useful L2TP service within 1492 | Real peer/kernel interop and discrimination | `checkLNSOmittedMRU` requires actual omitted request/Ack, CHAP/IPCP, Ze MTU 1492, useful traffic and cleanup. Rebuilt uncapped daemon fails measured 1500; restored final image passes. |
| PPPoE omission cannot exceed fixed 1492 | Real peer/kernel interop and discrimination | `checkOmittedPeerMRU` correlates omission and reciprocal Acks, measures Ze interface 1492, sends traffic both directions and checks kernel release. Cap-disabled measured 1500 fails; restored artifact passes. |
| Share cap without breaking peer state, auth or receive capacity | Driver, lifecycle and semantic ioctl proof | Boundary/restart/auth/control/handler regressions preserve fitting results and events; full-receive carrier actually reads 1500 and checks ioctl 1500, then fails when that ioctl alone is changed to 1492. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Refresh the changed full-receive RFC audit and 32 producer-changed discrimination record files listed in Blocker. | This is implementation-phase proof maintenance, which closure must not author. | `plan/release/fixes/spec-home-cpe-16a-omitted-mru-transmit-cap.md` remains open and returns to implementation. |

Child 16b retains its separately assigned accepted-proxy proof and test judgment;
child 16d retains protocol-floor closure. Their work is not removed or closed
by this handback.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Not issued: closure handed back for the owned RFC freshness blocker. |
| `./le spec review check` | Not run; no closure CLEAN artifact is recorded while proof debt remains. |
| Rounds | 2 source-review rounds: full scope, then the additive guard-entrypoint regression and unchanged guard. The source review is clean; closure remains blocked by RFC evidence. |
| Reviewer lenses used | Logic+wiring/removed behavior; security+edge cases/guards; performance+completeness/Ze Go style. All ran inline over committed runtime `18ea5e830a`, carrier `3cc99259b0`, current producers and closure changes. |

### Native prechecks

| Command | Observed result and scope |
|---------|---------------------------|
| `./le repo check` | Reports 9 unused-export findings in policy, extract, perf, CodeQL and terminal-demo tooling. None names a CPE runtime/checker or closure path. The root owns the separately authorized broader cleanup. |
| `./le test weakened check base 18ea5e830a~1` | Reports 10 unexplained findings in schema helpers, plugin coupling, site plugin-registry tests and OSPF. No CPE test is flagged; accepted test judgments in the implementation commits remain effective. Root owns those separate changes. |
| `./le doc check all` | The observed check reports 143 published wiki/site catalog mode drifts, three stale inventory-symbol anchors on two main pages and a stale documentation index warning. No finding names the four CPE guide/lab pages. Root's documentation agent repairs the broader surfaces; this check is not recorded green. |
| `./le rfc check` | Successful invocation reports 131 violations across the working tree, including this spec's stale RFC1661-6.1-1 audit and 53 producer-changed entries in 32 PPP record files. See Blocker for the owned repair population; foreign client/BGP failures are not imported into this spec. The earlier launcher compilation refusal was not an RFC verdict. |
| `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-16a-omitted-mru-transmit-cap.md` | Declared design page `docs/research/l2tpv2-ze-integration.md` and two related pages were missing from the checklist. All three are now explicitly assessed in row16 below; root can check the updated declaration without a new source-review round. |

These are the direct root outputs in precheck log `87e54778`, doc log
`baf5a445` and RFC/source-census log `94001bcd`. They do not replace the source review or the targeted green
runtime/live evidence above. No foreign finding is attributed to this
spec's completed transmit-cap capability.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The unfit MS-CHAPv2 Failure prefix was rejected only in the helper test; no real handler/driver test drove that rejection and proved absence of a truncated wire Failure. | `mschapv2.go` `runMSCHAPv2AuthPhase`; `transmit_auth_cap_test.go` `TestMSCHAPv2FailurePrefaceCannotBeTruncated` | Additive `transmit_mschap_failure_driver_test.go` `TestMSCHAPv2UnstructuredFailureDriverCap`; baseline green, semantic mutant red and restored targeted race green observed. |

### Coverage diagnosis

The symptom is a missing driver-level rejecting case, not an observed runtime
misbehavior. `radiusAuth.doRADIUS` in
`internal/component/l2tp/plugins/authradius/handler.go` copies an Access-Reject
Reply-Message to the auth response without MS-CHAP formatting. The drain and
`Driver.AuthResponse` preserve those bytes. An 80-byte ASCII message therefore
reaches `runMSCHAPv2AuthPhase` at the valid IPv4-only admitted MRU 68 and has no
fitting structured prefix. The existing guard correctly refuses it.

The owning defect is test coverage. A [workaround] would lower the test's MRU
below admission limits or test only the helper again. The [source] fix is
`TestMSCHAPv2UnstructuredFailureDriverCap`, using the real Driver/AuthResponse
route and a valid fitting structured control. Allowing the helper's unfit
result through is a semantic counterexample: the bounded encoder would emit
a truncated Failure that the final size guard accepts. The regression must
reject that output. A normal typed V3 prefix fits; no oversized typed prefix
is claimed.

### Run 2

The focused second source review covers only the additive
`transmit_mschap_failure_driver_test.go` and the unchanged result guard. It
finds 0 source BLOCKER and 0 source ISSUE. The real Driver/AuthResponse path
uses admitted MRU 68 with IPv6CP off, waits for actual session completion,
asserts no truncated Failure or NCP/interface side effects for the raw result,
and retains an exact fitting structured-result control. Existing tests and
runtime producers are unchanged.

Root's targeted normal test passes both subtests (`b18482b9`). The private
helper mutation returns the unfit message as accepted while retaining encoder
and final writer bounds: the unstructured case fails on actual truncated
Failure bytes and incorrect lifecycle/auth reasons, while the fitting
structured case still passes (`ac5d8bfa`). The restored normal targeted race
run passes both subtests, PPP `ok` at 1.060s (`616e8ba2`). This resolves the
coverage ISSUE without a runtime change or coverage reduction. The separate
RFC freshness blocker still prevents formal closure and a CLEAN artifact.

The explicit style pass checked socket-reachable panics, names, lifecycle
obligations, duplicate values, initialized/validated inputs and return widths.
MS-CHAPv2's encoder panic is protected by the exact 20-byte response validation
in `runMSCHAPv2AuthPhase`; challenge producers use fixed stack arrays. The
new cap uses existing bounded pooled buffers and adds no data-path allocation.
Missing closure tables and stale draft/research wording were record NOTEs,
corrected in this edit without adding a review round.

RFC text was checked directly. RFC 1661 Section 6.1 states: "The default value
is 1500 octets." The same section requires: "If smaller packets are requested,
an implementation MUST still be able to receive the full 1500 octet information
field in case link synchronization is lost." RFC 2516 Section 7 states: "Since
Ethernet has a maximum payload size of 1500 octets, the PPPoE header is 6 octets
and the PPP Protocol ID is 2 octets, the PPP MTU MUST NOT be greater than 1492."
RFC 1332 Section 2.1 states: "The maximum length of an IP packet transmitted over
a PPP link is the same as the maximum length of the Information field of a PPP
data link layer frame." RFC 1334 Section 2.2.2 and RFC 1994 Section 4.2 make
Message display text independent of protocol operation; RFC 2759 Sections 5/6
instead define structured success/failure fields, which the capped writers
retain. Existing requirement summaries were checked; no new requirement or
proof claim is introduced by closure.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `ppp/transmit_cap_test.go`, `transmit_control_cap_test.go`, `transmit_auth_cap_test.go`, `transmit_config_cap_test.go`, `transmit_ack_state_test.go` | Yes | Closure read the actual tracked test bodies from `internal/component/l2tp/ppp/`; committed in `18ea5e830a`. |
| Both omission scenario configurations and checker tests | Yes | Actual checker producers and registered scenario paths read from `3cc99259b0` and current source. |
| `internal/component/l2tp/ppp/transmit_mschap_failure_driver_test.go` | Yes | Additive test body read in round2; root green/red/restored-race-green results verify both the refusing entrypoint and fitting structured control. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | L2TP omission installs 1492 and carries traffic | Closure re-read `checkLNSOmittedMRU`; central final restored image log `718e9ad2` records the live pass after measured 1500 discrimination. |
| AC-2 | PPPoE omission installs 1492 and carries traffic | Closure re-read `checkOmittedPeerMRU` and `omittedMRUZeLink`; central final `8044ad91` restored live pass follows the measured 1500 red. |
| AC-3 | Smaller/equal/larger peer limits stay binding | Closure re-read all five `TestTransmitCapPeerMRUBoundaries` cases and real request/Nak/Ack helper; central race pass covers them. |
| AC-4 | Restart/reload preserve admitted policy and receive capacity | Closure read `Driver.spawnSession`, `transmitMRU`, `afterLCPOpen`, restart test and the real receive-ioctl site. Central race/reload pass and semantic receive 1492 red remain the unchanged evidence. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| L2TP LNS admission | Existing native interop selector `l2tp-lns-omitted-mru-xl2tpd` | `handleKernelSuccess` → `StartSession.MaxMRU` → `Driver.spawnSession` → `afterLCPOpen`; live checker measures Ze MTU and traffic. |
| PPPoE AC admission | Existing native interop selector `pppoe-omitted-mru-ze-ac` | `handlePADR` → `StartSession.MaxMRU=PPPoEMaxMTU` → same driver/setter; registered checker requires both traffic directions and cleanup. |
| User configuration/reload preservation | `test/l2tp/lns-mru-auth-allow-config.ci` | Existing carrier retains explicit list ordering, effective defaults and refused values; central promoted pass and reload isolation tests preserve auth policy. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Both concrete transport admission producers feed the same stored `maxMRU`, and both live omission carriers discriminate actual Ze MTU 1492 against defective 1500. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Guide transmit/receive distinction and reply bounds | `afterLCPOpen`, `transmitMRU`, control/authentication writers; both guides' source anchors in `18ea5e830a` | Yes. Kernel receive 1500 and effective transmit minimum are separate facts. |
| Live omission procedure and measured peer | `checkLNSOmittedMRU`, `checkOmittedPeerMRU`, `omittedMRUZeLink`; both lab pages in `3cc99259b0` | Yes. Both measure Ze's interface and require useful traffic. |
| Nonapplicable config/API/SDK/inventory/comparison categories | Scoped runtime diff has no new grammar, command, RPC, event, plugin, metadata or metric registration; anchor census finds the guide/lab claims above. | Yes; separately owned schema/output work is not claimed here. |
| Declared and related design pages | `docs/research/l2tpv2-ze-integration.md` control/data split and IPv6 route policy; `docs/architecture/l2tp/bng-1-radius-attributes.md` typed failure causes; `docs/research/l2tpv2-implementation-guide.md` FSM/event mapping, checked against `afterLCPOpen`, `runNCPPhase`, `fail`, `codeToEvent`, `applyTransition`. | Unaffected by the transmit cap; now named as the native source census requires. |
| Native documentation check | `./le doc check all` directly reports published catalog drift and stale inventory documentation outside the CPE pages; see Native prechecks. | Run and disclosed; broader repair is owned by the root's documentation agent. |
| Native RFC freshness | `./le rfc check` ran and found the owned stale audit and producer records listed in Blocker. | Not satisfied; spec returns to implementation and remains on disk. |
| Source-anchor census | Native census identifies one declared design and two related pages; checklist row16 names and assesses them. | Declaration repaired in this handback; no design change or doc rewrite is required for the transmit cap. |

### Risks and observations

R-1 remains a maintenance constraint: negotiated peer receive capacity must
never be replaced by the configured cap in output; child 16f owns its unequal
output proof. R-2 is covered by the single shared setter, complete writer
census, final refusal guard and restart regression. No unresolved release risk
was found for this child's completed capability.
