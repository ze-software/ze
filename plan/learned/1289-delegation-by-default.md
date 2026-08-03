# 1289 -- Delegation by default: overriding a harness guard from inside the repo

## Context

`ai/rules/planning.md` requires every spec phase to run in a subagent while
the main thread supervises. It was not happening. Some harness builds carry a
system-prompt guard from the Opus 4.6/4.7 era, *"Do not call the AgentTool unless
the user requested it"*, which a session reads as forbidding the very delegation
the repo rule mandates. The guard cannot be deleted: it is server-pushed into
`~/.claude.json`.

Two further frictions made inline work the path of least resistance even when a
session did delegate: the rule requires handing each subagent its spec, its phase
and the governing rules, which is manual work per spawn; and nothing observed
whether a session that claimed a spec ever delegated at all.

## Decisions

- **Override the guard rather than fight it.** The guard self-exempts on "unless
  the user requested it", so `ai/INSTRUCTIONS.md` now carries a STANDING REQUEST
  section stating that Thomas requests delegation in advance, in every session,
  naming the 4.6/4.7 guard explicitly and saying which one wins. It reaches every
  session through the generated `CLAUDE.md`. Chosen over editing
  `planning.md` alone, which the guard would still appear to outrank.
- **Three dispositions, not one, because the phases genuinely differ.** Skills
  that need no mid-skill user dialogue (`explore`, `audit`, `implement`, `review`,
  `review-spec`, `close`, `verify`) say "delegate, then stop". `spec` and `design`
  stay in the main thread because their gates mandate `AskUserQuestion` and a
  subagent cannot hold a dialogue. `review-deep` and `debug` stay in the main
  thread because they already fan out their own lenses; wrapping them in one agent
  would bury that parallelism a level down.
- **Make delegating cheap instead of merely mandatory.**
  `.claude/hooks/subagent-context.sh` hands every agent the parent's claimed spec
  with its Status, plus the subagent contract (cite `file:line` for the producer,
  no LSP, cannot ask the user). This works because subagents inherit
  `$CLAUDE_CODE_SESSION_ID` from the parent deliberately
  (`.claude/hooks/lib/session_id.py`), so the parent's claim marker is the
  one the agent is working under.
- **Nudge, never trap.** `mark-agent-spawned.sh` (PostToolUse on `Task|Agent`)
  records that a session delegated at least once, and `block-premature-stop.sh`
  warns at Stop when a spec was claimed and no agent was ever spawned. Exit 1, not
  2: a session may legitimately claim a spec for one mechanical edit, and a Stop
  hook that traps it is a worse failure than the miss it catches.
- **CORRECTION (2026-07-30).** That warning never fired.
  `block-premature-stop.sh` left the `Stop` event in `41e5fa44f` on 2026-06-29,
  before this summary was written. It is registered on no event today, and the
  marker still gets written for nothing. See
  `plan/learned/1306-delegation-reminder-position.md`.
- **CORRECTION (2026-07-31).** The 2026-07-30 correction above is itself stale.
  Thomas re-registered `block-premature-stop.sh` on `Stop`, first in the array.
  The warning in the bullet above now fires as written. It exits 1, never 2, when
  this session claimed a spec and spawned no agent
  (`.claude/hooks/block-premature-stop.sh`). The marker has a reader
  again. The nudge needs a CLAIMED spec, so a session that claimed none is never
  nudged, whatever it ran inline.

## Consequences

- Delegation is now the default in this repo without the user asking per session.
  The Stop-time notice for inline work never ran, because the hook that carried it
  was already unregistered. See the CORRECTION above.
- `subagent-context.sh` grew from static text to reading session state, so its
  hook timeout went 3s -> 5s (`.claude/settings.json`).
- One more per-session marker exists (`tmp/session/.agent-spawned-<sid>`), aged
  out at 24h alongside `.source-read-*` and `.lsp-invoked-*` in
  `_cleanup_stale_markers`. Ageing cannot widen any gate: every reader checks a
  far tighter window (the design-without-lsp gate uses 30 minutes).
- The model half of `ai/rules/planning.md` stays unenforceable. The `Agent`
  tool's `model` parameter selects a family (`opus`/`sonnet`/`haiku`/`fable`), so
  Opus 4.8 versus 5 cannot be pinned from inside a session. The rule already said
  so; nothing was added to it.

## Gotchas

- **`subagent-context.sh` derived its root from `$0`.** That resolves to the
  checkout the script lives in, which is the wrong tree under a worktree and
  untestable from a fixture project. It now uses `$CLAUDE_PROJECT_DIR` first,
  matching every other hook here.
- **A handoff report is a claim, not evidence.** This summary was written by the
  session that inherited the work, after re-verifying it: the fixture and golden
  counts, the `session_id.py` inheritance claim, and the presence of the settings
  wiring all held. One claim did not: the report said the model asymmetry was
  recorded in a "new Enforcement section" of `planning.md`, but that
  section is pre-existing and the file is unmodified. Verify before relaying
  (`ai/rules/evidence.md`).
- **A basename-matched guard catches innocent files.** Fixing the em dash rule in
  the user's own `~/.claude/CLAUDE.md` was blocked by `c_generated_files`, which
  matched any file named `CLAUDE.md` anywhere and told its author to edit an
  `ai/INSTRUCTIONS.md` that does not govern it. It now compares the full path
  against the project root's two generated files.
- **The standing request paid for itself immediately.** The first two reviewer
  subagents it authorised found two BLOCKERs in the same session's own work: a
  multi-line pipeline that bypassed the Bash pipe gate, and a commit-gate
  fail-open on PACKAGE-MAP. Both were in code its author had just tested, twice,
  and believed correct. That is the case `ai/rules/planning.md` makes,
  demonstrated rather than argued: the author is the one party guaranteed to
  share the blind spot that produced the bug.

## Files

- `ai/INSTRUCTIONS.md` -- STANDING REQUEST: delegate to subagents
- `ai/skills/ze-{explore,audit,implement,review,review-spec,close,verify,spec,design,review-deep,debug}.md` -- `## Delegation`
- `ai/rules/planning.md` -- reconciled with the standing request
- `.claude/hooks/subagent-context.sh` -- spec + contract injection, `$CLAUDE_PROJECT_DIR` root
- `.claude/hooks/mark-agent-spawned.sh` -- new marker
- `.claude/hooks/block-premature-stop.sh` -- delegation nudge, generic state header
- `.claude/hooks/lib/state-file.sh` -- marker ageing
- `.claude/hooks/pretool-writeedit.py` -- `c_generated_files` matches by path
- `.claude/settings.json` -- PostToolUse `Task|Agent`, subagent-context timeout
