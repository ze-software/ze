# Archive Pruning

## Meta

| Field | Value |
|-------|-------|
| Name | Archive Pruning |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/archive |
| Real-path tests | test/ui/config-archive-prune.ci |
| Doc review | 2026-10-07: ze-system-conf.yang leaf commit-revisions is uint16 range 0..1000 default 0; PruneFileArchives runs only when CommitRevisions > 0 and for file:// locations, and keeps only names starting with ArchivePrefix, which retains the 2000-01-0x date digits, so a format carrying {date} or {time}, the default included, is never pruned; the row now says so; docs/guide/config-archive.md is silent on commit-revisions |
| Defect review | 2026-10-07: journal row of 2026-10-07 in plan/journal/unwired-feature.md names ArchivePrefix; test/ui/config-archive-prune.ci is red until it is fixed |
| Extra criteria | supported: a functional test asserting the retained file count = test/ui/config-archive-prune.ci; supported: a user page documenting commit-revisions = none yet |

## Description

`commit-revisions` (0 to 1000, default 0) is meant to cap the config archive files kept at each `file://` archive location, pruning the oldest after each archive write; 0 keeps every file, and an HTTP archive is never pruned. Today the cap prunes nothing when the archive filename format carries `{date}` or `{time}`, which the default format does, so every archive file is kept. <!-- source: internal/component/config/system/yang/ze-system-conf.yang -- leaf commit-revisions --> <!-- source: internal/component/config/archive/archive.go -- ArchivePrefix -->
