# Spec: Home CPE 16 MRU enforcement and review fixes

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol, config, cli, tooling, docs |
| Priority | P1 |
| Release reason | An accepted 1492 MRU policy still permits oversized transmissions on implemented subscriber transports. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-04 |

## Issue Split and Current Ownership (2026-11-04)

The owner requested: "create a spec per issue". This record is now a
coordination umbrella and the source of the original review and owner direction.
Each finding has one child spec in the routing table below. That table
supersedes the combined implementation scope and next action in the original
sections retained here. No issue is fixed by this split.

### Work Not Done

| Finding | Owning spec | Coordination |
|---------|-------------|--------------|
| F-1 | `plan/release/fixes/spec-home-cpe-16a-omitted-mru-transmit-cap.md` | Owns the shared transmit-cap implementation and omitted-MRU proof. |
| F-2 | `plan/release/fixes/spec-home-cpe-16b-proxy-mru-transmit-cap.md` | Reuses F-1's shared cap; owns accepted-proxy proof and independent test judgment. |
| F-3 | `plan/release/fixes/spec-home-cpe-16c-auth-policy-repair-commands.md` | Owns editor-executable, transport-specific repair messages. |
| F-4 | `plan/release/fixes/spec-home-cpe-16d-mru-protocol-floors.md` | Owns IPv4/IPv6 MRU floors and config/runtime consistency. |
| F-5 | `plan/release/validation/spec-home-cpe-16e-live-mru-nak.md` | Owns live oversized-request Nak and convergence proof. |
| F-6 | `plan/release/validation/spec-home-cpe-16f-negotiated-mru-output-proof.md` | Owns unequal configured/negotiated output proof. |
| F-7 | `plan/release/validation/spec-home-cpe-16g-auth-refusal-order-proof.md` | Owns the live refusal checker's ConfAck exclusion. |
| F-8 | `plan/release/validation/spec-home-cpe-16h-subscriber-radius-pppd.md` | Owns paired peer-image release alignment and rebuild evidence. |

All children start at `skeleton`. Complete each child's research and design
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

## Next Action

In a fresh session, claim this spec and complete research/design through
`ze-spec`. The owner has supplied the fix direction and needs no new decision
on whether the eight defects should be fixed. This skeleton does not authorize
closure or claim a completed design.
