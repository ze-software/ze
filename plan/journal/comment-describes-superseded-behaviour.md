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
| 2026-08-17 | spec-rfc4271-med-ibgp-readvertisement | bgp/config_direction_test | `TestReceivedOnlyGuardReadsEveryShape` still described two one-line alias shapes as parser-refused after the parser accepted them, so the reactor package gate failed on stale `viaParser: false` expectations instead of a guard defect | updated the case names, comments, and `viaParser` expectations to pin the parsed-tree path |
| 2026-08-17 | fixit-relax-audit-reports-the-wrong-token | test-weakening audit tooling, spec text | The spec described a positional slice in `run_audit`, which quoted the wrong `test-relax:` reason back at a reviewer. That scanner was retired for a row-based audit over `test/weakened.md`. Eight days later the fix had no code to edit. Nothing in the spec said so | Closure read `run_audit` at its producer first, and confirmed that `relax_reasons`, `_RELAX_LINE`, `test-relax:` and `"RELAXED"` are absent. The spec closed with no code change. `test_the_audit_scans_no_token` reads the script source, so the old path cannot return unnoticed. General practice: check that a spec's named mechanism still exists before you write its failing test |
| 2026-08-17 | fixit-review-loop-has-no-termination-bound | review gate rules and skills | The review-round cap moved from three to five, and six surfaces teach that number. The landing commit `2a9773663` moved four of them. `ai/skills/ze-review.md` and `plan/TEMPLATE-CLOSURE.md` still priced a fourth pass, and neither named the owner authorisation a sixth pass now needs | This closure rewrote both surfaces. `ai/rules/repo-maintenance.md` and `ai/rules/context-economy.md` still teach three, and so do their two point renders. A rule edit regenerates `ai/rules/CORE.md`, which another live session holds, so the four files are reported for routing rather than fixed here. General practice: sweep every surface that carries a number, in the commit that changes it. A grep for the old value is that sweep |
