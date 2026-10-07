# Deactivate / Activate

## Meta

| Field | Value |
|-------|-------|
| Name | Deactivate / Activate |
| Page | docs/guide/config-deactivate.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/parser.go, internal/component/config/prune.go, internal/component/config/cli/cmd_deactivate.go |
| Real-path tests | test/parse/cli-config-deactivate-container.ci, test/parse/cli-config-deactivate-leaf.ci, test/parse/cli-config-activate.ci |
| Docs | docs/guide/config-deactivate.md |
| Doc review | 2026-10-07: PruneInactive removes inactive subtrees at apply and the file keeps them, as the row says (kept in file, skipped at apply) |
| Defect review | 2026-10-07: open: plan/immediate/spec-peer-deactivate-and-bulk-route-purge.md (a deactivated peer is removed rather than paused) |
| Extra criteria | supported: deactivate then activate a peer on a running daemon keeps its identity = none yet |

## Description

Junos-style `inactive:` prefix on any node (leaf, container, list entry, leaf-list value); kept in file, skipped at apply. CLI: `ze config deactivate/activate <file> <path>`. TUI: `deactivate <path>` / `activate <path>`. Engine-level, no schema annotation required. <!-- source: internal/component/config/parser.go -- applyInactive dispatch --> <!-- source: internal/component/config/prune.go -- PruneInactive removes inactive subtrees and leaves --> <!-- source: internal/component/config/cli/cmd_deactivate.go -- one-shot CLI verb -->
