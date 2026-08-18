# Anomaly Observation: Incident Lifecycle Store

The history tier of the behavioral security chain. It subscribes to the same
`anomaly-detect` events described in [anomaly-detect](anomaly-1-detect.md) and keeps
the LIFECYCLE of each incident: when it started, and when it ended.

The detector already keeps a report ring, and that ring is the reason this plugin
exists rather than a duplicate of it.

## Why the detector's ring is not enough

<!-- source: internal/plugins/anomaly/detect/detector.go -- activate, recentIncidents, incidentRingSize -->

`detector.activate` appends the `AnomalyDetected` event itself to a 128-entry ring,
and `show anomaly detect` returns those events. The record is therefore a
CONFIRMATION: it carries the time the incident started and has no field for the time
it ended. An operator can see that a source misbehaved and cannot see for how long,
which is the question an incident report is written to answer.

This plugin holds a second record with both ends. Nothing is duplicated: the
detector owns scoring and the events, and this store owns only the open and close
bookkeeping of what those events already said.

## Three close paths, and only two are events

<!-- source: internal/plugins/anomaly/observe/store.go -- open, finalize, sweepStale -->
<!-- source: internal/plugins/anomaly/observe/register.go -- subscribeStore, startStaleSweep -->

`Detected` opens a record. `Cleared` finalizes it. `Ongoing` is subscribed and
ignored, because the store records episodes rather than samples: one incident that
lasts a minute is one row, not sixty.

The third close path is the one that matters, and it is not an event at all. The
detector clears an entity only while it keeps SEEING that entity: `onTick` counts
consecutive below-threshold windows for a source that is still in the feature
snapshot. A source that goes silent is absent from the snapshot instead, so its
state ages out through the idle counter and is evicted with NO `Cleared` emitted.
Without a third path those records would read active forever, and the active count
would only ever climb.

`startStaleSweep` runs one worker that calls `sweepStale` once a second, which
finalizes any record open longer than `stale-incident-timeout`. One second matches
the detector's own tick, so a record is closed within one tick of its timeout, and
the scan it costs is bounded by the ring capacity.

This is where the plugin does MORE than the `ddos/observe` template it was ported
from. That template has the same `sweepStale` function and never calls it in
production, so its `stale-incident-timeout` leaf controls nothing.

## Eviction prefers finished history

<!-- source: internal/plugins/anomaly/observe/store.go -- evictOldest -->

The ring holds at most `incident-ring-size` records, and `open` evicts before it
appends, so the memory ceiling is exact. `evictOldest` drops the oldest FINALIZED
record and falls back to the head only when every record is active. A finished
record is complete history; an active one is still being written, and losing it
would leave the operator blind to the incident happening now.

## A re-fire is a new lifecycle

<!-- source: internal/plugins/anomaly/observe/store.go -- open, finalize -->

`open` always appends, so a source confirmed again after it cleared gets a SECOND
record. `finalize` scans newest-first and closes the newest active record for the
entity. A repeat offender therefore shows one row per episode with its own duration,
rather than one row whose end time is rewritten.

The same reverse scan makes a clear with no matching open record a safe no-op, which
is what a reconfigure produces: `apply` rebuilds the store, so a clear for an
incident opened before the reconfigure finds nothing to close.

## The start time comes from the detector

<!-- source: internal/plugins/anomaly/observe/store.go -- open -->

`StartTime` is the event's `At`, the moment the detector CONFIRMED, not the moment
this store received the event. An event with no `At` falls back to the receive time,
because a zero timestamp would date the record to 1970: it would be instantly stale
and would report a duration of decades.

## Reconfigure order is fixed

<!-- source: internal/plugins/anomaly/observe/register.go -- runEngine, teardown, apply -->

`teardown` unsubscribes first and stops the sweep worker second, and only then does
`apply` build the replacement store. The stop function returns after the worker has
exited, so no goroutine is ever left sweeping a store the plugin has dropped.

## Surface

<!-- source: internal/plugins/anomaly/observe/show.go -- show anomaly observe -->
<!-- source: internal/plugins/anomaly/observe/yang/ze-anomaly-observe-conf.yang -- observe config -->
<!-- source: internal/plugins/anomaly/observe/cmd/yang/ze-anomaly-observe-cmd.yang -- show anomaly observe command -->

`anomaly { observe { } }` loads the store with its defaults; there is no `enabled`
leaf, because an empty ring costs a slice. `show anomaly observe` returns
`enabled`, `active-count`, and the ring newest-first, with finalized records and
their `end-time` included. The plugin runs as a goroutine, so the handler reads the
live ring from a process-global `atomic.Pointer[store]` rather than calling back into
the plugin.

The plugin defines no metric and no doctor check. The detector already exports
`ze_anomaly_active` and `ze_anomaly_incidents_total`, and the store depends on
nothing outside the process.
