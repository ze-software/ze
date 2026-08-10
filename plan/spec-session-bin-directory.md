# Spec: session-bin-directory

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | `plan/spec-session-scoped-build-artifacts.md` (supersedes one of its decisions) |
| Phase | 10/10 (0, 1, E2, 2, 3, 3a, 4, 5, E1, 6 green; AC-27 is owner-held open) |
| Deferral shard | `-` (create `plan/deferrals/session-bin-directory.md` on the first deferral) |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** Under an AI session every canonical binary is built with the session id
as a NAME suffix: `bin/ze-<sid>`, `bin/ze-test-<sid>` (`mk/session.mk`, `ZE_BIN_SUFFIX`).
Cleanup is therefore a glob over a name pattern (`reap_binaries`, in two separate files).
A name sweep is unreliable: it misses anything whose suffix is not exactly the session id,
and it cannot run for a session that died without a SessionEnd event.

**The symptom.** `bin/` holds ~600 MB of orphaned build artifacts that no sweeper claims:
`ze-rfcrev`, `ze-test-rfcrev`, `ze-webprobe`, `ze-test-webprobe`, `ze-rfcgate2-clean`,
`ze-chaos-rev81d934`, `ze-race`, `ze-linux`. This is risk R-1/R-4 of
`plan/spec-session-scoped-build-artifacts.md`, materialized.

**The goal.** A session's binaries live in that session's own dated directory, together with
everything else that session wrote, so cleanup is one directory the operator can identify by
date and remove when they choose. `bin/` holds only what a human or CI built.

- On-session: `make ze` writes `tmp/session/<YYYY-MM-DD>-<sid>/bin/ze` — a **bare name in a
  private directory**.
- Off-session (human shell, CI): `make ze` writes `bin/ze` — byte-for-byte today's behavior.

Throughout this spec, **`<session-dir>`** means `tmp/session/<YYYY-MM-DD>-<sid>`.

**Owner decisions (2026-08-02), which set the shape of this spec:**

| Question | Answer |
|----------|--------|
| Config/DB dir for a relocated binary | **Session-local `<session-dir>/etc/ze`.** Full isolation; `<repo>/etc/ze` becomes the human's alone |
| Directory | **`<session-dir>/bin`** — the existing `ZE_SCRATCH_DIR` |
| Lifetime | ~~**Deleted with the session.** One `rm -rf`; that is the point~~ **Superseded 2026-08-03** — see "Scope extension" below |

## Scope extension (2026-08-03): one session root, no automatic deletion

**The second problem.** Per-session state lives under TWO roots that grew separately, and
nothing names either one as canonical:

| Root | Shape | Holds |
|------|-------|-------|
| `tmp/s/<sid>/` | directory per session | ad-hoc logs, probes, and (per this spec) `bin/` + `etc/ze` |
| `tmp/session/` | flat files suffixed `-<sid>` | spec claim, gate markers, id cache, session state |

The split already leaks: `.claude/hooks/session-end-scratch.sh` reaches into both roots in
one hook (`:48` and `:60`).

**Owner decisions (2026-08-03):**

| Question | Answer |
|----------|--------|
| Which root survives | **`tmp/session/`.** `tmp/s` needs a doc lookup to decode (`ai/rules/go-standards.md`), and `tmp/session/` is already the root hosting the files that structurally cannot move into a per-session directory (below) |
| Directory name | **`tmp/session/<YYYY-MM-DD>-<sid>/`** — the date prefix makes manual, date-ordered cleanup possible by eye and by glob |
| Deletion | **No automatic deletion of anything under `tmp/session/`.** Not at SessionEnd, not on an age timer. Cleanup is manual, or an explicit make target the operator runs |

**Why the flat marker files stay flat, and keep their names.** Three of them cannot live
inside a per-session directory at all:

| File | Keyed by | Why it cannot move |
|------|----------|--------------------|
| `.sid-by-pid-<clipid>` | CLI-ancestor PID | It *mints* the session id (`session_id.py` source 4). A directory named after the id cannot hold the file that produces the id |
| `.closure-ack-<stem>` | spec stem | Outlives the session that wrote it; a later session on the same spec must read it |
| `session-state-<stem>-<sid>.md` | spec + session | **Superseded 2026-08-10 (see "Scope extension" below): it moved into `<session-dir>/state/`.** The reason recorded here was that it is read ACROSS sessions for handoff; a glob that walks every session directory's `state/` reads it equally well |

The remaining gate markers (`.lsp-loaded-`, `.source-read-`, `.agent-spawned-`,
`.model-ack-`, `.compaction-detected-`, `.session-`) *could* move inside, and deliberately
do not. Their failure mode is a gate that silently stops firing (R-5's shape), they are
written by ~10 separate hooks, and moving them buys nothing once automatic deletion is gone.
Not moving them keeps this change a rename plus a policy change, not a hook rewrite.

**The directory is LOOKED UP, never recomputed.** `<YYYY-MM-DD>-<sid>` is not a pure
function of the id, so every consumer resolves it the same way: take the single directory
matching `tmp/session/????-??-??-<sid>`; only on a miss create `tmp/session/<today>-<sid>`.
Recomputing from today's date would move a session's directory at midnight and orphan its
binaries mid-run. Make, Go and shell each implement this, and `TestMakeAndGoAgreeOnBinDir`
is what stops the three drifting (three independent derivations of the
session id drifted for weeks behind a prose "MUST stay identical" invariant).

**This reverses a recorded owner decision.** `plan/spec-session-scoped-build-artifacts.md`
records the suffix as chosen *over* `tmp/s/<id>/bin/`, twice, marked `owner-selected`, on the
grounds that relocation repoints config/DB resolution away from the live `<repo>/etc/ze`.
That objection is real and verified (see Current Behavior). It is not ignored here: the
owner's answer to question 1 **dissolves** it by making per-session isolation the intent
rather than the accident. The rationale must survive into the superseding record
(`ai/rules/planning.md`), which is why it is quoted in Key Design Decisions.

Non-goals: `bin/gok`, `ze-host`, and the cross-compiled installer keep their current
locations (already Known Limitations of the predecessor spec). No change to what a human or
CI sees.

## Scope extension (2026-08-10): three subdirectories, and a guard on the `tmp/` root

Two owner decisions extend this spec. Neither changes the surviving root, the directory
name, or the no-automatic-deletion policy recorded above. Both were taken after a session
wrote 351 ad-hoc files to the `tmp/` root in one day
(`plan/journal/guard-message-teaches-the-violation.md`).

**Decision 1: the session directory carries three subdirectories.**

| Subdirectory | Holds | Producer |
|--------------|-------|----------|
| `bin/` | this session's binaries, and the `etc/ze` they resolve | `mk/session.mk`, `internal/test/sessionpath` |
| `scratch/` | ad-hoc logs, probes, captures | `scripts/dev/session-scratch.sh` |
| `state/` | the per-spec digest `session-state-<stem>-<sid>.md` | `.claude/hooks/lib/state-file.sh` |

This reverses one row of the 2026-08-03 table above. That row kept the per-spec digest flat
because `_find_latest_state_for_spec` reads it ACROSS sessions. A glob that walks every
session directory reads it equally well, so the cross-session property costs nothing and the
digest joins the directory of the session that wrote it. The other two rows stand:
`.sid-by-pid-<clipid>` produces the id the directory is named for, so it cannot live inside
one, and `.closure-ack-<stem>` is keyed by spec stem rather than by session.

**Decision 2: a file at the `tmp/` root is REFUSED, not warned.** `tmp/` is keyed per
checkout, so a fixed name there is one file shared by every session in the tree, and nothing
cleans it. The guard covers both surfaces an agent creates a file with: a Bash redirect or
`tee`, and the Write tool. A path with a directory component passes; a bare `tmp/<file>`
does not.

**The guard accepts BOTH layouts through the transition.** `tmp/s/<id>/` and
`tmp/session/<YYYY-MM-DD>-<sid>/` are equally acceptable until a grep proves no producer and
no live tree writes to `tmp/s/` any more. Refusing the old layout while the rename is in
flight would block sessions that are mid-run in a shared checkout, which is the one failure
this spec must not cause. Removing the legacy acceptance is its own step, gated on that
grep, and is AC-27.

**Root files that stay.** The guard exempts the root names that are session-keyed or shared
by design: `ze-verify*`, `.ze-verify*`, `commit-*`, `commit-msg-*`, `delete-*`, `mutation*`,
`test-timings*`. These are named by tooling that predates the session directory, and moving
them is not in this spec.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-19 | any consumer resolves the session directory | it holds `bin/`, `scratch/` and `state/`; `session-scratch.sh` prints the `scratch/` path, and finds the parent by the same glob-then-create rule as AC-13 |
| AC-20 | a hook writes a per-spec digest | it lands at `<session-dir>/state/session-state-<stem>-<sid>.md`; nothing writes `session-state-*` flat under `tmp/session/` |
| AC-21 | a later session resumes a spec an earlier session digested | `_find_latest_state_for_spec` finds that digest by walking every session directory's `state/`, newest first, and its two legacy fallbacks still resolve |
| AC-22 | `.sid-by-pid-<clipid>` and `.closure-ack-<stem>` | still flat under `tmp/session/`, unmoved and unrenamed |
| AC-23 | Bash: a redirect or `tee` names `tmp/<file>` | refused, exit 2, naming `session-scratch.sh` as the route |
| AC-24 | Write or Edit: the path is `tmp/<file>` | refused, exit 2, same message |
| AC-25 | Bash or Write: the path carries a directory component under `tmp/` | allowed, for `tmp/s/<id>/…`, `tmp/session/<dated-id>/…`, and every producer-backed folder (`verify/`, `review/`, `kernel/`, `gokrazy/`, `qemu/`, `evidence/`, `stress-repro/`, `rule-coverage/`) |
| AC-26 | Bash or Write: the path is an exempt root file (`tmp/ze-verify.log`, `tmp/commit-<sid>.sh`) | allowed |
| AC-27 | grep for `tmp/s/` across producers and rules, after the rename lands | no hit; the guard's legacy acceptance of `tmp/s/` is then removed, and its fixture becomes a refusal |

**AC-27 IS HELD OPEN ON PURPOSE, by owner instruction (2026-08-10): both layouts stay
accepted "until it does not trigger as fully ironed out".** The rename landed in the working
tree during phases 2 to E1, and it is NOT committed. Sibling sessions share this checkout, so
refusing `tmp/s/` now would break a session mid-run, which the scope extension names as the
one failure this spec must not cause. `tmp/s/<id>/` is accepted BY CONSTRUCTION in
`.claude/hooks/lib/scratch_path.py`, which names no layout at all, so nothing has to be added
to keep it. AC-27 is a deliberate follow-up, gated on the commit landing and on the grep
being clean, and it is the one AC this spec closes with open. Do not let a later phase or a
review "tidy" it: removing the acceptance early is a regression, not a completion.
| AC-28 | the parity corpus and the fixture runner | every row above is pinned: allowed paths exit 0, refused paths exit 2, and both layouts appear |

**Files this extension adds to the change set.**

| File | Change |
|------|--------|
| `.claude/hooks/pretool-bash.py` | `check_scratch_path` becomes blocking, and learns both layouts |
| `.claude/hooks/pretool-writeedit.py` | the same guard on the Write and Edit surface, which today refuses only absolute `/tmp` |
| `.claude/hooks/subagent-context.sh` | the scratch line every spawned agent reads |
| `.claude/hooks/lib/state-file.sh` | the digest path, and the cross-session resolver's glob |
| `.claude/hooks/lib/session-dir.sh` | NEW: the one shell definition of the glob-then-create rule, shared by `state-file.sh` and `session-scratch.sh` |
| `.claude/hooks/pretool-writeedit.py` | `state_file()`, the reader the Go and spec gates block on, which also accepts a digest written before the move |
| `ai/rules/points/commands/**`, `ai/rules/points/testing/**`, `ai/rules/points/repo-maintenance/**` | the rule text and the check table |
| `ai/INSTRUCTIONS.md` | the dispatch row for running a test or build command |
| `scripts/dev/hook-parity-check.py`, `scripts/dev/hook-fixture-check.py` | the fixtures that pin AC-23 to AC-28 |

## Required Reading

### Architecture Docs
- [ ] `mk/session.mk` (header, lines 1-30) - the design being replaced
  → Decision: the suffix exists to keep `ConfigDirFromBinary`'s prefix at the repo root.
  → Constraint: off-session behavior must stay byte-identical; that half is not in scope.
- [ ] `mk/test-functional.mk` (`ZE_ALT_DIR`, `ZE_ALT_BIN`, `ZE_ALT_BUILD`, `ZE_TEST_RUN`) - the
  precedent this spec generalizes
  → Decision: test binaries already build **bare-named** into `$(ZE_SCRATCH_DIR)/testbin-*/bin/`
    with no symlink, and export `ZE_BIN`/`ZE_TEST_BIN`. The target layout is already live there.
  → Constraint: the final path element MUST be `bin`, or `ze` cannot resolve a config dir at all.
- [ ] `internal/test/sessionpath/sessionpath.go` - the Go half
  → Decision: `BinDir` **already returns a per-session directory** on-session and `<base>/bin`
    off-session, so the SHAPE is right and make must catch up to it.
    → **Corrected 2026-08-03:** the path it returns is `tmp/s/<id>/bin` (`:70`,
    `filepath.Join(baseDir, "tmp", "s", id)`). The root rename and the dated glob-then-create
    rule mean the Go side DOES change now. The earlier "Go needs no change" reading is void.
  → Constraint: `FindPrebuiltDir` falls back to the shared `bin/` on a miss, so a human-built
    `bin/ze` still satisfies `ZE_TEST_NO_BUILD`. Preserve that.

### RFC Summaries (Scope: protocol)
N/A — build tooling, no protocol behavior.

**Key insights:**
- The Go helper and the make variables currently disagree about where a session's binaries
  live. This spec removes the disagreement rather than adding a mechanism.
- Three separate files carry the "suffix on purpose" rationale as prose. All three must be
  rewritten, not merely edited, or the tree argues with itself.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/paths/paths.go` - `ConfigDirFromBinary` maps `<prefix>/bin/ze` →
  `<prefix>/etc/ze`; `isBinDir` requires the parent to be named `bin` or `sbin`;
  `DefaultConfigDir` prefers `ze.config.dir`, else feeds `EvalSymlinks(os.Executable())`
  (absolute) into it.
  → Constraint: `tmp/session/<sid>/bin/ze` **does** satisfy `isBinDir`, and resolves to
    `tmp/session/<sid>/etc/ze`. The binary never fails to find *a* config dir; it finds a
    *different* one. This is the whole risk surface.
- [ ] `internal/component/config/storage/blob.go` - `NewBlob`: when the blob does not exist,
  it calls `zefs.Create` and returns **`err == nil`**.
  → Constraint: a wrong config dir does not raise. It silently yields an empty store.
    Every mitigation in this spec exists because of this line.
- [ ] `internal/test/sessionpath/sessionpath.go` - `ID`, `Root`, `BinDir`, `FindPrebuiltDir`.
- [ ] `cmd/ze/dispatch.go` - `binarySuffixRoot` dispatches argv[0]'s segment after the last
  `-` as a root command.
  → Decision: today `bin/ze-perf-<sid>` breaks personality dispatch, which is why
    `test/perf/run.py` carries a bare-naming symlink shim. A bare name retires both.

**Behavior to preserve:**
- Off-session: `bin/ze`, `bin/ze-test`, … exactly as today. Humans and CI see no change.
- An unsafe or absent session id falls back to the shared paths, never to an escaping path.
- `ZE_BIN` / `ZE_TEST_BIN` overrides keep winning over any default.
- `FindPrebuiltDir`'s fallback to the shared `bin/` for `ZE_TEST_NO_BUILD`.
- `.ci` tests keep exec'ing `ze` / `ze-stripped` by bare name off one PATH entry.

**Behavior to change:**
- On-session `make ze` &co write `$(ZE_SCRATCH_DIR)/bin/<bare-name>`.
- On-session `ze` resolves its config/DB to `tmp/session/<sid>/etc/ze` (owner decision 1).
- Cleanup becomes directory removal; the name-glob sweepers are deleted.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`CLAUDE_CODE_SESSION_ID`, exported by the CLI into every child process.

### Transformation Path
1. `mk/session.mk` validates the id (charset, `.`/`..`, `/`, quote) → `ZE_SESSION_ID`.
2. `ZE_SCRATCH_DIR` = `tmp` off-session, `tmp/session/<sid>` on-session (unchanged).
3. **New:** `ZE_BIN_DIR` = `bin` off-session, `$(ZE_SCRATCH_DIR)/bin` on-session.
4. `ZEBIN_*` = `$(ZE_BIN_DIR)/<bare-name>`.
5. Go reads the same id via `sessionpath.ID`, and `sessionpath.BinDir` already computes
   the identical directory.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness → Make | exported `CLAUDE_CODE_SESSION_ID` | **Yes** — `make ze-path` resolved this session's id live |
| Make → Go | exported `ZE_SESSION_ID` (`ze.session.id`) | No — the make/Go agreement test (AC wiring row 4) is what proves it |
| Make → runner | `ZE_BIN` / `ZE_TEST_BIN` (contract unchanged) | No — contract untouched; the functional suites are the gate |
| Binary → config dir | `EvalSymlinks(os.Executable())` → `ConfigDirFromBinary` | **Yes** — prototype resolved `tmp/session/<sid>/etc/ze`, isolated from `<repo>/etc/ze` |
| Host → QEMU guest | 9p share of the repo root, mounted `/workspace` | No — **R-3 lives here**; must be proven against a symlinked `tmp/`, not reasoned about |

### Integration Points
- `scripts/dev/session-scratch.sh` — owns `tmp/session/<id>/` lifecycle; loses `reap_binaries`.
- `.claude/hooks/session-end-scratch.sh` — SessionEnd sweep; loses its `bin/*-<sid>` loop.
- `internal/test/sessionpath` — already correct; becomes the single definition of the layout.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The session id keeps its ONE resolver (`.claude/hooks/lib/session_id.py` → exported var). This spec adds no second derivation; it removes a divergence between make and `sessionpath` |
| No unintended coupling (components stay isolated) | Yes | `internal/core/paths` is NOT modified. The alternative "teach `paths.go` the layout" was rejected precisely because it would put dev-tooling knowledge in the shipped binary's resolver (Key Design Decisions) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `ZE_SCRATCH_DIR` and the layout `sessionpath.BinDir` already computes. Net deletion of four mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Build tooling; no data path |
| Registration over hardcoding | N-A | Build tooling; no registry surface. The related discipline here is `evidence.md`: `ZEBIN_*` stays derived from one `ZE_BIN_DIR`, never spelled per-binary |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `<session-dir>/bin/ze` satisfies `isBinDir`, so `ze` always resolves *a* config dir and never errors with "cannot determine database location" | `internal/core/paths/paths.go` `isBinDir`, `ConfigDirFromBinary` | binaries would be unusable, not merely isolated | read the producer; empirically reproduced by the research pass | confirmed |
| A-2 | `NewBlob` creates an empty store and returns nil error on a missing blob | `internal/component/config/storage/blob.go` `NewBlob` | a wrong dir would fail loudly and need no mitigation | read the producer | confirmed |
| A-3 | Go's build cache is keyed by source, so a per-session build only re-links | Go toolchain | every new session pays a full compile | timed prototype, 2026-08-02 | **confirmed**: 18.54 s to build, then **4.91 s** to a different `-o` path, output byte-identical (`cmp` clean). A new session pays a link, not a compile |
| A-4 | `.ci` tests exec `ze`/`ze-stripped` by bare name from one PATH entry | `runner_exec.go`, `runner.go` `childPathEnv` | bare naming would not be required | functional suite run | confirmed |
| A-5 | `tmp/` is a real directory in this checkout, so the QEMU 9p share reaches `tmp/session/` | `ls -ld tmp`; `ensure-links.py` never converts a real `tmp/` in place | QEMU targets break immediately rather than latently | `ls -ld tmp` | confirmed |
| A-6 | A session's `etc/ze` being empty is acceptable, because agents seed it or do not need it | owner decision 1 | agents lose `ze data`/`ze connect`/`ze show` silently | working prototype, 2026-08-02 | **confirmed**: seeding works and isolation holds. See Prototype Evidence |
| A-7 | `ze init` can seed non-interactively from a make recipe | `mk/gokrazy.mk` does exactly this | AC-8 needs a different mechanism | ran it, 2026-08-02 | **confirmed, with a correction**: `--yes` alone is NOT enough. `ze init --force --yes --seed` with no stdin exits 1 (`username is required`). Credentials arrive as 5 stdin lines (username, password, host, port, name), the `printf '%s\n' … \|` pattern of `mk/gokrazy.mk` |
| A-8 | `make ze-clean-tmp` already excludes `session` from its directory sweep, so moving scratch under `tmp/session/` lands inside an existing exclusion rather than needing a new one | `Makefile:747`, `-not -name session -not -name kernel` | R-4 would still be live and AC-10 would need a code change | read the producer, 2026-08-03 | **confirmed** |
| A-9 | No surviving sweep in an operator-invoked target can touch a dated session DIRECTORY | `Makefile:746` `-type f`, `:747` `-type d` with `session` excluded, `:749` `-type f` | manual cleanup would delete live sessions as a side effect | read the producer, 2026-08-03 | **confirmed** — `-type f` never matches a directory, and the one `-type d` sweep excludes `session` |
| A-10 | Process start time is readable on both supported platforms with no new dependency | `session_id.py` already carries the `/proc`-then-`ps` fallback shape (`_ppid`, `_pcomm`, `_ps`) | R-9's fix needs a different key, or the age-out must survive as a carve-out | ran both, 2026-08-03 | **confirmed** — `/proc/<pid>/stat` field 22 returned `87742107`; `ps -o lstart=` returned `Mon Aug  3 00:59:30 2026`. Either is a usable key component |
| A-11 | ~~Deleting `_cleanup_stale_markers` breaks no caller~~ | — | — | `grep -rn _cleanup_stale_markers .claude/ scripts/`, 2026-08-03 | **BROKEN.** One live caller (`session-start.sh`) and **three fixtures that invoke it by name** (`hook-fixture-check.py`, `:2281`, plus the R-11 UUID-sid case at `:1144`). One of them asserts a live session's claim must NOT age out — behaviour that becomes vacuous once nothing ages out, so the fixture must be rewritten to assert the new contract, not deleted. Two further comment references (`session-end-summary.sh`, `block-premature-stop.sh`, `commit_helper.py`) describe the sweep and go stale (`ai/rules/stale-comments.md`). Phase 3 owns all of it |
| A-12 | Nothing under `tmp/` is tracked, so dated dirs and session binaries can never be committed | `.gitignore:15` `tmp/*` | per-session SSH host keys could reach a commit | read the producer, 2026-08-03 | **confirmed** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | **A session's `ze` uses an unseeded store.** The severity SPLITS by entry point, measured not assumed: the `zefs.Open` paths (`ze data`, CLI credentials) fail **loudly** (exit 2, verified); the `NewBlob` paths (`resolve.Storage`, `ResolveSSHStorage`) create an empty store and return `err == nil` (A-2, read but not yet run) — those are the silent ones. A relocated binary also creates `etc/ze/crash/` as a side effect even on a failed command (verified) | `ze data ls` exit 2; or a daemon that starts with zero users | AC-8: `make ze` seeds the session store on first build via the `printf \| ze init` pattern (A-7), and the seeding is asserted, not assumed |
| R-2 | **A new SSH host key per session.** `ResolveSSHStorage` writes a fresh key into the session store, so pinned clients fail verification | host-key mismatch on `ze connect` | Same seeding step; documented as intended isolation in `ai/rules/commands.md` |
| R-3 | **QEMU 9p cannot see `tmp/session/` on a migrated checkout.** `ensure-links.py` can make `tmp/` a symlink to `$TMPDIR/ze/<id>`; the share covers the repo root only, so `/workspace/tmp/session/<sid>/bin/ze` would not resolve in the guest | `ze-qemu-*` fails to exec the DUT; latent today (A-5) | AC-9: share the resolved `tmp/` target as a second 9p mount when `tmp/` is a symlink. A latent silent break is the worst shape, so this is fixed, not deferred. **Closed in phase 5**: `scratch_share` and `virtfs_args` in `scripts/evidence/qemu-run.py`, mounted at the path the link names. Proven by a live QEMU boot over a fixture checkout whose `tmp/` IS a symlink: the DUT under `/workspace/tmp/session/<dated>/bin/` execs; with the second share removed the same boot cannot resolve `/workspace/tmp/` at all |
| R-4 | ~~**`make ze-clean-tmp` reaps a live session's binaries.**~~ **DISSOLVED 2026-08-03** by the root rename: `ze-clean-tmp` already excludes `session` (`Makefile:747`, `-not -name session -not -name kernel`), so moving scratch under `tmp/session/` puts it inside the existing exclusion. AC-10 becomes an assertion of something already true rather than a change | — | — |
| R-9 | **PID reuse aliases a dead session's id.** `session_id.py` source 4 caches a minted id at `.sid-by-pid-<clipid>`, keyed on the CLI-ancestor PID. Its own docstring says reuse "cannot alias a stale marker set **once the cache file ages out**" — so the 24h sweep at `state-file.sh` is CORRECTNESS, not housekeeping. Removing automatic deletion removes it, and a reused PID then makes a new session adopt a dead session's spec claim and gate markers: incident 1162/1246 reopened | a session resolves an id it never minted; a spec claim it did not make | AC-17/AC-18: key the cache on **PID + process start time** (`/proc/<pid>/stat` field 22; `ps -o lstart=` on macOS/BSD). A reused PID carries a different start time, so the entry self-invalidates and needs no expiry at all — strictly better than the timer it replaces. This is the ONLY file under `tmp/session/` whose expiry did real work; `state-file.sh` records the rest as pure accumulation control |
| R-10 | **`tmp/session/` grows without bound.** With no automatic deletion, each session leaves a dated directory (hundreds of MB once binaries move into it) plus ~7 small markers. The predecessor already measured ~600 MB of orphans under the old scheme | disk pressure; `du -sh tmp/session` | Accepted by owner decision (2026-08-03): growth is the price of never deleting the operator's data unasked. Mitigated by making cleanup EASY rather than automatic — the `YYYY-MM-DD-` prefix (AC-14) and `make ze-clean-sessions BEFORE=…` (AC-15). Both are operator-invoked |
| R-11 | **The dated directory is resolved three times** (make, Go, shell) and they drift. The session-id derivation failure is precisely this: three independent derivations drifted for weeks behind a prose invariant, and a disagreement fails closed | a session builds into one dir and execs from another | `TestMakeAndGoAgreeOnBinDir` (already in the Wiring Test) extended to cover the glob-then-create rule, plus AC-13 for the midnight boundary. The rule is stated once in the Scope extension and MUST NOT be paraphrased per-implementation |
| R-5 | **The pipe-tail hook stops covering session binaries.** `EXPENSIVE_COMMAND` matches `(\./)?bin/ze[\w-]*` anchored on a command word | no red; the guard just silently stops firing | AC-11: extend the alternation, with a hook-parity golden case that fails without it |
| R-6 | Three prose copies of the "suffix on purpose" rationale disagree with the code after the change | grep finds the old claim | AC-12 greps for the retired vocabulary tree-wide |
| R-7 | `.claude/settings.local.json` allowlists `Bash(bin/ze-test:*)` etc., so every session-local invocation prompts | permission prompts on routine commands | Add `tmp/session/*/bin/…` prefixes; that file is user-local, so this is advisory, not gated |
| R-8 | **`ai/rules/CONDENSED.md` is a shared generated file and this checkout is under concurrent edit.** This spec requires regenerating it (`make ze-rules-condensed`) after the `commands.md` rewrite. Observed 2026-08-02 during the design phase: the index recorded `ai/rules/CONDENSED.md` and `ai/RFC-REQUIREMENTS.md` as unmerged (`UU`) with **no conflict markers in either file**, while HEAD moved under this session | `git status` shows `UU`, or the digest regenerates with a sibling session's rule text | Before regenerating, confirm no sibling session holds an uncommitted `ai/rules/*.md` edit (`ai/rules/rule-format.md`). Regenerate in the SAME commit as the rule edit, and never "clean" foreign rows out of a shared generated file (`ai/rules/git-safety.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible: no shipped code path changes. The blast radius is the development loop — a wrong landing breaks `make ze`, the functional runner, or QEMU targets for agents only. Humans and CI are on the untouched off-session branch |
| How is it reverted? | Single commit revert. No migration, no on-disk format, no released contract |
| Who else touches this path? | `plan/spec-session-scoped-build-artifacts.md` (open, its decision is superseded here) and `plan/spec-fixit-netns-test-dut-tags.md` (open, `ready`; its `ze-netns-test` blocker is *caused* by the suffix and is fixed as a side effect) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-path ZE_SESSION_ID=<id>` | → | `mk/session.mk` `ZE_BIN_DIR` derivation | `TestZePathIsSessionDirectory` (`scripts/dev/session_bin_dir_test.py`) |
| `make ze-path` with no session id | → | off-session branch | `TestZePathOffSessionIsSharedBin` (same file) |
| `make ze ZE_SESSION_ID=<id>` | → | recipe `mkdir -p $(dir $(ZEBIN_ZE))` + `-o` | `TestSessionBuildCreatesItsOwnDirectory` (same file) |
| `ze-test` run under a session | → | `sessionpath.BinDir` agreeing with make | `TestMakeAndGoAgreeOnBinDir` (same file) |
| a `.ci` suite under a session | → | bare-name exec off one PATH entry | `test/parse` + `test/encode` suites green |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ZE_SESSION_ID=abc`, `make ze` | binary at `tmp/session/<today>-abc/bin/ze`, bare name; `bin/ze` untouched |
| AC-2 | no session id, `make ze` | binary at `bin/ze`, identical to today |
| AC-3 | `ZE_SESSION_ID=abc`, `make ze` with the session dir absent | the recipe creates the directory; build succeeds |
| AC-4 | two session ids build concurrently | two directories, neither writes the other's path |
| AC-5 | unsafe id (`../../etc`, `a+b`, `a/b`, `.`, `..`, empty, quote-bearing) | falls back to `bin/<name>`; nothing is written outside `bin/` or `tmp/session/` |
| AC-6 | `ZE_SESSION_ID=test` (an id equal to a real binary's suffix) | yields `tmp/session/<today>-test/bin/ze`; **no collision is possible**, and the `ZE_BIN_NAMES` guard no longer exists |
| AC-7 | session ends (any reason: clean exit, kill, crash) | **nothing under `tmp/session/` is removed.** No `rm -rf` of the session dir, no `bin/*-<sid>` sweep, no marker deletion anywhere in `.claude/hooks/` |
| AC-8 | `ZE_SESSION_ID=abc`, `make ze`, then `<session-dir>/bin/ze data ls` | resolves `<session-dir>/etc/ze`, and the store is **seeded**, not silently empty |
| AC-9 | `tmp/` is a symlink to an out-of-tree target, `make ze-qemu-needs-linux-test` | the guest resolves the DUT binary; the run does not fail to exec |
| AC-10 | `make ze-clean-tmp` with a live session | the session dir survives (`tmp/session` is already excluded, `Makefile:747`) |
| AC-11 | `<session-dir>/bin/ze-test bgp plugin \| grep FAIL` | the pipe-tail hook blocks it, exactly as it blocks `bin/ze-test … \| grep FAIL` |
| AC-12 | grep the tree for `ZE_BIN_SUFFIX`, `reap_binaries`, `bin/ze-<sid>` prose | no live definition or claim remains; every prose copy states the directory design |
| AC-13 | any consumer resolves the session dir at 23:59 and again at 00:01 | **the same directory both times.** The dir is found by glob `tmp/session/????-??-??-<sid>`, created with today's date only on a miss |
| AC-14 | `ls tmp/session/` after several sessions | one dated directory per session, lexically sorted oldest-first; `rm -rf tmp/session/2026-07-*` removes exactly July's sessions and nothing else |
| AC-15 | `make ze-clean-sessions BEFORE=<YYYY-MM-DD>` | removes only session dirs dated strictly before that date; refuses to run without `BEFORE`; never touches the flat marker files or a dir dated on/after |
| AC-16 | grep `.claude/hooks/` and `scripts/dev/` for time-based or event-based deletion under `tmp/session/` | **no hit.** `find … -mmin +N -delete`, `reap_dead`, `reap_binaries`, and the SessionEnd `rm` are gone, not disabled |
| AC-17 | a CLI PID is reused by a new session after the old one died (source-4 id path) | the new session mints a **fresh** id: `.sid-by-pid-<clipid>-<starttime>` no longer matches, so no stale spec claim or gate marker is adopted |
| AC-18 | `.sid-by-pid-*` cache hit for a **live** session | resolves to the same id as before, on Linux and on macOS/BSD; the start-time key is read via `/proc/<pid>/stat` field 22 or `ps -o lstart=` |

## End-to-End User Stories

N/A — Scope is `tooling`. No user-facing operation changes; `docs/` states no session-binary
convention (verified: the research pass found zero hits for it under `docs/`).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZePathIsSessionDirectory` | `scripts/dev/session_bin_dir_test.py` | AC-1 | written, RED (phase 1) |
| `TestZePathOffSessionIsSharedBin` | same | AC-2 | written, GREEN today and must stay (phase 1) |
| `TestSessionBuildCreatesItsOwnDirectory` | same | AC-3 | |
| `TestUnsafeIDFallsBackToSharedBin` | same | AC-5 (all 7 rejected forms) | |
| `TestValidationDoesNotRunShell` | same | AC-5, quote-bearing id neither builds nor executes (carried over) | |
| `TestMakeAndGoAgreeOnBinDir` | same | make's `ze-path` dir == `sessionpath.BinDir` | written, RED (phase 1) |
| `TestNoSuffixVocabularyRemains` | same | AC-12, greps eight trees for the four retired identifiers and the suffixed-binary path shape; `plan/`, `RETIRED.md` and `*_test.py` are records rather than claims | written, GREEN (phase 6) |
| `TestCleanTmpPreservesSessionRoot` | `scripts/dev/session_scratch_test.py` | AC-10 | |
| `TestSessionEndDeletesNothing` | same | AC-7 — a SessionEnd payload leaves the dated dir, its `bin/`, and every marker in place | |
| `TestNoAutomaticDeletionRemains` | same | AC-16, greps `.claude/hooks/` + `scripts/dev/` for `-mmin +N -delete`, `reap_dead`, `reap_binaries` | |
| `TestSessionDirIsStableAcrossMidnight` | `scripts/dev/session_bin_dir_test.py` | AC-13 — resolve twice across a simulated date change, same dir | written, RED (phase 1) |
| `TestSessionStoreIsSeeded` | same | AC-8 — every ze_core recipe (`ze`, `ze-appliance`, `ze-stripped`) seeds and no other does; a real stripped-only build leaves a seeded store; the password is generated and 0600; a second build neither reseeds nor rotates; an init that leaves no store fails the build; a binary outside `tmp/session/` is refused | written, GREEN (phase 4) |
| `TestSessionDirsSortByDate` | same | AC-14 — dated prefix, lexical order, glob selects one month. Names come from `make ze-path`, not a fixture | written, GREEN (phase 6) |
| `TestCleanSessionsRefusesWithoutBefore` | same | AC-15 — no `BEFORE`, no deletion | |
| `TestCleanSessionsRemovesOnlyOlder` | same | AC-15 — boundary: a dir dated exactly `BEFORE` survives | |
| `session-id-live-cache-hit-stable`, `session-id-start-time-readable`, `session-id-start-time-ps-branch` | `scripts/dev/hook-fixture-check.py` (`session-id`) | AC-18 — cache hit returns the same id; both the `/proc` and the `ps` reader answer | PASS |
| `session-id-reused-pid-mints-fresh-id` | same | AC-17 — same PID, different start time, different id | PASS |
| `session-id-cache-key-carries-start-time`, `session-id-cache-file-named-by-key`, `session-id-reused-pid-leaves-dead-entry` | same | AC-17 — the key IS `<pid>-<starttime>`, and the dead entry is unreachable rather than swept | PASS |
| existing `sessionpath_test.go` | `internal/test/sessionpath/` | unchanged; already asserts the target layout | |

### Boundary Tests (numeric inputs)
N/A — no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/parse` suite | `test/parse/*.ci` | a suite runs green with session-directory binaries (bare-name exec regression gate) | |
| `test/encode` suite | `test/encode/*.ci` | same | |
| `test/ui/ze-stripped-surface.ci` | `test/ui/` | `ze-stripped` resolves by bare name; see the pre-existing gap in Known Limitations | |

### Interop Tests (Scope: protocol)
N/A — build tooling, no wire-visible behavior.

## Files to Modify

**Core mechanism**
- `mk/session.mk` - introduce `ZE_BIN_DIR`; `ZEBIN_*` become `$(ZE_BIN_DIR)/<bare-name>`;
  `ZE_SCRATCH_DIR` resolves the DATED dir by glob-then-create (AC-13);
  **delete** `ZE_BIN_SUFFIX`, `ZE_BIN_NAMES` and the collision guard; rewrite the header
- `Makefile` - 16 `mkdir -p bin` → `mkdir -p $(dir $(ZEBIN_*))`; `ze-clean-tmp` needs no
  change (`session` already excluded, `:747`); **delete** the `find tmp/session/ … -delete`
  line; add the `ze-clean-sessions` target (AC-15)
- `mk/appliance.mk`, `mk/gokrazy.mk` - the remaining 3 `mkdir -p bin` sites
- `mk/test-integration.mk` - `ZE_QEMU_*` rebuilt on `$(ZE_BIN_DIR)` instead of the suffix

**Root rename `tmp/s/` → `tmp/session/<YYYY-MM-DD>-<sid>/` (2026-08-03)**
- `internal/test/sessionpath/sessionpath.go` - `scratchRoot`/`Root` glob-then-create instead
  of `filepath.Join(baseDir, "tmp", "s", id)`; doc comments at `:9`, `:34`, `:74`
- `internal/test/sessionpath/sessionpath_test.go` - the `tmp/tmp/s/<id>` cases
- `scripts/dev/session-scratch.sh` - `dir=` and the whole header
- `scripts/dev/session_scratch_test.py` - the `tmp/s/sid-*` assertions
- `.claude/hooks/pretool-bash.py` - the `-o tmp/s/<id>/bin/` regex
- `scripts/dev/hook-parity-check.py` - the `tmp/s/x/ready` golden cases
- `ai/rules/testing.md`, `ai/rules/commands.md`,
  `ai/INDEX.md`, `Makefile`, `mk/session.mk`,
  `mk/test-functional.mk`

**Automatic deletion, REMOVED (owner decision 2026-08-03; deletions per `ai/rules/no-layering.md`)**
- `.claude/hooks/session-end-scratch.sh` - **delete the `rm -rf` of the session dir
  and the `bin/*-<sid>` loop**. Keep the spec-claim release? **No** —
  AC-7 admits no deletion; the claim ages out of relevance rather than being removed, and
  `block-premature-stop.sh` already heartbeats it with `touch -c`
- `scripts/dev/session-scratch.sh` - **delete `reap_dead` and `reap_binaries`** and the
  `--reap` flag; keep `--clean` (operator-invoked, so permitted)
- `.claude/hooks/session-start.sh` - **delete** the `find tmp/session/ … -mmin +1440 -delete`
  (`:22`) and the `--reap` call
- `.claude/hooks/lib/state-file.sh` - **delete `_cleanup_stale_markers`** entirely (all seven
  `find … -delete` calls and the orphaned-state-file loop, `:80-118`) and its callers
- `.claude/hooks/lib/session_id.py` - key the minted-id cache on **PID + start time**
  (`_cli_ancestor_pid`, `_mint_cached`, `:189-279`); update the `:196` docstring claim, which
  currently rests on the deleted age-out (R-9)

**Guards**
- `.claude/hooks/pretool-bash.py` - `EXPENSIVE_COMMAND` covers session bin paths
  (`check_root_build` accepts `-o tmp/s/<id>/bin/` today, `:109` — it needs the rename, see above)
- `scripts/evidence/qemu-run.py` - second 9p share when `tmp/` is a symlink (R-3)

**Shims that become dead**
- `test/perf/run.py` - delete `bare_named_perf` and its docstrings
- `internal/test/runner/runner.go` - `setupBinShims` keeps its darwin hazard role; its
  session-name rationale comment is retired

**Tests**
- `scripts/dev/session_bin_suffix_test.py` → renamed/rewritten as `session_bin_dir_test.py`
- `scripts/dev/qemu_binary_paths_test.py` - retarget the `$(ZE_BIN_SUFFIX)` assertion
- `scripts/dev/hook-parity-check.py` - golden case for AC-11
- `scripts/dev/session_scratch_test.py` - AC-7, AC-10

**Prose (all three copies of the retired rationale)**
- `ai/rules/commands.md` - rewrite the section, including the paragraph that explicitly
  rejects this design
- `ai/INDEX.md` - the `make ze-path` row
- `scripts/dev/session-scratch.sh`, `.claude/hooks/session-end-scratch.sh` - header comments
- `scripts/evidence/netns_qemu.py`, `scripts/evidence/qemu-run.py`,
  `scripts/dev/qemu_binary_paths_test.py` - `$(ZE_BIN_SUFFIX)` docstrings
- `plan/learned/HOOK-FRICTION.md` - the copy-paste workaround snippet
- `docs/functional-tests.md` - `bin/ze-test` command examples (off-session, so accurate;
  add the session note only if it reads as universal)

**Generated (regenerate, never hand-edit — `ai/rules/repo-maintenance.md`)**
- `ai/rules/CONDENSED.md` via `make ze-rules-condensed`
- `ai/PACKAGE-MAP.md` / `ai/DOCS-TO-CODE.md` via `make ze-discovery-index` if doc comments move

## Files to Create
- `scripts/dev/session_bin_dir_test.py` - the layout gate (replaces the suffix gate)
- `scripts/dev/session-seed-store.sh` - the AC-8 seeding step, called from the `ze` recipe
- `plan/deferrals/session-bin-directory.md` - deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | build tooling, no config surface |
| YANG validation constraints | No | as above |
| YANG custom validators | No | as above |
| CLI commands/flags | No | `ze-path` is a make target, not a `ze` command |
| CLI grammar | No | as above |
| Editor autocomplete | No | as above |
| Functional test for new RPC/API | No | no RPC |
| Pipe completeness | No | no command output |
| Env var registration | No | `ze.session.id` already registered (`sessionpath.go`); no new var |
| Doctor check | No | no new runtime dependency. See Known Limitations for a fail-open doctor check found in passing |
| Prometheus counters | No | none |
| BGP family surface | No | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | development tooling only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | verified: zero hits for the convention under `docs/` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if its `bin/ze-test` examples read as universal |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `ai/rules/commands.md`, `ai/INDEX.md`, `mk/session.mk` header |
| 13 | Route metadata? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered plugin/command/inventory changed? | No | none |
| 16 | Changed files referenced by doc source anchors? | Yes | grep `docs/` for anchors on every changed file |
| 17 | Existing docs show examples for this area? | Yes | `docs/functional-tests.md` command examples |

## Implementation Steps

0. **Phase: Session-id cache key (MANDATORY FIRST, blocks phase 3)** — make the minted-id
   cache self-invalidating so removing the age sweep cannot reopen 1162/1246 (R-9)
   - Tests: AC-17, AC-18 (`scripts/dev/hook_session_id_test.py` or the existing hook fixtures)
   - Files: `.claude/hooks/lib/session_id.py` (`_cli_ancestor_pid`, `_mint_cached`)
   - Shape: cache key becomes `<clipid>-<starttime>`; `/proc/<pid>/stat` field 22 on Linux,
     `ps -o lstart=` on macOS/BSD, matching the module's existing `/proc`-then-`ps` pattern
   - Verify: AC-17 mutation-verified — restore the PID-only key and the test goes red.
     **This phase lands before any deletion is removed**, or the window is open in between
1. **Phase: Wiring** — the layout gate, failing
   - Tests: `TestZePathIsSessionDirectory`, `TestZePathOffSessionIsSharedBin`,
     `TestMakeAndGoAgreeOnBinDir`, `TestSessionDirIsStableAcrossMidnight` (AC-13)
   - Files: `scripts/dev/session_bin_dir_test.py`
   - Verify: all RED against today's suffix behavior, for the right reason
2. **Phase: Root rename + dated directory** — `tmp/s/<sid>` → `tmp/session/<YYYY-MM-DD>-<sid>`,
   glob-then-create in all three implementations
   - Tests: AC-1..AC-6, AC-13, AC-14
   - Files: the "Root rename" list above, plus `mk/session.mk`, `Makefile`,
     `mk/appliance.mk`, `mk/gokrazy.mk`
   - Verify: phase-1 tests go green; off-session `make -n ze` output byte-identical to HEAD;
     `grep -rn 'tmp/s/' --exclude-dir=tmp --exclude-dir=plan .` is empty
3. **Phase: Remove automatic deletion** — every hook-driven and time-driven `delete`/`rm`
   under `tmp/session/` goes; only operator-invoked targets remain
   - Tests: AC-7, AC-10, AC-16 (`session_scratch_test.py`, hook fixtures)
   - Files: `.claude/hooks/session-end-scratch.sh`, `.claude/hooks/session-start.sh`,
     `.claude/hooks/lib/state-file.sh`, `scripts/dev/session-scratch.sh`, `Makefile:749`
   - **A-11 is BROKEN, and this phase owns the fallout.** `_cleanup_stale_markers` has one
     live caller and three fixtures that invoke it by name. The fixture at
     `hook-fixture-check.py` asserts a live session's claim must NOT age out —
     that becomes vacuous when nothing ages out, so **rewrite it to assert the new contract
     (nothing is deleted, ever), never delete it** (`ai/rules/testing.md`). The
     R-11 UUID-sid fixture tested sid recovery inside the sweep; if that parsing
     has no other consumer it goes with the function, and if it does, it moves
   - Also stale after this phase: the sweep descriptions in `session-end-summary.sh`,
     `block-premature-stop.sh`, `commit_helper.py` (`ai/rules/stale-comments.md`)
   - Verify: AC-16 grep clean; a SessionEnd event leaves the directory intact;
     `python3 scripts/dev/hook-fixture-check.py` passes with the rewritten fixtures
3a. **Phase: Operator cleanup path** — make manual cleanup easy, since it is now the only kind
   - Tests: AC-15
   - Files: `Makefile` (`ze-clean-sessions`), `ai/rules/commands.md`
   - Verify: refuses without `BEFORE`; removes only strictly-older dated dirs; leaves the
     flat marker files and any dir dated on/after `BEFORE` untouched
4. **Phase: Seed the session store** — close R-1/R-2 before they can bite
   - Tests: AC-8
   - Files: `mk/session.mk` or the `ze` recipe
   - Shape (proven, A-7): `printf '%s\n' <user> <pass> 127.0.0.1 2222 <name> | $(ZEBIN_ZE)
     init --seed`, guarded so it runs ONLY on-session and ONLY when
     → **Corrected 2026-08-10, and `--force --yes` is now BANNED here.** The prototype
     line named them; a recipe must never carry them. `--force` calls `moveAsideDB`
     (`internal/plugins/init/main.go` `Run`), which RENAMES an existing database to
     `.replaced-<date>` — including the operator's own `<repo>/etc/ze` if the guard
     ever resolves the shared `bin/ze`. `--yes` is read only inside `if *forceFlag`,
     so it does nothing on its own. Without `--force`, `runInit` refuses with
     "database already exists", which is the failure this recipe WANTS. This is not
     theoretical: during phase 4 the script reached the real `bin/ze` while a guard
     was mutated, and the operator's database survived only because `--force` was absent
     `$(ZE_BIN_DIR)/../etc/ze/database.zefs` is absent. `--yes` alone does not suffice
   - Open sub-decision for the implementer: where the dev credentials come from. A fixed
     literal is simplest and the store is throwaway, but it lands a password in a tracked
     makefile — raise it rather than pick silently
   - Verify: a fresh session's `ze data ls` returns a seeded store, not an error
5. **Phase: Guards and QEMU** — pipe-tail regex, the 9p share, `ZE_QEMU_*`
   - Tests: AC-9, AC-11; `qemu_binary_paths_test.py`; `hook-parity-check.py` golden
   - Files: `.claude/hooks/pretool-bash.py`, `scripts/evidence/qemu-run.py`,
     `mk/test-integration.mk`
   - Verify: AC-11 mutation-verified (revert the regex → the golden goes red); AC-9 proven
     against a symlinked `tmp/`, not merely reasoned about
6. **Phase: Retire the shims and the prose** — `bare_named_perf`, all rationale copies,
   regenerate the digests
   - Tests: AC-12
   - Files: `test/perf/run.py`, the prose list above, `make ze-rules-condensed`
   - Verify: AC-12 grep clean; `make ze-doc-test` and `make ze-regen-check` green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` |
| Off-session identity | `make -n` output for all 8 binaries byte-identical to HEAD with no session id |
| Deletion, not layering | `ZE_BIN_SUFFIX`, `ZE_BIN_NAMES`, `reap_binaries`, `bare_named_perf` are GONE, not bypassed (`ai/rules/no-layering.md`) |
| Fail-closed | An unsafe id still falls back to `bin/`; nothing escapes `tmp/session/` |
| Single source of truth | Exactly one definition of the session bin directory; make and Go agree by test, not by comment |
| Silent-failure surface | R-1/R-2 closed by an ASSERTED seed, not by a comment saying agents should seed |
| Rule: `spec-preservation` | The superseded `owner-selected` rationale survives into the learned summary |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Session layout | `make ze-path ZE_SESSION_ID=probe` prints `tmp/session/<today>-probe/bin/ze` |
| Off-session unchanged | `make ze-path` prints `bin/ze` |
| Suffix vocabulary retired | `grep -rn 'ZE_BIN_SUFFIX\|reap_binaries' Makefile mk/ scripts/ .claude/` is empty |
| One root | `grep -rn 'tmp/s/' --exclude-dir=tmp --exclude-dir=.git .` is empty outside `plan/` (this spec names the old path while describing the rename) |
| No automatic deletion | `grep -rn 'mmin +1440\|reap_dead\|rm -rf .*tmp/session' .claude/hooks/ scripts/dev/` is empty |
| Manual cleanup works | `make ze-clean-sessions BEFORE=2026-07-01` removes only pre-July dirs; bare `make ze-clean-sessions` refuses |
| Seeded session store | `<session-dir>/bin/ze data ls` lists keys |
| Gates green | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Path traversal | The id becomes a **path component**, not a name suffix, so traversal matters more than before. `.`/`..`/`/` and the charset check must be proven on the RESOLVED id, including a command-line `make ZE_SESSION_ID=…` override |
| Shell injection | The id still reaches a `$(shell … tr)` call; the quote refusal and `TestValidationDoesNotRunShell` carry over unchanged |
| Deletion scope | Cleanup now removes a DIRECTORY. Prove it can only ever remove `tmp/session/<validated-id>`, never `bin/`, never `tmp/` itself |
| Secret material | A per-session SSH host key is generated under `tmp/`. Confirm `tmp/` is gitignored (it is: `.gitignore` `tmp/*`) and that the key never lands in a commit |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Off-session `make -n` differs from HEAD | DESIGN — the off-session branch is not in scope and must not move |
| A `.ci` suite fails on binary resolution | Check A-4 and the PATH entry, not the test |
| QEMU cannot exec the DUT | R-3; prove against a symlinked `tmp/` |
| 3 fix attempts failed | STOP. Report all 3. Ask the user |

## Prototype Evidence (2026-08-02, design phase)

The layout was built and run before the spec was accepted, so the core claims are measured
rather than argued. Commands ran against a real `ze` binary compiled into
`tmp/session/<sid>/bin/ze` alongside the existing `bin/` build.

| Claim | Command | Result |
|-------|---------|--------|
| Rebuild cost is a link, not a compile (A-3) | same tags, two `-o` paths | 18.54 s then **4.91 s**; `cmp` reports byte-identical output |
| The session binary resolves an isolated config dir (A-1) | `tmp/session/<sid>/bin/ze data ls` | `…/tmp/session/<sid>/etc/ze/database.zefs: no such file` — resolved the session dir, not `<repo>/etc/ze` |
| Unseeded failure is loud on this path, not silent | same | exit **2**, explicit path in the message |
| …but a side effect is silent | `find tmp/session/<sid>/etc` after the failed command | `etc/ze/crash/` was **created** by the failed run (crashlog persist) |
| `ze init --yes` is not non-interactive (A-7) | `ze init --force --yes --seed`, no stdin | exit **1**, `username is required` |
| Seeding works via stdin (AC-8 mitigation) | `printf '%s\n' user pass host port name \| … ze init --force --yes --seed` | exit **0**, `initialized …/tmp/session/<sid>/etc/ze/database.zefs` |
| The seeded session store is usable | `tmp/session/<sid>/bin/ze data ls` | 7 keys listed, exit 0 |
| **Isolation holds both ways** | `bin/ze… data ls` before and after all of the above | unchanged listing, exit 0; `etc/ze/database.zefs` still 1205 B dated Jul 29, session store 629 B dated Aug 2 |

The last row is the one that matters for owner decision 1: a session can seed, write, and
read its own store without the human's live database observing any of it.

## Design Insights
- The Go half (`sessionpath.BinDir`) already implements the target layout. This work is
  make catching up to it, which is why it is mostly deletion.
- A per-session **directory** retires two guards a per-session **name suffix** required: the
  binary-name collision check, and the argv[0] personality-dispatch shim. Suffixes collide
  in a shared namespace; directories do not have that namespace.
- The predecessor's objection was never "a binary cannot live elsewhere" — it was "an
  isolated `etc/ze` is the wrong default for dev binaries". The owner has now chosen
  isolation as the intent, which converts a hazard into a feature. The hazard only becomes
  a defect if isolation is claimed but the store is left silently empty, which is R-1.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-session directory `tmp/session/<sid>/bin/<bare-name>` | name suffix `bin/<name>-<sid>` (the shipped design) | Cleanup is one `rm -rf`; ~600 MB of orphans in `bin/` is the evidence the name sweep does not hold. Owner-selected 2026-08-02, superseding the 2026-07-25 choice |
| Session-local `etc/ze` | symlink `tmp/session/<sid>/etc` → `<repo>/etc`; export `ze.config.dir`; teach `paths.go` the layout | Owner-selected. Full isolation: `<repo>/etc/ze` becomes the human's alone. The symlink option preserved today's behavior exactly but kept the live database a shared resource |
| Binaries die with the session | keep until the 24h reaper | Owner-selected. Rebuild is a re-link (A-3) |
| Delete `ZE_BIN_SUFFIX` outright | keep it as an empty compatibility variable | `ai/rules/no-layering.md`: delete X, then implement Y. An empty variable would leave the tree describing two designs |
| **`tmp/session/` is the one root** (2026-08-03) | keep `tmp/s/` and move the flat markers into it; keep both | Owner-selected. `tmp/s` needs a doc lookup to decode (`ai/rules/go-standards.md`). `tmp/s/` was the cheaper rename — it is what Go, make and the `.ci` binary paths already produce — and legibility won over churn because the operator reads this directory by hand |
| **Dated directory `<YYYY-MM-DD>-<sid>`** (2026-08-03) | bare `<sid>`; a sidecar mtime; a manifest file | Owner-selected. Once deletion is manual, the operator must be able to see age with `ls` and select it with a glob. mtime does not survive a copy and is not visible at a glance; a manifest is a second source of truth |
| **No automatic deletion under `tmp/session/`** (2026-08-03) | keep the SessionEnd `rm -rf`; keep only the 24h age sweep | Owner-selected, and the owner's mechanism argument is correct: SessionEnd fires only on a clean end — it returns early on `reason=resume` (`session-end-scratch.sh`) and never runs on a kill — so it is both unreliable AND the deletion the operator was never asked about. Growth (R-10) is accepted as the price |
| Flat marker files keep their names and stay at the root | move all of them into `<session-dir>/` for a single `rm -rf` | The single-`rm -rf` argument dies with automatic deletion. Three of them cannot move at all (id minting, spec-keyed ack, cross-session handoff — see Scope extension), and the other six are written by ~10 hooks whose failure mode is a gate that silently stops firing |
| Cache key `<clipid>-<starttime>` | keep the 24h age-out for `.sid-by-pid-*` alone | The age-out is the only automatic deletion doing correctness work (R-9). Making the key self-invalidating removes the need for it entirely, rather than carving an exception into a policy the owner stated without one |

**Superseded decision, preserved verbatim** from
`plan/spec-session-scoped-build-artifacts.md` (`ai/rules/planning.md`):

> | Dev binaries suffixed `bin/<name>-<sid>` | `tmp/s/<id>/bin/<name>` | Preserves `<repo>/etc/ze` resolution and the live database; owner-selected |

> A binary's path is part of its runtime contract. "Where do we put the binary" could not be
> answered from build concerns alone — `ConfigDirFromBinary` made it a data-durability
> question, which is why dev binaries take a suffix and test binaries take a directory.

That reasoning stands. What changed is the answer to the data-durability question: the live
database is no longer something a session should reach at all.

## Known Limitations
- `bin/gok`, `ze-host`, and the cross-compiled installer keep their current locations
  (inherited Known Limitations; a fourth binary-location convention already exists).
- **Pre-existing, found in passing, not fixed here:** in `ZE_TEST_CANONICAL=1` mode under a
  session, `ze-stripped` is not built into the runner's PATH entry, so
  `test/ui/ze-stripped-surface.ci` resolves whatever the inherited PATH offers. This spec
  makes it *more* likely to resolve correctly (bare names), but does not prove it. Deferral
  shard row required if it is not closed by AC-3's suite run.
- **Pre-existing, found in passing:** `internal/component/doctor/checks_storage.go`
  `checkStoreIntegrity` returns nil (healthy) when the store file is absent — a fail-open
  guard (`ai/rules/evidence.md`). Unrelated to this work; needs its own home.
- **Pre-existing, found in passing:** `ai/rules/repo-maintenance.md` and
  `ai/rules/planning.md` both state `block-premature-stop.sh` is unregistered and
  inert. It is registered on `Stop` with `blocking: true` in `.claude/settings.json`.
  Three gates documented as dead are live. Needs its own fix.

## RFC Documentation (Scope: protocol)
N/A — no protocol behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] The predecessor spec is closed or explicitly re-homed; it must not be left claiming a
      decision this spec reversed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A — build tooling)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Journal rows written. The numbered `plan/learned/NNN-*.md` route this line named
      no longer exists: that corpus was deleted, and a closure's lesson is now one row in
      `plan/journal/<class>.md` (owner directive 2026-08-10, `ai/rules/planning.md`)
- [ ] **Commit A:** code + tests + docs + spec + journal rows
- [ ] ~~**Commit B:** `git rm plan/spec-session-bin-directory.md` only~~ **NOT RUN.**
      AC-27 is held open by owner instruction (recorded under the AC table), so this spec
      stays in `plan/` with Status `in-progress` and the claim stays unreleased. Removing
      it would destroy the record of the one outstanding item (`ai/rules/completion.md`)

---

## Implementation Summary

### What Was Implemented

Ten phases, all green (0, 1, E2, 2, 3, 3a, 4, 5, E1, 6). One AC is open on purpose.

| Area | Change |
|------|--------|
| Binary location | `mk/session.mk` gained `ZE_SESSION_ROOT` and `ZE_BIN_DIR`; every `ZEBIN_*` is `$(ZE_BIN_DIR)/<bare-name>`. `ZE_BIN_SUFFIX`, `ZE_BIN_NAMES` and the collision `ifeq` are DELETED. `Makefile` turned 16 `mkdir -p bin` into `mkdir -p $(ZE_BIN_DIR)`; `mk/test-integration.mk` rebuilt `ZE_QEMU_*` on it |
| One root, dated | `tmp/s/<sid>/` became `tmp/session/<YYYY-MM-DD>-<sid>/`. The dated directory is LOOKED UP by glob and created only on a miss, in four languages: `mk/session.mk` (`ZE_SCRATCH_DIR`), `internal/test/sessionpath/sessionpath.go` (`Root`), `.claude/hooks/lib/session-dir.sh` (`_session_dir`), `.claude/hooks/pretool-writeedit.py` (`session_dir`) |
| Three subdirectories | `bin/` (binaries and the `etc/ze` they resolve), `scratch/` (`session-scratch.sh` prints it), `state/` (the per-spec digest, moved by `_state_file` in `.claude/hooks/lib/state-file.sh`) |
| No automatic deletion | `.claude/hooks/session-end-scratch.sh` DELETED with its `SessionEnd` registration; `_cleanup_stale_markers` DELETED from `state-file.sh`; `reap_dead`, `reap_binaries` and `--reap` DELETED from `session-scratch.sh`; both `-mmin +1440 -delete` lines DELETED from `session-start.sh`; `ze-clean-tmp` lost its `find tmp/session/ … -delete` |
| Operator cleanup | NEW `make ze-clean-sessions BEFORE=<YYYY-MM-DD>` (`Makefile`), which refuses without `BEFORE` and removes only strictly-older dated directories |
| Session store seeded | NEW `scripts/dev/session-seed-store.sh`, called through `ZE_SEED_SESSION_STORE` from all six `ze_core` recipes. Generates a 0600 per-session password from `/dev/urandom`, takes an atomic lock, and runs `ze init --seed`. No `--force` |
| PID-reuse safety | `.claude/hooks/lib/session_id.py`: the minted-id cache key became `<clipid>-<starttime>` (`_pstart`, `_cache_key`), so it self-invalidates and needs no age-out |
| `tmp/` root guard | NEW `.claude/hooks/lib/scratch_path.py` (`is_ad_hoc_root_file`), driven from `check_scratch_path` (Bash) and `c_scratch_path_we` (Write and Edit). A bare `tmp/<file>` is REFUSED, exit 2, in all three of its spellings |
| QEMU | `scripts/evidence/qemu-run.py` gained `scratch_share`/`virtfs_args`: a second 9p share, mounted at the link's own text, whenever `tmp/` is a symlink |
| Shims retired | `bare_named_perf` DELETED from `test/perf/run.py` with `ZE_PERF_SRC`; the session-name rationale retired from `internal/test/runner/runner.go` `setupBinShims` |

### Bugs Found/Fixed

| Bug | Covered by |
|-----|-----------|
| `reap_binaries` read `ZE_BIN_NAMES` out of `mk/session.mk` with `sed`. AC-6 deletes that variable, so the guard would have degraded to an EMPTY list silently, and `ZE_SESSION_ID=test` (legal under AC-6) would then have made `--clean` run `rm -f bin/*-test` over the shared `bin/ze-test` | the mechanism was deleted rather than repaired (`ai/rules/no-layering.md`); `TestNoAutomaticDeletionRemains` refuses its return |
| The seeding step named `ze init --force --yes` from the prototype. `--force` calls `moveAsideDB` (`internal/plugins/init/main.go` `Run`), which RENAMES an existing database. Reached live during mutation testing: the script hit the real `bin/ze` and ran `init` against the operator's `<repo>/etc/ze` | `--force` is banned in `session-seed-store.sh` and the reason is in its header. `plan/journal/mutating-a-guard-runs-the-unguarded-path.md` |
| Only `ze` seeded. `ze-appliance` and `ze-stripped` reach the same silent `NewBlob` path, so a stripped-only session got an empty store | measured with `go list -deps` per tag set; all six `ze_core` recipes now call it, and `test_a_stripped_only_build_seeds_its_store` runs the real build |
| Two hooks that CORRECT a path named `tmp/out-$$.log` and `tmp/<subfolder>/` in their own remediation text, teaching the shape the rules ban. 351 loose files at the `tmp/` root in one day | remediation text now names `session-scratch.sh`; `plan/journal/guard-message-teaches-the-violation.md` |
| **`make ze-clean-tmp` could delete the entire scratch tree.** `find tmp/ -maxdepth 1 -type d` yields `tmp/` ITSELF at depth 0, whose basename is `tmp`, so neither `-not -name session` nor `-not -name kernel` excluded it. A `tmp/` whose own mtime was older than 24h was a match, and `rm -rf` took `tmp/session/` with every session's binaries, seeded store and `state/` digest. Pre-existing at HEAD; in scope because AC-10 asserts this target preserves the session root, and because this spec's diff adds a comment beneath it claiming exactly that | `-mindepth 1`, and `TestCleanTmpPreservesSessionRoot.test_a_stale_tmp_root_is_not_swept_as_its_own_child`. MUTATION-VERIFIED |
| **The `tmp/`-root guard passed two of the three spellings of the same file.** `check_scratch_path` anchored on a literal `tmp/`, so `./tmp/x` and the absolute `<repo>/tmp/x` were allowed while the Write surface refused them. The harness hands agents absolute paths, and the same diff had already widened `EXPENSIVE_COMMAND` for that spelling | `_SCRATCH_WRITE` takes a leading directory prefix; the shared module already decided on the resolved parent, so the candidate widened and the refusal did not. Four new parity rows including an allow-control. MUTATION-VERIFIED |
| **`make -j build` raced three seeders on one database.** `build` names `$(ZEBIN_ZE)`, `$(ZEBIN_APPLIANCE)` and `$(ZEBIN_STRIPPED)`, each calling the seeder, and there is no `.NOTPARALLEL`. The existence test was a check-then-act: all three saw no database and all three reached `ze init` | an atomic `mkdir` lock with a bounded wait on the winner's postcondition. `test_a_parallel_build_seeds_exactly_once` asserts `ze init` ran ONCE. MUTATION-VERIFIED: without the lock it runs three times |
| **`BEFORE` reached a shell before its own format check.** `ze-clean-sessions` spliced `$(BEFORE)` into a double-quoted shell literal, so `make ze-clean-sessions 'BEFORE=";touch pwn;x"'` ran that `touch` and only then reported a bad date | `export BEFORE` and `$$BEFORE`, so the value is data. `test_a_before_that_is_shell_metacharacters_runs_nothing`. MUTATION-VERIFIED |
| **A dated regular file split the four resolvers.** make and Go took the first glob match of any type; the shell and python copies required a directory | the make wildcard took a trailing slash (its own directories-only idiom) with `$(patsubst %/,%,…)`, and `Root` skips non-directories. `test_a_dated_regular_file_is_no_session_directory_in_any_copy`. MUTATION-VERIFIED in both |
| Three rule points and `.claude/hooks/README.md` claimed the spec claim is released at `SessionEnd`. Verified against `_release_session`: its only caller is `scripts/dev/spec-session.sh release`, which `/ze-close` runs | corrected in phase 6; fixture counts re-measured 35 to 36 |
| Nine live instructions across `scripts/evidence/netns_qemu.py`, `qemu-run.py`, `scripts/checks/tracked_build.go`, `scripts/dev/stress-repro.py`, `plan/learned/HOOK-FRICTION.md`, `ai/INDEX.md`, `mk/test-unit.mk`, `internal/test/runner/runner.go` and `internal/test/sessionpath/sessionpath.go` still named the retired design, the age-out, or the session-end sweep | phase 6's sweep plus the closure sweep; `TestNoSuffixVocabularyRemains` (AC-12) refuses the vocabulary's return |

### Documentation Updates

| File | What changed | Anchor |
|------|--------------|--------|
| `docs/functional-tests.md` | `tmp/testbin-<id>/` is now named as the OFF-session spelling with the on-session one beside it; the cleanup sentence splits `ze-clean-tmp` (off-session) from `ze-clean-sessions` (on-session); "Run a single test" no longer says the runner rebuilds `bin/ze` | existing `<!-- source: mk/test-functional.mk … -->` and `<!-- source: internal/test/runner/runner.go … -->` anchors re-read and still true |
| `docs/architecture/testing/qemu-integration.md` | the mount diagram gained the conditional second 9p share, with one paragraph saying why | `scripts/evidence/qemu-run.py` `scratch_share` |
| `ai/INDEX.md` | the `make ze-path`, `session-scratch.sh` and `stress-repro.py` rows | — |
| `ai/INSTRUCTIONS.md` | the dispatch row for running a test or build command | — |
| `.claude/hooks/README.md` | NEW "The session directory" subsection; the source-4 resolution row; the marker list no longer calls the digest flat | — |
| `ai/rules/commands.md`, `testing.md`, `repo-maintenance.md`, `planning.md`, `context-economy.md` | rendered from their points by `make ze-rules-render`; digests by `ze-rules-condensed` and `ze-rules-index` | generated, never hand-edited |

`make ze-doc-test` rc=2. Sole driver: `WARNING: ai/DOCS-TO-CODE.md is stale`, FOREIGN
(evidence in Pre-Commit Verification). Both `NOT AT HEAD` notices name this closure's own
journal files and clear when the commit records them.

### Deviations from Plan

| Planned | Done | Why |
|---------|------|-----|
| Phase 2 leaves `reap_binaries` and the SessionEnd `bin/*-<sid>` loop standing for phase 3 | both DELETED in phase 2 | their subject (`bin/<name>-<sid>`) is what phase 2 deletes, and `reap_binaries` would have degraded to an empty list silently. Keeping it was layering plus an irreversible `rm` |
| Phase 3 leaves `.claude/hooks/session-end-scratch.sh` as a no-op | the FILE and its `settings.json` registration DELETED | both of its statements are banned by AC-7, and AC-16 says these are "gone, not disabled". `TestSessionEndDeletesNothing` iterates whatever `settings.json` registers, so it goes red the moment a deleting hook returns |
| AC-16 scopes the deletion ban to `tmp/session/` | phase 3 also removed the `tmp/` ROOT age sweep from `.claude/hooks/session-start.sh` | it was hook-driven, so it was automatic, and it deleted `tmp/delete-<sid>.sh` and `tmp/commit-*.sh` under the operator. Its operator-invoked twin survives untouched as `make ze-clean-tmp`. Wider than the AC asked for, and recorded here rather than left as an unexplained removal |
| Seed with `ze init --force --yes --seed` | `ze init --seed` | `--force` moves an existing database aside. See Bugs Found |
| `scripts/dev/hook_session_id_test.py` for AC-17/AC-18 | the existing `scripts/dev/hook-fixture-check.py` `session-id` section | the file never existed; creating a second runner for one section is layering |
| `plan/deferrals/session-bin-directory.md` created | not created | no deferral was taken. See Deferrals Resolved |
| Commit B removes the spec | not run | AC-27 is held open by owner instruction. See the Closure checklist above |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | **A-11 BROKEN.** The spec assumed deleting `_cleanup_stale_markers` broke no caller | one LIVE caller (`.claude/hooks/session-start.sh`) and three fixtures invoking it BY NAME, one of which asserted a live session's claim must NOT age out, an assertion that becomes vacuous once nothing ages out. Two further files carried comments describing the sweep | `grep -rn _cleanup_stale_markers .claude/ scripts/`, during the 2026-08-03 design pass, before any deletion | phase 3 REWROTE the three fixtures to assert the new contract rather than deleting them (`ai/rules/testing.md`), and corrected the stale comments in `session-end-summary.sh`, `block-premature-stop.sh`, `mark-agent-spawned.sh`, `commit_helper.py`, `README.md` |
| assumption | **A-7 confirmed with a correction.** `ze init --yes` was taken as non-interactive | `--yes` is read only inside `if *forceFlag`, so it does nothing on its own; credentials arrive as five stdin lines | ran it, 2026-08-02: exit 1, `username is required` | the recipe pipes five lines and carries neither `--force` nor `--yes` |
| approach | mutation-testing the seed script's path guard EXECUTED the unguarded path for real, reaching the shared `bin/ze` and running `ze init` against the operator's own `etc/ze` | the operator's database survived only because the recipe carries no `--force`. The command a guard protects must be non-destructive on its own | the probe ran; `etc/ze/crash/` appeared in the repository's config dir | `plan/journal/mutating-a-guard-runs-the-unguarded-path.md`; `--force` is banned in the script and the reason is in its header |
| escalation | two hooks whose whole job is to correct a path named the banned shape in their own remediation text, and a session then wrote 351 loose files to the `tmp/` root in one day | an enforcement message is read as an instruction, so a guard that spells the violation teaches it | counted the `tmp/` root | `plan/journal/guard-message-teaches-the-violation.md`; the messages now name `session-scratch.sh`, and the shape itself is refused on both surfaces |
| approach | phase 4's first landing seeded only from the `ze` recipe | `ze-appliance` and `ze-stripped` link the same silent `NewBlob` path and can each seed themselves; a stripped-only session got an empty store | `go list -deps` over each recipe's own tag set, after review | the script takes the BINARY rather than the bin directory, all six `ze_core` recipes call it, and `test_a_stripped_only_build_seeds_its_store` drives a real build |
| escalation | **A guard was written on ONE spelling of its subject, and its own test corpus used that spelling.** `check_scratch_path` refused `tmp/x` and passed `./tmp/x` and the absolute `<repo>/tmp/x`; 196 parity rows were green because every row spelled it the refused way | the shared module already resolved the parent correctly. Only the regex that decided what to HAND it was narrow, and the sibling surface refused all three, so the two dispatchers disagreed under a docstring saying they must not | the Review Gate, round 1, by driving the hook with each spelling rather than reading the corpus | the candidate regex widened, four parity rows added including an allow-control, mutation-verified. The lesson generalizes past this guard: a corpus written by the guard's author tests the shape the author had in mind |
| approach | the closure sweep found THREE defects the ten implementation phases had not: a `find` that could remove `tmp/` itself, a check-then-act race under `make -j`, and operator input reaching a shell before its own validator | each was pre-existing or newly reachable rather than newly written, which is why phase-scoped review missed all three: none sat on a line any phase edited | the Review Gate's independent lenses | all three fixed at the source with a mutation-verified regression test each. `ai/rules/planning.md`: the gate exists to find what nobody planned for, and a phase's own review cannot |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| On-session `make ze` writes `<session-dir>/bin/ze`, a bare name in a private directory | Done | `mk/session.mk` `ZE_BIN_DIR`, `Makefile` `ze` recipe | live build verified |
| Off-session `make ze` writes `bin/ze`, byte-for-byte today's behavior | Done | `mk/session.mk` off-session branch | `make -n` diffed against `git archive HEAD`, re-run at closure |
| `bin/` holds only what a human or CI built | Done | same | the suffix that put session binaries there is deleted |
| One session root, `tmp/session/<YYYY-MM-DD>-<sid>/` | Done | `mk/session.mk`, `sessionpath.go` `Root`, `session-dir.sh` `_session_dir`, `pretool-writeedit.py` `session_dir` | four implementations, pinned against each other by `TestMakeAndGoAgreeOnBinDir` |
| Three subdirectories `bin/`, `scratch/`, `state/` | Done | `session-scratch.sh`, `state-file.sh` `_state_file` | AC-19, AC-20 |
| No automatic deletion under `tmp/session/` | Done | deletions listed in Implementation Summary | AC-7, AC-16 |
| Cleanup is one directory the operator identifies by date | Done | `Makefile` `ze-clean-sessions` | AC-14, AC-15 |
| A file at the `tmp/` root is REFUSED, not warned | Done | `.claude/hooks/lib/scratch_path.py` `is_ad_hoc_root_file` | AC-23..AC-26, in all three spellings after the Review Gate |
| The superseded `owner-selected` rationale survives into the record | Done | Key Design Decisions, quoted verbatim; and a supersession block now heads `plan/spec-session-scoped-build-artifacts.md` | `ai/rules/planning.md` spec preservation |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestZePathIsSessionDirectory` (2 tests); live `make ze ZE_SESSION_ID=zesbd-build-probe` wrote `tmp/session/2026-08-10-zesbd-build-probe/bin/ze` and left `bin/ze` untouched | |
| AC-2 | Done | `TestZePathOffSessionIsSharedBin`; `make -n` for `ze`, `ze-appliance`, `ze-stripped`, `ze-setup-bin` and `build` diffed against the same commands over `git archive HEAD Makefile mk`, `buildDate` normalised: byte-identical | |
| AC-3 | Done | the recipe's `mkdir -p $(ZE_BIN_DIR)`, driven live by `test_a_stripped_only_build_seeds_its_store` | |
| AC-4 | Done | `TestMakeAndGoAgreeOnBinDir`, `TestSessionBinDirIsolatesSessions` (Go) | |
| AC-5 | Done | `TestZePathOffSessionIsSharedBin.test_unsafe_id_falls_back_to_the_shared_bin` (10 forms), `TestValidationDoesNotRunShell`, `TestSessionPathsRejectUnsafeID` (Go) | |
| AC-6 | Done | `TestZePathIsSessionDirectory.test_an_id_equal_to_a_binary_suffix_is_accepted` (`test`, `perf`); `ZE_BIN_NAMES` and the collision guard are gone | |
| AC-7 | Done | `TestSessionEndDeletesNothing` (2 tests) plus fixtures `delegation-no-session-end-hook-registered`, `delegation-session-start-deletes-no-marker`, `delegation-session-start-reaps-no-session-dir` | `_release_session` still `rm -f`s the claim marker, and is the ONE removal AC-7's wording does not carve out. It is operator-invoked, from `scripts/dev/spec-session.sh release` under `/ze-close`, so it is not automatic deletion. Read AC-7 as "no deletion a hook or a timer drives" |
| AC-8 | Done | `TestSessionStoreIsSeeded` (9 tests) including a real `make ze-stripped` and the parallel-build case; live `<session-dir>/bin/ze data ls` listed 7 seeded keys | |
| AC-9 | Done | live QEMU boot over a fixture checkout whose `tmp/` IS a symlink; control with `scratch_share` neutered fails to resolve `/workspace/tmp/` | |
| AC-10 | Done | `TestCleanTmpPreservesSessionRoot` (2 tests), one with two controls that MUST be swept and one with the `tmp/` root itself stale | the second was added at closure and found a real defect |
| AC-11 | Done | `hook-parity-check.py` rows for the relative and absolute session paths (=2), plus a dateless negative control (=0). Mutation-verified | |
| AC-12 | Done | `TestNoSuffixVocabularyRemains`: eight trees, five patterns, zero offenders. Re-run at closure, clean | |
| AC-13 | Done | `TestSessionDirIsStableAcrossMidnight`, `TestRootPrefersAnExistingDatedDirectory` (Go), `TestSessionDirIsLookedUp` (shell). Mutation-verified in make AND Go | |
| AC-14 | Done | `TestSessionDirsSortByDate` (2 tests): names come from `make ze-path`, ids ordered against the dates, and a `2026-07-*` glob selects exactly July | |
| AC-15 | Done | `TestCleanSessionsRefusesWithoutBefore` (4), `TestCleanSessionsRemovesOnlyOlder` (4). Mutation-verified on the `-lt`/`-le` boundary and on the `BEFORE` interpolation | |
| AC-16 | Done | `TestNoAutomaticDeletionRemains`; the grep re-run at closure over `.claude/hooks/` and `scripts/dev/` is EMPTY | |
| AC-17 | Done | `session-id-reused-pid-mints-fresh-id`, `session-id-cache-key-carries-start-time`, `session-id-reused-pid-leaves-dead-entry`. Mutation-verified: the PID-only key makes a dead session's id be adopted | |
| AC-18 | Done | `session-id-live-cache-hit-stable`, `session-id-start-time-readable`, `session-id-start-time-ps-branch`, `session-id-cache-file-named-by-key` | |
| AC-19 | Done | `session-state-digest-lands-in-the-session-directory`, `session-state-digest-reuses-an-existing-dated-directory`; `scratch/` and `state/` now resolve through the SAME `_session_dir` | |
| AC-20 | Done | `session-state-nothing-is-written-flat` drives both `session-end-summary.sh` and `pre-compact-save.sh`, then asserts zero flat and exactly one nested | |
| AC-21 | Done | five `session-state-resolver-*` checks covering the `state/` walk, the flat fallback and both `.claude/` legacy forms | |
| AC-22 | Done | `session-state-flat-markers-do-not-move`: eight flat markers survive a full `session-start.sh` run | |
| AC-23 | Done | `hook-parity-check.py` BASH goldens `tee tmp/t.log`, `tee tmp/v.log`, `grep … > tmp/notes.txt` = 2, plus `./tmp/notes.txt`, the absolute form and `tee` of the absolute form = 2 | the last three were added at closure and found a real defect |
| AC-24 | Done | WE goldens `Write\|scratch tmp root` = 2, `Edit\|scratch tmp root` = 2, on the ABSOLUTE path the Write tool sends | |
| AC-25 | Done | goldens = 0 for `tmp/s/<id>/`, `tmp/session/<dated>/`, all eight producer folders on both surfaces, and `sub/tmp/notes.txt` as the widening control | |
| AC-26 | Done | goldens = 0 for `ze-verify.log`, `.ze-verify-duration.txt`, `commit-*`, `delete-*`, `mutation*`, `test-timings*` | |
| AC-27 | **OPEN, owner-held** | not implemented, on purpose | The owner instructed on 2026-08-10 that both layouts stay accepted "until it does not trigger as fully ironed out". The rename lives only in the working tree; sibling sessions share this checkout, so refusing `tmp/s/` before the commit lands would break one mid-run, which the scope extension names as the one failure this spec must not cause. `tmp/s/<id>/` is accepted BY CONSTRUCTION in `scratch_path.py`, which names no layout, so nothing had to be added to keep it. This is the one item this spec closes with open, and it is why commit B is not run |
| AC-28 | Done | `hook-parity-check.py` 200/200, both layouts present on both surfaces | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestZePathIsSessionDirectory` | Done | `scripts/dev/session_bin_dir_test.py` | |
| `TestZePathOffSessionIsSharedBin` | Done | same | green before and after the rename, as required |
| `TestSessionBuildCreatesItsOwnDirectory` | Changed | same, as `TestSessionStoreIsSeeded.test_a_stripped_only_build_seeds_its_store` | AC-3 is proven by a REAL build rather than by a class of that name |
| `TestUnsafeIDFallsBackToSharedBin` | Changed | same, as `TestZePathOffSessionIsSharedBin.test_unsafe_id_falls_back_to_the_shared_bin` | one class, 10 forms, rather than a second class |
| `TestValidationDoesNotRunShell` | Done | same | carried over from the retired suffix gate |
| `TestMakeAndGoAgreeOnBinDir` | Done | same | extended at closure to all FOUR resolvers, plus the dated-regular-file case |
| `TestNoSuffixVocabularyRemains` | Done | same | |
| `TestCleanTmpPreservesSessionRoot` | Done | `scripts/dev/session_scratch_test.py` | |
| `TestSessionEndDeletesNothing` | Done | same | |
| `TestNoAutomaticDeletionRemains` | Done | same | |
| `TestSessionDirIsStableAcrossMidnight` | Done | `scripts/dev/session_bin_dir_test.py` | |
| `TestSessionStoreIsSeeded` | Done | same | 9 tests |
| `TestSessionDirsSortByDate` | Done | same | |
| `TestCleanSessionsRefusesWithoutBefore` | Done | same | |
| `TestCleanSessionsRemovesOnlyOlder` | Done | same | |
| the AC-17/AC-18 `session-id-*` checks | Done | `scripts/dev/hook-fixture-check.py` | 7 checks |
| existing `sessionpath_test.go` | Changed | `internal/test/sessionpath/` | the `tmp/s/<id>` cases were retargeted; NEW `TestRootPrefersAnExistingDatedDirectory`, `TestRootCreatesNothing` |
| `test/parse` + `test/encode` suites | Done | `test/parse/*.ci`, `test/encode/*.ci` | bare-name exec regression gate; `make ze-parse-test` ran 308/308 on-session during the review |
| `test/ui/ze-stripped-surface.ci` | Changed | `test/ui/` | the pre-existing `ZE_TEST_CANONICAL=1` gap is unchanged and stays a Known Limitation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `mk/session.mk`, `Makefile`, `mk/test-integration.mk` | Done | |
| `mk/appliance.mk`, `mk/gokrazy.mk` | Changed | NOT modified. Their `mkdir -p bin` serves `bin/ze-installer-*` and `bin/gok`, which the Non-goals keep in `bin/`, so the mkdir still matches where the `-o` goes. The "Files to Modify" row naming them contradicted this spec's own Non-goals |
| `internal/test/sessionpath/sessionpath.go` + `_test.go` | Done | |
| `scripts/dev/session-scratch.sh`, `session_scratch_test.py` | Done | |
| `.claude/hooks/pretool-bash.py`, `pretool-writeedit.py`, `subagent-context.sh`, `lib/state-file.sh`, `lib/session_id.py`, `session-start.sh` | Done | |
| `.claude/hooks/session-end-scratch.sh` | Changed | DELETED entirely, not edited. See Deviations |
| `.claude/hooks/lib/session-dir.sh` | Done | NEW |
| `.claude/hooks/lib/scratch_path.py` | Done | NEW, and not in the original file table: the extension's guard needed ONE definition shared by the Bash and the Write surface |
| `scripts/dev/session_bin_dir_test.py` | Done | NEW; `session_bin_suffix_test.py` DELETED |
| `scripts/dev/session-seed-store.sh` | Done | NEW |
| `scripts/dev/hook-parity-check.py`, `hook-fixture-check.py`, `qemu_binary_paths_test.py` | Done | |
| `scripts/evidence/qemu-run.py`, `netns_qemu.py` | Done | |
| `test/perf/run.py`, `internal/test/runner/runner.go` | Done | |
| `ai/rules/points/**`, `ai/rules/points/RETIRED.md`, `ai/INDEX.md`, `ai/INSTRUCTIONS.md` | Done | rendered and digested by `make ze-rules-render` / `-condensed` / `-index` |
| `plan/learned/HOOK-FRICTION.md`, `docs/functional-tests.md` | Done | |
| `ai/rules/CONDENSED.md` | Changed | that file no longer exists; the digests are `ai/rules/TRIGGERS.md` and `CORE.md`, both regenerated |
| `ai/PACKAGE-MAP.md` / `ai/DOCS-TO-CODE.md` | Changed | deliberately NOT in this commit. `PACKAGE-MAP.md` is fresh; `DOCS-TO-CODE.md` is stale on a sibling session's deleted file. See Pre-Commit Verification |
| `plan/deferrals/session-bin-directory.md` | Changed | not created; no deferral was taken |
| `mk/test-unit.mk`, `scripts/checks/tracked_build.go`, `scripts/dev/stress-repro.py`, `.claude/hooks/README.md`, `block-premature-stop.sh`, `session-end-summary.sh`, `mark-agent-spawned.sh`, `validate-spec.sh`, `.claude/settings.json`, `.claude/rules/session-start.md`, `post-compaction.md`, `ai/skills/ze-{implement,review,spec,debrief}.md`, `docs/architecture/testing/qemu-integration.md`, `scripts/dev/commit_helper.py`, `mk/test-functional.mk` | Done | not in the plan's file list; each was reached by the stale-comment and stale-path sweeps the plan required (`ai/rules/stale-comments.md`) |
| `plan/spec-fixit-unexport-package-private-symbols.md` | Done | not in the plan's file list. A sibling session's in-progress spec whose worklist recipe wrote `tmp/unexport-gofiles.txt` and `tmp/unexport-worklist.tsv` at the `tmp/` ROOT, which AC-23 now refuses. Both moved into the `tmp/unexport-chunks/` the same recipe already creates. Nothing else in that spec changed |
| `plan/spec-session-scoped-build-artifacts.md` | Done | not in the plan's file list. The Goal Gate requires the predecessor not be left claiming a decision this spec reversed; a supersession block now heads it |

### Audit Summary
- **Total items:** 28 AC + 9 Task requirements + 20 TDD rows + 22 file rows = 79
- **Done:** 71
- **Partial:** 0
- **Skipped:** 0
- **Open by owner instruction:** 1 (AC-27)
- **Changed:** 7 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A session's binaries live in that session's own dated directory, with everything else that session wrote | functional (real build) | `make ze ZE_SESSION_ID=zesbd-build-probe` produced `tmp/session/2026-08-10-zesbd-build-probe/bin/ze`, 85 MB, bare name. `bin/ze` unchanged. Re-run at closure: `make ze-path ZE_SESSION_ID=zesbd-close-probe` prints `tmp/session/2026-08-10-zesbd-close-probe/bin/ze` and creates nothing |
| Cleanup is one directory the operator can identify by date and remove when they choose | functional | `TestSessionDirsSortByDate.test_a_month_glob_selects_exactly_that_month` plants 2026-07-31 and 2026-08-01 and proves `2026-07-*` selects exactly July. `TestCleanSessionsRemovesOnlyOlder` proves the boundary (a directory dated exactly `BEFORE` survives) and that flat markers and out-of-root paths are untouched. Mutation-verified: `-lt` to `-le` reds the boundary test |
| `bin/` holds only what a human or CI built | grep + build diff | AC-12's `TestNoSuffixVocabularyRemains` finds zero live uses of `ZE_BIN_SUFFIX`, `ZE_BIN_NAMES`, `reap_binaries`, `bare_named_perf` or the `bin/<name>-<sid>` path shape across eight trees. Mutation-verified by writing `ZE_BIN_SUFFIX` back into `test/perf/run.py` |
| Off-session behaviour is byte-for-byte today's | build diff | `make -n` for `ze`, `ze-appliance`, `ze-stripped`, `ze-setup-bin` (2 lines each) and `build` (29 lines), diffed against the same commands run over `git archive HEAD Makefile mk` with `buildDate` normalised: byte-identical. Re-run at closure after every fix: `mkdir -p bin` and `-o bin/<bare-name>`, with no seed line |
| One session root, resolved identically by make, Go, shell and python | cross-implementation test | `TestMakeAndGoAgreeOnBinDir.test_every_implementation_resolves_the_same_session_directory` drives all FOUR against a directory dated YESTERDAY, so a copy that ignored the glob would answer today and be caught. `test_a_dated_regular_file_is_no_session_directory_in_any_copy` covers the one input that split them. MUTATION-VERIFIED four ways: break the shell glob, neuter the python glob, revert make's directories-only wildcard, revert Go's IsDir check, each reds the pair and each was restored byte-identically |
| A session's store is isolated AND seeded, never silently empty (R-1, R-2) | functional (real binary) | `make ze-stripped ZE_SESSION_ID=zesbd-strip-probe` in a session with no `ze` binary seeded in 7 s; `<session-dir>/bin/ze-stripped data ls` exited 0 and listed the 7 keys. Isolation proven both ways: `bin/ze … data ls` unchanged before and after, `etc/ze/database.zefs` still 1205 B dated Jul 29 while the session store is 629 B. Mutation-verified: deleting the recipe call reds `test_a_stripped_only_build_seeds_its_store` with "a stripped-only session got an empty store". A parallel `make -j` seeds exactly once, mutation-verified against a lock-free script that seeds three times |
| Nothing under `tmp/session/` is removed automatically (owner decision) | grep + fixture | AC-16's grep over `.claude/hooks/` and `scripts/dev/` for `-mmin +1440`, `reap_dead`, `reap_binaries` and `rm -rf … tmp/session` is EMPTY, re-run at closure. `TestSessionEndDeletesNothing` iterates whatever `settings.json` registers on `SessionEnd`, drives each with a real payload, and diffs the whole tree, so it goes red the moment any deleting hook is registered again. Mutation-verified: putting the age sweep back in `session-start.sh` reds two checks. And the one path that could still have taken the whole root, `ze-clean-tmp` matching `tmp/` as its own child, is closed and mutation-verified |
| Removing the age-out cannot reopen the PID-reuse incident (R-9) | fixture + mutation | `session-id-reused-pid-mints-fresh-id`. MUTATION-VERIFIED: restoring the PID-only cache key makes a new session adopt a dead session's id (`dead == live`), which is incident 1162/1246 exactly |
| QEMU still reaches the DUT when `tmp/` is a symlink (R-3) | live boot + control | a real QEMU boot over a fixture checkout whose `tmp/` IS a symlink: the DUT under `/workspace/tmp/session/2026-08-10-ac9probe/bin/` execs and prints its marker, `QEMU VM: PASS`. CONTROL, same fixture with `scratch_share` neutered: `mkdir: can't create directory '/workspace/tmp/'`, exit 1, marker absent |
| A file at the `tmp/` root is refused, on both surfaces an agent writes with | golden corpus + mutation | 200/200 parity rows, covering all three spellings of the root path on the Bash surface and the absolute one on Write and Edit. MUTATION-VERIFIED four ways: the Bash guard made to allow reds 4 goldens; the Write guard made to allow reds 2; the search-argument exemption removed reds the audit golden; the regex narrowed back to the literal `tmp/` reds exactly the 3 new refuse rows and leaves the allow-control green |

Interop: N-A. Build tooling, no wire-visible behaviour, no protocol peer
(`ai/rules/interop-and-goal-validation.md`, "When interop tests are NOT required").

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| *(no shard exists)* | n-a | `plan/deferrals/session-bin-directory.md` was never created: no phase took a deferral. `ls plan/deferrals/session-bin-directory.md` gives No such file or directory. The spec metadata carries `-` for the shard field, which is the "create on first deferral" state |
| `test/ui/ze-stripped-surface.ci` under `ZE_TEST_CANONICAL=1` | deferred, and NOT this spec's | Pre-existing and recorded under Known Limitations. This spec makes correct resolution MORE likely (bare names) and does not prove it. It was found in passing before implementation began |
| `internal/component/doctor/checks_storage.go` `checkStoreIntegrity` fails open | deferred, needs its own home | Pre-existing, found in passing, unrelated to this work. Recorded under Known Limitations |
| `ai/rules/repo-maintenance.md` and `planning.md` call `block-premature-stop.sh` inert | done | Corrected in phase 6 as a stale-comment fix: the hook IS registered on `Stop` with `blocking: true`. The Known Limitation is now historical |
| `internal/test/runner/runner.go` `TestHelperBuildTags` has no cross-package non-test caller | deferred, and already homed | `make ze-validate`'s one issue. PROVEN pre-existing: `git archive HEAD` into a clean tree and `python3 scripts/dev/validate.py` there reports the identical single issue. `plan/spec-fixit-unexport-package-private-symbols.md` (in-progress, phase 7/8) was created from this check's own log of 467 such findings and its AC-3 is that none remains, so this row belongs to that spec's remainder |

No foreign shard was emptied by these resolutions, so none is removed.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/session-bin-directory-ad6179cd-01b3-45c8-a280-7857979322ce.md`, 77 files hashed, verdict `clean` |
| `review_gate.py check` | `OK (33 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | Round 1: logic+wiring; security+edge-cases; spec-completeness (AC-1..AC-28, no-layering, stale comments, assumption re-validation). Round 2: scoped to the fixes round 1 drove and what they touched |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `make ze-clean-tmp` can `rm -rf` the whole scratch tree. `find tmp/ -maxdepth 1 -type d` yields `tmp/` itself at depth 0 and neither `-not -name` excludes it, so a stale root mtime takes `tmp/session/` with every session's binaries, store and digest. The diff adds a comment beneath it claiming the opposite | `Makefile` `ze-clean-tmp` | `-mindepth 1`. Regression: `TestCleanTmpPreservesSessionRoot.test_a_stale_tmp_root_is_not_swept_as_its_own_child`, which makes the ROOT stale rather than a child. Mutation-verified |
| 2 | BLOCKER | The `tmp/`-root guard fails open on two of the three spellings of the same file: `./tmp/x` and the absolute `<repo>/tmp/x` pass on Bash while the Write surface refuses them, under a shared-module docstring saying both surfaces must refuse the same paths | `.claude/hooks/pretool-bash.py` `check_scratch_path`, via `_SCRATCH_WRITE` | the candidate regex takes a leading directory prefix; `is_ad_hoc_root_file` already resolved the parent, so the refusal did not widen. Four parity rows including an allow-control (`sub/tmp/notes.txt`). Mutation-verified |
| 3 | ISSUE | Four file headers state that `TestMakeAndGoAgreeOnBinDir` is what stops the language copies drifting, while the test drove two of the four. R-11 is this spec's own named risk, and the claim is the shield that stops the next reviewer checking | `scripts/dev/session_bin_dir_test.py` `TestMakeAndGoAgreeOnBinDir` | the test now drives all four, against a directory dated YESTERDAY so the lookup branch is the one under test. Mutation-verified in the shell and python copies |
| 4 | ISSUE | Check-then-act in the seeder: `make -j build` runs three `ze_core` recipes that each call it, so three `ze init` runs race on one database and the losers fail the build | `scripts/dev/session-seed-store.sh` | an atomic `mkdir` lock, a bounded wait on the winner's postcondition, and a re-check under the lock. `test_a_parallel_build_seeds_exactly_once` asserts `ze init` ran once. Mutation-verified |
| 5 | ISSUE | `make clean` destroys this session's per-spec digest, and the comment beside it says it removes only `scratch/`. Phase E1 moved the digest inside the dated directory; `--clean` has always taken the whole directory | `Makefile` `clean` | the comment and the echo now state all three subdirectories and name what losing the digest costs. The behaviour is correct: `make clean` is operator-invoked and says "clean this session" |
| 6 | ISSUE | Five comments still describe the deleted age-out or session-end sweep as live, and one names the test file this diff deletes | `.claude/hooks/lib/session_id.py` `_mint_cached`; `internal/test/sessionpath/sessionpath.go` `EnsureScratchRoot`; `internal/test/runner/runner.go` `NewRunner`; `scripts/checks/tracked_build.go` `runSelftest`; `mk/test-unit.mk` | each rewritten against the producing code (`ai/rules/stale-comments.md`) |
| 7 | NOTE, fixed | `BEFORE` is spliced into a double-quoted shell literal before its own format check, so `make ze-clean-sessions 'BEFORE=";touch pwn;x"'` runs that `touch` and only then reports a bad date | `Makefile` `ze-clean-sessions` | `export BEFORE` and `$$BEFORE`, so the value is data. `test_a_before_that_is_shell_metacharacters_runs_nothing`. Mutation-verified against the real injection point |
| 8 | NOTE, fixed | make and Go take the first glob match of ANY type; the shell and python copies require a directory. A dated regular file splits the four | `mk/session.mk` `ZE_SCRATCH_DIR`; `internal/test/sessionpath/sessionpath.go` `Root` | make's wildcard took its directories-only trailing slash with `$(patsubst %/,%,…)`; `Root` skips non-directories. `test_a_dated_regular_file_is_no_session_directory_in_any_copy`. Mutation-verified in both |
| 9 | NOTE, fixed | The quote guard's comment claims "a quote-free id cannot inject shell", which is true of the shell layer only: make expands `$(…)` in the incoming value at the first reference, which is the guard itself | `mk/session.mk` (the id-validation block) | the comment now bounds its own claim and says why make's layer crosses no privilege boundary |
| 10 | NOTE, fixed | "the session directory carries three subdirectories" under-describes it: `testbin-<id>/` and `ze-functional-*/` are siblings of the three | `.claude/hooks/lib/session-dir.sh` (header) | the header names them |
| 11 | NOTE, fixed | `ze-clean-sessions` property 1 says "only a dated directory inside that root", but `ZE_SESSION_ROOT` is a plain `:=` a caller can override, which the tests themselves do | `Makefile` (the `ze-clean-sessions` comment) | property 1 now says the SHAPE is the bound, not the root, and names the test that relies on it |
| 12 | NOTE, not fixed | AC-7's wording ("no marker deletion anywhere in `.claude/hooks/`") is broader than what was built: `_release_session` still `rm -f`s the claim marker | `.claude/hooks/lib/state-file.sh` `_release_session` | correct as built and operator-invoked from `/ze-close`. Recorded in the AC-7 audit row so a later reader does not "fix" the release path |
| 13 | NOTE, not fixed | AC-4 says "two session ids build CONCURRENTLY"; no test runs two builds at once | `scripts/dev/session_bin_dir_test.py` | what is proven is stronger in practice: the path is a pure function of the validated id, so two ids name two directories and neither can write the other's path |
| 14 | NOTE, not fixed | `internal/test/runner/runner.go` `TestHelperBuildTags` has no cross-package non-test caller, so `make ze-validate` reports one issue | `internal/test/runner/runner.go` | PRE-EXISTING at HEAD, proven by running `validate.py` over a pristine `git archive HEAD` tree. Homed at `plan/spec-fixit-unexport-package-private-symbols.md`, whose AC-3 owns the whole class. Fixing it here would collide with that live session and fold an unrelated change into a closing commit |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/session_bin_dir_test.py` | Yes | `ls -l` reports a regular file, 37 KB |
| `scripts/dev/session-seed-store.sh` | Yes | `ls -l` reports mode `-rwxrwxr-x`, executable as the recipe requires |
| `.claude/hooks/lib/session-dir.sh` | Yes | `ls -l` reports a regular file |
| `.claude/hooks/lib/scratch_path.py` | Yes | `ls -l` reports a regular file |
| `plan/deferrals/session-bin-directory.md` | No, correctly | `ls` gives No such file or directory; no deferral was taken |
| `scripts/dev/session_bin_suffix_test.py` | No, correctly | deleted by phase 2; `git ls-files` still lists it until the commit records the removal |
| `test/parse/*.ci`, `test/encode/*.ci` | Yes | the suites named in the Wiring Test are the repository's existing ones; this spec adds no `.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-3, AC-4, AC-5, AC-6, AC-8, AC-12, AC-13, AC-14, AC-15 | the layout, the seeding, the cleanup target and the retired vocabulary | `python3 scripts/dev/session_bin_dir_test.py` gives `Ran 33 tests … OK`, run after every closure fix |
| AC-2 | off-session is `bin/ze` | `env -u CLAUDE_CODE_SESSION_ID -u ZE_SESSION_ID make ze-path` gives `bin/ze`; and `make -n` for `ze`, `ze-appliance`, `ze-stripped`, `ze-setup-bin` gives `mkdir -p bin` with `-o bin/<bare-name>` and no seed line |
| AC-1 (no side effect) | `ze-path` creates nothing | `make ze-path ZE_SESSION_ID=zesbd-close-probe` printed the path; `ls -d tmp/session/2026-08-10-zesbd-close-probe` gives No such file or directory |
| AC-7, AC-10, AC-16 | no automatic deletion survives, and the operator sweep cannot take the root | `python3 scripts/dev/session_scratch_test.py` gives `Ran 14 tests … OK` |
| AC-12 | the retired vocabulary is gone | `grep -rn 'ZE_BIN_SUFFIX\|ZE_BIN_NAMES\|reap_binaries\|bare_named_perf' Makefile mk/ scripts/ .claude/ internal/ test/ ai/ docs/`, excluding `*_test.py` and `RETIRED.md`: EMPTY, run at closure |
| AC-16 | the four banned deletion idioms are gone | `grep -rn 'mmin +1440\|reap_dead\|rm -rf .*tmp/session' .claude/hooks/ scripts/dev/`, excluding `*_test.py`: EMPTY, run at closure |
| AC-17..AC-22 | id cache key, digest location, flat markers | `python3 scripts/dev/hook-fixture-check.py` gives `356/356 passed`, run after every closure fix |
| AC-11, AC-23..AC-26, AC-28 | the guards refuse and allow exactly the pinned corpus | `python3 scripts/dev/hook-parity-check.py` gives `200/200 match`, and `make ze-hook-test` OK, both run after every closure fix |
| AC-27 | held OPEN by owner instruction | `.claude/hooks/lib/scratch_path.py` names no layout, so `tmp/s/<id>/` is accepted by construction; the parity corpus still carries both layouts. Nothing was added to keep it and nothing was removed |
| every rule point and digest | fresh | `make ze-rules-lint` 28 conform; `ze-rules-condensed-check`, `ze-rules-render-check`, `ze-rules-index-check`, `ze-rules-points-roundtrip` all rc=0; `ze-rules-gate-map` DANGLING 0, PUBLISHED 0 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-path ZE_SESSION_ID=<id>` to `mk/session.mk` `ZE_BIN_DIR` | `TestZePathIsSessionDirectory` (python, not `.ci`) | Yes, inside the 33/33 pass, and invoked directly: `tmp/session/2026-08-10-zesbd-close-probe/bin/ze` |
| `make ze-path` with no session id to the off-session branch | `TestZePathOffSessionIsSharedBin` | Yes, and invoked directly with the env var unset: `bin/ze` |
| `make ze ZE_SESSION_ID=<id>` to `mkdir -p $(dir …)` + `-o` | `TestSessionStoreIsSeeded.test_a_stripped_only_build_seeds_its_store` runs the REAL `make ze-stripped` | Yes, plus a live 85 MB `make ze` in phase 2 |
| `ze-test` under a session, `sessionpath.BinDir` agreeing with make | `TestMakeAndGoAgreeOnBinDir` | Yes. The test compiles a throwaway Go probe printing `sessionpath.BinDir(".")` and compares make's directory to it, and now compares the shell and python resolvers too |
| a `.ci` suite under a session, bare-name exec off one PATH entry | `test/parse/*.ci`, `test/encode/*.ci` | Yes. The suites exec `ze`/`ze-stripped` by bare name off the runner's single PATH entry (`internal/test/runner/runner_exec.go` `childPathEnv`), which is what A-4 confirms and what bare naming preserves. `make ze-parse-test` ran 308/308 on-session during the review, building into `tmp/session/<dated>/testbin-…/bin/` |
| a Bash redirect or a Write naming `tmp/<file>`, in any of its three spellings, to a refusal | `hook-parity-check.py` goldens | Yes, read row by row on both surfaces, exit 2, with `sub/tmp/notes.txt` as the allow-control. This gate fired on this closure session itself, which is why every log here is under `$(scripts/dev/session-scratch.sh)` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/core/paths/paths.go` `isBinDir` / `ConfigDirFromBinary` read; reproduced empirically, a session binary resolves `<session-dir>/etc/ze` |
| A-2 | confirmed | `internal/component/config/storage/blob.go` `NewBlob` read: `zefs.Create` then `err == nil`. This is why AC-8 exists |
| A-3 | confirmed | timed: 18.54 s to build, 4.91 s to a different `-o` path, `cmp` byte-identical |
| A-4 | confirmed | `runner_exec.go` / `runner.go` `childPathEnv` read; `make ze-parse-test` ran 308/308 on bare names during the review |
| A-5 | confirmed | `ls -ld tmp` gives a real directory in this checkout. AC-9 was therefore proven on a FIXTURE checkout whose `tmp/` is a symlink, not on this one |
| A-6 | confirmed | working prototype: seeding works, isolation holds both ways |
| A-7 | confirmed, with a correction | `--yes` alone exits 1 (`username is required`); credentials arrive as five stdin lines. Recorded in the Mistake Log |
| A-8 | confirmed | `Makefile` `ze-clean-tmp` excludes `session` from its `-type d` sweep, and after the Review Gate the sweep can no longer match `tmp/` itself. `TestCleanTmpPreservesSessionRoot` pins both, with controls that MUST be swept |
| A-9 | confirmed, with a correction | the `-type f` sweeps never match a directory and the one `-type d` sweep excludes `session`, but it DID match `tmp/` itself at depth 0, which the assumption's wording missed. Closed with `-mindepth 1` and a mutation-verified test rather than by narrowing the claim |
| A-10 | confirmed | `/proc/<pid>/stat` field 22 and `ps -o lstart=` both answered; `session-id-start-time-readable` and `-ps-branch` pin both readers |
| A-11 | **BROKEN** | one live caller and three fixtures invoking `_cleanup_stale_markers` by name. Mistake Log row written; Deviations records that phase 3 REWROTE the fixtures rather than deleting them. Re-verified at closure: `grep -rn _cleanup_stale_markers .claude/ scripts/` is empty and `hook-fixture-check.py` is 356/356 |
| A-12 | confirmed | `git check-ignore -v <session-dir>/etc/ze/.dev-password` resolves `.gitignore` `tmp/*`; `git status --porcelain` never shows it; the test asserts mode 0600 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1 New user-facing feature? No | no shipped code path changed; `internal/core/paths` is untouched | Yes |
| 2 Config syntax? No | no YANG, no parser change | Yes |
| 3 CLI command? No | `ze-path` and `ze-clean-sessions` are make targets, not `ze` commands | Yes |
| 4 API/RPC? No | none | Yes |
| 5 Plugin? No | none | Yes |
| 6 User guide page? No | grep of `docs/` for the session-binary convention finds only `docs/functional-tests.md`, which row 10 owns | Yes |
| 7 Wire format? No | none | Yes |
| 8 Plugin SDK? No | none | Yes |
| 9 RFC behaviour? No | no protocol code in the diff; `docs/features/rfc-status.md` untouched and unimplicated | Yes |
| 10 Test infrastructure? **Yes** | `docs/functional-tests.md`: the `tmp/testbin-<id>/` paragraphs now name the off-session spelling with the on-session one beside it, and the cleanup sentence splits `ze-clean-tmp` from `ze-clean-sessions`. Its `<!-- source: mk/test-functional.mk … -->` and `<!-- source: internal/test/runner/runner.go … -->` anchors were re-read against the changed files and still hold | Yes |
| 11 Daemon comparison? No | `docs/comparison.md` states no build-artifact convention | Yes |
| 12 Internal architecture? **Yes** | `ai/rules/commands.md` (rendered from its points), `ai/INDEX.md` rows for `make ze-path`, `session-scratch.sh` and `stress-repro.py`, the `mk/session.mk` header, `.claude/hooks/README.md`'s new "The session directory" subsection, and `docs/architecture/testing/qemu-integration.md`'s second 9p share | Yes |
| 13 Route metadata? No | none | Yes |
| 14 Prometheus counters? No | none | Yes |
| 15 Registered plugin/command inventory? No | no registry surface; `ze-rules-gate-map` PUBLISHED 0 | Yes |
| 16 Doc source anchors on changed files? **Yes** | grepped `docs/` and `ai/` for `source:` anchors naming every changed file. Hits: `internal/test/runner/runner.go` (4), `mk/test-integration.mk` (6), `mk/test-functional.mk` (4), `Makefile` (8), `scripts/checks/tracked_build.go` (1), `test/perf/run.py` (1). Each anchored claim re-read: none names a binary path, a suffix, or a sweep, so none went stale. The one that did, `docs/functional-tests.md`'s testbin paragraph, was rewritten in phase 6 | Yes |
| 17 Existing examples for this area? **Yes** | `docs/functional-tests.md` command examples are the off-session spelling and are still accurate; the on-session note now sits beside them rather than leaving them reading as universal | Yes |
| Doctor check | No new runtime dependency | `scripts/dev/session-seed-store.sh` runs at BUILD time from a make recipe. It adds no file path, socket, port, module, cert or external binary the daemon needs at run time, so `ai/rules/repo-maintenance.md`'s doctor rule does not fire. `internal/core/diagnostic/codes.go` is untouched | Yes |
| `make ze-doc-test` | rc=2, sole driver FOREIGN | The only failing line is `WARNING: ai/DOCS-TO-CODE.md is stale`. Re-verified at closure rather than assumed, and again after the closure fixes touched three `.go` files: regenerated with `make ze-discovery-index`, diffed against the committed file, restored. The delta is ONE row, the removal of `internal/component/bgp/rib/update.go`, a file a SIBLING session deleted. `docs_to_code.py` reads `// Design:` headers, and no closure edit touched one. `ai/PACKAGE-MAP.md` is fresh (623 packages, up to date). Both are kept OUT of the commit. The two `NOT AT HEAD` lines name this closure's own journal files and clear when the commit records them | Yes |
| `make ze-validate` | rc=2, 1 issue, FOREIGN and PRE-EXISTING | `internal/test/runner/runner.go` `TestHelperBuildTags` has no cross-package non-test caller. PROVEN pre-existing rather than assumed: `git archive HEAD` into a clean tree and `python3 scripts/dev/validate.py` there reports the identical single issue. This diff changes only a doc comment in that file and touches neither the symbol nor any caller. Already homed at `plan/spec-fixit-unexport-package-private-symbols.md`, whose AC-3 owns the class | Yes |
| `make ze-spec-citation-check` | rc=0 | green. `grep -rn "plan/spec-session-bin-directory" plan/*.md` finds ONE hit, this spec's own Closure checklist. The spec is not being removed, so that citation resolves and nothing needs repointing | Yes |
| `make ze-lint-changed` | rc=0 | `0 issues` over `./internal/test/cli`, `./internal/test/runner`, `./internal/test/sessionpath`, re-run after the closure fixes | Yes |

## Core Insight

**A per-session NAME is a shared namespace; a per-session DIRECTORY is not.** Every
guard this spec deleted existed to police that namespace: the binary-name collision
check, the argv[0] personality-dispatch shim, and the name-glob sweeper that had to
parse `ZE_BIN_NAMES` out of a makefile to know what it must not delete. A directory
retires all three at once, which is why a change that adds a feature is mostly
deletion.

The sweeper generalizes past this spec. Its 600 MB of misses were not a bug in the
glob. Removal keyed on a NAME can only reach what it still recognizes, so anything
named by a process that died, or named slightly differently, is invisible to it
forever. Removal keyed on a CONTAINER has no such blind spot. Once that was true,
automatic deletion stopped paying for itself, and the owner could remove it: the
operator reads the directory by eye, because it is dated, and removes it by glob.

The Review Gate added a third, and it is about guards rather than cleanup. **A guard
is written on one spelling of its subject, and its author's own corpus uses that
spelling.** Both blockers this closure fixed have that shape: `check_scratch_path`
refused `tmp/x` and passed `./tmp/x` and the absolute path, with 196 green rows that
all spelled it the refused way; `ze-clean-tmp`'s two `-not -name` exclusions read as
covering `tmp/session/`, while `find`'s depth-0 entry was a spelling of the root that
neither name could match. In both cases the decision function was already correct and
only the thing that FED it was narrow. The test that finds this is never another row
in the same corpus. It is enumerating the spellings of the subject, then driving the
guard from its entry point with each one.
