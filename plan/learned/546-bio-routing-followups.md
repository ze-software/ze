# 546 -- bio-routing comparison follow-ups (Sections A/B/C/D/E)

## Context

`plan/handoff-bio-routing-followups.md` was a working document from a
prior session that tracked five follow-ups from the bio-routing code
review in `docs/research/comparison/bio-routing/`. This session closed
all five and removed the handoff. Three commits landed:

- `e862dda4` -- Section A: four skeleton specs queued as tracked files
  (`spec-pkg-ze-bgp`, `spec-bgp-4-bmp`, `spec-rib-3-remote-client`,
  `spec-docs-1-canonical`).
- `6be9a0c3` -- Section E: FSM HoldTimer restart refactor.
- `7e0bfdd1` -- Sections B/C/D: gRPC domain-types spec, pull-metrics
  decision doc, five style bullets in `ai/rules/go-standards.md`.

## Section E -- FSM HoldTimer restart (the one that actually changed code)

### Original decision that was wrong

The handoff recorded Section E as "closed by user decision: leave the
dead `fsm.Event(EventUpdateMsg)` call, accept the ~15ns cost per UPDATE".
The call at `reactor/session_handlers.go:206` fired on every received
UPDATE, landed in the `handleEstablished` case `EventUpdateMsg` arm, and
did nothing (the arm was a documentation-only comment saying "handled
externally"). The layering argument was that the `fsm` package is pure
state and should not own side effects.

### What actually changed during the session

The user re-opened the decision by pointing at the "handled externally"
comment and saying "we should reset the hold timer there". The key
observations that made the refactor trivial instead of a layering break:

1. **`fsm.Timers` already lives in the `fsm` package**
   (`internal/component/bgp/fsm/timer.go`). Giving `FSM` a `*Timers`
   field is not a cross-package dependency; the timer primitive is
   already in `fsm`. The "pure state" framing was aspirational, not
   enforced.

2. **RFC 4271 §8.2.2 Events 26 (KeepAliveMsg) and 27 (UpdateMsg) both
   attach the HoldTimer restart to the EVENT**, not to the state. The
   RFC says "restarts its HoldTimer, if the negotiated HoldTime value
   is non-zero". Owning the reset in the FSM handler matches the RFC
   text literally; owning it in the session layer was a deliberate
   offset that had no RFC support.

3. **Lock ordering is safe.** `FSM.Event` takes `f.mu`; inside the
   handler it now calls `f.timers.ResetHoldTimer()` which briefly takes
   `t.mu`. Ordering is `f.mu -> t.mu`. The reverse direction does not
   exist: the `AfterFunc` hold-timer-expiry callback releases `t.mu`
   BEFORE invoking its callback (see `timer.go:205-214`), so the
   session's `OnHoldTimerExpires` handler can take `s.mu` and `f.mu`
   without ever nesting under `t.mu`.

4. **Nil guard keeps pure-FSM tests green.** `TestFSMUpdateInEstablished`
   and friends in `fsm_test.go` construct `fsm.New()` without wiring a
   `Timers`. Guarding `if f.timers != nil` lets those tests continue to
   assert state transitions without having to plumb a timer manager
   through every test.

### The change

- `fsm.FSM` gained a `timers *Timers` field and a `SetTimers` method.
- Three handler arms now perform the reset:
  `handleOpenConfirm` EventKeepaliveMsg (before `change(StateEstablished)`),
  `handleEstablished` EventKeepaliveMsg, and
  `handleEstablished` EventUpdateMsg.
- `reactor.NewSession` wires `s.fsm.SetTimers(s.timers)` right after
  both are constructed.
- External resets removed from `handleKeepalive`, `handleUpdate`, and
  two short-circuit UPDATE paths in `session_read.go` (RFC 7606
  treat-as-withdraw, prefix-limit AC-27 drop). Each of those paths
  already fired `logFSMEvent(fsm.EventUpdateMsg)`, so the reset happens
  automatically now.
- `handleOpen` / `processOpen` tail reset stays -- they restart with the
  **negotiated** hold value after OPEN processing, not as liveness proof.
- Two runbooks updated: `fsm-established.md` (removed the
  "handleUpdate is a no-op" RFC-deviation entry, timer table now points
  at the FSM) and `fsm-open-confirm.md` (the "subtle interaction"
  section splits keepalive-timer-start from hold-timer-reset, the
  former still in `handleKeepalive`, the latter now in the FSM).

### Out of scope (documented, not fixed)

ROUTE-REFRESH does not fire `EventUpdateMsg` and therefore does not
reset the HoldTimer on receipt. RFC 2918 does not mandate a reset, but
common practice treats any received byte as liveness. Deferred as a
separate bug.

## Section B -- gRPC domain types (the audit surprise)

The handoff framed Section B as "grep for `ze.api.v1.*Request` leaks
outside `internal/component/api/grpc/`". The expected outcome was a
list of places where proto types had leaked into the engine or other
transports.

**Zero leaks found.** `zepb` imports only in
`internal/component/api/grpc/server.go` and `server_test.go`. The
physical boundary is clean. The REST transport (`.../rest/server.go`)
has zero proto imports too; both transports call the same engine
methods with plain Go types.

The real gap, re-framed in the spec's Mistake Log: **the conversion
target does not exist**. Each transport extracts wire-format fields
inline in every handler and calls the engine with positional primitives
(`engine.Execute(auth, commandString)`, `sessions.Set(user, id, path,
value)`, etc.). Neither transport has a typed domain request like
`api.ExecuteRequest` or `api.ConfigSetRequest`. The spec proposes
adding those types, not plugging a leak.

Five open design questions preserved in the spec for `/ze-spec`:

1. Engine signature rewrite vs transport-only middle layer.
2. Name collision: `api.ConfigSetRequest` vs `zepb.ConfigSetRequest`.
3. Keep `Execute(command string)` or type the parameters end-to-end.
4. Does REST need the same convert helpers as gRPC.
5. Where `AuthContext` gets built -- transport or domain request field.

### Lesson for future audits

"Grep for leaks across a boundary" is a first step, not the whole
audit. If the grep returns empty, re-ask: does the receiving layer
have the type the leak WOULD land in? If not, the problem is the
missing layer, not the violation.

## Section C -- pull-model metrics decision (defer)

Decision doc at `plan/decision-pull-model-metrics.md`. Recommendation:
defer, not reject. The push-model plugin metrics (`ConfigureMetrics`
callback, abstract `metrics.Registry` interface in
`internal/core/metrics/metrics.go`) are correct and decoupled at the
type level. Six plugins use it today. Verified the handoff's pre-check:
zero `github.com/prometheus/` imports under `internal/component/bgp/`.

The forcing function for the pull-model refactor is **external-plugin
metrics**, not internal cleanup. `pkg/plugin/` has no metrics surface
at all -- no `Snapshot` RPC, no metrics namespace, nothing. Adopting
the pull model forces a decision about how external plugins publish
snapshots (sync RPC / streaming / in-process only), and none of the
three options is obviously right yet. Deferring until that question
ripens avoids a two-step refactor.

Revisit triggers listed in the doc: plugin count doubling, a second
export format (OpenTelemetry), a hot-path UPDATE profile showing
gauge `Inc`/`Add` significance, or the external-plugin metrics gap
becoming user-visible.

## Section D -- style bullets (nearly skipped for a glob bug)

Section D was to add five advisory bullets to a coding-standards file
if one existed. My first glob for
`**/{coding-standards,STYLE,CONTRIBUTING,contributing,go-standards}*.md`
returned nothing, which almost led to the "skip per original prompt"
branch. The file actually exists at `ai/rules/go-standards.md`
and was missed because `.claude/` is hidden and my glob pattern did
not force-match it.

Fixed by a direct `ls` of `.claude/rules/`. Added a new "Style patterns
to prefer" section with five bullets:

- Drain `time.NewTimer` on Stop (with ze-specific note: FSM uses
  `clock.Timer`+`AfterFunc` and needs no helper; only `time.NewTimer`
  site is `rs/worker.go` which already has a `drainTimer` helper).
- Slice type with methods beats wrapping struct.
- Family of narrow constructors beats `New(Config)`.
- Table-driven tests with `t.Run`.
- Honest `// TODO:` over silent gaps.

### Lesson

Glob `**/` does not descend into hidden directories like `.claude/`
by default. When searching for style / rules / config files, either
list the hidden directories explicitly or drop to a direct `ls`.

## Section A -- four queued skeleton specs

No surprises. Committed the four untracked skeletons as-is so they
stop floating across sessions:

- `spec-pkg-ze-bgp.md` -- expose BGP primitives under `pkg/ze/bgp/`.
- `spec-bgp-4-bmp.md` -- BMP receiver plugin (RFC 7854).
- `spec-rib-3-remote-client.md` -- subscribe to remote RIB as a synthetic
  peer (streaming format TBD).
- `spec-docs-1-canonical.md` -- top-level docs entry pages. Note
  the `bgp-fsm.md` page it proposes is now partially redundant with
  the per-state runbooks already shipped; the spec should be trimmed
  during its design pass.

## Late user correction: "remove bio-routing reference"

After I produced the Section B spec, Section C decision doc, and
Section D rules edit with explicit bio-routing / pattern-#N references
throughout, the user asked for them to be removed. The lesson is the
difference between the **origin** of an idea and its **internal
justification**. The bio-routing analysis is where these patterns came
from, but ze's specs and rules should stand on their own merit. A rule
that reads "we do X because bio-rd does X" is weaker than a rule that
reads "we do X because of the specific problem it solves". Internal
specs should carry the justification, not the genealogy.

The handoff itself kept the "bio-routing" title because it IS the
tracker for that research effort; the artifacts it spawned are
ze-native from the moment they land.

### Lesson for future comparison research

When adopting a pattern from external research, state the pattern in
ze's terms and give a ze-specific rationale. Keep the external origin
in the research directory (`docs/research/`) where comparison lives.
Do not sprinkle "pattern #N" references into specs, rules, or commit
messages -- they age badly and they invite "but bio-rd does it that
way" arguments instead of "but this is the best design for ze".

## Stale handoff entry (U1)

The handoff's "What changed since the last commit" section said three
FSM doc files were still uncommitted and pointed at
`tmp/commit-b4f45fc5.sh`. In fact the work had already landed as
commit `2d11673b docs(bgp-fsm): add peer-lifecycle runbook and tighten
FSM anchors` BEFORE the session started. The commit script was a
no-op. This was visible in the session-start `git log` but I did not
cross-check against U1 until the user ran the stale script.

### Lesson

A handoff's "uncommitted state" section is a claim dated to when it
was written. Before executing any commit script referenced from a
handoff, cross-check its target files against `git log --oneline` to
see if they were already committed under a matching subject line.
Prefer `git log --follow <file>` to a file-state diff for this kind
of check.

## Files touched (summary)

**Commit `e862dda4` (Section A, 4 files):**
- `plan/spec-pkg-ze-bgp.md`, `plan/spec-bgp-4-bmp.md`,
  `plan/spec-rib-3-remote-client.md`,
  `plan/spec-docs-1-canonical.md`

**Commit `6be9a0c3` (Section E, 7 files):**
- `internal/component/bgp/fsm/fsm.go` (struct + SetTimers + 3 arms)
- `internal/component/bgp/reactor/session.go` (fsm.SetTimers wiring)
- `internal/component/bgp/reactor/session_handlers.go` (2 external
  resets deleted)
- `internal/component/bgp/reactor/session_read.go` (2 external
  resets deleted)
- `docs/architecture/behavior/fsm-established.md` (rewrote the
  "handleUpdate is a no-op" section, removed the RFC-deviation entry,
  updated the timer table)
- `docs/architecture/behavior/fsm-open-confirm.md` (rewrote the
  "subtle interaction" section)
- `plan/handoff-bio-routing-followups.md` (first commit of the file --
  it was previously untracked; Section E section updated to reflect
  the implementation-based closure)

**Commit `7e0bfdd1` (Sections B/C/D, 4 files):**
- `plan/spec-arch-1-grpc-types.md` (new)
- `plan/decision-pull-model-metrics.md` (new)
- `ai/rules/go-standards.md` (added "Style patterns to prefer"
  section)
- `plan/handoff-bio-routing-followups.md` (closed sections B/C/D
  in place)

**This summary's commit (delete handoff + add summary):**
- `plan/handoff-bio-routing-followups.md` (git rm)
- `plan/learned/546-bio-routing-followups.md` (this file)

## Verification evidence

`make ze-verify` was run once this session, for commit `6be9a0c3`
(the FSM code change). **PASS** on all 8 suites including 37/37
ExaBGP compatibility tests. Log captured at
`tmp/ze-test-e862dda4.log` (deleted during session-close cleanup).

All other commits this session were docs-only (`plan/**/*.md`,
`docs/**/*.md`, `.claude/**/*.md`) and did not require `ze-verify`
per `.claude/rules/git-safety.md` Step 0.
