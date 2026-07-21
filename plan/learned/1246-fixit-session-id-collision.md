# 1246 -- fixit-session-id-collision

## Context

The `.claude/hooks` harness resolves a per-session `<sid>` to name markers under
`tmp/session/` (`.lsp-invoked-<sid>`, `.source-read-<sid>`, `.session-<sid>`,
`session-state-<sid>.md`). Three INDEPENDENT derivations of that `<sid>` existed --
Bash (`lib/session-id.sh`), the Python hook (`pretool-writeedit.py`), and
`scripts/dev/commit_helper.py` -- kept "identical" by prose invariant only, and they
drifted for weeks. A disagreement fails CLOSED: a hook looks for a marker a different
derivation wrote, finds nothing, and re-blocks already-done work (incident
2026-07-16). Worse, the last-resort fallback was a SHARED CONSTANT
`"claude-session-fallback"`, so every concurrent session lacking an env id collided
on ONE marker set and interfered with each other's gates. Goal: one resolver, and a
unique-per-session fallback, verified by `make ze-hook-test` (native, no product).

## Decisions

- **One resolver `.claude/hooks/lib/session_id.py`** (importable `session_id()` +
  `__main__` that prints it) **over** three copies synced by prose. Bash reaches it
  through a one-line shim (`session-id.sh` -> `python3 .../session_id.py`), so the
  ~11 Bash consumers need no change. `commit_helper.py`'s third derivation (JWT /
  `comm`-walk / `getppid`) is deleted; it now delegates via `importlib`, so the
  commit-session file keys on the SAME id the hooks use.
- **Four-source precedence: env (`CLAUDE_CODE_SESSION_ID`) -> process-tree
  `--session-id` -> JWT claim -> minted UUID.** Env-primary is load-bearing: normal
  sessions (and forks, which inherit the parent's env value deliberately, so a fork
  sees the fail-closed markers its parent wrote) are unaffected, which is why the
  change is safe to land mid-session.
- **Replace the shared constant with a per-session UUID minted once and cached at
  `tmp/session/.sid-by-pid-<clipid>`, keyed by the CLI-ancestor PID** **over** an
  atomic counter or a hash: the UUID is unique-per-session AND stable across the many
  short-lived hook subprocesses of one session. `O_EXCL` makes two racing subprocesses
  converge on one id; a cache HIT refreshes the file mtime so a live session's cache
  never ages out from under it. The PID is only the cache KEY, never the id, so PID
  reuse cannot alias a stale marker set once the cache ages out.
- **Reject-not-rewrite unsafe ids** (`_sid_safe`): an id is used only when it is a
  safe filename component (`[A-Za-z0-9._-]+`), else the resolver falls through. If one
  end rewrote an unsafe id and the other rejected it, their marker paths would diverge
  -- the exact drift this spec removes.
- **`state-file.sh` SID re-parse fixed**: `sid="${fname##*-}"` mangled every UUID into
  its final hyphen group; now a UUID-shape regex extracts the whole id (falls back to
  the last group for non-UUID ids).
- **Tests home in the existing `scripts/dev/hook-fixture-check.py` `session-id`
  section** (run by `make ze-hook-test`), NOT the `.claude/hooks/tests/*.py` tree the
  original TDD plan named -- that path was superseded.

## Consequences

- One source of truth; the three-way drift class is structurally impossible and a
  grep test pins "exactly one derivation" (`SESSION_ID_FALLBACK`/`_ps_field` gone,
  one `_session_id_from_argv`). Concurrent sessions no longer collide on a fallback
  marker set.
- The minted cache ages out at 24h (`state-file.sh`), after which PID reuse can no
  longer alias a dead session's id.
- Because env is primary, the migration is invisible to any session with
  `CLAUDE_CODE_SESSION_ID` set (the normal case).

## Gotchas

- **The failure mode is fail-CLOSED, which is why prose "MUST stay identical" was
  insufficient** -- a drifted derivation does not error loudly; it silently re-blocks
  done work. Only collapsing to one copy removes it.
- **The parity test was VACUOUS as written** (`b4 == p4 and b4 != ""`): it asserted
  both ends agree on the shared constant, so it PASSED on the buggy code. Tightening
  to require a UUID AND not-the-constant is what makes it gate (mutation: it fails
  against the old constant fallback).
- **Editing the live hooks that gate your own Write/Edit**: a syntax error in
  `pretool-writeedit.py` blocks your NEXT edit. `python3 -m py_compile` after each
  hook edit (via Bash, which is ungated) before the next Edit.
- **never-destroy-work**: `state-file.sh`'s session-state aging (`-mmin +1440`) is
  pre-existing; the SID-reparse fix makes it MORE conservative (correct marker match),
  not more aggressive. A dangling `session-state-claude-session-fallback.md` symlink
  pre-existed (its target aged out earlier) and was not deleted by this work.

## Files

`.claude/hooks/lib/session_id.py` (new), `.claude/hooks/lib/session-id.sh` (shim),
`.claude/hooks/lib/state-file.sh` (SID reparse + `.sid-by-pid` aging),
`.claude/hooks/pretool-writeedit.py` (delegate), `.claude/hooks/block-until-lsp.sh`,
`.claude/hooks/README.md` (Session Identity section), `scripts/dev/commit_helper.py`
(delegate), `scripts/dev/hook-fixture-check.py` (tightened + new session-id checks).
