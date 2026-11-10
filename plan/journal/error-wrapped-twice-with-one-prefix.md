# error wrapped twice with one prefix

A caller wraps an error with a prefix that the callee already wrote. Each layer
is correct alone, and the operator reads the same words twice before the cause.
No test sees it, because each layer is tested with its own error.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-13 | spec-config-enum-refusal-proposes-valid-values | hub reload (`runReloadContext`, `cmd/ze/hub/main_reload.go`) | A reload of a file the parser refuses prints `reload: parse config: parse config: line N: ...`: `parseTreeWithYANG` wraps `parse config: %w` and `runReloadContext` wraps `reload: parse config: %w` | fixed: only the loader adds `parse config` (`parseConfigError`) and the reload adds only `reload: ` (spec-operator-errors-name-the-fix AC-5; `test/reload/reload-parse-error-single-prefix.ci`) |
| 2026-10-13 | spec-config-enum-refusal-proposes-valid-values | `ze config validate` on a set-format file (`runValidation`, `config/cli/cmd_validate.go`) | A parse error that is not an enum refusal prints `line N: parse config: line N: ...`: `withoutLinePrefix` trimmed only a leading `line N: ` and the renderer wrote it again | fixed: both parsers return `config.LineError`, the one owner of `line N: `; `config.SplitLine` splits it, text helpers deleted (spec-operator-errors-name-the-fix AC-5; `TestSplitLineNamesTheLineOnce`, `test/parse/validate-parse-error-single-prefix.ci`) |
