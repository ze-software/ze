# Archive Pruning

## Meta

| Field | Value |
|-------|-------|
| Name | Archive Pruning |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/archive |
| Doc review | 2026-10-07: ze-system-conf.yang leaf commit-revisions is uint16 range 0..1000 default 0; PruneFileArchives runs only when CommitRevisions > 0 and for file:// locations; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming archive pruning |
| Extra criteria | supported: a functional test asserting the retained file count = none yet; supported: a user page documenting commit-revisions = none yet |

## Description

`commit-revisions` (0 to 1000, default 0) caps the config archive files kept at each `file://` archive location: after each archive write, the files beyond the count are pruned, oldest first. 0 keeps every file, and an HTTP archive is never pruned. <!-- source: internal/component/config/system/yang/ze-system-conf.yang -- leaf commit-revisions -->
