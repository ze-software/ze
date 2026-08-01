# 1011 -- Automatic DDoS Detection & Auto-Mitigation (umbrella + all children)

## Context

Ze had no automatic attack detection. The cp-survival umbrella (spec-0) provided the mitigation levers (FlowSpec origination, firewall) but left the decision of when to pull them to operators or external detectors. This umbrella closes that gap by porting the Flowtriq ftagent's decision logic onto Ze's primitives: a two-stage detector (cheap rate trigger + on-attack pattern analysis), a local nft responder, a FlowSpec/RTBH upstream responder with leak-probe clear, and an incident store for observability.

## Decisions

- Chose EventBus broadcast (1:N) for detector-to-responder communication over DirectBridge (request/response), because detection is a broadcast to all interested responders, not a directed call
- Chose a two-stage detection design (rate trigger first, pattern analysis on-trigger only) over continuous per-IP monitoring, to keep steady-state cost near zero (one rate comparison per interface per tick)
- Chose to compute threshold BEFORE adding to baseline (not after) over ftagent's simpler order, because adding the spike sample first poisons the baseline on the very tick it should trigger
- Chose leak-probe for flowspec-mode clear over passive clear, because Ze has no inbound flow collector and upstream drop blinds every local sensor (the only honest signal is a bounded trickle)
- Chose injectable function vars (`registerTables`/`applyAll`, `announceFunc`/`withdrawFunc`) over interface injection for firewall/announce stubs, matching the existing `getActiveConnector` pattern
- Put shared event contract in `internal/core/ddosevent/` (core leaf) over inside the detector plugin, so responders import only the leaf and never the detector

## Consequences

- FlowSpec announce/withdraw is stubbed: the flowspec responder will fully activate only after cp-survival-4 lands its verb
- Stage-2 characterization (DirectBridge to trafficusage/flowexport) is designed but not yet wired: the detector emits `FamilyGenericFlood` until those handlers exist
- CLI commands (`show ddos status/incidents`, `clear ddos-mitigation`), doctor checks, and Prometheus gauges are designed in the specs but deferred: the core detection and response logic is complete
- The 5 new plugin packages (`ddosdetect`, `ddoslocal`, `ddosflowspec`, `ddosobserve`, plus core leaf `ddosevent`) are independent and individually removable per plugin-self-containment

## Gotchas

- Baseline poisoning: the spike sample must NOT be added to the baseline before the threshold check. ftagent's `_tick()` guards this by excluding attack-window samples, but our initial implementation added the sample first, which poisoned the p99 and prevented triggering. Fixed by checking threshold before `baseline.Add` and excluding above-threshold samples
- The detector's `onRate` callback receives cumulative packet counters (not PPS): PPS = delta between consecutive calls. Tests that fed the same counter value each tick got PPS=0 and never triggered
- `firewall.ApplyAll()` returns error when no backend is loaded (test environment); the responder must use injectable vars to be testable without a real nft backend
- YANG type names and firewall constants differ from naive guesses: `FamilyIP` not `FamilyIPv4`, `ChainFilter` not `ChainTypeFilter`, `HookInput` not `ChainHookInput`
- `sdk.ConfigSection.Data` is a JSON string, not `map[string]any`: ParseConfig must unmarshal from string, extracting the config-root key

## Files

### New packages
- `internal/core/ddosevent/` (event.go, event_test.go) -- shared event contract
- `internal/plugins/ddos/detect/` (baseline, state, detector, config, register, yang) -- detector plugin
- `internal/plugins/ddos/local/` (match, responder, config, register, yang) -- local nft responder
- `internal/plugins/ddos/flowspec/` (probe, match, responder, config, register, yang) -- FlowSpec/RTBH responder
- `internal/plugins/ddos/observe/` (store, config, register, yang) -- incident store

### Modified files
- `internal/component/vpp/vpp.go` -- IfaceStatsReader interface, GetActiveStatsProvider/setActiveStatsProvider
- `internal/plugins/iface/vpp/ifacevpp.go` -- getActiveStatsProvider var, GetStats wired
- `internal/plugins/iface/vpp/query.go` -- readVPPIfaceStats, ListInterfaces/GetInterface populate Stats
- `internal/plugins/iface/vpp/query_test.go` -- 6 new VPP stats tests
