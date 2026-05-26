# 791 -- Configurable Default CLI Output Format

## Context

CLI output format was hardcoded to `pipeTable` in `ProcessPipesDefaultTable` and `ProcessPipesDetectLog`. Users who preferred JSON or text output had to append `| json` or `| text` to every command. The goal was to make the default configurable via YANG config and overridable per-session.

## Decisions

- Chose `env.Get`/`env.Set` in the `command` package over caller-passed `pipeKind` parameter because it keeps `ProcessPipes*` signatures unchanged and avoids threading config state through three callers
- Chose process-global session override via `env.Set` over per-Model state because `environment/` values are process-scoped by design and the implementation is simpler
- Placed `env.MustRegister` in `command/pipe.go` (the consumer) over `config/environment.go` (the conventional location) because the `command` package's tests need the registration and don't import `config`
- Renamed `ProcessPipesDefaultTable` to `ProcessPipesDefaultFormat` because the function no longer defaults to table
- Removed the resolve/origin-forces-text heuristic: when the user configures a non-text default, their choice wins over the old readability heuristic

## Consequences

- YANG default is `text`, not `table`. Any existing deployment that relied on the implicit table default will now get text output unless they add `environment { cli { format { default table; } } }` to their config
- `set cli format <value>` in operational mode is process-global: one SSH user changing it affects all connected sessions
- The `set` prefix in operational mode is intercepted by `isConfigCommand` for mode switching. Session-level `set cli format` must be checked BEFORE `isConfigCommand` or it gets routed to config mode

## Gotchas

- `isConfigCommand` maps `set` as a config command, so `set cli format` in operational mode gets silently swallowed into a config-mode switch if the intercept is placed after `isConfigCommand`. The intercept must be placed between the `configure` check and the `isConfigCommand` check in `handleEnter`.
- `handleSetCLIFormat` prefix matching with `strings.HasPrefix("set cli format")` also matches `"set cli formatting"`. Fixed by requiring exact match or space-followed match.

## Files

- `internal/component/hub/schema/ze-hub-conf.yang` -- YANG cli/format/default leaf
- `internal/component/config/constants.go` -- "cli" in extractSections
- `internal/component/config/apply_env.go` -- cli.format plumbing entry
- `internal/component/config/apply_env_test.go` -- TestApplyEnvConfigCLIFormat
- `internal/component/command/pipe.go` -- configuredDefault(), rename, env.MustRegister
- `internal/component/command/pipe_test.go` -- configuredDefault tests, updated existing tests
- `internal/component/cli/model_mode.go` -- updated caller
- `internal/component/cli/model_keys.go` -- handleSetCLIFormat, intercept placement
- `internal/component/cli/model_commands_test.go` -- session override tests
- `test/editor/pipe/set-cli-format.et` -- functional test
- `docs/features.md`, `docs/guide/command-reference.md`, `docs/guide/configuration.md` -- docs
