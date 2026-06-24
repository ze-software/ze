# 976 -- chaos-mrt-recording

## Context
ze-chaos produced JSON event logs and dashboard/metrics output, but no standard
archival record of the BGP traffic it generated, so chaos sessions could not be
re-analysed with off-the-shelf MRT tooling. The goal was a new `report.Consumer`
(MRTLog) that emits RFC 6396 BGP4MP_MESSAGE_AS4 and BGP4MP_STATE_CHANGE_AS4
records from peer events, producing files readable by bgpdump, bgpkit-parser,
and ze-analyse. The core data-model change was carrying raw BGP wire bytes on
`peer.Event`, which previously had no field for them.

## Decisions
- Carry wire bytes on `peer.Event` via a new `BGPMessage []byte` field, chosen over
  a parallel channel or sender-side callbacks because every consumer already
  receives the Event and one zero-value-compatible field beats new infrastructure.
- Always emit AS4 subtypes (4/5), chosen over detecting 2- vs 4-byte AS because
  chaos peers are always 4-byte-AS and AS4 is the modern default (matches GoBGP/FRR).
- Reuse `internal/mrt` for all encoding (WriteCommonHeader/WriteBGP4MPMessage/
  WriteBGP4MPStateChange) and file I/O (Writer with strftime rotation) rather than
  re-implementing the wire format in the chaos tree.
- MRTLog mirrors jsonlog.go structurally (mutex, first-error tracking, Close
  returns accumulated error) for consistency across consumers.
- Track per-peer established[] state so a Disconnected before an Established (or a
  duplicate Disconnect) emits no spurious STATE_CHANGE record.

## Consequences
- Every chaos run can now produce a portable MRT file via `--mrt-file` alongside
  the JSON log, analysable by standard tooling and ze-analyse.
- peer.Event copies are slightly larger, but only when BGPMessage is non-nil;
  zero-value field keeps JSONLog/Dashboard/Metrics/Summary unaffected.
- Sent messages are zero-copy (same slice from sender to encoder); received
  messages cost one copy per UPDATE because readLoop reuses its buffer.

## Gotchas
- Received-message bytes cannot be threaded through emit() like sent bytes; the
  readLoop reuses a buffer, so they are assembled into a fresh slice and attached
  to the next pushed event via EventBuffer.SetBGPMessage/Push (pending-message
  mechanism, mirroring pending-bytes-recv).
- Out-of-bounds/negative PeerIndex must be guarded in both ProcessEvent and the
  header builder, else a malformed peer index panics; covered by an explicit test.
- The spec's Wiring Test table named `TestMRTLogProcessEvent` and
  `TestOrchestratorMRTFlag`, neither written verbatim: ProcessEvent is covered
  under other test names, but the `--mrt-file` orchestrator wiring (AC-9) has no
  executable test (the spec's TDD plan classifies orchestrator/functional tests as
  N/A for this tool). Internal spec inconsistency to watch for in future audits;
  the flag-to-consumer wiring is verified present in code (cli.go/types.go/run.go).
- This spec shipped without a `## Review Gate` or `## Implementation Summary`
  section; closure verdict came from an independent AC-by-AC code+test audit (8/9
  ACs have direct test coverage, AC-9 is code-only glue).

## Files
- `internal/chaos/report/mrtlog.go` -- MRTLog consumer (MRTLogConfig, MRTPeer, ProcessEvent, writeMessage, writeStateChange)
- `internal/chaos/report/mrtlog_test.go` -- unit + round-trip tests (AC-1..AC-8)
- `internal/chaos/peer/event.go` -- BGPMessage []byte field on Event
- `internal/chaos/peer/simulator.go` -- BGPMessage populated at 8 emit() sites
- `internal/chaos/peer/simulator_reader.go`, `ringbuf.go` -- received-message copy via SetBGPMessage/Push
- `internal/chaos/orchestrator/{cli.go,types.go,run.go}` -- --mrt-file flag, MRTFile config, consumer wiring
- `internal/mrt/{encode.go,types.go,writer.go,reader.go,decode.go}` -- shared MRT wire library (reused)
- `docs/features.md`, `docs/research/mrt-implementation-comparison.md` -- documentation updates
