# Spec: ddos-per-destination-detection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** ze's ddos detection is aggregate-only. `(*detector).applyTick`
(`internal/plugins/ddos/detect/detector.go`) thresholds exactly two scalars per
tick, `maxPps` and `maxBps`, each a maximum across interfaces, against one
`*baseline` and one `*baselineBps`. The `baseline` type
(`internal/plugins/ddos/detect/baseline.go`) holds a flat `samples []float64`
producing one p99 and one threshold. The only per-key state on the detector is
keyed by interface name, not by address.

**What that misses.** An attack against one customer prefix that is large for
that customer but small against the box's aggregate never crosses the threshold.
On a transit or edge router carrying tens of gigabits, a 200 Mbit/s flood that
completely saturates one downstream customer is invisible: it is noise in the
aggregate. This is the ordinary shape of the attacks an edge operator is asked
to mitigate, so aggregate-only detection understates ze's usefulness in exactly
the deployment it targets.

**What exists today, and why it is not this.** Per-source and per-destination
maps do exist, but strictly downstream of a trigger, for labelling only:
`rankTopSources`, `sourceEntropy` and `dominantDestination`
(`internal/plugins/ddos/detect/characterize.go`) run inside
`characterizeFromFlows`, after the state machine has already fired. They answer
"who is this attack against" once an attack is declared. They cannot declare
one.

The nearest per-entity machinery is a different plugin:
`internal/plugins/anomaly/detect` keeps per-entity EWMA baselines over
`trafficfeature.Snapshot`. Two properties bound its reuse. Its entity is a
SOURCE only, emitted when an address acted as a source in the window
(`internal/component/trafficfeature`), and its features are behavioural (fan-out,
out-in ratio, port entropy, beaconing) rather than packet and bit rates. Its own
header records that it takes no action. So it is a precedent for the shape, not
a component to extend directly.

**The goal.** Per-destination-prefix baselines and thresholds, so a victim
prefix under attack triggers while the aggregate looks normal. The spec must
settle four things:

1. **The entity, and where its rates come from.** The detector's current rates
   come from kernel interface counters (`internal/component/iface/rate.go`),
   which carry no address. A per-destination rate needs a different source: the
   traffic-usage (track-ip) path or the conntrack flow ring that
   `characterizeFromFlows` already queries. Which one, and at what cadence, is
   the first decision.
2. **Cardinality control.** One baseline per prefix is unbounded by
   construction. A cap, an eviction policy, and what happens at the cap have to
   be in the design, not discovered in review. The aggregate detector has no
   such exposure today, so this feature introduces the risk.
3. **Interaction with the aggregate path.** Two detectors that can each fire
   need a defined relationship: does a per-prefix trigger raise its own
   incident, or annotate the aggregate one? Does mitigation differ?
4. **The mitigation target.** A per-prefix trigger names its victim directly,
   which is strictly better information than `dominantDestination` inferred
   after the fact. The responders key on `Family`, `Target` and `Severity`, so
   the target field already exists to carry it.

**Why this is future work and not a defect.** Aggregate detection does what it
says and does it correctly. Nothing is wrong; a capability is missing.

**Owning gate.** `go test -race ./internal/plugins/ddos/detect`, then
`./le functional plugin`. A new `.ci` fixture is required: aggregate traffic held
flat while one destination prefix is flooded, asserting a detection that today
does not happen.
