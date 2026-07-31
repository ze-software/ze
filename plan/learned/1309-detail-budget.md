# 1309 -- Detail budget

## Context

Thomas reported that reports were over-precise. Line numbers and exhaustive
detail cost tokens and give no benefit, because a reader recovers a detail by
opening the code. The habit had reached the rules themselves. `CONDENSED.md`
had grown to 99k tokens, loaded by every session before any work starts, and
one table row in `hook-mapping.md` was 1,327 tokens.

## Decisions

- **Separate verification from citation, over softening either.** `file:line` was doing two jobs. Verification is an action you take: read the producing function. The citation is written for the reader, so it names the file and the symbol. A line number is correct only when the line IS the fact.
- **Fix the demand at every source, over writing one new rule.** Nine rules minted `file:line` independently, and seven `ze-*` skills repeated it for each claim through a shared preamble. A new rule beside nine unchanged ones would have lost. The per-turn `verify-claim-reminder.sh` mattered most, because it lands after the whole system prompt.
- **Give `rule-format.md` a body budget.** It capped the trigger line and nothing else. That is why every long rule in the corpus is format-legal.
- **Two readings beat a third example.** When a directive is ambiguous, name both readings and say which governs. Examples hide an ambiguity rather than settle it.
- **Advisory budgets, over a gate with a baseline ratchet.** About 16 rules and half the learned corpus are already over budget, so a gate needs a committed baseline. That is a separate decision the owner has not taken.

## Consequences

- The digest costs 671 fewer tokens per session, and that number already carries the new rule. The trims paid for it and for 1.8k more.
- `ai/rules/detail-budget.md` holds the budgets that `fix-dont-record.md` used to carry. That rule keeps the fix-don't-record half and points here for length.
- The largest remaining win is untaken. `hook-mapping.md` is a pull-model lookup table whose own trigger is "looking up which check enforces a rule", and 7,379 digest tokens of it load eagerly in every session. A `Digest: pointer` mode in `rules_condensed.py` would recover them, at the risk of hiding a directive that sits inside a reference rule.

## Gotchas

- **Trimming a narrated rule found a wrong fact.** The `delegation` fixture count was documented as 30 in four places. It is 35. Detail that is re-derivable is also detail nobody re-checks.
- **A rule that narrates its gate holds a stale second copy of that gate.** The 1,327-token row described guard order, exit codes, and line offsets. The script and its 35 fixtures already state all three.
- **The STE gate reads the whole working tree, so it reports a concurrent session's prose.** Four uncommitted `internal/component/ike/**.go` files failed it throughout this work. Read the path before the habit, and check your own files alone with `ste_check.py --check <file>...`.

## Files

- `ai/rules/detail-budget.md` -- new: the standard, the per-artifact budgets, the banned list
- `ai/rules/rule-format.md` -- body budget, one example for one point, no gate narration
- `ai/rules/no-fabrication.md`, `fix-dont-record.md`, and seven further rules -- verification split from citation
- `ai/INSTRUCTIONS.md`, `.claude/hooks/verify-claim-reminder.sh`, `ai/skills/*.md` -- the per-session and per-turn statements, and the shared delegation preamble
- `ai/rules/hook-mapping.md`, `planning.md`, `spec-delegation.md`, `plan/learned/1308-stop-hook-reregistration.md` -- trimmed
