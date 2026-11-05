# Spec: Cap transmit MTU when the peer omits MRU

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, config, docs |
| Priority | P1 |
| Release reason | A working subscriber can transmit packets larger than an accepted 1492 session limit. |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | verify |
| Updated | 2026-11-05 |

## Task

Own finding F-1 from `plan/release/fixes/spec-home-cpe-16-mru-enforcement-and-review-fixes.md`, which retains the original owner direction and review evidence. A peer that omits MRU must establish a useful session whose transmit MTU respects its admitted session cap. L2TP configured to 1492 and PPPoE's fixed 1492 cap must both hold.

This spec owns the shared transmit-cap implementation. F-2's proxy regression and protected test judgment belong to `plan/release/fixes/spec-home-cpe-16b-proxy-mru-transmit-cap.md`; implement one shared cap, not a second proxy-specific policy.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md`.
  → Constraint: keep transport-independent PPP setup shared between L2TP and PPPoE.
- [ ] `docs/guide/l2tp.md`, PPP negotiation and reload; `docs/guide/pppoe.md`, MRU ceiling.
  → Constraint: preserve admission-time policy isolation and correct each cap description with its implementation.
- [ ] RFC 1661 Section 6.1, RFC 2516 Section 7 and RFC 1332 in the repository authoritative texts.
  → Constraint: distinguish transmit limits from required receive capacity; verify existing requirement ownership before adding RFC tags.

## Current Behavior

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

## Draft Acceptance Criteria

| AC | Input / condition | Required outcome | Permitted survivors and why |
|----|-------------------|------------------|-----------------------------|
| AC-1 | L2TP cap 1492; peer omits MRU | Authentication and IPCP complete, installed MTU is 1492, and traffic succeeds without transmitting an Information field larger than 1492. | The session remains admitted because omitted MRU supplies a peer receive default, not permission to ignore local policy. |
| AC-2 | PPPoE peer omits MRU | The session passes useful traffic with installed MTU at most 1492. | Default pppd behavior remains interoperable within the transport cap. |
| AC-3 | Cap 1492; peer requests MRU 1400, 1492 or 1500 | Installed MTU is 1400, 1492 or at most 1492 respectively after valid negotiation; valid-sized traffic succeeds. | Smaller peer limits remain binding; a larger request can converge within the local cap. |
| AC-4 | LCP restarts or configuration reloads during established service | Replacement negotiation uses the admitted session policy; new sessions use the new policy. Required receive capacity and auth behavior remain intact. | Existing sessions retain their admitted policy, as the reload contract requires. |

## 🧪 Draft Test Plan

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
| A-1 | The shared interface setter reaches both transports. | Trace each transport's admitted `StartSession` and run both live cases. | Unvalidated; an alternate path must join the same enforcement before closure. |

## Research Remaining Before Ready

Complete the full design template, writer/consumer census, exact test and scenario names, RFC mapping, and integration/documentation checklists. Resolve low-MRU behavior with child 16d; its protocol-floor validation is separately owned. No implementation or closure is claimed here.

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
verification is complete; the spec stays open for the separate closure review.
