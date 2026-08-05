# 1352 -- Regenerating An Index From The Working Tree Publishes Other Sessions' Unlanded Work

## Context

Adding `ze_bgp_announce_dropped_oversize_total` meant a new section in
`docs/guide/monitoring.md`, and every factual doc claim here carries a
`<!-- source: -->` anchor. Those anchors feed `ai/CODE-TO-DOCS.md`, a generated
index, so the commit had to carry a regenerated copy or `make ze-doc-test` goes
red. Running the generator the obvious way, `make ze-doc-index`, produced 46
added lines when the change justified four.

## Decisions

- Regenerated the index from a `git archive HEAD` extraction plus this change's
  own files, over regenerating from the working tree: the working tree held
  three concurrent sessions' uncommitted doc edits, and their anchors became
  index rows citing text absent from HEAD. That is the consumer-without-producer
  split that left HEAD unbuildable for 34 commits, in documentation form.
- Counted at the two drop LOGGERS over counting inside `announceAttrs.emit`,
  which is where the four detailed refusal lines live: `emit` returns one bool
  that both rails turn into a single drop, so counting there double-counts.
- Used a package-level `atomic.Pointer` over `Reactor.rmetrics`, which every
  other reactor counter reads. Neither drop site can reach a receiver: both
  loggers are free functions and `buildRIBRouteUpdate` takes none either. Same
  shape and same reason as `filterapi.SetMetricsRegistry`.
- Two labels, `rail` and `stage`, over one: `rail` alone does not say which
  region overflowed and `stage` alone does not say which writer refused. Four
  series is the whole product, and both values are constants at the call site.

## Consequences

- A drop that was visible only as a Warn line is now alertable. The route
  silently not arriving at one peer was the entire symptom.
- Any future generated-index regeneration in this repo has the same hazard. The
  recipe is the one `ai/rules/rule-format.md` already gives for rule digests,
  and it applies to `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` and
  `ai/LEARNED-FULL-INDEX.md` too.
- The local `ze-doc-test` stays RED while those sessions hold their edits,
  because the check regenerates from the working tree and compares. The
  committed content is what CI regenerates from HEAD, so the two disagree by
  construction until the other work lands. Attribute it, do not chase it.

## Gotchas

- **A scratch tree under `tmp/` silently generates NOTHING.**
  `code_to_docs.py` calls `filter_gitignored`, `tmp/` is gitignored, so every
  doc under `tmp/s/<scratch>/docs/` was filtered out and the generator reported
  "0 code paths" and cheerfully overwrote the real index with an empty one. The
  fix is `git init` inside the scratch tree so gitignore resolves against it
  rather than against the parent repo. Nothing warned; the exit code was 0.
- **Generators differ in how they take a root.** `learned_index.py` accepts
  `--root` through `discovery_sources.root_from_argv`. `code_to_docs.py` does
  not: it derives the root from `Path(__file__).resolve().parents[2]`, so the
  script itself must be copied into the scratch tree. Check which before
  assuming a flag exists.
- **A "0 items" generator result is a failure, not an empty input.** Both times
  it happened the exit code was 0 and the message read like a normal write. Read
  the count.
- **The obvious counting site is the wrong one.** Four log lines inside
  `announceAttrs.emit` describe why an announce was refused, and all four sit
  under one `false` return that each rail turns into a single drop. Counting
  where the detail is printed is not counting where the decision is made.

## Files

- `internal/component/bgp/reactor/announce_metrics.go` -- counter, closed label sets, recorder
- `internal/component/bgp/reactor/announce_metrics_test.go` -- three tests, the wiring one mutation-verified twice
- `internal/component/bgp/reactor/reactor_api_batch.go` -- batch rail records
- `internal/component/bgp/reactor/peer_rib_routes.go` -- queued rail records
- `internal/component/bgp/reactor/reactor.go` -- registry wired beside `filterapi.SetMetricsRegistry`
- `docs/guide/monitoring.md` -- operator-facing section with source anchors
