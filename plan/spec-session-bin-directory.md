# Spec: session-bin-directory

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | `plan/spec-session-scoped-build-artifacts.md` (supersedes one of its decisions) |
| Phase | - |
| Deferral shard | `-` (create `plan/deferrals/session-bin-directory.md` on the first deferral) |
| Updated | 2026-08-03 |

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
| Which root survives | **`tmp/session/`.** `tmp/s` needs a doc lookup to decode (`ai/rules/naming.md`), and `tmp/session/` is already the root hosting the files that structurally cannot move into a per-session directory (below) |
| Directory name | **`tmp/session/<YYYY-MM-DD>-<sid>/`** — the date prefix makes manual, date-ordered cleanup possible by eye and by glob |
| Deletion | **No automatic deletion of anything under `tmp/session/`.** Not at SessionEnd, not on an age timer. Cleanup is manual, or an explicit make target the operator runs |

**Why the flat marker files stay flat, and keep their names.** Three of them cannot live
inside a per-session directory at all:

| File | Keyed by | Why it cannot move |
|------|----------|--------------------|
| `.sid-by-pid-<clipid>` | CLI-ancestor PID | It *mints* the session id (`session_id.py` source 4). A directory named after the id cannot hold the file that produces the id |
| `.closure-ack-<stem>` | spec stem | Outlives the session that wrote it; a later session on the same spec must read it |
| `session-state-<stem>-<sid>.md` | spec + session | Read ACROSS sessions for handoff (`state-file.sh:42` globs other sessions' files) |

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
is what stops the three drifting (`plan/learned/1246`: three independent derivations of the
session id drifted for weeks behind a prose "MUST stay identical" invariant).

**This reverses a recorded owner decision.** `plan/spec-session-scoped-build-artifacts.md`
records the suffix as chosen *over* `tmp/s/<id>/bin/`, twice, marked `owner-selected`, on the
grounds that relocation repoints config/DB resolution away from the live `<repo>/etc/ze`.
That objection is real and verified (see Current Behavior). It is not ignored here: the
owner's answer to question 1 **dissolves** it by making per-session isolation the intent
rather than the accident. The rationale must survive into the superseding record
(`ai/rules/spec-preservation.md`), which is why it is quoted in Key Design Decisions.

Non-goals: `bin/gok`, `ze-host`, and the cross-compiled installer keep their current
locations (already Known Limitations of the predecessor spec). No change to what a human or
CI sees.

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

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

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
| Registration over hardcoding | N-A | Build tooling; no registry surface. The related discipline here is `derive-not-hardcode.md`: `ZEBIN_*` stays derived from one `ZE_BIN_DIR`, never spelled per-binary |

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
| A-11 | ~~Deleting `_cleanup_stale_markers` breaks no caller~~ | — | — | `grep -rn _cleanup_stale_markers .claude/ scripts/`, 2026-08-03 | **BROKEN.** One live caller (`session-start.sh:11`) and **three fixtures that invoke it by name** (`hook-fixture-check.py:1166`, `:2281`, plus the R-11 UUID-sid case at `:1144`). One of them (`:2247-2262`) asserts a live session's claim must NOT age out — behaviour that becomes vacuous once nothing ages out, so the fixture must be rewritten to assert the new contract, not deleted. Two further comment references (`session-end-summary.sh:37`, `block-premature-stop.sh:152`, `commit_helper.py:962`) describe the sweep and go stale (`ai/rules/stale-comments.md`). Phase 3 owns all of it |
| A-12 | Nothing under `tmp/` is tracked, so dated dirs and session binaries can never be committed | `.gitignore:15` `tmp/*` | per-session SSH host keys could reach a commit | read the producer, 2026-08-03 | **confirmed** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | **A session's `ze` uses an unseeded store.** The severity SPLITS by entry point, measured not assumed: the `zefs.Open` paths (`ze data`, CLI credentials) fail **loudly** (exit 2, verified); the `NewBlob` paths (`resolve.Storage`, `ResolveSSHStorage`) create an empty store and return `err == nil` (A-2, read but not yet run) — those are the silent ones. A relocated binary also creates `etc/ze/crash/` as a side effect even on a failed command (verified) | `ze data ls` exit 2; or a daemon that starts with zero users | AC-8: `make ze` seeds the session store on first build via the `printf \| ze init` pattern (A-7), and the seeding is asserted, not assumed |
| R-2 | **A new SSH host key per session.** `ResolveSSHStorage` writes a fresh key into the session store, so pinned clients fail verification | host-key mismatch on `ze connect` | Same seeding step; documented as intended isolation in `ai/rules/bash-output.md` |
| R-3 | **QEMU 9p cannot see `tmp/session/` on a migrated checkout.** `ensure-links.py` can make `tmp/` a symlink to `$TMPDIR/ze/<id>`; the share covers the repo root only, so `/workspace/tmp/session/<sid>/bin/ze` would not resolve in the guest | `ze-qemu-*` fails to exec the DUT; latent today (A-5) | AC-9: share the resolved `tmp/` target as a second 9p mount when `tmp/` is a symlink. A latent silent break is the worst shape, so this is fixed, not deferred |
| R-4 | ~~**`make ze-clean-tmp` reaps a live session's binaries.**~~ **DISSOLVED 2026-08-03** by the root rename: `ze-clean-tmp` already excludes `session` (`Makefile:747`, `-not -name session -not -name kernel`), so moving scratch under `tmp/session/` puts it inside the existing exclusion. AC-10 becomes an assertion of something already true rather than a change | — | — |
| R-9 | **PID reuse aliases a dead session's id.** `session_id.py` source 4 caches a minted id at `.sid-by-pid-<clipid>`, keyed on the CLI-ancestor PID. Its own docstring (`:196`) says reuse "cannot alias a stale marker set **once the cache file ages out**" — so the 24h sweep at `state-file.sh:87` is CORRECTNESS, not housekeeping. Removing automatic deletion removes it, and a reused PID then makes a new session adopt a dead session's spec claim and gate markers: incident 1162/1246 reopened | a session resolves an id it never minted; a spec claim it did not make | AC-17/AC-18: key the cache on **PID + process start time** (`/proc/<pid>/stat` field 22; `ps -o lstart=` on macOS/BSD). A reused PID carries a different start time, so the entry self-invalidates and needs no expiry at all — strictly better than the timer it replaces. This is the ONLY file under `tmp/session/` whose expiry did real work; `state-file.sh:88-91` records the rest as pure accumulation control |
| R-10 | **`tmp/session/` grows without bound.** With no automatic deletion, each session leaves a dated directory (hundreds of MB once binaries move into it) plus ~7 small markers. The predecessor already measured ~600 MB of orphans under the old scheme | disk pressure; `du -sh tmp/session` | Accepted by owner decision (2026-08-03): growth is the price of never deleting the operator's data unasked. Mitigated by making cleanup EASY rather than automatic — the `YYYY-MM-DD-` prefix (AC-14) and `make ze-clean-sessions BEFORE=…` (AC-15). Both are operator-invoked |
| R-11 | **The dated directory is resolved three times** (make, Go, shell) and they drift. `plan/learned/1246` is precisely this failure: three independent session-id derivations drifted for weeks behind a prose invariant, and a disagreement fails closed | a session builds into one dir and execs from another | `TestMakeAndGoAgreeOnBinDir` (already in the Wiring Test) extended to cover the glob-then-create rule, plus AC-13 for the midnight boundary. The rule is stated once in the Scope extension and MUST NOT be paraphrased per-implementation |
| R-5 | **The pipe-tail hook stops covering session binaries.** `EXPENSIVE_COMMAND` matches `(\./)?bin/ze[\w-]*` anchored on a command word | no red; the guard just silently stops firing | AC-11: extend the alternation, with a hook-parity golden case that fails without it |
| R-6 | Three prose copies of the "suffix on purpose" rationale disagree with the code after the change | grep finds the old claim | AC-12 greps for the retired vocabulary tree-wide |
| R-7 | `.claude/settings.local.json` allowlists `Bash(bin/ze-test:*)` etc., so every session-local invocation prompts | permission prompts on routine commands | Add `tmp/session/*/bin/…` prefixes; that file is user-local, so this is advisory, not gated |
| R-8 | **`ai/rules/CONDENSED.md` is a shared generated file and this checkout is under concurrent edit.** This spec requires regenerating it (`make ze-rules-condensed`) after the `bash-output.md` rewrite. Observed 2026-08-02 during the design phase: the index recorded `ai/rules/CONDENSED.md` and `ai/RFC-REQUIREMENTS.md` as unmerged (`UU`) with **no conflict markers in either file**, while HEAD moved under this session | `git status` shows `UU`, or the digest regenerates with a sibling session's rule text | Before regenerating, confirm no sibling session holds an uncommitted `ai/rules/*.md` edit (`ai/rules/rule-format.md`). Regenerate in the SAME commit as the rule edit, and never "clean" foreign rows out of a shared generated file (`ai/rules/git-safety.md`) |

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
| `TestZePathIsSessionDirectory` | `scripts/dev/session_bin_dir_test.py` | AC-1 | |
| `TestZePathOffSessionIsSharedBin` | same | AC-2 | |
| `TestSessionBuildCreatesItsOwnDirectory` | same | AC-3 | |
| `TestUnsafeIDFallsBackToSharedBin` | same | AC-5 (all 7 rejected forms) | |
| `TestValidationDoesNotRunShell` | same | AC-5, quote-bearing id neither builds nor executes (carried over) | |
| `TestMakeAndGoAgreeOnBinDir` | same | make's `ze-path` dir == `sessionpath.BinDir` | |
| `TestNoSuffixVocabularyRemains` | same | AC-12, greps for the retired tokens | |
| `TestCleanTmpPreservesSessionRoot` | `scripts/dev/session_scratch_test.py` | AC-10 | |
| `TestSessionEndDeletesNothing` | same | AC-7 — a SessionEnd payload leaves the dated dir, its `bin/`, and every marker in place | |
| `TestNoAutomaticDeletionRemains` | same | AC-16, greps `.claude/hooks/` + `scripts/dev/` for `-mmin +N -delete`, `reap_dead`, `reap_binaries` | |
| `TestSessionDirIsStableAcrossMidnight` | `scripts/dev/session_bin_dir_test.py` | AC-13 — resolve twice across a simulated date change, same dir | |
| `TestSessionDirsSortByDate` | same | AC-14 — dated prefix, lexical order, glob selects one month | |
| `TestCleanSessionsRefusesWithoutBefore` | same | AC-15 — no `BEFORE`, no deletion | |
| `TestCleanSessionsRemovesOnlyOlder` | same | AC-15 — boundary: a dir dated exactly `BEFORE` survives | |
| `TestMintedIdSurvivesLiveSession` | `scripts/dev/hook_session_id_test.py` | AC-18 — cache hit returns the same id | |
| `TestReusedPidMintsFreshId` | same | AC-17 — same PID, different start time, different id | |
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
  line (`:749`); add the `ze-clean-sessions` target (AC-15)
- `mk/appliance.mk`, `mk/gokrazy.mk` - the remaining 3 `mkdir -p bin` sites
- `mk/test-integration.mk` - `ZE_QEMU_*` rebuilt on `$(ZE_BIN_DIR)` instead of the suffix

**Root rename `tmp/s/` → `tmp/session/<YYYY-MM-DD>-<sid>/` (2026-08-03)**
- `internal/test/sessionpath/sessionpath.go` - `scratchRoot`/`Root` glob-then-create instead
  of `filepath.Join(baseDir, "tmp", "s", id)` (`:70`); doc comments at `:9`, `:34`, `:74`
- `internal/test/sessionpath/sessionpath_test.go` - the `tmp/tmp/s/<id>` cases (`:177`, `:211`)
- `scripts/dev/session-scratch.sh` - `dir=` (`:118`) and the whole header
- `scripts/dev/session_scratch_test.py` - the `tmp/s/sid-*` assertions (`:82`, `:100`, `:116`)
- `.claude/hooks/pretool-bash.py` - the `-o tmp/s/<id>/bin/` regex (`:109`)
- `scripts/dev/hook-parity-check.py` - the `tmp/s/x/ready` golden cases (`:92`, `:107`, `:579`, `:592`)
- `ai/rules/testing.md` (`:430`), `ai/rules/bash-output.md` (`:70`, `:96`, `:101`),
  `ai/INDEX.md` (`:198`, `:199`), `Makefile` (`:714`, `:721`, `:1005`), `mk/session.mk` (`:11`),
  `mk/test-functional.mk`

**Automatic deletion, REMOVED (owner decision 2026-08-03; deletions per `ai/rules/no-layering.md`)**
- `.claude/hooks/session-end-scratch.sh` - **delete the `rm -rf` of the session dir (`:48`)
  and the `bin/*-<sid>` loop (`:69-71`)**. Keep the spec-claim release (`:60`)? **No** —
  AC-7 admits no deletion; the claim ages out of relevance rather than being removed, and
  `block-premature-stop.sh` already heartbeats it with `touch -c`
- `scripts/dev/session-scratch.sh` - **delete `reap_dead` and `reap_binaries`** and the
  `--reap` flag; keep `--clean` (operator-invoked, so permitted)
- `.claude/hooks/session-start.sh` - **delete** the `find tmp/session/ … -mmin +1440 -delete`
  (`:22`) and the `--reap` call (`:24`)
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
- `ai/rules/bash-output.md` - rewrite the section, including the paragraph that explicitly
  rejects this design
- `ai/INDEX.md` - the `make ze-path` row
- `scripts/dev/session-scratch.sh`, `.claude/hooks/session-end-scratch.sh` - header comments
- `scripts/evidence/netns_qemu.py`, `scripts/evidence/qemu-run.py`,
  `scripts/dev/qemu_binary_paths_test.py` - `$(ZE_BIN_SUFFIX)` docstrings
- `plan/learned/HOOK-FRICTION.md` - the copy-paste workaround snippet
- `docs/functional-tests.md` - `bin/ze-test` command examples (off-session, so accurate;
  add the session note only if it reads as universal)

**Generated (regenerate, never hand-edit — `ai/rules/canonical-sources.md`)**
- `ai/rules/CONDENSED.md` via `make ze-rules-condensed`
- `ai/PACKAGE-MAP.md` / `ai/DOCS-TO-CODE.md` via `make ze-discovery-index` if doc comments move

## Files to Create
- `scripts/dev/session_bin_dir_test.py` - the layout gate (replaces the suffix gate)
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
| 12 | Internal architecture changed? | Yes | `ai/rules/bash-output.md`, `ai/INDEX.md`, `mk/session.mk` header |
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
   - Files: `.claude/hooks/session-end-scratch.sh`, `.claude/hooks/session-start.sh` (`:11`),
     `.claude/hooks/lib/state-file.sh`, `scripts/dev/session-scratch.sh`, `Makefile:749`
   - **A-11 is BROKEN, and this phase owns the fallout.** `_cleanup_stale_markers` has one
     live caller and three fixtures that invoke it by name. The fixture at
     `hook-fixture-check.py:2247-2262` asserts a live session's claim must NOT age out —
     that becomes vacuous when nothing ages out, so **rewrite it to assert the new contract
     (nothing is deleted, ever), never delete it** (`ai/rules/no-test-deletion.md`). The
     R-11 UUID-sid fixture (`:1144`) tested sid recovery inside the sweep; if that parsing
     has no other consumer it goes with the function, and if it does, it moves
   - Also stale after this phase: the sweep descriptions in `session-end-summary.sh:37`,
     `block-premature-stop.sh:152`, `commit_helper.py:962` (`ai/rules/stale-comments.md`)
   - Verify: AC-16 grep clean; a SessionEnd event leaves the directory intact;
     `python3 scripts/dev/hook-fixture-check.py` passes with the rewritten fixtures
3a. **Phase: Operator cleanup path** — make manual cleanup easy, since it is now the only kind
   - Tests: AC-15
   - Files: `Makefile` (`ze-clean-sessions`), `ai/rules/bash-output.md`
   - Verify: refuses without `BEFORE`; removes only strictly-older dated dirs; leaves the
     flat marker files and any dir dated on/after `BEFORE` untouched
4. **Phase: Seed the session store** — close R-1/R-2 before they can bite
   - Tests: AC-8
   - Files: `mk/session.mk` or the `ze` recipe
   - Shape (proven, A-7): `printf '%s\n' <user> <pass> 127.0.0.1 2222 <name> | $(ZEBIN_ZE)
     init --force --yes --seed`, guarded so it runs ONLY on-session and ONLY when
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
| **`tmp/session/` is the one root** (2026-08-03) | keep `tmp/s/` and move the flat markers into it; keep both | Owner-selected. `tmp/s` needs a doc lookup to decode (`ai/rules/naming.md`). `tmp/s/` was the cheaper rename — it is what Go, make and the `.ci` binary paths already produce — and legibility won over churn because the operator reads this directory by hand |
| **Dated directory `<YYYY-MM-DD>-<sid>`** (2026-08-03) | bare `<sid>`; a sidecar mtime; a manifest file | Owner-selected. Once deletion is manual, the operator must be able to see age with `ls` and select it with a glob. mtime does not survive a copy and is not visible at a glance; a manifest is a second source of truth |
| **No automatic deletion under `tmp/session/`** (2026-08-03) | keep the SessionEnd `rm -rf`; keep only the 24h age sweep | Owner-selected, and the owner's mechanism argument is correct: SessionEnd fires only on a clean end — it returns early on `reason=resume` (`session-end-scratch.sh:38`) and never runs on a kill — so it is both unreliable AND the deletion the operator was never asked about. Growth (R-10) is accepted as the price |
| Flat marker files keep their names and stay at the root | move all of them into `<session-dir>/` for a single `rm -rf` | The single-`rm -rf` argument dies with automatic deletion. Three of them cannot move at all (id minting, spec-keyed ack, cross-session handoff — see Scope extension), and the other six are written by ~10 hooks whose failure mode is a gate that silently stops firing |
| Cache key `<clipid>-<starttime>` | keep the 24h age-out for `.sid-by-pid-*` alone | The age-out is the only automatic deletion doing correctness work (R-9). Making the key self-invalidating removes the need for it entirely, rather than carving an exception into a policy the owner stated without one |

**Superseded decision, preserved verbatim** from
`plan/spec-session-scoped-build-artifacts.md` (`ai/rules/spec-preservation.md`):

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
  guard (`ai/rules/fail-closed-guards.md`). Unrelated to this work; needs its own home.
- **Pre-existing, found in passing:** `ai/rules/hook-mapping.md` and
  `ai/rules/spec-delegation.md` both state `block-premature-stop.sh` is unregistered and
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
- [ ] Learned summary written to `plan/learned/NNN-session-bin-directory.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-session-bin-directory.md` only
