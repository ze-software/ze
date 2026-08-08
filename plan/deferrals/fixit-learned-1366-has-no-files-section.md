# Deferrals: fixit-learned-1366-has-no-files-section

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** `make ze-doc-test` is RED at HEAD on the learned-staleness ceiling

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc, running `make ze-doc-test` for the `docs/guide/configuration.md` correction that rode with the prefix-set rollback fix | `plan/learned/1366-found-problem-spec-first.md` carries no `## Files` section. `learned_staleness.py` counts that as one finding, which takes the corpus to 318 against the committed ceiling of 317 in `plan/.learned-staleness-baseline`, and `make ze-doc-test` fails. It is in HEAD, not in a working tree: the summary was committed by `a756f094c` and `git cat-file -p a756f094c:plan/learned/1366-found-problem-spec-first.md` shows no `## Files` heading. Reproduction: `python3 scripts/dev/learned_staleness.py`, whose last finding names the file. The other 317 findings are the grandfathered rot the baseline exists for, and none of them is new | Not mine to fix and not mine to attribute further: the file belongs to the session that wrote it, and `ai/rules/never-destroy-work.md` plus this session's file ownership put another agent's just-committed summary outside my reach. The fix is one heading plus its path list, or `None recorded.` when there is nothing to list, which is what the checker's own message asks for. Every other `ze-doc-test` stage passed, and no finding names a file this session touched, so the guide correction is proven by the stages that own it | needs a destination spec, and it is a live red on main rather than a latent hazard | deferred |
