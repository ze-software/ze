# spec-fixit-session-id-collision

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-15 |

## Task

This is a **DEV TOOLING fix (the `.claude/hooks` harness), not the ze product**. No shipped
binary, no `internal/`, no `pkg/`, no `cmd/` code is involved: the blast radius is agent
sessions, not routers.

The hooks derive a per-session id and use it to name marker files under `tmp/session/`
(`.lsp-loaded-<sid>`, `.lsp-invoked-<sid>`, `.source-read-<sid>`, `.session-<sid>`,
`session-state-<sid>.md`). Two independent implementations of that derivation exist, one in
Bash and one in Python, and they **do not agree**: they try their sources in opposite order,
and their terminal fallbacks (`claude-session-fallback` vs `str(os.getppid())`) can never
produce the same value. Markers are therefore written under one id and read under another,
and PID-derived ids collide between concurrent sessions. The result is false-positive blocks
from `c_design_without_lsp` and `block-until-lsp.sh`, a false-negative pass whenever two
sessions share the fixed fallback, and a risk of one session overwriting another's state
file. The root cause is CONFIRMED by reading both implementations (see Problem / Evidence);
the remedy is NOT yet chosen and needs research.

## Origin

Found 2026-07-15 during `spec-fixit-migrate-sleeps-infra` work, when a legitimate
`plan/spec-*.md` edit was blocked as "stale" against a different concurrent session's markers.

## Required Reading

### Source (read before designing)
- [ ] `.claude/hooks/lib/session-id.sh:27` - `_session_id()`, the Bash resolver: argv walk, then JWT, then the fixed constant `claude-session-fallback` (line 60).
- [ ] `.claude/hooks/pretool-writeedit.py:152` - `session_id()`, the Python resolver: JWT first, then `_walk_for_claude()`.
- [ ] `.claude/hooks/pretool-writeedit.py:109` - `_walk_for_claude()`: process-tree walk; returns `str(pid)` on an argv0 `claude` match (line 143), `str(os.getppid())` as terminal fallback (line 149).
- [ ] `.claude/hooks/pretool-writeedit.py:1250` - `c_design_without_lsp()`, the gate that false-positived; reads the markers at lines 1264-1267 against `LSP_FRESHNESS_SECONDS` default 1800 (line 1282).
- [ ] `.claude/hooks/pretool-writeedit.py:172` - `state_file()`; Python twin of `_state_file()`.
- [ ] `.claude/hooks/lib/state-file.sh:17` - `_state_file()`; reads `.session-<sid>`, returns `session-state-<stem>-<sid>.md` or `session-state-<sid>.md`.
- [ ] `.claude/hooks/block-until-lsp.sh:21` - sources `session-id.sh`; writes/reads `tmp/session/.lsp-loaded-<SID>`.
- [ ] `.claude/hooks/mark-lsp-invoked.sh:14` - writes `.lsp-invoked-<SID>` with `date -Iseconds`.
- [ ] `.claude/hooks/mark-source-read.sh:18` - writes `.source-read-<SID>` with `date -Iseconds`, gated on `*/internal/*.go|*/pkg/*.go|*/cmd/*.go`.
- [ ] `.claude/hooks/validate-spec.sh:71` - `REQUIRED_SECTIONS`; enforces the full implementation-spec shape on every `plan/spec-*.md` regardless of Status.
- [ ] `scripts/dev/spec-session.sh:60` - writes the `tmp/session/.session-<sid>` marker both `state_file()` implementations key off.

→ Constraint: the id must stay stable across many short-lived hook subprocesses. Both resolvers document abandoning `$PPID` for exactly this reason (`session-id.sh:11-12`, `pretool-writeedit.py:115-116`). No fix may regress to a per-subprocess value.
→ Constraint: hooks must keep working on macOS (no `/proc`, `ps`-based walk) and on Linux.
→ Decision: research must settle whether to keep two resolvers in sync or delete one, before any code is written. Two mirrored implementations are what rotted here.

### Architecture Docs
- [ ] `.claude/hooks/README.md` - the hook harness overview; must stay truthful about id derivation once this changes.
- [ ] `.claude/rules/session-start.md` - defines the BLOCKING LSP step that `block-until-lsp.sh` mechanizes.
- [ ] `.claude/rules/post-compaction.md` - depends on `session-state-<spec-stem>-<SID>.md` naming for recovery.

→ Decision: any naming change must be reflected in these three docs in the same commit (`ai/rules/discovery-updates.md`).

## Current Behavior (MANDATORY)

**Source files (cite file:line):**
- [ ] `.claude/hooks/pretool-writeedit.py` lines 153-155 - `session_id()`, **inverted order**: tries the JWT first, calls `_walk_for_claude()` only when the token env var is absent.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 142-143 - `_walk_for_claude()` returns `str(pid)` when argv0 basename is `claude`, a branch the Bash resolver does not have.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 149 and 169 - both terminal fallbacks return `str(os.getppid())`, a PID. Never equals `claude-session-fallback`.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 1264-1267 - `c_design_without_lsp` stats `.lsp-invoked-<sid>` / `.source-read-<sid>` using the **Python** id, while those files are written by mark hooks using the **Bash** id.
- [ ] `.claude/hooks/pretool-writeedit.py` line 1283 - blocks when `time.time() - max(mtimes) > fresh`; a stale foreign marker blocks, a fresh foreign marker silently passes.
- [ ] `.claude/hooks/lib/session-id.sh` lines 30-45 - `_session_id()` walks the process tree for the CLI's `--session-id` (`/proc/<pid>/cmdline` on Linux, `ps -o command=` on macOS), returns it first if found.
- [ ] `.claude/hooks/lib/session-id.sh` lines 47-56 - only if no argv id: decodes `CLAUDE_CODE_SESSION_ACCESS_TOKEN` (JWT), greps a `session_id` claim.
- [ ] `.claude/hooks/lib/session-id.sh` line 60 - terminal fallback is the literal `claude-session-fallback`, deliberately shared across sessions ("stable, unlike `$PPID`", lines 58-59).

**Behavior to preserve:**
- The gates keep firing for their real purpose: LSP loaded first (`block-until-lsp.sh`), and a spec/design write preceded by a genuine same-session source read or LSP invocation (`c_design_without_lsp`).
- Ids stay stable across short-lived hook subprocesses (no `$PPID` regression).
- `session-state-<spec-stem>-<SID>.md` naming keeps satisfying `.claude/rules/post-compaction.md` recovery and `_find_latest_state_for_spec()`.
- Hooks keep working on both macOS and Linux.

**Behavior to change:**
- None yet, research first. Direction (single shared resolver, one language, or a resolver that writes an id file both sides read) is an Open Question.

## Problem / Evidence

### CONFIRMED: two resolvers, three divergences

Read from source this session:

| Aspect | `lib/session-id.sh` (Bash) | `pretool-writeedit.py` (Python) |
|--------|---------------------------|--------------------------------|
| 1st source | argv `--session-id` walk (`:30-45`) | JWT `CLAUDE_CODE_SESSION_ACCESS_TOKEN` (`:153-168`) |
| 2nd source | JWT (`:47-56`) | argv walk via `_walk_for_claude()` (`:109-149`) |
| Extra branch | none | argv0 `claude` returns `str(pid)` (`:142-143`) |
| Terminal fallback | `claude-session-fallback` (`:60`) | `str(os.getppid())` (`:149`, `:169`) |

The precedence inversion alone makes the two disagree whenever a session has **both** a JWT
and an argv `--session-id` that differ. The fallback divergence is unconditional: a fixed
string can never equal a PID, so any session reaching both fallbacks is guaranteed to write
and read markers under different names.

### CONFIRMED: the observed collision

- `tmp/session/session-state-4427.md` exists and its first line names the spec
  `spec-vrrp-4-transport`, so it belongs to a **different, concurrent session**. Yet
  `session_id()` in `pretool-writeedit.py` resolved `4427` for the
  `spec-fixit-migrate-sleeps-infra` session too. `4427` is PID-shaped, consistent with the
  `str(os.getppid())` / `str(pid)` paths.
- `c_design_without_lsp` blocked a legitimate `plan/spec-*.md` edit with "implementation
  investigation is stale (> 1800s)" although source had just been read, because it checked
  the other session's `4427` markers.
- `block-until-lsp.sh` later blocked a Bash call with "LSP tool must be loaded before any
  other tool call" mid-session, despite LSP being loaded at session start.

### CONFIRMED: the two resolvers demonstrably disagree on disk

`tmp/session/` currently holds `.lsp-loaded-3f657872-3eeb-403c-9f2b-95282f9c203e` (a **UUID**,
ISO-8601 content, written by `block-until-lsp.sh` via the Bash resolver) alongside PID-named
`4427` markers consulted by the Python resolver. Both naming schemes coexist in one directory:
the divergence is not theoretical.

### CONFIRMED: the shared fallback leaks freshness across sessions

`.lsp-loaded-claude-session-fallback` and `.source-read-claude-session-fallback` exist and are,
by construction (`session-id.sh:60`), shared by **every** session reaching the Bash fallback.
One session's LSP load satisfies another session's gate. This false-negative is arguably worse
than the false-positive block, because nothing surfaces it.

### CONFIRMED: manual workarounds have accumulated

- Twelve symlinks of the form `.lsp-invoked-<PID> -> ./.lsp-invoked-claude-session-fallback`
  and `.source-read-<PID> -> ./.source-read-claude-session-fallback` (ids 2605, 2749, 40688,
  40694, 40763, 52264, 60632, 6503, 97940, 97941, 98058, 98112).
- `session-state-52264.md -> ./session-state-ddos-direction-allowlist-6503.md`, bridging **two
  different ids** by hand.
- `.lsp-invoked-4427` and `.source-read-4427` contain the literal text `source-read`, not an
  ISO-8601 timestamp, so they were **not** written by `mark-source-read.sh` /
  `mark-lsp-invoked.sh` (which write `date -Iseconds`). They were hand-forged to unblock the
  session. A fix should make hand-forged markers unnecessary.

### Correction to the originating brief

The brief stated the `4427` markers were "~16 hours old (the OTHER session's markers)". As read
this session they are dated 2026-07-15 16:37 and contain the literal string `source-read`. They
were evidently touched during the blocked session to work around the gate, overwriting the
evidence of the 16-hour age. The collision itself (`session-state-4427.md` belonging to
`spec-vrrp-4-transport`) is confirmed unchanged.

### UNVERIFIED

- **State-file overwrite risk.** `state_file()` (`pretool-writeedit.py:172-186`) and
  `_state_file()` (`lib/state-file.sh:17-32`) both fall back to `session-state-<sid>.md` when
  no `.session-<sid>` marker exists. With a colliding `<sid>`, two sessions resolve the same
  path. No corrupted state file has been observed, and no `.session-<sid>` markers exist in
  `tmp/session/` right now, so the fallback branch is the live one. The overwrite is a
  mechanism-level inference, not an observed event. (UNVERIFIED)
- Whether the Claude CLI always passes `--session-id` in argv on macOS, and whether
  `ps -o command=` truncates long argv before the flag is reached, which would silently push
  the Bash resolver to its shared fallback. (UNVERIFIED)
- Whether `CLAUDE_CODE_SESSION_ACCESS_TOKEN` is exported into the hook environment at all. If
  it never is, the Python resolver always takes the walk path and the precedence inversion is
  latent rather than active. (UNVERIFIED)

## Data Flow

### Entry Point
A Claude Code hook subprocess starts, one per tool call. Two entry points exist for the same
logical value: Bash hooks source `.claude/hooks/lib/session-id.sh` and call `_session_id()`
(`block-until-lsp.sh:21-22`, `mark-lsp-invoked.sh:14-15`, `mark-source-read.sh:18-19`); the
Python hook calls `session_id()` in-process (`pretool-writeedit.py:152`). Inputs available at
entry: the process tree (argv of ancestor processes), the environment variable
`CLAUDE_CODE_SESSION_ACCESS_TOKEN`, and the hook's JSON payload on stdin.

### Transformation Path
1. Resolve a session id string from argv `--session-id`, the JWT `session_id` claim, or a fallback. Bash and Python order these differently and fall back differently.
2. Interpolate that id into a marker filename under `tmp/session/`: `.lsp-loaded-<sid>`, `.lsp-invoked-<sid>`, `.source-read-<sid>`, `.session-<sid>`.
3. Write path: the mark hooks write `date -Iseconds` into the marker (Bash id).
4. Read path: `c_design_without_lsp` stats the marker mtime (Python id) and compares against `LSP_FRESHNESS_SECONDS` (default 1800, `pretool-writeedit.py:1282`).
5. Decide: allow (exit 0) or block (exit 2). Because step 3 and step 4 use different ids, the value read in step 4 is not the value written in step 3.
6. State-file path: `.session-<sid>` is read to derive a spec stem, producing `session-state-<stem>-<sid>.md` or `session-state-<sid>.md`.

### Boundaries Crossed
| Boundary | Crossing | Consequence of divergence |
|----------|----------|---------------------------|
| Bash resolver -> Python resolver | Same logical id, two implementations, no shared code | Markers written under one name, read under another |
| Hook process -> filesystem (`tmp/session/`) | Id becomes a filename; the filesystem is the only inter-subprocess state | A colliding id silently aliases two sessions' state |
| Session A -> Session B (concurrent) | Shared `tmp/session/` directory, shared fallback constant | Cross-session false-positive block and false-negative pass |
| Hook subprocess -> next hook subprocess | Id must be reproduced identically per tool call | An unstable id (PID) breaks all marker matching |

### Integration Points
- `block-until-lsp.sh` (PreToolUse, all tools): gates on `.lsp-loaded-<SID>`.
- `mark-lsp-invoked.sh` (PostToolUse, LSP), `mark-source-read.sh` (PostToolUse, Read): writers.
- `pretool-writeedit.py` `c_design_without_lsp` (PreToolUse, Write/Edit): the reader that blocked.
- `scripts/dev/spec-session.sh:60`: writes `.session-<sid>`, consumed by both `state_file()` twins.
- `.claude/rules/post-compaction.md`: consumes `session-state-<stem>-<SID>.md` for recovery.

## Wiring Test

Note: `.ci` functional tests are N/A for this spec. `.ci` covers the ze product; this is dev
tooling under `.claude/hooks`, which has no `.ci` surface. Test names below are the intended
targets and must be created by this work, since no hook test harness exists yet (Open Question).

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Bash hook sources the resolver | -> | `.claude/hooks/lib/session-id.sh` `_session_id()` | `test_bash_and_python_resolve_identical_id` |
| Python hook resolves in-process | -> | `.claude/hooks/pretool-writeedit.py` `session_id()` | `test_bash_and_python_resolve_identical_id` |
| Two concurrent sessions resolve | -> | resolver fallback path | `test_concurrent_sessions_get_distinct_ids` |
| Read `internal/**.go`, then edit `plan/spec-*.md` | -> | `c_design_without_lsp` marker read | `test_source_read_then_spec_edit_allowed` |
| Session B edits a spec, never read source | -> | `c_design_without_lsp` marker read | `test_foreign_marker_does_not_satisfy_gate` |
| Session A loaded LSP, session B did not | -> | `block-until-lsp.sh` `.lsp-loaded-<SID>` | `test_lsp_marker_is_not_shared_across_sessions` |
| Resolver called from many subprocesses | -> | `_session_id()` / `session_id()` | `test_id_stable_across_subprocesses` |
| Two sessions write state | -> | `state_file()` / `_state_file()` | `test_state_files_do_not_collide` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `test_bash_and_python_resolve_identical_id` | `.claude/hooks/tests/test_session_id.py` (new) | AC-1: both resolvers agree for argv, JWT, and fallback sources |
| `test_concurrent_sessions_get_distinct_ids` | `.claude/hooks/tests/test_session_id.py` (new) | AC-2: no shared marker or state path between two sessions |
| `test_fallback_is_per_session_or_loud` | `.claude/hooks/tests/test_session_id.py` (new) | AC-3: fallback is unique per session, or fails loudly |
| `test_source_read_then_spec_edit_allowed` | `.claude/hooks/tests/test_gates.py` (new) | AC-4: `c_design_without_lsp` passes on same-session markers |
| `test_foreign_marker_does_not_satisfy_gate` | `.claude/hooks/tests/test_gates.py` (new) | AC-4/AC-5: another session's marker neither blocks nor satisfies |
| `test_lsp_marker_is_not_shared_across_sessions` | `.claude/hooks/tests/test_gates.py` (new) | AC-5: `block-until-lsp.sh` isolation |
| `test_id_stable_across_subprocesses` | `.claude/hooks/tests/test_session_id.py` (new) | AC-6: no `$PPID`-style per-subprocess drift |
| `test_state_files_do_not_collide` | `.claude/hooks/tests/test_state_file.py` (new) | AC-7: distinct `session-state-*.md` paths |
| `test_resolver_works_without_proc` | `.claude/hooks/tests/test_session_id.py` (new) | macOS `ps` path (no `/proc`) still resolves |

### Functional Tests
N/A, not applicable: no user-facing ze feature is involved. This is dev tooling under
`.claude/hooks`, which has no `.ci` surface. The verification path is the hook test harness
above plus a manual two-concurrent-session check recorded in the Review Gate.

## Files to Modify

- `.claude/hooks/lib/session-id.sh` - the Bash resolver; the precedence and fallback fix, or deletion in favour of a single shared resolver.
- `.claude/hooks/pretool-writeedit.py` - `session_id()`, `_walk_for_claude()`, `state_file()`; align or delegate to the shared resolver.
- `.claude/hooks/lib/state-file.sh` - keep `_state_file()` consistent with its Python twin.
- `.claude/hooks/block-until-lsp.sh` - consumer; update if the resolver interface changes.
- `.claude/hooks/mark-lsp-invoked.sh` - consumer; same.
- `.claude/hooks/mark-source-read.sh` - consumer; same.
- `.claude/hooks/session-start.sh` - candidate site to mint and cache a per-session id once.
- `.claude/hooks/README.md` - document the single id derivation.
- `scripts/dev/spec-session.sh` - writes `.session-<sid>`; must use the same resolver.
- `.claude/rules/post-compaction.md` - update if state-file naming changes.

(Exact set to be confirmed by research; this is the candidate list, not a committed plan.)

## Implementation Steps

1. Research the Open Questions below, especially which id sources are actually available to hooks on macOS and Linux. Do not write code first.
2. Decide the single source of truth for the id and the fallback policy, and record the decision in the spec.
3. Decide whether to keep two resolvers or collapse to one, and measure the per-tool-call fork cost if collapsing to a subprocess.
4. Create the hook test harness and write the failing tests from the TDD Test Plan.
5. Implement the chosen resolver; make every consumer use it.
6. Align `state_file()` and `_state_file()` on the same id and naming.
7. Update `.claude/hooks/README.md`, `.claude/rules/post-compaction.md`, and `.claude/rules/session-start.md` to match.
8. Decide the migration for existing `tmp/session/` markers, symlinks, and forged files. Ask the user before deleting anything (`ai/rules/never-destroy-work.md`).
9. Verify with two genuinely concurrent sessions that gates fire independently.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Same session, Bash resolver and Python resolver both queried | Both return the identical id string, for every source (argv, JWT, fallback), on macOS and Linux |
| AC-2 | Two concurrent sessions on the same repo | They resolve two different ids; no marker or state-file path is shared |
| AC-3 | Neither argv `--session-id` nor JWT is resolvable | The fallback is still per-session unique, or the harness fails loudly rather than silently sharing state |
| AC-4 | Session reads implementation source, then edits `plan/spec-*.md` | `c_design_without_lsp` passes, consulting only that session's markers |
| AC-5 | Session A loaded LSP, session B did not | `block-until-lsp.sh` allows A and blocks B; A's marker never satisfies B's gate |
| AC-6 | Id resolved many times across distinct short-lived hook subprocesses | Identical value every time (no `$PPID`-style drift) |
| AC-7 | Two concurrent sessions each write session state | Distinct `session-state-*.md` paths; neither overwrites the other |
| AC-8 | Regression test for the resolver | An automated test asserts Bash/Python agreement and cross-session uniqueness, so this cannot silently rot again |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The CLI's `--session-id` is genuinely unique per session and present in the process tree | `session-id.sh:5-12` asserts it is the canonical session UUID; a real UUID marker (`.lsp-loaded-3f657872-...`) exists on disk | The preferred source is unusable; must fall back to a hook-minted id file | Read argv of the live CLI process on macOS and Linux | unvalidated |
| A-2 | A single shared resolver can serve both Bash and Python callers | Both implementations already try to mirror each other (`pretool-writeedit.py:116` says "Mirrors .claude/hooks/lib/session-id.sh") | Need one resolver invoked as a subprocess, costing a fork per hook call | Prototype and measure hook latency | unvalidated |
| A-3 | Nothing outside `.claude/hooks/` and `scripts/dev/spec-session.sh` derives a session id | grep of `.session-` / `session-state-` found only these | A fixed resolver leaves a third, still-diverging caller | Full-repo grep for marker-name construction | unvalidated |
| A-4 | Existing `tmp/session/` markers and symlinks may be discarded | They are hand-forged workarounds and transient session scratch | A live session loses its state file mid-flight | Confirm with the user before any cleanup (`ai/rules/never-destroy-work.md`) | unvalidated |
| A-5 | The JWT-vs-argv precedence can be settled on one order without breaking either caller | Both sources claim to yield the same session identity | The two sources disagree in a real session and the choice is behavioral | Decode both in one live session and compare | unvalidated |
| A-6 | A hook test harness can be created under `.claude/hooks/tests/` | No harness exists today; the repo uses Python for scripts (`ai/rules/go-standards.md`, Scripts: Python Only) | AC-8 has no home; the fix can rot again undetected | Confirm during research; check `make` targets for a hook test entry point | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter, unique fallback turns today's silent false-negative into a hard block for every session that cannot resolve an id | Sessions blocked at the first tool call after the change | Ship with a loud diagnostic naming the resolved id and marker path; keep a documented bypass |
| R-2 | Fixing the resolver invalidates every marker on disk, blocking all in-flight sessions at once | Concurrent sessions block immediately post-change | Land at a quiet moment, or let the gate tolerate a missing marker once and re-mark |
| R-3 | Two implementations drift again after this fix | Divergence reappears silently, exactly as now | AC-8 regression test; prefer deleting one implementation over syncing two |
| R-4 | Per-fork subprocess resolution adds latency to every hook, and hooks run on every tool call | Noticeable lag on tool calls | Cache the resolved id in a file keyed by the CLI pid, or resolve once in `session-start.sh` |
| R-5 | Deleting stale `tmp/session/` files destroys a concurrent session's state | A live session loses post-compaction recovery | Do not delete; ask the user (`ai/rules/never-destroy-work.md`) |
| R-6 | The gate is fixed but the shared-fallback false-negative is left in place, so gates keep passing wrongly and nobody notices | No signal at all, by definition | Treat AC-3 and AC-5 as first-class, not just the visible AC-4 block |
| R-7 | The fix is scoped to `c_design_without_lsp` only, leaving `block-until-lsp.sh` and the state files on the old id | Symptom recurs on a different gate | Fix the resolver, not the caller; enumerate every consumer (Files to Modify) |

## Open Questions (research before design)

- Is `--session-id` reliably present in the CLI's argv on macOS via `ps -o command=`, and does `ps` truncate the command line before reaching it? This decides whether the argv walk can be the single source of truth.
- Is `CLAUDE_CODE_SESSION_ACCESS_TOKEN` actually exported into the hook environment? If yes, does its `session_id` claim equal the argv `--session-id`? If they can differ, which is authoritative?
- Is there a first-class session identifier already available to hooks (hook stdin JSON payload, a `CLAUDE_*` env var) that both languages could read without a process walk? The stdin JSON is already parsed for `tool_name` and `tool_input` (`block-until-lsp.sh:26-31`); does it carry a session id field?
- Should the fix eliminate one implementation entirely (Bash hooks shell out to a single resolver both call) rather than keep two in sync? What is the per-call fork cost, given hooks run on every tool call?
- What should the terminal fallback be when no id resolves: a per-session id minted once in `session-start.sh` and cached in a file keyed by the CLI pid, a hard failure, or the current shared constant? Each trades a false-positive block against a false-negative pass.
- How should the transition handle markers written under the old scheme, plus the hand-made symlinks and forged markers in `tmp/session/`? Who cleans them up, and can that be done without destroying a concurrent session's state?
- Does `.session-<sid>` (written by `scripts/dev/spec-session.sh:60`) need the same treatment, and does the spec-claim workflow break if ids change mid-session?
- How is AC-8 tested? No hook test harness exists. Does one need creating, and does `ai/rules/discovery-updates.md` require registering it in a verification path so it actually runs?
- Should `validate-spec.sh:71-88` exempt `Status: skeleton` specs from the implementation-only required sections? It accepts `skeleton` as a Status (line 53) but still demands Data Flow, Wiring Test, TDD Test Plan, Files to Modify, Implementation Steps, and Checklist, which a pre-research skeleton cannot honestly fill. This is adjacent dev-tooling friction found while filing this spec (`ai/rules/friction-reporting.md`); it may warrant its own spec rather than scope creep here.

## Critical Review Checklist

- [ ] Registration over hardcoding: N/A for the resolver itself (dev tooling, not a ze feature, no core registry involved). But the fix must not hardcode a per-consumer id derivation: every hook discovers the id from one shared resolver rather than each spelling its own fallback, which is the same "one source of truth, consumers discover it" principle (`ai/rules/plugin-self-containment.md`).
- [ ] Every consumer of the id enumerated and migrated, not just the gate that surfaced the bug.
- [ ] No `$PPID`-style unstable id reintroduced.
- [ ] macOS and Linux both verified.
- [ ] No hand-forged marker required to pass any gate after the fix.

## Checklist

### Goal Gates
- [ ] Research complete: every Open Question answered and recorded in the spec.
- [ ] Root cause fixed at the resolver, not worked around at the gate (`ai/rules/no-workarounds-for-missing-behavior.md`).
- [ ] Tests written (hook test harness created, tests from the TDD Test Plan).
- [ ] Tests FAIL before the fix (they reproduce the Bash/Python divergence and the collision).
- [ ] Tests PASS after the fix.
- [ ] AC-1 through AC-8 each implemented, tested, and verified.
- [ ] Two concurrent sessions verified by hand: gates fire independently.

### Quality Gates
- [ ] `make ze-test` green (no ze product code changed; confirms no collateral damage).
- [ ] `make ze-lint-changed` green (`ai/rules/lint-gate.md`).
- [ ] `.claude/hooks/README.md`, `.claude/rules/post-compaction.md`, `.claude/rules/session-start.md` updated to match the new derivation (`ai/rules/discovery-updates.md`).
- [ ] Stale `tmp/session/` markers, symlinks, and forged files handled; user asked before any deletion (`ai/rules/never-destroy-work.md`).
- [ ] Review Gate: 0 BLOCKER, 0 ISSUE.
