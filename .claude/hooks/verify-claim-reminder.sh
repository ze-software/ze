#!/bin/bash
# UserPromptSubmit hook: per-turn reminder of the "verify before you claim" contract.
#
# Emits ONE line to STDOUT. UserPromptSubmit stdout is injected into the model
# context (unlike compaction-reminder.sh, which uses stderr precisely so it does
# NOT reach the model). This fires every turn, so brevity is the whole design:
# a single line that lands in fresh context, where a banner read once at session
# start does not. Full rule: ai/rules/no-fabrication.md.

cat >/dev/null  # consume the prompt payload on stdin

echo "Reminder: verify a claim about code by reading the function that PRODUCES it, not the caller. Unread means unverified. Cite file + symbol, not a line number. Report the conclusion, not the search. (no-fabrication.md, detail-budget.md)"

exit 0
