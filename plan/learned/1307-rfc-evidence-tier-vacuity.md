# 1307 -- A suite can be counted, tiered, and never run

## Context

`ai/rules/testing.md` grades RFC evidence on two axes. KIND says which layer a test
exercises, and TIER says whether anything executes it. A `.ci` test earns
`functional/verify` only when its suite is one `make ze-functional-test` actually runs.

The tier is derived from the `all_suites="..."` string in `mk/test-functional.mk`
(`functional_suites`, `scripts/dev/rfc_requirements.py:668-693`). That function's own
docstring says it fails closed, because a gate that answers "everything runs" without
evidence is the zero that looks like a number.

The `ipsec` suite was added to `all_suites` so `test/ipsec/*.ci` would count as merge-gate
evidence. It was never added to the recipe's `run_suite` lines. The suite was counted in
the progress denominator. It was honored by `ZE_SKIP_SUITES`. Any RFC tag beneath it would
have earned a `functional/verify` tier. It executed nothing.

**No requirement was actually mis-credited, and the reason is luck rather than design.**
No `.ci` under `test/ipsec/` carries an RFC tag, so the tier the deriver was ready to grant
had no claimant. An independent review established that, after the first report of this
defect overstated its effect. The fix is prophylactic, and the trap is real.

Two independent subagents found it on the same day, from opposite directions. Both had
been told the suite ran, because the supervising session believed the `all_suites` edit
was sufficient.

## Decisions

- `all_suites` declares the set. `run_suite` performs it. A suite needs BOTH. An edit that
  adds one without the other is the defect this record names.
- The tier derivation keeps reading `all_suites` rather than the `run_suite` lines. The
  declaration is the contract. A recipe that declares a suite it does not run is the thing
  to fix, over a deriver taught to tolerate it.
- A `ze-ipsec-test` target was added beside the recipe line, so the suite can run alone
  the way every other suite can.
- The evidence claim in the owning spec was corrected in place rather than footnoted. A
  claim that a suite runs is either true, or it is removed.

## Consequences

- Adding a suite to `all_suites` alone now reads as an incomplete change. Whoever enrols a
  suite adds the `run_suite` line in the same edit.
- No row was withdrawn, because no row rested on this suite. Whoever tags a `test/ipsec`
  `.ci` in the future gets a tier the suite now earns.
- The shape generalises past this suite. A declaration consumed by a gate, and a recipe
  consumed by a runner, CAN disagree in silence whenever they are two separate lists.
  `ai/rules/derive-not-hardcode.md` is the standing answer. It is not applied here, so
  this record is the compensating control.

## Gotchas

- **A green functional run is not proof a suite ran.** `make ze-functional-test` printed a
  denominator that included `ipsec`, and it passed. The suite contributed nothing to that
  pass. Read the recipe, never the summary line.
- **Two agents reporting the same surprising fact is signal, not redundancy.** The
  supervising session had told both of them the opposite. Their agreement overturned it.
- **The RFC gate cannot see this class of defect.** It reads the declaration and believes
  it. That is deliberate, and it means the declaration carries more weight than its
  one-line appearance suggests.

## Files

- `mk/test-functional.mk` -- the missing `run_suite ipsec` line, and a `ze-ipsec-test` target
- `scripts/dev/rfc_requirements.py` -- `functional_suites`, the deriver that reads `all_suites`
- `ai/rules/testing.md` -- the KIND and TIER table this record is about
- `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` -- the spec whose evidence claim was corrected
