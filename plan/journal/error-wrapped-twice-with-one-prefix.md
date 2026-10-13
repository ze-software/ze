# error wrapped twice with one prefix

A caller wraps an error with a prefix that the callee already wrote. Each layer
is correct alone, and the operator reads the same words twice before the cause.
No test sees it, because each layer is tested with its own error.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-13 | spec-config-enum-refusal-proposes-valid-values | hub reload (`runReloadContext`, `cmd/ze/hub/main_reload.go`) | A reload of a file the parser refuses prints `reload: parse config: parse config: line N: ...`: `parseTreeWithYANG` (`config/loader.go`) wraps the parser error as `parse config: %w`, `LoadConfig` returns it through `loadBoth`, and `runReloadContext` wraps it again as `reload: parse config: %w`. A walk refusal is not doubled (`reload: parse config: config validation failed`) | open: recorded while designing the enum refusal spec |
| 2026-10-13 | spec-config-enum-refusal-proposes-valid-values | `ze config validate` on a set-format file (`runValidation`, `config/cli/cmd_validate.go`) | A parse error that is not an enum refusal prints `line N: parse config: line N: ...`: `parseTreeWithYANG` (`config/loader.go`) wraps it as `parse config: %w`, `withoutLinePrefix` trims only a leading `line N: `, so it trims nothing, and the text renderer writes `line N: ` again | open: found in design review round 7 |
