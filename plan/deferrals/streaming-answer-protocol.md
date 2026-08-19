# Deferrals: streaming-answer-protocol

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-streaming-answer-protocol design | record-level streaming for the REST, gRPC, web, MCP and looking-glass surfaces | Those 24 consumers call `CommandDispatcher.JSON` and read `RenderedResponse.Output` as one string. Each can take the buffering path unchanged, so none of them blocks the goal, which is the operator and plugin wire. Streaming them is separable work with its own consumers and its own failure modes | its own spec, to be raised with the owner when this one closes | deferred |
| 2026-08-19 | spec-streaming-answer-protocol design | `table` and `text` rendering over a record stream | Column widths need every row before the first can be printed, so these two formats buffer whatever the wire does. Declared widths were considered and rejected as an option nobody asked for (Key Design Decisions) | none: accepted as a permanent limit, recorded in Known Limitations | accepted |
