# Finishing Work

**When:** before claiming any work done, complete, or ready to commit, and whenever a defect, a red test, or a missing behavior blocks that claim
**Severity:** blocking
**Related:** planning, testing, interop-and-goal-validation, writing, evidence, rule-precedence

## Directives

**You MUST NOT claim work is done, complete, ready to commit, or ready for review while any in-scope acceptance criterion remains unimplemented, and the claim has no synonyms.** "Deferred", "tracked in a shard", "all core functionality implemented", "the remaining items are minor" and "will be handled in a follow-up" each name work that is not done.
**You MUST have READ the diff, hunk by hunk, before the claim.** A gate covers what somebody thought to check, so a green run over an unread diff is neither done nor green: say what you have, which is that the gates pass and you have not read the change.

**A new exported symbol MUST have a non-test caller, and wiring MUST be the FIRST implementation step rather than a check at the end.** Grep the symbol across `internal/` and `cmd/`: if the only hits are its definition and test files it is dead code, and dead code is a BLOCKER rather than a NOTE. A wiring test that cannot be written means the feature is BLOCKED rather than done.
**`./le doc wiring` is a STRUCTURAL stage of `./le verify worktree`, so its red says the tree is broken and MUST be fixed rather than recorded.** `docs/contributing/spec-workflow.md` says where new code has to be called from, which test each feature type owes, and where its `.ci` test lives.

**One question sorts every defect you meet: does the goal this work exists to achieve still hold if I leave this? When it does not, you MUST fix the defect now**, and you MUST NOT park it, move it to `tmp/`, file it as a deferral, or offer to drop the deliverable. When you are unsure which side you are on, you are on the fix-it side.
**You MUST NOT offer the user a reduction in coverage as a way out of a red.** Dropping an interop or functional test, weakening an assertion, and marking a goal-validation row "N/A" are the failure, never a choice to put on the table.

**A problem you FIND while working on something else gets a JOURNAL ROW, not a spec (owner directive, 2026-08-10).** You MUST append one row to `plan/journal/<class>.md`, close the work in hand, and stop. No spec, no deferral row, no question to Thomas, no report paragraph. A class that collects rows earns its fix in a deliberate pass over the journal.
**Three finds are FIXED on the spot, and they are the only three:** a defect that stops a test or a gate from passing, a test that is wrong about what it asserts, and code related to the problem in hand, edited or not, its tests included. Fix a small one anyway, and still write the row.
**You MUST NOT characterise the find beyond the row's five columns, `| Date | Spec | Surface | Symptom | Fix |`.** Reproducing it, tracing its producer, sizing its blast radius and drafting its options are work nobody commissioned. Grep `plan/journal/` for the symptom first, because many sessions share this checkout, and COMMIT the row: an uncommitted one dies at the next clean or checkout.

**"Pre-existing" describes WHEN a defect started. It says nothing about whose it is, and it MUST NOT be written as a reason to leave one in place.** You met it because you are the first person to exercise that path end to end, and that is exactly the person who fixes it. The moment the work in hand depends on the path, the bug is in scope.

**LOAD IS NEVER AN EXPLANATION. IT IS THE BUG.** A test that passes on a quiet host and fails on a busy one is a BROKEN TEST: load did not break it, load REVEALED that it asserts on elapsed time instead of on state. Naming the host's load is therefore the diagnosis, not an excuse and not a non-deterministic hatch, so you MUST fix the test rather than record it.
**You MUST find what the test waits ON and make it wait for that thing.** Poll the condition, or wait on the readiness signal the daemon emits, and ADD that signal when none exists, because a missing one is a product gap. Raising a timeout only moves the load level at which the test lies. `checkLoadExcuses` (`internal/le/doc/wiring/docwiring.go`) fails a changed `plan/known-failures/` shard carrying "passes in isolation" or any of its synonyms.

**When you catch yourself explaining why a test, a gate, or a completion standard does not apply this time, you MUST answer "no."** The explanation is the tell, and so is the word "just": "let me just rename", "just skip", "just special-case", "just adjust the test". Write the diagnosis instead, and fix the source.
**A diagnosis MUST name the exact function where behavior diverges from intent, as file plus symbol, read rather than guessed.** Without that name there is no diagnosis, and an edit that silences the symptom before the root cause is named is the defect rather than the fix.
**After three failed fixes you MUST STOP, report all three approaches, question the mental model, and ask the user which way to fix it.** A fourth attempt from the same model of the problem is the same attempt.

**VERIFICATION debt MAY be recorded. DEFECT debt MUST NOT be.** Verification debt is a gate that has not yet run over code you believe correct, or one red on another session's uncommitted work: nothing is broken, only a check is owed. A gate that RAN and went red on YOUR code is a defect, and so is behavior an acceptance criterion requires and nothing implements; neither is recordable.
**The override keywords on `./le commit create` are SELF-SERVICE, and you MUST NOT stop to ask Thomas before using one.** `unverified`, `structural-red-ok`, `missing-full-verify-ok`, `stale-index-ok` and `review-override` each take a truthful reason, admit one unrun gate, and write a row in `plan/verification-debt/<session>.md`.
**Enforcement is at the PUSH, where code reaches users: `create push <remote>` refuses while any row is open, and `./le commit debt-clear` re-runs each owed gate once and writes `cleared` only where it exits 0.**

**You MUST finish the work you were asked to do and then report it; asking permission to start it wastes a round trip, and the Stop hook refuses the turn.** Three standing exceptions, where asking IS owed: genuinely ambiguous scope, a destructive action `ai/rules/never-destroy-work.md` gates, and a reduction in scope or a dropped acceptance criterion. The question is always `which way do I fix it`, never `may I skip it`.

**`hookStop` (`internal/le/hookruntime/lifecycle.go`) reads your last message and refuses the stop on any phrase below. These are the words the gate matches, case-insensitively, and you MUST NOT end a turn on one.**

| Scanned | Phrases |
|---------|---------|
| Always | `let me know if you`, `would you like me to`, `feel free to`, `if you'd like me to`, `if you want me to`, `happy to help`, `I can <verb> ... if you`, `I'll stop here`, `I'll pause here`, `that's all for now`, `I'll leave ... to you`, `should I proceed/continue/go ahead`, `do you want me to`, `want me to`, `want me to ... or`, `shall I proceed/continue/go ahead/start/keep`, `before I proceed`, `ready for me to`, `or leave/skip/ignore it`, `or should I`, `or something else` |
| Only while a claimed spec is `in-progress` | `what would you like`, `what do you want to do`, `what's next`, `what next` |

**A blocked Stop is NOT an instruction to do the work you just offered.** Answer one question: who asked for it? The user did, so finish it and do not ask again. You thought of it, so DROP it, and MUST NOT start it, size it, or offer it a second time.
**A phrase inside backticks or a closed fence is QUOTED and does not block, and the list is not exhaustive, so a green Stop is no proof you followed this rule.**
