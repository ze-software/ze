# 194 — Plugin Decode Features

## Objective

Eliminate per-plugin CLI boilerplate by introducing a data-driven `PluginConfig` struct and shared `RunPlugin()` function in `plugin_common.go`.

## Decisions

- Data-driven approach: plugin declares `PluginConfig{NLRI: true, Capa: true, TextDecode: true}` and `RunPlugin()` handles all flag parsing, dispatch, and error messages — no per-plugin main loop.
- Standard error format: `"error: plugin 'X' does not support --Y (available: --Z)"` — consistent across all plugins.
- Functional tests skipped for this spec due to test framework format mismatch (deferred).

## Patterns

- `--nlri` for NLRI decode plugins, `--capa` for capability plugins, `--text` bool for text output, `--features` for feature discovery.

## Gotchas

- None beyond the deferred functional tests.

## Files

- `internal/component/plugin/cli/plugin_common.go` — PluginConfig, RunPlugin
- `internal/component/bgp/plugins/*/` — updated to use RunPlugin
