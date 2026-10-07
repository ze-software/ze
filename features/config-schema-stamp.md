# Config Schema Stamp

## Meta

| Field | Value |
|-------|-------|
| Name | Config Schema Stamp |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/config/stamp.go |
| Real-path tests | test/ui/config-schema-stamp-recovers.ci |
| Docs | docs/architecture/config/syntax.md |
| Doc review | 2026-10-07: RecoverConfig runs only when the startup LoadConfig fails (cmd/ze/hub/main.go), walks rollback versions newest first, skips those IsNewerRelease, loads the first that parses, backs up the current config with WriteVersion, publishes the result with FormatSchemaStamp, and prunes nothing; the row now says the recovery follows a load failure |
| Defect review | 2026-10-07: no open spec or journal row found naming the schema stamp |
| Extra criteria | supported: a functional test loading a config stamped by a newer release = test/ui/config-schema-stamp-recovers.ci |

## Description

Config files carry a schema version stamp. When a config written by a newer release fails to load at startup, recovery keeps it in history, walks the rollback versions newest first, skips those a newer release wrote, loads the first that parses, and re-stamps it with this release. No field is pruned. <!-- source: internal/component/config/stamp.go -- RecoverConfig -->
