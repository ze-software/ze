# The config and YANG door

<!-- door-base: internal/component/config -->

Read `ai/doors/system.md` first. This door covers:

- `internal/component/config` (the provider, `transaction`, `yang`, `storage`,
  `cli`, `claims`)
- the YANG modules each component ships in its own `yang/` directory
- env vars
- the config transaction protocol
- config delivery to plugins
- `ze config validate`
- the in-process verifiers

## Where it sits

- **An infrastructure component nearly everything uses.** It is allowed to import
  `plugin`, `plugin/registry` and `command` (`docs/architecture/core-design.md`,
  section 19). It holds zero BGP imports: the dependency runs from the generic
  loader into `bgp/config` (`plan/learned/DESIGN-HISTORY.md`, Configuration,
  load-bearing invariants).
- **`config.Provider` implements `ze.ConfigProvider`**, the single authority the
  editor, the web UI, subsystems and plugins use
  (`docs/architecture/plugin/component-boundaries.md`).
- **The schema is spread across components.** Each component embeds its `.yang`
  files and registers them from `init()` (`docs/architecture/config/yang-config-design.md`,
  "Module Loading"). `schemaFromResolved` (`yang_schema.go`) takes top-level nodes
  from every loaded module whose name ends in `-conf`.
- **It cannot be compiled out.** A gated component's module leaves the schema
  when the component leaves the build.

## Who depends on it from outside

| Outside party | Contract | Page |
|---------------|----------|------|
| An operator | The config grammar the YANG schema accepts, the `set` form, the error text, the `ze config validate` verdict | `docs/architecture/config/syntax.md`, `ai/rules/config.md`, `ai/rules/user-facing-errors.md` |
| An operator | Env var names, and the precedence OS env, then the `environment { }` block, then the default | `docs/architecture/config/environment.md` |
| A third-party plugin | Its config at Stage 2 as `rpc.ConfigSection` (a root plus a JSON body), every value a JSON string, a removed root as `{}` | `docs/plugin-development/protocol.md`, "Stage 2", and `ai/rules/config.md` |
| A third-party plugin | The transaction callbacks `OnConfigVerify`, `OnConfigApply`, `OnConfigRollback`, `OnConfigCommitted` (`pkg/plugin/sdk/sdk_callbacks.go`) and the `sdk.Journal` | `docs/architecture/config/transaction-protocol.md` |
| An external plugin author | An external plugin's own YANG is stored but not compiled, so config under its own root is refused today | `docs/plugin-development/schema.md`, "What works today" |
| API clients, web, MCP | The same schema, and the `-api` and `-cmd` modules that declare RPCs | `yang-config-design.md`, "YANG Module Architecture" |

A leaf rename breaks every saved config and every script that writes it.

## Boundaries

- **Config text changes through a parsed tree or `set` lines only.** No raw text
  surgery and no private merge (`ai/rules/config.md`).
- **A plugin receives only the roots it declares.** On `registry.Registration`,
  `ConfigRoots` are the roots it owns, and a present root auto-loads it.
  `ConfigReads` are roots it reads and that never auto-load it
  (`internal/component/plugin/registry/registry.go`). The live plugin also
  declares `WantsConfig` on `sdk.Registration`, which makes it a reload
  participant (`plugin-relationships.md`, section 7). A plugin lowers its config
  with `Tree.ToPluginMap`, never `Tree.ToMap` (`ai/rules/config.md`).
- **A change is a transaction, not a reload call.** `TxCoordinator`
  (`transaction/orchestrator.go`) runs verify, apply and rollback over stream
  events, one transaction at a time (`transaction-protocol.md`). A backend that
  cannot apply exactly refuses at verify (`ai/rules/protocol.md`).
- **Extend a container with `augment`, never with a second definition.** A
  redefinition replaces it (`DESIGN-HISTORY.md`, Configuration, invariant 334).
- **The generated YANG glue is not hand-edited.** `./le yang glue update`
  regenerates each `yang/` package's `embed.go` and `register.go`.
- **A tunable defaults to a YANG leaf, and env-only is the exception.** The leaf
  `description` names its env override (`ai/rules/config.md`).

## Read before code

| Read | The question it answers |
|------|-------------------------|
| `docs/architecture/config/yang-config-design.md` | How modules load, the four validation layers, "Where a validator actually runs", `ze:` extensions |
| `docs/architecture/config/transaction-protocol.md` | Verify, apply, rollback, exclusion, the apply journal, dependency waiting |
| `docs/architecture/core-design.md`, section 18 | The transaction in one table |
| `docs/architecture/config/syntax.md` | The file grammar an operator writes |
| `docs/architecture/config/environment.md`, `environment-block.md` | Env keys, precedence, the `environment` block |
| `docs/plugin-development/schema.md` | What a plugin can declare in YANG today |
| `ai/digests/config-pipeline.md` | File to tree to BGP peers, and the SIGHUP path |
| `ai/digests/cli-editor.md` | The editor, completion and commit path |
| `ai/patterns/config-option.md` | The steps to add one option |
| `ai/rules/config.md` | Text manipulation, env-only exceptions, string coercion, help texts |
| `ai/rules/protocol.md` | Refuse at verify rather than approximate |

## Traps

- **`ze config validate` never calls a live `OnConfigVerify`.** It runs YANG
  checks and each registration's `InProcessConfigVerifier`
  (`plugin_verify.go` `VerifyPluginConfigContent`). A rule only in
  `OnConfigVerify` passes offline and fails at start, and the in-process
  verifier does not run at daemon startup (`yang-config-design.md`, "Layer 3").
  Source: `plan/journal/validate-accepts-what-start-refuses.md`, and `DESIGN-HISTORY.md`,
  Configuration, invariants.
- **A `ze:validate` can be declared and never run.** The walk checks only the
  sections in `validatedSections` (`yang-config-design.md`, "Where a validator
  actually runs"), and an extension on a typedef does not reach the leaves that
  use it. Source: `DESIGN-HISTORY.md`, Configuration, invariant 356.
- **A removed root arrives as `{}`, and `(nil, nil)` hides it.** A parser that
  returns nothing for an empty body treats deletion as no change, so kernel
  state stays. Source: `plan/journal/component-rebuilt-during-reload.md`, copp
  2026-09-09 and policy-routes 2026-09-20 rows.
- **A root that a reload adds has no running participant.** Reload verifies with
  the plugins already running, so a plugin that the new root would start sees
  nothing. Source: `plan/journal/gate-excludes-part-of-its-population.md`,
  2026-08-25 row.
- **Values arrive as strings.** A direct `v.(bool)` or `v.(float64)` assertion
  silently takes the default. `./le config coercion check` refuses it
  (`ai/rules/config.md`).
- **Two declarations say which roots a plugin reads.** Pages drifted from
  `WantsConfig`. Source: `plan/journal/comment-describes-superseded-behaviour.md`,
  2026-10-14 row. Config also reaches consumers by a third route, which is why
  `./le config claims` exists (`gate-excludes-part-of-its-population.md`, 2026-08-29 row).
- **An env key spelled as a literal in `envPlumbingTable` registers nothing,**
  so the first `env.Set` ends the daemon. Source:
  `plan/journal/env-key-read-but-never-registered.md`, 2026-09-07 row.
- **Two learned entries are stale here.** `RECURRING-PATTERNS.md`, "YANG module
  registered but not in `yang_schema.go`", and the DESIGN-HISTORY "TWO
  registrations" invariant describe an explicit module list. `schemaFromResolved`
  now discovers modules by the `-conf` suffix. "An unregistered plugin callback
  answers OK" describes an old default: a plugin with no `OnConfigApply` now
  gets a refusal (`sdk_callbacks.go` `Plugin.refuseConfigApply`). Trust the code.

## Gates

| Command | What it judges |
|---------|----------------|
| `./le config claims` | Every writable config subtree reaches a consumer |
| `./le config coercion check` | Config parsers coerce the string form |
| `./le config ports check` | Listener default table agrees with YANG refine defaults |
| `./le yang glue check` | Generated `embed.go` and `register.go` match the `.yang` files |
| `./le doc yang-contract command-contract` | YANG command declarations against handlers |
| `./le job run label config command go test -race ./internal/component/config/...` | The scoped package test, including `TestEveryValidatorSectionIsWalkedOrExcused` |
| `./le doc check verify` | Source anchors in any page you edited |

`test/plugin/config-validate-agrees-with-boot.ci` is the functional proof that
offline validation and boot agree.
