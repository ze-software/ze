---
kind: table
level:
stage:
---
| Flag | Meaning | Flag | Meaning |
|------|---------|------|---------|
| `--dry-run` | Preview | `--socket` | Unix socket path |
| `--log-level` | Logging level | `--no-header` | Exclude headers |

A rendering flag (`--json`, `--text`, `--yaml`, `--format`) is legitimate only on
a tool that reaches no pipe layer. Elsewhere it is the pipe operator's second
spelling: see "What a `--flag` MUST NOT be" above.
