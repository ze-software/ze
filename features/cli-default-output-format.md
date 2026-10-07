# CLI Default Output Format

## Meta

| Field | Value |
|-------|-------|
| Name | CLI Default Output Format |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/command/pipe.go, internal/component/cli/client/main.go, internal/component/cli/model_keys.go |
| Real-path tests | test/ui/cli-format-default.ci, test/parse/cli-format-default-valid.ci, test/parse/cli-format-default-invalid.ci |
| Docs | docs/features/formatting.md |
| Doc review | 2026-10-07: the format enum in ze-hub-conf.yang carries text, table, json, yaml and ndjson; pipe.go reads ze.cli.format and the configured default |
| Defect review | 2026-10-07: no open spec or journal row found naming the default output format |

## Description

Configurable default output format via `environment { cli { format { default text; } } }`. Supported values: text (default), table, json, yaml, ndjson. Session override via `set cli format <value>` in operational mode. Explicit pipe operators (`\| json`, `\| table`, etc.) always win over the configured default, and over the `--format` flag of `ze cli -c`. <!-- source: internal/component/command/pipe.go -- configuredDefault, ProcessPipesDefaultFormatChecked, HasFormatPipe --> <!-- source: internal/component/cli/client/main.go -- commandWithFormat --> <!-- source: internal/component/cli/model_keys.go -- handleSetCLIFormat -->
