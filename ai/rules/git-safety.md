# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Directives

Rationale: `ai/rationale/git-safety.md`

## Commit Rules

**FORBIDDEN as direct AI tool calls:** `git commit`, `git add`, `git rm`,
`git restore --staged`, `git stash`. Sessions share staging -- a bare
`git add` from one tool call is visible to every other session's
`git commit` and files cross-commit. So never issue these verbs as
separate tool calls. Instead package add + delete + commit into a single
script (so staging never sits open between calls, nothing left dangling),
then run that script yourself: `bash tmp/commit-<SESSION>.sh`. Committing
is allowed; committing outside a script is not.

**A shared single-file plan log cross-commits even with a correct, explicit
`--file` list.** The rule above fixes staging *timing*; it cannot fix staging
*granularity*. `git add <file>` stages the WHOLE file, including hunks
another session left uncommitted in it. The fix is to SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/deferral-tracking.md`), so `git
add plan/deferrals/<source>.md` stages only your row. Known failures live one
file per failure under `plan/known-failures/` (a `<make-target>-<test-name>.md`
shard, with `RESOLVED.md` archiving the history and `README.md` holding the
logging instructions), so `git add plan/known-failures/<make-target>-<test-name>.md`
stages only your entry. The hazard was observed twice on 2026-07-15/16 (before
either was sharded): one session's `deferrals.md` edits landed inside two
unrelated VRRP commits, and three concurrent sessions (ping, ipc, lg) each had
the single `deferrals.md` file in their own `--file` list at the same time.

Consequences (they apply whenever two sessions touch the same tracked file, so
keep each shard single-writer), in order of importance:

| Situation | Do |
|-----------|-----|
| Your rows in a shared plan file are already committed by someone else | Nothing. The content is correct and preserved; only attribution is off. NEVER rewrite history to reclaim it (`git revert`/`reset` are forbidden anyway). |
| You edited a shared plan file | Commit it promptly. The longer it sits, the likelier another session's commit absorbs it. |
| Your commit omits a shared plan file you edited | Check `git log -1 -- <file>` before assuming the edit was lost: another session probably committed it already. |
| You see foreign rows in a shared plan file's diff | That is expected, not misconduct. Do not "clean" them out; you would revert another session's work. |

Do not read a cross-commit as a rule violation by the other session. With
concurrent sessions and a shared single-file log it is structural.

**Explicit commit requests are a fast path.** When the user asks for a
commit, the implementation/review phase is over. Prepare the commit
script and run it immediately. Do not re-audit the implementation, run late
completeness/remaining-work tables, inspect speculative companion artifacts,
or rerun lint/tests just because commit was requested. Inspect only enough
state to avoid staging unrelated, ignored, generated, or out-of-scope paths.
If scope is ambiguous, ask one narrow question; otherwise proceed.

**Commit workflow:**
1. Use `scripts/dev/commit_helper.py session` to create or reuse the 8-char session ID stored in `tmp/commit-session-id-<claude-session>` (keyed per Claude session so concurrent sessions never share a script path).
2. Use `scripts/dev/commit_helper.py create` to write `tmp/commit-msg-<SESSION>-<tag>.txt` and `tmp/commit-<SESSION>.sh`. Pass `--file` once per explicit file, `--remove` for tracked deletions, `--replace` for the first logical commit, and `--append` for later commits in the same script.
3. The helper writes executable scripts, uses `git commit -F <message-file>`, rejects ignored/generated paths, and refuses to overwrite an existing script unless `--replace` or `--append` is explicit. It also **gates on verify-status**: `create` runs `verify-status.sh check` and refuses unless FRESH, or unless you pass `--unverified "<reason>"` (owner override, or a known-red logged in `plan/known-failures/`). This makes "verify before commit" enforced rather than honor-system.
   It further **gates on discovery-index freshness**: `create` refuses if a generated index (`ai/PACKAGE-MAP.md`, `ai/DOCS-TO-CODE.md`, `ai/LEARNED-FULL-INDEX.md`) is stale (run `make ze-regen`), or if the commit changes an index-feeding source (a `register.go`, a `.go` with a `// Package`/`// Design:` header, a `plan/learned/*.md`) but omits the regenerated index. Override with `--stale-index-ok "<reason>"`. With no CI, this is the only place index freshness is enforced. `create` additionally **warns (non-blocking)** when HEAD's committed index does not match HEAD's committed sources, which catches a prior commit that bypassed the gate; it detects this by re-running the generators against a materialized copy of HEAD, so it works even when the working tree carries unrelated uncommitted changes.
4. Lesson learned check: when a commit changes agent workflow, rules, tooling, verification, or discovery surfaces, include `plan/learned/NNN-<name>.md` in `--file`. If no reusable lesson is useful, pass `--lesson-not-needed "<reason>"`. For known-required lessons, pass `--lesson-required`.
   Allocate the number with `scripts/dev/commit_helper.py learned-next <slug>` -- it picks max(existing file prefixes)+1 and creates the file immediately, so concurrent sessions **sharing one working tree** cannot allocate the same number. Never hand-pick a number.
   **This does not extend across branches.** `learned_next` (`scripts/dev/commit_helper.py`) scans the local filesystem, so it cannot see a number allocated on a branch you have not merged yet. Two branches routinely allocate the same number and the duplicate only appears when they meet: the 2026-07-16 rebase of 12 local commits onto 25 upstream ones produced five collisions at once (1120-1124). Do not treat a duplicate as misconduct; it is structural, exactly like the shared-file cross-commit above. `make ze-learned-numbers-check` detects duplicates (it runs inside `ze-doc-test` and `ze-regen-check`) and `make ze-learned-numbers-fix` resolves them, keeping the most-referenced summary at the contested number and renumbering the rest. Run the check after any merge or rebase that brings in `plan/learned/`.
5. If the helper cannot express the commit shape, hand-write the same `tmp/commit-<SESSION>.sh` pattern and `chmod +x` it. Do not use heredocs. Always use `git commit -F <file>`.
6. Never end an output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. Put path on its own line or follow with a space.
7. Run the finished script yourself: `bash tmp/commit-<SESSION>.sh`. Then report the resulting commit SHA(s), included files, message file, script path, and verification evidence or skip reason. Do not add a late completeness or remaining-work review unless the user explicitly asked for one.
8. Before writing a commit script, read `.gitignore` and never `git add` ignored paths. Key ignored paths: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/`, `.codex/skills/`, `.agents/skills/`, `tmp/`, `/bin/`. Only add canonical sources (e.g., `ai/skills/`, `ai/INSTRUCTIONS.md`).

`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when it runs.
Run the finished script yourself with `bash tmp/commit-<SESSION>.sh`;
because the tool call is `bash <script>`, not a bare `git commit`, it
passes the hook that blocks the raw verbs. `git restore --staged <file>`
is allowed inside a commit script only; all other `git restore` variants
remain forbidden.

**`git rm` safety:** before using `git rm` in a commit script, verify
the file is tracked (`git ls-files --error-unmatch <file>`). For files
modified during implementation (specs, stubs), use `git rm -f` to avoid
"has local modifications" errors. Never `git rm -f` without first
committing the file's current state (see Spec Closure in planning rules).

**Helper format:**
```bash
# Single commit (most common):
scripts/dev/commit_helper.py create \
  --replace \
  --subject "hook: allow tee pipe, per-session log paths" \
  --body "Explanation of why the change was made." \
  --file .claude/hooks/pretool-bash.py \
  --file ai/rules/bash-output.md \
  --lesson-not-needed "hook fix, no novel pattern"

# Second commit in the same script:
scripts/dev/commit_helper.py create \
  --append \
  --subject "feat: add widget support" \
  --body "Implements widget rendering for the dashboard." \
  --file internal/component/web/widget.go \
  --file internal/component/web/widget_test.go

# Spec closure (remove spec file):
scripts/dev/commit_helper.py create \
  --append \
  --subject "spec: close spec-widget" \
  --remove plan/spec-widget.md

# With a learned summary:
scripts/dev/commit_helper.py create \
  --replace \
  --subject "rules: add goroutine lifecycle rule" \
  --file ai/rules/goroutine-lifecycle.md \
  --file plan/learned/042-goroutine-lifecycle.md
```

Key flags: `--replace` for the first commit in a session, `--append`
for subsequent commits. `--file` per path to add, `--remove` per
tracked path to delete. `--lesson-not-needed "<reason>"` when no
learned summary applies; `--lesson-required` to enforce one.
Body lines are wrapped to 72 characters. Subjects are single-line and
must be at most 72 characters.

The generated script has this shape:
```bash
#!/bin/bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# Commit a: type: subject line
# Lesson: not needed - hook fix, no novel pattern
git add -- \
  file1.go \
  file2.go \
  file3_test.go
git commit -F tmp/commit-msg-<SESSION>-a.txt
```

**Never suggest / ask / hint at committing.** Complete ALL work first
(testing, spec, docs, learned summary), then report. User decides.
Banned phrases: "ready to commit?", "shall I commit?", "we could
commit now", "want me to commit?".

**Never bypass hooks.** No `--no-verify`, no `--no-gpg-sign`. On GPG
failure (`gpg failed to sign` / `cannot open /dev/tty`), ask the user
to run `! echo test | gpg --clearsign` to unlock the agent, then
re-run the script.

## Commit Granularity

Single-focus commits: one logical change per commit. Same system =
one commit (feature + tests + docs). Multiple unrelated changes =
multiple commits, not one bundle. Unrelated bug fix = separate commit.
Review fixes from a review pass = one commit.

## Commit Ownership in Parallel Sessions (2026-07-10, owner decision)

When several sessions work the same tree, each session MUST commit the
features it is in charge of implementing -- never leave your own finished
work uncommitted for another session to sweep or strand. Scope every commit
script to your own files (explicit `--file` lists; verify
`git diff --cached --name-only` shows nothing foreign before running it).

Exception, per owner direction: an uncommitted improvement written by
ANOTHER agent may be included in your commit when it is IN SCOPE of the
feature you own (it edits a file your feature owns, or your ACs depend on
it). Name the inclusion and its origin in the commit body. Out-of-scope
foreign files stay untouched, always.

Refinement, per owner direction (2026-07-17): the "always" above governs
CODE. A concurrent session's out-of-scope NON-CODE files -- generated
discovery indexes, docs, tracking markdown -- MAY be swept into your commit
when doing so keeps the tree consistent or unblocks you; foreign source and
test files are never included. Name the inclusion and its origin in the
commit body. This does not clear the whole-tree closure gates: the deferral
homing check folds over every shard in `plan/deferrals/`, so a foreign entry
in any shard that lacks a real destination spec is surfaced at closure
regardless of what you commit -- home it in its own shard, do not paper over
it (the check is advisory, but the obligation is not).

## Before Any Commit

### Step 0: Does `ze-verify` apply?

BLOCKING only when the commit could plausibly affect build, tests, or
generated code.

| Files in commit | Run `ze-verify`? |
|-----------------|------------------|
| Any `.go`, `go.mod`, `go.sum`, `vendor/**` | YES |
| `Makefile`, `scripts/**`, build/CI config | YES |
| `*.yang`, generated code, codegen templates | YES |
| Anything that runs at build time or affects a binary | YES |
| `ai/**/*.md`, `.claude/**/*.md`, `plan/**/*.md`, `docs/**/*.md`, `README.md` | NO |

Mixed commit: one YES row -> run. Do not split a commit to skip.
Decision rule: "could this make a Go test fail or break the build?"
No = skip and note in commit summary. Unsure = run.

### Step 1: If `ze-verify` applies (BLOCKING)

`make ze-verify` (timeout 240s). Not `go test`, not any subset.
Before any verify target, check freshness. A FRESH status covers the
byte-identical tree and forbids rerunning `make ze-verify` or
`make ze-verify-changed`. The check output is qualified by mode:
`FRESH(ze-verify)` covers everything; `FRESH(ze-verify-changed)` is a
weaker pass (no full lint, no vet evidence, no cached full unit pass) --
for commit preparation treat both as FRESH, but when the work explicitly
needs the full gate (release evidence, repo-wide changes), a full
`make ze-verify` on a FRESH(ze-verify-changed) tree is permitted. A pass
recorded with skipped suites (`ZE_SKIP_SUITES`) reports STALE. `ze-verify` uses a two-pass strategy: cached
full pass (no `-race`) + `-race` only on component groups with changed
`.go` files. `ze-verify-changed` scopes to packages with uncommitted
`.go` changes PLUS packages committed since the last green verify
(`scripts/dev/changed-pkgs.sh`, baseline = `git_sha` in
`tmp/ze-verify.status`), so a package committed before it was verified is
still tested rather than skipped on the now-clean tree. For reactor concurrency changes, also run `make
ze-race-reactor`. Output writes: `tmp/ze-verify.log`, per-stage logs
under `tmp/verify/`, `tmp/ze-verify-failures.log`,
`tmp/ze-verify-failures.json`, and `tmp/ze-verify.status`.

```
[ ] 0. `scripts/dev/verify-status.sh check`. FRESH -> MUST NOT run `make ze-verify` or `make ze-verify-changed` again; note timestamp. STALE -> continue only if the table above says verification applies.
[ ] 1. `make ze-verify` (240s) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open that group's `tmp/verify/<nn>-<stage>.log`.
[ ] 2. Failure from current work: fix + re-run. Pre-existing: fix after primary task in separate commit; if >10 min, log to `plan/known-failures/` (one `<make-target>-<test-name>.md` shard per failure).
```

### Structural Gates Are Never Known-Red (BLOCKING)

The item-2 "log to `plan/known-failures/`" path is for **non-deterministic**
failures only -- flaky or environmental TEST reds (load-sensitive races,
GC-pressure pool flakes, host-specific listener probes). A **deterministic
structural gate** is NEVER eligible: `ze-lint`, `ze-lint-changed`, `ze-tier-check`,
`ze-vet-evidence`, `ze-plugin-boundary-check`, `ze-iface-resolution-check`,
`ze-regen-check-readonly`, and `ze-verify-wiring-docs` fail only when the tree is
structurally broken (a misplaced module tier, a lint/vet violation, a broken
plugin boundary, an unresolved iface, a stale generated file, a stale wiring
index). Such a red must be fixed at the source before any commit -- do not park
it, do not `--unverified` past it.

**The one escape is owner-only: `--structural-red-ok "<reason>"`.** It is a
SEPARATE flag from `--unverified` precisely so the flaky-test path can never
reach this branch, it refuses an empty reason, and it prints the red gate names
with the reason to stderr so a red tree can never look green in a transcript.
Use it only when Thomas says so and the red provably belongs to another
session's in-flight work that this commit cannot affect. It exists because a
refusal with NO escape made a green tree the only route to any commit at all,
including one touching no compiled code -- which pushed sessions toward the real
hole this gate was built to close: widening `--unverified`, or editing
`STRUCTURAL_GATES` to drop the failing name. An override that is written down
and shouted is safer than one that is improvised. It is never a substitute for
fixing your own red (`ai/rules/no-parking.md`).

`ze-regen-check-readonly` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `make ze-regen` (or the
specific `--fix` the failing check names). It is never flaky or environmental.

Known gap, recorded rather than papered over. Several checks run under BOTH
`ze-doc-test` and `ze-regen-check-readonly`. That overlap is harmless: the runner
continues across stage failures, so one underlying red fails both stages in the
same run, `structural_gate_reds` always sees `ze-regen-check-readonly`, and the
commit is blocked regardless of what `plan/known-failures/` says about
`ze-doc-test`. The real gap is the checks that run ONLY under `ze-doc-test` --
`doc_drift.go`, `commands.go`, `digest_check.py`, and `rfc_requirements.py
--check-fresh` (`mk/inventory.mk`; note the script's `--selftest`/`--check`
invocations DO run as the `ze-rfc-check` stage, so only the `--check-fresh`
ledger-staleness one is doc-test-exclusive). Those are just as deterministic and
structural, and they ARE
parkable, because `ze-doc-test` is not in the set. Whoever picks this up should
decide whether `ze-doc-test` belongs in `STRUCTURAL_GATES`; that is where reds
actually escape.

This list is the prose mirror of `STRUCTURAL_GATES` in `scripts/dev/commit_helper.py`,
and every name in it must be a stage `stagesForMode` actually emits
(`scripts/status/verify_run.go`) -- otherwise the entry matches nothing and gates
nothing. `test_structural_gates_are_live_stages` (`scripts/dev/commit_helper_test.py`)
and `TestStructuralGatesAreLiveStages` (`scripts/status/verify_run_test.go`) enforce
that. `ze-cli-grammar-check` was listed here until 2026-07-20 and was exactly that
dead entry: a real make target (`mk/inventory.mk`), but never a verify stage, so
`structural_gate_reds` could never match it. Its underlying gate is not lost --
`TestCLIGrammarGateStatic` (`scripts/checks/cli_grammar_test.go`) runs the real
checker under the unit stage.

This is enforced, not honor-system: `scripts/dev/commit_helper.py create` reads
`tmp/ze-verify-failures.json` (which `verify_run.go` rewrites after every run) and
refuses to prepare a script while any structural gate is red, even with
`--unverified` (`structural_gate_reds` / `STRUCTURAL_GATES`). A green verify
rewrites the artifact, so a fixed-and-reverified gate clears automatically. This
closed the hole that let a misplaced-tier gate (`routeinstall`) be logged as
"pre-existing" and ship red on `main` for a week (see `plan/known-failures/RESOLVED.md`,
2026-07-07).

### Thomas Owner Override: Commit Without Verify

Thomas owns the repository and may explicitly override the `ze-verify`
requirement for commit-script preparation. This override exists because an
OpenAI session blocked Thomas by treating the agent rule as if it also bound
the repository owner. It was added for OpenAI behavior, not for Anthropic.

The override is valid only when Thomas explicitly directs both parts:

1. prepare a commit script, and
2. skip tests, skip verify, or commit without running tests.

Examples that activate it: `owner override: commit without verify`, `commit no
test`, `commit without running tests`, or an equivalent direct instruction from
Thomas in the active conversation. Do not infer the override from urgency alone.

When the override is active:

- Do not run `make ze-verify`, `make ze-verify-changed`, lint, or tests as a
  late commit gate.
- Do inspect only enough state to stage exactly the requested files and avoid
  ignored, generated, unrelated, or user-owned paths.
- Do use `scripts/dev/commit_helper.py create` with the normal user-run script
  path. The override changes verification requirements only.
- Do not run `git add`, `git commit`, `git rm`, `git stash`, or prohibited git
  commands from an AI tool.
- Do not add `--no-verify`, `--no-gpg-sign`, disabled hooks, or any bypass to
  the generated script.
- Report `Verification skipped by Thomas owner override` in the final response
  and, when useful, in the commit body.
- Do not claim tests, lint, `ze-verify`, integrations, or behavior were
  verified if they were skipped.

### Known-Red Full Verify: Scope to Changed (BLOCKING)

When `make ze-verify` is known-red from failures this session did not cause --
pre-existing reds, or a separate session is actively clearing the global suite --
do NOT rerun full `ze-verify` before committing. Rerunning re-surfaces other
sessions' noise that is not your regression and blocks progress. Gate the commit
on changed scope only:

- `make ze-lint-changed`
- the touched packages' `go test` (or `make ze-verify-changed`)
- `make ze-doc-test` / `make ze-validate` when those surfaces changed
- a QEMU run for any linux-only runtime code touched

Then prepare the user-run commit script listing ONLY this session's files
explicitly in `commit_helper.py create --file ...`, so the commit never pulls in
another session's working-tree edits; exclude other sessions' files when
reviewing `git diff`. This applies only when the global red is not yours -- a red
caused by your own change must be fixed, not scoped around. Activate it only on an
explicit owner direction (e.g. "another session is clearing ze-verify, check only
what we changed"), never inferred from a red suite alone.

**The red must be attributed, not assumed (BLOCKING).** "Known-red" means you
have identified the specific failing stage/test and confirmed it is pre-existing
(logged in `plan/known-failures/`) or owned by another active session. An
*undocumented* red is NOT scope-aroundable: treat it as possibly your own
regression until proven otherwise. Scope-to-changed has a blind spot -- it tests
packages you edited, not packages your edit breaks **transitively**: a new import
can break a different package's compile/test (a real case: `aihelp` broke
`bgp/config` through `plugin/all`), a config-driven gap surfaces only in a
consumer's tests (a missing YANG typedef failed `bgp/config`, not the plugin that
introduced it), and adding a plugin invalidates the `plugin/all` golden
snapshots. Before scoping around a red, `go test`/`vet` the reverse-dependency
closure of your changed packages, or run full `ze-verify` once.

**Do not let a red persist.** Scope-to-changed is a temporary bridge while the
global suite is being cleared, not a standing mode. A `ze-verify` that stays red
across sessions hides newly-introduced breakage under the existing red -- that is
exactly how an import cycle, a YANG typedef gap, and stale registry snapshots all
landed under one persistent red without any gate firing. Log the failing stage in
`plan/known-failures/` with who owns clearing it; if nobody does, clearing it
comes before stacking more changes on top.

### Concurrent Verify Runs (BLOCKING)

One `make ze-verify*` (or `ze-chaos-verify`) at a time repo-wide --
parallel runs share build cache + ports + `bin/ze` processes and
trash each other. All variants are wrapped by
`scripts/dev/verify-lock.sh` (`flock` on `tmp/.ze-verify.lock`); a
second invocation blocks automatically.

| Do | Don't |
|----|-------|
| Let the second invocation block | Kill the running verify |
| If the run is yours (same tree), read `tmp/ze-verify.log` instead of re-running | Delete the lockfile |
| If "waiting for lock" appears, do other work | Start `go test` / `golangci-lint` / `bin/ze-test` in parallel (bypasses lock) |

Lock releases when the command exits. `flock` is fd-backed, not
PID-backed -- no cleanup after a crash.

### Running ze-verify

Foreground with 240s timeout. No background execution, no polling
loops. Wait for completion.

### Step 2: Always

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Learned summary written to plan/learned/NNN-<name>.md (NNN from `commit_helper.py learned-next <slug>`)
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left.
```

Unless Thomas Owner Override is active, never commit with lint issues and never
commit without test evidence when code changed.

## Forbidden Without Permission

`git reset`, `git revert`, `git checkout -- <file>`, `git restore`,
`git stash drop`, `git push --force`.

## Branch Changes Are Forbidden

Stay on the branch you started on. Never change branches, create
branches, delete branches, rename branches, or integrate branches from
an AI tool call.

Forbidden branch-changing commands include `git switch`, `git checkout
<branch>`, `git branch`, `git rebase`, `git merge`, and
`git cherry-pick`.

If branch movement or integration is needed, stop and ask the user to do
it manually.

## Before Destructive Actions

Save: `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Forbidden Raw Output

`git diff --stat` / `git status` dumped raw in output -- summarise.

## Branch Integration

When the user integrates a worktree branch manually, it lands on main
via `git rebase <branch>`, never `git merge`. Linear history.

## Rebase Onto Diverged main: driving the bookkeeping conflicts

A rebase of local commits onto a diverged `origin/main` re-conflicts on
`ai/LEARNED-FULL-INDEX.md` at nearly every learned-touching commit -- the
cross-branch learned-number collision covered in "Commit Rules" step 4 and
`plan/learned/1155`. That file is derivable, so drive the rebase with
`scripts/dev/rebase_learned.py`: the human starts (and, if needed, aborts) the
rebase; the script regenerates the index (via `learned_index.py`) at each stop
and HALTS on any other unmerged path. Resolve that file, then re-run with
`--take-theirs PATH` / `--take-ours PATH` / `--accept-incoming-delete` (each
logged, never silent). `--help` documents the flags and exit codes.

Finish the rebase first, then fix numbering -- never mid-rebase. Afterwards run
`make ze-learned-numbers-fix` (renumbers colliding summaries and rewrites their
references) and recompute any derived ratchet the rebase loosened (e.g.
`test/.ci-sleep-baseline` = actual `time.sleep(` count in `test/**/*.ci`).

Gotcha: `git rebase --continue` refuses with a MISLEADING "You must edit all
merge conflicts" whenever there are unstaged tracked changes, not only when
index entries are unmerged (`builtin/rebase.c` ACTION_CONTINUE checks
`has_unstaged_changes()`). rebase_learned.py detects this and names the real
files. The behavior is pinned by `TestRebaseContinueMessageIsMisleading` in
`scripts/dev/rebase_learned_test.py`, so believe the test, not the message.
Per "Branch Changes Are Forbidden" above, the AI never runs `git rebase`
itself; the script only resolves conflicts within a rebase the user started.

## GPG Signing

Never `--no-gpg-sign` / `-c commit.gpgsign=false`.
Never `--no-verify`.

On `gpg failed to sign` / `cannot open /dev/tty`, ask user to unlock
the agent, then retry.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
