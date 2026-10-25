| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-25 | le-cli-usability | `ze help ai all`, `ze help ai --json`, MCP `ze_reference` (`internal/component/aihelp` `Build`, `matchEnvVars`) | Two runs print different references: `matchEnvVars` breaks a prefix tie by map order (`bmp` gets gokrazy's env vars on some runs), and the RPC order varies. The AC-41 golden test compares sorted lines and skips `Env vars:` | fixed: Fixed 2026-10-25 by "aihelp: one reference per binary, each service its own env vars" (leaf-name matching was the bug) |
