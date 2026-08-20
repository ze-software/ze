# Deferrals: record-answers

Deferral rows for this spec family (`spec-record-answers-1-sdk-path`,
`spec-record-answers-2-only-encoding`, `spec-record-answers-3-zero-alloc`).
The aggregate live backlog is folded on read from `plan/deferrals/` by
`/ze-status`; nothing stores it (`ai/rules/planning.md`).

Two rows from the closed `spec-streaming-answer-protocol` still govern this
family and are NOT restated here. They live in
`plan/deferrals/streaming-answer-protocol.md`: record-level streaming for the
REST, gRPC, web, MCP and looking-glass surfaces (status `deferred`), and
`table` / `text` rendering buffering whatever the wire does (status `accepted`,
a permanent limit).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-20 | spec-record-answers-3-zero-alloc design | converting the payload handlers outside the two RIB walks | The other 380-odd handlers walk collections bounded by peer count, interface count or table size, so each answers as one `type=json` document whatever this family does. The per-row cost does not compound, and converting them would touch 219 `plugin.Map` call sites for no measured gain | its own spec, raised with the owner only if a bounded walk is ever measured as a cost | deferred |
