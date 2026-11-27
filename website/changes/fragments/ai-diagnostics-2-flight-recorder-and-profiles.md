---
spec: ai-diagnostics-2-flight-recorder-and-profiles
topic: Diagnostics
---

## Before

`show system profile` supported only `cpu`, `heap`, `goroutine`, and `allocs` profiles and returned only raw base64-encoded `pprof` blobs requiring `go tool pprof` offline, leaving mutex and blocking contention unmeasurable from the CLI or MCP. Pre-incident Go execution traces were not captured automatically when a guarded task panicked or a plugin stalled. High-volume `info` or `debug` log bursts evicted earlier `warn` and `error` entries from the single 512-entry log ring, and `show event recent` reported only `timestamp`, `namespace`, and `event-type` without naming the event's emitter, peer subject, or direction.

## After

`show system profile` adds `type mutex` and `type block` (backed by low-overhead daemon startup contention sampling reporting `sampling-rate`) and a `| summary` pipe operation (`pipe: [{"name": "summary"}]` over MCP) that decodes any of the six profile types inside the daemon into a bounded JSON summary of up to 32 top functions with flat and cumulative totals and percentages while keeping bare `pprof-base64` output unchanged. The daemon now runs Go's always-on execution flight recorder in a bounded 10-second / 4 MiB rolling window with a 4-slot in-memory snapshot ring, auto-capturing (with a 30-second cooldown) on recovered task panics and slow plugin deliveries; operators and AI assistants can also capture snapshots on demand with `request system trace snapshot`, list recorder status and snapshot metadata with `show system trace`, and fetch base64 Go trace payloads with `show system trace id <id>`. The in-memory log ring now separates `warn` and `error` entries (256 slots) from `debug` and `info` entries (512 slots) so noisy traffic never evicts earlier faults, `show event recent` includes optional `source`, `subject`, and `direction` fields captured before subscriber dispatch, and `show skill ze-diagnostics` documents the new profile summary and trace commands.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
