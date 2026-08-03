# 1164 -- deferral-destination-spec-gate

## Context

`ai/rules/deferral-tracking.md` required every open deferral to name a destination, but the
commit gate only rejected a literal placeholder (`""`, `-`, `unassigned`, `tbd`, `none`). A row
saying `none yet (future confederation spec)` therefore passed: it named a home that did not
exist and nobody had committed to. Owner direction was to make the rule absolute (deferred work
ALWAYS lands in a spec, an existing one where possible, else a new `spec-<source>-deferred-<subtask>.md`)
and to enforce it. Turning the enforcement on immediately blocked every commit in every session
on 17 real rows that had accumulated behind the hole, which is the honest measure of how much a
gate that never fired was worth.

## Decisions

- Destination must name a file that EXISTS on disk, over merely being non-placeholder text: a path to a spec nobody created loses the work exactly as "later" does.
- ONE named file must exist, not all of them, over requiring every match to resolve. A Destination cell is a filename plus prose, and good rows cite their retired original destination to explain a re-homing.
- Accept both `plan/spec-x.md` and a bare `spec-x.md` resolved against plan/, over demanding one spelling: the rule is about the work having a home, not path punctuation.
- `deferral_destination_paths` is the single source of truth for reading a Destination, over letting the gate and the test harness each resolve paths: two implementations of the same question drift invisibly.
- When the source spec is ALREADY closed as the deferral spec is written, `<source>` names the subsystem, over naming the dead spec: a filename pointing at an unopenable file reads as a broken reference, not provenance.
- The gate stays one notch wider than the rule (any existing `plan/**.md`, so `plan/known-failures.md` passes for a test that stays red), over encoding the judgement in code that cannot tell a deliberate row from a lazy one.

## Consequences

- A deferral now cannot be recorded without a spec existing first, so "I will write the spec later" is not a reachable state.
- Spec closure `git rm`s the source spec, so `<source>` in a deferral spec name usually points at a file that is gone. That is intended: provenance lives in git history and the deferral spec is the tracker.
- Homing the 17 rows surfaced work that was already done (the birdwatcher counts shipped a day after the row was written) and rows that were wrong when written (the community leaf-list existed a month before the row claimed it did not). A deferral row rots faster than the code it describes.
- Any future widening of what a Destination may name must go through `deferral_destination_paths`, or the harness and the gate will disagree about which file to check.

## Gotchas

- The gate blocked the very commit that introduced it. Enforcement that is honest about existing debt is unlandable until the debt is paid, so budget for the cleanup in the same session, not after.
- A `Destination` cell is prose, not a field. Any regex over it will match filenames the author only MENTIONED. The first version flagged two correctly-homed rows because their annotation named a deleted spec, and the cheapest way to silence it would have been to delete the provenance: a gate that punishes good practice trains people out of it.
- `plan/learned/1127-x.md` does not match a `[\w.-]+\.md` pattern (the nested `/`), so a regex written for flat spec names silently reads a real destination as "no file". Nested plan paths need their own alternative, ordered first so the longest form wins.
- The pre-existing `commit-gate-deferral-assigned-ok` fixture asserted the OLD contract (a destination naming a nonexistent spec passes). A tightening gate makes such a fixture fail; the fix is to make the fixture create the spec, not to loosen the gate back.
- Two sessions implemented this rule concurrently in the same working tree and produced duplicate deferral specs for the same two ddos rows under different `<source>` conventions. With a shared tree and a shared single-file log this is structural, not misconduct (see `ai/rules/git-safety.md`).
- **A row's Status answers "is the work outstanding", not "does it have a home".** Filing work in a spec is not finishing it, so a homed row stays `deferred` and keeps being destination-checked until the work lands. The old "moved to another spec -> `done`" line read as "filing closes the row" and I believed it: I closed 9 rows on filing, which hid them from the only thing checking them, and then wrote a rule saying every row is "born `done`" that would have routed ALL future rows into the unchecked terminal state. The gate would have gone from checking 51 rows to checking 0 while looking healthy.
- The tell was in the tree the whole time: 42 live rows all named real specs and the gate was clean, and the sibling test called them "correctly-homed live rows" (`commit_helper_test.py:292`). A rule that declares 42 existing correct rows to be violations is wrong about the rule, not about the rows. Read what the code and the data already agree on before writing down what they "should" do.
- The gate skips terminal rows entirely, so `done` plus a prose destination sails through. That is tolerable ONLY under the correct semantics, where `done` means the work landed: nobody is routed toward it while work is outstanding, and its Destination is legitimately a commit SHA. The same blind spot under "born done" would have been the main road. A blind spot's severity depends entirely on whether the rule steers people into it.

## Files

- `ai/rules/deferral-tracking.md` - destination-spec requirement, naming convention, closed-source clause, status vocabulary
- `ai/rules/planning.md` - Deferred Work section points at the same contract
- `ai/rules/INDEX.md` - regenerated
- `scripts/dev/commit_helper.py` - `deferral_destination_paths`, `deferral_destination_problem`, `deferral_unassigned_problems`
- `scripts/dev/commit_helper_test.py` - `TestDeferralDestination`, `TestDeferralUnassigned`
- `scripts/dev/hook-fixture-check.py` - commit-gate deferral fixtures
- `plan/deferrals.md` - 13 rows re-homed
- `plan/spec-*-deferred-*.md` - eight new deferral specs
- `plan/learned/1179-fixit-local-asn-config-key.md` - bug found while homing a row
