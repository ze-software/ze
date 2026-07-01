# Spec: DDoS Attack Characterization (Stage-2 Producer)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | cp-survival-5-detect-0 (closed), flow-export-2 (closed) |
| Phase | 5/5 |
| Updated | 2026-07-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-design.md` (Import Rules / DirectBridge / OptionalDependencies), `ai/patterns/config-option.md`, `ai/rules/doctor-checks.md`
4. `internal/plugins/ddos/detect/detector.go`, `internal/core/ddosevent/event.go`, `internal/plugins/trafficusage/show.go`, `internal/plugins/flowexport/exporter.go`

## Task

The cp-survival-5-detect set shipped a two-stage DDoS detector but deferred Stage 2
("characterization"): the detector emits `FamilyGenericFlood` with an empty
`VectorTuple` and no `TopSources` (`detect/detector.go:144-145`). Both responders
(`ddos-local` kernel, `ddos-flowspec` upstream) then refuse to act because they require
a valid `DstPrefix` (`local/match.go:44-46`, `flowspec/match.go:30-31`). The mitigation
chain is fully wired but inert.

This spec adds Stage-2 characterization to the `ddos-detect` plugin: on rate trigger,
query on-box flow data, compute the target prefix, the narrowest discriminating
`VectorTuple` (proto / ports / TCP flags), the `AttackFamily`, and `TopSources`, then
emit a fully-populated `AttackDetected` so the existing responders build surgical block
rules. Goal: production-deployable.

Locked scope decisions (SCOPE gate, 2026-06-28):
- Data source: trafficusage (existing `ze-show:traffic-usage` RPC) + a NEW flowexport
  recent-flow query RPC (flowexport is push-only today).
- Method: heuristic (proto/port signature + Shannon entropy), NOT machine learning.
- Coverage: volumetric vectors (UDP / SYN / ICMP / reflection / fragment / generic).
  Low-rate / application-layer is OUT of scope (deferred).
- Placement: extend the `ddos-detect` plugin (new on-trigger path); responders unchanged.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - cross-plugin communication and dependency rules
  → Constraint: plugins MUST NOT import sibling plugins (line 133); cross-plugin data flows via `DispatchCommand` (text RPC routed by the engine), NOT direct calls. DirectBridge (`pkg/plugin/rpc/bridge.go`) is engine↔plugin only.
  → Constraint: a `DispatchCommand` targeting another plugin AT STARTUP must run in `OnAllPluginsReady` (line 178). Characterization runs at trigger time (runtime), so the event loop is safe, but the data-source plugins must be loaded.
  → Decision: declare `traffic-usage` and `flow-export` as `OptionalDependencies` (soft, lines 204-229) with graceful fallback. If a source is absent the dispatch returns `ErrUnknownCommand`; treat as "source absent", log once via `sync.Once` WARN, and fall back to `FamilyGenericFlood` (current behavior, the bgp-rs pattern).
  → Constraint: cross-boundary payloads are value types only (already satisfied: `VectorTuple`/`AttackDetected` carry `netip.Prefix`/`netip.Addr`/`uint8`).
- [ ] `ai/patterns/plugin.md` - extending an existing plugin
  → Constraint: extend `ddos-detect` in place; atomic logger already present (`detector.go:16-29`). No new plugin, so no `TestAllPluginsRegistered` count bump for detect. The NEW flowexport recent-flow RPC IS a new command surface and needs registration + YANG.
- [ ] `ai/patterns/registration.md` - RPC + YANG registration
  → Constraint: the new flowexport recent-flow query registers via `pluginserver.RegisterRPCs` in flowexport's `plugins/.../cmd` surface, with a YANG command tree; it is dispatched by `WireMethod` through the engine (mirror of `ze-show:traffic-usage`).
- [ ] `ai/patterns/config-option.md` - new tuning leaves
  → Constraint: new `ddos-detect` tuning leaves (characterize on/off, top-n-sources, characterize-window, characterize-timeout, entropy thresholds) extend the existing `ze-ddos-detect-conf` module; every leaf needs `type` + `default` + `description` + `range`.
  → Decision: operational policy → YANG leaves, not env vars (`config-surface.md`).
- [ ] `ai/rules/doctor-checks.md` - runtime dependency readiness
  → Decision: characterization depends on `traffic-usage` (track-ip) and/or `flow-export` (conntrack) being enabled. Add a `ddos-detect` doctor check (via `Registration.DoctorChecks`) that warns when characterization is enabled but neither source is configured; register diagnostic code `doctor-ddos-detect-no-flow-source` in `internal/core/diagnostic/codes.go`.
- [ ] `ai/rules/discovery-updates.md` - discovery surfaces
  → Constraint: new RPC + behavior change must update `docs/features.md`, the ddos guide, an `ai/INDEX.md` keyword row, and remain reachable via `make ze-inventory`; learned summary + `ai/LEARNED-INDEX.md` at closure.

### Learned Summaries
- [ ] `plan/learned/1011-cp-survival-5-detect-0-umbrella.md` - the deferred Stage 2
  → Constraint: Stage 2 is on-trigger ONLY; steady-state cost must stay ~zero (one rate comparison per interface per tick). Do not add per-IP monitoring to the steady-state path.
  → Constraint: baseline poisoning fix (compute threshold BEFORE `baseline.Add`, exclude above-threshold samples) must NOT regress (`detector.go:111-116`).
  → Correction: the summary's phrase "DirectBridge to trafficusage/flowexport" is imprecise. The verified path is `DispatchCommand` (engine-routed text RPC), because plugins cannot call siblings directly.
- [ ] `plan/learned/819-flow-export-2-flow-records.md` - flow record shape and limits
  → Constraint: `ConntrackFlow` value type carries `SrcAddr/DstAddr/SrcPort/DstPort/Protocol/Bytes/Packets/FirstMs/LastMs/SrcAS/DstAS` (IPv4 and IPv6).
  → Constraint: conntrack export is periodic-dump only (no immediate destroy events). A recent-flow ring sees periodic batches via `ExportFlows`, not per-packet.

**Key insights:**
- The mitigation chain is wired and inert solely because the producer emits an empty `VectorTuple`. Filling `DstPrefix` is the unblock; proto/ports/flags make rules surgical.
- Two data sources, complementary by necessity: trafficusage gives cheap target dest-port/proto + top IPv4 talkers; flowexport gives source-port (reflection), TCP flags (SYN), IPv6, and full 5-tuple. The chosen family set REQUIRES both.
- Communication is `DispatchCommand` (engine-routed), and the sources are optional dependencies with graceful fallback to generic-flood.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/detect/detector.go` - rate trigger pipeline
  → Constraint: `onRate` (63) computes per-iface pps/bps deltas, tracks the hottest iface (`attackIface`, `peakRxPps/Bps`), feeds the confirm/clear state machine; `emitDetected` (140) emits `AttackDetected` with `Target: VectorTuple{}` (empty) and `Family: FamilyGenericFlood` (144-145). This is the single emit point to enrich. `onRate` holds `d.mu` (68).
- [ ] `internal/plugins/ddos/detect/state.go` - confirm/clear state machine
  → Constraint: `OnDetected` fires once on the idle/confirming→active transition (`activate`, 99-105). Characterization must run on THIS transition, not every tick.
- [ ] `internal/plugins/ddos/detect/register.go` - plugin wiring
  → Constraint: detect subscribes to iface rate via `iface.RegisterCollectNotify(det.onRate)` (91); declares `Dependencies: ["interface"]` (34). Add `OptionalDependencies` for the data sources here.
- [ ] `internal/core/ddosevent/event.go` - event contract (fields to fill)
  → Constraint: `AttackDetected{Interface, Target VectorTuple, Family, TopSources []netip.Addr, PeakRxPps, PeakRxBps}`; `VectorTuple{DstPrefix, Proto, DstPort, SrcPort, TCPFlags}` (30-46). Value types only. No schema change needed; all target fields already exist.
- [ ] `internal/plugins/ddos/local/match.go` + `flowspec/match.go` - responder gates
  → Constraint: `shouldMitigate`/`shouldAnnounce` refuse an invalid `DstPrefix` (local 44-46, flowspec 30-31). `buildMatch` (flowspec) consumes Proto/DstPort/SrcPort/TCPFlags (19-26); `buildDropTerm` (local 19-42) consumes Proto/DstPort/SrcPort but NOT TCPFlags. → filling `DstPrefix` unblocks both; local `TCPFlags` support is a small follow-on gap to close in this spec.
- [ ] `internal/plugins/trafficusage/show.go` + `monitor.go` - existing query source
  → Constraint: `ze-show:traffic-usage [name <iface>]` returns per-iface `ingress-ports`/`egress-ports` ({port, protocol, bytes}) and `ingress-ips`/`egress-ips` ({ip, bytes}). `counts.ingressIP`/`egressIP` are `map[uint32]uint64` IPv4-ONLY and "track-ip only" (monitor.go:28-29); bytes are cumulative absolute (32). ingressPort = dest (port,proto); egressPort = source (port,proto) (30-31).
  → Constraint: trafficusage yields target dest-port/proto + top IPv4 sources by bytes; it does NOT yield source-port, packet counts, TCP flags, or IPv6.
- [ ] `internal/plugins/flowexport/exporter.go` + `flowtypes.go` - flow data (push-only today)
  → Constraint: `ExportFlows(flows []ConntrackFlow)` (230) is the push dispatch to collector encoders; there is NO query/recent accessor. The tap = a bounded recent-flow ring fed at `ExportFlows` + a new query RPC.

**Behavior to preserve:**
- Steady-state detector cost (one rate comparison per iface per tick); characterization runs only on the active transition.
- Baseline poisoning guard (threshold computed before `baseline.Add`).
- Responder gating semantics (allowlist + valid-prefix checks) and the EventBus 1:N contract.
- When sources are absent/empty, emit `FamilyGenericFlood` with the best target available (current behavior is the floor, never worse).

**Behavior to change:**
- `emitDetected` emits a populated `Target`/`Family`/`TopSources` when characterization succeeds.
- `ddos-local` `buildDropTerm` learns to match `TCPFlags` (close the SYN gap).

## Data Flow (MANDATORY)

### Entry Point
- State machine transition idle/confirming → active inside `detector.onRate` (the rate trigger), carrying `attackIface`, `peakRxPps/Bps` captured under `d.mu`.

### Transformation Path
1. On the active transition, snapshot `attackIface` + peaks under lock, then run characterization OFF the `onRate` hot path (spawned goroutine with a bounded timeout) so the steady-state tick and the mutex are not blocked.
2. Source A (trafficusage): `DispatchCommand("show traffic-usage name <attackIface>")` → parse JSON → dominant ingress (port,proto) = victim service + proto; top `ingress-ips` = candidate top sources (IPv4); dominant `egress-ips`/dest = candidate target.
3. Source B (flowexport, NEW RPC): `DispatchCommand(<recent-flow query> name <attackIface>)` → recent `ConntrackFlow` set → aggregate by `DstAddr` (target prefix, IPv6-capable), by `SrcAddr` (top sources, IPv6-capable), source-port histogram (reflection), proto mix, and TCP flags from sampled headers (SYN).
4. Heuristic classify → `AttackFamily` + narrowest `VectorTuple` (proto + discriminating dst/src port + flags); compute Shannon entropy of source addresses to annotate spoofed/distributed (logged; optional future field).
5. Emit populated `AttackDetected` on the EventBus → unchanged responders build surgical rules.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| detect ↔ engine ↔ traffic-usage | `DispatchCommand` "show traffic-usage" → JSON | [ ] |
| detect ↔ engine ↔ flow-export | `DispatchCommand` recent-flow RPC (NEW) → JSON | [ ] |
| detect → responders | EventBus `AttackDetected` (value types) | [ ] |
| flowexport ExportFlows → recent ring | in-process append (bounded) | [ ] |

### Integration Points
- `detect/detector.go` `emitDetected` - fill `Target`/`Family`/`TopSources`
- `detect/characterize.go` (NEW) - DispatchCommand queries + heuristic classifier
- `flowexport` - recent-flow ring + new query RPC + command YANG
- `ddos-detect` YANG - new characterization tuning leaves + doctor check

### Architectural Verification
- [ ] No bypassed layers (cross-plugin via DispatchCommand, not sibling import)
- [ ] No unintended coupling (sources are OptionalDependencies; detect runs without them)
- [ ] No duplicated functionality (reuse `ze-show:traffic-usage`; add one query RPC on flowexport)
- [ ] Zero-copy / value-type payloads preserved on the EventBus

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ze-show:traffic-usage` is reachable via `DispatchCommand` from `ddos-detect` at runtime | DispatchCommand routes by prefix to the RPC registry; trafficusage registers it (`show.go:15-17`) | characterization cannot read trafficusage | functional test: detect dispatches and parses a response | confirmed (Phase 1: `Plugin.DispatchCommand` at `pkg/plugin/sdk/sdk_engine.go:149`; live precedent bgp-gr->bgp-rib `gr/gr.go:591`; detector now injected with `p.DispatchCommand` in `register.go`) |
| A-2 | During a volumetric flood the attacker's cumulative byte share dominates trafficusage's IPv4 map enough to rank as top sources without per-window deltas | floods dwarf baseline; counts are cumulative (`monitor.go:32`) | top-sources noisy/wrong | inject test with known attacker IPs, assert ranking | confirmed -- `TestTopSourcesRanking` (packet-volume ranking, tie-break) + `TestCharacterizeEmitsCharacterized` (TopSources populated end-to-end) in `detect/characterize_test.go` |
| A-3 | Conntrack periodic-dump cadence fills a recent-flow ring with attack flows within the confirm window | learned 819 (periodic dump); confirm-duration default 3 | ring empty at characterize time → fall back to trafficusage-only | timing test against active-timeout | confirmed (mechanism): ring append + bounded snapshot unit-tested (`recent_test.go`); graceful empty-ring fallback tested (`TestCharacterizeSkipsWhenNoFlowSource`). Runtime fill-timing is QEMU-only (`ddos-flow-recent.ci`, needs-linux) |
| A-4 | Responders act once `DstPrefix` is valid, with no other change except local `TCPFlags` | `shouldMitigate`/`shouldAnnounce` gate only on prefix validity (`match.go`) | mitigation still inert | existing responder tests + new functional test | confirmed (`local/match.go:44-46` and `flowspec/match.go:29-31` return false ONLY on invalid prefix; `local/responder.go:52-91` installs the nft drop on a valid-prefix enforce-mode event) |
| A-5 | Operators enabling characterization also enable traffic-usage (track-ip) and/or flow-export (conntrack) | deployment guidance | characterization degrades to generic-flood (= today) | doctor check + documented requirement | confirmed -- `doctor-ddos-detect-no-flow-source` warns when characterization is on with neither source (`detect/doctor.go`, `TestCheckFlowSourceWarnsWhenNoSource`); documented in the ddos guide "Flow source and observability" |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Synchronous DispatchCommand on the attack path adds latency under load | characterization slow in load test | bounded timeout; run off the `onRate` hot path in a goroutine; fall back to generic-flood on timeout; steady-state unaffected (trigger-only) |
| R-2 | flowexport recent-flow ring grows unbounded under flood | memory growth | fixed-size ring, drop-oldest, bounded N (config leaf) |
| R-3 | trafficusage byte-based maps misclassify packet-heavy low-byte SYN floods | SYN flood labeled generic | use flowexport packet counts + TCP flags for SYN; trafficusage is the coarse fallback |
| R-4 | Wrong target prefix → collateral drop or missed attack | collateral or no mitigation effect | choose prefix by dominant dst; default host /32 (v4) /128 (v6); responder allowlist guard already present |
| R-5 | IPv6 attacks invisible to trafficusage (IPv4-only map) | IPv6 flood → empty top sources | flowexport tap covers IPv6; document trafficusage-only as IPv4-only |
| R-6 | Characterization races with detector state under `d.mu` | data race in `-race` tests | snapshot needed state under lock; run classifier outside the mutex |

## Key Design Decisions

| Decision | Alternatives considered | Rationale |
|----------|------------------------|-----------|
| Two-signal producer: emit a fast "victim identified" event, then a "characterized" event | Single characterized event (slow); single coarse event (never precise) | Local mitigation can act on the fast signal and refine live; upstream needs the characterized signal. One event cannot serve both. |
| Local responder: act on the fast signal, then narrow in place | Wait for characterization before any local rule | A local drop still lets the box observe the attack (packets arrive, are dropped on the way in), so refining-while-blocking is free. Fastest protection. |
| Upstream (flowspec) responder: act ONLY on the characterized signal | Announce a rough rule then refine | Once the upstream filters, the attack stops reaching us and we go blind (`flowspec/probe.go` leak-probe exists precisely for this). A rough rule cannot be refined afterward. Get it right once. |
| Local filter is the upstream stop-gap: hold the local rough drop until the precise flowspec rule is sent | Immediate upstream blackhole always | The local filter protects AND preserves visibility for building the upstream rule (user decision). Blackhole-now is the fallback when local filtering is unavailable/unsustainable. |
| New typed event `AttackCharacterized`; `AttackDetected` becomes the fast "victim identified" signal | Add a `Characterized bool` flag to `AttackDetected` | Distinct typed events match the EventBus pattern (`events.Register`) and let each responder subscribe to exactly what it acts on. `AttackDetected` is currently emitted empty (`detector.go:142-148`), so re-purposing it is non-breaking. |
| Cross-plugin data via `DispatchCommand` to `traffic-usage` + a NEW flowexport recent-flow RPC | Direct calls / DirectBridge | Plugins cannot import siblings (`plugin-design.md:133`); DirectBridge is engine↔plugin only. |
| Data sources are `OptionalDependencies` with fallback to generic | Hard `Dependencies` | Detector must still protect (volumetric + coarse target) when sources are off; degrade to current behavior, never worse. |
| Characterization runs off the `onRate` hot path (goroutine, bounded timeout) | Inline under `d.mu` | Keeps steady-state cost ~zero (learned 1011) and avoids holding the detector mutex / blocking the tick (R-1, R-6). |
| Graded `Severity` from `PeakRxPps / Threshold()` on the fast event (Refinement 3, NetHawk-derived) | No severity (peaks only); or a YANG-tuned threshold set | The detector already holds the ratio at emit time; a derived grade is free and lets blackhole-fallback (AC-8) escalate at `critical` while staying surgical below. Reuses NetHawk's 1x/2x/5x tiering; no wire/schema change. |

### Triple Challenge
- **Simplicity:** Phase 1 (fill `DstPrefix` from trafficusage, local acts) is the minimum that unblocks mitigation; later phases add precision. Each phase ships value alone.
- **Uniformity:** cross-plugin via `DispatchCommand`; new RPC mirrors `ze-show:traffic-usage`; new event via `events.Register`; doctor via `Registration.DoctorChecks`. No new mechanisms.
- **Performance:** steady-state unchanged (trigger-only characterization); recent-flow ring is fixed-size; EventBus payloads are value types.

## External Reference: NetHawk Classifier (research capture, 2026-06-28)

Source: `https://github.com/Flowtriq/nethawk` (Flowtriq/nethawk), `internal/capture/capture.go`,
`internal/ui/app.go`. NetHawk is a single-binary libpcap TUI that performs exactly the
proto/port-signature classification this spec's Stage-3 (`characterize.go`, Phase 3) proposes.
It is a working reference for the heuristic-not-ML scope decision (line 36) and supplies
field-tested constants. The following NetHawk traits are deliberately NOT adopted:
- Static absolute PPS threshold. Our adaptive P99 x multiplier vs floor is stronger
  (`internal/plugins/ddos/detect/baseline.go:60-62`); do not regress to a fixed number.
- "SYN flood" inferred from TCP-share alone (no flag inspection). R-3 already mandates
  flowexport TCP flags; keep that. NetHawk's heuristic mislabels legitimate TCP bursts.
- libpcap per-packet capture. Does not fit the forwarding-plane target; our counter trigger
  (`detector.go:63`) + on-trigger flow query stays at ~zero steady-state cost.

### Refinement 1 -- Reflection-port seed table (refines Phase 3, AC-3)

Phase 3's reflection classifier (currently "known reflection ports (DNS 53, NTP 123, etc.)",
AC-3) gets a concrete seed constant in `characterize.go`. NetHawk's full set, plus common
omissions to extend with:

| Port | Proto | Service (amplifier) | Source |
|------|-------|---------------------|--------|
| 53 | UDP | DNS | NetHawk |
| 123 | UDP | NTP | NetHawk |
| 161 | UDP | SNMP | NetHawk |
| 389 | UDP | CLDAP/LDAP | NetHawk |
| 1900 | UDP | SSDP | NetHawk |
| 11211 | UDP | Memcached | NetHawk |
| 19 | UDP | CharGEN | NetHawk |
| 111 | UDP | RPC/portmap | extend |
| 137 | UDP | NetBIOS-NS | extend |
| 5353 | UDP | mDNS | extend |
| 3702 | UDP | WS-Discovery | extend |
| 1434 | UDP | MS-SQL resolution | extend |
| 520 | UDP | RIPv1 | extend |

Match rule: `Proto==17 (UDP)` AND the attack's dominant **source** port is in this table
=> `FamilyReflection`, `VectorTuple.SrcPort` set. Source port comes from flowexport; trafficusage
does not carry it (R-5 / Known Limitations). The table lives as one constant in `characterize.go`,
not spread across packages (plugin self-containment).

### Refinement 2 -- Protocol-dominance default thresholds (refines Phase 3, AC-3..AC-6)

Phase 3 says "dominant proto" without numbers. NetHawk's field-tested cut-offs become the
default constants for the classifier (tunable later; NOT new YANG leaves in this pass):

| Constant | Default | Used by |
|----------|---------|---------|
| `udpDominantPct` | 80 | UDP-flood / reflection gate (AC-3, AC-6) |
| `tcpDominantPct` | 80 | SYN-flood gate, combined with TCP-flags (AC-4) |
| `icmpDominantPct` | 50 | ICMP-flood gate (AC-5) |
| `topPortPct` | 50 | a single dst/src port dominates -> set it in VectorTuple |

These are heuristic seeds, not protocol constants; they belong in `characterize.go` beside the
classifier. SYN-flood (AC-4) requires BOTH `tcpDominantPct` AND a SYN-flag majority from
flowexport, never TCP-share alone (the NetHawk weakness above, R-3).

### Refinement 3 -- Graded Severity on the event contract (NEW design element)

NetHawk grades intensity into normal/medium/high/critical at 1x/2x/5x its threshold. Our event
contract has `PeakRxPps`/`PeakRxBps` (`internal/core/ddosevent/event.go:43-44`) but no graded
severity. Add a `Severity` field, graded from the ratio the detector already holds at emit time
(`peakRxPps` vs `baseline.Threshold()`):

| Ratio `PeakRxPps / Threshold()` | Severity |
|---------------------------------|----------|
| < 1 (sub-threshold; not emitted) | (n/a) |
| >= 1 and < 2 | `medium` |
| >= 2 and < 5 | `high` |
| >= 5 | `critical` |

- Contract: add `type Severity string` + consts and a `Severity Severity` field on
  `AttackDetected` (and on `AttackCharacterized` when that event is added in Phase 3). Value
  type, JSON-tagged, no schema/wire change. Extends `event.go` (already in Files to Modify);
  graded in `emitDetected` (`detector.go:140-152`, already listed).
- Purpose: severity gates responder aggressiveness. It lets the blackhole-fallback policy
  (AC-8) auto-engage at `critical` without waiting for characterization, while `medium` stays
  surgical, the surgical-vs-blackhole choice Control Flow step 7 already contemplates.
- Tests: `TestSeverityGrading` (`detect/detector_test.go`, boundary table at 1x/2x/5x);
  responder gating in `TestBlackholeFallbackOnCritical` (`flowspec/responder_test.go`).

## Control Flow (unified)

| Step | Actor | Action |
|------|-------|--------|
| 1 | detect `onRate` | SM → active; snapshot `attackIface`/peaks under `d.mu`; kick characterization goroutine |
| 2 | detect (goroutine) | fast target query (`show traffic-usage name <iface>`); emit `AttackDetected{Target.DstPrefix, Family=generic}` (replaces the empty emit at `detector.go:142-148`) |
| 3 | ddos-local | on `AttackDetected` → install coarse drop (all to victim); box still observes traffic |
| 4 | detect (goroutine) | characterize from flowexport recent-flow tap (proto/ports/flags/top-sources + entropy); emit `AttackCharacterized{full vector}` |
| 5 | ddos-local | on `AttackCharacterized` → narrow rule in place (re-register tables) |
| 6 | ddos-flowspec | on `AttackCharacterized` → announce ONE precise rule; then blind, leak-probe decides ongoing/withdraw |
| 7 | ddos-flowspec (fallback) | if local filtering unavailable/unsustainable → immediate blackhole on `AttackDetected` (policy) |
| 8 | clear | SM clear → `AttackCleared` → local removes rule; flowspec leak-probe withdraws |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| rate trigger (SM active) | → | `detect.characterize` emits `AttackDetected` with valid `DstPrefix` | `TestDetectorEmitsTargetOnTrigger` (detect) |
| `AttackDetected` on bus | → | `local.onDetected` installs coarse drop | `TestLocalRespondsToFastSignal` (local) |
| `AttackCharacterized` on bus | → | `local.onCharacterized` narrows; `flowspec.onCharacterized` announces | `TestRespondersActOnCharacterized` (local, flowspec) |
| `DispatchCommand` recent-flow | → | flowexport recent-flow RPC handler | `test/plugin/ddos-flow-recent.ci` |
| end-to-end trigger→local drop | → | full chain | `test/plugin/ddos-detect-mitigate.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Rate trigger fires; trafficusage reports a dominant destination | Detector emits `AttackDetected` with a valid `DstPrefix` (no longer empty) |
| AC-2 | `AttackDetected` with valid `DstPrefix`, local in enforce mode | ddos-local installs a drop rule for that prefix |
| AC-3 | Recent flow set dominated by UDP from a known reflection source port | `AttackCharacterized.Family == reflection`, `VectorTuple` sets Proto=17 + SrcPort |
| AC-4 | Recent flow set dominated by TCP SYN packets | `Family == syn-flood`, `VectorTuple.TCPFlags` has SYN set |
| AC-5 | Recent flow set dominated by ICMP | `Family == icmp-flood`, Proto=1 |
| AC-6 | No reflection/SYN/ICMP signature | `Family == generic-flood` with best-effort target |
| AC-7 | `AttackCharacterized` received | ddos-local narrows its rule to match Proto/port/flags; ddos-flowspec announces exactly one rule |
| AC-8 | `AttackDetected` received by flowspec (no characterization yet) | ddos-flowspec does NOT announce (unless blackhole-fallback policy enabled) |
| AC-9 | `VectorTuple.TCPFlags != 0` | `local.buildDropTerm` emits a TCP-flags match (closes the gap at `local/match.go:19-42`) |
| AC-10 | Neither data source configured/reachable (`ErrUnknownCommand`) | Detector emits best-effort `AttackDetected` and logs once; no crash; mitigation no worse than today |
| AC-11 | flowexport recent-flow RPC queried for an interface | Returns recent `ConntrackFlow` records bounded to the configured ring size |
| AC-12 | Top source IPs present in flow data | `AttackDetected/Characterized.TopSources` populated (IPv4 from trafficusage, IPv4+IPv6 from flowexport) |
| AC-13 | Rate trigger fires with peak >= 5x threshold | `AttackDetected.Severity == "critical"` (graded from `PeakRxPps / baseline.Threshold()`); >=1x medium, >=2x high, >=5x critical (Refinement 3) |
| AC-14 | `Severity == "critical"` and blackhole-fallback policy enabled | ddos-flowspec may auto-engage blackhole on the fast `AttackDetected` without waiting for characterization (severity-gated escalation of AC-8) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | suffers a UDP reflection flood; ze auto-mitigates locally then upstream | trigger → fast target → local drop → characterize (reflection) → narrow local → flowspec announce | `test/plugin/ddos-detect-mitigate.ci` |
| 2 | runs `show traffic-usage` / flow-recent to see what drove the decision | DispatchCommand → trafficusage / flowexport RPC | `ddos-flow-recent.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClassifyReflection/Syn/Icmp/Frag/Generic` | `detect/characterize_test.go` | family heuristics (table-driven) | |
| `TestNarrowestVectorTuple` | `detect/characterize_test.go` | tuple built from flow aggregates | |
| `TestTopSourcesRanking` | `detect/characterize_test.go` | top-N by volume, A-2 ranking | |
| `TestSourceEntropy` | `detect/characterize_test.go` | distributed vs concentrated annotation | |
| `TestFallbackOnSourceAbsent` | `detect/characterize_test.go` | ErrUnknownCommand → generic, log once | |
| `TestLocalNarrowsInPlace` | `local/responder_test.go` | re-register on AttackCharacterized | |
| `TestLocalTCPFlagsMatch` | `local/match_test.go` | flags match in buildDropTerm | |
| `TestFlowspecWaitsForCharacterized` | `flowspec/responder_test.go` | no announce on AttackDetected | |
| `TestRecentFlowRingBounded` | `flowexport/recent_test.go` | fixed-size, drop-oldest | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| top-n-sources | 1..100 | 100 | 0 | 101 |
| characterize-window (s) | 1..60 | 60 | 0 | 61 |
| characterize-timeout (ms) | 50..5000 | 5000 | 49 | 5001 |
| recent-flow-ring | 64..65536 | 65536 | 63 | 65537 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-detect-mitigate` | `test/plugin/*.ci` | trigger → local drop → narrowed | |
| `ddos-flow-recent` | `test/plugin/*.ci` | recent-flow RPC returns flows | |

### Interop Tests
N/A: no wire-protocol change (flowspec NLRI already exists; this spec only decides WHEN to announce).

## Files to Modify
- `internal/plugins/ddos/detect/detector.go` - split emit; snapshot + kick characterization goroutine
- `internal/plugins/ddos/detect/config.go` + `detect/yang/ze-ddos-detect-conf.yang` - characterize enable, top-n, window, timeout leaves
- `internal/core/ddosevent/event.go` - add `AttackCharacterized` event; document `AttackDetected` as the fast signal
- `internal/plugins/ddos/local/responder.go` + `register.go` + `match.go` - subscribe AttackCharacterized; narrow in place; TCP-flags match
- `internal/plugins/ddos/flowspec/responder.go` + `register.go` + `config.go` - act on AttackCharacterized; optional blackhole-fallback policy
- `internal/plugins/flowexport/exporter.go` - tap `ExportFlows` (`exporter.go:230`) into a bounded recent-flow ring
- `internal/plugins/flowexport/register.go` + new `cmd/` RPC + `yang/` - recent-flow query RPC + command YANG + ring-size config
- `internal/core/diagnostic/codes.go` - `doctor-ddos-detect-no-flow-source`
- `docs/features.md`, `docs/guide/ddos-mitigation.md`, `ai/INDEX.md` - discovery

## Files to Create
- `internal/plugins/ddos/detect/characterize.go` (+ `_test.go`) - queries + heuristic classifier + entropy
- `internal/plugins/flowexport/recent.go` (+ `_test.go`) - bounded recent-flow ring
- `internal/plugins/flowexport/cmd/` recent-flow RPC handler + `yang/`
- `test/plugin/ddos-detect-mitigate.ci`, `test/plugin/ddos-flow-recent.ci`

## Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (detect tuning + flowexport ring + recent RPC) | Yes | `ddos/detect/yang/`, `flowexport/yang/` |
| YANG validation constraints | Yes | range on every new leaf (see Boundary Tests) |
| CLI commands | Yes | flowexport recent-flow RPC (`ze-show:flow-recent` or `request`-style) |
| Functional test for new RPC | Yes | `test/plugin/ddos-flow-recent.ci` |
| Env var registration | N/A | operational policy → YANG only |
| Doctor check for runtime dependency | Yes | `ddos-detect` DoctorChecks + `doctor-ddos-detect-no-flow-source` |
| Prometheus counters | Yes | characterization outcomes (per family), fallback count, recent-ring drops |

## Implementation Phases (deliverable independently)

1. **Phase 1 — Unblock (smallest, highest value):** detect queries trafficusage on trigger, fills `DstPrefix`, emits populated `AttackDetected`; ddos-local already acts. Mitigation goes from inert to working. Tests: AC-1, AC-2, AC-10.
2. **Phase 2 — flowexport tap:** bounded recent-flow ring + recent-flow RPC + YANG. Tests: AC-11, ring boundary.
3. **Phase 3 — Classifier:** `characterize.go` family/tuple/top-sources/entropy from flow data; emit `AttackCharacterized`. Tests: AC-3..AC-6, AC-12.
4. **Phase 4 — Responder split:** local narrows in place + TCP flags; flowspec acts on characterized only. Tests: AC-7, AC-8, AC-9.
5. **Phase 5 — Upstream stop-gap + ops:** local-stop-gap/blackhole-fallback policy, doctor check, metrics, docs, deployment guidance.

## Known Limitations
- trafficusage path is IPv4-only and byte-based; IPv6 and packet/flag detail come only from the flowexport tap.
- Low-rate / application-layer attacks are out of scope (volumetric vectors only).
- Hardware-offload detection for the local stop-gap is modeled as policy/config, not auto-probed.

## Critical Review Checklist

| # | What to verify | How |
|---|----------------|-----|
| CR-1 | Characterization runs OFF the `onRate` hot path | `onAttackStart` snapshots under `d.mu` then spawns a goroutine; no `DispatchCommand` call holds `d.mu` (`detector.go`, `characterize.go`) |
| CR-2 | Baseline poisoning guard not regressed | threshold still computed before `baseline.Add`; `Add` still excludes attacking/above samples (`detector.go:111-116`, `baseline.go:28-31`) |
| CR-3 | Steady-state cost unchanged | no per-IP work on the tick; the only added work is one goroutine per idle->active transition |
| CR-4 | Graceful fallback never worse than today | nil/absent/error/empty source -> empty target + `FamilyGenericFlood`; AttackDetected still emitted |
| CR-5 | No sibling-plugin import | detect reaches trafficusage only via `DispatchCommand`; no `internal/plugins/trafficusage` import in detect |
| CR-6 | Data race free | `-race` unit run green; goroutine reads only snapshot args + immutable `d.bus`/`d.dispatch`; tests sync via `d.wg` |
| CR-7 | Source-absent logged once | `sync.Once` guards the WARN (no per-attack log spam) |

## Deliverables Checklist

| Deliverable | Verification method | Evidence (Phase 1, 2026-06-28) |
|-------------|--------------------|----------|
| `characterize.go` (parse + query + emit) | `ls internal/plugins/ddos/detect/characterize.go` | present (created) |
| Dispatch injected into detector | `grep -n 'p.DispatchCommand' internal/plugins/ddos/detect/register.go` | both newDetector sites pass `p.DispatchCommand` |
| AC-1 target filled | `go test -race -run TestDetectorEmitsTargetOnTrigger` | PASS (target = 203.0.113.42/32) |
| AC-10 fallback | `go test -race -run TestDetectorFallbackWhenSourceAbsent` | PASS (empty target + generic-flood on ErrUnknownCommand) |
| Parse heuristic | `go test -race -run TestParseTopDestination` | PASS (9 subcases) |
| AC-2 local acts on valid prefix (pre-existing) | `go test ./internal/plugins/ddos/local/...` | PASS (`TestEnforceModeActivates`, responder_test.go:41) |
| No regression in existing detector tests | `go test -race ./internal/plugins/ddos/detect/...` | PASS |
| Lint gate | `make ze-lint-changed` | exit 0 |
| Functional `.ci` parses + gated | `ze-test bgp plugin --list` / `132` | discovered #132/447; SKIP on darwin (needs-linux); runtime verified only under QEMU |

## Security Review Checklist

| # | Concern | Check |
|---|---------|-------|
| SR-1 | Untrusted JSON from the data source | `parseTopDestination` reads only expected fields, validates with `netip.ParseAddr`, returns not-ok on malformed input; no panic |
| SR-2 | Resource exhaustion on the attack path | bounded `context.WithTimeout`; one goroutine per attack transition; no unbounded allocation from the response |
| SR-3 | DoS via the dispatch round-trip under flood | dispatch is off the tick path and time-bounded; a hung source cannot stall detection or the mutex |
| SR-4 | Target-prefix correctness (collateral drop) | dominant-destination host prefix only; responder allowlist guard (`shouldMitigate`) still applies downstream |
| SR-5 | Command injection into the dispatched string | iface name comes from kernel iface stats, passed as fixed `show traffic-usage name <iface>`; no shell; engine tokenises internally |

## Documentation Update Checklist

| Category | Update needed? | File / action |
|----------|---------------|---------------|
| Feature list | No | behaviour change to an existing opt-in feature; no new top-level feature row |
| User guide | Yes | the ddos mitigation guide: with `traffic-usage` (track-ip) enabled, mitigation now targets the attacked destination; without it behaviour is unchanged |
| Config syntax | No | no new YANG leaf in Phase 1 (constant timeout) |
| CLI reference | No | no new command |
| API / RPC docs | No | consumes the existing `ze-show:traffic-usage` RPC; no new RPC in Phase 1 |
| Plugin SDK | No | uses existing `Plugin.DispatchCommand` |
| Wire format | No | event payload is value types; no wire/schema change |
| Comparison table | No | n/a |
| Test infrastructure | Raised with user | engine-routed end-to-end `.ci` needs eBPF/QEMU; Phase 1 is covered by unit tests with injected dispatch + existing local responder tests (see Deviations) |
| Architecture design | No | `// Design:` annotation points at this spec / learned summary |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Files to Create / TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify-changed` |
| 14. Present summary | Executive Summary + learned summary roll-up |

### Implementation Phases (deliverable independently)
1. **Phase 1 — Unblock:** detect queries trafficusage on trigger, fills `DstPrefix`, emits populated `AttackDetected`; ddos-local acts. Tests: AC-1, AC-2, AC-10.
2. **Phase 2 — flowexport tap:** bounded recent-flow ring + recent-flow RPC + YANG. Tests: AC-11, ring boundary.
3. **Phase 3 — Classifier:** `characterize.go` family/tuple/top-sources/entropy; emit `AttackCharacterized`. Tests: AC-3..AC-6, AC-12.
4. **Phase 4 — Responder split:** local narrows in place + TCP flags; flowspec acts on characterized only. Tests: AC-7, AC-8, AC-9.
5. **Phase 5 — Upstream stop-gap + ops:** local-stop-gap/blackhole-fallback policy, doctor check, metrics, docs.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Stage-2 used DirectBridge to trafficusage/flowexport (per learned 1011) | Plugins cannot call siblings; the path is `DispatchCommand` via the engine (`plugin-design.md:133`) | reading plugin-design.md + sdk_engine.go | corrected design seam before writing code |
| The `flow-recent` RPC could filter by interface (`name <iface>`), per Data Flow / Control Flow | `ConntrackFlow` (`flowexport/flowtypes.go`) carries no ingress interface; conntrack is host-global | reading the conntrack reader + flow type during Phase 2 audit | RPC redesigned to `dst <prefix>` (D-9); the detector already holds the victim prefix, so this is strictly better |
| flowexport supplies TCP header flags "from sampled headers" for SYN detection | `ConntrackFlow` has no flags; the sampling path retains nothing on-box. But netlink exposes conntrack `ProtoInfoTCP.State` | reading `conntrack_linux.go` in the netlink lib | SYN-flood detected from half-open TCP *state* instead (D-11 context); added a `TCPState` field end-to-end |
| Fragment-flood is detectable as a volumetric vector | Conntrack runs after IP defrag; fragments never appear as flows | reading the conntrack pipeline | fragment classifies as generic (D-10, user-approved); enum kept for a future sampling classifier |

## Deviations (Phase 1)

| # | Deviation | Reason | In scope for |
|---|-----------|--------|--------------|
| D-1 | Phase 1 fills `Target.DstPrefix` only (IPv4, from trafficusage); `Proto`/`DstPort`/`SrcPort`/`TCPFlags`/`Family` stay generic | Phase 1 = "Unblock"; classifier needs the flowexport tap | Phase 3 (classifier) |
| D-2 | `characterizeTimeout` is a 2s constant, not a YANG leaf | keeps Phase 1 free of YANG/`make generate` churn; tuning leaves staged with later phases (user scoped this to Phase 1) | the phase that adds tuning leaves |
| D-3 | `AttackCharacterized` event + graded `Severity` field (NetHawk Refinement 3) not implemented | later-phase work; Phase 1 re-uses the empty `AttackDetected` emit point | Phase 3 / Refinement 3 |
| D-4 | `test/plugin/ddos-detect-mitigate.ci` authored, parses, and SKIPs on darwin, but its runtime trigger->drop was NOT executed on the dev host | eBPF + live engine are Linux/QEMU-only; authored per user instruction, ships for QEMU CI to run | QEMU CI (`make ze-qemu-all-test`) |
| D-5 | IPv6 targets not covered | trafficusage IPv4-only; IPv6 via the flowexport tap | Phase 2/3 |
| D-6 | No `ze doctor` check / `doctor-ddos-detect-no-flow-source` for the soft trafficusage dependency (Run-2 ISSUE-2) | soft dependency with graceful fallback; daemon fully functional without it; spec assigns doctor work to Phase 5 | Phase 5 (ops) |
| D-7 | Prometheus counters (characterization outcomes per family, fallback count) not added | spec assigns metrics to Phase 5 | Phase 5 (ops) |
| D-8 | Characterization goroutine not awaited at shutdown; 2s dispatch ctx not tied to plugin lifecycle (Run-2 NOTE-3) | bounded (<=2s, one per attack), RPC fails gracefully after close | a later phase |

### Deviations resolved / added (Phases 2-5)

| Phase-1 deviation | Resolution in Phases 2-5 |
|-------------------|--------------------------|
| D-1 (target only, no proto/ports/flags/family) | RESOLVED -- classifier fills family + narrowest `VectorTuple` (`characterize.go:classifyFlows`) |
| D-2 (constant timeout, no tuning leaves) | RESOLVED -- 5 tuning leaves added (`characterize-enable/-window/-timeout`, `top-n-sources`, `entropy-threshold`) |
| D-3 (no `AttackCharacterized`/`Severity`) | RESOLVED -- both added to `ddosevent`; `GradeSeverity` grades 1x/2x/5x |
| D-5 (IPv6 not covered) | RESOLVED -- recent-flow ring is v4+v6; victim derived from flows when trafficusage (v4-only) gives none |
| D-6 (no doctor check) | RESOLVED -- `doctor-ddos-detect-no-flow-source` + `checkFlowSource` |
| D-7 (no Prometheus counters) | RESOLVED -- per-family + fallback counters; `ze_flowexport_recent_ring_drops` |
| D-8 (goroutine not awaited, ctx not lifecycle-bound) | RESOLVED -- `detector.Stop()` cancels `d.ctx` + waits `d.wg`, called at shutdown and reconfigure |
| D-4 (`ddos-detect-mitigate.ci` runtime unverified on darwin) | STILL OPEN -- QEMU-only; joined by `ddos-flow-recent.ci` (both `needs-linux`, verified to parse + SKIP on darwin) |

| New # | Deviation | Reason |
|-------|-----------|--------|
| D-9 | `flow-recent` RPC filters by **destination prefix** (`dst <prefix>`), not the spec's literal `name <iface>` | `ConntrackFlow` carries no ingress interface (conntrack is host-global); dst-prefix is what characterization needs and is honest to the data (Mistake Log below) |
| D-10 | Fragment-flood classifies as **generic** (enum kept, not emitted) | No on-box conntrack signal (defrag precedes conntrack); no AC requires it (AC-6 routes to generic); user approved "accept as generic, defer sampling tap" (2026-07-01) |
| D-11 | `flow-recent` runtime is Linux/QEMU-only | flowexport hard-depends on the `interface` plugin (Linux netlink backend); the daemon cannot start off-Linux, so the `.ci` is `needs-linux`. RPC wiring + ring mechanics are unit-tested on any platform |

## Review Gate

### Run 1 (self-review, 2026-06-28, Phase 1)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| CR-1..CR-7 | NOTE | Characterization runs off the hot path (`onAttackStart` snapshots under `d.mu`, spawns `wg.Go`); poisoning guard intact (`detector.go:111-116`); fallback emits generic; no sibling import; `-race` clean; source-absent logged once via `sync.Once` | `detector.go`, `characterize.go` | none (all checks pass) |
| SR-1..SR-5 | NOTE | Untrusted JSON parsed defensively (returns not-ok, no panic); bounded timeout + one goroutine; iface name (not user free-text) into a fixed command; downstream allowlist guard intact | `characterize.go` | none |
| 1 | NOTE | `.ci` runtime path unverifiable on darwin | `test/plugin/ddos-detect-mitigate.ci` | recorded as D-4; QEMU CI verifies |

### Run 2 (/ze-review, 2026-06-28, Phase 1)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Detected emit became async; the first synchronous Ongoing could reach subscribers before Detected under a slow flow query | `detector.go`, `characterize.go` | FIXED: gate Ongoing on a `detectedEmitted` atomic set after the Detected emit, reset on clear; regression test `TestOngoingGatedUntilDetected` (-race) |
| 2 | ISSUE | No `ze doctor` check / `doctor-ddos-detect-no-flow-source` for the soft trafficusage dependency | (none) | DEFERRED to Phase 5 (D-6); soft dependency with graceful fallback, daemon fully functional without it |
| 3 | NOTE | characterization goroutine not awaited at shutdown (<=2s window, RPC fails gracefully) | `detect/register.go` | DEFERRED (D-8) |
| 4 | NOTE | Prometheus counters not added in Phase 1 | (none) | DEFERRED to Phase 5 (D-7) |
| 5 | NOTE | functional `.ci` runtime is QEMU-only, unverified on darwin | `test/plugin/ddos-detect-mitigate.ci` | recorded D-4 |
| 6 | NOTE | pre-existing stale source anchor (NOT in this diff) leaves `ze-validate` red | `docs/integrations/flowtriq-api.md:3` | out of scope; flagged for separate fix |

### Run 3 (/ze-review, 2026-06-28, fresh pass on the ISSUE-1 fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | Async Detected could fire after Cleared (slow query + low `clear-consecutive`), installing a local drop with no matching Cleared; `max-mitigation-duration` is parsed but NOT enforced in ddos-local, so no backstop | `characterize.go`, `detector.go`; `local/config.go`, `local/responder.go:98` | FIXED: `attackGen` generation guard bumped on activate + clear; `characterizeAndEmit` drops the emit when the generation advanced; regression test `TestNoStaleDetectedAfterClear` (-race) |
| 2 | NOTE | `max-mitigation-duration` is config-only in ddos-local (no timer enforces it); pre-existing, out of this diff | `local/config.go:19` | flagged for a separate fix; the generation guard removes this path's dependency on it |

### Run 4 (adversarial review, 2026-07-01, Phases 2-5)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | The Phase-1 generation guard was TOCTOU: `attackStale(gen)` released `d.mu` before the `Emit`, so a concurrent `emitCleared` (runs its Cleared emit under `d.mu` on the rate tick) could slip a Cleared between the check and the emit. `AttackCharacterized`/`AttackDetected` could then land after `AttackCleared`, leaving a permanent ddos-local drop (no backstop). | `characterize.go`, `detector.go` | FIXED: `emitDetected`/`emitCharacterized` hold `d.mu` across the generation check AND the (synchronous) emit, serializing with `emitCleared`. Regression tests `TestNoStaleCharacterizedAfterClear`, `TestNoStaleDetectedAfterClear` (-race, -count=5). |
| 2 | ISSUE | `Stop()`'s `wg.Wait()` could race `wg.Go` in `onAttackStart` (a late trafficstat tick can arrive after Unsubscribe because trafficstat invokes subscribers outside its lock) -> `panic: WaitGroup reused`. `OnConfigure` also stopped the old detector before unsubscribing. | `detector.go`, `register.go` | FIXED: `stopped` flag set under `d.mu` in `Stop()` fences `onAttackStart` (also under `d.mu`) from spawning; `OnConfigure` now unsubscribes before `Stop`. Regression test `TestStopFencesCharacterization`. |
| 3 | NOTE | ddos-local narrow-apply failure left the registry empty while the kernel kept the coarse rule and `active=true` (self-heals on next Cleared). | `local/responder.go` | FIXED: error path rolls the registry back, reconciles the kernel best-effort, and sets `active=false`. |
| 4 | NOTE | `ddos-flow-recent.ci` runtime is QEMU-only (flowexport hard-depends on the Linux `interface` backend). | `test/plugin/ddos-flow-recent.ci` | recorded D-11; RPC + ring unit-tested on any platform; parses + SKIPs on darwin. |
| Reviewed-clean | NOTE | Ring index/wrap math, lock ordering (`ExportFlows` e.mu->ring.mu vs RPC ring.mu-only), TCPState plumbing, classifier divide-by-zero/tie-breaks, `filterByWindow`, config default/range parity, flowspec single-announce -- all verified correct. | (multiple) | none |

### Run 5 (/ze-review, 2026-07-01, post-fix fresh pass)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | No functional test exercised the end-to-end characterized path (real flows -> AttackCharacterized -> ddos-local narrow); only units + the flow-recent RPC `.ci`. | `test/plugin/` | FIXED: added `test/plugin/ddos-detect-characterize.ci` (needs-linux) -- UDP reflection flood on a memcached source port, pre-warms the recent-flow ring, asserts ddos-local narrows to the source port. |
| 2 | NOTE | The ddos-local narrow-apply-failure rollback had no regression test. | `local/responder.go` | FIXED: `TestLocalApplyFailureRollsBack` injects an apply error and asserts active=false + registry rolled back to nil. |
| 3 | NOTE | ze-validate flags `RecentFlows`/`RecentDrops` "no cross-package caller". | `flowexport/exporter.go` | NO ACTION: pre-existing exporter.go convention (all Exporter methods are package-internal; `ExportFlows` is flagged identically and predates this work); both are wired via `cmd_show.go` / `conntrack_worker.go` and reachable from `show flow-recent` / the metrics path. |
| 4 | NOTE (design, from deep analysis) | Characterization is one-shot at confirm (`detector.go:onAttackStart`) and reads the recent-flow ring, which refreshes only at each conntrack dump (`conntrack_worker.go:run`, `active-timeout`); a long `active-timeout` leaves the ring stale for a just-started flood, so it degrades to the coarse drop. Intentional (A-3 + coarse fallback), but undocumented. | `characterize.go`, `conntrack_worker.go` | DOCUMENTED: ddos guide now tells operators to set a short `active-timeout` for responsive characterization. |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (Run 5: 0 BLOCKER, 0 ISSUE; the ISSUE and both NOTEs resolved with tests/docs; NOTE-3 is a pre-existing validator false positive; re-verified green -- lint 0, doc-test PASS, -race clean)
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
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
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-5-detect-5-characterization.md`
- [ ] Commit A: code + tests + docs + spec + learned summary
- [ ] Commit B: `git rm` spec
