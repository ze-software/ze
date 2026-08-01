# 901: Env Autocomplete

Spec: `plan/spec-env-autocomplete.md`

## Context

Operators had no env-key completion in operational CLI mode or shell
completion, even though the env registry and log subsystem registries
held all the metadata needed. Config-mode YANG completion under
`environment {}` already worked.

## Decisions

1. **Core catalog, not plugin.** Shared env-key catalog lives in
   `internal/core/envcatalog` so both components and plugins can import
   it without violating dependency rules.

2. **ValueHints, not YANG args.** Env keys are wired through the
   existing `command.Node.ValueHints` mechanism (beside `wireRibHints`)
   rather than YANG leaf arguments. The positional grammar
   `show env get <key>` stays unchanged.

3. **ensureEnvPath creates missing nodes.** The env commands are local
   handlers, not RPCs, so `BuildCommandTree` does not produce
   `show > env > get` nodes from RPC registrations. `wireEnvHints` uses
   `ensureEnvPath` to create these nodes with YANG-derived descriptions.
   Empty-description steps (like "show") require the node to already
   exist, preventing spurious node creation.

4. **Shell roots from registry.** Shell script generators derive their
   root command list from `registry.ListRoot()` through one shared
   helper in `root_commands.go`, filtered by section and mode. The
   "show" verb is added synthetically (it is a CLI dispatch verb, not a
   registered root command).

5. **Log-subsystem lookup fallback.** `ze env get ze.log.<subsystem>`
   now resolves through `envcatalog.LookupLogSubsystem` when the key is
   not in the static env registry, using `slogutil.SubsystemInfo` for
   the description.

## Consequences

- Shell completion for `ze env get`, `ze env registered`,
  `ze show env get`, and `ze show env registered` now offers sorted
  public env keys including concrete `ze.log.<subsystem>` rows.
- All four shell generators (bash, zsh, fish, nushell) include `env` as
  a completable root command with dynamic completion.
- `.et` editor tests cannot test operational command-mode completion
  because the headless editor framework operates in config-edit mode.
  Unit tests and `.ci` tests cover the same behavior.

## Gotchas

- Local handlers (registered via `MustRegisterLocal`) do not create
  command-tree nodes. Any feature attaching ValueHints to local-handler
  paths must ensure the nodes exist via `ensureEnvPath` or equivalent.
- `shellVisible()` filters by section and mode but does not perfectly
  match the pre-migration hardcoded list. Some previously hidden commands
  (like `doctor`, `connect`) now appear in shell completion. This is
  intentional: they are registered user-facing commands.

## Files

None recorded.
