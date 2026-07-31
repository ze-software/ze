# Ze Project Memory -- Pointer

All project memory lives in `.claude/rules/memory.md` inside the repo.
Do NOT duplicate entries here. Read the repo copy.

Repo memory includes: project knowledge, mistake log (feature not wired, wrong production path,
count-only assertions, wrapper struct pattern, plugin placement anchor bias).

## User Profile

- [user_trust_and_delegation.md](user_trust_and_delegation.md) - User trusts Claude with hard, long work and delegates the difficult parts. Honor that trust with thoroughness.
- [user_profile_expertise.md](user_profile_expertise.md) - 20+yr director/engineer, real-time high-perf (asm/C, Python), ExaBGP author. Requests are expert judgment, not "instinct": no praise, no second-guessing.

## Reference

- [reference_discord_bot.md](reference_discord_bot.md) - Discord bot in ~/Unix/bin/discord.sh (--channel ze-news/ze-test, --text "msg"; token baked in); Zeledon weekly-update tooling in scripts/zeledon/ (STYLE.md, post_weekly.py, weekly/)
- [feedback_discord_voice.md](feedback_discord_voice.md) - Discord posts as Zeledon; use third person for Thomas, not "I"
- [reference_python_uv.md](reference_python_uv.md) - Install Python deps via `uv run --with`. No scapy dep remains; stress path is the in-memory Go injector (`ze-test peer --mode inject`), `test/stress/` holds the Python harness

## Feedback (testing)

- [feedback_sleep_hides_races.md](feedback_sleep_hides_races.md) - Replacing time.Sleep with proper sync exposes real data races; treat as bug-finding technique
- [feedback_periodic_test_sweep.md](feedback_periodic_test_sweep.md) - Untested code falls into 3 predictable categories: pure functions with only integration coverage, platform code assumed untestable, missing test infra support

## Moved to ai/rules/ or .claude/rules/

- feedback_autonomous_work -> ai/rules/no-asking.md. Enforced at Stop by block-premature-stop.sh, which blocks a stop on permission-seeking phrases (exit 2). It sat unregistered from 2026-06-29 (`41e5fa44f`) to 2026-07-31
- feedback_memory_is_in_repo -> derivable from project structure
- feedback_no_em_dashes -> ~/.claude/CLAUDE.md global rule
- feedback_no_taskoutput_polling -> ai/rules/git-safety.md (verify section)
- feedback_scope_verify_to_changed -> ai/rules/git-safety.md (Known-Red Full Verify: Scope to Changed)
- feedback_parallel_sessions_no_stash -> CLAUDE.md + ai/rules/git-safety.md
- feedback_read_before_overwrite -> ai/rules/never-destroy-work.md
- feedback_workflow_cycle -> /ze-implement and /ze-review skill definitions
- feedback_never_strip_context_param -> ai/rules/go-standards.md (Context)
- feedback_no_cross_boundary_pointers -> ai/rules/plugin-design.md (Cross-Boundary Value Types)
- feedback_verify_specs_against_code -> ai/rules/planning.md (Verify Specs Against Code)
- feedback_aliased_imports -> rules/go-standards.md (Aliased Imports)
- feedback_python_not_shell -> rules/go-standards.md (Scripts: Python Only)
- feedback_rebase_not_merge -> rules/git-safety.md (Branch Integration)
- feedback_gpg_signing_recovery -> rules/git-safety.md (GPG Signing Recovery)
- feedback_verify_before_deferring -> rules/deferral-tracking.md (Verify Before Deferring)
- feedback_understand_before_coding -> rules/before-writing-code.md (Memory Lifecycle Tracing)
- feedback_no_edit_without_approval -> rules/planning.md (design discussion wait)
- feedback_trust_learned_summaries -> rules/quality.md (Learned Summary Verification)
- feedback_confirm_before_switching -> rules/session-start.md (Session Focus)
- feedback_no_deferral -> /ze-implement skill (core rule + design-doc "Deferred" carve-out)
- feedback_no_git_add -> rules/git-safety.md
- feedback_no_git_reset -> rules/git-safety.md
- feedback_multiple_commits -> rules/git-safety.md
- feedback_test_failures_always_report -> rules/anti-rationalization.md
- feedback_never_disable_gpg -> CLAUDE.md
- feedback_consistency_predictability -> implicit in all BLOCKING rules
- project_cli_dispatch_discovery -> ai/rules/project-knowledge.md (CLI dispatch discoverability gaps)
- project_no_filtered_routes -> ai/rules/project-knowledge.md (No filtered/noexport route tracking)
- project_gokrazy_appliance -> ai/rules/project-knowledge.md (Gokrazy appliance owns process lifecycle)
- project_stress_injector -> ai/rules/project-knowledge.md (Stress injector is in-memory Go)
