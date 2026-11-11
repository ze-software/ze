# Non-Idempotent Append Duplicates The Record

A writer appends a row without testing whether the file already holds it, so a
second run of the same step records the same fact twice.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | - | commit_helper.py, verification-debt shard | `record_debt` (retired `scripts/dev/commit_helper.py`) appended one row per owed gate without checking the shard, so a second `create` wrote it again: on `d0ddef174` two runs left four rows in `plan/verification-debt/aabf4645.md` where two were owed | fixed: obsolete. Commit 2688449b1b deleted the per-commit ledger; `internal/le/commit` derives the debt from git (`debt-status`) and appends no shard row, verified at HEAD b3c348b90c |
| 2026-08-19 | - | verification-debt shard, replace route | Repeating create with replace appended already-owed debt again; [original details](#2026-08-19-replace-repeats-debt). | fixed: 2688449b1b; foreign-shard proof efe2fad70b ([P-5/P-6][tooling]) |
| 2026-08-29 | - | `./le commit create append` and the message file it names (`Create`, `internal/le/commit/prepare.go`) | The message path `tmp/commit-msg-<session>-<tag>.txt` came from the tag alone, so `create replace` then `create append` under one tag had the second message overwrite the first and both commits carried the closing text | fixed: `bc987697e1`; `allocateMessage` (`internal/le/commit/script.go`) names each block's message `commit-msg-<session>-<tag>-<nonce>.txt`, so a second block cannot name the first's file |
| 2026-10-13 | spec-daemon-drain-before-stop | `filter_community` tag, LARGE_COMMUNITY (code 32) | Tagging appends every value without testing presence (`genericCommunityHandler` Adds, `ingressTagCommunities`; `TestTagExistingCommunityNoDeDup` pins code 8), so tagging a large community the route already carries, or two tag definitions sharing one, transmits it twice. RFC 8092 Section 3: "Duplicate BGP Large Community values MUST NOT be transmitted." | fixed: spec-bgp-update-propagation-rfc-defects D2, `writeUpdateBody` drops code-32 repeats; 8, 16 kept |
| 2026-11-06 | - | route refresh, `sendBatchUpdate` (`reactor_api_batch.go`) over `ribOut` | A refresh replays post-policy `ribOut` bytes through `writeUpdate` with the gate on and no replay exemption, so a prepend member applies one extra time per refresh. It does not accumulate, because `handleSent` drops replays. The whole-chain stale run reads the same bytes. | open: the RIB keeps pre-policy attributes and the chain runs once over them. RFC 2918 Section 4 rules out skipping the chain. `TestOrdinaryRefreshDoesNotDoublePrepend` is the red test. |

## Occurrence details

### 2026-08-19 replace repeats debt

Original surface: commit_helper.py, verification-debt shard

Original symptom: Second occurrence of the row above, the same day, by the route that row predicted. `create --replace` rewrites the script and appends the debt rows a second time. `record_debt` (the retired `scripts/dev/commit_helper.py` (current producer: `internal/le/commit/prepare.go`)) opens the shard with mode `a` and tests nothing. Commit `2f2eb832b` owes 2 gates and `plan/verification-debt/f8bfdd86.md` holds 4 rows. The inflated count reaches the session-start banner. It also refuses `--push` until each duplicate is cleared, which re-runs one gate twice

[tooling]: ../post-release/engineering/tooling-one-off-evidence.md
