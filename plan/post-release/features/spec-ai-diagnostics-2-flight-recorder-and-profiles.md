# Spec: ai-diagnostics-2-flight-recorder-and-profiles

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Adds pre-incident flight-recorder capture and AI-readable profile summaries so an AI over MCP can diagnose unexpected bugs and lock wedges without waiting for a second reproduction; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 2 of the AI diagnostics architecture series. Today, when an unexpected bug happens in production at `info` log level, debug context leading up to the incident was never captured (`slogutil.ringHandler.Enabled` delegates to the output level and keeps 512 entries across all severities), `EventRing` (`internal/component/plugin/server/event_ring.go`) stores only `{Timestamp, Namespace, EventType}` with no subject or outcome, Go's `runtime/trace.FlightRecorder` is unused, and `show system profile` (`internal/component/cmd/show/profile.go`) lacks `mutex`/`block` profiles and returns only raw `pprof-base64` blobs that an AI over MCP cannot parse natively.

This spec covers:
1. Extending `show system profile` with `mutex` and `block` profile types and a structured text/JSON top-frames summary mode alongside `pprof-base64`.
2. Wiring Go's `runtime/trace.FlightRecorder` with a bounded rolling window, automatic snapshot triggers on `taskguard` panic or slow plugin delivery, and `show system trace` / `request system trace snapshot` inspection commands.
3. Splitting `slogutil.LogRing` so `warn`/`error` entries are retained independently of `info` bursts and low-volume control-plane transitions can be retained in memory without raising stderr/syslog verbosity.
4. Enriching `EventRecord` in `internal/component/plugin/server/event_ring.go` with compact, allocation-free source, subject, and outcome fields.
