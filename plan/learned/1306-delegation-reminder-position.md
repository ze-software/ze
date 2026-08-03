# 1306 -- The Delegation Rule Lost on Position, Not on Argument

## Context

The main thread kept doing spec work inline. It did this while
`ai/INSTRUCTIONS.md` carried a "STANDING REQUEST: delegate to subagents" section
that names the obstacle and answers it. Thomas asked why, twice.

The cause was position. The harness appends a guard to the END of the system
prompt: "Do not call the AgentTool unless the user requested it". The standing
request sits at `ai/INSTRUCTIONS.md`, far earlier in the same prompt. The
later instruction reads as the operative one. The rule was not weighed and
rejected. It was not seen at the moment of the decision.

## Decisions

- **Counter the guard at `UserPromptSubmit` stdout, over another restatement in `ai/INSTRUCTIONS.md`.** That stdout is the only harness position that lands after the whole system prompt. `verify-claim-reminder.sh` already relies on the same mechanism for `ai/rules/evidence.md`. More argument in an earlier position cannot beat a later position.
- **Both premises are asserted, not demonstrated.** The first is reachability: "UserPromptSubmit stdout reaches the model, and stderr does not". It rests on three author comments, `.claude/hooks/verify-claim-reminder.sh`, `.claude/hooks/compaction-reminder.sh`, and `plan/learned/1010-verify-producer-before-claiming.md`. The second is position: "UserPromptSubmit stdout is the only harness position that lands after the whole system prompt". Nothing in this repository establishes either one. No test observes the injection, and no harness document is cited. A fixture can assert what a hook writes. It cannot assert what the harness does with it, or where the harness puts it. Treat both as strong convention, never as proven (`ai/rules/evidence.md`).
- **Reframe the standing request as SATISFIED, not OVERRIDING.** The guard permits the Agent tool once the user has requested it. This repository is that request. The old text said "when the two appear to conflict, this one wins", which invites an adjudication, and the adjudication kept going the wrong way. A precondition that is already met leaves nothing to decide.
- **Unconditional over conditional.** A reminder gated on a claimed spec adds a "did the condition fire" failure mode. The reminder is correct on every turn, so it fires on every turn.
- **No blocking gate.** A `PreToolUse` block on source edits would read `tmp/session/.agent-spawned-<sid>`. That marker is a one-time flag, so one throwaway spawn opens the gate for the rest of the session. It measures "did you ever spawn an agent", never "did you delegate this work". A gate that reads as enforcement and is not one is worse than a nudge.
- **Leave `block-premature-stop.sh` unregistered** (owner decision, 2026-07-30), and correct every document that called it live. Ten surfaces did. Eight are corrected here: `ai/rules/planning.md`, `ai/rules/planning.md`, `.claude/hooks/README.md`, `.claude/memory/MEMORY.md`, `.claude/hooks/mark-agent-spawned.sh`, `scripts/dev/spec-closure-check.py`, `scripts/dev/spec_closure_check_test.go`, and `plan/learned/1289-delegation-by-default.md`. Two are held back because a concurrent session holds hunks in them: `ai/rules/repo-maintenance.md` and `ai/INDEX.md`.
- **CORRECTION (2026-07-31).** Thomas REVERSED the 2026-07-30 decision above and
  re-registered `block-premature-stop.sh` on `Stop`, first in the array. All four
  gates run: two block with exit 2 (stop-phrase scan, spec-closure), and two warn
  with exit 1 (spec in-progress, delegation nudge). The last three need a CLAIMED
  spec, because each sits behind the `tmp/session/.session-<SID>` marker
  (`.claude/hooks/block-premature-stop.sh`). Order is load-bearing, and
  the release that deleted the marker on every `Stop` moved to `SessionEnd`, so
  the claim survives past turn one (`session-end-scratch.sh`).
  This correction voids every present-tense claim below under "Consequences" and
  "Gotchas", and the regenerated `CONDENSED.md` no longer disagrees with itself.
  Two new fixtures pin the registration and the order:
  `delegation-stop-hook-registered` and
  `delegation-stop-hook-runs-before-marker-release`.
- **A guard that reads prose blocks the prose that documents it.** On its first
  live turn the stop-phrase scan refused a report that quoted a banned phrase in
  backticks as an example. The phrase was NAMED, not used, and a raw `grep -iE`
  cannot tell the two apart. The fix strips fenced blocks and inline backtick
  spans into a `SCAN` copy before the loop runs
  (`.claude/hooks/block-premature-stop.sh`). It falls back to the raw text
  when the filter fails, so a broken filter scans more rather than less. Backticks
  only, deliberately: a filter over every double-quoted span would hide real
  permission-seeking that quotes something. Four fixtures pin it, and
  `stop-phrase-fence-is-not-a-bypass` proves the exemption is not an evasion
  route.
- **A stale-claim census is itself a claim, and it was wrong four times.** The first sweep found four surfaces and wrote "the three documents". Review round three raised it to eight. Round four found the count sentence listed eight paths under a claim of seven, and it found a ninth surface. Checking that ninth showed the file was contended, making ten with two held back. Count by grepping the repository, then count the list you wrote, and treat both numbers as suspect until a reviewer reproduces them.
- **Correct the rule's own table, not only its Enforcement prose.** `ai/rules/planning.md` listed `/ze-spec`, `/ze-design`, `/ze-review-deep` and `/ze-debug` under a column headed "Delegate to a subagent running", while its Enforcement bullet said those four stay in the main thread. The table now carries a `Runs in` column that names each exception. A reader opens the table first, so a correct footnote under a wrong table is still a wrong rule.

## Consequences

- Three hooks now run on every `UserPromptSubmit`: compaction, verify-claim, delegation.
- Delegation stays a nudge. Nothing blocks inline work, and nothing judges whether a given piece of work belonged in a subagent.
- `block-premature-stop.sh` stays inert. The spec-closure gate that `ai/rules/planning.md` documents lives inside that same script, so it has never fired either. Anyone who wants closure enforcement must register a hook. Citing that script is not enough.
- The `## Enforcement` section of `ai/rules/planning.md` is now the accurate inventory. Read it before you claim a delegation gate exists.

## Gotchas

- **A hook on disk with passing fixtures still fires on nothing.** `block-premature-stop.sh` exists, and `hook-fixture-check.py --only delegation` passes against it. Every signal a reader checks says "live" except the `Stop` array in `.claude/settings.json`. Two rules described it as a working gate for a month. Check registration, not existence.
- **`hook-parity-check.py` covers only the three dispatchers.** A new `UserPromptSubmit` hook needs no `--bless`, and it cannot move the golden table.
- **An `ai/rules/*.md` edit obliges a regenerated `CONDENSED.md` in the same commit.** With a concurrent session editing other rules, generate the digest from HEAD plus your own files (`ai/rules/rule-format.md`). A plain `make ze-rules-condensed` publishes their unlanded text.
- **A shared file cannot be committed by two sessions at once.** Two files here carry another session's hunks, so they stay out of this commit. See `F-ste-3` in `plan/learned/HOOK-FRICTION.md`.
- **`ai/rules/CONDENSED.md` disagrees with itself until `repo-maintenance.md` lands.** The digest is generated from HEAD plus this commit's own rule files. Its hook-mapping section therefore still calls the Stop hook a live BLOCKING gate, while its spec-delegation and planning sections call it inert. `CLAUDE.md` imports the digest eagerly, so every session holds both statements. Read the `Stop` array in `.claude/settings.json`, which settles it. This is the cost of the shared-file deadlock, and it is disclosed rather than hidden.
- **A reminder that wins on position must carry the exceptions.** The first draft of the hook line said "run spec phases in parallel Agent calls" with no qualifier. `/ze-spec` and `/ze-design` must stay in the main thread, because a subagent cannot call `AskUserQuestion`. An unqualified last word would have deleted the one-decision-per-question gate those skills exist for. An independent reviewer caught this. The mechanism that makes the hook effective is the same mechanism that makes an imprecise line dangerous.

## Files

- `.claude/hooks/delegation-reminder.sh` (new), `.claude/settings.json`
- `ai/INSTRUCTIONS.md`, `ai/rules/planning.md`, `ai/rules/planning.md`, CONDENSED.md (deleted 2026-08-03)
- `.claude/hooks/README.md`, `.claude/memory/MEMORY.md` (both called the Stop hook live)
- `plan/learned/HOOK-FRICTION.md` (`F-ste-3`)
- Held back, shared with a live session: `ai/rules/repo-maintenance.md`, `scripts/dev/hook-fixture-check.py`
