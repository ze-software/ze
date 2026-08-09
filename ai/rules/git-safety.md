# Git Safety

**When:** before any git operation, and when writing or running a commit script
**Severity:** blocking

## Directives

Rationale: `ai/rationale/git-safety.md`

## Commit Rules

See: `ai/INSTRUCTIONS.md`, "git commit, git add, git rm: FORBIDDEN as bare Bash tool calls" -- the five banned verbs, the single add + delete + commit script, and "committing outside a script is not" reach every session from there, so this rule does not restate them.

**A shared single-file plan log cross-commits even with a correct, explicit
`--file` list.** The ban on the bare staging verbs fixes staging *timing*; it
cannot fix staging *granularity*. `git add <file>` stages the WHOLE file, including hunks
another session left uncommitted in it. The fix is to SHARD the log so each
session writes only files it owns and git merges disjoint creations without
conflict. **Both cross-spec logs are now sharded.** Deferrals live one file
per source under `plan/deferrals/` (`ai/rules/planning.md`), so `git
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
**One check is exempt, because it cannot run earlier: `make ze-tracked-build-check`
after the script has run** (step 7). It judges the commit you just made, which no
run before that commit could see.

**Thomas ruled on this exemption on 2026-08-04: KEEP IT.** It was raised twice as
a narrowing of the fast path, because it adds about 45 seconds to a commit that
carried Go. It is settled, so do not re-open it. The reasoning he accepted: the
check is not a rerun, since its input is a commit that did not exist until the
script ran, and it is the only thing that reads the population git holds. The
failure it prevents is unbounded where its cost is bounded and one-shot. HEAD was
unbuildable for 34 commits across more than a day (`eae57dfca`, 2026-08-03, to
`7abe8a07e`) precisely because the break was only discoverable at a full verify
that nobody in that window ran.
If scope is ambiguous, ask one narrow question; otherwise proceed.

**Commit workflow:**
1. Use `scripts/dev/commit_helper.py session` to create or reuse the 8-char session ID stored in `tmp/commit-session-id-<claude-session>` (keyed per Claude session so concurrent sessions never share a message or script namespace).
2. Use `scripts/dev/commit_helper.py create` to write one message file and one commit script. Pass `--file` once per explicit file and `--remove` for tracked deletions. The path is the `script=` line it prints (`ai/INSTRUCTIONS.md`). Keying the script on the session was enough while a session was one agent. One session now runs many subagents that share the session id. On 2026-08-05 one session produced 53 message files against 18 scripts, each `--replace` overwriting a sibling's prepared commit. `--push` adds a push after the commits, on an owner instruction only (see "Pushing").
   `--append` adds a later commit block to a script you already prepared. Pass `--script` with the path that create printed. Without `--script` it resolves only when the session has exactly one script, and otherwise refuses with the list. `--replace` rewrites the script `--script` names. It is refused when that script was prepared for a file set sharing nothing with yours. To start over, prepare a new one: a `create` without `--script` always gets its own path.
3. The helper writes executable scripts, uses `git commit -F <message-file>`, and rejects ignored/generated paths. It never writes over an existing script unless `--script` names it, with `--replace` or `--append`. It also **gates on verify-status**: `create` runs `verify-status.sh check` and refuses unless FRESH, or unless you pass `--unverified "<reason>"` (owner override, or a failure you tried and could not reproduce, logged in `plan/known-failures/`). This makes "verify before commit" enforced rather than honor-system.
   It further **gates on discovery-index freshness**: `create` refuses if a generated index (`ai/PACKAGE-MAP.md`, `ai/DOCS-TO-CODE.md`) is stale (run `make ze-regen`), or if the commit changes an index-feeding source (a `register.go`, a `.go` with a `// Package`/`// Design:` header) but omits the regenerated index. Override with `--stale-index-ok "<reason>"`. With no CI, this is the only place index freshness is enforced. `create` additionally **warns (non-blocking)** when HEAD's committed index does not match HEAD's committed sources, which catches a prior commit that bypassed the gate; it detects this by re-running the generators against a materialized copy of HEAD, so it works even when the working tree carries unrelated uncommitted changes.
4. Lesson learned check: the helper asks whether the commit ADDS content to an agent-workflow, rule, tooling, verification, or discovery surface, or only relocates it. Content earns a journal row: include `plan/journal/<class>.md` in `--file`. A move, a rename, or a reformat does not, and passes untouched. When the change adds content but taught nothing reusable, say so with `--lesson-not-needed "<reason>"`; `--lesson-required` is the operator demanding one regardless (`ai/rules/planning.md`, "Writing Journal Rows").
5. If the helper cannot express the commit shape, hand-write the same script pattern and `chmod +x` it. Give it a name no other agent will pick: `tmp/commit-<SESSION>-<tag>-<random>.sh`. Do not use heredocs. Always use `git commit -F <file>`.
6. Never end an output line with `.`, `,`, `:`, or `)` directly after a path/URL/command -- users copy-paste; trailing punctuation breaks it. Put path on its own line or follow with a space.
7. Run the finished script yourself with `bash` and the helper's `script=` path. **When the commit contained any `.go`, `go.mod`, `go.sum`, or `vendor/` path, run `make ze-tracked-build-check` immediately afterwards** (about 45s): it compiles what git now holds, and it is the only check that reads that population -- see "Your Working Tree Is Not What You Committed" below. Then report the resulting commit SHA(s), included files, message file, script path, whether the script pushed, and verification evidence or skip reason. Do not add a late completeness or remaining-work review unless the user explicitly asked for one.
8. Before writing a commit script, read `.gitignore` and never `git add` ignored paths. Key ignored paths: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/`, `.codex/skills/`, `.agents/skills/`, `tmp/`, `/bin/`. Only add canonical sources (e.g., `ai/skills/`, `ai/INSTRUCTIONS.md`).

`git commit`/`git add` inside the script is fine -- the ban is on
direct AI tool invocations, not on what the script does when it runs.
Run the finished script yourself with `bash` and the printed path.
The tool call is `bash <script>`, not a bare `git commit`, so it
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
  --file ai/rules/commands.md \
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

# With a journal row:
scripts/dev/commit_helper.py create \
  --replace \
  --subject "rules: add goroutine lifecycle rule" \
  --file ai/rules/goroutine-lifecycle.md \
  --file plan/journal/<class>.md
```

Key flags: `--replace` for the first commit in a session, `--append`
for subsequent commits. `--file` per path to add, `--remove` per
tracked path to delete. `--lesson-not-needed "<reason>"` when no
journal row applies; `--lesson-required` to enforce one.
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
(testing, spec, docs, journal row), then report. User decides.
Banned phrases: "ready to commit?", "shall I commit?", "we could
commit now", "want me to commit?".

## Pushing (2026-08-05, owner amendment)

- **A bare `git push` from a Bash call stays forbidden; the hook enforces it.**
- **Push only by passing `--push` to `scripts/dev/commit_helper.py create` (step 2); it runs from the script you run at step 7.**
- **The owner orders a push; you never decide one. `--push` on your own initiative is a push without authority.**
- **`git push --force` and `-f` stay forbidden; `--push` is no route to them.**

### Why the amendment, and what to do when a push goes wrong

Thomas wrote the absolute push ban and amended it on 2026-08-05: a push is
allowed, from the commit script only, and only when he has ordered that push.
His reason for the original ban is what makes the exception safe. It stopped a
partial `git add` landing while several agents shared one index, and one script
bundling add, remove, commit and push leaves no such window open.

The hook's refusal of the bare command is deliberate, not an oversight: it is
what forces every push through the script, where the mechanism stays visible to
the next reader. Writing a throwaway script that carries a push and deleting it
afterwards is banned for the same reason. It reaches the same remote by the same
hand and leaves no record of why the push happened, and `ai/INSTRUCTIONS.md`
carries that ban into every session. A remote history that needs rewriting is
the owner's decision, made at his own terminal, which is why `--force` and `-f`
have no `--push` path (`ai/INSTRUCTIONS.md`, "Destructive git commands are
FORBIDDEN").

| Situation | Do |
|-----------|-----|
| The script's commit step failed | Nothing is pushed and nothing should be: `set -euo pipefail` stops the script before the push. Fix the cause (staged index, GPG), then re-run the script |
| The commits are made and no push was ordered | Stop and report the SHA(s). A push nobody asked for is a push without authority, whatever the branch looks like |
| You are a worktree agent | Never push. Work on your branch and stop there (`ai/INSTRUCTIONS.md`, "Worktree agents must not touch main") |
| The owner orders a push after your script already ran | Say so and let him push, or carry `--push` on the next commit you prepare. Do not type the command to close the gap |

## Commit Granularity

Single-focus commits: one logical change per commit. Same system =
one commit (feature + tests + docs). Multiple unrelated changes =
multiple commits, not one bundle. Unrelated bug fix = separate commit.
Review fixes from a review pass = one commit.

## Commit Ownership in Parallel Sessions (2026-07-10, owner decision)

**A FAILED commit leaves the index STAGED, and the next session's commit inherits
it. Clear it before you walk away.** The script stages first and commits second, so
a commit that fails has already staged everything. On 2026-08-03 a GPG passphrase
prompt with no TTY failed the signing step, eleven files sat staged in the shared
index for roughly forty minutes, and a concurrent session's 1467-file commit took
ten of them. Nothing was lost and every file's content was intact, but the work
landed under another commit's message.

**The failure mode is invisible from the failed run.** It exits non-zero, prints
`failed to write commit object`, and reads as "nothing happened". The staging IS
what happened. After ANY failed commit in a shared checkout, read
`git diff --cached --name-only`, then either fix the cause and re-run at once or
unstage your own paths. A signing failure is the usual trigger precisely because it
fails LAST, after every gate has passed and every file is already staged.

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

`make ze-verify`, in the foreground ("Running ze-verify" below). Not `go test`,
not any subset.
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
[ ] 1. `make ze-verify` (foreground, largest timeout your harness allows, never killed early) only when status is STALE and the table above says YES. On failure read `tmp/ze-verify-failures.log` FIRST, choose a stage-local group, then open that group's `tmp/verify/<nn>-<stage>.log`.
[ ] 2. Failure from current work, or any failure that blocks this commit's goal: fix + re-run. Any other failure, and never a deterministic structural gate, which is fixed before any commit (see "Structural Gates Are Never Known-Red" below): write its spec, finish this commit, ask Thomas whether that spec runs (`ai/rules/completion.md`). A `plan/known-failures/` shard is for a failure you tried and could not reproduce, and it carries the reproduction attempt and the next step.
```

### Structural Gates Are Never Known-Red (BLOCKING)

The pre-commit checklist's "write its spec, finish this commit, ask" branch, and
its `plan/known-failures/` shard, are for **non-deterministic** failures only.
Those are flaky or environmental TEST reds: load-sensitive races, GC-pressure pool
flakes, host-specific listener probes ("Before Any Commit", above). A **deterministic
structural gate** is NEVER eligible: `ze-lint`, `ze-lint-changed`, `ze-tier-check`,
`ze-vet-evidence`, `ze-plugin-boundary-check`, `ze-iface-resolution-check`,
`ze-regen-check-readonly`, `ze-verify-wiring-docs`, and `ze-tracked-build-check`
fail only when the tree is structurally broken (a misplaced module tier, a
lint/vet violation, a broken plugin boundary, an unresolved iface, a stale
generated file, a stale wiring index, a HEAD that does not compile). Such a red
must be fixed at the source before any commit -- do not park it, do not
`--unverified` past it.

`ze-tracked-build-check` is the one entry whose red is cleared BY a commit
rather than before one. It judges what git already holds, so a broken HEAD is
fixed by committing the producer a previous commit left behind, and every other
gate on the list is fixed in the working tree first. Refusing every commit until
it goes green would therefore deadlock: the refusal would block the only commit
that can lift it. **`--broken-head-fix "<reason>"` is that commit's route
through**, and it is narrow by construction: `commit_helper.py` accepts it only
when tracked-build is the ONLY structural red, so a lint, tier or wiring failure
riding alongside still refuses. Run `make ze-tracked-build-check` after the
script and confirm it went green. If it did not, HEAD is still broken for
everybody who builds it.

**The general escape is owner-only: `--structural-red-ok "<reason>"`** (the
narrow `--broken-head-fix` above is the only other, and it reaches one gate).
It is a
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
fixing your own red (`ai/rules/completion.md`).

`ze-regen-check-readonly` qualifies on the rule's own terms: a stale generated
file is deterministic, reproducible, and fixed by `make ze-regen` (or the
specific `--fix` the failing check names). It is never flaky or environmental.

### Your Working Tree Is Not What You Committed (BLOCKING)

**Nothing else in this repository COMPILES what git holds.** `make ze`,
`ze-verify`, `ze-lint-changed`, `ze-rfc-check` and every test target build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `commit_helper.py` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So a commit
that takes a CONSUMER while its PRODUCER stays uncommitted is green for you and
broken for everybody who builds what git holds. On 2026-08-04 four commits broke
`make ze` at HEAD that way in one day (7abe8a07e, 025a74b72, aa1b7a4d4,
fa372140b), with every gate green at the moment each was made. It is a blind
spot, not four accidents.

`make ze-tracked-build-check` (`scripts/checks/tracked_build.go`) is the one
check that reads what git holds: it extracts the commit with `git archive` and
compiles six build flavors of the extracted tree. Three rules follow.

| Situation | Do |
|-----------|-----|
| You are about to `--file` a consumer | Name the file that DEFINES every symbol it newly uses, and check that file is in the same `--file` list or already committed (`git log -1 -- <path>`) |
| The commit script has just run and it carried Go | Run `make ze-tracked-build-check`. About 45s. This is step 7 of the commit workflow, not an optional extra |
| It goes red | Commit the producer. Never revert the consumer, and never park it: HEAD is broken for everyone until you do |

`REV=<commit-ish>` judges any commit, so a break found later is bisectable:
`make ze-tracked-build-check REV=7abe8a07e`. `ARGS=--keep` leaves the extracted
tree in place for inspection.

**What it does NOT read: test files.** `go build` never compiles `_test.go`, so a
test file committed without its fixture producer stays invisible here.

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

Each directive below is one physical line on purpose. `condense_body`
(`scripts/dev/rules_condensed.py`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `make ze-verify` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
No background run, no sleep-and-check loop, no `tail` on a log that is still
growing.

**Do not kill it for being slow. Give the call the largest timeout your harness allows.**
A verify that is still running is not a verify that is hung, and killing one
costs the whole pass rather than the seconds it saves.

**Never take a timeout from a duration written in a rule: read `tmp/.ze-verify-duration.txt` instead.**
How long a full pass takes depends on the machine, and on what else that machine
is doing. "25 to 30 minutes" below and "4-10 minutes" in `ai/rules/testing.md`
are not a contradiction. They are different hardware. A loaded VM is not
deterministic either, so even one machine gives a spread rather than a figure.
`_record_duration` (`scripts/dev/verify-lock.sh`) appends the real elapsed
seconds for the machine you are on, and `tmp/*` is gitignored, so that file is
the only per-machine record there is. Read it as an expectation, never as a
threshold: a run past it is a slow run, not a failed one
(`plan/learned/1359-rules-corpus-paraphrase-drift.md`).  <!-- doc-links: ignore -->

**A slow run can outlast the lock's own break threshold: raise `ZE_VERIFY_MAX_LOCK_AGE` rather than lose the pass.**
When a second invocation is waiting, `verify-lock.sh` breaks a lock whose holder
has run past `MAX_LOCK_AGE` (default 1800s) and SIGKILLs its process group. Half
an hour is often enough for a full pass and is not guaranteed on a loaded VM, so
that threshold can reach a healthy run rather than a stuck one.

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.

### A SHARED CHECKOUT NEVER GIVES A CLEAN `ze-verify` (BLOCKING)

**Several agents work this checkout at once. `make ze-verify` reads the WORKING
TREE, so it reads their half-finished edits too, and a fully green run is
unreachable by construction. Waiting for one is a strategy that cannot terminate.**

It is worse than futile. The full gate saturates every core for half an hour, and
that contention is what makes the functional suites flake -- so a run started to
prove your own work reddens somebody else's at the same time.

**So the full gate is not the pre-commit evidence here. Evidence scoped to YOUR
files is.** Before preparing the commit script:

1. Run the narrow gate owning each surface you changed (the table below).
2. Run the tests of each package you touched, with `make ze-test-pkg`.
3. ATTRIBUTE every red you saw: name the file, and say whether it is yours. `git
   status --porcelain` plus a modification time settles it in seconds. A red in a
   path your diff does not contain is not yours to chase.
4. Prepare the script with `--unverified "<attribution>"`, giving the gates you ran
   and their verdicts, and naming the concurrent session's paths you excluded.

**`--unverified` is the CORRECT path in a shared checkout, not a shortcut.** It
exists for exactly this: a full-tree gate whose red belongs to somebody else's
in-flight work. Its own text names the owner override and a failure you tried and could not reproduce;
concurrent-session interference is the third case, and the reason has to say so.

**A deterministic STRUCTURAL gate is still never waved through** (see "Structural
Gates Are Never Known-Red"). Those read files, not a moving tree, so they are
reproducible and yours to fix when your diff caused them: `ze-lint`,
`ze-rules-lint`, `ze-doc-test`, `ze-rfc-check`, `ze-verify-wiring-docs`,
`ze-tier-check`. Green those, always. It is the TEST stages -- unit, functional,
web -- whose reds a concurrent tree can manufacture.

**Never edit the tree while a verify runs**, yours or anybody's. Regenerating an
index or touching a rule mid-run invalidates every stage that already read it, and
the failures it produces look exactly like real ones. Measured: one such run
reported five failing stages, all five self-inflicted, and none reproduced on the
settled tree.

### ONCE, AT THE END. Never during development (BLOCKING)

**`make ze-verify` is a 24-stage full gate and takes 25 to 30 minutes. Run it ONE
time, when the work is finished and you are about to prepare the commit script.**
Running it to "check in" mid-change is the single most expensive habit available in
this repository, and it buys nothing a scoped check does not.

**Run what the change touches.** Every surface has one owning target, and it costs
seconds to minutes rather than half an hour. Find yours in this table, run it after
each edit, and keep `ze-verify` for the end.

**Go through `make`, or carry `GOCACHE` yourself.** `Makefile` exports
`GOCACHE := $(CURDIR)/cache/go-cache`, and that export reaches make RECIPES only. A
bare `go test` typed into a shell uses the user's own `~/.cache/go-build` instead,
so it rebuilds the world cold, shares nothing with `ze-verify`, and leaves the
project cache no warmer than it found it. `Makefile` also defines the canonical
invocation (`GO_TEST`, `GO_TEST_RACE`): the feature tags, the timeout, `GOMAXPROCS`
and `CGO_ENABLED=1` for race. A bare `go test` drops all of it
(`ai/rules/commands.md`, "Bare `go test` Lies").

`make ze-test-pkg PKG=<pattern>` is the supported way to test ONE package while
you develop it. It carries all of the above. Add `RUN=<regexp>` to narrow, and
`RACE=0` to drop `-race` while iterating -- but a package tested without `-race`
has not been tested the way `ze-verify` tests it, so put it back before the end.

```
make ze-test-pkg PKG=./internal/component/ike/eap
make ze-test-pkg PKG=./internal/component/ike/... RUN=TestEAPTLS
```

| You changed | Run this |
|-------------|----------|
| A `.go` file | `make ze-test-pkg PKG=<that package>`, or the group target covering it (`ze-test-bgp`, `ze-test-core`, `ze-test-plugins`, `ze-test-config`, `ze-test-cli`, `ze-test-rest`). Then `make ze-lint-changed` (`ai/rules/commands.md`) |
| Reactor concurrency (`reactor/session*.go`, `forward_pool*.go`, `peer.go`) | `make ze-race-reactor` (`ai/rules/testing.md`) |
| A `.ci` or `.et` test | its suite target: `make ze-plugin-test`, `ze-parse-test`, `ze-encode-test`, `ze-editor-test`, `ze-web-test`. Draft first in `test/draft/` |
| Linux-only code (`//go:build linux`) | `make ze-qemu-integration-test`, or `make ze-qemu-needs-linux-test` for a `needs-linux` `.ci` (`ai/rules/platform-linux.md`) |
| `rfc/short/*.md`, an `RFC requirement:` tag, `rfc/extraction/*` | `make ze-rfc-check` |
| `docs/**`, `ai/**`, `plan/**` | `make ze-doc-test`, and `make ze-verify-wiring-docs` for the changed-file gates |
| `ai/rules/*.md` | `make ze-rules-condensed` then `make ze-rules-lint`, and commit all three digests with the rule |
| A `*.yang` file or a `ze:command` | `make ze-doc-test`, `make ze-cli-grammar-check` |
| A plugin `register.go`, or anything generated | `make generate`, `make ze-plugin-imports-check` |
| A new package's placement | `make ze-tier-check` |
| Anything, once the commit script has run and it carried Go | `make ze-tracked-build-check` -- the only check that compiles what git holds |
| A `scripts/dev/*.py` tool | its sibling `*_test.py` directly (python needs no build cache), then `make ze-test-pkg PKG=./scripts/dev` |
| Several of the above, and you want breadth | `make ze-verify-changed` |

**When the table has no row for what you touched, derive it.** `mk/*.mk` names every
target and what it runs, `make help` lists them, and `ai/rules/repo-maintenance.md` maps
each gate to the rule it enforces. A surface with no owning target is worth saying so
in the report rather than reaching for the full gate.

**READ THE WHOLE FAILURE SUMMARY BEFORE YOU RE-RUN.** A verify run ends with
`FAIL N verify stage(s) failed` and one line per failing stage, and
`tmp/ze-verify-failures.log` holds the same list. A re-run started from a partial
read costs another half hour and usually reports the same stages. Two specific
traps, both of which have cost a full run:

- **`tail` on the log of a run that is still going.** The stage banner tells you
  where it is (`### Stage 18/22`). Check `scripts/dev/verify-status.sh check` for
  the verdict instead: it says FRESH, or names the failure and its time.
- **Grepping for `--- FAIL` only.** Lint, tier, doc and inventory stages fail with
  their own wording and no `FAIL` token, so a test-shaped grep reads a red lint
  stage as a clean run. Read the summary block, not a pattern you chose.

**A second `ze-verify` cannot overlap the first: it blocks on the repo-wide lock**
and runs the whole thing again afterwards, so starting one while another is live
does not overlap the work, it doubles the wall clock.

**A NARROW FAILURE GETS A NARROW RE-RUN, NEVER A SECOND FULL PASS.** When the
end-of-development run comes back with one or two failing stages and you fix
exactly those, re-run the GATE THAT FAILED and the tests of the package you
touched. Twenty-three green stages that nothing has touched since do not become
more green by being run again.

**The status record is what forces the second pass, so plan the FIRST one to be
the last.** `commit_helper.py create` refuses unless
`scripts/dev/verify-status.sh check` reports FRESH, and only a full `ze-verify`
writes that record. A narrow fix therefore still needs one more full run before a
commit, which is precisely why the full run must come AFTER every gate you can
check cheaply is already green: `make ze-lint`, the touched packages' `go test`,
and the gate owning each surface you changed. A run started before those are clean
is a run you will pay for twice.

**Do not stop to ask which way.** The operator is often not present, and a session
that halts on this question has spent their time to save its own. Clear the cheap
gates, run the full pass once, and commit.

### Step 2: Always

```
[ ] 3. Spec completion gate (if driven by a plan/ spec):
      [ ] Journal row appended to plan/journal/<class>.md, its Spec cell naming the spec stem
      [ ] Spec file staged for deletion (git rm)
      Not done -> STOP.
[ ] 4. Executive Summary Report (rules/planning.md). What was done, what is left.
```

Unless Thomas Owner Override is active, never commit with lint issues and never
commit without test evidence when code changed.

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

See: `ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN" -- `git reset`, `git checkout -- <file>`, `git restore`, `git clean`, `git revert`, `git push --force`, `git push -f`, `git stash drop` and `git stash clear` are banned there, in every session, and are not restated here.

Save: `git diff > backups/work-$(date +%Y%m%d-%H%M%S).patch`,
then write the destructive command(s) to `tmp/delete-SESSION.sh`,
tell the user, STOP.

## Forbidden Raw Output

`git diff --stat` / `git status` dumped raw in output -- summarise.

## Branch Integration

When the user integrates a worktree branch manually, it lands on main
via `git rebase <branch>`, never `git merge`. Linear history.

## Rebase Onto Diverged main: driving the bookkeeping conflicts

A rebase of local commits onto a diverged `origin/main` can re-conflict on
derivable bookkeeping files (`ai/PACKAGE-MAP.md`, `ai/DOCS-TO-CODE.md`).
Regenerate with `make ze-discovery-index` at each rebase stop and continue.

Finish the rebase first, then repair bookkeeping -- never mid-rebase. Afterwards
regenerate the derived indexes with `make ze-discovery-index` and recompute any
derived ratchet the rebase loosened (e.g. `test/.ci-sleep-baseline` = actual
`time.sleep(` count in `test/**/*.ci`).

Gotcha: `git rebase --continue` refuses with a MISLEADING "You must edit all
merge conflicts" whenever there are unstaged tracked changes, not only when
index entries are unmerged (`builtin/rebase.c` ACTION_CONTINUE checks
`has_unstaged_changes()`). Read `git status` for the unstaged tracked files and
stage or discard them; the message names conflicts you do not have.
Per "Branch Changes Are Forbidden" above, the AI never runs `git rebase`
itself; the script only resolves conflicts within a rebase the user started.

## GPG Signing

Never `--no-gpg-sign` / `-c commit.gpgsign=false`.
Never `--no-verify`. Never disable a hook to make a commit pass.

On `gpg failed to sign` / `cannot open /dev/tty`, ask the user to run
`! echo test | gpg --clearsign` to unlock the agent, then re-run the
commit script.

## Codeberg CLI

`tea` for PRs/issues: `tea pr list`, `tea pr create`, `tea issue list`, `tea issue create`.
