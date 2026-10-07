# Config Schema Stamp

## Meta

| Field | Value |
|-------|-------|
| Name | Config Schema Stamp |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/stamp.go |
| Docs | docs/architecture/config/syntax.md |
| Doc review | 2026-10-07: RecoverConfig walks rollback versions, skips those IsNewerRelease, loads the first that parses, publishes it with FormatSchemaStamp, and prunes nothing; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming the schema stamp |
| Extra criteria | supported: a functional test loading a config stamped by a newer release = none yet |

## Description

Config files carry a schema version stamp. When a config was written by a newer release, recovery walks the rollback versions newest first, skips those a newer release wrote, loads the first that parses, and re-stamps it with this release. No field is pruned. <!-- source: internal/component/config/stamp.go -- RecoverConfig -->
