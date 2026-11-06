# Spec: Home CPE 16 MRU enforcement and review fixes

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol, config, cli, tooling, docs |
| Priority | P1 |
| Release reason | An accepted 1492 MRU policy still permits oversized transmissions on implemented subscriber transports. |
| RFC gaps | - |
| Depends | - |
| Phase | All eight children closed; final independent umbrella review and native closure checks complete |
| Handoff | verify |
| Updated | 2026-11-06 |

## Issue Split and Current Ownership (2026-11-04)

Execution instruction (2026-11-04): "use one agent per tasks to fix the issue,
they should not compile or run code and bring their work here for review where
you will centralise the compiling and running of the test they have authored".
Each issue has one author. The main thread owns formatting, builds, test runs,
proof records and commits. Authoring agents return uncommitted changes and
identify all unrun checks explicitly.

The owner requested: "create a spec per issue". This record is now a
coordination umbrella for the original review. The intact dated owner direction
and its derived design live in `docs/architecture/l2tp/subscriber-session-model.md`,
Subscriber transmit limit.
Each finding has one child spec in the routing table below. That table
supersedes the combined implementation scope and next action in the original
sections retained here. No issue is fixed by this split.

### Completed issue ownership

| Finding | Owning spec or completed result | Coordination |
|---------|-------------|--------------|
| F-1 implemented | `spec-home-cpe-16a-omitted-mru-transmit-cap`; `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, both interop lab pages | Shared cap and live omission proof landed in `18ea5e830a` and `3cc99259b0`; `1a32d7f93d` refreshes all 53 owned records and three audits. The fresh independent closure review is CLEAN. Both transports install measured Ze MTU1492, pass useful traffic and fail on actual MTU1500 with the cap removed. Normal images are restored; broader unrelated findings remain disclosed. |
| F-2 implemented | `spec-home-cpe-16b-proxy-mru-transmit-cap`; `docs/guide/l2tp.md`, `docs/labs/l2tp-interop.md` | The shared cap admits peer1500/9000/omission with measuredMTU1492, useful1492-byte ICMP and cleanup. Cap-disabled actualMTU1500 fails; restored native proof and original accounting companion pass. Independent closure confirms the protected row-name-only judgment and retained authentication assertions. |
| F-3 implemented | `spec-home-cpe-16c-auth-policy-repair-commands`; `docs/guide/l2tp.md`, `docs/guide/pppoe.md` | Executable transport-specific repairs and real editor regressions landed in `18ea5e830a`; matching wiki examples landed in `312639d`. Independent closure reviews the message producers and parser against the guides. |
| F-4 | `spec-home-cpe-16d-mru-protocol-floors` | Protocol-floor validation and strict proxy MRU interpretation are implemented. `docs/guide/l2tp.md` documents 68..1500 and default-enabled IPv6CP's 1280 floor; `features/l2tpv2-bng.md` distinguishes IPv4 interop from the driver-only IPv6CP proof. Full subscriber IPv6 remains in `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` under the existing owner ruling. |
| F-5 implemented | `spec-home-cpe-16e-live-mru-nak`; `docs/labs/l2tp-interop.md`, l2tp-lns-mru-xl2tpd | Direct rebuilt upper-MRU-guard fault correctly fails on missing Nak despite CHAP/IPCP. Fresh rebuilt normal native service restores1493→Nak1492→retry1492/Ack, CHAP/IPCP, measured MTU1492, useful traffic and complete cleanup after Docker resource recovery. |
| F-6 implemented | `spec-home-cpe-16f-negotiated-mru-output-proof`; `docs/labs/l2tp-interop.md`, l2tp-lns-negotiated-mru-xl2tpd | Independent closure source review is CLEAN. Config1492, wire/session1400 and measured Ze MTU1400 pass with traffic; a rebuilt configuration-copy publication fails on aggregate1492 versus1400, then the normal artifact passes. Null/down and negotiated authentication coverage remain intact. |
| F-7 implemented | `spec-home-cpe-16g-auth-refusal-order-proof`; `docs/labs/l2tp-interop.md` | Checker/regressions landed in `3cc99259b0`, generic build-ID progress in `0bef13eda7`. A real early peer Ack passes the old checker and fails the fixed checker on identical retained images. Normal rebuilt refusal and permitted CHAP traffic pass. |
| F-8 implemented | `spec-home-cpe-16h-subscriber-radius-pppd`; `docs/labs/l2tp-interop.md`, Peer-source basis | Paired pppd 2.5.5 source/hash pins and version/plugin tests landed in `3cc99259b0`. Both native image probes, all four subscriber-RADIUS scenarios and both L2TP policy scenarios pass. |

All eight children completed implementation, independent closure review and the
two-commit closure. The table records completed ownership, not remaining work.
The final umbrella review checks their combined implementation and evidence
against all eight original outcomes; the closure history below retains each
child's commits.

## Task

Fix all eight findings from the owner's review of
`spec-home-cpe-16-lns-mru-and-auth-allow-list`. The earlier spec was removed
at closure; this spec owns its remaining defects. Auth allow-list enforcement
is a preservation requirement. Recording these findings does not fix them or
establish that the earlier MRU goal is complete.

The final implementation and proof are committed. This coordination record
retains the owner's original findings and the combined closure evidence. No
child closure substitutes for the final whole-scope source review below.

### Owner direction (2026-11-04)

The governing verbatim excerpts and the design derived from them are retained
on the durable page `docs/architecture/l2tp/subscriber-session-model.md`,
Subscriber transmit limit. That page carries the original date and preserves
the intact transmit-cap instruction without reconstructing the truncated text.

## Review Findings to Resolve

The table preserves the reported findings and their proof obligations.
The original defect descriptions remain historical; Current Behavior and the
closure tables record the final source and directly observed central results.

| ID | Severity | Reported defect | Required outcome |
|----|----------|-----------------|------------------|
| F-1 | Blocker | Omitted peer MRU defaults to 1500, exceeding a 1492 limit on L2TP and PPPoE. | A working session with omitted MRU installs transmit MTU at most its session cap; PPPoE remains at most 1492. Prove useful traffic as well as size enforcement. |
| F-2 | Blocker | Accepted proxy LCP can install peer MRU 1500 or 9000 despite local MRU 1492. | Cap installed transmit MTU on the shared session path. Preserve admissible proxy sessions and existing authentication checks; do not add a proxy rejection as the size fix. |
| F-3 | High | Refusal and PPPoE config errors print bracketed list forms the editor cannot accept, and a refusal prints repairs for both transports. | Execute each printed repair through the real editor, verify the resulting list, and print only the relevant transport's repair. |
| F-4 | High | MRU 64..67 conflicts with IPv4's 68 floor; MRU below 1280 is accepted while IPv6CP defaults on. | Validate protocol size floors consistently at schema/runtime entry points. Define and test IPv4-only and IPv6-enabled boundaries without silently raising an agreed peer limit. |
| F-5 | Medium | Original AC-1's peer MRU 1500 Nak is not exercised live; the scenario peer requests 1492. | Read the pinned pppd source and use a value it emits explicitly, such as 1493..1499. Prove request above cap, matching Nak to 1492, convergence and working data traffic. Keep unit coverage of explicit 1500. |
| F-6 | Medium | Original live AC-15 compares 1492 configured with 1492 negotiated. | Use unequal configured and negotiated values and correlate session output with the wire exchange, so copying config fails. Preserve null lifecycle values and negotiated authentication output. |
| F-7 | Medium | L2TP refusal checker allows a ConfAck before TermReq. | Reject traces that open LCP before refusing required authentication. Add a negative checker fixture containing that false-positive trace, alongside a valid refusal trace. |
| F-8 | Medium | Subscriber RADIUS LAC image remains on pppd 2.5.4 while the paired L2TP image uses 2.5.5. | Align both images with latest stable upstream at implementation time, verify hashes and plugin paths, rebuild both, and run their affected scenarios. |

## Required Reading

- [ ] `ai/doors/system.md` and `ai/doors/access-security.md`.
  → Constraint: PPP stays a transport-independent leaf; subscriber handlers remain shared.
- [ ] `ai/doors/le-tooling.md` and `docs/architecture/testing/interop.md`.
  → Constraint: prove live assertions discriminate the defect, including a rebuilt defective artifact where required.
- [ ] `docs/guide/l2tp.md`, PPP negotiation, session output and reload sections; `docs/guide/pppoe.md`, authentication and MRU sections.
  → Constraint: correct the cap description and keep configured, negotiated and installed values distinct.
- [ ] RFC 1661 Section 6.1, RFC 2516 Section 7, RFC 1332 and RFC 5072 Section 2, using repository authoritative texts and existing summaries.
  → Constraint: preserve required receive capacity while limiting transmitted Information-field size. Confirm exact minimum-size rules before selecting validation changes.
- [ ] `plan/templates/TEMPLATE.md` and `plan/learned/DESIGN-HISTORY.md`.
  → Constraint: finish the source census, checklists and design admission before implementation.

## Current Behavior

**Final source checked during independent closure (2026-11-06):**

| Producer | Current behavior |
|----------|------------------|
| `ppp/session_run.go`, `transmitMRU`, `afterLCPOpen`, `writeFrame` | Installed MTU and every userspace send obey `min(peer MRU, admitted session cap)`; omission means peer1500. Kernel receive MRU remains1500. An unsent required response stops negotiation before its next state is committed. |
| `ppp/proxy.go`, `EvaluateProxyLCP`, `decodeProxyMRU`; `ppp/auth.go`, `acceptProxyPolicy` | Exact-width, unique converged MRUs preserve absence separately from explicit0. Invalid/undersized streams trigger fresh LCP. Valid1500/9000/omission remain admitted and capped without changing authentication policy. |
| `config_policy.go`, `validateMRUPolicy`; `ppp/mru_policy.go`, `ValidateNetworkMRU`, `ipv6MRUSupported` | IPv4 requires68; enabled IPv6CP admission requires1280. Negotiated smaller links retain independent IPv4 and refuse IPv6 service. YANG, extraction, startup/reload and direct driver admission agree. |
| `ppp/auth.go`, `refuseAuthMethod`; `pppoe/config.go`, `ExtractParameters` | Repairs contain executable per-member editor commands for the relevant transport; preserving existing allowed methods requires no automatic policy change. |
| `reactor_kernel.go`, `handleLCPUp/Down`; `snapshot.go`, `sessionSnapshot`; `cmd/l2tp.go`, `sessionJSON` | Config1492, negotiated peer1400/1500 and installed MTU remain distinct. Unopened/down negotiated values serialize as null; negotiated no-auth is `none`. |
| Paired LAC Dockerfiles and `interoplab/lac_images*_test.go` | Both pin pppd2.5.5 and xl2tpd1.3.20 with matching archive hashes. Native probes check the rebuilt pppd version and versioned plugin path. |

**Preserved:** permitted-method enforcement across Nak, Reject and proxy paths;
required-auth refusal before Opened; admitted valid proxy sessions; isolated
session policy across reload; full PPP1500 receive capacity; and useful IPv4
subscriber traffic. The owner ruled on2026-08-30 that full subscriber IPv6 is a
post-release feature. The1280 positive carrier proves driver/NCP behavior with a
fake allocator, not a full subscriber path; `poolPlugin.handle` still declines
IPv6. Its existing owner is `plan/post-release/features/spec-l2tp-ipv6-subscriber.md`.

## Existing Mechanisms

| Mechanism | Location | Current purpose | Direction |
|-----------|----------|-----------------|-----------|
| Shared PPP interface setup | `pppSession.afterLCPOpen` | Installs MTU for both transports and accepted proxy sessions. | Extend this existing path with the session transmit cap. |
| Proxy LCP parsing | `EvaluateProxyLCP` | Keeps the two directions' negotiated MRUs separate. | Preserve parsing and admission; enforce transmit policy at installation. |
| Transport config and session policy | L2TP MRU leaf and shared PPP driver | Supplies a per-session limit; PPPoE uses 1492. | Trace all producers, restarts and consumers before implementation. |
| Existing interop policy scenarios | L2TP and PPPoE lab suites | Exercises MRU/auth negotiation and output. | Strengthen live inputs and assertions instead of adding parallel harnesses. |

## Data Flow

Configuration and transport defaults enter `StartSession` through
`handleKernelSuccess` or PPPoE `handlePADR`; `Driver.spawnSession` validates and
copies the admitted policy. Normal LCP or accepted proxy LCP sets the raw peer
receive MRU. `afterLCPOpen` authenticates, negotiates the independent NCPs and
installs `transmitMRU`; userspace senders and their final writer use the same
limit. LCP-up/down events update reactor state, snapshots and both command
views. Restart resets negotiation and network resources while retaining the
admitted cap; reload affects only new sessions.

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Treating a transmit cap as proxy admission policy rejects working subscribers. | Oversized peer MRU fails admission instead of producing a capped interface. | Keep positive admitted-proxy tests and assert installed MTU. |
| R-2 | Changing the named proxy test row weakens its authentication coverage. | New test checks only size or deletes a policy assertion. | Obtain independent `ze-test-judge` approval before changing protected expectations; retain auth and admission coverage. |
| R-3 | Clamping upward exceeds a peer's small receive MRU. | Interface floor exceeds the peer limit. | Resolve floors at validation/negotiation; test lower boundaries explicitly. |
| R-4 | Session output is confused with installed MTU after adding a local cap. | Output reports config as negotiation. | Specify field meaning and test unequal values across wire, snapshot and interface. |

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | A pppd value in 1493..1499 provides the explicit oversized request needed for live Nak proof. | Owner's review. | Scenario does not exercise the intended Nak. | Pinned pppd `lcp_addci`/`lcp_nakci` and final native request1493/Nak1492/retry1492/Ack in `job-cpe16e-nak-restored-after-recovery-c2b9a595.log`. | confirmed |

## Acceptance and Test Plan

Each F-N row is a required outcome, mapped one-to-one to AC-N and the final
implementation audit below. The completed unit, boundary, functional and
interop carriers include
omitted, smaller, equal and larger peer MRUs, accepted proxy values 1500/9000,
both transports, restart and reload. Check installed interface MTU and useful
data delivery. The boundary matrix covers64,67,68,1279,1280,1492 and1500 with IPv6CP enabled
and disabled; separate proxy cases cover explicit0, malformed widths and duplicates.

Independent `ze-test-judge` review is required for the proxy expectation
change and every other protected test change. Admission can remain accepted;
the incorrect implication that accepted peer MRU may raise installed MTU
above the session cap must change.

The central runs below cover scoped L2TP/PPP/PPPoE race tests, actual editor
workflows, strengthened policy interop and rebuilt subscriber-RADIUS images.
Actual rebuilt-producer counterfactuals discriminate omission, proxy cap,
Nak, output publication and refusal ordering; source reachability is not
counted as semantic proof.

## Documentation and Closure Context

The implementation updated the L2TP/PPPoE guides, both lab pages, relevant
architecture/research pages and wiki operator examples. Source anchors connect
cap, floor, auth, lifecycle and checker claims to their deciding functions.
`features/l2tpv2-bng.md` describes the actual scoped proof while retaining its
Experimental (partial) level. No full BNG or subscriber-IPv6 claim is made.
The original three weakened-tooling failures were subsequently fixed at their
producers in `f16fe6136b`; its complete owning packages and config corpus pass.
The broader repository, RFC and published-site findings remain separate limits
and are named below rather than represented as green.

## Owned journal rows

No rows were filed by the review. This spec owns F-1 through F-8 directly.

## Historical Implementation Review and Central Validation

Rows1 and2 are the original independent implementation review. Rows3 and4
are coordinator validation, not independent source-review rounds. They are
retained as history and are not added to the fresh final closure round count.

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
| 1 | Eight-agent implementation: correctness, security and coverage | Independent review agent | Found false FSM advancement after unsent Acks and proxy peer MRU below68 bypassing the floor. Both fixed after centrally observed regression reds. | `job-cpe16-handler-and-floor-red-7e7e4fa1.log`, then `job-cpe16-handler-and-floor-fixed-d12ee1a2.log` |
| 2 | Both source fixes and the native accounting regression carrier | Independent review agent | Product blockers resolved. Atomic accounting status-publication note fixed; actual original accounting fixture then passed. | `job-cpe16-final-l2tp-live-e6b8eb67.log` |
| 3 | Scoped race, configuration stress, actual kernel/data-plane tests | Main coordinator | All L2TP packages and both checker packages pass race; both configuration carriers pass 80/80 stress runs and promoted execution; all normal live scenarios pass. | Central verification below. |
| 4 | Rebuilt-artifact semantic discrimination and restoration | Main coordinator | Both transport omission faults and proxy oversize fault fail on actual MTU 1500; copied-output fault fails on 1492 versus peer 1400; old refusal checker passes actual early peer Ack, fixed checker rejects it. All final normal artifacts and controls pass after restoration. | Named fault logs below and `job-cpe16-final-artifacts-restored-718e9ad2.log`. |

## Closure state

All eight child specs have landed their implementation/evidence commit A and
removal commit B. The fresh final independent umbrella source review is CLEAN.
The final native review/hash and citation-anchor checks pass. The final RFC
check reports only79 foreign findings; all owned evidence is fresh and the
summary update passes. This is the frozen closure snapshot. The coordinator
runs its one two-commit closure script before reporting actual closure.

## Central Verification (2026-11-05)

The owner requested one author per issue and centralized execution. Eight
issue agents authored code/tests without compiling or executing them. The
main coordinator formatted, compiled, ran tests, reviewed independent
findings and recorded the observed results. Earlier author notes marked
NOT RUN are historical handoffs, superseded only by the named central runs.

The runtime fix is committed as `18ea5e830a`, and the live carriers, image
alignment and helper changes as `3cc99259b0`. All committed distro,
appliance, setup, host and installer flavors compile after both Go commits.
The matching L2TP and PPPoE wiki corrections are committed as `312639d`.
No push was performed by this coordinator.

| Finding | Observed central result |
|---------|-------------------------|
| F-1 | Both transport omission carriers authenticate, complete IPCP, install measured Ze MTU 1492 and pass traffic. Each rebuilt uncapped artifact fails on actual MTU 1500, then the rebuilt normal artifacts pass again. |
| F-2 | Accepted proxy peer MRUs1500,9000 and omission retain useful CHAP/IPCP and1492-byte ICMP with actual MTU1492 and per-session cleanup. A rebuilt uncapped artifact fails on actual MTU1500; the restored artifact passes all cases again. |
| F-3 | Printed repairs run through the real editor/dispatcher. Both transport-specific refusal/counter cases pass under race, retaining the original authentication-policy assertions. |
| F-4 | Configuration, startup/reload, driver admission and negotiated IPv6 guards pass. Proxy1/64/67 now renegotiate and a corrected68 session remains useful. Both promoted config tests pass80/80 stress invocations each and registered execution. |
| F-5 | Actual pppd1493 Configure-Request receives correlated Ze Nak1492, retries1492 and receives its matching Ack; CHAP/IPCP and useful traffic complete. |
| F-6 | Actual wire/session MRU 1400, configured 1492 and installed MTU 1400 remain distinct in both command views. Rebuilding the real publication path to copy configuration fails on reported 1492 versus peer 1400; the rebuilt normal artifact passes again. |
| F-7 | Required-auth and PPPoE refusals pass; permitted CHAP traffic remains useful. Removing the guard fails the new early-Ack unit/registered-checker cases. A real early peer Ack passes the old checker and is rejected by the fixed checker against the same faulty image. The rebuilt normal artifact and traffic controls pass again. |
| F-8 | Both rebuilt images report pppd2.5.5 and the matching installed plugin. Release pins/hashes agree; all four native subscriber-RADIUS scenarios pass. |

The initial central coverage measurement was82.6% for PPP, with complete
measured coverage of the shared transmit-limit, final-writer and LCP/NCP
Configure-Ack helpers. It is historical coverage, not a new final measurement.
The changed full-receive RFC1661 test has native semantic discrimination
records for both claims: changing the actual kernel receive-MRU argument
to1492 fails its required1500 assertion. This is separate from the transmit
cap and does not count a halted producer as behavior proof.

The final normal L2TP log includes all five selected scenarios: live Nak,
CHAP-only refusal, both original accounting fixtures, omitted MRU and
distinct negotiated output. It is
`tmp/session/2026-11-04-222204/scratch/job-cpe16-final-l2tp-live-e6b8eb67.log`.
Other named logs are under the same coordinator scratch directory.

The final restored artifact log is
`job-cpe16-final-artifacts-restored-718e9ad2.log`: required-auth refusal,
omitted-MRU MTU 1492 and distinct negotiated output all pass. The fault logs
include `job-cpe16-l2tp-omitted-cap-disabled-22d28569.log`,
`job-cpe16-pppoe-cap-disabled-0d495f97.log`,
`job-cpe16-copied-output-red-1d421026.log`,
`job-cpe16-live-early-ack-old-checker-8708cdb4.log` and
`job-cpe16-live-early-ack-fixed-checker-a3f0496b.log`. These are observed
semantic outcomes, including the old checker's actual false positive.

The original configuration corpus failures and three weakened-tooling reds
were fixed in `f16fe6136b`: the two complete owning packages and config corpus
(2884+241 cases) pass. An initial wider fixture run had four unrelated le-on-PATH
failures; the affected parser, native inventory and original accounting proof
passed. The remaining broader9 repository and10 weakening findings are foreign.
The full documentation check still reports142 published gh-pages drifts;
targeted discovery/index/source checks pass after repair. Foreign inventory
source-model work prevents a complete site build. None is claimed globally green.

## Executive Summary

All eight original defects are implemented and independently reviewed. The
combined source uses the shared PPP cap for normal and accepted proxy sessions,
retains full receive capacity and authentication policy, and supplies live
counterfactual proof rather than equal configured/negotiated values or checker
success alone. This umbrella adds closure prose, its surviving citer repair, durable owner-requirement record and
an accurate RFC5072 testing-gap annotation; it adds no runtime change.

Surviving risks R-1 through R-4 are covered respectively by admitted-proxy
traffic, the independent protected-test judgment, explicit floor/zero/width/
duplicate controls, and separate wire/config/list/detail/interface evidence.
Full subscriber IPv6 is the already owner-approved post-release feature. The
remaining foreign repository/RFC/site findings do not establish global health.

## Changed-Wire Census and Architectural Verification

Source census searched `StartSession`, `MaxMRU`, `SetMTU`, `setMRU`,
`writeFrame`, `chanFile.Write`, `NegotiatedMRU`, `mru`, `ConfReq`, `ConfNak`,
`ConfAck` and the scenario names across `internal/`, `test/` and `docs/`.
Positive controls are the actual L2TP/PPPoE session producers and registered
native checker maps. No new packet field or JSON key was introduced; limits,
admission and the meaning proved by existing carriers changed.

| Wire or boundary | Writers and readers checked | Preservation/change |
|------------------|-----------------------------|---------------------|
| Session policy | `l2tpReactor.handleKernelSuccess`, PPPoE `InterfaceServer.handlePADR` → `Driver.spawnSession` | L2TP copies effective session MRU; PPPoE supplies1492. Admission validates defaults/floors before descriptors transfer to a session. |
| LCP/proxy options | `sendConfigureRequest`, `EvaluateProxyLCP`/`decodeProxyMRU`, `handleLCPPacket`, `acceptProxyPolicy`; fixture `tunnelL2TPOpenProxySession`; pppd option files | Explicit widths/presence/duplicates are validated; omission remains1500. Invalid converged proxy options renegotiate; valid oversized receive capacities remain usable under the local transmit cap. |
| PPP userspace transmit | LCP, IPCP/IPv6CP, PAP, CHAP and MS-CHAPv2 producers → `writeFrame` | Every complete frame is capped. Display text/reject payloads may shorten; mandatory auth fields and Configure option sets are never partly sent. All fitting positive controls remain useful. |
| Kernel PPP receive/transmit | `afterLCPOpen`, driver channel/unit readers, ioctl/backend | Receive buffers/ioctl retain1500. Installed IP MTU uses the smaller peer/session limit with no framing subtraction or upward clamp. Restart retains the admitted policy. |
| Negotiation JSON | `EventLCPUp/Down` → reactor → `sessionSnapshot` → `sessionJSON`; REST command checker consumers | Raw negotiated MRU/auth and null lifecycle remain; configured1492, peer1400 and installed1400 are asserted separately in both views. |
| Outside-language carriers | pppd `ppp-options`, xl2tpd configs, both Dockerfiles, embedded `radiusFixtureWorker` shell and its status reader | Peer1493 emits a real Nak sequence; default peer omits MRU. Hash checks precede extraction; accounting status is published atomically after worker cleanup. Fixed shell arguments and a private directory remain bounded to the owned lab. |
| Native scenario/proof | `scenarioCheckerMap`, PPPoE `wireCheckers`, fixture registration, `Suite.prepareImages`/`resolvedPeers` | Existing discovery reaches all new carriers. Successful local builds retain the consumed immutable reference; same-image refusal comparison and full normal restoration are distinct evidence. |

| Architectural check | Holds? | Evidence |
|---------------------|--------|----------|
| Intended layers and component boundaries | Yes | Shared PPP leaf owns the limit; transports provide policy through `StartSession`; unchanged subscriber handler registries own auth/pool. |
| Registration over hardcoding | Yes | MRU validator registers in `l2tp/register.go`; existing scenario/checker/fixture registration reaches tests. No new product command, plugin, event or central engine feature case. |
| No duplicated runtime mechanism | Yes | `transmitMRU` is the common effective-limit producer; peer negotiation and operator policy retain separate facts. |
| Buffer/resource ownership | Yes | Existing fixed pooled buffers and descriptor ownership remain. New reply views bound writes; `spawnSession` closes rejected transferred files; fixture socket ownership is documented. |

## Wiring Test and Acceptance Criteria

| Entry point | Feature code | Concrete carrier |
|-------------|--------------|------------------|
| Normal L2TP/PPPoE subscriber | `spawnSession` → LCP/auth/NCP → `afterLCPOpen` | `TestOmittedMRUTransmitCap`, both omission interop scenarios |
| ICCN proxy AVPs | `EvaluateProxyLCP` → `acceptProxyPolicy` → same setup | `TestProxyTransmitMRUCap`, `TestProxyMRUOptionEncodingDriver`, `TestProxyDuplicateMRUOptionDriver`, native proxy fixture |
| Printed operator repair | `refuseAuthMethod`/`ExtractParameters` → actual editor parser | `TestAuthRefusalRepairCommands`, `TestPPPoEAuthPolicyRepairCommands` |
| Configuration and reload | YANG/tree validator, extractor, startup, provider reload, driver | `test/l2tp/lns-mru-protocol-floors.ci`, `lns-mru-auth-allow-config.ci`, `TestReactorPPPPolicyReloadIsolation` |
| Native L2TP MRU/output/refusal | Scenario discovery → registered checker → real peer and daemon REST | `l2tp-lns-mru-xl2tpd`, `l2tp-lns-negotiated-mru-xl2tpd`, `l2tp-lns-chap-only-xl2tpd` |
| Paired rebuilt peers | Both suite image declarations → Dockerfile build → native probes/scenarios | `TestLACPeerReleasePinsAgree`, `TestLACPeerRuntimeVersions`, four subscriber-RADIUS scenarios |

| AC ID | Observable result | Permitted survivors and why |
|-------|-------------------|-----------------------------|
| AC-1 / F-1 | Omitted MRU on both transports yields useful authenticated traffic with measured Ze MTU1492. | Peer omission still means receive capacity1500; it cannot override the admitted cap. |
| AC-2 / F-2 | Proxy peer1500/9000/omission remains authenticated and useful at measured1492 with full-sized ICMP and cleanup. | Oversized receive capacity is valid; it is not a reason to reject proxy admission. |
| AC-3 / F-3 | Every printed repair executes through the actual editor and yields the intended ordered policy. | Existing permitted methods survive; adding a method remains an explicit operator choice. |
| AC-4 / F-4 | Invalid protocol floors/defaults are refused at all entries; valid68 IPv4 and1280 driver/NCP controls work without raising the peer limit. | Smaller IPv4-only sessions and independently useful IPv4 remain; full subscriber IPv6 has its existing post-release owner. |
| AC-5 / F-5 | Actual1493 request receives matching Nak1492, retries1492 and receives matching Ack before CHAP/IPCP/traffic. | Literal1500 is retained in unit proof because pinned pppd omits its default option. |
| AC-6 / F-6 | Config1492, wire/session1400 and measured Ze MTU1400 remain distinct; both session views agree. | Unopened/down values remain null and negotiated no-auth remains `none`. |
| AC-7 / F-7 | A real peer Ack of Ze's request before TermReq fails the refusal checker. | Ze's Ack of the peer request alone is a valid Ack-Sent survivor; normal refusal and allowed CHAP traffic pass. |
| AC-8 / F-8 | Both LAC images use matching pppd2.5.5 pins/runtime/plugin path and preserve all affected scenarios. | Existing subscriber authentication/accounting scenarios remain intact. |

## Deliverables Checklist

| Deliverable | Verification method and result |
|-------------|--------------------------------|
| F-1 shared cap and receive preservation | Driver normal/restart/boundary tests; two real omission carriers; cap-disabled rebuilt reds and restored greens. Done. |
| F-2 valid proxy service | Full PAP/IPCP driver, original CHAP/accounting fixture and real kernel proxy1500/9000/omission with1492-byte ICMP. Done. |
| F-3 executable repairs | Both named actual-editor tests, original refusal counter/policy assertions and old-producer discrimination. Done. |
| F-4 consistent floors | YANG/config/startup/reload/driver matrix, negative/positive IPv6 gates, corrected68 service, final20-case proxy driver proof. Done. |
| F-5 live Nak | Pinned pppd source and1493 packet trace,11 negative/correlation controls, actual Nak-disabled producer red and full normal restoration. Done. |
| F-6 measured output | Unequal1400/1492 wire/JSON/kernel proof, independent list/detail negatives, config-copy producer red and normal restored green. Done. |
| F-7 refusal ordering | Old/current checker on identical immutable faulty images; direction controls and restored refusal/service. Done. |
| F-8 rebuilt peer alignment | Paired source/hash test, native runtime/plugin probes, subscriber-RADIUS4/4 and L2TP policy2/2. Done. |

## Security Review Checklist

| Concern | Source check and result |
|---------|-------------------------|
| Untrusted MRU/option bounds | `ParseLCPOptions`, `decodeProxyMRU`, admission floors and final writer distinguish absence/zero, reject malformed/duplicate converged MRUs and bound complete sends. No overflow or peer-sized allocation introduced. |
| Auth downgrade/fail-open | `acceptProxyPolicy`, Nak/Reject handling and `applyTransition` preserve explicit allowed methods and prevent required-auth Opened without a method. PAP/CHAP/MS-CHAPv2 result codes/digests remain independent of optional display truncation. |
| Mandatory response integrity | Configure replies fail before FSM advancement; MS-CHAPv2 oversized mandatory fields fail visibly. Fitting replies and accepted auth controls still succeed. |
| Resource exhaustion/ownership | Fixed1502-byte pool; finite proxy option scans and bounded authentication/restart/context waits. Rejected driver admission closes both owned descriptors; fixture cleanup closes the socket and removes measured PPP state. |
| Injection/path traversal | Repair methods come from validated enums. Native checker commands use argument vectors; fixed shell scripts use owned paths and positional arguments, not peer-controlled shell text. No new product file path or authorization surface. |
| Race/restart/TOCTOU | Session snapshots and interface lifecycle retain their locks and acknowledgement ordering. LCP restart withdraws old NCP resources before replacement; final race evidence includes proxy fallback/restart and untouched transports. |
| Secrets/crypto/logs | No new credential or crypto algorithm. Existing lab-only credentials remain local fixtures; output records negotiation and measured state. Repair text requests explicit operator policy choice. |
| Panic/style | New registration panic is startup-only for duplicate validator registration, not peer input. Authentication method conversion receives validated enums; no new socket-reachable panic, worker, cache or unbounded retry. Names, paired ownership, bounded copies, raw-input validation and return contracts were reviewed against the Go style guide. |

## Documentation Update Checklist

| # | Category | Applies? | Commit/source-aware disposition |
|---|----------|----------|---------------------------------|
| 1 | Feature scope/evidence/level | Yes | `features/l2tpv2-bng.md` in `840b0fdd69` corrects68/1280 and distinguishes actual IPv4 interop from driver-only IPv6CP. Central feature report retains Experimental (partial); no promotion. |
| 2 | Config syntax | Yes | `18ea5e830a`: guide MRU range/floors and executable per-member editor syntax; YANG68..1500 and real editor/parser confirm examples. The generic config-language grammar is unchanged. |
| 3 | CLI command | Yes | Existing refusal/config error text changes; `docs/guide/l2tp.md` and `docs/guide/pppoe.md` anchors name `refuseAuthMethod`/`ExtractParameters`. No command registration/name/flag change requires a reference inventory change. |
| 4 | API/RPC | No | `sessionJSON` and existing config/list/detail handlers retain keys and null contracts; source-anchor search finds their claims in the guides, already updated there. No RPC schema/route changed. |
| 5 | Plugin behavior | No | No plugin registration or auth/pool contract changes. `bng-1-radius-attributes.md` and `subscriber-session-model.md` source claims remain accurate for preserved accounting and session lifecycle. |
| 6 | User guides/wiki | Yes | Main guides in `18ea5e830a`; wiki L2TP/PPPoE corrections `312639d`; wiki generated catalog `ebb7c93`. Cap, receive capacity, floors, repairs and raw/null output match their named producers. |
| 7 | Wire format | No | Existing PPP/LCP/NCP and proxy AVP formats/fields remain; source census verifies only permitted values, sizes and responses changed. Existing wire codec diagrams remain accurate. |
| 8 | Plugin SDK/protocol | No | No `pkg/plugin`, `pkg/ze`, RPC/event encoding or cross-process payload edit. Source-anchor search gives no changed SDK contract. |
| 9 | RFC behavior/proof | Yes | `1a32d7f93d`, `426b892b43`, `7e2fb4953f`: owned discrimination/audits current after implementation repairs. This closure corrects `rfc/short/rfc5072.md`'s stale implementation annotation while retaining RFC5072-2-2 as an unmet testing gap; no new proof/coverage claim. Final `job-cpe16-umbrella-final-rfc-check-722fbda6.log` has79 foreign findings and no owned/summary finding; `job-cpe16-umbrella-final-rfc-update-8119b55c.log` passes, rendering229 ignored shards and derived indices. |
| 10 | Test infrastructure | Yes | `3cc99259b0`: both lab pages describe measured/kernel/wire proof and peer pins. `0bef13eda7`: owned six-line `docs/architecture/testing/interop.md` paragraph anchors successful local-build identity to `Suite.prepareImages`. Foreign whole-page edits are not included. |
| 11 | Comparison table | No | Existing L2TP/PPPoE capability remains partial; no daemon comparison capability changes. Source `poolPlugin.handle` still refuses IPv6; no support promotion is published. |
| 12 | Architecture/design | Yes | Source-anchored guides, lab and research pages describe cap/floors/auth results. Declared `docs/research/l2tpv2-implementation-guide.md`, `l2tpv2-ze-integration.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `bng-1-radius-attributes.md`, `bng-5-pppoe.md` and `docs/architecture/testing/interop.md` were assessed at their reached producers; unchanged topology/boundaries stay accurate. |
| 13 | Route metadata | No | No route metadata key or producer changes; source census contains no new metadata contract. |
| 14 | Prometheus metrics | No | Existing refusal counter is preserved and checked exactly once per refusal by both transport subtests; no metric/label registration changes. |
| 15 | Runtime inventory | No | No product plugin, event, send type, command or capability added. The tree validator and test scenario/fixture registrations do not add an operator command. Main help/inventory corrections are committed in `d5efcf8bd3`. |
| 16 | Existing anchors/Design headers | Yes | `rg` over docs source anchors for the manifest producers; all relevant named pages above, plus `docs/guide/configuration.md` and `docs/config-reference.md`, were assessed. Generic configuration examples and generated reference constraints retain accurate YANG/parser contracts. The durable subscriber model now retains the intact dated owner requirement and derived cap design. Final native anchor gate `job-cpe16-umbrella-final-citation-anchors-1b88903b.log` exits0 without advisory. |
| 17 | Config/CLI examples | Yes | Printed commands run through real editor regressions; promoted config carriers validate range/defaults and effective-policy readback. No bracket-list editor command remains in the owning guide examples. |

## Implementation Summary and Child Closure History

The production/runtime change is `18ea5e830a`, lab/checker/image work is
`3cc99259b0`, generic image provenance is `0bef13eda7`, proof refresh is
`1a32d7f93d`, RFC floor comments are `426b892b43` and final strict proxy decoding
is `7e2fb4953f`. The final Go commit has passing committed compilation for all
five flavors; later closures are prose only. Scoped weakened-tooling/composition
repair is `f16fe6136b`; main help docs are `d5efcf8bd3`.

| Child / completed outcome | Commit A | Commit B |
|---------------------------|----------|----------|
| 16a omitted-MRU cap | `074c1265f7` | `b21c1ebcc2` |
| 16b proxy-MRU cap | `2664e9268b` | `7ffaf575d6` |
| 16c executable auth repairs | `c5f4a65aaf` | `389f10fb16` |
| 16d protocol floors/strict proxy MRU | `840b0fdd69` | `1875e80c5c` |
| 16e actual live Nak | `bc0e4df5b0` | `c3a1f04941` |
| 16f unequal negotiated output | `3c4ac0ca98` | `433c56dad8` |
| 16g refusal ordering/provenance | `8520606ed9` | `b64e0b88e2` |
| 16h paired pppd images | `8b15b22a11` | `095c3f43d2` |

### Deviations and Mistake Log

| Kind | What happened | Producing truth/owning layer | Action and reason |
|------|---------------|-----------------------------|-------------------|
| approach | Initial oversized replies could report successful suppression and advance the FSM. | `sendConfigureAck`/`sendConfigureNakOrReject` and NCP counterparts must report failure before `applyTransition` commits its next state. | Source fixes plus handler reds/greens; fitting positive controls retained. |
| approach | Proxy floor/presence/width/duplicate edge cases bypassed fresh negotiation. | `EvaluateProxyLCP`/`decodeProxyMRU` own decoding; `acceptProxyPolicy` owns floors. | Final20-case driver red/green and full normal proxy service; no blanket proxy rejection. |
| approach | The initial live MRU peer did not emit a Nak-provoking option; copied config and early Ack could satisfy old proof. | Pinned pppd `lcp_addci` omits default1500; wire/JSON/checker consumers must discriminate their own claim. | Explicit1493, unequal1400 and same-image old/current checker counterfactuals; retain literal1500 and direction controls. |
| approach | Closure found stale owned RFC records after shared producer edits. | Discrimination/audit fingerprints belong to implementation evidence. | Handback and source-bound record refresh before child closure; no closure-authored proof. |
| approach | Direct Nak fault teardown stalled in Docker and fresh normal setup initially could not run. | Shared Docker resource removal/preflight, not the LCP assertion. | Record `b4215935ac` blocked handback; quarantine fault artifacts; resume only after natural recovery and complete normal build/service/cleanup pass. No daemon restart or foreign process kill. |
| approach | Historical closure prose/summary still described skeleton work or removed minIPMTU. | Record layer: current `ValidateNetworkMRU`/`ipv6MRUSupported` implement guards but do not constitute RFC-tagged proof. | Correct records once; preserve RFC5072-2-2 testing gap and existing post-release IPv6 owner. Deleting the gap or claiming full IPv6 would be a workaround. No new product round. |

No additional rule, learned page or journal row is required: the work applies
existing completion, evidence, RFC and interop rules. No session-owned journal
row remains uncommitted.

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| F-1 | Done | `transmitMRU`/`afterLCPOpen`, both omission checkers | Useful traffic and measured Ze cap; full RX preserved. |
| F-2 | Done | `acceptProxyPolicy`, `TestProxyTransmitMRUCap`, native proxy fixture | Accepted1500/9000/omission and original accounting remain. |
| F-3 | Done | `refuseAuthMethod`, PPPoE `ExtractParameters`, actual-editor tests | Transport-specific executable repairs. |
| F-4 | Done | `ValidateNetworkMRU`, `ipv6MRUSupported`, strict proxy decoder | Config/runtime defaults, both floors, positive service and invalid fallback. |
| F-5 | Done | `lnsMRUEvidence`, `checkLNSMRU`,1493 options | Exact live Nak sequence and direct producer red. |
| F-6 | Done | `checkLNSNegotiatedMRU`, reactor/snapshot/JSON | Unequal config/negotiated/installed proof and copy red. |
| F-7 | Done | `lnsCHAPOnlyEvidence`, `lns_auth_order_test.go` | Same-image real early peer Ack rejected; legitimate direction retained. |
| F-8 | Done | Both LAC Dockerfiles and release/runtime tests | Hash/runtime/plugin and affected subscriber services. |

### Acceptance Criteria

| AC ID | Status | Demonstrated by | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Both omission native carriers and cap-disabled reds | Actual MTU1492 versus faulty1500. |
| AC-2 | Done | Final native proxy `5bad2c84` | Three admitted cases,1492-byte ICMP and cleanup. |
| AC-3 | Done | Restored editor race `1cc32b69` | Both transport repairs and both PPPoE alternatives. |
| AC-4 | Done | Floor/config stress, final20-case proxy and PPP race | IPv6 positive is driver/NCP only. |
| AC-5 | Done | `job-cpe16e-nak-restored-after-recovery-c2b9a595.log` | Request1/1493→Nak1/1492→retry2/1492→Ack2 before CHAP/IPCP. |
| AC-6 | Done | Publication fault `1d421026`, normal `718e9ad2` | Both views1400, config1492 and Ze MTU1400. |
| AC-7 | Done | Old `eeb1ae95`/current `267f5fe1`, normal `8cbdeacb` | Identical three faulty images; old false positive and fixed red. |
| AC-8 | Done | Probes `3461699d`, subscriber-RADIUS `d83f8741` | Both versions/plugins; subscriber4/4 and L2TP policy controls. |

### Tests from TDD Plan

| Test group | Status | Location | Notes |
|------------|--------|----------|-------|
| Normal/control/auth/config transmit cap | Done | `ppp/transmit_*_test.go` | Fitting/oversized, mandatory-field and unsent-Ack controls. |
| Proxy admission/cap/restart | Done | `proxy_transmit_cap_test.go`, `rfc1661_proxy_policy_driver_test.go` | Full credential/IPCP/cleanup and full receive proof retained. |
| Editor repairs/counter | Done | `auth_repair_commands_test.go`, `cli/pppoe_repair_commands_test.go`, `auth_policy_metrics_test.go` | Real parser/dispatcher and policy assertions. |
| Floor/direct admission/proxy encodings | Done | Both `mru_protocol_floor_test.go` files, `proxy_mru_floor_driver_test.go`, `proxy_mru_encoding_driver_test.go` | All documented bounds, zero/presence/width/duplicates. |
| Nak/output/refusal consumers | Done | `lns_mru_nak_test.go`, `lns_negotiated_mru_test.go`, `lns_auth_order_test.go` | Negative packet/JSON/order fixtures plus complete useful controls. |
| Omission/proxy/accounting consumers | Done | Both omission test files, `proxy_mru_test.go`, `radius_fixture_test.go` | Required measured evidence cannot be replaced by exit0. |
| Native config and kernel carriers | Done | Both `.ci` files; scenario directories; registered proxy/accounting fixtures | Central stress/promotion and actual native interop. |
| Peer/provenance | Done | `lac_images*_test.go`, `image_provenance_test.go` | Both physical images and consumed immutable build references. |

### Files from Plan

| File group | Status | Notes |
|------------|--------|-------|
| Shared PPP runtime | Done | All reached senders, admission, decoder and lifecycle reviewed at current source. |
| L2TP config/startup/reload/YANG | Done | Effective defaults and schema/runtime boundaries agree. |
| PPPoE policy/repair and existing server | Done |1492 producer and repair parser match source. |
| L2TP/PPPoE checker packages | Done | Registered through actual discovery/selector paths. |
| Native fixtures/scenario inputs | Done | Real wire peer and kernel/state/traffic assertion producers checked. |
| Paired LAC Dockerfiles/tests | Done | Matching release pins, runtime test and unchanged suite wiring. |
| Documentation and RFC evidence | Done | Committed source anchors/proofs; closure's summary testing-gap correction retains unmet status. |
| Plan records/citers | Done | All eight children closed; sole tracked umbrella citer repointed to surviving guides/labs; historical name is a bare stem. |

### Audit Summary

32 grouped items:32 Done,0 Partial,0 Skipped,0 Changed. Grouped rows retain
all eight original outcomes; source/proof refinements are recorded in Deviations.

## Goal Validation

| Goal from Task | Evidence type | Concrete evidence |
|----------------|---------------|-------------------|
| F-1 useful omission service under the cap | Rebuilt-artifact interop | Both normal omission scenarios pass measured1492 and traffic; `22d28569`/`0d495f97` uncapped builds fail actual1500; normal artifacts restore. |
| F-2 admit valid proxy without oversized sends | Real kernel fixture | Final `5bad2c84`:1500/9000/omission each CHAP/IPCP, installed1492, full1492-byte ICMP, interface removal. Uncapped producer red retained. |
| F-3 operator can execute repairs | Actual editor workflows | `1cc32b69`: restored producer passes transport-specific member commands and both PPPoE alternatives; old producers fail shape/count/editor proof. |
| F-4 supported floors without peer-limit inflation | Config/driver/network proof | Config carriers80/80 stress each; final20 proxy driver cases pass `3392f282`, complete PPP race `fdd309de`; below-floor inputs renegotiate and corrected68 service remains. Full subscriber IPv6 is not asserted. |
| F-5 real Nak and useful convergence | Direct producer counterfactual | Actual guard-disabled build `1f721660` Acks1493 and fails missing-Nak despite CHAP/IPCP. Fresh full normal `job-cpe16e-nak-restored-after-recovery-c2b9a595.log` passes all service and cleanup assertions. |
| F-6 output is actual negotiated state | Publication counterfactual | Config-copy mutation `1d421026` fails aggregate1492 versus wire1400; restored `718e9ad2` passes both1400 views/config1492/kernel1400 and traffic. Historical immutable IDs/full argv were not retained and are not inferred. |
| F-7 refusal proof cannot pass after early peer Ack | Same-image checker counterfactual | Old `eeb1ae95` passes1/1 and current `267f5fe1` fails `LCP could open` on the same three immutable references. Normal full rebuild `8cbdeacb` passes refusal and cleanup; permitted CHAP traffic remains green. |
| F-8 peer alignment preserves real subscriber services | Rebuilt runtime/interop | Paired probes `3461699d` show2.5.5/plugin path; `d83f8741` subscriber-RADIUS4/4; native L2TP policy controls pass. Historical IDs are not reconstructed. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None of F-1 through F-8 | All original outcomes and preservation obligations are implemented, proven and closed in their eight children. | - |
| Full subscriber IPv6 and its remaining IPv6CP floor RFC-tagged proof | Existing owner ruling2026-08-30 ships first-release subscribers IPv4-only. The1280 positive here is driver/NCP proof with a fake allocator; the real pool still declines IPv6. RFC5072-2-2 remains a testing gap, not proven support. | `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Round1 scope | Whole logical current F-1 through F-8 source, tests, reached unchanged producers/consumers, evidence and closure prose. Union of eight child review populations; exactly eight deleted child specs omitted, no code omitted.147 current paths,69 Go. |
| Reviewer | `close_umbrella-Codex-GPT6`, fresh independent closure context; did not author runtime/tests. No subagents or code execution. |
| Reviewer lenses used | Logic+wiring/spec completeness; security+boundary/lifecycle; style+evidence discrimination. Each traces the deciding producers and preserved useful cases. |
| Source verdict | CLEAN:0 BLOCKER,0 ISSUE. Record-only NOTEs corrected in one prose edit; no product fix or extra round. |
| Artifact | Final native `job-cpe16-umbrella-native-final-review-57a58409.log` records CLEAN1 over147 paths at `tmp/review/home-cpe-16-mru-enforcement-and-review-fixes-3a525723-d369-4547-9124-fe35463dc0f5.md`. |
| `./le spec review check` | PASS in final native `57a58409`:72 code files (69 Go, YANG and two `.ci`) clean with matching hashes. An earlier launcher build stopped on a concurrent foreign inventory edit; its owner restored the missing helper before this successful check. No foreign edit was overwritten. |
| Rounds |1 fresh independent umbrella closure round. The four historical rows above retain two implementation reviews and two coordinator validations; they are not four earlier rounds of this pass. |
| Final anchors/RFC summary checks | Final citation-anchor `1b88903b` exits0 without advisory after the durable owner-page edit. Final RFC check `722fbda6` has79 foreign findings, zero owned/summary findings; update `8119b55c` exits0. |

### Findings fixed

| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| NOTE | Historical skeleton/unvalidated labels, missing closure tables and stale scope/evidence wording. | This coordination record and surviving `spec-home-cpe-0-umbrella.md` citer. | Current-source closure prose; no source fix or new review round. |
| NOTE | RFC5072 summary described removed minIPMTU rather than implemented guards. | `rfc/short/rfc5072.md`, RFC5072-2-2. | Correct annotation to a testing gap; retain the obligation, unmet status and existing post-release owner. |
| NOTE | The dated owner requirement lived only in the removable umbrella. | `docs/architecture/l2tp/subscriber-session-model.md`, Subscriber transmit limit. | Transfer intact excerpts and derived source-anchored design to that existing durable page; dependent plan text points there. |
| NOTE | Native automatic model header selects an old Claude transcript. | Native artifact metadata. | Explicit actual reviewer identity and private Findings provenance note; no CLI model override exists. |

The style pass covered the changed Go and reached writers: socket-to-panic,
bounds, names, paired ownership, copies, raw-input validation and necessary
return dimensions. Fixed-size pooled buffers and session-loop ordering remain;
no new unbounded product work or hidden auth-policy bypass was found.

## Pre-Commit Verification

### Files Exist (source read)

| File | Exists | Evidence |
|------|--------|----------|
| New runtime/tests/scenario inputs and both promoted `.ci` files | Yes | Read current manifest population and canonical carriers tracked in `18ea5e830a`, `3cc99259b0`, `0bef13eda7`, `6c065fdc93`, `7e2fb4953f`.147 paths exist; only the eight removed child specs were excluded. |
| Surviving operator/design pages and IPv6 owner | Yes | Guides/labs, source anchors and `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` read; source `poolPlugin.handle` confirms the actual IPv6 boundary. |

### AC Verified (source/test)

| AC ID | Claim | Fresh evidence |
|-------|-------|----------------|
| AC-1 | Both omission transports obey policy with useful traffic | Current `transmitMRU`/`afterLCPOpen`, both native checker producers and their cap-disabled/restored outcomes read. |
| AC-2 | Valid proxy admission survives under the cap | Final `job-cpe16d-proxy-final-native-5bad2c84.log` read: all three measured1492/full-sized ICMP/cleanup cases pass. |
| AC-3 | Real editor accepts every printed repair | Both actual editor test bodies and restored `1cc32b69` proof read; transport/counter preservation checked. |
| AC-4 | Floors and proxy zero/width/duplicate boundaries | `decodeProxyMRU`, admission/negotiated guards and final20-case matrices read; final PPP race `fdd309de` passes17.729s. |
| AC-5 | Correlated real Nak before useful service | Actual final full-label `job-cpe16e-nak-restored-after-recovery-c2b9a595.log` read in full; direct `1f721660` fault is a missing-Nak failure, with cleanup stall separately disclosed. |
| AC-6 | Both views report raw negotiation | `handleLCPUp/Down`→`sessionSnapshot`→`sessionJSON` and unequal checker read; restored `718e9ad2` trace1400 and publication red assessed. |
| AC-7 | Early peer Ack is rejected, legitimate direction retained | Current `lnsCHAPOnlyEvidence`, full direction controls, same-image structured comparisons and normal restoration evidence assessed. |
| AC-8 | Rebuilt aligned peers preserve service | Both exact Dockerfiles and native probe/suite consumers read; retained2.5.5/plugin and4/4 outputs assessed. |

### Wiring Verified (end-to-end)

| Entry point | Carrier | Verified |
|-------------|---------|----------|
| Product config/editor | `lns-mru-protocol-floors.ci`, `lns-mru-auth-allow-config.ci`; two real-editor Go carriers | Reads actual config/commands; checks individual refusals, parsed order, running effective config and whole workflow. |
| Actual subscriber | Native L2TP/PPPoE scenario directories, compiled registered proxy/accounting fixtures | Discovery/checker/fixture registrations reach real normal/proxy auth/NCP/kernel/traffic/cleanup. No new `.ci` is substituted for this live carrier. |
| Operator output | Existing daemon REST execute route in live L2TP checker | Queries actual config, aggregate and selected detail; independently measures peerZe interface. |

### Assumptions Resolved

| ID | Final status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Pinned pppd source and actual final1493→Nak1492→retry1492/Ack trace. No unvalidated assumption remains. |
| R-1..R-4 | mitigated, observations retained | Positive proxy/auth preservation, independent protected-test judgment, exact boundary/fallback controls and separate wire/config/output/MTU evidence. Full subscriber IPv6 remains explicitly outside first-release capability by the existing owner ruling. |

### Documentation Verified

| Documentation claim/category | Source evidence | Verified |
|------------------------------|-----------------|----------|
| Cap/receive/floors/repairs/raw output | Deciding runtime, config, editor and snapshot functions named in the17-row checklist; source-anchor search across docs | Yes; committed guides/wiki and lab claims match. |
| Lab assertion and peer identity | `lnsMRUEvidence`, omission/unequal/refusal consumers, fixture, Dockerfiles, `Suite.prepareImages` | Yes; normal, semantic-fault, setup-only and historical-ID limits remain distinct. |
| Declared design pages and unaffected categories | Current `// Design:`/source-anchor census, named architecture/research/config pages and unchanged SDK/API/metrics/metadata contracts | Yes; source-aware No dispositions recorded. Foreign shared documentation hunks are preserved outside closure A. |
| RFC5072 floor note | `ValidateNetworkMRU`, `ipv6MRUSupported`, NCP/service guards and retained untagged driver tests | Yes; annotation retains a testing gap. Final native RFC check reports no summary/owned finding; final update passes. |
| Native documentation validity | `job-cpe16d-final-doc-index-check-dc77857d.log` |365 design docs,4206 paths,772 packages; all references valid. Full `baf5a445` retains142 foreign published-site drifts. Final closure anchor `1b88903b` exits0 without advisory after the durable owner-page edit. |

### Central validation and limits

| Check | Directly observed scope/result |
|-------|--------------------------------|
| Final runtime | All L2TP packages race `e9d0980c` PASS; final whole PPP race `fdd309de` PASS after final proxy decoder;20-case matrix `3392f282` PASS. |
| Native service | Final proxy `5bad2c84` PASS; live Nak final full-label `c2b9a595` PASS after complete normal rebuild/recovery; earlier omission/output/refusal/accounting/PPPoE/RADIUS controls retained. |
| Committed compilation | `job-cpe16d-runtime-committed-build-9f891c3d.log`: HEAD `7e2fb4953f30`, all distro/appliance/setup/host/installer flavors PASS. No Go change followed. |
| RFC evidence | Final `job-cpe16-umbrella-final-rfc-check-722fbda6.log`:79 foreign findings, no owned stale/missing54 proof entries, three audits or RFC5072 summary finding. Shared RFC1661-5.8-2 stale negative belongs to `pppoeclient`, not this PPP proof. Final update `8119b55c` PASS,229 ignored shards/derived indices. Earlier76 was a historical corpus state. Closure authors no record. |
| Original tooling blockers | `f16fe6136b` fixes the original3 weakened reds and test-plugin composition; complete owning packages and config corpus pass. Broader9 repo/10 weakening findings remain foreign. |
| Publication | Main help/wiki repairs committed. Full doc check142 gh-pages drifts; site build `bba3847b` blocked by foreign source-model inventory work. Owned7 inventory leaf tests pass `47d3d31f`; complete package is foreign-red `d8564526`, not green. Dependent uncommitted hunks remain with the active owner and cannot land alone. |
| Attribution | F-6 and historical F-8 immutable IDs/full argv were not retained. Fresh F-7 exact-image pair and final E identities are recorded where actually emitted; no historical identity is inferred from cache tags. |
| Execution/commit boundary | This independent closure context performed static reads and prose edits only. Root owns every build/test/native artifact/check/commit. No whole-tree `le verify` or push was run. All native results above are observed; Status is done for this final frozen closure snapshot. Actual removal/claim release occurs only through the coordinator's native script. |
