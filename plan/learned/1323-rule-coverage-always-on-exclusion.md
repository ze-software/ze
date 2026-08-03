# 1323 -- Rule-coverage: always-on rules leave the measured population

## Context

The `rule-coverage` Stop hook reported blocking rules whose trigger matched a
session's files and which the session never read. Three of the names it printed
were always-on rules. `ai/rules/CORE.md` carries them and `CLAUDE.md` imports
that file. No session ever Reads `ai/rules/<name>.md`, so no session can ever
clear them. Over the 75 sessions recorded in `tmp/rule-coverage/report.ndjson`
those three appeared in 87% of reports, beside five genuine misses. A report
whose lines cannot be acted on teaches the reader to skip the lines that can.

## Decisions

- Excluded the CORE.md set from the blocking population, rather than counting an import as a read. The detector asks whether a model that SEES a matching trigger opens the rule. An always-on rule reaches the session with no trigger and no read to perform. It is outside that population, not a member scoring zero.
- Derived membership by parsing CORE.md at run time, over importing `rules_condensed.core_members`. Reading the artifact measures what the session actually received.
- Keyed on the standalone backticked path line `rule_block` emits, over the `<!-- always-on: -->` marker `build_core` writes. The marker is the tighter contract and is the better key if this drifts again.
- Kept over-reporting as the failure direction, and made every path back to it announce itself on stderr. Silent over-reporting is indistinguishable from the noise the change removes.

## Consequences

- `blocking-total` now counts only the routable set, and the summary line states the exclusion unconditionally so the reduced total is never unexplained.
- The accumulated NDJSON is not comparable across this change. Its earlier lines still carry always-on names inside `missed`. A rate over the whole file therefore shows a definition change, not a behaviour change.
- Whether a session opened a rule that is IN the core can no longer be observed. That signal left with the population.

## Gotchas

- **The `^`/`$` anchors on the path regex are load-bearing.** CORE.md cites other rules inline throughout its directive text, so dropping the anchors nearly doubles the muted set and swallows rules every session must read. The first fixture had no inline citation, so the deletion stayed green. It carries one now.
- **A test named "says so" that never captures stderr proves half a fail-closed guard.** Two of them asserted only the behaviour, leaving a deleted `print()` green.
- **A sandbox fixture is the test's own idea of a shape, not the generator's.** Nothing bound the reader to `rules_condensed.rule_block` until a test called it directly.
- **Never pin a count from this corpus in prose.** The rule total moved 97 to 98, and the core moved 12 to 11 when a `plan/` task description began surfacing `evidence.md`, both inside one session. Two rounds of review found stale counts, including one added by the commit that removed the others.

## Files

- `scripts/dev/rule_coverage.py` -- `always_on_rules`, `CORE_RULE_LINE`, `load_rules`, `analyse`, `format_text`
- `scripts/dev/rule_coverage_test.py` -- 8 tests added, 18 total
