# 1109 -- ddos-detect Enhancements (bandwidth trigger, baseline persistence, incident confidence)

## Context
Ported three Flowtriq ftagent DDoS-detection improvements into `internal/plugins/ddos/detect`:
a bandwidth (BPS) trigger that catches low-PPS/high-bandwidth amplification the packet-rate
trigger misses, rolling-baseline persistence across restarts, and a 0-100 incident
"confidence" score surfaced on `show ddos incidents`. The confidence math and the observe
store were unit-tested and green, but validating AC-9 end-to-end under QEMU exposed that
characterization -- the stage that produces the confidence-bearing `AttackCharacterized`
event -- was effectively non-functional at production defaults: it read a stale flow ring
and always fell back to generic-flood, so confidence was never computed on a live box.

## Decisions
- BPS baseline as a SECOND `baseline` instance (`d.baselineBps`) over generalizing `baseline` to a metric-agnostic type: minimum change, reuses the p99 recalc + poisoning guard + `Ready()` verbatim.
- `bps-floor` leaf in bits/sec with an ×8 convert-at-compare over bytes/sec: operators think in Mbps/Gbps; ze's `RxBps` is bytes/sec (`iface/rate.go`).
- Confidence on `AttackCharacterized` only (not `AttackDetected`): the fast signal carries no classification, so it has nothing to score.
- **Force a conntrack dump on `AttackDetected` + bounded retry in the characterizer** over (a) leaving one-shot characterization, (b) telling operators to lower `active-timeout` globally, or (c) a fast-dump-while-attacking mode. The forced dump is a single extra full-table dump per attack (rare); the delta tracker makes it safe (each byte counted once, it just exports earlier).
- flow-export SUBSCRIBES to `ddosevent.Detected` (a core event) to trigger the dump, over adding a new cross-plugin `request` command: reuses the shared event bus flow-export already consumes (BGP enrichment), avoids CLI-grammar/schema plumbing, and keeps the dependency plugin->core.
- The out-of-band dump routes through the worker's run-loop via a coalesced `refreshCh`, over calling `dumpAndExport` directly from the event goroutine: serializes with the ticker dump, and a burst of triggers cannot queue a backlog.

## Consequences
- Characterization no longer races the ring-refresh cadence; `active-timeout` is now purely a NetFlow/IPFIX export-cadence knob, not a correctness lever for the classifier. The mitigation guide's "tune active-timeout" advice was removed.
- flow-export now depends on `internal/core/ddosevent` -- a generic exporter that reacts to attack events to keep its ring fresh. Tier-safe (plugin->core) but a real coupling to be aware of.
- The appliance still needs ze to set up conntrack ITSELF (see Gotchas): nothing in ze loads `nf_conntrack`/`nf_conntrack_netlink`, registers a `ct` hook, or enables accounting. On gokrazy only ze runs, so flow-export/conntrack (and thus DDoS characterization) silently produces an empty ring unless ze does this. `flowexport/doctor.go` warns; the ze_appliance conntrack-init is the tracked follow-up.
- Confidence attaches to an incident by `Target.DstPrefix`, with a fallback to the still-unresolved active incident on the same interface (`observe/store.go`): a victim resolved only from the flow ring (IPv6-only, invisible to trafficusage; or trafficusage lag at confirm) now still records its confidence. The fallback leaves the incident target empty so `finalize` (which also carries an empty target) keeps matching.

## Gotchas
- **The unit tests all passed while the feature was broken end-to-end.** `GradeConfidence` and the observe store were green; only the live QEMU path revealed that the ring is stale at attack-confirm. One-shot characterization at confirm (~1s) reads a ring that refreshes only every `active-timeout` (default 60s), so it never saw the attack. A confidence feature can be fully "unit-tested" and still never fire in production.
- **flow-export conntrack export needs FOUR kernel prerequisites, none provided by a bare module load:** `nf_conntrack` + `nf_conntrack_netlink` (the reader dumps via ctnetlink), a rule referencing `ct state` (registers the tracking hooks netns-wide -- the module alone tracks nothing), AND `nf_conntrack_acct=1` (without accounting every flow's byte/packet counters are 0, so `dumpAndExport` drops every zero-delta flow before it reaches the ring). Module-loaded != tracking != accounting.
- **Loopback victim attribution needs a sink socket.** Flooding a loopback victim port with no listener makes the kernel emit one ICMP port-unreachable per packet back to the source; on `lo` that backscatter out-totals the flood itself, so trafficusage egress-ips picks the reflection SOURCE as the victim. A real multi-interface router never sees this; the .ci binds a sink to suppress the ICMP.
- **The recent-flow ring is passive.** It is fed only by the periodic/forced conntrack dump; `show flow-recent` snapshots it and never triggers a dump. Freshness must be pushed (the forced dump), not pulled by the reader.
- BPS reuses the `baseline` type, so the planned `TestBaseline_Bps*` unit tests folded into detector-level trigger tests + config tests -- testing the actual new logic, not re-testing the reused type.

## Files
- `internal/plugins/ddos/detect/`: `baseline.go` (BPS series + snapshot/restore), `detector.go` (BPS trigger, restore/save lifecycle), `characterize.go` (`awaitClassifiableFlows` bounded retry + `genCurrent`), `config.go`, `register.go`, `persist.go`, `metrics.go`
- `internal/plugins/flowexport/`: `conntrack_worker.go` (`refreshCh` + `Refresh`), `exporter.go` (`refreshConntrack`), `register.go` (AttackDetected subscription), `doctor.go` (conntrack-unavailable warning)
- `internal/core/ddosevent/event.go`, `internal/core/diagnostic/codes.go`
- `internal/plugins/ddos/{observe,flowtriq,local,flowspec}/` (confidence carry + `confidence-min` gate)
- `test/plugin/ddos-bps-amplification.ci`, `test/plugin/ddos-incident-confidence.ci`
- `docs/guide/ddos-mitigation.md`, `ai/digests/flow-ddos.md`, `scripts/evidence/qemu-run.py` (test-harness conntrack setup)
