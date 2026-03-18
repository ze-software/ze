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

## Project Decisions (pending)

- [project_rib_internal_vs_external.md](project_rib_internal_vs_external.md) - Should ExaBGP migration produce internal rib instead of external? User will decide.

## References (cross-project, not in repo)

- [reference_codeberg_tea.md](reference_codeberg_tea.md) - tea CLI available for Codeberg repo (PRs, issues, comments, API)
