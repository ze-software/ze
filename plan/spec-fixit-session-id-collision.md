# spec-fixit-session-id-collision

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-17 |

## Task

This is a **DEV TOOLING fix (the `.claude/hooks` harness), not the ze product**. No shipped
binary, no `internal/`, no `pkg/`, no `cmd/` code is involved: the blast radius is agent
sessions, not routers.

The hooks derive a per-session id and use it to name marker files under `tmp/session/`
(`.lsp-loaded-<sid>`, `.lsp-invoked-<sid>`, `.source-read-<sid>`, `.session-<sid>`,
`session-state-<sid>.md`). ~~Two independent implementations of that derivation exist, one in
Bash and one in Python, and they **do not agree**: they try their sources in opposite order,
and their terminal fallbacks (`claude-session-fallback` vs `str(os.getppid())`) can never
produce the same value. Markers are therefore written under one id and read under another,
and PID-derived ids collide between concurrent sessions.~~

→ **SUPERSEDED 2026-07-16 by the design-phase research below.** The Bash/Python precedence
inversion described above was **already fixed** in commit `79cc5c588` ("fix(hooks): resolve
session id the same way the writer does"), which rewrote the Python resolver to
`_sid_from_process_tree() or _sid_from_jwt() or SESSION_ID_FALLBACK`
(`pretool-writeedit.py:193`) with the same `claude-session-fallback` constant
(`pretool-writeedit.py:113`). Both hook resolvers now agree, and were **measured agreeing
live** this session. The skeleton's premise is stale; do not implement against it.

**The real, still-live defect is the third source of preference: the terminal fallback
itself.** Neither of the two preferred id sources exists in a normal interactive session on
this machine: the Claude CLI's argv carries **no `--session-id`** (measured, see Problem /
Evidence), and `CLAUDE_CODE_SESSION_ACCESS_TOKEN` is **not set**. Both resolvers therefore
reach `session-id.sh:60` / `pretool-writeedit.py:113` and return the *fixed literal*
`claude-session-fallback` for **every** session on the box. Two resolvers agreeing on one
shared constant is not a fix, it is the collision: every concurrent session reads and writes
one marker set. The failure is now the silent one (a false-negative pass: session A's LSP
load satisfies session B's gate), not the loud one. A third, still-divergent derivation
survives in `scripts/dev/commit_helper.py:172-205`.

The root cause is CONFIRMED by reading the producing functions and by live measurement (see
Problem / Evidence). The remedy is proposed below (→ Decision, "Remedy") and needs Thomas's
approval before implementation.

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

**Source files, as the code stands 2026-07-16 (cite file:line).** Read this list, not the
struck-through one further down: the skeleton's description of this code is stale.

→ **SUPERSEDED 2026-07-17 for line numbers and for the "constant is the live path" narrative.**
Commit `6254969e4` (see the PARTIALLY LANDED banner in Proposed Design) added source-1
`CLAUDE_CODE_SESSION_ID` and shifted the code below. Corrected citations, verified 2026-07-17:
`session_id()` now at `pretool-writeedit.py` line 197 (was :193); source-1 `_sid_from_env()` at
:125; the constant `SESSION_ID_FALLBACK` at :113 and `session-id.sh` line 90 (was :60) — and
it is **no longer the live path**, because source-1 resolves first on this machine;
`c_design_without_lsp` now at :1311 (sid at :1319), the other `session_id()` sites at :1368,
:1407, :1438 (were :1331, :1370, :1401); `commit_helper.py` `claude_session_fingerprint()` now
at lines 176-210, naming at :218. **Reversed claim:** every statement below that
`CLAUDE_CODE_SESSION_ID` is read by no code, and that both resolvers measure
`claude-session-fallback` live, is now FALSE — both resolvers read the env var first and return
the session UUID. Retained below as filing-time design history.

*The two hook resolvers, which now AGREE (this is no longer the bug):*
- [ ] `.claude/hooks/pretool-writeedit.py` line 193 - `session_id()` returns `_sid_from_process_tree() or _sid_from_jwt() or SESSION_ID_FALLBACK`. **Same three sources, same order** as the Bash resolver.
- [ ] `.claude/hooks/pretool-writeedit.py` line 113 - `SESSION_ID_FALLBACK = "claude-session-fallback"`. **Identical literal** to `session-id.sh:60`.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 116-152 - `_sid_from_process_tree()`: `/proc`-then-`ps` walk, `--session-id` extraction via `_session_id_from_argv` (lines 99-106). No PID branch remains.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 155-173 - `_sid_from_jwt()`: same base64url payload fix-up and same `"session_id"` regex as `session-id.sh:50-54`.
- [ ] `.claude/hooks/pretool-writeedit.py` lines 176-192 - the `session_id()` docstring records the 2026-07-16 incident and states the order "MUST stay identical to `.claude/hooks/lib/session-id.sh`". The invariant is prose only; **nothing enforces it** (what AC-8 must fix).
- [ ] `.claude/hooks/lib/session-id.sh` lines 27-61 - `_session_id()`, the Bash resolver. Precedence: (1) walk the process tree for the CLI's `--session-id`, `/proc/<pid>/cmdline` on Linux, `ps -o command=` on macOS (lines 30-45); (2) decode `CLAUDE_CODE_SESSION_ACCESS_TOKEN`, grep its `session_id` claim (lines 47-56); (3) **`echo "claude-session-fallback"` (line 60)**, the fixed literal, documented at lines 58-59 as "not per-session, but stable -- unlike `$PPID`".

*The line that produces the collision:*
- [ ] `.claude/hooks/lib/session-id.sh` line 60 - `echo "claude-session-fallback"`. This single line is the root cause. It is reached whenever neither preferred source resolves, which on this machine is **always** (see Problem / Evidence). Every concurrent session then names every marker identically.

*The THIRD derivation, still divergent (never touched by `79cc5c588`):*
- [ ] `scripts/dev/commit_helper.py` lines 172-205 - `claude_session_fingerprint()`, the **pre-`79cc5c588` Python logic, still live**. Diverges from both hook resolvers on all three axes: (1) JWT **first** (line 183), not the argv walk; (2) never looks for `--session-id` at all, instead matches `comm=` basename `claude` and returns `str(pid)` (lines 198-200); (3) terminal fallback `str(os.getppid())` (line 205), not the constant.
- [ ] `scripts/dev/commit_helper.py` line 213 - the fingerprint names `tmp/commit-session-id-<fingerprint>`, so a collision silently overwrites another session's prepared commit script. Lines 174-177 record that exact incident (2026-06-10).
- [ ] **Measured live this session:** `claude_session_fingerprint()` returns `28182` (the CLI PID) while both hook resolvers return `claude-session-fallback`. Two different answers to "which session am I", in one process tree, at one instant.

→ **SUPERSEDED 2026-07-16.** The three rows below described the code as of the skeleton's
filing. Commit `79cc5c588` ("fix(hooks): resolve session id the same way the writer does")
replaced `_walk_for_claude()` with `_sid_from_process_tree()`, removed the argv0-`claude`→PID
branch, removed the `os.getppid()` fallbacks, and adopted the Bash order and the Bash constant.
Verified by reading the current file, not by trusting the commit message.

- ~~`.claude/hooks/pretool-writeedit.py` lines 153-155 - `session_id()`, **inverted order**: tries the JWT first, calls `_walk_for_claude()` only when the token env var is absent.~~
- ~~`.claude/hooks/pretool-writeedit.py` lines 142-143 - `_walk_for_claude()` returns `str(pid)` when argv0 basename is `claude`, a branch the Bash resolver does not have.~~
- ~~`.claude/hooks/pretool-writeedit.py` lines 149 and 169 - both terminal fallbacks return `str(os.getppid())`, a PID. Never equals `claude-session-fallback`.~~

*The gate that consumes the id:*
- [ ] `.claude/hooks/pretool-writeedit.py:1282` - `c_design_without_lsp()` resolves `sid = session_id()`.
- [ ] `.claude/hooks/pretool-writeedit.py:1288-1291` - stats `.lsp-invoked-<sid>` and `.source-read-<sid>`; **either** marker satisfies the gate.
- [ ] `.claude/hooks/pretool-writeedit.py:1298-1305` - blocks (exit 2) when neither marker exists.
- [ ] `.claude/hooks/pretool-writeedit.py:1306-1307` - blocks when `time.time() - max(mtimes) > fresh`, `fresh` from `LSP_FRESHNESS_SECONDS`, default `1800`. A *fresh foreign* marker silently passes; a *stale foreign* marker blocks.
- [ ] `.claude/hooks/pretool-writeedit.py:1331`, `:1370`, `:1401` - three further `session_id()` call sites in the same file (additional gates keyed on the same id).

*State-file naming, the two twins:*
- [ ] `.claude/hooks/lib/state-file.sh:17-32` - `_state_file()`: reads `tmp/session/.session-<sid>`; returns `session-state-<stem>-<sid>.md` when a spec is claimed (line 28), else `session-state-<sid>.md` (line 30).
- [ ] `.claude/hooks/pretool-writeedit.py:196-210` - `state_file(sid)`: the Python twin, same two branches (lines 207-209, 210). These two agree today.
- [ ] `.claude/hooks/lib/state-file.sh:61-68`, `:71-76` - `_claim_spec()` / `_release_session()` both re-derive the id via `_session_id()`.
- [ ] `.claude/hooks/lib/state-file.sh:80-100` - `_cleanup_stale_markers()` deletes `.session-*` markers older than 24h and orphaned state files; it parses the sid back out of a filename with `sid="${fname##*-}"` (line 93), which mis-parses a UUID sid (it yields the last hyphen-separated group only).

*The eleven consumers of the Bash resolver (every one must migrate together):*
- [ ] `.claude/hooks/block-until-lsp.sh:21-22` - sources `session-id.sh`, `SID=$(_session_id)`; gates every tool call on `.lsp-loaded-<SID>` (line 24, tested line 43).
- [ ] `.claude/hooks/mark-lsp-invoked.sh:14-15,19` - writes `date -Iseconds` into `.lsp-invoked-<SID>`.
- [ ] `.claude/hooks/mark-source-read.sh:18-19,26-31` - writes `date -Iseconds` into `.source-read-<SID>`, only for `*/internal/*.go|*/pkg/*.go|*/cmd/*.go`.
- [ ] `.claude/hooks/session-start.sh:23` - `SID_CHECK=$(_session_id)`, reads `.session-<SID>` for the claimed spec.
- [ ] `.claude/hooks/compaction-reminder.sh:13`, `.claude/hooks/pre-compact-save.sh:14`, `.claude/hooks/session-end-summary.sh:14`, `.claude/hooks/block-premature-stop.sh:67` - all `_session_id()`.
- [ ] `.claude/hooks/lib/state-file.sh:19,64,73` - three internal call sites.
- [ ] `scripts/dev/spec-session.sh:28,59` - sources `state-file.sh`, `sid=$(_session_id)`; writes/reads `.session-<sid>`. Its header (lines 9-11) asserts "the session id is derived the same way the hooks derive it", which is true **because it literally sources the same library**. This is the one consumer that is correct by construction.

*The THIRD derivation, still divergent (never touched by `79cc5c588`):*
- [ ] `scripts/dev/commit_helper.py:172-205` - `claude_session_fingerprint()`. This is the **pre-`79cc5c588` Python logic, still live**. It diverges from both hook resolvers on all three axes: (1) JWT **first** (line 183), not the argv walk; (2) it never looks for `--session-id` at all, instead matching `comm=` basename `claude` and returning `str(pid)` (lines 198-200); (3) terminal fallback `str(os.getppid())` (line 205), not the constant.
- [ ] `scripts/dev/commit_helper.py:213` - the fingerprint names `tmp/commit-session-id-<fingerprint>`, so a collision here silently overwrites another session's prepared commit script. Line 174-177 records that exact incident (2026-06-10).
- [ ] **Measured live this session:** `claude_session_fingerprint()` returns `28182` (the CLI PID) while both hook resolvers return `claude-session-fallback`. Two different answers to "which session am I", in one process tree, at one instant.

**Behavior to preserve:**
- The gates keep firing for their real purpose: LSP loaded first (`block-until-lsp.sh`), and a spec/design write preceded by a genuine same-session source read or LSP invocation (`c_design_without_lsp`).
- Ids stay stable across short-lived hook subprocesses (no `$PPID` regression). This is the constraint that motivated the fixed constant in the first place; the fix must beat it on *both* stability and uniqueness, not trade one for the other.
- `session-state-<spec-stem>-<SID>.md` naming keeps satisfying `.claude/rules/post-compaction.md` recovery and `_find_latest_state_for_spec()` (`state-file.sh:37-57`).
- Hooks keep working on both macOS (no `/proc`) and Linux.
- `spec-session.sh`'s source-the-same-library property: whatever replaces the resolver, it must stay the *same* code, not a copy.

**Behavior to change:**
- `session-id.sh:60` / `pretool-writeedit.py:113` must stop returning a **shared constant**. The terminal fallback must be per-session unique while remaining stable across subprocesses.
- A first-class id source must be tried **before** the process walk, if A-7 confirms `CLAUDE_CODE_SESSION_ID` reaches hook processes.
- `commit_helper.py:172-205` must stop being a third spelling and use the shared resolver.

**Behavior to preserve:**
- The gates keep firing for their real purpose: LSP loaded first (`block-until-lsp.sh`), and a spec/design write preceded by a genuine same-session source read or LSP invocation (`c_design_without_lsp`).
- Ids stay stable across short-lived hook subprocesses (no `$PPID` regression).
- `session-state-<spec-stem>-<SID>.md` naming keeps satisfying `.claude/rules/post-compaction.md` recovery and `_find_latest_state_for_spec()`.
- Hooks keep working on both macOS and Linux.

**Behavior to change:**
- None yet, research first. Direction (single shared resolver, one language, or a resolver that writes an id file both sides read) is an Open Question.

## Problem / Evidence

### ~~CONFIRMED: two resolvers, three divergences~~ → SUPERSEDED 2026-07-16

~~Read from source this session:~~

| Aspect | ~~`lib/session-id.sh` (Bash)~~ | ~~`pretool-writeedit.py` (Python)~~ |
|--------|---------------------------|--------------------------------|
| ~~1st source~~ | ~~argv `--session-id` walk (`:30-45`)~~ | ~~JWT `CLAUDE_CODE_SESSION_ACCESS_TOKEN` (`:153-168`)~~ |
| ~~2nd source~~ | ~~JWT (`:47-56`)~~ | ~~argv walk via `_walk_for_claude()` (`:109-149`)~~ |
| ~~Extra branch~~ | ~~none~~ | ~~argv0 `claude` returns `str(pid)` (`:142-143`)~~ |
| ~~Terminal fallback~~ | ~~`claude-session-fallback` (`:60`)~~ | ~~`str(os.getppid())` (`:149`, `:169`)~~ |

→ **Reason superseded:** commit `79cc5c588` already collapsed this divergence. Re-read
2026-07-16, the two hook resolvers are now identical in order and in constant, and were
**measured returning the same string live** (both `claude-session-fallback`). Retained as
design history: the skeleton was filed against pre-`79cc5c588` code.

### CONFIRMED: the two hook resolvers now agree, on the WRONG value

Measured live this session, from a subprocess inside a real Claude session:

| Probe | Result |
|-------|--------|
| `source .claude/hooks/lib/session-id.sh; _session_id` | `claude-session-fallback` |
| `pretool-writeedit.session_id()` | `claude-session-fallback` |
| This session's actual state file on disk | `tmp/session/session-state-claude-session-fallback.md` |

Agreement is not correctness. Both resolvers agree on a **shared constant**, which is exactly
the collision: `session-id.sh:60` is not a rare last resort, it is **the live path**.

### CONFIRMED: both preferred id sources are absent, so the fallback is the ONLY path

This is the keystone fact, and it is measured, not inferred:

| Preferred source | Expected by resolver | Actually present? | Evidence |
|------------------|----------------------|-------------------|----------|
| argv `--session-id` | `session-id.sh:30-45` walks for it | **NO** | The live CLI process (pid 28182) argv is exactly `/Users/thomas/.local/bin/claude --dangerously-skip-permissions --model claude-opus-4-8[1m] --effort xhigh`. No `--session-id`. A second concurrent CLI (pid 13491) likewise: `... --effort xhigh --resume`. No `--session-id`. |
| JWT `CLAUDE_CODE_SESSION_ACCESS_TOKEN` | `session-id.sh:47-56` decodes it | **NO** | `env \| grep -c CLAUDE_CODE_SESSION_ACCESS_TOKEN` returns `0`. |

An interactively launched `claude` simply does not carry `--session-id` in argv. The flag is
only present when a caller passes it explicitly. **Both preferred sources miss by design, not
by accident**, so every ordinary session lands on the constant.

### BROKEN: the `ps` truncation hypothesis

The skeleton's Open Questions suspected `ps -o command=` of truncating argv before reaching
the flag, silently pushing the Bash resolver to its fallback. **Disproved by measurement:**
`ps -o command= -p 28182` returns 106 characters, complete; `ps` rendered another CLI's
1000+ character argv in full. `ps` is not truncating. The flag is genuinely not there. This
matters because it kills the tempting "just make the walk more robust" fix: the walk is
working perfectly and finding nothing, because there is nothing to find.

### CONFIRMED: two concurrent sessions sharing one marker set, right now

Two independent top-level CLI processes are live on this machine (pid 28182 and pid 13491),
neither with `--session-id`, neither with a JWT. Both therefore resolve
`claude-session-fallback`. On disk, `.lsp-loaded-claude-session-fallback` and
`.source-read-claude-session-fallback` are both stamped `Jul 16 11:00`, shared by both.
Additionally, **11 concurrent subagents are running inside this session**, all inheriting the
same resolution. One session's LSP load satisfies every other session's gate, silently.

### CONFIRMED: a third derivation still disagrees with the other two

`scripts/dev/commit_helper.py:172-205` was never migrated by `79cc5c588`. Measured live, in
the same process tree, at the same instant:

| Implementation | Live value | Names |
|----------------|-----------|-------|
| `session-id.sh:60` (Bash hooks) | `claude-session-fallback` | `tmp/session/.lsp-*`, `.session-*`, `session-state-*` |
| `pretool-writeedit.py:193` (Python hook) | `claude-session-fallback` | reads the same markers |
| `commit_helper.py:205` (commit scripts) | `28182` | `tmp/commit-session-id-28182` |

Two answers, three implementations. `commit_helper.py` is not currently *broken* (a PID is at
least per-session unique, which is why its 2026-06-10 incident stayed fixed), but it is a
third spelling of the same question and it will rot the same way. It is in scope precisely
because the spec's thesis is "one derivation, not N".

### CONFIRMED: a first-class session id exists and no code reads it

`CLAUDE_CODE_SESSION_ID=8d3d7c6b-fbad-4077-8f06-4678828041d0` is present in the environment of
a tool subprocess of the CLI. It is **the real session UUID**: it matches this session's
scratchpad path (`/private/tmp/claude-502/.../8d3d7c6b-fbad-4077-8f06-4678828041d0/scratchpad`)
exactly. A repo-wide grep for `CLAUDE_CODE_SESSION_ID` returns **zero hits**: no hook, no
script reads it. The resolver walks the process tree and decodes a JWT to reconstruct a value
the CLI is already handing it for free.

Its provenance is **injected, not inherited**: pid 13491 (a top-level CLI) carries only
`CLAUDE_CODE_FORK_SUBAGENT=1` in its own environment and no `CLAUDE_CODE_SESSION_ID`, while a
subprocess the CLI spawns does carry it. The CLI sets it *for its children*. This is the same
mechanism hooks already depend on for `CLAUDE_PROJECT_DIR` (`block-until-lsp.sh:19`).

→ **Caveat, deliberately not glossed over (A-7).** The hook environment and the Bash-tool
environment are demonstrably **not the same set**: `CLAUDE_PROJECT_DIR` is *unset* in a Bash
tool subprocess yet is relied on by hooks, so the two are populated differently. Observing
`CLAUDE_CODE_SESSION_ID` in a Bash-tool subprocess therefore does **not** prove it reaches a
hook subprocess. That is an unvalidated assumption (A-7) and it is the single fact the whole
proposed design rests on. It must be measured before any code is written, per
`ai/rules/no-fabrication.md`: read the producer, do not infer from a neighbour.

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

## Proposed Design (needs Thomas's approval)

→ **ADOPTED AS PLAN OF RECORD (readiness pass, 2026-07-17).** This design is the plan of
record; a fresh implementer builds to it and may start without further approval. Thomas:
override any → Decision below if wrong.

→ **PARTIALLY LANDED ALREADY — do NOT re-implement these.** Commit `6254969e4`
("fix(hooks): key session markers on CLAUDE_CODE_SESSION_ID", 2026-07-16 21:13, landed AFTER
this spec was filed) implemented the keystone parts. Verified by reading current source
2026-07-17:
- **Source-1 `CLAUDE_CODE_SESSION_ID` is live in BOTH resolvers.** `_sid_from_env()` at
  `pretool-writeedit.py` line 125, tried first in `session_id()` (line 197: returns
  `_sid_from_env() or _sid_from_process_tree() or _sid_from_jwt() or SESSION_ID_FALLBACK`,
  lines 221-226); Bash equivalent inline at `session-id.sh` line 52. **A-7 is CONFIRMED** (the
  env var reaches the hook env; the shipped fix is built on it). Every Current-Behavior /
  Problem-Evidence claim that "the constant is the live path" or "no code reads
  `CLAUDE_CODE_SESSION_ID`" is now STALE — see the → SUPERSEDED note in Current Behavior.
- **The hook test harness EXISTS and is wired.** `scripts/dev/hook-fixture-check.py` (section
  `session-id`) locks the Bash writer and Python reader to one id; `make ze-hook-test`
  (Makefile line 266) runs it and it is registered in `stagesForMode`
  (`scripts/status/verify_run.go`), so it runs in verify/CI. **A-6 is CONFIRMED.** New tests
  EXTEND this file; they do NOT create a parallel `.claude/hooks/tests/` tree (see Wiring Test
  and TDD notes).
- **Fork-sharing is decided.** Both resolver docstrings and the commit message record that
  subagents/forks deliberately inherit the PARENT `CLAUDE_CODE_SESSION_ID` so a fork sees its
  parent's markers (fail-closed gates require it). Resolves the fork Open Question — see the
  → AUTONOMOUS DEFAULT there.

→ **STILL REMAINING — this is the live implementation scope.** All five verified present
2026-07-17:
1. **One shared resolver.** Two full copies still exist — `session-id.sh` `_session_id()`
   (lines 48-91) and `pretool-writeedit.py` `session_id()` (lines 197-226) — held in sync only
   by the prose invariant at `pretool-writeedit.py` lines 200-203. Collapse to one Python
   `session_id.py` + Bash shim, per the → Decision below.
2. **Minted-UUID fallback.** Source-4 is still the shared constant `claude-session-fallback`
   (`session-id.sh` line 90, `pretool-writeedit.py` line 113). Replace with a UUID minted once
   and cached by CLI-ancestor PID, per the → Decision. The existing harness assertion
   `session-id-no-source-parity` (`hook-fixture-check.py`, asserts `b4 == p4 and b4 != ""`)
   currently ENFORCES the shared constant and must be tightened to per-session uniqueness
   (AC-10/AC-11) when the minted fallback lands.
3. **`commit_helper.py` third derivation, UNCHANGED.** `claude_session_fingerprint()`
   (`scripts/dev/commit_helper.py` lines 176-210) still runs its own JWT-first / `comm=`-walk /
   `os.getppid()` logic and does NOT read `CLAUDE_CODE_SESSION_ID`. Migrate to the shared
   resolver (AC-9); it names `commit-session-id-<fingerprint>` at line 218.
4. **`_cleanup_stale_markers()` UUID re-parse.** `sid="${fname##*-}"` at `state-file.sh` line 93
   still yields only a UUID's final hyphen group. Fix for the chosen id shape (R-11).
5. **Docs.** `.claude/hooks/README.md` still has NO session-id-derivation section (grep
   confirms). Add one; re-check `.claude/rules/post-compaction.md` / `session-start.md` `<SID>`
   naming (a minted UUID is still a safe filename component, so naming is unaffected — confirm).

→ **Decision: ONE resolver, owned by Python, sourced by nobody.** Reject "keep two
implementations in sync". Two mirrored implementations are what rotted here, and the current
code proves the sync discipline fails silently: `pretool-writeedit.py:176-192` *documents in
its own docstring* that the order "MUST stay identical to `lib/session-id.sh`", and that prose
invariant still did not stop `commit_helper.py` drifting for weeks. `ai/rules/go-standards.md`
"Scripts: Python Only" (line 77) settles the language: "Do not use shell/bash for scripts. Use
Python." Python is also already a hard dependency of the hook path (`pretool-writeedit.py` runs
on every Write/Edit).

→ **Decision: the resolver becomes `.claude/hooks/lib/session_id.py`, with two faces.**
An importable `session_id()` for `pretool-writeedit.py` and `commit_helper.py` (in-process, no
fork), and a `__main__` that prints the id for Bash callers. `.claude/hooks/lib/session-id.sh`
is **deleted**, not fixed; `_session_id()` survives only as a one-line shim
(`_session_id() { python3 .claude/hooks/lib/session_id.py; }`) so the eleven existing Bash call
sites keep working unchanged. Deleting the Bash logic is the point: a shim cannot drift from
the thing it calls, whereas a copy can.

→ **Decision: precedence gains a new first source, and loses its shared constant.**

| # | Source | Why here | Status |
|---|--------|----------|--------|
| 1 | `CLAUDE_CODE_SESSION_ID` env var | Free, canonical, no walk, no decode. The CLI already hands it to us. | **Blocked on A-7** |
| 2 | argv `--session-id` walk | Existing behavior; correct when the flag is present. Keep. | Confirmed working |
| 3 | JWT `session_id` claim | Existing behavior. Keep. | Confirmed working |
| 4 | Minted UUID, cached in a file keyed by the **CLI ancestor PID** | Per-session unique AND stable across subprocesses. Replaces the constant. | Proposed |
| ~~5~~ | ~~fixed constant `claude-session-fallback`~~ | **Deleted.** | Removed |

→ **Decision: the fallback is minted, not constant.** The constant exists (per
`session-id.sh:58-59`) because `$PPID` was unstable across hook subprocesses. That reasoning is
sound but the conclusion was a false dilemma: it traded uniqueness for stability when both are
obtainable. Walk to the CLI ancestor (the technique `commit_helper.py:198-200` already uses),
use *its* PID as a **cache key** (stable across subprocesses, unique across concurrent
sessions), and store a minted UUID at `tmp/session/.sid-by-pid-<clipid>`. The PID keys the
cache; it is never itself the id, so PID reuse across reboots cannot alias a stale marker set.
This beats the constant on uniqueness and matches it on stability.

→ **Decision: no per-call fork cost worth optimising, so no `session-start.sh` cache.** The
skeleton floated minting the id once in `session-start.sh` (R-4). Reject: if A-7 confirms, the
common path is an env-var read costing nothing, and Bash hooks already fork `jq` and Python
routinely. Source 4 touches the filesystem only when sources 1-3 all miss. Premature.

→ **Constraint:** `spec-session.sh` must keep *sourcing* the shared resolver rather than
copying it (its header at lines 9-11 already claims this property; the shim preserves it).

→ **Constraint:** `_cleanup_stale_markers()` (`state-file.sh:93`) parses a sid back out of a
filename with `sid="${fname##*-}"`, which yields only the final hyphen group of a UUID. A UUID
sid therefore mis-parses today. Whatever id shape wins, this line must be revisited or the
cleanup will orphan state files.

### Resolved during design (2026-07-16)

- **State-file overwrite risk: now CONFIRMED as a shared path, still UNOBSERVED as an event.**
  `state_file()` (`pretool-writeedit.py:196-210`) and `_state_file()` (`lib/state-file.sh:17-32`)
  both fall back to `session-state-<sid>.md` when no `.session-<sid>` marker exists. The
  colliding `<sid>` is no longer hypothetical: it is `claude-session-fallback` for every
  session, and this session's state file is literally
  `tmp/session/session-state-claude-session-fallback.md`. No `.session-<sid>` marker exists in
  `tmp/session/` right now, so the collide-prone branch is the live one for all sessions
  simultaneously. Two concurrent sessions **do** resolve one path. A corrupting interleave has
  still not been *observed*, so the harm remains mechanism-level; the shared path itself is
  measured fact.
- ~~Whether the Claude CLI always passes `--session-id` in argv on macOS, and whether
  `ps -o command=` truncates long argv before the flag is reached.~~ **ANSWERED.** It does
  **not** pass `--session-id` for an interactive launch (measured on two live CLIs), and `ps`
  does **not** truncate (106 chars returned complete; a 1000+ char argv rendered in full). The
  walk is correct and finds nothing. See "BROKEN: the `ps` truncation hypothesis" above.
- ~~Whether `CLAUDE_CODE_SESSION_ACCESS_TOKEN` is exported into the hook environment at all.~~
  **ANSWERED for the tool environment:** it is **not set** (`env | grep -c` returns `0`). The
  JWT branch (`session-id.sh:47-56`) is dead code on this machine. Whether it is set in some
  other deployment (SDK, CI, remote) is unknown; the branch is retained in the proposed
  precedence rather than deleted, since it costs nothing and may be live elsewhere.

### Still UNVERIFIED (must be settled before implementation)

- **A-7, the keystone.** Whether `CLAUDE_CODE_SESSION_ID` is exported into the **hook**
  environment specifically. Proven present in a Bash-tool subprocess; the hook environment is a
  demonstrably different set (`CLAUDE_PROJECT_DIR` is set for hooks, unset for Bash tools), so
  this does not transfer. The entire proposed source-1 depends on it. Cheap to settle: dump a
  hook's `env` once and read it.
- Whether `CLAUDE_CODE_SESSION_ID` differs between two concurrent top-level sessions. Strongly
  implied (it is the session UUID, and it matches the per-session scratchpad path), but not
  directly measured: macOS `ps -Ewww` returned the other CLI's environment without a
  `CLAUDE_CODE_SESSION_ID` in it, because the CLI *injects* the var downward rather than
  carrying it. Settle by reading the var from two sessions' own hook subprocesses.
- Whether subagents/forks **should** share the parent session's id. They currently would: this
  session's 11 concurrent subagents all inherit one `CLAUDE_CODE_SESSION_ID`, and
  `CLAUDE_CODE_FORK_SUBAGENT=1` / `CLAUDE_CODE_CHILD_SESSION=1` are set alongside it. Arguably
  correct (they are one session, one LSP load), but it means one fork's source-read satisfies a
  sibling fork's gate. This is a **policy question for Thomas**, not a bug to fix silently.

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

→ **SUPERSEDED 2026-07-17: the harness now EXISTS.** `scripts/dev/hook-fixture-check.py`
(section `session-id`, run by `make ze-hook-test`, wired into `stagesForMode` in
`scripts/status/verify_run.go`) already locks the Bash writer and Python reader to one id.
Home the tests below THERE — extend the `session-id` section and add `gates` / `state-file`
sections (or assertions) — do NOT create the `.claude/hooks/tests/*.py` tree named in the TDD
Test Plan and Files to Modify; a second test-infra fork is exactly the drift this spec fights.
The `.claude/hooks/tests/test_session_id.py` / `test_gates.py` / `test_state_file.py` paths are
superseded by `scripts/dev/hook-fixture-check.py`.

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
| `test_no_shared_constant_is_ever_returned` | `.claude/hooks/tests/test_session_id.py` (new) | AC-10: with no `--session-id`, no JWT, and no `CLAUDE_CODE_SESSION_ID`, the resolver returns a unique id, never `claude-session-fallback`. **This is the test that fails against today's code** |
| `test_env_session_id_preferred_over_walk` | `.claude/hooks/tests/test_session_id.py` (new) | Source-1 precedence, if A-7 confirms. Skipped/deleted if A-7 breaks |
| `test_minted_fallback_stable_across_subprocesses` | `.claude/hooks/tests/test_session_id.py` (new) | AC-11: the CLI-ancestor-PID cache key yields the same minted UUID from repeated subprocesses (A-9) |
| `test_single_derivation_in_repo` | `.claude/hooks/tests/test_session_id.py` (new) | AC-9: greps the tree; fails if a second id derivation reappears. Guards against the `commit_helper.py` class of drift (R-3) |
| `test_commit_helper_uses_shared_resolver` | `.claude/hooks/tests/test_session_id.py` (new) | AC-9: `claude_session_fingerprint()` agrees with the hook resolver (today it returns `28182` vs `claude-session-fallback`) |
| `test_cleanup_reparses_uuid_sid` | `.claude/hooks/tests/test_state_file.py` (new) | R-11: `_cleanup_stale_markers()` recovers a full UUID sid, not its last hyphen group |

### Functional Tests
N/A, not applicable: no user-facing ze feature is involved. This is dev tooling under
`.claude/hooks`, which has no `.ci` surface. The verification path is the hook test harness
above plus a manual two-concurrent-session check recorded in the Review Gate.

## Files to Modify

Confirmed by research 2026-07-16. Every `_session_id` / `session_id` / fingerprint call site
in the repo was enumerated by grep; the list below is exhaustive, not a candidate set.

→ **AMENDED 2026-07-17.** Two entries below are superseded by the shipped state:
- **Create** — the three `.claude/hooks/tests/*.py` files are SUPERSEDED. The harness that
  landed is `scripts/dev/hook-fixture-check.py` (section `session-id`, run by `make ze-hook-test`).
  Extend it — add/adjust `session-id` assertions and new `gates` / `state-file` sections — rather
  than create a parallel `tests/` tree. `.claude/hooks/lib/session_id.py` (the single resolver)
  is still to be **created**.
- **Modify** — corrected, verified 2026-07-17: `pretool-writeedit.py` `session_id()` at :197
  (source-1 `_sid_from_env()` :125 already ADDED; still to delete the now-duplicate helpers when
  the shared resolver lands); `commit_helper.py` `claude_session_fingerprint()` at **:176-210**
  (was :172-205), naming at **:218**, still unmigrated; `state-file.sh` sid re-parse still at
  **:93**; `session-id.sh` constant now at **:90** (was :60), source-1 already ADDED at :52.
  `.claude/hooks/README.md` still has no id-derivation section (confirmed by grep) — add one.

**Create:**
- `.claude/hooks/lib/session_id.py` - **the** resolver. Importable `session_id()` plus a `__main__` that prints the id. Sources: `CLAUDE_CODE_SESSION_ID` env (pending A-7), argv `--session-id` walk, JWT claim, minted-UUID cached by CLI-ancestor PID.
- `.claude/hooks/tests/test_session_id.py` - AC-1/2/3/6 regression tests (harness does not exist; see A-6).
- `.claude/hooks/tests/test_gates.py` - AC-4/AC-5.
- `.claude/hooks/tests/test_state_file.py` - AC-7.

**Delete:**
- `.claude/hooks/lib/session-id.sh` - the Bash resolver logic. Replaced by a one-line shim; the argv walk (`:30-45`), JWT decode (`:47-56`) and the constant (`:60`) all go. Deletion over sync, per the → Decision.

**Modify:**
- `.claude/hooks/pretool-writeedit.py` - delete `_session_id_from_argv()` (`:99-106`), `SESSION_ID_FALLBACK` (`:113`), `_sid_from_process_tree()` (`:116-152`), `_sid_from_jwt()` (`:155-173`); `session_id()` (`:176-193`) imports the shared resolver. Four call sites unchanged (`:1282`, `:1331`, `:1370`, `:1401`).
- `scripts/dev/commit_helper.py` - `claude_session_fingerprint()` (`:172-205`) delegates to the shared resolver. **The third derivation; found during this research, absent from the skeleton's list.**
- `.claude/hooks/lib/state-file.sh` - keep `_state_file()` (`:17-32`) consistent; **fix the `sid="${fname##*-}"` sid re-parse at `:93`** (R-11).
- `.claude/hooks/block-until-lsp.sh` - `_session_id()` shim consumer (`:21-22`); also fix the stale bypass hint at `:58` which tells the reader to guess the id.
- `.claude/hooks/mark-lsp-invoked.sh` (`:14-15`), `.claude/hooks/mark-source-read.sh` (`:18-19`), `.claude/hooks/session-start.sh` (`:23`), `.claude/hooks/compaction-reminder.sh` (`:13`), `.claude/hooks/pre-compact-save.sh` (`:14`), `.claude/hooks/session-end-summary.sh` (`:14`), `.claude/hooks/block-premature-stop.sh` (`:67`) - consumers; all use `SID=$(_session_id)` and need no change if the shim keeps the name.
- `scripts/dev/spec-session.sh` - sources `state-file.sh` (`:28`), `sid=$(_session_id)` (`:59`). Correct by construction; verify the shim preserves it.
- `.claude/hooks/README.md` - document the single derivation (currently silent on session id entirely; grep found no mention).
- `.claude/rules/post-compaction.md`, `.claude/rules/session-start.md` - update if state-file naming changes (`ai/rules/discovery-updates.md`).

**Not modified (deliberate):**
- `.claude/worktrees/agent-af24655dd2ac354ab/**` - a worktree's own copy of the hooks. Out of scope per CLAUDE.md ("Worktree agents must not touch main", and the converse); it will pick the fix up on rebase.

## Implementation Steps

Steps 1-3 of the skeleton's list are **done** (this design phase). Revised list:

→ **UPDATE 2026-07-17: revised Steps 1-3 below are ALSO done, by commit `6254969e4`.** A-7 was
settled (env var confirmed in the hook env; source-1 shipped), A-8 relied upon (forks inherit;
the `session-id-distinct-sessions-differ` harness assertion covers top-level uniqueness), and
the harness was created and wired (`scripts/dev/hook-fixture-check.py` + `make ze-hook-test`).
The live implementation therefore starts at Step 4 and covers Steps 4-13, minus the parts source-1
already delivered: write the failing per-session-uniqueness test (tighten
`session-id-no-source-parity`), build the single `session_id.py` + Bash shim, add the minted-UUID
fallback, migrate `commit_helper.py`, fix `state-file.sh:93`, and update the docs. See the STILL
REMAINING list in Proposed Design for the exact five items.

1. **Settle A-7 before writing any code.** Dump a hook subprocess's environment to a file and read it. Confirm or break `CLAUDE_CODE_SESSION_ID` in the *hook* env (not the Bash-tool env, which is a different set). If broken, drop source-1 and proceed with sources 2-4; the fix does not depend on it (R-8).
2. **Settle A-8.** Compare `CLAUDE_CODE_SESSION_ID` across two concurrent sessions' hook subprocesses. If equal, source-1 is a new shared constant: drop it (R-9).
3. Create the hook test harness under `.claude/hooks/tests/` (A-6) and register it in a verification path so it actually runs (`ai/rules/discovery-updates.md`).
4. Write the failing tests from the TDD Test Plan. **They must fail against today's code**, reproducing the shared constant, not the (already-fixed) Bash/Python inversion.
5. Implement `.claude/hooks/lib/session_id.py`: the four-source precedence, no shared constant.
6. Reduce `.claude/hooks/lib/session-id.sh` to a shim, or delete it and inline the shim in each consumer. Prefer the shim: the eleven call sites then need no edit (A-2).
7. Point `pretool-writeedit.py` at the shared resolver; delete its four now-duplicate helpers.
8. Point `commit_helper.py:claude_session_fingerprint()` at the shared resolver (the third derivation).
9. Fix the `sid="${fname##*-}"` re-parse in `_cleanup_stale_markers()` (`state-file.sh:93`) for the chosen id shape (R-11).
10. Align `state_file()` / `_state_file()` and confirm `session-state-<stem>-<sid>.md` still satisfies `_find_latest_state_for_spec()` and `.claude/rules/post-compaction.md`.
11. Update `.claude/hooks/README.md` (currently silent on id derivation), `.claude/rules/post-compaction.md`, `.claude/rules/session-start.md`.
12. **Ask Thomas** about migrating `tmp/session/`: the existing markers, the 12 hand-made symlinks, the forged `4427` files, and critically `session-state-claude-session-fallback.md`, which is the **live state file of every currently running session** (`ai/rules/never-destroy-work.md`).
13. Verify with two genuinely concurrent sessions that gates fire independently.

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
| AC-9 | Grep the repo for session-id derivation after the fix | Exactly **one** implementation exists. `session-id.sh`'s walk/JWT/constant and `commit_helper.py:claude_session_fingerprint()`'s independent logic are both gone; every caller reaches the same code |
| AC-10 | Any session, any hook, on a machine where no `--session-id` and no JWT exist (i.e. the normal case, measured 2026-07-16) | The resolved id is **per-session unique**. The literal `claude-session-fallback` appears nowhere in the resolved output, and `grep -r claude-session-fallback` over `.claude/` and `scripts/` returns no id-producing hit |
| AC-11 | Two concurrent sessions, neither able to resolve a canonical id, both on the minted fallback | They mint two different UUIDs, and each session's id is identical across its own repeated hook subprocesses (uniqueness and stability together, the trade-off `session-id.sh:58-59` treated as forced) |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The CLI's `--session-id` is genuinely unique per session and present in the process tree | `session-id.sh:5-12` asserts it is the canonical session UUID; a real UUID marker (`.lsp-loaded-3f657872-...`) exists on disk | The preferred source is unusable; must fall back to a hook-minted id file | Read argv of the live CLI process on macOS and Linux | **BROKEN**. Measured 2026-07-16: the live CLI (pid 28182) argv is `claude --dangerously-skip-permissions --model claude-opus-4-8[1m] --effort xhigh`, **no `--session-id`**; a second concurrent CLI (13491) likewise. The flag appears only when a caller passes it explicitly. Consequence: source 1 of the current precedence is dead on an interactive session, which is *why* the constant is the live path. Drives the new source-1 (`CLAUDE_CODE_SESSION_ID`) and the minted fallback. |
| A-2 | A single shared resolver can serve both Bash and Python callers | Both implementations already try to mirror each other (`pretool-writeedit.py:116` says "Mirrors .claude/hooks/lib/session-id.sh") | Need one resolver invoked as a subprocess, costing a fork per hook call | Prototype and measure hook latency | **confirmed (design-level)**. The eleven Bash call sites all use the identical form `SID=$(_session_id)` (`block-until-lsp.sh:22`, `mark-lsp-invoked.sh:15`, `mark-source-read.sh:19`, `session-start.sh:23`, `compaction-reminder.sh:13`, `pre-compact-save.sh:14`, `session-end-summary.sh:14`, `block-premature-stop.sh:67`, `state-file.sh:19,64,73`, `spec-session.sh:59`). A one-line shim delegating to Python satisfies every one without touching a single call site. Fork cost not yet measured (see R-4, downgraded). |
| A-3 | Nothing outside `.claude/hooks/` and `scripts/dev/spec-session.sh` derives a session id | grep of `.session-` / `session-state-` found only these | A fixed resolver leaves a third, still-diverging caller | Full-repo grep for marker-name construction | **BROKEN**. `scripts/dev/commit_helper.py:172-205` (`claude_session_fingerprint()`) is a third derivation, with its own JWT-first order, its own `comm=`-based walk, and its own `os.getppid()` fallback. Measured live it returns `28182` while the hooks return `claude-session-fallback`. Exactly the "third, still-diverging caller" this row warned about. Now in Files to Modify. |
| A-4 | Existing `tmp/session/` markers and symlinks may be discarded | They are hand-forged workarounds and transient session scratch | A live session loses its state file mid-flight | Confirm with the user before any cleanup (`ai/rules/never-destroy-work.md`) | unvalidated, **and now higher-stakes**: `session-state-claude-session-fallback.md` is the LIVE state file of this session and of every other concurrent session. It cannot be deleted casually. Must ask Thomas. |
| A-5 | The JWT-vs-argv precedence can be settled on one order without breaking either caller | Both sources claim to yield the same session identity | The two sources disagree in a real session and the choice is behavioral | Decode both in one live session and compare | **moot on this machine**. Neither source resolves (`--session-id` absent, `CLAUDE_CODE_SESSION_ACCESS_TOKEN` unset, `env \| grep -c` = 0), so the two can never be compared here and the ordering between them is unobservable. Both hook resolvers already use the same order regardless (`session-id.sh:30-56`, `pretool-writeedit.py:193`). Keep both branches, order argv-then-JWT, matching today's code: zero risk, zero observable effect. |
| A-6 | A hook test harness can be created under `.claude/hooks/tests/` | No harness exists today; the repo uses Python for scripts (`ai/rules/go-standards.md`, Scripts: Python Only) | AC-8 has no home; the fix can rot again undetected | Confirm during research; check `make` targets for a hook test entry point | unvalidated. No `.claude/hooks/tests/` exists and no `make` target references one. Python-only rule confirmed at `ai/rules/go-standards.md:77`. Needs a discovery-updates registration decision (see Open Questions). → **CONFIRMED 2026-07-17 (`6254969e4`):** harness `scripts/dev/hook-fixture-check.py` (section `session-id`) created and registered via `make ze-hook-test` in `stagesForMode`. |
| A-7 | **`CLAUDE_CODE_SESSION_ID` is exported into the HOOK environment** (not merely the Bash-tool environment) | Measured present in a Bash-tool subprocess: `CLAUDE_CODE_SESSION_ID=8d3d7c6b-fbad-4077-8f06-4678828041d0`, matching this session's scratchpad path exactly. Injected by the CLI into children (absent from the top-level CLI's own env, pid 13491) | **The entire proposed source-1 collapses.** Fall back to the argv walk plus the minted-UUID fallback (sources 2-4), which still fixes the collision but loses the free canonical id | **Dump a hook subprocess's `env` to a file once and read it.** Do this FIRST, before any code | **unvalidated, KEYSTONE.** Explicitly not inferred: the hook env and the Bash-tool env are different sets (`CLAUDE_PROJECT_DIR` is set for hooks per `block-until-lsp.sh:19`, but **unset** in a Bash-tool subprocess), so presence in one does not imply presence in the other. → **CONFIRMED 2026-07-17:** `6254969e4` adopted source-1 and ships against it, so `CLAUDE_CODE_SESSION_ID` does reach the hook env. |
| A-8 | `CLAUDE_CODE_SESSION_ID` differs between two concurrent top-level sessions | It is the session UUID and matches the per-session scratchpad path | Source 1 is a *new shared constant*: same bug, better disguised. Would be worse than today, because it looks correct | Read the var from two concurrent sessions' own hook subprocesses and compare | unvalidated. Could not be measured from here: macOS `ps -Ewww` shows the other CLI's env but the var is injected downward, not carried by the CLI itself. → **RELIED UPON 2026-07-17:** `6254969e4` treats the var as per-session; distinct top-level sessions differ (harness assertion `session-id-distinct-sessions-differ`), forks intentionally share (see fork Open Question). |
| A-9 | Walking to the CLI ancestor process yields a stable, per-session-unique PID usable as a fallback **cache key** | `commit_helper.py:198-200` already does exactly this walk (`comm=` basename `claude`) and it resolved `28182` live; the CLI process outlives every hook subprocess | The minted-UUID fallback has no stable key and the constant cannot be removed | Resolve the ancestor PID from several distinct hook subprocesses in one session; assert equal | unvalidated. The walk is proven to work (`28182` measured); its *stability across subprocesses* is inherited from the CLI's lifetime but not yet asserted by test |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A stricter, unique fallback turns today's silent false-negative into a hard block for every session that cannot resolve an id | Sessions blocked at the first tool call after the change | Ship with a loud diagnostic naming the resolved id and marker path; keep a documented bypass |
| R-2 | Fixing the resolver invalidates every marker on disk, blocking all in-flight sessions at once | Concurrent sessions block immediately post-change | Land at a quiet moment, or let the gate tolerate a missing marker once and re-mark |
| R-3 | Two implementations drift again after this fix | Divergence reappears silently, exactly as now | AC-8 regression test; prefer deleting one implementation over syncing two |
| R-4 | Per-fork subprocess resolution adds latency to every hook, and hooks run on every tool call | Noticeable lag on tool calls | **Downgraded 2026-07-16.** If A-7 confirms, the hot path is an env-var read, and the Bash shim's `python3` fork replaces a resolver that already forked `ps`/`awk`/`base64` several times per call (`session-id.sh:35-54`). Likely a latency *win*, not a cost. Measure before optimising; do not pre-cache in `session-start.sh` |
| R-8 | A-7 is false and `CLAUDE_CODE_SESSION_ID` never reaches hooks, invalidating the proposed source-1 after the design is approved | The env dump (Implementation Step 1) comes back without the var | Sources 2-4 still fix the collision on their own; the minted-UUID fallback is the actual fix and does not depend on A-7. Design degrades, does not collapse. **This is why Step 1 is an env dump, before any code** |
| R-9 | `CLAUDE_CODE_SESSION_ID` is shared across concurrent sessions (A-8 false), making source-1 a new shared constant that *looks* correct | Two concurrent sessions resolve the same UUID | AC-2's two-session test catches it. If it fires, drop source-1 and rely on sources 2-4 |
| R-10 | Subagents/forks share the parent's id, so one fork's marker satisfies a sibling's gate | Cannot be observed from inside a session; only visible by design review | Surface it as a policy question for Thomas rather than fixing silently. Today's shared constant makes this true across *unrelated sessions*, which is strictly worse; forks sharing is arguably correct |
| R-11 | `_cleanup_stale_markers()` (`state-file.sh:93`) parses the sid with `sid="${fname##*-}"`, which mangles a UUID into its last hyphen group, orphaning state files | Stale state files accumulate; cleanup silently no-ops | Fix the parse alongside the resolver, or choose an id shape without hyphens. Enumerated in Files to Modify |
| R-5 | Deleting stale `tmp/session/` files destroys a concurrent session's state | A live session loses post-compaction recovery | Do not delete; ask the user (`ai/rules/never-destroy-work.md`) |
| R-6 | The gate is fixed but the shared-fallback false-negative is left in place, so gates keep passing wrongly and nobody notices | No signal at all, by definition | Treat AC-3 and AC-5 as first-class, not just the visible AC-4 block |
| R-7 | The fix is scoped to `c_design_without_lsp` only, leaving `block-until-lsp.sh` and the state files on the old id | Symptom recurs on a different gate | Fix the resolver, not the caller; enumerate every consumer (Files to Modify) |

## Open Questions

### Answered by the 2026-07-16 design phase

- ~~Is `--session-id` reliably present in the CLI's argv on macOS via `ps -o command=`, and does `ps` truncate?~~ **No, and no.** The flag is absent from an interactively launched CLI (measured on two live processes); `ps` does not truncate (106 chars, complete). The argv walk cannot be the single source of truth. (A-1 BROKEN)
- ~~Is `CLAUDE_CODE_SESSION_ACCESS_TOKEN` actually exported into the hook environment?~~ **Not set at all** on this machine (`env | grep -c` = 0). The JWT branch is dead code here. Retained anyway: it costs nothing and may be live in SDK/CI/remote deployments. (A-5 moot)
- ~~Is there a first-class session identifier already available without a process walk?~~ **Yes: `CLAUDE_CODE_SESSION_ID`**, the real session UUID (matches the scratchpad path). Zero hits in a repo-wide grep: nothing reads it. Whether it reaches *hook* processes specifically is A-7, the keystone, still open.
- ~~Should the fix eliminate one implementation entirely rather than keep two in sync?~~ **Eliminate.** → Decision recorded above: one Python resolver, Bash reduced to a shim, `commit_helper.py` migrated. Prose invariants demonstrably do not hold the line (`pretool-writeedit.py:176-192` says the orders "MUST stay identical" and `commit_helper.py` drifted anyway).
- ~~What should the terminal fallback be?~~ **A minted UUID cached by CLI-ancestor PID.** The stability-vs-uniqueness trade-off that produced the constant (`session-id.sh:58-59`) is a false dilemma: keying a cache by the CLI's PID gives both.
- ~~How is AC-8 tested? No hook test harness exists.~~ Create `.claude/hooks/tests/`, Python (`ai/rules/go-standards.md:77`). Registration in a verification path is still open, below.

### Still open: needs Thomas's decision

- **Should subagents/forks share the parent session's id?** They currently would: this session's 11 concurrent subagents inherit one `CLAUDE_CODE_SESSION_ID`, with `CLAUDE_CODE_FORK_SUBAGENT=1` / `CLAUDE_CODE_CHILD_SESSION=1` set. Arguably correct (one session, one LSP load, one investigation). But it means one fork's `.source-read` satisfies a sibling fork's gate. Policy, not a bug: state the intent explicitly rather than inherit it accidentally.
  - → **AUTONOMOUS DEFAULT (2026-07-17): forks SHARE the parent session's id.** Rationale: (a) this is already the shipped behaviour — commit `6254969e4` reads `CLAUDE_CODE_SESSION_ID`, which the CLI injects unchanged into forks, and both resolver docstrings state the choice deliberately; (b) the isolation guarantee this spec provides is between distinct **top-level** sessions (AC-2, harness `session-id-distinct-sessions-differ`), and a fork is not a distinct session; (c) the gates are fail-closed — a fork that could not see its parent's `.lsp-invoked` / `.source-read` markers would be blocked for work the session already did (the docstring at `pretool-writeedit.py:125-134` says exactly this); (d) minting a fork-distinct id would require special `CLAUDE_CODE_FORK_SUBAGENT` detection, i.e. inventing a new direction, which the readiness pass may not do. Thomas: override if forks should instead be isolated.
- **`tmp/session/` migration.** `session-state-claude-session-fallback.md` is the **live state file of every currently running session**, including this one. The 12 hand-made symlinks and the forged `4427` markers are safe to remove; the shared state file is not. Nothing gets deleted without Thomas saying so (`ai/rules/never-destroy-work.md`, A-4).
  - → **AUTONOMOUS DEFAULT (2026-07-17): delete NOTHING; let the old markers age out.** Rationale: `never-destroy-work.md` is the standing exception where asking IS required, so this is not a design gap to close silently. The safe, zero-loss path needs no migration at all: `_cleanup_stale_markers()` (`state-file.sh:80-94`) already `-delete`s `.session-*` markers older than 1440 min, and new sessions write fresh ids, so the stale constant-named and forged markers become orphans that expire on their own. Implementation Step 12 stays an explicit ask-first gate: the implementer surfaces the shared `session-state-claude-session-fallback.md` to Thomas and removes nothing until told. Thomas: say the word if you want the old symlinks/forged files swept sooner.
- **Where does the hook test harness register** so it actually runs (`ai/rules/discovery-updates.md`)? A `make ze-hook-test` target, a hook in `make ze-verify`, or CI only? An unregistered harness is how AC-8 rots.
  - → **RESOLVED 2026-07-17 (already done by `6254969e4`): `make ze-hook-test`, registered in `stagesForMode`.** The target is `Makefile:266` (`python3 scripts/dev/hook-fixture-check.py`) and `scripts/status/verify_run.go` adds it to the verify/CI stages, so it runs unattended — the exact discovery-updates requirement. Implementer action: EXTEND `scripts/dev/hook-fixture-check.py`, do not stand up a second harness.
- **Is `validate-spec.sh` in scope?** (below)
  - → **AUTONOMOUS DEFAULT (2026-07-17): OUT of scope — file it as its own friction spec.** Rationale: the smaller, self-contained option (brief's scope-decision default). The `validate-spec.sh` skeleton-exemption and `.sh`/`file:line` citation-regex complaints are real (see "Adjacent friction" below) but orthogonal to the session-id resolver; folding them in would broaden a dev-tooling fix into a spec-validator rewrite. The two warnings this spec currently trips ("Behavior to preserve" not found in the first 30 scanned lines; "Functional Tests should use table format") are non-blocking (hook exits 0) and are precisely those documented friction items. Thomas: pull it in if you'd rather fix both at once.

### Adjacent friction found while filing (`ai/rules/friction-reporting.md`)

- Should `validate-spec.sh:71-88` exempt `Status: skeleton` specs from implementation-only required sections? It accepts `skeleton` (line 53) yet still demands Data Flow, Wiring Test, TDD Test Plan, Files to Modify, Implementation Steps, and Checklist, which a pre-research skeleton cannot honestly fill. Probably its own spec, not scope creep here.
- `validate-spec.sh:92-99` only scans the **first 30 lines** of `## Current Behavior` and only accepts paths ending `.(go|py|rs|ts|js)`. Two consequences hit this spec directly: (1) a `.sh` citation does not count as a source file, which is perverse for a spec whose subject **is** shell code; (2) the canonical `file:line` form this repo mandates (`ai/rules/no-fabrication.md`) fails the regex, because `` `foo.py:193` `` does not end in `.py`. The section had to be reordered and reformatted to `` `foo.py` line 193 `` purely to satisfy the hook. The rule and the gate disagree about the house citation format.
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
