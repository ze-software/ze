# Self-Documenting System

## Meta

| Field | Value |
|-------|-------|
| Name | Self-Documenting System |
| Page | docs/features/introspection.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/yang, internal/component/command/node.go, internal/component/cli/model_keys.go, pkg/plugin/rpc/types.go |
| Real-path tests | test/ui/help-command-json-two-fields.ci, test/ui/help-command-json-summary-only.ci, test/ui/help-command-json-argument-texts.ci, test/ui/help-ai-json-rpc-leaf-texts.ci, test/ui/help-parent-node.ci, test/plugin/plugin-command-two-texts.ci |
| Docs | docs/features/introspection.md |
| Doc review | 2026-10-07: model_keys.go handleTab reaches revealExplanation, which opens the long explanation; matches the row |
| Defect review | 2026-10-07: open: plan/journal/declared-summary-without-its-explanation.md row 11 |
| Extra criteria | supported: a gate that every command declares both help texts = none yet |

## Description

Runtime introspection of plugins, env vars, RPCs, schemas, commands. Each command declares two help texts: a one-line summary every listing prints verbatim, and a long explanation only that command's own help page prints. A YANG command node declares them as `ze:help` and `description`. A plugin declares them as `short-help` and `description` in its Stage 1 registration. No surface derives one from the other. Inside `ze cli`, Tab reaches both halves. The completion menu lists command names, and the second message line above the prompt carries the summary of the selected command. Tab with nothing left to complete opens that command's long explanation in its own box. A command argument, an rpc or notification leaf, a config node and an enum value declare the same pair, and every surface that names the node renders both: the CLI JSON help, the site and wiki catalogs, the MCP tool schema, the web admin and config forms, the API and gRPC schemas, the published configuration reference and the completion menu. A config node several modules declare shows every declaration's summary, joined, because nothing in the schema says which module owns the node. <!-- source: internal/component/cli/model_keys.go -- handleTab, revealExplanation --> <!-- source: internal/component/config/yang/rpc.go -- extractEntryLeaves --> <!-- source: internal/component/config/yang/enum.go -- EnumValueSummaries --> <!-- source: internal/component/config/yang/command.go -- GetHelpExtension, IsHelpExtension, mergeHelpText, argDefFor --> <!-- source: internal/component/command/node.go -- Node.ShortHelp, Node.Description --> <!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
