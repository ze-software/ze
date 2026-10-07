# Protocol Event Capture and Replay

## Meta

| Field | Value |
|-------|-------|
| Name | Protocol Event Capture and Replay |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/reactor/capture_replay.go, internal/core/capture, internal/test/cli/cmd_replay.go |
| Real-path tests | test/plugin/bgp-capture-replay.ci |
| Docs | docs/guide/configuration.md, docs/features/bgp-protocol.md |
| Doc review | 2026-10-07: maximum-size range 1..1024 default 100 and on-limit rotate or stop read in ze-bgp-conf.yang; ze_bgp_capture_dropped_events_total registered in reactor_metrics.go |
| Defect review | 2026-10-07: no journal row or immediate spec names capture_replay.go or internal/core/capture |
| Extra criteria | supported: rotation at the maximum-size cap = test/plugin/bgp-capture-rotate.ci |

## Description

Per-peer, opt-in JSONL capture of a BGP session's inbound protocol events, and a replay harness that feeds a capture back through the same read path. Capture tees the COMPLETE wire message at both read paths (standard and coalesced) before RFC 7606 enforcement and before coalescing, so a malformed UPDATE is recorded exactly as the peer sent it and a coalesced batch is recorded as the separate messages it arrived as. Config operations applied while a capture runs are recorded with their transaction id, with secret-bearing values redacted. Off by default: the tee costs one nil comparison and no allocation per received message. Each file is capped at `capture { maximum-size }` megabytes exactly (1-1024, default 100); at the cap it rotates once to `<file>.1` or stops, per `capture { on-limit }`, so a peer uses at most twice the cap on disk. Writer backpressure sheds events rather than blocking the read loop, counts them in `ze_bgp_capture_dropped_events_total`, and writes the gap into the stream. `ze doctor` reports a capture directory the daemon cannot write. Replay: `le test replay <file>` drives `Session.ReadAndProcess` over a stub connection and an injected clock and reports the FSM transitions, the prefixes announced and withdrawn, and any NOTIFICATION the session sent. <!-- source: internal/component/bgp/reactor/capture_replay.go -- sessionCapture, teeCapture --> <!-- source: internal/core/capture/writer.go -- Writer --> <!-- source: internal/test/cli/cmd_replay.go -- runReplay -->
