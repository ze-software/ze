# Spec: DDoS FlowSpec/RTBH Responder — Upstream Mitigation + Leak-Probe Clear

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-cp-survival-5-detect-1-detector, spec-cp-survival-4-flowspec-origination, spec-cp-survival-5-detect-0-umbrella |
| Phase | - |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cp-survival-5-detect-1-detector.md` (event contract), `plan/spec-cp-survival-4-flowspec-origination.md` (the announce/withdraw lever)
3. `plan/spec-cp-survival-5-detect-0-umbrella.md` (the flowspec-blindness/leak-probe rationale, safety controls)
4. `internal/component/bgp/reactor/reactor_api_batch.go` (`AnnounceNLRIBatch`/`WithdrawNLRIBatch`), `internal/component/bgp/plugins/nlri/flowspec/encode.go`

## Task

Build the **flowspec/RTBH responder**: it subscribes to the detector's `AttackDetected`/`AttackCleared`
events and mitigates **upstream** by originating a surgical BGP FlowSpec rule (or an RTBH /32) over an
existing BGP session, then determining when the attack has ended and withdrawing it.

It originates nothing itself — it drives `spec-cp-survival-4`'s `announce flowspec|blackhole … for
<duration>` verb (or `AnnounceNLRIBatch`/`WithdrawNLRIBatch` directly). It owns the **decision** of when
to announce, when to probe, and when to withdraw.

### The central problem this spec solves: clear-detection under upstream drop
Once the FlowSpec takes effect upstream, the attack traffic is dropped before it reaches any Ze sensor,
so the detector goes blind (umbrella A-2/R-2: Ze has no inbound flow collector). The detector's
`AttackCleared` is therefore NOT trustworthy while this responder is mitigating. This responder owns its
own attack-end detection via a **bounded leak-probe**:

- Hold the announcement for at least `hold-down` (attacks pulse; never withdraw on a brief lull).
- After hold-down, every `probe-interval`, **narrow the rule to a small non-zero `probe-rate`** (raise the
  FlowSpec traffic-rate action from discard/0 to e.g. a few Mbps) and observe the local iface rate for
  `probe-window`.
  - If the leaked rate **saturates** `probe-rate` → the flood is still arriving → re-tighten to
    discard, extend `hold-down` (exponential backoff, capped) and keep mitigating.
  - If the leaked rate stays **well below** `probe-rate` → the attack is over → withdraw.

This bounds the leaked traffic to `probe-rate` instead of the full flood (vs a blunt withdraw-and-watch),
and it is the only passive-ish signal available until an inbound flow collector exists (umbrella follow-on).

## Required Reading

### Architecture Docs
- [ ] `spec-cp-survival-4-flowspec-origination.md` - the announce/withdraw lever
  → Constraint: CALL its `announce flowspec|blackhole … for <duration>` verb (or `AnnounceNLRIBatch`/`WithdrawNLRIBatch`); never re-encode FlowSpec or open a second announce path.
- [ ] `ai/rules/plugin-design.md` (EventBus Typed Payloads + DirectBridge) - input + actuation
  → Decision: subscribe to `ddosevent` (import the leaf only); reach the reactor via cp-survival-4's command/DirectBridge path, not reactor internals.
- [ ] `ai/rules/plugin-self-containment.md` + `ai/rules/cli-grammar.md`
  → Constraint: this plugin owns its YANG, its `clear` verb (action-before-identifier), and its announcement bookkeeping.
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md`
  → Decision: response-level, action, selector, allowlist, and all probe/safety timers are YANG leaves (kebab-case); durations/rates use `ze-types.yang` types with `range`.
- [ ] `ai/patterns/bgp-family.md` - BGP Family Gate
  → Decision: N/A — no new SAFI/capability/attribute; reuses FlowSpec SAFI 133/134 + BLACKHOLE via cp-survival-4 (see BGP Family Gate below).

### RFC Summaries
- [ ] `rfc/short/rfc8955.md` (+ `rfc8956` v6) - FlowSpec match + traffic-action ext-comm
  → Constraint: build the match from the existing 13 components; `traffic-rate 0` = discard, non-zero = the probe rate. Upstream validates the flow against the best unicast route for the dest and MAY reject it.
- [ ] `rfc/short/rfc7999.md` - BLACKHOLE community 0xFFFF029A (RTBH fallback)
  → Constraint: RTBH = unicast /32 + BLACKHOLE; coarser than FlowSpec (drops ALL traffic to the dst).

**Key insights:**
- Active mitigation = FlowSpec match with **traffic-rate 0** (the RFC 8955 discard action); the
  leak-probe simply **raises that rate** to a non-zero `probe-rate` to test whether the flood is still
  arriving. Block and probe are the same rule with two rate values, not two rules.
- The match is built from FlowSpec **header** components only (dst/src prefix, proto, dst/src **port**,
  tcp-flags, packet-length, fragment); `FlowSourcePort` (types.go:78) / `FlowSpecRoute.SourcePorts` are
  encoder-supported. **Source-port** matching is the primary discriminator for reflection/amplification
  floods (src-port 53 DNS, 123 NTP, 19 chargen, 11211 memcached, …). Payload/byte-pattern blocking is
  NOT expressible in FlowSpec — that stays in the detector's Stage-2 analysis and the local-nft responder.
- Leak-probe is the attack-end signal; the detector's `AttackCleared` is ignored while mitigating (the
  detector is blind under upstream drop). The responder consults the event's "observable" flag to confirm.
- Default action is **rate-limit**, not discard, where the operator allows it: it preserves legitimate
  traffic and makes the probe cheaper (the limiter already passes a measurable trickle).
- Announce/withdraw churn is operationally dangerous (route-flap damping, de-peering): `hold-down` +
  `announce-rate-limit` + incident coalescing + preferring narrow/widen over withdraw+reannounce.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:28` - `AnnounceNLRIBatch(sel, batch)` / `WithdrawNLRIBatch`; matches peers, queues non-established, builds + sends UPDATE.
  → Constraint: the only actuation API; reuse via cp-survival-4 or directly. Announcements are ephemeral (not persisted) — re-trigger after restart (cp-survival-4 A-2).
- [ ] `internal/component/bgp/plugins/nlri/flowspec/encode.go` - FlowSpec NLRI encoder; `traffic-rate` bps/pps, discard, redirect, marking actions; families `ipv4/flow`, `ipv6/flow`.
  → Constraint: build the match + action via this encoder (through cp-survival-4); discard for the active rule, non-zero traffic-rate for the probe; pick AFI by target.
- [ ] `internal/component/bgp/attribute/community.go:99` - `CommunityBlackhole = 0xFFFF029A`.
  → Constraint: RTBH fallback = unicast /32 + this community.
- [ ] `internal/core/ddosevent/event.go` - the event contract (child 1): target, vector tuple, family, sources, observable flag.
  → Constraint: build the surgical match from these; select v4/v6 FlowSpec by target address family.
- [ ] `internal/component/iface/rate.go` - `iface.GetRate(name)` RxPps/RxBps (now dataplane-agnostic via 1a).
  → Constraint: the leak-probe reads this to measure the leaked rate during the probe window.

**Behavior to preserve:**
- cp-survival-4 origination, static FlowSpec, flowspec-firewall receive, and existing BGP sessions are
  unchanged. This responder only originates ephemeral rules over an existing session and withdraws them.

**Behavior to change:** None in existing code. New opt-in plugin that drives cp-survival-4; it emits `MitigationApplied`/`MitigationRemoved` on the EventBus (event types defined by child 4) carrying the announce/withdraw action and leaked-byte counts for the incident store.

## Data Flow (MANDATORY)

### Entry Point
- `ddosevent.AttackDetected` / `AttackCleared` on the EventBus; the `clear ddos-mitigation <target>` verb.

### Transformation Path
1. Receive `AttackDetected`. If response-level is `alert`: log the intended announcement, record, stop.
2. Build a surgical FlowSpec match from the vector tuple (dst prefix + proto + dst-port + src-port/tcp-flags), pick v4/v6, subtract the never-block allowlist. If empty → skip + log. If the peer/path does not support FlowSpec (config), build an RTBH /32 + BLACKHOLE instead.
3. Announce via cp-survival-4 (`announce flowspec … then discard for <max-mitigation-duration>`), respecting `announce-rate-limit` and coalescing repeats for the same target. Start the hold-down timer.
4. While mitigating, ignore the detector's `AttackCleared` (blind); run the leak-probe state machine: after hold-down, each `probe-interval` narrow to `probe-rate`, observe iface rate for `probe-window`, decide saturated (re-tighten + backoff) vs clear (withdraw).
5. On clear (or manual `clear`, or `max-mitigation-duration` expiry): withdraw via cp-survival-4 (`WithdrawNLRIBatch`), record the incident outcome.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| EventBus ↔ responder | subscribe `ddosevent` (`events.Register[T]`) | [ ] |
| responder ↔ reactor (announce/withdraw) | cp-survival-4 verb → `AnnounceNLRIBatch`/`WithdrawNLRIBatch` → UPDATE | [ ] |
| responder ↔ iface (probe) | `iface.GetRate` during the probe window | [ ] |
| config tree ↔ responder | YANG `ddos-flowspec` container → settings | [ ] |
| CLI ↔ responder | `clear ddos-mitigation <target>` verb | [ ] |

### Integration Points
- `internal/core/ddosevent` - event contract (consume)
- `spec-cp-survival-4` announce/withdraw verb / `reactor_api_batch.go` - actuation
- `internal/component/bgp/plugins/nlri/flowspec/encode.go` - match/action building (via cp-survival-4)
- `internal/component/iface/rate.go` - leak-probe measurement

### Architectural Verification
- [ ] No bypassed layers (actuation through cp-survival-4 / public batch API, not reactor internals)
- [ ] No unintended coupling (imports the event leaf + cp-survival-4's command, not the detector plugin)
- [ ] No duplicated functionality (reuses the FlowSpec encoder, BLACKHOLE community, announce API)
- [ ] Zero-copy preserved (UPDATE built by the existing buffer-first encoders via cp-survival-4)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | cp-survival-4's announce/withdraw verb (or `AnnounceNLRIBatch`) is callable from this plugin | cp-survival-4 status `ready`; `reactor_api_batch.go:28` exists today | flowspec mode blocked | depend on cp-survival-4; fallback: `AnnounceNLRIBatch` directly | unvalidated |
| A-2 | FlowSpec `traffic-rate` supports a configurable non-zero rate for the probe | grep-confirmed encoder has traffic-rate bps/pps | leak-probe must fully withdraw instead | reuse the encoder; unit test the probe action | confirmed (encoder supports it) |
| A-3 | Traffic leaked up to `probe-rate` reaches Ze's iface and is measurable | upstream-drop model: only matched traffic is dropped; leaked trickle transits | probe cannot distinguish ongoing vs ended | functional test with a synthetic upstream | unvalidated |
| A-4 | The event vector carries enough to build a surgical v4/v6 match | child 1 contract | matches become coarse (/32) | validate against `ddosevent/event.go` | unvalidated |
| A-5 | Runtime announcements are ephemeral (lost on restart) — acceptable (re-trigger) | cp-survival-4 A-2 | stale/duplicate rules after restart | reconcile on restart; doc | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Each leak-probe lets attack traffic through (bounded to `probe-rate`) | brief traffic burst every probe | bound by `probe-rate`; back off `probe-interval`; document the trade-off; inbound collector removes it (follow-on) |
| R-2 | Cannot confirm the upstream ACCEPTED the FlowSpec (RFC 8955 validation; no inbound feedback) | traffic never drops after announce | leak-probe disambiguates partly (no drop → treat as still-attacking, keep rule); optionally check own adj-rib-out / BMP; operator alert |
| R-3 | Announce/withdraw churn flaps the BGP session (damping / de-peering) | rapid announce+withdraw | `hold-down` + `announce-rate-limit` + incident coalescing; prefer narrow/widen over withdraw+reannounce |
| R-4 | Over-broad match or RTBH drops legitimate traffic upstream (collateral) | legit users lose service | surgical match + allowlist; default action rate-limit not discard; RTBH only as explicit fallback |
| R-5 | Depends on cp-survival-4 landing (D1) | flowspec mode cannot announce | fallback: call `AnnounceNLRIBatch` directly (exists today) |
| R-6 | Busy legit service exceeds `probe-rate`, so the probe always reads "saturated" (never clears) | rule never withdraws | set `probe-rate` above the detector's pre-attack baseline for the target; `max-mitigation-duration` valve |
| R-7 | Detector emits `AttackCleared` (blind) and a naive responder withdraws early | premature withdraw → flood returns | responder ignores `AttackCleared` while mitigating; clear decided ONLY by leak-probe |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `AttackDetected` (enforce mode) | → | build match → cp-survival-4 announce flowspec → UPDATE | `ddos-flowspec-respond.ci` |
| leak-probe reads iface rate below `probe-rate` after hold-down | → | responder withdraws via cp-survival-4 | `ddos-flowspec-leakprobe.ci` |
| `clear ddos-mitigation <target>` | → | responder withdraws the active rule | `ddos-flowspec-clear-cmd.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `AttackDetected` (enforce) with a v4 vector | a surgical FlowSpec (dst prefix, proto, and the discriminating port — destination port, or **source port** for reflection/amplification) with traffic-rate 0 (discard) is announced to the selected peer via cp-survival-4 |
| AC-1b | `AttackDetected` for a reflection/amplification flood (e.g. src-port 53) | the FlowSpec match keys on the source port (+ proto), not the destination port |
| AC-2 | target overlaps a never-block allowlist entry | the allowlisted prefix is excluded; a fully-allowlisted target announces nothing and logs why |
| AC-3 | within `hold-down` after announce | no probe and no withdraw occur |
| AC-4 | after hold-down, attack still raging | a leak-probe narrows to `probe-rate`, sees the rate saturate, re-tightens to discard, and extends hold-down (backoff) |
| AC-5 | after hold-down, attack has stopped | a leak-probe sees the rate stay below `probe-rate` and the rule is withdrawn via cp-survival-4 |
| AC-6 | response-level `alert` | the intended announcement is logged; nothing is sent |
| AC-7 | peer/path configured as non-FlowSpec | an RTBH /32 + BLACKHOLE is announced instead of FlowSpec |
| AC-8 | repeated `AttackDetected` for the same target | announcements are coalesced and capped by `announce-rate-limit` (no session churn) |
| AC-9 | `max-mitigation-duration` elapses | the rule is force-withdrawn and a warning is emitted |
| AC-10 | `clear ddos-mitigation <target>` | the active rule for that target is withdrawn |
| AC-11 | `AttackDetected` with an IPv6 target | an `ipv6/flow` FlowSpec is announced |
| AC-12 | detector emits `AttackCleared` while mitigating | the responder does NOT withdraw on it (clear decided by leak-probe only) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables flowspec mode; a volumetric flood hits | `AttackDetected` → surgical FlowSpec announced upstream | `ddos-flowspec-respond.ci` + interop |
| 2 | the flood stops mid-incident | leak-probe after hold-down sees low rate → withdraw | `ddos-flowspec-leakprobe.ci` |
| 3 | upstream does not speak FlowSpec | RTBH /32 + BLACKHOLE announced instead | `ddos-flowspec-rtbh.ci` + interop |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildFlowspecMatchFromVector` | `internal/plugins/ddosflowspec/match_test.go` | surgical v4/v6 match + action from the event vector (AC-1, AC-11) | |
| `TestAllowlistSubtraction` | `internal/plugins/ddosflowspec/match_test.go` | allowlist excluded; fully-allowlisted → no announce (AC-2) | |
| `TestRTBHFallbackBuild` | `internal/plugins/ddosflowspec/match_test.go` | /32 + BLACKHOLE when FlowSpec unsupported (AC-7) | |
| `TestLeakProbeSaturatedReTightens` | `internal/plugins/ddosflowspec/probe_test.go` | saturated probe → re-tighten + backoff (AC-4) | |
| `TestLeakProbeClearWithdraws` | `internal/plugins/ddosflowspec/probe_test.go` | sub-probe-rate → withdraw (AC-5) | |
| `TestHoldDownBlocksEarlyProbe` | `internal/plugins/ddosflowspec/probe_test.go` | no probe/withdraw within hold-down (AC-3) | |
| `TestIgnoresDetectorClearWhileMitigating` | `internal/plugins/ddosflowspec/responder_test.go` | AttackCleared does not withdraw (AC-12) | |
| `TestAnnounceRateLimitAndCoalesce` | `internal/plugins/ddosflowspec/announce_test.go` | repeats coalesced; cap enforced (AC-8) | |
| `TestMaxDurationForceWithdraw` | `internal/plugins/ddosflowspec/responder_test.go` | safety valve (AC-9) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hold-down (s) | 1-86400 | 86400 | 0 | 86401 |
| probe-interval (s) | 1-3600 | 3600 | 0 | 3601 |
| probe-window (s) | 1-300 | 300 | 0 | 301 |
| probe-rate (bps) | 1-… | n/a | 0 | N/A |
| announce-rate-limit (per min) | 1-600 | 600 | 0 | 601 |
| max-mitigation-duration (s) | 0-604800 | 604800 | N/A (0 = no cap) | 604801 |
| backoff-cap (s) | hold-down..604800 | 604800 | < hold-down | 604801 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-flowspec-respond` | `test/plugin/ddos-flowspec-respond.ci` | detect → surgical FlowSpec announced | |
| `ddos-flowspec-leakprobe` | `test/plugin/ddos-flowspec-leakprobe.ci` | hold-down → probe → withdraw when clear | |
| `ddos-flowspec-rtbh` | `test/plugin/ddos-flowspec-rtbh.ci` | non-FlowSpec peer → RTBH /32 + BLACKHOLE | |
| `ddos-flowspec-clear-cmd` | `test/plugin/ddos-flowspec-clear-cmd.ci` | manual clear withdraws | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-ddos-flowspec-originate` | `test/interop/scenarios/` | ExaBGP / GoBGP / BIRD | a real peer accepts the detector-triggered FlowSpec NLRI + action (reuses cp-survival-4 interop scaffolding) | |
| `NN-ddos-rtbh-originate` | `test/interop/scenarios/` | ExaBGP / GoBGP | a real peer receives the /32 with BLACKHOLE | |

### Future (deferred tests)
- Passive (no-leak) clear via an inbound flow collector — deferred follow-on (umbrella).

## Files to Modify
- `internal/component/plugin/all/all.go` - add the responder to the composition root (`make generate`)
- `docs/features.md` - add the "DDoS upstream FlowSpec/RTBH mitigation" row
- `docs/guide/plugins.md` - list the `ddosflowspec` plugin
- `docs/guide/command-reference.md` - document `clear ddos-mitigation`
- `docs/guide/ddos-mitigation.md` - the leak-probe behavior + the no-confirm-of-acceptance caveat (R-2)

### BGP Family Gate
Evaluated per `ai/patterns/bgp-family.md`. **N/A** — no new SAFI/capability/attribute; reuses FlowSpec
SAFI 133/134 + BLACKHOLE via cp-survival-4 / the existing encoder.

| BGP Integration Point | Needed? | Evidence |
|----------------------|---------|----------|
| New SAFI / NLRI codec | No | reuses `plugins/nlri/flowspec/encode.go` |
| New capability | No | FlowSpec capability already negotiated |
| New attribute/community | No | BLACKHOLE `community.go:99`; traffic-action ext-comm already encoded |
| ExaBGP bridge family | No | FlowSpec bridge already exists |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (config) | Yes | `internal/plugins/ddosflowspec/yang/ze-ddos-flowspec-conf.yang` — response-level, action, selector, allowlist, probe/safety timers; native constraints per `ai/patterns/config-option.md` |
| CLI command | Yes | `clear ddos-mitigation <target>` (shared verb with the local responder; one owner) — action-before-identifier |
| Functional test | Yes | `test/plugin/ddos-flowspec-*.ci` |
| Interop test | Yes | `test/interop/scenarios/NN-ddos-flowspec-originate` |
| Prometheus counters | Yes | active-announcements, leaked-during-probe bytes, announce/withdraw counts — surfaced in child 4 |
| Doctor check | Yes (child 4) | the FlowSpec BGP session must be up (flowspec mode is useless otherwise) |
| Env var registration | No | YANG config only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Maybe (reuses cp-survival-4) | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` |
| 9 | RFC behavior implemented? | Yes (8955/7999 via reuse) | `rfc/short/rfc8955.md`, `rfc/short/rfc7999.md` |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin/command changed? | Yes | `docs/plugin-overview.md` |

## Files to Create
- `internal/plugins/ddosflowspec/register.go` - registration, event subscription, YANG binding
- `internal/plugins/ddosflowspec/responder.go` - event handlers, mitigation lifecycle, max-duration valve
- `internal/plugins/ddosflowspec/probe.go` - leak-probe state machine (hold-down, probe cycle, backoff)
- `internal/plugins/ddosflowspec/match.go` - vector → FlowSpec v4/v6 match + action; allowlist; RTBH build
- `internal/plugins/ddosflowspec/announce.go` - drive cp-survival-4 announce/withdraw; rate-limit + coalesce
- `internal/plugins/ddosflowspec/cmd.go` - `clear ddos-mitigation` verb (shared with local responder)
- `internal/plugins/ddosflowspec/config.go` - config parse + validation
- `internal/plugins/ddosflowspec/yang/ze-ddos-flowspec-conf.yang` - config schema
- `internal/plugins/ddosflowspec/*_test.go` - unit tests above
- `test/plugin/ddos-flowspec-respond.ci`, `ddos-flowspec-leakprobe.ci`, `ddos-flowspec-rtbh.ci`, `ddos-flowspec-clear-cmd.ci`
- `test/interop/scenarios/NN-ddos-flowspec-originate/`, `NN-ddos-rtbh-originate/`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + detector + cp-survival-4 |
| 2. Audit | Files to Create/Modify, TDD Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — plugin skeleton + YANG subscribing to `ddosevent` + a failing `ddos-flowspec-respond.ci`; handler is a stub.
2. **Phase: Match + announce** — vector → FlowSpec v4/v6 match + action + allowlist; drive cp-survival-4 announce; rate-limit + coalesce.
   - Tests: `TestBuildFlowspecMatchFromVector`, `TestAllowlistSubtraction`, `TestAnnounceRateLimitAndCoalesce`; `ddos-flowspec-respond.ci`.
3. **Phase: Leak-probe** — hold-down, probe cycle (narrow to probe-rate, read iface rate, saturated vs clear), backoff, withdraw; ignore detector clear.
   - Tests: `TestHoldDownBlocksEarlyProbe`, `TestLeakProbeSaturatedReTightens`, `TestLeakProbeClearWithdraws`, `TestIgnoresDetectorClearWhileMitigating`; `ddos-flowspec-leakprobe.ci`.
4. **Phase: RTBH fallback + safety** — RTBH build, max-duration valve, manual clear, alert mode.
   - Tests: `TestRTBHFallbackBuild`, `TestMaxDurationForceWithdraw`; `ddos-flowspec-rtbh.ci`, `ddos-flowspec-clear-cmd.ci`.
5. **Phase: Interop** — originate to ExaBGP/GoBGP/BIRD (reuse cp-survival-4 scaffolding).
6. **Full verification** → `make ze-verify-changed` + `make generate`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | each AC-N has implementation with file:line |
| Correctness | clear decided by leak-probe ONLY; detector AttackCleared ignored while mitigating; hold-down honored |
| Foot-gun | match surgical + allowlist; default rate-limit not discard; RTBH explicit-only |
| Session safety | announce-rate-limit + coalesce + hold-down prevent churn |
| Naming | YANG kebab-case; CLI action-before-identifier; v4/v6 family selection correct |
| Data flow | actuation via cp-survival-4, not reactor internals |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| responder plugin registered | `make ze-inventory` lists `ddosflowspec` |
| functional tests pass | `ze-test bgp plugin test/plugin/ddos-flowspec-*.ci` |
| interop passes | cp-survival-4-style originate scenario green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Collateral upstream | match cannot exceed the vector; allowlist enforced; RTBH gated behind explicit config |
| Session abuse | announce-rate-limit caps churn; cannot be driven to flap by repeated events |
| Leak bound | probe leak is bounded by `probe-rate`; cannot accidentally fully un-mitigate |
| Stale state | max-duration valve + restart reconcile; ephemeral-announcement behavior documented |
| Input validation | YANG ranges + allowlist prefix bounds enforced |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- **Leak-probe leaks bounded attack traffic** to learn the attack is over; a passive, no-leak clear
  needs an inbound flow collector (umbrella follow-on).
- **No confirmation the upstream accepted the FlowSpec** (R-2): RFC 8955 validation can silently reject;
  the leak-probe treats "traffic never dropped" as still-attacking and keeps the rule.
- RTBH fallback drops ALL traffic to the /32 (coarse); explicit-config-only.

## Design Insights
- The attack-end question has no clean local answer once you mitigate upstream: the only honest signal
  is to let a bounded trickle through and watch. The leak-probe makes that trade-off explicit and small.

## Implementation Audit

| AC | Evidence |
|----|----------|
| AC-1 | `buildMatch` constructs FlowSpec match from vector (`match.go`); `TestBuildMatchFromVector` passes |
| AC-2 | `shouldAnnounce` checks allowlist overlap (`match.go`); `TestAllowlistOverlap` passes |
| AC-3 | probe `Tick` returns `probeActionProbe` only after hold-down (`probe.go`); `TestHoldDownBlocksEarlyProbe` passes |
| AC-4 | `Tick(saturated=true)` → `probeActionReTighten` + backoff (`probe.go`); `TestSaturatedReTightens` passes |
| AC-5 | `Tick(saturated=false)` after hold-down → `probeActionWithdraw` (`probe.go`); `TestClearWithdraws` passes |
| AC-6 | alert mode returns early (`responder.go`); `TestAlertModeInstallsNothing` passes |
| AC-12 | `onCleared` is a no-op while mitigating (`responder.go`); `TestIgnoresDetectorClearWhileMitigating` passes |

### Files created
- `internal/plugins/ddosflowspec/probe.go` -- leak-probe state machine
- `internal/plugins/ddosflowspec/probe_test.go` -- 6 tests
- `internal/plugins/ddosflowspec/match.go` -- vector → FlowSpec/RTBH match
- `internal/plugins/ddosflowspec/match_test.go` -- 4 tests
- `internal/plugins/ddosflowspec/responder.go` -- event handlers, lifecycle
- `internal/plugins/ddosflowspec/responder_test.go` -- 5 tests (was 4, includes allowlist)
- `internal/plugins/ddosflowspec/config.go` -- all probe/safety timers
- `internal/plugins/ddosflowspec/register.go` -- plugin registration
- `internal/plugins/ddosflowspec/yang/` -- YANG schema, embed, register

### Deferred
- Actual announce/withdraw via cp-survival-4 (stubbed with injectable vars)
- Interop tests (need cp-survival-4 scaffolding)

## Review Gate
### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/ddosflowspec`)
- [ ] Interop tests pass (originate to a real peer)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-3-flowspec-responder.md`
