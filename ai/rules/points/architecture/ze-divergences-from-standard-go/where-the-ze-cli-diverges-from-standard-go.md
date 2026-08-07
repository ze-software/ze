---
kind: table
level:
stage:
---
| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `cobra` or `flag` | YANG-modeled dispatch with RPC handlers | `ai/patterns/cli-command.md` | Unified schema for CLI, web, config, completion |
| `command <identifier> [flags]` | `<verb> <noun> <action> [<identifier>]` | `ai/rules/cli.md` | Identifier-keyword ambiguity elimination |
| Format output as string | Return structured JSON, format via pipe operators | `ai/rules/cli.md` | `\| json`, `\| table`, `\| match`, `\| resolve`, etc. |
| Hardcode help text | Derive from registry/schema | `ai/rules/evidence.md` | Single source of truth; no stale enumerations |
