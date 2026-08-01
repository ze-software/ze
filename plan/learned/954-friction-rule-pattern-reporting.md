# 954 -- friction reporting: treat recurring development problems as rule gaps

## Context

Thomas asked for a rule that when an agent finds a pattern of problems during
development, it must report the pattern so the team can decide whether better
rules are required. Ze already had `ai/rules/friction-reporting.md`, but it
focused on confusion, surprise, stale docs, and tooling friction. It did not
explicitly require naming a recurring problem pattern or making a rule/no-rule
decision.

## Decisions

- **Update the existing rule instead of creating a sibling.** The behavior is
  the same reporting surface, so a second "problem pattern" rule would split
  the trigger and make future agents check two places.
- **Require a rule decision in the report.** The format now includes
  `Rule decision: [new rule needed / update existing rule / no rule, because...]`
  so the agent does not merely vent friction. It must say whether a rule change
  would prevent recurrence.
- **Expose the trigger in startup guidance.** `ai/INSTRUCTIONS.md` now has a
  "Find recurring development friction or problem patterns" row pointing to the
  rule, and `make ze-ai-instructions` propagates it to generated agent files.
- **Keep one-off unfamiliarity out of the report stream.** The rule still says
  to read relevant docs first and not report non-recurring one-offs, so the new
  trigger stays focused on reusable improvement.

## Consequences

- Future sessions should report repeated rejected edits, missed wiring classes,
  stale conventions, ambiguous rules, and other recurring development failure
  modes as soon as the pattern is identifiable.
- The report should include the proposed artifact to change: a narrow
  `ai/rules/*.md` update, `ai/INDEX.md`, `plan/learned`, docs, or hook logic.
- Regeneration checks for this class of change are `make ze-rules-index-check`
  and `make ze-ai-check`; full `ze-verify` is not required for markdown-only
  agent-rule updates.

## Files

None recorded.
