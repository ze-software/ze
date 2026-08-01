# 759: Config Archive Commit-Revisions Pruning

## Context
Config archives accumulated files indefinitely for `file://` locations. Operators had to manually clean up old archive files or set up external cron jobs.

## Decision
Added `system { commit-revisions N }` YANG leaf that limits the number of config archive files kept per `file://` location. After each archive write, files beyond the limit are pruned oldest-first.

Key choices:
- **Stable prefix matching**: `ArchivePrefix` computes the non-time-varying portion of the filename by generating two filenames at different times and returning their common prefix. Only files matching this prefix are pruned, preventing deletion of unrelated `.conf` files in the same directory.
- **Oldest-first by mtime**: files sorted by modification time, oldest removed first. mtime chosen over parsing timestamps from filenames because the filename format is user-configurable.
- **Per-location pruning**: runs after each successful archive write, scoped to the location of that archive block. Different archive blocks targeting different directories prune independently.
- **file:// only**: HTTP/HTTPS archives are not pruned (the remote server manages retention). `PruneFileArchives` silently returns for non-file schemes.
- **`uint16` for max-keep**: YANG leaf range is `1..65535`. Zero means no pruning (the feature is off by default).

## Consequences
- Archive directories stay bounded without external cleanup.
- Pruning is non-blocking: errors (permission denied, directory gone) are silently ignored.
- The prefix computation is a neat trick: two dummy timestamps at different dates expose the time-varying portion.

## Gotchas
- `ArchivePrefix` uses `time.Date(2000, 1, 1, ...)` and `time.Date(2000, 1, 2, ...)` as the two reference times. If a custom filename format does not include any time token, the prefix equals the full filename, and pruning would keep exactly N copies of the same filename (which is correct but might surprise operators expecting rotation).
- Path traversal check in `ToFile` catches malicious `{host}` values (e.g., `../../etc/cron.d/evil`), but the pruning function does not re-validate. Safe because `PruneFileArchives` only deletes files already in the target directory.

## Files

None recorded.
