# Handoff: bio-routing analysis follow-ups

## Rationale (verify this matches what we agreed)

- **The FSM "make it easier to follow" goal is closed via documentation,
  not refactor.** We chose Option D (per-state runbooks) over a
  state-object FSM refactor because (a) Option C would absorb the
  session I/O layer into the FSM and add channel-hop overhead on the
  per-UPDATE hot path, breaking ze's lazy-wire / ContextID-reuse perf
  story, and (b) the layering between `fsm` (pure state) and `reactor`
  (I/O, timers, sessions) is deliberate. The runbooks under
  `docs/architecture/behavior/fsm-*.md` and `peer-lifecycle.md` give
  readers the "open one place, see everything for state X" property
  without touching code or merging layers. -> EDIT 1, EDIT 2 already
  shipped.
- **The dead `fsm.Event(EventUpdateMsg)` per-UPDATE call stays.** User
  decision: leave as-is, accept the ~15ns lock+switch cost per UPDATE.
  Note for future readers: the compiler cannot elide this; the lock
  has memory-ordering side effects. The decision is not a
  misunderstanding to revisit, it is "the cost is acceptable today".
- **The bio-routing "free patterns" group (#2 stopTimer, #3 slice-type,
  #4 narrow constructors, #9 table-driven tests, #10 honest TODOs) is
  not in scope as a sweep.** Adopt opportunistically. The original
  prompt said "if a coding-standards.md or equivalent exists, propose
  one line per pattern; otherwise skip". That check has not been done.
- **Pattern #7 (gRPC domain-type + toProto conversion) is a separate
  task.** User explicitly carved it out in the original prompt. The
  follow-up prompt is reproduced verbatim below for one-shot resume.
- **Pattern #8 (pull-model metrics Collector) is a decision doc, not
  an implementation.** User wanted three questions answered before any
  refactor: is it worth doing now, what is the cost, how does it work
  for external plugins running over RPC. Reproduced below.
- **Four bio-routing-derived skeleton specs are still untracked**
  in `plan/`. They are valid skeleton specs ready to be picked up or
  decided against. Listed below with one-line summaries.

If any bullet is wrong, STOP and fix this handoff before picking up
the work.

## Files already handled (don't re-read for the FSM goal)

These are the docs that close the readability goal. No need to re-read
them unless you are changing them:

- `docs/architecture/behavior/fsm.md` (index, links to all runbooks)
- `docs/architecture/behavior/fsm-idle.md`
- `docs/architecture/behavior/fsm-connect.md`
- `docs/architecture/behavior/fsm-active.md`
- `docs/architecture/behavior/fsm-open-sent.md`
- `docs/architecture/behavior/fsm-open-confirm.md`
- `docs/architecture/behavior/fsm-established.md`
- `docs/architecture/behavior/peer-lifecycle.md`
- `docs/research/comparison/bio-routing/analysis.md`
- `docs/research/comparison/bio-routing/code-review.md`

## What's left, ordered by independence

### A. Decide and act on the four untracked bio-routing specs

These five files exist on disk untracked:

| File | What it is | Status |
|------|------------|--------|
| `plan/spec-pkg-ze-bgp.md` | Public Go API for BGP primitives under `pkg/ze/bgp/` | Skeleton, ready for design pass |
| `plan/spec-bmp-receiver.md` | BMP receiver (RFC 7854) as a new plugin | Skeleton |
| `plan/spec-remote-rib-client.md` | Subscribe to a remote RIB stream and inject as a synthetic peer | Skeleton |
| `plan/spec-canonical-in-repo-docs.md` | Top-level docs entry pages (`docs/architecture.md`, `docs/config-reference.md`, `docs/bgp-fsm.md`, `docs/plugin-overview.md`) | Skeleton, **partially overlapping with the FSM runbooks already shipped** |

The fifth, `plan/spec-fsm-per-state-split.md`, was deleted by the user
(`rm plan/spec-fsm-per-state-split.md`) because it was the
file-split-only Option A which we superseded with Option D.

**Decisions to make for each:**

1. `spec-pkg-ze-bgp.md`: keep as-is (skeleton), refine via `/ze-spec`,
   commit, or drop?
2. `spec-bmp-receiver.md`: same.
3. `spec-remote-rib-client.md`: same. Note: needs a streaming format
   choice (gRPC) that is still TBD inside the spec.
4. `spec-canonical-in-repo-docs.md`: trim. The "BGP FSM walkthrough" page
   it proposes is now redundant because `docs/architecture/behavior/fsm.md`
   plus the runbooks already do that job better. The other three
   proposed pages (`architecture.md`, `config-reference.md`,
   `plugin-overview.md`) are still useful. Decide whether to (a) trim
   the spec to drop the FSM page, (b) keep it as-is and let
   implementation discover the redundancy, or (c) drop the spec
   entirely and let `docs/DESIGN.md` continue to be the only top-level
   entry doc.

**Suggested action:** for each spec, either commit it as a skeleton
in `plan/` so the work is queued, or delete it. Mixing untracked
specs with tracked ones loses state across sessions. Either commit-them-all
in one script, or rm-them-all.

### B. Pattern #7: gRPC domain-type + toProto conversion at the API boundary

User's original prompt, reproduced verbatim for resume:

> Read @<FILE>, specifically the pattern #7 section ("Domain struct +
> toProtoRequest conversion"). Ze recently added a gRPC API — see
> `api/proto/ze.proto` and `internal/component/api/grpc/`. I want every
> boundary between the gRPC transport and the API engine to go through
> a domain-type conversion layer, so proto types never leak into the
> engine. Do an audit: grep for `ze.api.v1.*Request` and
> `ze.api.v1.*Response` usage outside `internal/component/api/grpc/`.
> Anywhere else? Write a short plan to add the conversion layer. Don't
> code yet.

Reference for `<FILE>`: `docs/research/comparison/bio-routing/code-review.md`,
pattern #7 section.

**Concrete first steps:**
1. `Grep "ze.api.v1" internal/` excluding `internal/component/api/grpc/`
2. `Grep "ze.api.v1" cmd/` and `pkg/`
3. `Grep "ze.api.v1" plugins/` (if external plugin code path exists)
4. Tabulate every leak: file, symbol, what proto type appears.
5. Write `plan/spec-grpc-domain-types.md` proposing the conversion
   layer location and the mapping pattern (mirror bio-rd's `Request`
   struct + `toProtoRequest()` shape).
6. Stop. Do not code.

### C. Pattern #8: pull-model metrics Collector decision doc

User's original prompt, verbatim:

> Read @<FILE>, specifically pattern #8 (pull-model metrics Collector).
> Ze just landed ConfigureMetrics-based plugin metrics on six plugins.
> bio-rd does it differently: plugins expose a Snapshot() method
> returning a plain Go struct, and a separate adapter package converts
> to Prometheus on scrape. This is a real refactor — not a spot fix.
> Before starting, answer: (1) is this worth doing now or should I wait
> until the metrics surface is larger? (2) what would the cost be?
> (3) how does external-plugin metrics work in the pull model (since
> external plugins run over RPC)? Write a one-page decision doc with a
> recommendation. Don't start the refactor.

Reference for `<FILE>`: `docs/research/comparison/bio-routing/code-review.md`,
pattern #8 section.

**Important pre-check that has already been done:** `internal/component/bgp/`
contains **zero direct `github.com/prometheus/` imports**. Only
`internal/core/metrics/prometheus.go` and `internal/chaos/report/metrics.go`
touch Prometheus types. So the BGP plugins are already decoupled at the
import level. The decision doc should incorporate this finding because
the urgency framing in bio-rd's analysis ("there are still plugins that
take a Prometheus-typed registry directly") is no longer accurate.

The decision doc lives at `plan/decision-pull-model-metrics.md`. Format:
one page, three answered questions, recommendation, no spec.

### D. Free patterns #2, #3, #4, #9, #10

User's original prompt, verbatim:

> Read @<FILE>, specifically the numbered patterns 2 (stopTimer
> helper), 3 (slice-type with methods), 4 (narrow constructor families),
> 9 (table-driven tests with t.Run), 10 (honest TODO comments). These
> are "free" — adopt them the next time I'm in the relevant code. Don't
> do a sweep now; just note them as coding-standard additions. If this
> repo has a `coding-standards.md` or equivalent, propose a small PR to
> add a one-line mention of each with a link to the bio-rd analysis
> file. Otherwise skip it.

Reference: `docs/research/comparison/bio-routing/code-review.md`,
patterns #2, #3, #4, #9, #10.

**Important note on #2 (stopTimer):** the FSM package does not need
the helper. `internal/component/bgp/fsm/timer.go` uses `clock.Timer`
with `AfterFunc` exclusively, which is callback-based and has no
channel to drain. The only place in the BGP tree that uses
`time.NewTimer` directly is `internal/component/bgp/plugins/rs/worker.go`
which already has its own `drainTimer` helper at line 379. So
"#2" is already correctly handled in the codebase. The coding-standards
mention should reflect "use `drainTimer` from rs/worker.go (or promote
it) when introducing a new `time.NewTimer` site, prefer `AfterFunc`
where possible".

**Concrete first steps:**
1. `Glob "**/coding-standards.md"`, `Glob "**/STYLE.md"`,
   `Glob "**/contributing*.md"`, `Glob ".claude/rules/go-standards.md"`
2. If a suitable file exists: propose a 5-bullet edit linking each
   pattern to `docs/research/comparison/bio-routing/code-review.md`.
3. If nothing exists: per the original prompt, skip.

### E. Decision: move the HoldTimer restart into the FSM

**Status: closed by implementation (2026-04-11).** Do not revisit.

Original decision (superseded): leave `fsm.Event(EventUpdateMsg)` as a
dead call, accept the ~15ns cost.

Revised decision (acted on): instead of leaving the FSM event dead,
make it productive by performing the RFC 4271 §8.2.2 Events 26/27
HoldTimer restart inside the FSM handler itself. The FSM now holds a
`*Timers` reference (wired in `reactor.NewSession` via `fsm.SetTimers`)
and calls `ResetHoldTimer()` from:

- `handleOpenConfirm` on `EventKeepaliveMsg`
- `handleEstablished` on `EventKeepaliveMsg`
- `handleEstablished` on `EventUpdateMsg`

The corresponding external `s.timers.ResetHoldTimer()` calls in
`handleKeepalive`, `handleUpdate`, and the two RFC 7606 / prefix-limit
short-circuit paths in `session_read.go` are removed; the FSM event
fired on those paths (`logFSMEvent(fsm.EventUpdateMsg)`) performs the
reset. The `handleOpen` tail reset stays, because it resets to the
**negotiated** value after OPEN processing, not as a liveness proof.

Architectural rationale:

- RFC §8.2.2 attaches the HoldTimer restart to Events 26 and 27, not to
  arbitrary session layer points. Owning it in the FSM matches the
  RFC text and centralizes the liveness rule.
- The fsm/reactor layering is not broken: `fsm.Timers` already lives
  in the `fsm` package. The FSM is not gaining a dependency on the
  reactor; it is simply exercising a primitive already in its own
  package.
- ROUTE-REFRESH does not currently fire `EventUpdateMsg` and still
  skips the HoldTimer restart. That is a separate bug (RFC 2918 does
  not mandate the restart, but liveness proof from ROUTE-REFRESH is
  common practice). **Out of scope for this change.**

## What changed since the last commit (b4f45fc5 second commit script)

If the second commit script (`tmp/commit-b4f45fc5.sh`) has not been
run yet, three files are still uncommitted:

- `docs/architecture/behavior/fsm.md` (added link to peer-lifecycle.md)
- `docs/architecture/behavior/fsm-established.md` (two anchor fixes:
  startSendHoldTimer location, fwdPool name)
- `docs/architecture/behavior/peer-lifecycle.md` (new file)

Run the script to land them:

```
bash tmp/commit-b4f45fc5.sh
```

After that, the FSM-readability work is fully shipped. Anything in
this handoff is independent and can be picked up in any order.

## Verification command after applying anything from this handoff

For docs-only changes:

```
make ze-doc-test
```

For Go code changes (none in this handoff yet):

```
make ze-verify
```

For the spec commits:

```
git status plan/
```

## Where this handoff lives

`plan/handoff-bio-routing-followups.md` (this file). Keep it until all
sections are closed; delete it when nothing remains. It is a working
document, not a permanent artifact.
