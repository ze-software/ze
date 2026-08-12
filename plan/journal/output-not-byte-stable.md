| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-10 | fixit-forward-rail-initial-sync-ordering | mcp | `test/plugin/mcp-tools-list-deterministic-order.ci` fails at HEAD: `tools-order stable=false calls=8 diverged-on-call=7 names=identical byte-offset=22848`. The tool NAMES are stable and the bytes are not, so the divergence is inside a tool's serialized schema, not in the list order the test is named for | not fixed here: found while running `make ze-plugin-test` for an unrelated spec, and it blocks no goal of that spec |
