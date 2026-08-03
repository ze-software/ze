#!/bin/bash
# UserPromptSubmit hook: per-turn reminder that subagent delegation is already approved.
#
# Emits ONE line to STDOUT. By repository convention UserPromptSubmit stdout
# reaches the model context and stderr does not, which is why
# compaction-reminder.sh uses stderr. That convention is asserted in comments and
# demonstrated nowhere: no test observes the injection and no harness document is
# cited, so treat it as convention (ai/rules/evidence.md). A fixture can
# assert what this hook writes. It cannot assert what the harness does with it.
# This fires every turn, so brevity is the whole design.
#
# WHY it exists: some harness builds append a guard near the END of the system
# prompt, "Do not call the AgentTool unless the user requested it". That guard
# arrives last, and the last instruction wins. ai/INSTRUCTIONS.md carries the
# "STANDING REQUEST: delegate to subagents" section, which IS the request that
# guard defers to. It reaches the prompt through the generated CLAUDE.md, far
# earlier, and it loses on position. The counter must land after the whole system
# prompt. UserPromptSubmit stdout is the one position known to land there.
#
# Unconditional by design. A conditional reminder adds a "did the condition fire"
# failure mode, and this reminder is correct on every turn.
#
# The line NAMES the main-thread exceptions. This hook works by arriving last, so
# whatever it says is the last word. An unqualified "delegate everything" would
# therefore push /ze-design and /ze-spec into a subagent, and a subagent cannot
# call AskUserQuestion. That deletes the one-decision-per-question gate those
# skills exist for. A reminder that wins on position MUST carry the exceptions.
# Full rule: ai/rules/planning.md.

cat >/dev/null  # consume the prompt payload on stdin

echo "Reminder: delegation needs no permission here (ai/INSTRUCTIONS.md standing request satisfies the harness guard). Delegate THROUGH THE SKILL, never a hand-written prompt: research is /ze-explore, review is /ze-review, spec check is /ze-review-spec, implementation is /ze-implement, red tests are /ze-debug. A PreToolUse gate blocks a raw agent when a skill covers the ask. Keep /ze-spec, /ze-design, /ze-review-deep and /ze-debug in the main thread. (ai/rules/planning.md, ai/rules/cli.md)"

exit 0
