# Spec: Home CPE 16 MRU enforcement and review fixes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol, config, cli, tooling, docs |
| Priority | P1 |
| Release reason | An accepted 1492 MRU policy still permits oversized transmissions on implemented subscriber transports. |
| RFC gaps | - |
| Depends | - |
| Phase | Eight issues implemented and centrally verified; closure review remains |
| Handoff | verify |
| Updated | 2026-11-05 |

## Issue Split and Current Ownership (2026-11-04)

Execution instruction (2026-11-04): "use one agent per tasks to fix the issue,
they should not compile or run code and bring their work here for review where
you will centralise the compiling and running of the test they have authored".
Each issue has one author. The main thread owns formatting, builds, test runs,
proof records and commits. Authoring agents return uncommitted changes and
identify all unrun checks explicitly.

The owner requested: "create a spec per issue". This record is now a
coordination umbrella and the source of the original review and owner direction.
Each finding has one child spec in the routing table below. That table
supersedes the combined implementation scope and next action in the original
sections retained here. No issue is fixed by this split.

### Work Not Done

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

The children were created at `skeleton`. Complete each child's research and design
before implementation. The umbrella closes only after all eight children
meet their outcomes and their retained evidence supports the original goals.
RFC requirement mapping belongs in the relevant child's research, not this
umbrella's metadata.

## Task

Fix all eight findings from the owner's review of
`spec-home-cpe-16-lns-mru-and-auth-allow-list`. The earlier spec was removed
at closure; this spec owns its remaining defects. Auth allow-list enforcement
is a preservation requirement. Recording these findings does not fix them or
establish that the earlier MRU goal is complete.

This is a research skeleton for a fresh implementation session. Complete the
design-time template and source census before advancing to `ready`.

### Owner direction (2026-11-04)

> The auth policy holds, but the MRU control doesn't do its job, so I wouldn't treat this spec as closed. With a 1492 limit, Ze can still transmit above it. All of this still needs a fix; I changed nothing and no rows are filed.

> Fix for 1 and 2: limit the transmit MTU to the session's mru

The second excerpt is the intact opening of the owner's final paragraph;
the rest arrived with truncated words. The requested direction is recorded
here without attempting to reconstruct that text.

## Review Findings to Resolve

The table preserves the reported findings and their proof obligations.
Only the source checks listed under Current Behavior were repeated when
creating this skeleton; live probes and race results remain review evidence.

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

**Source checked when creating this skeleton:**

| Producer | Observed behavior |
|----------|-------------------|
| `internal/component/l2tp/ppp/session_run.go`, `pppSession.afterLCPOpen` | Missing negotiated MRU falls back to 1500; installed MTU is the maximum of peer MRU and 68, with no session-cap bound. Receive buffer is separately set to 1500. |
| `internal/component/l2tp/ppp/proxy.go`, `EvaluateProxyLCP` | Records local and peer MRUs separately from Last-Sent and Last-Received options. |
| `internal/component/l2tp/ppp/rfc1661_proxy_policy_driver_test.go`, `TestProxyLCPPolicy` | Contains the row "peer MRU is not local cap" with local1492/peer1500 and accepted proxy policy. |
| `internal/component/l2tp/yang/ze-l2tp-conf.yang`, `mru` and `ncp/enable-ipv6cp` | MRU range is 64..1500; IPv6CP defaults true. |
| `test/interop-l2tp/Dockerfile.lac` and `test/interop-radius-subscriber/Dockerfile.lac` | Pin pppd 2.5.5 and 2.5.4 respectively. |

**Preserve:** allowed-method enforcement across Nak, Reject and proxy paths;
required authentication refusal before Opened; valid proxy admission;
session policy isolation across reload; receive capacity required by PPP;
working IPv4 and IPv6 subscriber traffic for supported sizes.

## Existing Mechanisms

| Mechanism | Location | Current purpose | Direction |
|-----------|----------|-----------------|-----------|
| Shared PPP interface setup | `pppSession.afterLCPOpen` | Installs MTU for both transports and accepted proxy sessions. | Extend this existing path with the session transmit cap. |
| Proxy LCP parsing | `EvaluateProxyLCP` | Keeps the two directions' negotiated MRUs separate. | Preserve parsing and admission; enforce transmit policy at installation. |
| Transport config and session policy | L2TP MRU leaf and shared PPP driver | Supplies a per-session limit; PPPoE uses 1492. | Trace all producers, restarts and consumers before implementation. |
| Existing interop policy scenarios | L2TP and PPPoE lab suites | Exercises MRU/auth negotiation and output. | Strengthen live inputs and assertions instead of adding parallel harnesses. |

## Data Flow

Trace configuration and transport defaults into the admitted session, then
normal LCP or proxy LCP into negotiated state, interface MTU installation,
session events and CLI snapshots. Account for restart and reload paths.
Keep peer receive MRU distinct from the local receive request and effective
transmit MTU. The cap must survive all paths to interface installation.

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Treating a transmit cap as proxy admission policy rejects working subscribers. | Oversized peer MRU fails admission instead of producing a capped interface. | Keep positive admitted-proxy tests and assert installed MTU. |
| R-2 | Changing the named proxy test row weakens its authentication coverage. | New test checks only size or deletes a policy assertion. | Obtain independent `ze-test-judge` approval before changing protected expectations; retain auth and admission coverage. |
| R-3 | Clamping upward exceeds a peer's small receive MRU. | Interface floor exceeds the peer limit. | Resolve floors at validation/negotiation; test lower boundaries explicitly. |
| R-4 | Session output is confused with installed MTU after adding a local cap. | Output reports config as negotiation. | Specify field meaning and test unequal values across wire, snapshot and interface. |

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | A pppd value in 1493..1499 provides the explicit oversized request needed for live Nak proof. | Owner's review. | Scenario does not exercise the intended Nak. | Read the pinned peer producer and capture its actual Configure-Request. | unvalidated |

## Acceptance and Test Plan to Complete

Each F-N row is a required outcome, not a completed AC. Expand it into named
unit, boundary, functional and interop cases in the design template. Include
omitted, smaller, equal and larger peer MRUs, accepted proxy values 1500/9000,
both transports, restart and reload. Check installed interface MTU and useful
data delivery. Complete the boundary matrix around 64, 67, 68, 1279, 1280,
1492 and 1500 with IPv6CP enabled and disabled.

Independent `ze-test-judge` review is required for the proxy expectation
change and every other protected test change. Admission can remain accepted;
the incorrect implication that accepted peer MRU may raise installed MTU
above the session cap must change.

Run scoped L2TP, PPP and PPPoE race tests, affected editor tests and the three
original policy interop scenarios after strengthening them. Run subscriber
RADIUS interop after rebuilding its image. Record actual output and red/green
discrimination; do not promote source reachability to semantic proof.

## Documentation and Closure Obligations

Update the L2TP and PPPoE guides, their affected architecture/source anchors,
and corresponding wiki pages with the implementation. Reverse descriptions
that permit an effective transmit MTU above the local cap. Verify repair
commands in the editor rather than treating file syntax as command syntax.
Complete all integration and documentation checklist rows before design is ready.

The owner's review reports no new auth-policy defect, passing scoped race
tests, matching RFC quotes and reproducible pppd 2.5.5 hashes. These are prior
review results, not reruns by this skeleton's author. The three original live
interop scenarios were not rerun in that review; their prior results remain
historical. Commit 3902dc1ada demonstrates reachability, not behavior.

The separately reported three `internal/le/test/weakened` failures must not
be called green. The review attributes their messages to 8c64ad2d4a and
repo/objects and notes four unrelated edited files. Reproduce and route that
red separately; it is not evidence that any F-N is fixed.

## Owned journal rows

No rows were filed by the review. This spec owns F-1 through F-8 directly.

## Review Gate

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
| 1 | Eight-agent implementation: correctness, security and coverage | Independent review agent | Found false FSM advancement after unsent Acks and proxy peer MRU below68 bypassing the floor. Both fixed after centrally observed regression reds. | `job-cpe16-handler-and-floor-red-7e7e4fa1.log`, then `job-cpe16-handler-and-floor-fixed-d12ee1a2.log` |
| 2 | Both source fixes and the native accounting regression carrier | Independent review agent | Product blockers resolved. Atomic accounting status-publication note fixed; actual original accounting fixture then passed. | `job-cpe16-final-l2tp-live-e6b8eb67.log` |
| 3 | Scoped race, configuration stress, actual kernel/data-plane tests | Main coordinator | All L2TP packages and both checker packages pass race; both configuration carriers pass 80/80 stress runs and promoted execution; all normal live scenarios pass. | Central verification below. |
| 4 | Rebuilt-artifact semantic discrimination and restoration | Main coordinator | Both transport omission faults and proxy oversize fault fail on actual MTU 1500; copied-output fault fails on 1492 versus peer 1400; old refusal checker passes actual early peer Ack, fixed checker rejects it. All final normal artifacts and controls pass after restoration. | Named fault logs below and `job-cpe16-final-artifacts-restored-718e9ad2.log`. |

## Next Action

Implementation, central review, scoped testing and rebuilt-artifact semantic
discrimination are complete. Review these eight issue specs for closure;
their recorded results supersede the skeleton research and unrun handoffs.
No whole-tree green or formal spec closure is claimed here.

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

PPP statement coverage is 82.6%. Shared transmit-limit, final-writer and
LCP/NCP Configure-Ack helpers are fully covered by the measured count run.
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

The configuration corpus check reported four unrelated test-plugin
registration failures; the repository-wide documentation check reported
143 drift findings, including published command catalogs, and is not counted
green. An earlier broad
fixture run failed four unrelated tests requiring le on PATH; the changed
parser, native carrier inventory and original accounting regression passed.
The separately reported weakened-tooling package is not called green. These
results do not represent a whole-tree pass, and this spec is not closed.
