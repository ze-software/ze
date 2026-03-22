# Ze Project Memory -- Pointer

All project memory lives in `.claude/rules/memory.md` inside the repo.
Do NOT duplicate entries here. Read the repo copy.

Repo memory includes: project knowledge, mistake log (feature not wired, wrong production path,
count-only assertions, wrapper struct pattern, plugin placement anchor bias).

## User Profile

- [user_trust_and_delegation.md](user_trust_and_delegation.md) - User trusts Claude with hard, long work and delegates the difficult parts. Honor that trust with thoroughness.

## User Preferences (cross-project, not in repo)

- [feedback_no_em_dashes.md](feedback_no_em_dashes.md) - Never use em dashes in English text (AI writing tell)
- [feedback_never_claim_done_unwired.md](feedback_never_claim_done_unwired.md) - Completion claims require wiring proof: name the entry point, name the .ci test. Hooks enforce at commit time.
- [feedback_test_failures_always_report.md](feedback_test_failures_always_report.md) - Always report test failures visibly. Pre-existing does not block commits but must be tracked.
- [feedback_no_git_stash.md](feedback_no_git_stash.md) - Never use git stash in any form; it is disallowed
- [feedback_multiple_commits.md](feedback_multiple_commits.md) - Break work into multiple focused commits, not one big bundle
- [feedback_durability_over_velocity.md](feedback_durability_over_velocity.md) - Optimize for "never revisit this code", not "get to commit fast". Thoroughness over speed.
- [feedback_unanswered_questions_block.md](feedback_unanswered_questions_block.md) - Re-state unanswered questions before proceeding. Never silently assume answers.
- [feedback_consistency_predictability.md](feedback_consistency_predictability.md) - Follow the same process every time. Inconsistency forces the user to be vigilant about catching shortcuts.
- [feedback_memory_is_in_repo.md](feedback_memory_is_in_repo.md) - ~/.claude/projects/.../memory/ is the repo's .claude/memory/. Always commit memory changes.

## References (cross-project, not in repo)

- [reference_codeberg_tea.md](reference_codeberg_tea.md) - tea CLI available for Codeberg repo (PRs, issues, comments, API)
