# 1326 -- A deferral shard outlives the spec that opened it

## Context

`ai/rules/deferral-tracking.md` said two things that cannot both be true. "A spec's
shard is deleted at the spec's closure" (Shard key) and "A homed row stays live"
(Status Vocabulary). A row homed at a DIFFERENT spec is live, and its shard belongs
to a spec that just closed. The rule gave no answer for that state, and the state is
not rare: `scripts/dev/deferral_orphans.py` counts 39 shards in it on 2026-08-03,
holding 68 live rows between them. `ai/rules/planning.md` commit B carried the same unconditional instruction.

The contradiction was invisible from either rule file. It only appears when you count
what is on disk.

## Decisions

- **The live row wins, not the deletion.** A shard still holding a live row survives
  its source spec and keeps its source-keyed name. Deletion-at-closure governs the
  all-terminal case only. Chosen over migrating orphaned rows into their destination
  spec's shard: the shard key is the row's SOURCE, and re-keying on destination would
  move rows every time a home changed, which is the cross-commit hazard the
  per-source layout exists to remove.
- **Both readings are written down, and the governing one is named**
  (`ai/rules/rule-format.md`). Deleting the losing sentence would have left the next
  reader to rediscover why the obvious tidy-up is wrong.
- **An explicit "do not sweep" directive was added.** Without it the 39 orphaned
  shards read as mess, and the tidying agent is the one who deletes 68 live records.

## Consequences

- Commit B needs the Status column read before `--remove plan/deferrals/<stem>.md` is
  added, and `deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`) BLOCKS
  the removal when a row is live. That gate is not optional politeness: every other
  signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a
  live-bearing shard LOWERS their counts. The forbidden action silences every observer
  of the rows it destroys, which is why prose alone could not hold it
  (`ai/rules/fail-closed-guards.md`).
- **Changing a rule is not changing behaviour: four surfaces EXECUTE this one and all
  four still taught the superseded version.** `ai/skills/ze-close.md` (the skill an
  agent actually follows at closure), `plan/TEMPLATE-CLOSURE.md` (inlined into every
  closing spec), `ai/rules/critical-review.md` (a directive resting on the false
  premise), and `ai/skills/ze-progress.md`. An independent review found them; the rule
  edit alone would have shipped a rule nothing obeyed (`ai/rules/discovery-updates.md`).
- `ze-progress` stage 2 was filtering on `Status == open` while 127 live rows carry
  `deferred`, so it under-fired. Fixing that alone would have made it never terminate:
  homing a row is the correct resolution and leaves the row LIVE, so status alone
  re-enters stage 2 forever. The condition needs both halves, live AND unhomed.
- 14 orphaned shards are all-terminal residue. The rule names an actor for them (the
  closer of the last spec that homed one of their rows), because "safe to delete and
  nobody's job" is how 14 accumulated. **Naming the actor in the rule was not enough:**
  round 2 found the duty reached no executing surface, because `ze-close`, commit B and
  the closure template all scope `--remove` to the closing spec's OWN stem. All three
  now say to remove a foreign shard this closure emptied.

## Gotchas

- **A rule contradiction does not show up in either file.** Both sentences read as
  correct in place. The measurement is what surfaces it, so a rule that describes a
  lifecycle is worth checking against the tree's actual state, not only against
  itself.
- **A prose destination survives because the check is advisory.** "future usability
  spec" sat in a Destination cell from 2026-07-17. It was reported on every commit for
  17 days and cost nothing to ignore.
- **I got the headline number wrong THREE times, in a BLOCKING rule.** 62 (summed
  from a printed list by eye), then 71 after round 1 recounted, then 68 after round 3
  recounted again. The 71 was wrong for a reason worth keeping: one shard's name
  carried a doubled `spec-` prefix, so pairing it with `plan/spec-<stem>.md` found
  nothing and it read as orphaned while its spec was alive and in progress. Each wrong
  number survived a careful reading and died the moment somebody re-derived it. Numbers
  quoted in a rule are claims, so they now come from `scripts/dev/deferral_orphans.py`
  and the rule cites it. A count nobody can reproduce is the thing the next reader
  re-derives, and then distrusts the whole ruling.
- **The tests passed a mutation I had not thought to try.** Round 2 replaced the
  `git show HEAD:` read with a working-tree read and all five cases stayed green: the
  fixture leaves both trees identical, so nothing distinguished them. That mutation
  reinstates the exact bypass the HEAD read exists to prevent -- `rm` the shard, then
  run `create --remove`, and the gate sees nothing. Writing the fixture so the two
  trees AGREE is what made the tests blind; the case that catches it deletes the
  working copy first.
- **A Python mutation test can silently not apply, and it reports as a PASS.** A
  mutation that keeps the file's byte SIZE identical (`live_bearing, residue,
  misnamed = found` -> `residue, live_bearing, misnamed = found`, both 39 chars) and
  lands in the same wall-clock SECOND as the `__pycache__` write leaves the stale
  `.pyc` valid: CPython keys its check on (mtime seconds, size). The interpreter runs
  the ORIGINAL bytecode while `inspect.getsource` shows the mutated line, so every
  check agrees the mutation is present and the suite is green. Read as "the test does
  not discriminate", which is the opposite of the truth. Run every Python mutation
  under `PYTHONDONTWRITEBYTECODE=1` and delete `__pycache__` between arms. Size-changing
  mutations are unaffected, which is why the earlier arms in this same battery were
  valid and only the one same-length arm lied.
- **A fix inside a file an earlier round already fixed is not safe.** Round 1 corrected
  `ze-progress` step 5; round 2 found the same superseded filter alive at four other
  lines of the same file, including the stage table read first, and round 3 found the
  same shape again in `planning.md`, 40 lines below the paragraph round 1 corrected. Grep the whole file for
  the old predicate, do not edit the occurrence you were pointed at.
- **The source spec's closure is what makes a prose destination unrecoverable.** While
  the spec lives, someone could still create the named spec. Once it closes, nothing
  on disk points at the work at all.

## Files

- `ai/rules/deferral-tracking.md` -- both readings, the governing one, the do-not-sweep directive, the named actor for residue
- `ai/rules/planning.md` -- commit B reads the Status column; the "blocks every future commit" claim corrected to advisory
- `ai/rules/critical-review.md` -- directive kept, its false rationale replaced
- `ai/skills/ze-close.md` -- conditional `--remove`, and the gate that enforces it
- `ai/skills/ze-progress.md` -- stage 2 fires on live AND unhomed, not on `open` alone
- `plan/TEMPLATE-CLOSURE.md` -- same correction, inlined into every closing spec
- `scripts/dev/commit_helper.py` -- `deferral_shard_removal_problems`, wired into `commit_gate_problems`
- `scripts/dev/commit_helper_test.py` -- `TestDeferralShardRemoval`, 13 cases, driven through the assembly point rather than the gate alone
- `scripts/dev/deferral_orphans.py` -- the producing script for every count this rule quotes
- `plan/deferrals/fixit-rs-community-strip-arity.md` -- renamed from a doubled `spec-` prefix
- `ai/rules/CONDENSED.md` -- regenerated
