# Ze Project Memory -- Pointer

All project memory lives in `.claude/rules/memory.md` inside the repo.
Do NOT duplicate entries here. Read the repo copy.

Repo memory includes: project knowledge, mistake log (feature not wired, wrong production path,
count-only assertions, wrapper struct pattern, plugin placement anchor bias).

## User Profile

- [user_trust_and_delegation.md](user_trust_and_delegation.md) - User trusts Claude with hard, long work and delegates the difficult parts. Honor that trust with thoroughness.

## User Preferences (cross-project, not in repo)

- [feedback_no_em_dashes.md](feedback_no_em_dashes.md) - Never use em dashes in English text (AI writing tell)
- [feedback_test_failures_always_report.md](feedback_test_failures_always_report.md) - Always report test failures. Investigate. Ask user how to proceed based on risk.
- [feedback_multiple_commits.md](feedback_multiple_commits.md) - Same system = one commit. Disjoint systems = separate commits.
- [feedback_consistency_predictability.md](feedback_consistency_predictability.md) - Follow the same process every time. Inconsistency forces the user to be vigilant about catching shortcuts.
- [feedback_memory_is_in_repo.md](feedback_memory_is_in_repo.md) - ~/.claude/projects/.../memory/ is the repo's .claude/memory/. Always commit memory changes.
- [feedback_no_deferral.md](feedback_no_deferral.md) - Do not defer hard work. Implement it. Deferring defeats the purpose of delegation.
- [feedback_no_edit_without_approval.md](feedback_no_edit_without_approval.md) - During design discussions, present options and wait. Never edit files until explicitly approved.
- [feedback_trust_learned_summaries.md](feedback_trust_learned_summaries.md) - Verify "deferred/impossible" claims in learned summaries against actual code before reporting
- [feedback_confirm_before_switching.md](feedback_confirm_before_switching.md) - Do not switch line of work without explicit user confirmation
- [feedback_never_disable_gpg.md](feedback_never_disable_gpg.md) - Never bypass GPG signing on commits. Investigate failures, ask user.
