# 1162 -- Session id shared marker

## Context

Every agent-guard marker is keyed by session id: `.lsp-loaded-<sid>`,
`.lsp-invoked-<sid>`, `.source-read-<sid>`, `.session-<sid>` (the spec claim) and
`session-state-<stem>-<sid>.md`. `_session_id` had three strategies -- `--session-id`
from the process tree, the JWT access token, then a fixed constant -- and on a normal
interactive machine ALL of them missed: an interactive `claude` carries no
`--session-id` in argv, and subscription auth issues no access token. So every
concurrent session fell through to the shared constant `claude-session-fallback` and
they all shared ONE marker set. `spec-session.sh`'s documented guarantee ("no two
sessions ever write the same path") inverted: `claim` silently OVERWROTE whatever spec
another session had claimed. Observed 2026-07-16 -- a spec claim was replaced by a
concurrent session's within two minutes, and the write hook then demanded session
state for the OTHER session's spec.

## Decisions

- Read `$CLAUDE_CODE_SESSION_ID` FIRST, over the process-tree walk: the CLI exports it
  into every process it spawns, so it reaches each short-lived hook subprocess with no
  `ps` walk, no argv truncation risk and no parsing. The tree walk stays as a fallback
  for CLIs that do not export it.
- REJECT an id that is not a safe filename component (`[A-Za-z0-9._-]`) rather than
  sanitize it: bash and Python must agree exactly, and two independent rewrite rules
  would drift. Rejecting falls through identically in both.
- Fixed BOTH resolvers (`lib/session-id.sh` and the port in `pretool-writeedit.py`)
  in one change; they key the same files and only ever agree by construction.
- Added a `session-id` section to `hook-fixture-check.py` over writing prose: the
  "orders MUST stay identical" invariant already existed as a docstring, and prose
  cannot fail a build. It had already been violated once.
- Wired `ze-hook-test` into `stagesForMode` (both modes) and into the required-stage
  test, over leaving it manual-only. Chose both modes over changed-only: it runs in
  ~2s and its failure mode is silent.

## Consequences

- Concurrent sessions now get distinct markers, so `spec-session.sh claim` is finally
  as collision-safe as its header always claimed.
- Changing the session id ORPHANS markers written under the old one. Every in-flight
  session gets blocked once (the LSP gate fires) and self-heals by re-loading LSP.
  Expect the same one-time disruption from any future change to id derivation.
- `make ze-hook-test` now runs under `make ze-verify`, which is the only step
  `.woodpecker/verify.yml` invokes -- so all 157 hook checks are finally in CI.
- Touching either resolver now requires touching both; the test enforces it.

## Gotchas

- **The fallback was not a safe default, it was the bug.** A constant is stable, which
  is what the comments optimized for ($PPID is unstable), but stable-and-shared silently
  breaks the one property the markers exist to provide. "Stable" and "per-session" are
  different requirements; the code satisfied only the first.
- **Editing a live-sourced hook lib affects every running session immediately.** A hook
  that sourced `session-id.sh` mid-Edit produced a 0-byte `.lsp-loaded-` (empty sid
  suffix) that no code path can produce. Expect transient artifacts when editing
  `.claude/hooks/lib/*`.
- **The fix locks you out of your own session.** `_session_id` changes -> the LSP gate
  cannot see the marker written under the old id -> Bash AND Edit are blocked, so you
  cannot even run the documented `touch` bypass. Escape by re-invoking
  `ToolSearch query="select:LSP"`, which re-writes the marker under the new id.
  Do NOT hand-touch `.lsp-invoked`/`.source-read` to escape: those are evidence that
  the producing code was read, and forging them defeats the no-fabrication gate.
- **`ze-hook-test` was reachable only by hand.** It was absent from `ze-test`, from
  `stagesForMode` and from `.woodpecker/`. The Makefile already warned about the
  mirror-image trap (dead `_impl` targets that drifted); a gate can be dead from either
  side. Check a new gate is actually REACHED, not just that it passes.
- `ls tmp/session/` hides dotfiles -- every marker is a dotfile. Use `ls -a` before
  concluding a marker is missing (or that you deleted them all).

## Files

- `.claude/hooks/lib/session-id.sh` - strategy 1 env lookup, `_sid_safe` validation
- `.claude/hooks/pretool-writeedit.py` - `_sid_from_env`, `_sid_safe`, same order
- `scripts/dev/hook-fixture-check.py` - new `session-id` parity section (7 checks)
- `scripts/status/verify_run.go` - `ze-hook-test` in both stage lists
- `scripts/status/verify_run_test.go` - `ze-hook-test` in required stages
- `scripts/dev/spec-session.sh` - header corrected; the guarantee is a property of the id
- `ai/rules/repo-maintenance.md` - documents the two resolvers and their parity test
- `Makefile` - `ze-hook-test` comment lists the session-id section
