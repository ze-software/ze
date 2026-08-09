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
<!-- source: internal/plugins/ddos/detect/detector.go -- onRate trigger order -->

ftagent adds the sample first. That order poisons the baseline on the exact tick
that should trigger: the spike raises p99 and the comparison then finds nothing
above it. Ze checks the threshold first, and `baseline.Add` excludes samples taken
while the interface is above the threshold or already attacking.

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

## Traps

- The detector's `onRate` callback receives CUMULATIVE packet counters, not
  packets per second. PPS is the delta between consecutive calls. A test that
  feeds the same counter value on every tick produces PPS 0 and never triggers.
- The firewall constants are not what a guess produces: `FamilyIP`, not
  `FamilyIPv4`; `ChainFilter`, not `ChainTypeFilter`; `HookInput`, not
  `ChainHookInput`.
- `sdk.ConfigSection.Data` is a JSON string, not a `map[string]any`. `ParseConfig`
  unmarshals from the string and then extracts the config-root key.
