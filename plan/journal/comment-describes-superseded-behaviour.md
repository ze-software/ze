# comment describes superseded behaviour

A package or function comment states what the code did before a change, and the
change lands without touching it. The comment then teaches the old contract to
every later reader, and it reads as authority because it sits above the symbol
it describes. `ai/rules/stale-comments.md` governs the write side. This class
counts the times the write side was skipped.

The cost is paid by a reader who trusts the comment instead of the producer.
That reader is often a documentation pass, so the wrong contract reaches an
operator-facing page before anybody runs the code.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | - | bgp/filter_modify | Both package headers said the modifier always returns modify and unconditionally sets declared attributes on every route, and told the reader to compose with an earlier match filter for conditional modification. `handleFilterUpdate` has returned `sdk.FilterAccept` for a route that meets no stated condition since the match container landed in d9c724e38 | rewrote both headers against the producer, naming the accept-unchanged path |
