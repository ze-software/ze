# 1331 -- Five config roots nobody claims, and the third delivery path

## Context

Every rule under `ai/rules/` governs how well a single plugin is written. None
asked whether a config root an operator can write is read by anything at all.
`Server.reloadConfig` selects plugins by `WantsConfigRoots`, and when nothing
matches it logs "no affected plugins" and applies the tree regardless.

Measured with an empty allowlist: **5 unclaimed roots covering 172 YANG leaves**,
over 36 roots and 72 claims. `system` alone carries 113 of them.

## Decisions

- **The audit refuses, rather than reports.** An unclaimed root is config the
  operator wrote and nothing consumes, which is indistinguishable from a typo
  that silently did nothing. Chosen over a warning because a warning is what
  this repo has repeatedly proven nobody acts on.
- **It runs three ways: verify stage, unit gate, and doctor check.** The first
  two catch it before merge; the doctor says it on a running daemon, where the
  operator who wrote the config is the one who needs to hear it.
- **The allowlist is EMBEDDED, not testdata.** The doctor needs the same entries
  at run time and `testdata/` is unreachable from a binary. This was a
  correction to the spec's own design, not a preference.
- **Every allowlist entry names its consuming symbol, and reason plus owner are
  mandatory.** An entry whose root later becomes claimed fails, so the list
  cannot quietly become a permanent exemption.

## Consequences

- Adding a YANG root now requires either a plugin that claims it or an allowlist
  entry naming what reads it. That is the intended cost.
- `scripts/checks/port_defaults.go` had no `serviceYANG` mapping for `gnmi`, so
  `ze-port-defaults-check` failed with `unmapped-service` and blocked this gate.
  The gnmi module carried the correct default all along; only the mapping was
  missing. Fixed rather than routed around.

## Gotchas

- **The spec's research missed an entire delivery path, and the gate is what
  found it.** Assumption A-3 assumed two ways config reaches code. There is a
  third: components INSIDE the daemon read the tree directly, never through the
  plugin claim mechanism. Five producers -- `ExtractPluginsFromTree`,
  `pppoe.ExtractParameters`, `extractSmartConfig`, `ExtractTuningFromMap`,
  `extractTelemetryConfig`. A design derived from a spec's assumption list, with
  no mechanical check, will inherit that list's blind spots.
- **A measurement can be wrong in the safe direction and still mislead.** The
  companion leaf-mention report first read 235 findings; struct-tag literals
  (`json:"add-path"`) and YANG list-key leaves were inflating it. The real figure
  is 81 of 1075 leaves. A number nobody sampled is a number nobody should quote.
- **An unclaimed root is not the same as an unused one.** Three of the five are
  read perfectly well, just not through the mechanism the gate understands. The
  gate's job is to make that explicit, not to imply the config is dead.

## Files

- `internal/component/config/claims/` -- `Audit`, `AuditConfigured`, `covers`, `FromConfigRoots`, `FromHubHandlers`, and the embedded allowlist
- `internal/component/config/schema/cli/claims.go` -- `ConfigHandlerPaths`, derived from `buildSchemaRegistry`
- `scripts/checks/config_claims.go` -- the blocking gate, wired into both `stagesForMode` branches
- `internal/component/doctor/checks_config_claims.go` -- `checkConfigClaims`
- `scripts/checks/yang_leaf_mentions.go` -- the advisory leaf report, deliberately in no stage
