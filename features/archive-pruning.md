# Archive Pruning

## Meta

| Field | Value |
|-------|-------|
| Name | Archive Pruning |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/config/archive |
| Real-path tests | test/ui/config-archive-prune.ci |
| Docs | docs/guide/config-archive.md |
| Doc review | 2026-10-08: ze-system-conf.yang leaf commit-revisions is uint16 range 0..1000 default 0. pruneAfterWrite runs after each successful block write (NewNotifier, Scheduler fireAll and fireByTrigger) when CommitRevisions > 0; PruneFileArchives acts only on file:// locations, counts only names ArchiveMatcher accepts (the whole format, {date} as 8 digits, {time} as 6, other tokens literal, then .conf), removes the oldest by modification time with ties by name, and reports matcher, read and remove failures. docs/guide/config-archive.md "Pruning Old Archives" re-read against that code says the same, including that a hand-named file of the same shape counts |
| Defect review | 2026-10-07: the ArchivePrefix row in plan/journal/unwired-feature.md is marked fixed by the commit that added ArchiveMatcher; no open journal row names this feature's code |
| Extra criteria | supported: a functional test asserting the retained file count = test/ui/config-archive-prune.ci; supported: a user page documenting commit-revisions = docs/guide/config-archive.md |

## Description

`commit-revisions` (0 to 1000, default 0) caps the config archive files kept at each `file://` archive location, pruning the oldest after each archive write; 0 keeps every file, and an HTTP archive is never pruned. Only files whose whole name matches the archive block's filename format are counted or removed. <!-- source: internal/component/config/system/yang/ze-system-conf.yang -- leaf commit-revisions --> <!-- source: internal/component/config/archive/archive.go -- ArchiveMatcher -->
