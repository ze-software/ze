# Ze Project Memory -- Pointer

All project memory lives in `.claude/rules/memory.md` inside the repo.
Do NOT duplicate entries here. Read the repo copy.

Repo memory includes: project knowledge, mistake log (feature not wired, wrong production path,
count-only assertions, wrapper struct pattern, plugin placement anchor bias).

## Project

- [project_cli_dispatch_discovery.md](project_cli_dispatch_discovery.md) - Three CLI gaps blocking debugging: no one-shot command, help shows RPC names not dispatch keys, no dispatch key listing
- [project_no_filtered_routes.md](project_no_filtered_routes.md) - Ze does not track filtered/noexport routes; birdwatcher endpoints return empty
- [project_gokrazy_appliance.md](project_gokrazy_appliance.md) - Ze targets gokrazy appliance (no systemd); must own full process lifecycle for VPP


## User Profile

- [user_trust_and_delegation.md](user_trust_and_delegation.md) - User trusts Claude with hard, long work and delegates the difficult parts. Honor that trust with thoroughness.
- [feedback_autonomous_work.md](feedback_autonomous_work.md) - Work autonomously, do not ask questions or wait for confirmation
- [feedback_parallel_sessions_no_stash.md](feedback_parallel_sessions_no_stash.md) - Multiple Claude sessions run in parallel; never use git stash (corrupts other sessions' work)

## Reference

- [reference_discord_bot.md](reference_discord_bot.md) - Discord bot in ~/Unix/bin/discord.sh: --channel ze-news/ze-test, --text "msg"

## Feedback (workflow)

- [feedback_workflow_cycle.md](feedback_workflow_cycle.md) - Standard cycle: /ze-implement -> work -> /ze-review -> fix -> /ze-commit -> repeat
- [feedback_verify_specs_against_code.md](feedback_verify_specs_against_code.md) - Never trust spec "What Remains"; grep code before reporting progress

## Feedback (testing)

- [feedback_sleep_hides_races.md](feedback_sleep_hides_races.md) - Replacing time.Sleep with proper sync exposes real data races; treat as bug-finding technique
- [feedback_periodic_test_sweep.md](feedback_periodic_test_sweep.md) - Untested code falls into 3 predictable categories: pure functions with only integration coverage, platform code assumed untestable, missing test infra support

## User Preferences (cross-project, not in repo)

- [feedback_no_em_dashes.md](feedback_no_em_dashes.md) - Never use em dashes in English text (AI writing tell)
- [feedback_memory_is_in_repo.md](feedback_memory_is_in_repo.md) - ~/.claude/projects/.../memory/ is the repo's .claude/memory/. Always commit memory changes.
- [feedback_no_deferral.md](feedback_no_deferral.md) - Do not defer hard work. Implement it. Deferring defeats the purpose of delegation.

## Moved to .claude/rules/ (2026-04-05)

The following memories were folded into project rules and deleted from memory:
- feedback_aliased_imports -> rules/go-standards.md (Aliased Imports)
- feedback_python_not_shell -> rules/go-standards.md (Scripts: Python Only)
- feedback_rebase_not_merge -> rules/git-safety.md (Branch Integration)
- feedback_gpg_signing_recovery -> rules/git-safety.md (GPG Signing Recovery)
- feedback_verify_before_deferring -> rules/deferral-tracking.md (Verify Before Deferring)
- feedback_understand_before_coding -> rules/before-writing-code.md (Memory Lifecycle Tracing)
- feedback_no_edit_without_approval -> rules/planning.md (design discussion wait)
- feedback_trust_learned_summaries -> rules/quality.md (Learned Summary Verification)
- feedback_confirm_before_switching -> rules/session-start.md (Session Focus)

Deleted as duplicates of existing rules:
- feedback_no_git_add (rules/git-safety.md)
- feedback_no_git_reset (rules/git-safety.md)
- feedback_multiple_commits (rules/git-safety.md)
- feedback_test_failures_always_report (rules/anti-rationalization.md)
- feedback_never_disable_gpg (CLAUDE.md)
- feedback_consistency_predictability (implicit in all BLOCKING rules)
