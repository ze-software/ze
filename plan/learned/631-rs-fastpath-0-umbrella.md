# 631 -- rs-fastpath-0-umbrella

## Context

Ze's route-server forwarding throughput had regressed badly relative to pre-RIB baselines: 49k rps at 100k routes, 16x behind bird on the same docker harness, with a superlinear cliff above 75k routes. The user constraint was explicit: "the RIB must not slow down packet forwarding." The umbrella coordinated a three-child effort to profile the regression, move the adj-rib-in off the hot path, and install a zero-copy pass-through fast path, then re-benchmark to prove the per-route forwarding cost had returned to sub-microsecond territory with no scaling cliff.

## Decisions

- Split the work into three children executed in order rather than a single mega-spec: child 1 captured the profile evidence that anchored the AC-1 numeric target, child 2 decoupled adj-rib-in (no longer a hard dep), child 3 introduced `Plugin.ForwardCached` / `ReleaseCached` + `DirectBridge` so rs never round-trips through the text-RPC command registry on the forward hot path.
- Fixed AC-1's absolute floor (400k rps, 200k floor, 50ms first-route) rather than re-anchoring to whatever bird happened to measure at close time. Bird moved from 781k rps (2026-04-17) to 1.67M rps (2026-04-18) on the same harness between baseline and close, almost certainly due to host-level variance; re-anchoring would have hidden the 8x real improvement ze made.
- Closed deferral row 202 (`CommandRegistry.All()` forward-path allocation) by structural bypass rather than engine-side fix. Verified all three `registry.All()` callers (`HasCommandPrefix`, `dispatchPlugin`, no-match debug log) are CLI-input dispatch, none reachable from `rs.plugin.ForwardCached -> DirectBridge -> handleForwardCachedDirect`.

## Consequences

- Ze at 100k routes: 407k rps, 13ms first-route, 246ms convergence. 8x throughput improvement over the 49k baseline; 16x-behind-bird gap closed to 4x.
- Scaling sweep 10k-100k: 294k / 338k / 431k / 395k / 407k rps, spread -21%/+16% from mean (within AC-3 +/-25%). No cliff above 75k anymore.
- `Plugin.ForwardCached` / `ReleaseCached` are now the canonical fast-path SDK surface for plugin-authored forwarders. Future forwarders (route-reflector variants, redistribute egress) should pattern after rs, not the legacy text-RPC path.
- `bgp-rs` no longer requires `bgp-adj-rib-in`. rs now works standalone; replay-on-new-peer lights up only when adj-rib-in is present as a side-subscriber. Opens the door to lighter-weight route-server deployments.
- Remaining bird gap (4x on throughput, 4x on convergence) is future work tracked in the follow-up list, not this umbrella. Likely candidates: lower `updateRouteTimeout`, `maxForwardDestinations` as env var, `sync.Pool` for `destinationsToSelector` working buffers, full extract of the `ForwardUpdate` per-destination loop into `forwardUpdateCore`.

## Gotchas

- `first-route-ms` at 10k came back `-1296` (sender observed the route before the measurement window opened). Harness artefact at small scale; the number is meaningless and was ignored for AC purposes. At 25k and above the artefact disappears.
- Bird's throughput on the same hardware jumped 2x between the 2026-04-17 baseline and the 2026-04-18 close. Docker / colima / macOS scheduler variance can swing raw rps by 2x on M4 Max; treat same-day ze vs bird numbers as the only comparable snapshots, not cross-day.
- The ze-interop docker image caches aggressively. The 2026-04-05 image was stale against rs-fastpath-3; must rebuild (`python3 test/perf/run.py --build ze`) before trusting any AC-1 number. Build takes ~5 min on M4 Max.

## Files

- `plan/learned/625-rs-fastpath-1-profile.md` -- child 1 profile + harness.
- `plan/learned/626-rs-fastpath-2-adjrib.md` -- child 2 adj-rib-in side-subscriber.
- `plan/learned/630-rs-fastpath-3-passthrough.md` -- child 3 zero-copy pass-through.
- `test/perf/results/ze-{10k,25k,50k,75k,100k}.json` -- 2026-04-18 sweep evidence.
- `test/perf/results/bird-100k.json` -- 2026-04-18 bird comparison.
- `plan/deferrals.md` rows 202 + 203 -- closed at umbrella close.
