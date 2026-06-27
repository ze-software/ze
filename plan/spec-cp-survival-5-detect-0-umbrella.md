# Spec: Automatic DDoS Attack Detection & Auto-Mitigation (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-cp-survival-0-umbrella, spec-cp-survival-4-flowspec-origination |
| Phase | - |
| Updated | 2026-06-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-cp-survival-0-umbrella.md` (parent) and `plan/spec-cp-survival-4-flowspec-origination.md` (the origination lever this drives)
4. `internal/component/iface/rate.go` (detection signal), `internal/component/firewall/model.go` (local mitigation + counters), `internal/component/bgp/reactor/reactor_api_batch.go` (`AnnounceNLRIBatch`)

## Task

The `cp-survival` umbrella deliberately excluded **automatic attack detection**:
> *"The umbrella does not attempt automatic attack detection. Gap D provides the lever (originate on
> demand / from a firewall rule); deciding when to pull it is out of scope (operator or external
> detector)."* (`spec-cp-survival-0-umbrella.md` Known Limitations)

This umbrella closes that gap. It adds a **detector** that watches local traffic, learns a baseline,
and decides *when an attack starts and when it ends*, plus **responders** that act on that decision by
driving mechanisms that **already exist**: the local firewall (component `firewall`) and the on-demand
FlowSpec/RTBH origination verb (`spec-cp-survival-4`). It reimplements neither mitigation mechanism;
it supplies the missing automatic decision layer and the glue.

Inspiration: the Flowtriq `ftagent` (Python) DDoS agent — local rate baseline, hysteresis on attack
end, on-host firewall blocks, optional upstream pipeline signalling. We port the *decision logic*, not
the code, onto Ze's primitives.

This umbrella ships **no production code of its own** beyond coordination; the four child specs own
implementation. It owns the shared research and the cross-cutting design decisions.

### Two operating modes (per the responder enabled)
- **local** — copy `ftagent` behaviour: detect on local interface rate, mitigate with an on-host nft
  drop rule, clear when the interface rate falls (valid because nft drops *after* the NIC RX counter).
- **flowspec** — announce a surgical FlowSpec (or RTBH) to an upstream peer via `spec-cp-survival-4`;
  clear via a narrowed-rate **leak-probe**, because upstream drop blinds every local sensor and Ze has
  **no inbound flow collector** today.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` (EventBus Typed Payloads + DirectBridge) - detector→responder comms
  → Decision: detector emits typed EventBus events (`events.Register[T]`, `pkg/ze/eventbus.go`); responders subscribe. EventBus is for broadcast (1→N), which is exactly detector→responders. NOT DirectBridge (that is request/response).
  → Constraint: register typed events via `events.Register[T]`, never raw `bus.Subscribe`.
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Constraint: each responder/detector child owns its own command, YANG, doctor check, and metrics; removing a child removes all its features with no dangling references in core.
- [ ] `ai/patterns/plugin.md` + `ai/patterns/registration.md` - plugin/registration shape
  → Constraint: register via `init()` in `register.go`; goroutine lifecycle per `ai/rules/goroutine-lifecycle.md` (tick loop started in `OnAllPluginsReady`, stopped on shutdown; mitigation withdrawn on clean stop — Ze owns what it touches).
- [ ] `ai/rules/config-surface.md` + `ai/rules/config-naming.md` - the timer/mode config
  → Decision: mode, response-level, allowlist, and the four timers are operational policy → YANG leaves, not env vars. Durations use `ze-types.yang` duration type with `range`.
- [ ] `ai/rules/buffer-first.md` - origination path
  → Constraint: any UPDATE we cause is built through the existing buffer-first encoders via `spec-cp-survival-4`; this umbrella adds no new wire encoding.
- [ ] `spec-cp-survival-4-flowspec-origination.md` - the actuation lever
  → Constraint: the flowspec/RTBH responder CALLS the `announce flowspec|blackhole … for <duration>` verb (or `AnnounceNLRIBatch` directly); it must not re-encode FlowSpec or duplicate the origination path.

### RFC Summaries
- [ ] `rfc/short/rfc8955.md` (+ `rfc8956` v6) - FlowSpec match components + traffic-action ext-comm
  → Constraint: surgical match uses the existing 13-component encoder; rate-limit/discard actions already encoded. Upstream may reject a flow failing RFC 8955 validation against the best unicast route.
- [ ] `rfc/short/rfc7999.md` - BLACKHOLE community 0xFFFF029A (RTBH fallback)
  → Constraint: RTBH = unicast /32 NLRI + BLACKHOLE community; reuse `community.go:99`.

**Key insights:**
- The single hardest problem is **clear detection under FlowSpec mitigation**: a successful upstream
  FlowSpec removes the very traffic the sensor measures. Ze has **no inbound flow collector**
  (`flowexport` is export-only), so the clean "Option B" (independent upstream telemetry, as `ftagent`
  uses) does not exist. Flowspec-mode clear is therefore either (a) **leak-probe** (narrow the rule to
  a small non-zero rate every check-interval and watch) or (b) a **new inbound collector** (follow-on).
- **local-mode** clear is clean and copies `ftagent` directly: nft drops occur *after* the kernel RX
  counter increments, so `iface` RxPps keeps reflecting the arriving flood even while it is dropped.
  The attack is "finished" only when RxPps actually falls. (Caveat: an XDP drop backend would break
  this — XDP_DROP precedes the RX counter — and would require reading the XDP map drop counter.)
- Detection and mitigation mechanisms already exist; the genuinely new capability is the **decision
  engine** (baseline + trigger + clear state machine + safety controls).

## Current Behavior (MANDATORY)

**Source files read:** (shared grounding; per-responder detail lives in the child specs)
- [ ] `internal/component/iface/rate.go` - `RegisterCollectNotify(fn CollectNotifyFunc)` (rate.go:66), `CollectNotifyFunc func([]InterfaceInfo)` (rate.go:59); rate tracker computes per-interface RxBps/TxBps/RxPps/TxPps at ~1 Hz (`rateDelta` rate.go:181) and publishes `ze_interface_rx_packets_per_second` etc. (LSP-confirmed.)
  → Constraint: this is the detection signal; do not add a second `/proc/net/dev` reader. Subscribe via `RegisterCollectNotify` rather than polling.
- [ ] `internal/plugins/trafficusage/` - eBPF TCX per-IP/per-port byte counters: `ze_traffic_usage_ingress_bytes_total{interface,src_ip}`, `..._ingress_port_bytes_total{dst_port,protocol}` (absolute totals; derive rates from deltas).
  → Constraint: source-IP dispersion/entropy + per-(dst-IP,port) attribution come from here; bytes only, no packet counts at this granularity.
- [ ] `internal/plugins/flowexport/conntrack/flow.go` - `FlowEntry{Src,Dst,Ports,Proto,Bytes,Packets}`; `conntrack/delta.go DeltaTracker` for per-flow deltas. `flowexport` is **export-only** (no receiver).
  → Constraint: new-flow rate (SYN-flood signal) and 5-tuple match construction come from here. There is NO inbound sFlow/NetFlow/IPFIX collector to reuse.
- [ ] `internal/component/firewall/model.go` + `internal/plugins/firewall/nft/backend_linux.go` - `RegisterTables(name,tables)`, `ApplyAll()`, `GetCounters(name) []ChainCounters` with `TermCounter{Packets,Bytes}` (nft counter per rule).
  → Constraint: local-mode mitigation registers a table and reads its per-term counter; flowspec-egress (cp-survival-4 D2) can lower a tagged rule into an outbound announce.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go:28` - `AnnounceNLRIBatch(sel,batch)` / `WithdrawNLRIBatch`; the single actuation API behind cp-survival-4.
  → Constraint: flowspec/RTBH responder reaches the wire only through cp-survival-4's verb or this call; never a second announce path.
- [ ] `internal/component/bgp/attribute/community.go:99` - `CommunityBlackhole = 0xFFFF029A`.
  → Constraint: RTBH responder reuses this; no new attribute.

**Behavior to preserve:**
- `iface` rate collection, `trafficusage`, `flowexport` export, `firewall`, and cp-survival-4
  origination all keep working unchanged. This umbrella only *consumes* them.

**Behavior to change:** None in existing code. Each child adds new, opt-in plugins/commands.

## Data Flow (MANDATORY)

### Entry Point
- Local traffic statistics: `iface.RegisterCollectNotify` callback (1 Hz) + `trafficusage`/`flowexport`
  deltas. No external trigger — detection is automatic.

### Transformation Path
1. **Detect (two-stage, per child 1):** Stage 1 — `iface` aggregate rate vs rolling baseline (ported
   `ftagent` p99×3, with baseline-poisoning guards, absolute floor, startup grace) triggers. Stage 2 —
   on trigger ONLY, an on-attack pattern analysis (per-IP/flow via DirectBridge to trafficusage/flowexport)
   characterizes target/vector/family and filters false positives. Steady state reads only iface rate.
2. **Decide:** state machine `IDLE → MITIGATING → (PROBING|passive-clear) → COOLDOWN`, gated by the
   four timers (`check-interval`, `hold-down`, `clear-consecutive-checks`, `probe-window`).
3. **Emit:** detector publishes `AttackDetected{target, vector, characterization}` /
   `AttackCleared{target}` on the EventBus.
4. **Respond:** enabled responders subscribe and act —
   - local-nft: `firewall.RegisterTables`+`ApplyAll` (drop); clear when `iface` RxPps falls.
   - flowspec: cp-survival-4 `announce flowspec … for <duration>`; clear via leak-probe.
   - rtbh: cp-survival-4 `announce blackhole …`.
   - rest (optional): HTTP POST to an external scrubber; poll its status endpoint for clear.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface/trafficusage/flowexport ↔ detector | rate callback + metric/delta reads | [ ] |
| detector ↔ responders | typed EventBus event (`events.Register[T]`) | [ ] |
| local responder ↔ dataplane | `firewall.RegisterTables`/`ApplyAll`/`GetCounters` | [ ] |
| flowspec/rtbh responder ↔ wire | cp-survival-4 verb → `AnnounceNLRIBatch` → UPDATE | [ ] |

### Integration Points
- `internal/component/iface/rate.go` `RegisterCollectNotify` - detection signal
- `internal/component/firewall` `RegisterTables`/`GetCounters` - local responder
- `spec-cp-survival-4` announce verb / `reactor_api_batch.go AnnounceNLRIBatch` - flowspec/rtbh responder
- `pkg/ze/eventbus.go` / `internal/core/events/typed.go` - detector→responder broadcast

### Architectural Verification
- [ ] No bypassed layers (responder → existing subsystem API, never internals)
- [ ] No unintended coupling (detector emits events; responders independent and individually removable)
- [ ] No duplicated functionality (reuses iface rate, firewall, cp-survival-4 origination)
- [ ] Zero-copy preserved (origination via buffer-first cp-survival-4 path)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `iface.RegisterCollectNotify`/`GetRate` give ~1 Hz RxPps/RxBps sufficient for attack onset/offset | LSP-confirmed `rate.go:66/59`, telemetry survey | detector needs its own sampler | unit test the callback cadence | confirmed (API exists) |
| A-2 | Ze has NO inbound flow collector; flowspec-mode clean clear is impossible without leak-probe or a new collector | grep-confirmed: `flowexport/{sflow,ipfix,netflow9}` are encoders (`SetEncoder`, `CollectorConfig`=remote target); no `ReadFromUDP`/`Listen` receiver | flowspec clear can be passive | grep for sflow/netflow/ipfix receiver | confirmed (export-only) |
| A-3 | The flowspec/rtbh responder can drive cp-survival-4's verb (or `AnnounceNLRIBatch`) | cp-survival-4 status `ready` | flowspec mode blocked on D1 landing first | depend on cp-survival-4; fallback: call `AnnounceNLRIBatch` directly (exists today) | unvalidated |
| A-4 | nft drop leaves the NIC RX counter intact, so local-mode RxPps stays a valid clear signal | netfilter runs after driver RX accounting | local clear flaps | QEMU functional test: RxPps stays high while an nft drop rule matches a synthetic flood | unvalidated |
| A-5 | Typed EventBus events suit detector→responder decoupling | `ai/rules/plugin-design.md` EventBus | need request/response instead | `events.Register[T]` example + a responder consumer | unvalidated |
| A-6 | `firewall.RegisterTables`/`ApplyAll`/`GetCounters` are callable at runtime from a new plugin | grep-confirmed `GetCounters` at `firewall/nft/backend_linux.go:269` AND `firewall/vpp/backend_linux.go:396` (both dataplanes); `flowspec-firewall` uses RegisterTables/ApplyAll | local responder needs a different hook | trace RegisterTables/ApplyAll call from a plugin + unit test | confirmed (counters exist, both backends) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | False-positive auto-mitigation on a flash crowd or backup job | legit traffic dropped | multi-signal detection (entropy/new-flow), `confirm-duration`, **alert-only default** response-level |
| R-2 | Flowspec-mode sensor blindness (upstream drop hides the attack) | rate falls to baseline instantly after announce | leak-probe (narrowed non-zero rate) every check-interval; inbound collector as follow-on; documented |
| R-3 | Mitigation flapping churns the BGP session (upstream may damp/de-peer) | rapid announce/withdraw | `hold-down` minimum + announce rate-limit + incident coalescing; prefer narrow/widen over withdraw+reannounce |
| R-4 | Auto-generated match blackholes own infrastructure (control plane, DNS, mgmt) | self-inflicted outage | **never-block allowlist** subtracted from every match; match validation; ties to cp-survival-4 R-2 foot-gun |
| R-5 | Registering child #5 edits `spec-cp-survival-0-umbrella.md` while another session owns it | merge/overwrite conflict | minimal append-only edit; flag to user; or defer registration to a coordinated step |
| R-6 | Stuck mitigation never withdraws (probe keeps seeing residual) | rule up far longer than the incident | `max-mitigation-duration` safety valve + force-withdraw + manual `clear` command + alert |
| R-7 | XDP firewall backend would silently break local-mode clear signal | RxPps drops the moment XDP attaches | document nft-only for v1; if XDP added, read XDP drop-counter instead of RxPps |
| R-8 | Detection is blind on a DPDK VPP dataplane: `iface` rate skips VPP interfaces (`detailsToInfo` Stats=nil, `rate.go:140`); whole detect→respond chain dead on VPP | detector never triggers on a VPP box | RESOLVED at source by child `spec-cp-survival-5-detect-1a-vpp-iface-rate` (wires `vpp/telemetry.go statsProvider.GetInterfaceStats` into `iface.InterfaceInfo.Stats`); detector stays dataplane-agnostic; QEMU-tested |

## Wiring Test (MANDATORY)

The umbrella has no executable feature of its own; its wiring is that each child's wiring test passes.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Each child spec closed | → | child wiring tests | child `/ze-review` gates + functional tests (see children) |
| synthetic flood on a test iface | → | detector emits `AttackDetected` | `ddos-detect-trigger.ci` (child 1) |
| `AttackDetected` event present | → | local responder installs nft drop | `ddos-local-respond.ci` (child 2) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child 1 (detector) complete | `spec-cp-survival-5-detect-1-detector.md` closed: baseline + multi-signal trigger + clear state machine + four timers; emits typed `AttackDetected`/`AttackCleared` |
| AC-2 | Child 2 (local responder) complete | `…-2-local-responder.md` closed: nft drop on detect, clear on RxPps fall; copies `ftagent` local behaviour |
| AC-3 | Child 3 (flowspec/rtbh responder) complete | `…-3-flowspec-responder.md` closed: surgical FlowSpec via cp-survival-4 + RTBH fallback; leak-probe clear; safety controls (allowlist, hold-down, rate-limit, max-duration) |
| AC-4 | Child 4 (observability) complete | `…-4-observability.md` closed: Prometheus metrics, `show ddos status/incidents`, doctor check (FlowSpec BGP session up), incident top-talker evidence record |
| AC-5 | Operator reads docs | `docs/guide/ddos-mitigation.md` (cp-survival-0 AC-5) extended with the auto-detection section: modes, the flowspec-blindness/leak-probe caveat, the alert-only-first rollout, the allowlist |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs in local mode; a flood hits | iface rate → detector → `AttackDetected` → nft drop; flood stops → RxPps falls → `AttackCleared` → nft removed | child-1 + child-2 functional tests |
| 2 | runs in flowspec mode; a volumetric flood hits | detector → cp-survival-4 announce surgical FlowSpec; leak-probe detects end → withdraw | child-3 functional test + cp-survival-4 interop |
| 3 | runs alert-only; a flood hits | detector emits event; responders log/report but take no action | child-1 + child-4 (status shows would-mitigate) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (per child) | (child packages) | baseline math, state machine transitions, match construction, allowlist subtraction | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-detect-trigger` | `test/plugin/ddos-detect-trigger.ci` | synthetic flood trips the detector | |
| `ddos-local-respond` | `test/plugin/ddos-local-respond.ci` | detect → nft drop → clear on rate fall | |
| `ddos-flowspec-respond` | `test/plugin/ddos-flowspec-respond.ci` | detect → cp-survival-4 announce → leak-probe withdraw | |
| `ddos-doc` | `test/plugin/ddos-doc.ci` (+ `make ze-doc-test`) | mitigation guide builds, anchors resolve | |

### Interop Tests
Owned by child 3 (flowspec/rtbh to ExaBGP/GoBGP/BIRD), reusing cp-survival-4 interop scaffolding.

### QEMU / Linux-only
| Test | Validates |
|------|-----------|
| RxPps-under-nft-drop | A-4: NIC RX counter unaffected by nft drop (local-mode clear validity) |

## Files to Modify
- `plan/spec-cp-survival-0-umbrella.md` - register this as child #5 (AC + recommended-order row). **R-5: minimal append, coordinate with the active session.**
- `docs/guide/ddos-mitigation.md` - extend with the auto-detection section (created by cp-survival-0)
- `docs/features.md` - add "automatic DDoS detection & auto-mitigation" row

## Files to Create (child specs)
- `plan/spec-cp-survival-5-detect-1a-vpp-iface-rate.md` - **prerequisite**: wire VPP interface stats into `iface.InterfaceInfo.Stats` so the rate signal is dataplane-agnostic (closes R-8)  **[READY]**
- `plan/spec-cp-survival-5-detect-1-detector.md` - detector: two-stage (iface rate trigger + on-attack pattern analysis), baseline, state machine, timers, EventBus events  **[READY]**
- `plan/spec-cp-survival-5-detect-2-local-responder.md` - on-host nft responder (`ftagent` local parity)  **[READY]**
- `plan/spec-cp-survival-5-detect-3-flowspec-responder.md` - FlowSpec/RTBH responder over cp-survival-4 + safety controls + leak-probe  **[READY]**
- `plan/spec-cp-survival-5-detect-4-observability.md` - metrics, CLI, doctor, incident evidence  **[READY]**
- (follow-on, not in v1) inbound flow collector spec — true un-blinded flowspec-mode clear

## Implementation Steps

The umbrella is closed by closing its children and extending the deployment doc. Recommended order:

1. **child 1a (VPP iface rate)** — prerequisite: makes the iface rate signal dataplane-agnostic so detection works on VPP (closes R-8). Independent iface-component fix; QEMU-tested.
2. **child 1 (detector)** — the genuinely new capability; emits events; no actuation. Independently testable. Depends on 1a for VPP coverage.
3. **child 2 (local responder)** — `ftagent` local parity; depends on child 1 + firewall.
4. **child 3 (flowspec responder)** — depends on child 1 + cp-survival-4 (D1) landing.
5. **child 4 (observability)** — depends on children 1-3 producing state.
6. **deployment doc** — extend cp-survival-0's `ddos-mitigation.md`.

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + the relevant child |
| 14. Present summary | Roll up child summaries into `plan/learned/NNN-cp-survival-5-detect-0-umbrella.md` |

## Known Limitations
- **No clean flowspec-mode clear without new infrastructure.** Until an inbound flow collector exists,
  flowspec mode detects attack-end by leak-probe (bounded traffic leak) rather than passive telemetry.
- **Detection is local-vantage only.** Ze sees what reaches its interfaces; an attack fully absorbed
  upstream before any Ze sensor is invisible to detection (only relevant once flowspec mode is active).
- **nft backend assumed for local mode.** An XDP drop backend would require a different clear signal (R-7).

## Design Insights
- The clear signal must be measured at or before the drop point; a victim-side sensor goes blind under
  its own mitigation. This single constraint determines every mode's clear mechanism.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit

### Roll-up of child implementations

| Child | Package | Tests | Status |
|-------|---------|-------|--------|
| 1a (VPP iface rate) | `internal/component/vpp/vpp.go`, `internal/plugins/iface/vpp/query.go` | 6 new | core complete |
| 1 (detector) | `internal/core/ddosevent/`, `internal/plugins/ddosdetect/` | 20 new | Stage-1 complete, Stage-2 characterize deferred |
| 2 (local responder) | `internal/plugins/ddoslocal/` | 6 new | core complete, max-duration timer + CLI deferred |
| 3 (flowspec responder) | `internal/plugins/ddosflowspec/` | 15 new | core + probe complete, announce stubbed (needs cp-survival-4) |
| 4 (observability) | `internal/plugins/ddosobserve/` | 4 new | incident store complete, CLI + doctor + metrics deferred |

Total: 51 new unit tests, all passing. Lint clean.

## Review Gate
### Final status
- [ ] All four child specs closed
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated (each child closed + deployment doc)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] All four child specs closed
- [ ] Implementation Summary filled (roll-up of child summaries)
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-0-umbrella.md`
