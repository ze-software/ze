# Deferrals: fixit-learned-1366-has-no-files-section

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** `make ze-doc-test` was RED at HEAD on the learned-staleness ceiling

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc, running `make ze-doc-test` for the `docs/guide/configuration.md` correction that rode with the prefix-set rollback fix | The `found-problem-spec-first` learned summary, committed by `a756f094c`, carried no `## Files` section. `learned_staleness.py` counted that as one finding, which took the corpus to 318 against the committed ceiling of 317, and `make ze-doc-test` failed | Not mine to fix and not mine to attribute further: the file belonged to the session that wrote it, and `ai/rules/never-destroy-work.md` plus this session's file ownership put another agent's just-committed summary outside my reach. Every other `ze-doc-test` stage passed, and no finding named a file this session touched, so the guide correction is proven by the stages that own it | RESOLVED 2026-08-09 by `2cff2050a`, which retired the learned corpus. The summary, the checker `scripts/dev/learned_staleness.py` and the ceiling file `plan/.learned-staleness-baseline` are all gone, so the red has no subject left | done |
