# DDoS Detection and Auto-Mitigation: Architecture

Ze had mitigation levers (FlowSpec origination, firewall) and no automatic
decision of when to pull them. This subsystem ports the Flowtriq ftagent decision
logic onto ze primitives: a two-stage detector, a local nft responder, a
FlowSpec/RTBH upstream responder with a leak probe, and an incident store.

Related pages: [characterization and the responder split](cp-survival-5-detect-5-characterization.md),
[bandwidth trigger, baseline persistence and confidence](ddos-detect-enhancements.md).

## Packages

<!-- source: internal/core/ddosevent/event.go -- AttackDetected, AttackCharacterized, AttackFamily, VectorTuple -->

| Package | Role |
|---------|------|
| `internal/core/ddosevent` | the shared event contract, a core leaf |
| `internal/plugins/ddos/detect` | the two-stage detector |
| `internal/plugins/ddos/local` | the on-host nft responder |
| `internal/plugins/ddos/flowspec` | the FlowSpec and RTBH upstream responder |
| `internal/plugins/ddos/observe` | the incident store |
| `internal/plugins/ddos/flowtriq` | the Flowtriq report client |

The event contract sits in a core leaf so a responder imports the leaf and never
the detector. The plugin packages are independent and individually removable.

## Decisions

### EventBus broadcast, not DirectBridge

Detection is a broadcast to every interested responder, not a directed call to
one. The detector emits on the EventBus (1:N). DirectBridge is request and
response (1:1) and would make the detector name its consumers.

### Two stages, not continuous per-IP monitoring

Stage 1 is a cheap rate trigger: one rate comparison per interface per tick.
Stage 2 is pattern analysis, and it runs only after stage 1 triggers. Steady-state
cost is therefore near zero. Continuous per-IP monitoring pays the analysis cost
on every tick, whether or not anything is happening.

### The threshold is computed BEFORE the sample joins the baseline

<!-- source: internal/plugins/ddos/detect/baseline.go -- Add, Threshold -->
<!-- source: internal/plugins/ddos/detect/detector.go -- onRates trigger order -->

ftagent adds the sample first. That order poisons the baseline on the exact tick
that should trigger: the spike raises p99 and the comparison then finds nothing
above it. Ze checks the threshold first, and `baseline.Add` excludes samples taken
while the interface is above the threshold or already attacking.

The exclusion admits one sample per `slowAdaptSamples` consecutive above-threshold
samples, and any sample below the threshold restarts that run. Without a way back
the guard latches on a permanent rise in offered load, which is not an attack and
never ends. The damping factor is the whole separation between the two cases, so
`baseline.go` states the timescale it implies.

This was implemented in the wrong order once, and the symptom was a detector that
never triggered.

### The leak probe clears a FlowSpec mitigation

<!-- source: internal/plugins/ddos/flowspec/probe.go -- leak probe -->

Ze has no inbound flow collector, and an upstream drop blinds every local sensor.
A passive clear therefore has no signal to read. The responder instead lets a
bounded trickle through and measures it. That trickle is the only honest evidence
that the attack has stopped.

### The backend seams are replaceable

<!-- source: internal/plugins/ddos/local/responder.go -- registerTables, applyAll -->
<!-- source: internal/plugins/ddos/flowspec/responder.go -- routeDispatcher, newResponder -->

`firewall.ApplyAll()` returns an error when no backend is loaded, which is the
state of every unit test, so the responder cannot call it directly. The local
responder holds the firewall seam as package-level function variables
(`registerTables`, `applyAll`), matching the existing `getActiveConnector`
pattern. The flowspec responder takes a `routeDispatcher` at construction, so a
test drives announce and withdraw with no BGP session.

### A blind detector is a runtime doctor check over the rate feed

The detector's input is the per-interface receive rate (`trafficstat`
`SubscribeRates`; `trafficstat.EnsureGlobal` creates the service on first use,
so there is no second feed). `traffic-usage` and `flow-export` only characterize an attack
after detection, so the config-time `ddos-detect-flow-source` check cannot see
a dead rate feed. A feed with no traffic leaves `(*baseline).Ready` true, since
it counts samples and not their values, and the threshold at the floor, so the
detector looks armed and can never fire.

`blind.go` watches the feed under `d.mu`: the time of the last sample, the
length of the current all-zero run and the interface count of the last sample.
The window is `baseline-window` times `check-interval` feed samples, the moment
the baseline holds only zeros, so an idle box warns no earlier than its
baseline is blind. The check is declared in the SDK registration
(`ddos-detect-rate-feed`, code `doctor-ddos-detect-blind`) and answered by the
plugin's `OnDoctorCheck` callback, as OSPF's `ospf-state` is, because only the
running plugin holds the detector: a check in the daemon's registry runs in the
daemon process and sees no detector when the plugin runs external. An external
plugin's own trafficstat service ticks over an empty interface list, so its
samples would read as "no interface": `arm` records `p.IsInternal()`, and an
external detector reports that it runs external, from the moment it is armed,
with the config block to remove. The finding is a warning, the log marks each edge once, and nothing latches: the first
sample with traffic clears it.
<!-- source: internal/plugins/ddos/detect/blind.go -- feedWatch, observeFeed, rateFeedFinding -->
<!-- source: internal/plugins/ddos/detect/doctor.go -- checkRateFeed, rateFeedCheckDecl -->
<!-- source: internal/plugins/ddos/detect/register.go -- runEngine -->

## Traps

- The detector's input is `onRates`, which receives packets and bytes per
  second already computed by the trafficstat service, not cumulative counters.
  A test drives it with the rate it wants each tick to carry.
- The firewall constants are not what a guess produces: `FamilyIP`, not
  `FamilyIPv4`; `ChainFilter`, not `ChainTypeFilter`; `HookInput`, not
  `ChainHookInput`.
- `sdk.ConfigSection.Data` is a JSON string, not a `map[string]any`. `ParseConfig`
  unmarshals from the string and then extracts the config-root key.
