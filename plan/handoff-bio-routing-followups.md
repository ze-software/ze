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

**Status: closed by skeleton spec (2026-04-11).** `plan/spec-grpc-domain-types.md`.

Audit result (baked into the spec): **no proto leaks.** `zepb` only
appears in `internal/component/api/grpc/server.go` and `server_test.go`.
The physical boundary is already clean. The actual gap is that the
conversion target — domain request types in `internal/component/api/` —
**does not exist at all**. Each transport handler extracts fields inline
and calls the engine with positional primitives (`string command`,
`session ID + path + value`, etc.). REST and gRPC maintain parallel
extraction logic for the same engine methods.

The spec proposes adding typed request structs in
`internal/component/api/requests.go`, moving ad-hoc extraction into
`<transport>/convert.go` helpers, and (optionally — open question 1 in
the spec) changing the engine signatures to take the domain request type
instead of positional primitives. Five open design questions remain for
the `/ze-spec` pass:

1. Engine signature change or optional middle layer?
2. Domain struct name collisions with `zepb.*Request`.
3. Keep `Execute(command string)` or type the params?
4. Does the REST side need the same convert helpers as gRPC?
5. Where does `AuthContext` get constructed — transport or domain?

### C. Pattern #8: pull-model metrics Collector decision doc

**Status: closed by decision doc (2026-04-11).** `plan/decision-pull-model-metrics.md`.

Recommendation: **defer.** Reasons in the doc:

- Ze already has the abstract `metrics.Registry` interface in
  `internal/core/metrics/metrics.go`. The "Prometheus types everywhere"
  urgency framing does not apply — verified by grep for
  `github.com/prometheus/` (only matches outside vendor are
  `internal/core/metrics/prometheus.go` and
  `internal/chaos/report/metrics.go`).
- Six internal plugins use the push model (`ConfigureMetrics` callback).
  Small enough that a later refactor is mechanical.
- **External plugins have zero metrics hook today** — no `Snapshot` RPC,
  no metrics namespace, nothing. The pull model would force a protocol
  decision (sync `Snapshot` RPC vs streaming snapshot vs in-process only)
  that the current push model sidesteps by also not handling the external
  case. This is the forcing function the refactor is waiting on.

Revisit triggers recorded in the doc: plugin count doubling, a second
export format request (OpenTelemetry / JSON / CLI dump), a hot-path
UPDATE profile showing gauge `Inc`/`Add` significance, or the external
plugin metrics gap becoming user-visible.

### D. Free patterns #2, #3, #4, #9, #10

**Status: closed by edit to `.claude/rules/go-standards.md` (2026-04-11).**

Added a new "Style patterns to prefer" section with five one-line bullets:
drain `time.NewTimer` on Stop, slice-type-with-methods over wrapping struct,
family of narrow constructors, table-driven `t.Run` tests, honest TODO
comments over silent gaps. Each bullet is advisory (adopt opportunistically,
not a sweep).

Pattern #2 (stopTimer) was reframed in the rule because ze does not need
the bio-rd helper verbatim: the FSM uses `clock.Timer` with `AfterFunc`
(no channel to drain), and the only `time.NewTimer` site in the BGP tree
(`internal/component/bgp/plugins/rs/worker.go:379`) already has its own
`drainTimer` helper. The rule captures "promote or copy `drainTimer` when
adding a new `time.NewTimer` site, prefer `AfterFunc` where possible."

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
