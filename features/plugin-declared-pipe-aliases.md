# Plugin-Declared Pipe Aliases

## Meta

| Field | Value |
|-------|-------|
| Name | Plugin-Declared Pipe Aliases |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/command/alias.go, pkg/plugin/rpc/types.go |
| Real-path tests | test/plugin/plugin-pipe-alias.ci, test/plugin/plugin-pipe-alias-collision.ci, test/plugin/plugin-pipe-alias-help.ci, test/plugin/plugin-pipe-alias-namespaced.ci, test/plugin/rpki-pipe-summary.ci, test/ui/plugin-pipe-alias-completion.ci |
| Docs | docs/features/pipe-operators.generated.md |
| Doc review | 2026-10-07: every source anchor resolves (PipeDecl, RegisterPluginAliases, UnregisterPluginAliases, aliasBarriers, summaryAliasExpansion, pipeAliasHelp) |
| Defect review | 2026-10-07: audit found no open immediate spec against pipe aliases; journal rows naming a Component, not each re-verified here: zero-value-as-valid-answer.md:32 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

A pipe alias names an operator chain, so `show bgp \| summary` says what `show bgp \| display router-id local-as uptime peers-configured peers-established` says. A plugin names one for its own commands in the `pipes` list of its Stage 1 registration, and the engine parses the expansion once, at registration. The alias leaves the registry when the plugin stops, so a stopped plugin can start again. The pipe layer selects and re-sequences and computes nothing, so a command that offers an alias MUST emit its aggregate fields beside its detail rows. `show bgp rpki` does this, and `show bgp rpki \| summary` answers the same record as `show bgp rpki summary`. An entry that states an `argument` in place of an expansion is a pipe operation the plugin's own handler runs: it reaches the handler in the `pipe` list of `execute-command`, in the order typed, and the engine applies the rest of the chain to the answer. `command help "<name>"` lists the aliases and operations a command answers to in its `pipe-operations` list, with the chain each alias stands for. A declared alias resolves wherever the daemon reads the chain: `ze cli -c`, the `ze cli` and ssh sessions, a plugin's `dispatch-command`, MCP, REST and gRPC; it does NOT resolve for an operational command typed in `ze config edit`, which reads the chain in the client process. <!-- source: pkg/plugin/rpc/types.go -- PipeDecl, PipeOperation --> <!-- source: internal/component/command/alias.go -- RegisterPluginAliases, UnregisterPluginAliases, aliasBarriers --> <!-- source: internal/component/command/pipe_filter_plugin.go -- RegisterPluginPipeFilters --> <!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, summaryAliasExpansion --> <!-- source: internal/plugins/meta/cmd/help.go -- commandHelp --> <!-- source: internal/component/config/cli/cmd_edit.go -- sshCommandExecutor --> <!-- source: internal/le/cli/stdio/actions.go -- Answer -->
